package filebrowser

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Suvukaraz/leanterm/internal/services"
	"github.com/Suvukaraz/leanterm/internal/sftpsvc"
)

// Kind is the browser's backend: SFTP (SSH tabs) or the local file system
// (local shells).
type Kind string

const (
	KindSFTP  Kind = "sftp"
	KindLocal Kind = "local"
)

// Backend is what the browser needs from SFTP or the local file system
// (frontend/src/lib/stores/fsBackend.ts).
type Backend interface {
	Kind() Kind
	// Open prepares the browser of a session and returns the id for the
	// other calls.
	Open(tmpID string) (string, error)
	Getwd(id string) (string, error)
	Home(id string) (string, error)
	List(id, path string, showHidden bool) ([]sftpsvc.FileEntry, error)
	StatVFS(id, path string) (sftpsvc.VFSInfo, error)
	Mkdir(id, path string) error
	CreateFile(id, path string) error
	Remove(id string, paths []string) error
	Rename(id, from, to string) error
	Chmod(id string, paths []string, mode uint32, recursive bool, scope string) error
	OpenWithDefaultApp(id, path string) error
	SetFollow(id string, on bool)
	Close(id string)
}

// SFTPBackend browses an SSH session's SFTP server.
type SFTPBackend struct{ S *services.SFTPService }

func (b SFTPBackend) Kind() Kind                          { return KindSFTP }
func (b SFTPBackend) Open(tmpID string) (string, error)   { return b.S.OpenForSession(tmpID) }
func (b SFTPBackend) Getwd(id string) (string, error)     { return b.S.Getwd(id) }
func (b SFTPBackend) Home(id string) (string, error)      { return b.S.Getwd(id) } // the login folder
func (b SFTPBackend) Mkdir(id, p string) error            { return b.S.Mkdir(id, p, false) }
func (b SFTPBackend) CreateFile(id, p string) error       { return b.S.CreateFile(id, p) }
func (b SFTPBackend) Remove(id string, ps []string) error { return b.S.Remove(id, ps, true) }
func (b SFTPBackend) Rename(id, from, to string) error    { return b.S.Rename(id, from, to) }
func (b SFTPBackend) SetFollow(string, bool)              {} // SSH sessions report folders via OSC 7
func (b SFTPBackend) Close(string)                        {}
func (b SFTPBackend) List(id, p string, hidden bool) ([]sftpsvc.FileEntry, error) {
	return b.S.List(id, p, hidden)
}
func (b SFTPBackend) StatVFS(id, p string) (sftpsvc.VFSInfo, error) { return b.S.StatVFS(id, p) }
func (b SFTPBackend) Chmod(id string, ps []string, mode uint32, rec bool, scope string) error {
	return b.S.Chmod(id, ps, mode, rec, scope)
}
func (b SFTPBackend) OpenWithDefaultApp(id, p string) error {
	_, err := b.S.OpenWithDefaultApp(id, p)
	return err
}

// LocalBackend browses the local file system next to a local shell.
type LocalBackend struct{ S *services.LocalFSService }

func (b LocalBackend) Kind() Kind                        { return KindLocal }
func (b LocalBackend) Open(tmpID string) (string, error) { return tmpID, nil }
func (b LocalBackend) Getwd(id string) (string, error)   { return b.S.Getwd(id) }
func (b LocalBackend) Home(id string) (string, error)    { return b.S.Home(id) }
func (b LocalBackend) List(id, p string, hidden bool) ([]sftpsvc.FileEntry, error) {
	if p == Drives {
		return b.S.Drives(id)
	}
	return b.S.List(id, p, hidden)
}
func (b LocalBackend) StatVFS(id, p string) (sftpsvc.VFSInfo, error) { return b.S.StatVFS(id, p) }
func (b LocalBackend) Mkdir(id, p string) error                      { return b.S.Mkdir(id, p, false) }
func (b LocalBackend) CreateFile(id, p string) error                 { return b.S.CreateFile(id, p) }
func (b LocalBackend) Remove(id string, ps []string) error           { return b.S.Remove(id, ps, true) }
func (b LocalBackend) Rename(id, from, to string) error              { return b.S.Rename(id, from, to) }
func (b LocalBackend) Chmod(id string, ps []string, mode uint32, rec bool, scope string) error {
	return b.S.Chmod(id, ps, mode, rec, scope)
}
func (b LocalBackend) OpenWithDefaultApp(id, p string) error { return b.S.OpenWithDefaultApp(id, p) }
func (b LocalBackend) SetFollow(id string, on bool)          { b.S.SetFollow(id, on) }
func (b LocalBackend) Close(id string)                       { b.S.Close(id) }

