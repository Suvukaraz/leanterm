package sftpsvc

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// EventEmitter defines the interface for event delivery to the frontend.
type EventEmitter interface {
	Emit(event string, data any)
}

type FileEntry struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	Mode        uint32    `json:"mode"`
	ModTime     time.Time `json:"modTime"`
	IsDir       bool      `json:"isDir"`
	IsLink      bool      `json:"isLink"`
	LinkTarget  string    `json:"linkTarget,omitempty"`
	LinkBroken  bool      `json:"linkBroken,omitempty"`
	Owner       string    `json:"owner,omitempty"`
	Group       string    `json:"group,omitempty"`
	Permissions string    `json:"permissions"`
}

type VFSInfo struct {
	TotalSpace uint64 `json:"totalSpace"`
	FreeSpace  uint64 `json:"freeSpace"`
}

type SFTPClientInstance struct {
	client    *sftp.Client
	sshClient *ssh.Client
	cwd       string
}

type SFTPManager struct {
	mu      sync.Mutex
	clients map[string]*SFTPClientInstance // sftpID -> instance
	session map[string]string              // tmpID -> sftpID
}

func NewSFTPManager() *SFTPManager {
	return &SFTPManager{
		clients: make(map[string]*SFTPClientInstance),
		session: make(map[string]string),
	}
}

func (m *SFTPManager) OpenForSession(tmpID string, sshClient *ssh.Client) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sftpID, exists := m.session[tmpID]; exists {
		if _, ok := m.clients[sftpID]; ok {
			return sftpID, nil
		}
	}

	if sshClient == nil {
		return "", errors.New("SSH client is not connected")
	}

	client, err := sftp.NewClient(
		sshClient,
		sftp.MaxPacket(32768),
		sftp.UseConcurrentReads(true),
		sftp.UseConcurrentWrites(true),
		sftp.MaxConcurrentRequestsPerFile(64),
	)
	if err != nil {
		return "", fmt.Errorf("failed to start SFTP subsystem: %w", err)
	}

	cwd, err := client.Getwd()
	if err != nil || cwd == "" {
		cwd = "/"
	}

	sftpID := tmpID // Use tmpID as sftpID for 1:1 mapping
	m.clients[sftpID] = &SFTPClientInstance{
		client:    client,
		sshClient: sshClient,
		cwd:       cwd,
	}
	m.session[tmpID] = sftpID

	return sftpID, nil
}

func (m *SFTPManager) CloseForSession(tmpID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sftpID, exists := m.session[tmpID]
	if !exists {
		return nil
	}

	delete(m.session, tmpID)
	if inst, ok := m.clients[sftpID]; ok {
		delete(m.clients, sftpID)
		return inst.client.Close()
	}
	return nil
}

func (m *SFTPManager) getClient(sftpID string) (*sftp.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.clients[sftpID]
	if !ok || inst.client == nil {
		return nil, fmt.Errorf("SFTP client %s not found or disconnected", sftpID)
	}
	return inst.client, nil
}

func (m *SFTPManager) Getwd(sftpID string) (string, error) {
	client, err := m.getClient(sftpID)
	if err != nil {
		return "", err
	}
	return client.Getwd()
}

func (m *SFTPManager) RealPath(sftpID, p string) (string, error) {
	client, err := m.getClient(sftpID)
	if err != nil {
		return "", err
	}
	return client.RealPath(p)
}

