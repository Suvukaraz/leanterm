package services

import (
	"errors"
	"sync"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/session"
	"github.com/Suvukaraz/leanterm/internal/sftpsvc"
)

type SFTPService struct {
	mu           sync.Mutex
	mgr          *sftpsvc.SFTPManager
	sessionMgr   *session.SessionManager
	emitter      sftpsvc.EventEmitter
	panelVisible map[string]bool
	store        *config.Store
}

func NewSFTPService(mgr *sftpsvc.SFTPManager, sessionMgr *session.SessionManager, emitter sftpsvc.EventEmitter) *SFTPService {
	if mgr == nil {
		mgr = sftpsvc.NewSFTPManager()
	}
	return &SFTPService{
		mgr:          mgr,
		sessionMgr:   sessionMgr,
		emitter:      emitter,
		panelVisible: make(map[string]bool),
	}
}

func (s *SFTPService) SetPanelVisible(tmpID string, visible bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panelVisible[tmpID] = visible
}

func (s *SFTPService) IsPanelVisible(tmpID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.panelVisible[tmpID]
}

func (s *SFTPService) OpenForSession(tmpID string) (string, error) {
	if s.sessionMgr == nil {
		return "", errors.New("session manager not available")
	}

	sshClient := s.sessionMgr.GetSSHClient(tmpID)
	if sshClient == nil {
		return "", errors.New("no active SSH client for session")
	}

	return s.mgr.OpenForSession(tmpID, sshClient)
}

func (s *SFTPService) CloseForSession(tmpID string) error {
	return s.mgr.CloseForSession(tmpID)
}

func (s *SFTPService) Getwd(sftpID string) (string, error) {
	return s.mgr.Getwd(sftpID)
}

func (s *SFTPService) RealPath(sftpID, p string) (string, error) {
	return s.mgr.RealPath(sftpID, p)
}

func (s *SFTPService) List(sftpID, path string, showHidden bool) ([]sftpsvc.FileEntry, error) {
	return s.mgr.List(sftpID, path, showHidden)
}

func (s *SFTPService) Stat(sftpID, path string) (sftpsvc.FileEntry, error) {
	return s.mgr.Stat(sftpID, path)
}

func (s *SFTPService) Mkdir(sftpID, path string, parents bool) error {
	return s.mgr.Mkdir(sftpID, path, parents)
}

func (s *SFTPService) CreateFile(sftpID, path string) error {
	return s.mgr.CreateFile(sftpID, path)
}

func (s *SFTPService) Remove(sftpID string, paths []string, recursive bool) error {
	return s.mgr.Remove(sftpID, paths, recursive)
}

func (s *SFTPService) Rename(sftpID, from, to string) error {
	return s.mgr.Rename(sftpID, from, to)
}

func (s *SFTPService) Chmod(sftpID string, paths []string, mode uint32, recursive bool, scope string) error {
	return s.mgr.Chmod(sftpID, paths, mode, recursive, scope)
}

func (s *SFTPService) StatVFS(sftpID, path string) (sftpsvc.VFSInfo, error) {
	return s.mgr.StatVFS(sftpID, path)
}

// Download copies remote files/folders to localDir ("" = configured download folder)
// and returns the folder the files were written to.
func (s *SFTPService) Download(sftpID string, remotePaths []string, localDir string) (string, error) {
	opts, dir := s.transferSettings()
	if localDir == "" {
		localDir = dir
	}
	return localDir, s.mgr.DownloadWithOptions(sftpID, remotePaths, localDir, opts)
}

func (s *SFTPService) Upload(sftpID string, localPaths []string, remoteDir string) error {
	opts, _ := s.transferSettings()
	return s.mgr.UploadWithOptions(sftpID, localPaths, remoteDir, opts)
}

// SetStore gives the service access to the [sftp_browser] preferences.
func (s *SFTPService) SetStore(store *config.Store) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store = store
}

func (s *SFTPService) transferSettings() (sftpsvc.TransferOptions, string) {
	s.mu.Lock()
	store := s.store
	s.mu.Unlock()
	if store == nil {
		return sftpsvc.DefaultTransferOptions(), sftpsvc.DefaultDownloadDir("")
	}
	b := store.Config().SFTPBrowser
	return sftpsvc.TransferOptions{
		PreserveTimestamps:  b.PreserveTimestamps,
		PreservePermissions: b.PreservePermissions,
		UsePartFiles:        b.UsePartFiles,
	}, sftpsvc.DefaultDownloadDir(b.DownloadDir)
}

func (s *SFTPService) OpenWithDefaultApp(sftpID, remotePath string) (string, error) {
	s.mu.Lock()
	emitter := s.emitter
	s.mu.Unlock()
	return s.mgr.OpenWithDefaultApp(sftpID, remotePath, emitter)
}

func (s *SFTPService) UploadBack(sftpID, localPath, remotePath string) error {
	return s.mgr.UploadBack(sftpID, localPath, remotePath)
}
