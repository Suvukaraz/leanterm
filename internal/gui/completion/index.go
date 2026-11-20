package completion

import (
	"sync"
	"time"
)

// Snapshot is the word index of one text: the words of every line except
// Line (the cursor line when it was taken).
type Snapshot struct {
	Gen   uint64
	Line  int
	Words map[string]int
}

// Indexer keeps a document's word index up to date in the background.
// Updates in quick succession (typing) are coalesced; the worker indexes
// the latest text only. A snapshot is never changed after it is published,
// so the editor can read it while the next one is built.
type Indexer struct {
	delay  time.Duration
	notify func()

	mu      sync.Mutex
	pending *request
	snap    *Snapshot
	closed  bool

	wake chan struct{}
	quit chan struct{}
	done chan struct{}
}

type request struct {
	gen  uint64
	text string
	line int
}

// NewIndexer starts an indexer; notify (if set) is called from the worker
// when a new snapshot is ready.
func NewIndexer(notify func()) *Indexer {
	x := &Indexer{delay: 200 * time.Millisecond, notify: notify,
		wake: make(chan struct{}, 1), quit: make(chan struct{}), done: make(chan struct{})}
	go x.run()
	return x
}

// Update asks for an index of text (generation gen) without line.
func (x *Indexer) Update(gen uint64, text string, line int) {
	x.mu.Lock()
	if x.closed {
		x.mu.Unlock()
		return
	}
	x.pending = &request{gen: gen, text: text, line: line}
	x.mu.Unlock()
	select {
	case x.wake <- struct{}{}:
	default:
	}
}

// Snapshot is the latest index (nil before the first one).
func (x *Indexer) Snapshot() *Snapshot {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.snap
}

// Close stops the worker.
func (x *Indexer) Close() {
	x.mu.Lock()
	if x.closed {
		x.mu.Unlock()
		return
	}
	x.closed = true
	x.mu.Unlock()
	close(x.quit)
	<-x.done
}

func (x *Indexer) run() {
	defer close(x.done)
	first := true
	for {
		select {
		case <-x.quit:
			return
		case <-x.wake:
		}
		if !first {
			select {
			case <-x.quit:
				return
			case <-time.After(x.delay):
			}
		}
		first = false
		x.mu.Lock()
		req := x.pending
		x.pending = nil
		x.mu.Unlock()
		if req == nil {
			continue
		}
		snap := &Snapshot{Gen: req.gen, Line: req.line, Words: IndexWords(req.text, req.line)}
		x.mu.Lock()
		if x.closed {
			x.mu.Unlock()
			return
		}
		if x.snap == nil || x.snap.Gen <= snap.Gen {
			x.snap = snap
		}
		x.mu.Unlock()
		if x.notify != nil {
			x.notify()
		}
	}
}
