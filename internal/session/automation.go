package session

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/vars"
)

// automation runs the per-connection features around a session's lifecycle:
// pre exec before connecting; expect list, session log and periodic string
// sending while connected; post exec and autoreconnect after a disconnect.
//
// It observes the session's own "session:state"/"session:status" events through a
// wrapping emitter and handles them in order on a single goroutine (sessions emit
// while holding their lock, so nothing here may run synchronously).
type automation struct {
	sm      *SessionManager
	tmpID   string
	conn    config.Connection
	global  config.Config
	inner   EventEmitter
	events  chan [2]string // {kind, value}
	stopped chan struct{}

	mu            sync.Mutex
	sess          Session
	runCtx        context.Context // valid while connected
	runCancel     context.CancelFunc
	logger        *SessionLogger
	listenerIDs   []int
	sendString    *SendStringController
	lastState     string
	userStopped   bool
	cleanExit     bool
	reconnects    int
	closed        bool
	connectedMark uint64 // stream offset when the session became connected
}

func newAutomation(sm *SessionManager, tmpID string, conn config.Connection, cfg config.Config) *automation {
	a := &automation{
		sm:      sm,
		tmpID:   tmpID,
		conn:    conn,
		global:  cfg,
		inner:   sm.emitter,
		events:  make(chan [2]string, 64),
		stopped: make(chan struct{}),
	}
	go a.loop()
	return a
}

// Emit implements EventEmitter for the wrapped session.
func (a *automation) Emit(event string, data any) {
	if a.inner != nil {
		a.inner.Emit(event, data)
	}
	m, ok := data.(map[string]any)
	if !ok || m["tmpID"] != a.tmpID {
		return
	}
	switch event {
	case "session:state":
		if st, _ := m["state"].(string); st != "" {
			if st == "connected" {
				// Taken synchronously: the shell output starts after this event, so the
				// expect engine and the log see everything from the first byte on.
				a.mu.Lock()
				if a.sess != nil {
					a.connectedMark = a.sess.Stream().Mark()
				}
				a.mu.Unlock()
			}
			a.enqueue("state", st)
		}
	case "session:status":
		if txt, _ := m["text"].(string); txt != "" {
			a.enqueue("status", txt)
		}
	}
}

func (a *automation) enqueue(kind, value string) {
	select {
	case a.events <- [2]string{kind, value}:
	case <-a.stopped:
	}
}

func (a *automation) attach(s Session) {
	a.mu.Lock()
	a.sess = s
	a.mu.Unlock()
}

func (a *automation) loop() {
	for {
		select {
		case ev := <-a.events:
			switch ev[0] {
			case "state":
				a.onState(ev[1])
			case "status":
				if ev[1] == "DISCONNECTED:remote shell exited" || strings.Contains(ev[1], "exited with status 0") {
					a.mu.Lock()
					a.cleanExit = true
					a.mu.Unlock()
				}
			}
		case <-a.stopped:
			return
		}
	}
}

// userDisconnect marks the next disconnect as intentional (no autoreconnect).
func (a *automation) userDisconnect() {
	a.mu.Lock()
	a.userStopped = true
	a.mu.Unlock()
}

// userInput is called for keystrokes; it also re-enables autoreconnect after a
// manual reconnect (Enter in a disconnected terminal).
func (a *automation) userInput() {
	a.mu.Lock()
	ss := a.sendString
	if a.lastState == "disconnected" {
		a.userStopped = false
		a.reconnects = 0
	}
	a.mu.Unlock()
	if ss != nil {
		ss.MarkActive()
	}
}

func (a *automation) close() {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	a.closed = true
	wasConnected := a.lastState == "connected"
	a.mu.Unlock()

	a.stopConnectedFeatures()
	close(a.stopped)
	if wasConnected {
		go a.runPrePost("POST_EXEC", a.conn.PostExec)
	}
}

func (a *automation) onState(st string) {
	a.mu.Lock()
	prev := a.lastState
	a.lastState = st
	closed := a.closed
	a.mu.Unlock()
	if closed {
		return
	}

	switch st {
	case "connected":
		a.mu.Lock()
		a.cleanExit = false
		a.reconnects = 0
		a.mu.Unlock()
		a.startConnectedFeatures()
	case "disconnected":
		a.stopConnectedFeatures()
		if prev != "connected" {
			return // failed connection attempt: nothing ran, no reconnect loop on auth errors
		}
		go a.runPrePost("POST_EXEC", a.conn.PostExec)
		a.maybeReconnect()
	case "closed":
		a.stopConnectedFeatures()
	}
}

func (a *automation) substCtx() vars.SubstContext {
	return a.sm.buildSubstContext(a.conn.ID)
}

func (a *automation) lineEnding() string {
	if a.conn.Method == "telnet" {
		switch a.conn.Telnet.LineEnding {
		case "crnul":
			return "\r\x00"
		case "lf":
			return "\n"
		default:
			return "\r\n"
		}
	}
	return "\n"
}

func (a *automation) status(text string) {
	if a.inner != nil {
		a.inner.Emit("session:status", map[string]any{"tmpID": a.tmpID, "text": text, "code": strings.SplitN(text, ":", 2)[0]})
	}
}

