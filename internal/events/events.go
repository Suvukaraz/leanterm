// Package events is the backend's event bus. Sessions, clusters, settings and
// dialogs announce changes through Emit; GUIs subscribe in Go. The web GUI
// (Wails) is one subscriber that forwards every event to the frontend, the
// native GUI subscribes to the events it shows.
//
// Payloads are JSON-compatible values (maps, slices, strings), as the web
// frontend receives them unchanged; the names and payloads are listed below.
package events

import (
	"sort"
	"strings"
	"sync"
)

// Events with a fixed name. Payload keys are given as {key, ...}.
const (
	// AppCloseRequested: nil. The window was asked to close; the GUI
	// confirms open connections and transfers, then quits.
	AppCloseRequested = "app:close-requested"
	// SettingsChanged: nil. The configuration was saved or reloaded.
	SettingsChanged = "settings:changed"
	// FilesDropped: {files []string, attributes, id}. Files dropped onto a
	// window (web GUI).
	FilesDropped = "files:dropped"

	// UIOpenQuickConnect: nil. UIOpenEdit: connection id. UIOpenPreferences:
	// nil. Requests from the command line of a second start.
	UIOpenQuickConnect = "ui:open:quick-connect"
	UIOpenEdit         = "ui:open:edit"
	UIOpenPreferences  = "ui:open:preferences"

	// SessionOpened: {tmpID, title, kind, where, connID, ...}.
	SessionOpened = "session:opened"
	// SessionClosed: {tmpID}.
	SessionClosed = "session:closed"
	// SessionState: {tmpID, state}; state is connecting, connected,
	// disconnected or closed.
	SessionState = "session:state"
	// SessionStatus: {tmpID, code, text}. Status line text (HOSTKEY, ...).
	SessionStatus = "session:status"
	// SessionExpect: {tmpID, index, total, state}. Expect sequence progress.
	SessionExpect = "session:expect"
	// SessionChainStarted / SessionChainFinished: {tmpID, ...}.
	SessionChainStarted  = "session:chain:started"
	SessionChainFinished = "session:chain:finished"
	// SessionTeeStatus: {tmpID, active, path, error}.
	SessionTeeStatus = "session:tee:status"
	// SessionPipeStarted / SessionPipeOutput / SessionPipeStopped: {tmpID, ...}.
	SessionPipeStarted = "session:pipe:started"
	SessionPipeOutput  = "session:pipe:output"
	SessionPipeStopped = "session:pipe:stopped"

	// ClusterChanged: the list of clusters.
	ClusterChanged = "cluster:changed"
	// SFTPFileModified: {sftpID, remotePath, localPath}. A file opened with
	// the default application changed on disk.
	SFTPFileModified = "sftp:file:modified"
	// EditorPending: nil. Files wait to be opened by the editor window.
	EditorPending = "editor:pending"
	// DialogRequest: a prompt the GUI shows (see services.DialogRequest);
	// DialogCancel: {id}.
	DialogRequest = "dialog:request"
	DialogCancel  = "dialog:cancel"
)

// Per-session events: the prefix followed by the session's tmpID.
const (
	// TermDataPrefix: session.StreamPacket {seq, b64}. Terminal output for
	// the web GUI; native GUIs read the session stream directly instead.
	TermDataPrefix = "term:data:"
	// SessionCwdPrefix: string. The working directory the shell reported.
	SessionCwdPrefix = "session:cwd:"
)

// Handler receives an event; it runs on the goroutine that emitted it and
// should return quickly (hand the data over, wake the GUI).
type Handler func(name string, data any)

type subscription struct {
	match  string
	prefix bool
	fn     Handler
}

// Bus delivers events to subscribers. The zero value is ready to use and all
// methods are safe for concurrent use.
type Bus struct {
	mu   sync.RWMutex
	next int
	subs map[int]subscription
}

// New returns an empty bus.
func New() *Bus { return &Bus{} }

// Emit delivers an event to every matching subscriber, in subscription order.
// It implements the Emit(event, data) interfaces of the backend packages.
func (b *Bus) Emit(name string, data any) {
	if b == nil {
		return
	}
	b.mu.RLock()
	ids := make([]int, 0, len(b.subs))
	for id, s := range b.subs {
		if s.match == name || (s.prefix && strings.HasPrefix(name, s.match)) {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	fns := make([]Handler, len(ids))
	for i, id := range ids {
		fns[i] = b.subs[id].fn
	}
	b.mu.RUnlock()
	// Called without the lock, so handlers may subscribe and emit.
	for _, fn := range fns {
		fn(name, data)
	}
}

func (b *Bus) add(s subscription) (unsubscribe func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs == nil {
		b.subs = map[int]subscription{}
	}
	id := b.next
	b.next++
	b.subs[id] = s
	var once sync.Once
	return func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.subs, id)
			b.mu.Unlock()
		})
	}
}

// On subscribes to one event; the returned function unsubscribes.
func (b *Bus) On(name string, fn Handler) (unsubscribe func()) {
	return b.add(subscription{match: name, fn: fn})
}

// OnPrefix subscribes to every event whose name starts with prefix; "" is
// every event.
func (b *Bus) OnPrefix(prefix string, fn Handler) (unsubscribe func()) {
	return b.add(subscription{match: prefix, prefix: true, fn: fn})
}

// Session returns the name of a per-session event (prefix + tmpID).
func Session(prefix, tmpID string) string { return prefix + tmpID }
