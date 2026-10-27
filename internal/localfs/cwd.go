package localfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// EventEmitter delivers events to the frontend.
type EventEmitter interface {
	Emit(event string, data any)
}

// PIDFunc returns the shell process id of a local session, 0 if it is not running.
type PIDFunc func(tmpID string) int

// CwdPollInterval is how often a followed shell's working directory is checked.
var CwdPollInterval = time.Second

// Tracker reads the working directory of local shells from /proc/<pid>/cwd and, while
// "follow terminal folder" is on, reports changes as "session:cwd:<tmpID>" events
// (the same event SSH sessions emit for OSC 7), so no shell integration is needed.
type Tracker struct {
	mu      sync.Mutex
	pidOf   PIDFunc
	emitter EventEmitter
	follow  map[string]chan struct{} // tmpID -> stop channel of the poller
}

func NewTracker(pidOf PIDFunc, emitter EventEmitter) *Tracker {
	return &Tracker{pidOf: pidOf, emitter: emitter, follow: make(map[string]chan struct{})}
}

// ProcessCwd returns the working directory of process pid.
func ProcessCwd(pid int) (string, error) {
	if pid <= 0 {
		return "", fmt.Errorf("shell is not running")
	}
	cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
	if err != nil {
		return "", friendly(err)
	}
	return strings.TrimSuffix(cwd, " (deleted)"), nil
}

// Getwd returns the shell's working directory, falling back to the home folder.
func (t *Tracker) Getwd(tmpID string) (string, error) {
	if cwd, err := ProcessCwd(t.pidOf(tmpID)); err == nil {
		return cwd, nil
	}
	return Home()
}

// Home returns the user's home folder, or "/" if it is unknown.
func Home() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		return "/", nil
	}
	return filepath.Clean(home), nil
}

// SetFollow starts or stops reporting directory changes of the shell in tmpID.
func (t *Tracker) SetFollow(tmpID string, on bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	stop, running := t.follow[tmpID]
	if !on {
		if running {
			close(stop)
			delete(t.follow, tmpID)
		}
		return
	}
	if running {
		return
	}
	stop = make(chan struct{})
	t.follow[tmpID] = stop
	go t.poll(tmpID, stop)
}

// Stop ends following for tmpID (session closed).
func (t *Tracker) Stop(tmpID string) {
	t.SetFollow(tmpID, false)
}

func (t *Tracker) poll(tmpID string, stop chan struct{}) {
	ticker := time.NewTicker(CwdPollInterval)
	defer ticker.Stop()
	last, _ := ProcessCwd(t.pidOf(tmpID))
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			// A reconnected shell has a new pid, so it is looked up every time.
			cwd, err := ProcessCwd(t.pidOf(tmpID))
			if err != nil || cwd == last {
				continue
			}
			last = cwd
			if t.emitter != nil {
				t.emitter.Emit("session:cwd:"+tmpID, cwd)
			}
		}
	}
}