func (a *automation) startConnectedFeatures() {
	a.mu.Lock()
	sess := a.sess
	if sess == nil || a.runCancel != nil {
		a.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.runCtx, a.runCancel = ctx, cancel
	mark := a.connectedMark
	a.mu.Unlock()

	stream := sess.Stream()
	sctx := a.substCtx()

	// Session log: the connection's own settings, else the global ones.
	logCfg := a.conn.SessionLogs
	if !logCfg.Enabled {
		logCfg = a.global.Terminal.SessionLogs
	}
	if logCfg.Enabled {
		defaultDir := ""
		if a.sm.store != nil {
			defaultDir = filepath.Join(a.sm.store.ConfigDir(), "session_logs")
		}
		if logger, err := NewSessionLogger(logCfg, sctx, defaultDir); err == nil && logger != nil {
			id := stream.AddListenerFrom(logger.Write, mark)
			a.mu.Lock()
			a.logger = logger
			a.listenerIDs = append(a.listenerIDs, id)
			a.mu.Unlock()
		} else if err != nil {
			a.status("LOG:cannot write session log: " + err.Error())
		}
	}

	// Periodic string sending.
	if a.conn.SendString.Enabled {
		ss := NewSendStringController(a.conn.SendString, sctx, sess.Write, a.lineEnding(), a.conn.SendDelayMs)
		id := stream.AddListener(func([]byte) { ss.MarkActive() })
		a.mu.Lock()
		a.sendString = ss
		a.listenerIDs = append(a.listenerIDs, id)
		a.mu.Unlock()
		ss.Start(ctx)
	}

	// Expect list.
	var entries []config.ExpectEntry
	for _, e := range a.conn.Expect {
		if e.Enabled {
			entries = append(entries, e)
		}
	}
	if len(entries) > 0 {
		timeout := a.conn.Terminal.TimeoutCommand
		if !a.conn.Terminal.UsePersonal || timeout <= 0 {
			timeout = a.global.Terminal.TimeoutCommand
		}
		runner := NewExpectRunner(ExpectOptions{
			TmpID:          a.tmpID,
			ConnID:         a.conn.ID,
			Entries:        entries,
			DefaultTimeout: timeout,
			SendDelayMs:    a.conn.SendDelayMs,
			LineEnding:     a.lineEnding(),
			SubstCtx:       sctx,
			Vault:          a.sm.vault,
			SendFunc:       sess.Write,
			StatusFunc:     a.status,
			ExpectProgFunc: func(index, total int, state string) {
				if a.inner != nil {
					a.inner.Emit("session:expect", map[string]any{"tmpID": a.tmpID, "index": index, "total": total, "state": state})
				}
			},
			CloseFunc: func(reason string) {
				a.status("CLOSE:" + reason)
				go func() { _ = sess.Disconnect() }()
			},
		})
		id := stream.AddListenerFrom(runner.Feed, mark)
		a.mu.Lock()
		a.listenerIDs = append(a.listenerIDs, id)
		a.mu.Unlock()
		go func() {
			runner.Run(ctx)
			runner.Close()
		}()
	}
}

func (a *automation) stopConnectedFeatures() {
	a.mu.Lock()
	cancel := a.runCancel
	a.runCancel, a.runCtx = nil, nil
	logger := a.logger
	a.logger = nil
	ids := a.listenerIDs
	a.listenerIDs = nil
	ss := a.sendString
	a.sendString = nil
	sess := a.sess
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if ss != nil {
		ss.Stop()
	}
	if sess != nil {
		for _, id := range ids {
			sess.Stream().RemoveListener(id)
		}
	}
	if logger != nil {
		_ = logger.Close()
	}
}

// runPrePost executes pre/post exec commands; the user is asked when entries want it.
func (a *automation) runPrePost(phase string, entries []config.PrePostExec) {
	if len(entries) == 0 {
		return
	}
	a.mu.Lock()
	sess := a.sess
	a.mu.Unlock()
	var stream *Stream
	if sess != nil {
		stream = sess.Stream()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	_ = ExecutePrePost(ctx, PrePostOptions{
		Phase:      phase,
		Entries:    entries,
		SubstCtx:   a.substCtx(),
		AskHandler: a.askPrePost,
		Stream:     stream,
		StatusFunc: a.status,
	})
}

func (a *automation) askPrePost(phase string, entries []config.PrePostExec) ([]config.PrePostExec, bool) {
	var lines []string
	var selected []config.PrePostExec
	for _, e := range entries {
		if !e.Default {
			continue
		}
		selected = append(selected, e)
		lines = append(lines, "  "+e.Command)
	}
	if len(selected) == 0 {
		return nil, true
	}
	what := "before connecting"
	if phase == "POST_EXEC" {
		what = "after disconnecting"
	}
	resp, err := ask(context.Background(), a.sm.prompter, PromptRequest{
		TmpID:   a.tmpID,
		Kind:    "confirm",
		Title:   "Local commands",
		Message: fmt.Sprintf("Execute these local commands %s '%s'?\n\n%s", what, a.conn.Title, strings.Join(lines, "\n")),
		Options: []string{"Execute", "Skip"},
	})
	if err != nil || resp.Choice != "Execute" {
		return nil, true
	}
	return selected, false
}

func (a *automation) maybeReconnect() {
	a.mu.Lock()
	enabled := a.conn.AutoReconnect && !a.userStopped && !a.cleanExit && !a.closed
	a.reconnects++
	attempt := a.reconnects
	sess := a.sess
	a.mu.Unlock()

	max := a.global.Terminal.MaxReconnectAttempts
	if !enabled || sess == nil || (max > 0 && attempt > max) {
		return
	}
	go func() {
		wait := BackoffDuration(attempt)
		_, _ = sess.Stream().Write([]byte(fmt.Sprintf("\x1b[31mRECONNECTING in %ds ... attempt:%d\x1b[0m\r\n", int(wait.Seconds()), attempt)))
		select {
		case <-time.After(wait):
		case <-a.stopped:
			return
		}
		a.mu.Lock()
		stop := a.userStopped || a.closed || a.lastState != "disconnected"
		a.mu.Unlock()
		if !stop {
			_ = sess.Start()
		}
	}()
}
