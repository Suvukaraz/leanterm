package termview

import (
	"sync"

	"github.com/Suvukaraz/leanterm/internal/events"
	"github.com/Suvukaraz/leanterm/internal/gui/term"
	"github.com/Suvukaraz/leanterm/internal/session"
)

// Session connects a terminal engine to a session of the session manager:
// its output feeds the engine, the engine's input (keys, pastes, answers to
// queries) goes to the session, and its state follows the session's events.
type Session struct {
	TmpID  string
	Engine term.Engine

	mgr     *session.SessionManager
	changed func()
	unsubs  []func()

	mu     sync.Mutex
	title  string
	state  string
	status string
	cwd    string
}

// Attach binds a new engine (""= default engine) to the session tmpID.
// changed is called on any goroutine whenever the terminal needs a redraw.
func Attach(mgr *session.SessionManager, bus *events.Bus, tmpID, engine string, changed func()) (*Session, error) {
	s := &Session{TmpID: tmpID, mgr: mgr, changed: changed, state: "connecting"}
	if info := mgr.GetInfo(tmpID); info != nil {
		s.title = info.Title
	}
	eng, err := term.New(engine, term.Options{Cols: 80, Rows: 24, Callbacks: term.Callbacks{
		Output: func(p []byte) { _ = mgr.WriteBytes(tmpID, p) },
		Title:  func(t string) { s.set(func() { s.title = t }) },
		Dir:    func(d string) { s.set(func() { s.cwd = d }) },
	}})
	if err != nil {
		return nil, err
	}
	s.Engine = eng

	mine := func(data any) (map[string]any, bool) {
		m, ok := data.(map[string]any)
		return m, ok && m["tmpID"] == tmpID
	}
	s.unsubs = append(s.unsubs,
		bus.On(events.SessionState, func(_ string, data any) {
			if m, ok := mine(data); ok {
				st, _ := m["state"].(string)
				s.set(func() { s.state = st })
			}
		}),
		bus.On(events.SessionStatus, func(_ string, data any) {
			if m, ok := mine(data); ok {
				text, _ := m["text"].(string)
				s.set(func() { s.status = text })
			}
		}),
		// The shell's folder as the session reports it (OSC 7, OSC 9;9,
		// /proc), also where the engine does not see it.
		bus.On(events.Session(events.SessionCwdPrefix, tmpID), func(_ string, data any) {
			if d, ok := data.(string); ok {
				s.set(func() { s.cwd = d })
			}
		}),
	)

	// Read the state after subscribing to its changes, so none is missed.
	if sess := mgr.Get(tmpID); sess != nil {
		st := sess.State()
		s.set(func() { s.state = st })
	}

	unsub, err := mgr.Subscribe(tmpID, func(p []byte) {
		eng.Feed(p)
		changed()
	})
	if err != nil {
		s.Close()
		return nil, err
	}
	s.unsubs = append(s.unsubs, unsub)
	return s, nil
}

func (s *Session) set(fn func()) {
	s.mu.Lock()
	fn()
	s.mu.Unlock()
	s.changed()
}

// Resize resizes the session's terminal (the view resizes the engine).
func (s *Session) Resize(cols, rows int) { _ = s.mgr.Resize(s.TmpID, cols, rows) }

// Title is the window title the program set, else the session's title.
func (s *Session) Title() string { s.mu.Lock(); defer s.mu.Unlock(); return s.title }

// State is connecting, connected, disconnected or closed.
func (s *Session) State() string { s.mu.Lock(); defer s.mu.Unlock(); return s.state }

// Status is the last status line message of the session.
func (s *Session) Status() string { s.mu.Lock(); defer s.mu.Unlock(); return s.status }

// Dir is the working directory the shell reported.
func (s *Session) Dir() string { s.mu.Lock(); defer s.mu.Unlock(); return s.cwd }

// Close detaches from the session (it keeps running) and ends the engine.
func (s *Session) Close() {
	for _, u := range s.unsubs {
		u()
	}
	s.unsubs = nil
	if s.Engine != nil {
		s.Engine.Close()
	}
}