// Transfer is an entry of the transfer queue.
type Transfer struct {
	ID        string
	Name      string
	Direction string // upload, download, copy, move
	Progress  int
	Speed     string
	Status    string // running, completed, failed, cancelled
	Err       string
}

// State is what the panel shows (frontend/src/lib/stores/sftp.ts's
// SftpViewState).
type State struct {
	ID         string
	Win        bool
	Start      string
	Current    string
	History    []string
	HistoryIdx int
	Entries    []sftpsvc.FileEntry
	ShowHidden bool
	Follow     bool
	Loading    bool
	Err        string
	VFS        *sftpsvc.VFSInfo
	Selected   []string
	Transfers  []Transfer
}

// Browser is the file browser of one terminal tab. Its methods run the
// backend calls in goroutines (they may go over the network) and call
// Changed when the state changed.
type Browser struct {
	TmpID   string
	Backend Backend
	// SFTP and Local give the transfers (download, upload, local copy and
	// move); either may be nil.
	SFTP  *services.SFTPService
	Local *services.LocalFSService
	// Changed asks for a redraw; it may be called on any goroutine.
	Changed func()
	// OnError reports a failed action (the panel shows it in a dialog).
	OnError func(err error)

	mu      sync.Mutex
	st      State
	opened  bool
	lastCwd string
	nextID  int
}

// Snapshot returns a copy of the state.
func (b *Browser) Snapshot() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.st
	s.History = append([]string(nil), s.History...)
	s.Selected = append([]string(nil), s.Selected...)
	s.Transfers = append([]Transfer(nil), s.Transfers...)
	return s
}

func (b *Browser) update(f func(s *State)) {
	b.mu.Lock()
	f(&b.st)
	b.mu.Unlock()
	if b.Changed != nil {
		b.Changed()
	}
}

func (b *Browser) fail(err error) {
	if err != nil && b.OnError != nil {
		b.OnError(err)
	}
}

// Open connects the browser once (opening the panel); follow starts
// following the terminal's folder (sftp_browser.follow_terminal_folder).
func (b *Browser) Open(showHidden, follow bool) {
	b.mu.Lock()
	if b.opened {
		b.mu.Unlock()
		return
	}
	b.opened = true
	b.mu.Unlock()
	go func() {
		id, err := b.Backend.Open(b.TmpID)
		if err != nil || id == "" {
			msg := "cannot open the browser"
			if err != nil {
				msg = err.Error()
			}
			b.update(func(s *State) { s.Err = msg })
			return
		}
		start, _ := b.Backend.Getwd(id)
		win := b.Backend.Kind() == KindLocal && IsWindowsPath(start)
		if start == "" {
			start = RootPath(win)
		}
		start = NormalizePath(start, win)
		b.update(func(s *State) {
			*s = State{ID: id, Win: win, Start: start, Current: start, History: []string{start}, ShowHidden: showHidden}
		})
		b.load(start)
		if follow {
			b.SetFollow(true)
		}
	}()
}

// Close releases the backend (the tab closed).
func (b *Browser) Close() {
	if id := b.Snapshot().ID; id != "" {
		b.Backend.Close(id)
	}
}