func (m *SFTPManager) List(sftpID, dirPath string, showHidden bool) ([]FileEntry, error) {
	client, err := m.getClient(sftpID)
	if err != nil {
		return nil, err
	}

	if dirPath == "" {
		dirPath, _ = client.Getwd()
		if dirPath == "" {
			dirPath = "/"
		}
	}

	entries, err := client.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var result []FileEntry
	for _, fi := range entries {
		name := fi.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}

		fullPath := path.Join(dirPath, name)
		mode := uint32(fi.Mode())
		isLink := fi.Mode()&os.ModeSymlink != 0
		linkTarget := ""
		linkBroken := false

		if isLink {
			target, err := client.ReadLink(fullPath)
			if err == nil {
				linkTarget = target
				// Verify if target resolves
				_, statErr := client.Stat(fullPath)
				linkBroken = statErr != nil
			}
		}

		result = append(result, FileEntry{
			Name:        name,
			Path:        fullPath,
			Size:        fi.Size(),
			Mode:        mode,
			ModTime:     fi.ModTime(),
			IsDir:       fi.IsDir(),
			IsLink:      isLink,
			LinkTarget:  linkTarget,
			LinkBroken:  linkBroken,
			Permissions: fi.Mode().String(),
		})
	}

	return result, nil
}

func (m *SFTPManager) Stat(sftpID, p string) (FileEntry, error) {
	client, err := m.getClient(sftpID)
	if err != nil {
		return FileEntry{}, err
	}

	fi, err := client.Stat(p)
	if err != nil {
		return FileEntry{}, err
	}

	return FileEntry{
		Name:        fi.Name(),
		Path:        p,
		Size:        fi.Size(),
		Mode:        uint32(fi.Mode()),
		ModTime:     fi.ModTime(),
		IsDir:       fi.IsDir(),
		Permissions: fi.Mode().String(),
	}, nil
}

func (m *SFTPManager) Mkdir(sftpID, p string, parents bool) error {
	client, err := m.getClient(sftpID)
	if err != nil {
		return err
	}
	if parents {
		return client.MkdirAll(p)
	}
	return client.Mkdir(p)
}

func (m *SFTPManager) CreateFile(sftpID, p string) error {
	client, err := m.getClient(sftpID)
	if err != nil {
		return err
	}
	f, err := client.Create(p)
	if err != nil {
		return err
	}
	return f.Close()
}

func (m *SFTPManager) Remove(sftpID string, paths []string, recursive bool) error {
	client, err := m.getClient(sftpID)
	if err != nil {
		return err
	}

	for _, p := range paths {
		if recursive {
			if err := m.removeRecursive(client, p); err != nil {
				return err
			}
		} else {
			if err := client.Remove(p); err != nil {
				_ = client.RemoveDirectory(p)
			}
		}
	}
	return nil
}

func (m *SFTPManager) removeRecursive(client *sftp.Client, p string) error {
	fi, err := client.Lstat(p)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		entries, err := client.ReadDir(p)
		if err == nil {
			for _, e := range entries {
				_ = m.removeRecursive(client, path.Join(p, e.Name()))
			}
		}
		return client.RemoveDirectory(p)
	}
	return client.Remove(p)
}

func (m *SFTPManager) Rename(sftpID, from, to string) error {
	client, err := m.getClient(sftpID)
	if err != nil {
		return err
	}
	return client.Rename(from, to)
}

func (m *SFTPManager) StatVFS(sftpID, p string) (VFSInfo, error) {
	client, err := m.getClient(sftpID)
	if err != nil {
		return VFSInfo{}, err
	}
	stat, err := client.StatVFS(p)
	if err != nil {
		return VFSInfo{}, err
	}
	return VFSInfo{
		TotalSpace: stat.TotalSpace(),
		FreeSpace:  stat.FreeSpace(),
	}, nil
}

func (m *SFTPManager) Chmod(sftpID string, paths []string, mode uint32, recursive bool, scope string) error {
	client, err := m.getClient(sftpID)
	if err != nil {
		return err
	}

	for _, p := range paths {
		fi, statErr := client.Lstat(p)
		shouldApply := true
		if statErr == nil {
			if scope == "dirs" && !fi.IsDir() {
				shouldApply = false
			} else if scope == "files" && fi.IsDir() {
				shouldApply = false
			}
		}

		if shouldApply {
			if err := client.Chmod(p, os.FileMode(mode)); err != nil {
				return err
			}
		}

		if recursive && statErr == nil && fi.IsDir() {
			entries, _ := client.ReadDir(p)
			for _, e := range entries {
				_ = m.Chmod(sftpID, []string{path.Join(p, e.Name())}, mode, true, scope)
			}
		}
	}
	return nil
}

