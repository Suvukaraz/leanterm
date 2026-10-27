package services

import (
	"sync"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/localfs"
	"github.com/Suvukaraz/leanterm/internal/session"
	"github.com/Suvukaraz/leanterm/internal/sftpsvc"
)

// LocalFSService backs the file browser of local shell tabs. Its methods mirror
// SFTPService so the frontend can use either through one interface; the id passed
// in is the session's tmpID.
type LocalFSService struct {
	mu      sync.Mutex
	tracker *localfs.Tracker
	store   *config.Store
}

func NewLocalFSService(sessionMgr *session.SessionManager, emitter localfs.EventEmitter) *LocalFSService {
	pidOf := func(tmpID string) int {
		if sessionMgr == nil {
			return 0
		}
		return sessionMgr.LocalShellPID(tmpID)
	}
	return &LocalFSService{tracker: localfs.NewTracker(pidOf, emitter)}
}

// SetStore gives the service access to the [sftp_browser] preferences.
func (s *LocalFSService) SetStore(store *config.Store) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store = store
}

func (s *LocalFSService) copyOptions(overwrite bool) localfs.CopyOptions {
	s.mu.Lock()
	store := s.store
	s.mu.Unlock()
	opts := localfs.CopyOptions{PreserveTimestamps: true, PreservePermissions: true, Overwrite: overwrite}
	if store != nil {
		b := store.Config().SFTPBrowser
		opts.PreserveTimestamps, opts.PreservePermissions = b.PreserveTimestamps, b.PreservePermissions
	}
	return opts
}

// Getwd returns the working directory of the shell in tmpID (home if unknown).
func (s *LocalFSService) Getwd(tmpID string) (string, error) {
	return s.tracker.Getwd(tmpID)
}

// Home returns the user's home folder (the file browser's Home button).
func (s *LocalFSService) Home(tmpID string) (string, error) {
	return localfs.Home()
}

// SetFollow turns "follow terminal folder" on or off; changes arrive as
// "session:cwd:<tmpID>" events.
func (s *LocalFSService) SetFollow(tmpID string, on bool) {
	s.tracker.SetFollow(tmpID, on)
}

// Close stops following when the browser of tmpID goes away.
func (s *LocalFSService) Close(tmpID string) {
	s.tracker.Stop(tmpID)
}

func (s *LocalFSService) List(tmpID, path string, showHidden bool) ([]sftpsvc.FileEntry, error) {
	return localfs.List(path, showHidden)
}

func (s *LocalFSService) Stat(tmpID, path string) (sftpsvc.FileEntry, error) {
	return localfs.Stat(path)
}

func (s *LocalFSService) Mkdir(tmpID, path string, parents bool) error {
	return localfs.Mkdir(path, parents)
}

func (s *LocalFSService) CreateFile(tmpID, path string) error {
	return localfs.CreateFile(path)
}

func (s *LocalFSService) Remove(tmpID string, paths []string, recursive bool) error {
	return localfs.Remove(paths)
}

func (s *LocalFSService) Rename(tmpID, from, to string) error {
	return localfs.Rename(from, to)
}

func (s *LocalFSService) Chmod(tmpID string, paths []string, mode uint32, recursive bool, scope string) error {
	return localfs.Chmod(paths, mode, recursive, scope)
}

func (s *LocalFSService) StatVFS(tmpID, path string) (sftpsvc.VFSInfo, error) {
	return localfs.StatVFS(path)
}

// Conflicts returns the names of srcPaths that already exist in dstDir.
func (s *LocalFSService) Conflicts(tmpID string, srcPaths []string, dstDir string) ([]string, error) {
	return localfs.Conflicts(srcPaths, dstDir)
}

// Copy copies srcPaths into dstDir; existing targets are replaced only with overwrite.
func (s *LocalFSService) Copy(tmpID string, srcPaths []string, dstDir string, overwrite bool) error {
	return localfs.Copy(srcPaths, dstDir, s.copyOptions(overwrite))
}

// Move moves srcPaths into dstDir; existing targets are replaced only with overwrite.
func (s *LocalFSService) Move(tmpID string, srcPaths []string, dstDir string, overwrite bool) error {
	return localfs.Move(srcPaths, dstDir, s.copyOptions(overwrite))
}

// OpenWithDefaultApp opens the file directly with xdg-open (no temporary copy).
func (s *LocalFSService) OpenWithDefaultApp(tmpID, path string) error {
	return localfs.OpenDefault(path)
}