// load lists a folder and puts it in the history (blocking).
func (b *Browser) load(target string) {
	st := b.Snapshot()
	if st.ID == "" {
		return
	}
	norm := NormalizePath(target, st.Win)
	b.update(func(s *State) { s.Loading, s.Err = true, "" })
	entries, err := b.Backend.List(st.ID, norm, st.ShowHidden)
	if err != nil {
		b.update(func(s *State) { s.Loading, s.Err = false, err.Error() })
		return
	}
	b.update(func(s *State) {
		if s.HistoryIdx >= len(s.History) || s.History[s.HistoryIdx] != norm {
			s.History = append(s.History[:min(len(s.History), s.HistoryIdx+1)], norm)
			s.HistoryIdx = len(s.History) - 1
		}
		s.Current, s.Entries, s.Selected, s.Loading, s.Err = norm, entries, nil, false, ""
	})
	if norm != Drives {
		if vfs, err := b.Backend.StatVFS(st.ID, norm); err == nil {
			b.update(func(s *State) { s.VFS = &vfs })
		}
	}
}

// Navigate opens a folder.
func (b *Browser) Navigate(p string) { go b.load(p) }

// Refresh lists the current folder again.
func (b *Browser) Refresh() { go b.load(b.Snapshot().Current) }

// Back and Forward move in the history.
func (b *Browser) Back() {
	b.mu.Lock()
	if b.st.HistoryIdx <= 0 {
		b.mu.Unlock()
		return
	}
	b.st.HistoryIdx--
	p := b.st.History[b.st.HistoryIdx]
	b.mu.Unlock()
	b.Navigate(p)
}

func (b *Browser) Forward() {
	b.mu.Lock()
	if b.st.HistoryIdx >= len(b.st.History)-1 {
		b.mu.Unlock()
		return
	}
	b.st.HistoryIdx++
	p := b.st.History[b.st.HistoryIdx]
	b.mu.Unlock()
	b.Navigate(p)
}

// Up opens the parent folder.
func (b *Browser) Up() {
	st := b.Snapshot()
	if p, ok := ParentPath(st.Current, st.Win); ok {
		b.Navigate(p)
	}
}

// Home opens the home folder (SFTP: the login folder).
func (b *Browser) Home() {
	st := b.Snapshot()
	go func() {
		target := RootPath(st.Win)
		if h, err := b.Backend.Home(st.ID); err == nil && h != "" {
			target = h
		}
		b.load(target)
	}()
}

// ToggleHidden shows or hides dot files.
func (b *Browser) ToggleHidden() {
	b.update(func(s *State) { s.ShowHidden = !s.ShowHidden })
	b.Refresh()
}

// SetFollow follows the terminal's folder; a local shell's folder is
// known, so the browser goes there at once.
func (b *Browser) SetFollow(on bool) {
	st := b.Snapshot()
	b.update(func(s *State) { s.Follow = on })
	b.Backend.SetFollow(st.ID, on)
	if on && b.Backend.Kind() == KindLocal {
		b.GoToTerminalFolder()
	}
}

// TerminalFolder is the folder the terminal reported (OSC 7, /proc); while
// following, the browser goes there.
func (b *Browser) TerminalFolder(dir string) {
	b.mu.Lock()
	b.lastCwd = dir
	follow, cur := b.st.Follow, b.st.Current
	b.mu.Unlock()
	if follow && dir != "" && dir != cur {
		b.Navigate(dir)
	}
}

// GoToTerminalFolder opens the shell's folder (local) or the last one the
// terminal reported.
func (b *Browser) GoToTerminalFolder() {
	st := b.Snapshot()
	b.mu.Lock()
	target := b.lastCwd
	b.mu.Unlock()
	go func() {
		if b.Backend.Kind() == KindLocal {
			if d, err := b.Backend.Getwd(st.ID); err == nil {
				target = d
			}
		}
		if target != "" && target != st.Current {
			b.load(target)
		}
	}()
}

// SetSelected replaces the selection.
func (b *Browser) SetSelected(paths []string) {
	b.update(func(s *State) { s.Selected = paths })
}

// act runs a backend action, reports its error and lists the folder again.
func (b *Browser) act(f func(id string) error) {
	st := b.Snapshot()
	if st.ID == "" {
		return
	}
	go func() {
		b.fail(f(st.ID))
		b.load(b.Snapshot().Current)
	}()
}