// Download copies remote files or folders (recursively) into localDir.
func (m *SFTPManager) Download(sftpID string, remotePaths []string, localDir string) error {
	return m.DownloadWithOptions(sftpID, remotePaths, localDir, DefaultTransferOptions())
}

// Upload copies local files or folders (recursively) into remoteDir.
func (m *SFTPManager) Upload(sftpID string, localPaths []string, remoteDir string) error {
	return m.UploadWithOptions(sftpID, localPaths, remoteDir, DefaultTransferOptions())
}

// watchForEdits reports saves of localPath. It watches the folder (editors often save
// via a temp file + rename, which a file watch would miss) and debounces the burst of
// events one save produces. Watching stops after 12 hours.
func watchForEdits(dir, localPath string, onSaved func()) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	if err := watcher.Add(dir); err != nil {
		_ = watcher.Close()
		return
	}
	go func() {
		defer watcher.Close()
		var debounce <-chan time.Time
		lifetime := time.After(12 * time.Hour)
		for {
			select {
			case ev, ok := <-watcher.Events:
				if !ok {
					return
				}
				if filepath.Clean(ev.Name) == filepath.Clean(localPath) &&
					ev.Has(fsnotify.Write|fsnotify.Create|fsnotify.Rename) {
					debounce = time.After(700 * time.Millisecond)
				}
			case <-debounce:
				debounce = nil
				if _, err := os.Stat(localPath); err == nil {
					onSaved()
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			case <-lifetime:
				return
			}
		}
	}()
}

// OpenWithDefaultApp downloads a remote file to a temp folder, opens it with xdg-open,
// and watches it for modifications with fsnotify.
func (m *SFTPManager) OpenWithDefaultApp(sftpID string, remotePath string, emitter EventEmitter) (string, error) {
	client, err := m.getClient(sftpID)
	if err != nil {
		return "", err
	}

	stat, err := client.Stat(remotePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat remote file '%s': %w", remotePath, err)
	}
	if stat.IsDir() {
		return "", fmt.Errorf("'%s' is a directory", remotePath)
	}

	baseName := filepath.Base(remotePath)
	tempDir, err := os.MkdirTemp("", "leanterm-open-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	localPath := filepath.Join(tempDir, baseName)

	rf, err := client.Open(remotePath)
	if err != nil {
		return "", fmt.Errorf("failed to open remote file: %w", err)
	}
	defer rf.Close()

	lf, err := os.Create(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to create local temp file: %w", err)
	}
	if _, err := io.Copy(lf, rf); err != nil {
		_ = lf.Close()
		return "", fmt.Errorf("failed to download remote file: %w", err)
	}
	_ = lf.Close()

	_ = os.Chtimes(localPath, stat.ModTime(), stat.ModTime())

	// Launch the default desktop application. xdg-open returns quickly; a non-zero
	// exit means no application is associated with the file type.
	if out, err := exec.Command("xdg-open", localPath).CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return localPath, fmt.Errorf("no application could open '%s': %s", baseName, msg)
	}

	watchForEdits(tempDir, localPath, func() {
		if emitter != nil {
			emitter.Emit("sftp:file:modified", map[string]string{
				"sftpID":     sftpID,
				"remotePath": remotePath,
				"localPath":  localPath,
			})
		}
	})

	return localPath, nil
}

// UploadBack writes the modified local temporary file back to its remote path.
func (m *SFTPManager) UploadBack(sftpID string, localPath string, remotePath string) error {
	client, err := m.getClient(sftpID)
	if err != nil {
		return err
	}

	lf, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer lf.Close()

	rf, err := client.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer rf.Close()

	if _, err := io.Copy(rf, lf); err != nil {
		return fmt.Errorf("failed to upload file back: %w", err)
	}

	return nil
}