func (b *Browser) CreateFolder(name string) {
	st := b.Snapshot()
	b.act(func(id string) error {
		return b.Backend.Mkdir(id, JoinPath(st.Current, strings.TrimSpace(name), st.Win))
	})
}

func (b *Browser) CreateFile(name string) {
	st := b.Snapshot()
	b.act(func(id string) error {
		return b.Backend.CreateFile(id, JoinPath(st.Current, strings.TrimSpace(name), st.Win))
	})
}

func (b *Browser) Remove(paths []string) {
	b.act(func(id string) error { return b.Backend.Remove(id, paths) })
}

func (b *Browser) Rename(from, newName string) {
	win := b.Snapshot().Win
	dir, ok := ParentPath(from, win)
	if !ok {
		dir = RootPath(win)
	}
	b.act(func(id string) error {
		return b.Backend.Rename(id, from, JoinPath(dir, strings.TrimSpace(newName), win))
	})
}

func (b *Browser) Chmod(paths []string, mode uint32, recursive bool, scope string) {
	b.act(func(id string) error { return b.Backend.Chmod(id, paths, mode, recursive, scope) })
}

// OpenWithDefaultApp opens a file in its application (SFTP: a temporary
// copy, watched for changes).
func (b *Browser) OpenWithDefaultApp(p string) {
	st := b.Snapshot()
	go func() { b.fail(b.Backend.OpenWithDefaultApp(st.ID, p)) }()
}

// ---- transfers ------------------------------------------------------------------

func (b *Browser) addTransfer(dir string, paths []string, speed string) string {
	b.mu.Lock()
	b.nextID++
	id := fmt.Sprintf("%s_%d_%d", dir, time.Now().UnixNano(), b.nextID)
	b.mu.Unlock()
	name := fmt.Sprintf("%d files", len(paths))
	if dir == "copy" || dir == "move" {
		name = fmt.Sprintf("%d items", len(paths))
	}
	if len(paths) == 1 {
		name = BaseName(paths[0])
	}
	t := Transfer{ID: id, Name: name, Direction: dir, Progress: 10, Speed: speed, Status: "running"}
	b.update(func(s *State) { s.Transfers = append([]Transfer{t}, s.Transfers...) })
	return id
}

func (b *Browser) setTransfer(id string, f func(t *Transfer)) {
	b.update(func(s *State) {
		for i := range s.Transfers {
			if s.Transfers[i].ID == id {
				f(&s.Transfers[i])
			}
		}
	})
}

func (b *Browser) finish(id string, err error, done string) {
	b.setTransfer(id, func(t *Transfer) {
		if err != nil {
			t.Status, t.Err = "failed", err.Error()
			return
		}
		t.Progress, t.Speed, t.Status = 100, done, "completed"
	})
}

// Download copies remote paths into localDir ("" = the download folder).
func (b *Browser) Download(paths []string, localDir string) {
	st := b.Snapshot()
	if b.SFTP == nil || len(paths) == 0 {
		return
	}
	id := b.addTransfer("download", paths, "Calculating...")
	go func() {
		saved, err := b.SFTP.Download(st.ID, paths, localDir)
		done := "Done"
		if saved != "" {
			done = "Saved to " + saved
		}
		b.finish(id, err, done)
		b.fail(err)
	}()
}

// Upload copies local paths into the current remote folder.
func (b *Browser) Upload(paths []string) {
	st := b.Snapshot()
	if b.SFTP == nil || len(paths) == 0 {
		return
	}
	id := b.addTransfer("upload", paths, "Calculating...")
	go func() {
		err := b.SFTP.Upload(st.ID, paths, st.Current)
		b.finish(id, err, "Done")
		b.fail(err)
		b.load(b.Snapshot().Current)
	}()
}

// UploadBack sends a file changed in its application back to the server.
func (b *Browser) UploadBack(localPath, remotePath string) {
	st := b.Snapshot()
	if b.SFTP == nil {
		return
	}
	go func() {
		b.fail(b.SFTP.UploadBack(st.ID, localPath, remotePath))
		b.load(b.Snapshot().Current)
	}()
}

// CopyOrMove copies or moves local paths into dstDir (local tabs).
func (b *Browser) CopyOrMove(op string, paths []string, dstDir string, overwrite bool) {
	st := b.Snapshot()
	if b.Local == nil || len(paths) == 0 {
		return
	}
	speed := "Copying..."
	if op == "move" {
		speed = "Moving..."
	}
	id := b.addTransfer(op, paths, speed)
	go func() {
		var err error
		if op == "move" {
			err = b.Local.Move(st.ID, paths, dstDir, overwrite)
		} else {
			err = b.Local.Copy(st.ID, paths, dstDir, overwrite)
		}
		b.finish(id, err, "Done → "+dstDir)
		b.fail(err)
		b.load(b.Snapshot().Current)
	}()
}

// Conflicts are the names of paths that already exist in dstDir (blocking).
func (b *Browser) Conflicts(paths []string, dstDir string) []string {
	st := b.Snapshot()
	if b.Backend.Kind() == KindLocal && b.Local != nil {
		names, _ := b.Local.Conflicts(st.ID, paths, dstDir)
		return names
	}
	existing := map[string]bool{}
	for _, e := range st.Entries {
		existing[e.Name] = true
	}
	var out []string
	for _, p := range paths {
		if n := BaseName(p); existing[n] {
			out = append(out, n)
		}
	}
	return out
}

// Cancel marks a transfer cancelled; Dismiss removes it from the queue.
func (b *Browser) Cancel(id string) { b.setTransfer(id, func(t *Transfer) { t.Status = "cancelled" }) }

func (b *Browser) Dismiss(id string) {
	b.update(func(s *State) {
		out := s.Transfers[:0]
		for _, t := range s.Transfers {
			if t.ID != id {
				out = append(out, t)
			}
		}
		s.Transfers = out
	})
}

// ---- sorting ----------------------------------------------------------------------

// SortBy is a column of the file list.
type SortBy string

const (
	SortName  SortBy = "name"
	SortSize  SortBy = "size"
	SortMod   SortBy = "modTime"
	SortPerms SortBy = "permissions"
)

// Sorted orders entries like SftpFileList.svelte: folders first, then by
// the column (names naturally, ignoring case).
func Sorted(entries []sftpsvc.FileEntry, by SortBy, asc bool) []sftpsvc.FileEntry {
	out := append([]sftpsvc.FileEntry(nil), entries...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		var c int
		switch by {
		case SortSize:
			c = cmpInt(a.Size, b.Size)
		case SortMod:
			c = cmpInt(a.ModTime.UnixNano(), b.ModTime.UnixNano())
		case SortPerms:
			c = strings.Compare(a.Permissions, b.Permissions)
		default:
			c = naturalCompare(a.Name, b.Name)
		}
		if !asc {
			c = -c
		}
		return c < 0
	})
	return out
}

func cmpInt(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// naturalCompare compares case-insensitively with digit runs as numbers
// (localeCompare with numeric: true, sensitivity: base).
func naturalCompare(a, b string) int {
	a, b = strings.ToLower(a), strings.ToLower(b)
	for a != "" && b != "" {
		if isDigit(a[0]) && isDigit(b[0]) {
			i, j := digits(a), digits(b)
			na, nb := strings.TrimLeft(a[:i], "0"), strings.TrimLeft(b[:j], "0")
			if len(na) != len(nb) {
				return cmpInt(int64(len(na)), int64(len(nb)))
			}
			if c := strings.Compare(na, nb); c != 0 {
				return c
			}
			a, b = a[i:], b[j:]
			continue
		}
		if a[0] != b[0] {
			return cmpInt(int64(a[0]), int64(b[0]))
		}
		a, b = a[1:], b[1:]
	}
	return cmpInt(int64(len(a)), int64(len(b)))
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func digits(s string) int {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return i
}

// FormatBytes formats a size like the panel (B, KB, MB, GB).
func FormatBytes(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1<<20:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	case n < 1<<30:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
}
