package session

import (
	"encoding/base64"
	"sync"
	"sync/atomic"
	"time"
)

const (
	RingBufferSize = 2 * 1024 * 1024 // 2 MiB
	BatchSize      = 64 * 1024       // 64 KiB
	BatchInterval  = 16 * time.Millisecond
	HighWatermark  = 1024 * 1024 // 1 MiB unacknowledged
	LowWatermark   = 256 * 1024  // 256 KiB resume
)

// Sink defines the interface for emitting terminal data events to the frontend.
type Sink interface {
	Send(seq uint64, b64 string)
}

// EventPayload represents the stream packet delivered to the frontend.
type StreamPacket struct {
	Seq uint64 `json:"seq"`
	B64 string `json:"b64"`
}

// RingBuffer stores the last 2 MiB of terminal output.
type RingBuffer struct {
	mu    sync.RWMutex
	buf   []byte
	start int
	size  int
}

func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		buf: make([]byte, capacity),
	}
}

func (r *RingBuffer) Write(p []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	n := len(p)
	cap := len(r.buf)
	if n >= cap {
		// If data is larger than buffer, keep only the trailing cap bytes
		copy(r.buf, p[n-cap:])
		r.start = 0
		r.size = cap
		return
	}

	// Copy in at most two contiguous blocks instead of byte by byte.
	end := (r.start + r.size) % cap
	first := copy(r.buf[end:], p)
	if first < n {
		copy(r.buf, p[first:])
	}
	r.size += n
	if r.size > cap {
		r.start = (r.start + (r.size - cap)) % cap
		r.size = cap
	}
}

func (r *RingBuffer) Bytes() []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]byte, r.size)
	cap := len(r.buf)
	for i := 0; i < r.size; i++ {
		out[i] = r.buf[(r.start+i)%cap]
	}
	return out
}

// Stream manages the terminal stream batching, sequencing, and flow control.
type Stream struct {
	tmpID       string
	ring        *RingBuffer
	sink        Sink
	seq         uint64
	mu          sync.Mutex
	cond        *sync.Cond
	pending     []byte
	flushTimer  *time.Timer
	timerActive bool

	unackBytes int64
	attached   bool
	closed     bool

	// Track bytes per seq for ack calculation
	seqBytes map[uint64]int

	listenersMu    sync.RWMutex
	listeners      map[int]*streamListener
	nextListenerID int

	written uint64 // total bytes written to the ring (guarded by mu)
}

type streamListener struct {
	fn     func([]byte)
	minEnd uint64 // only chunks ending after this offset are delivered
}

func NewStream(tmpID string, sink Sink) *Stream {
	s := &Stream{
		tmpID:     tmpID,
		ring:      NewRingBuffer(RingBufferSize),
		sink:      sink,
		seqBytes:  make(map[uint64]int),
		attached:  true,
		listeners: make(map[int]*streamListener),
	}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// AddListener registers a callback that receives incoming stream bytes.
func (s *Stream) AddListener(fn func([]byte)) int {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	s.mu.Lock()
	end := s.written
	s.mu.Unlock()
	id := s.nextListenerID
	s.nextListenerID++
	s.listeners[id] = &streamListener{fn: fn, minEnd: end}
	return id
}

// Mark returns the current output offset (see AddListenerFrom).
func (s *Stream) Mark() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.written
}

// AddListenerFrom registers a listener and first replays everything written since
// mark (as far as the ring buffer still holds it). No chunk is delivered twice or lost.
func (s *Stream) AddListenerFrom(fn func([]byte), mark uint64) int {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()

	s.mu.Lock()
	end := s.written
	var backlog []byte
	if end > mark {
		all := s.ring.Bytes()
		n := end - mark
		if n > uint64(len(all)) {
			n = uint64(len(all))
		}
		backlog = all[len(all)-int(n):]
	}
	s.mu.Unlock()

	id := s.nextListenerID
	s.nextListenerID++
	s.listeners[id] = &streamListener{fn: fn, minEnd: end}
	if len(backlog) > 0 {
		fn(backlog)
	}
	return id
}

// RemoveListener unregisters a listener.
func (s *Stream) RemoveListener(id int) {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	delete(s.listeners, id)
}

// notifyListeners invokes registered stream listeners for a chunk ending at end.
func (s *Stream) notifyListeners(p []byte, end uint64) {
	s.listenersMu.RLock()
	defer s.listenersMu.RUnlock()
	for _, l := range s.listeners {
		if end > l.minEnd {
			l.fn(p)
		}
	}
}

// Write receives raw bytes from the session (SSH channel, Telnet, or PTY Master).
func (s *Stream) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, nil
	}

	// Flow control backpressure if attached
	for s.attached && !s.closed && s.unackBytes >= HighWatermark {
		s.cond.Wait()
	}

	if s.closed {
		return 0, nil
	}

	// Record to the ring buffer under s.mu so an Attach() snapshot and the
	// sequence numbers always agree (no duplicated or lost output on replay).
	s.ring.Write(p)
	s.written += uint64(len(p))
	end := s.written
	s.pending = append(s.pending, p...)

	// Notify listeners (expect engine, loggers, idle timers) outside s.mu.
	s.mu.Unlock()
	s.notifyListeners(p, end)
	s.mu.Lock()
	if s.closed {
		return len(p), nil
	}

	// If pending batch reached 64 KiB, flush immediately
	if len(s.pending) >= BatchSize {
		s.flushLocked()
	} else if !s.timerActive && s.attached {
		s.timerActive = true
		if s.flushTimer == nil {
			s.flushTimer = time.AfterFunc(BatchInterval, func() {
				s.mu.Lock()
				defer s.mu.Unlock()
				s.timerActive = false
				if !s.closed && len(s.pending) > 0 {
					s.flushLocked()
				}
			})
		} else {
			s.flushTimer.Reset(BatchInterval)
		}
	}

	return len(p), nil
}

func (s *Stream) flushLocked() {
	if len(s.pending) == 0 || !s.attached || s.closed {
		return
	}

	currentSeq := atomic.AddUint64(&s.seq, 1)
	chunkLen := len(s.pending)
	b64 := base64.StdEncoding.EncodeToString(s.pending)

	s.seqBytes[currentSeq] = chunkLen
	s.unackBytes += int64(chunkLen)
	s.pending = nil

	if s.sink != nil {
		s.sink.Send(currentSeq, b64)
	}
}

// Ack handles client acknowledgment of received output up to seq.
func (s *Stream) Ack(seq uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var acknowledgedBytes int64
	for sSeq, bLen := range s.seqBytes {
		if sSeq <= seq {
			acknowledgedBytes += int64(bLen)
			delete(s.seqBytes, sSeq)
		}
	}

	if acknowledgedBytes > 0 {
		s.unackBytes -= acknowledgedBytes
		if s.unackBytes < 0 {
			s.unackBytes = 0
		}
		if s.unackBytes <= LowWatermark {
			s.cond.Broadcast()
		}
	}
}

// Attach returns the full ring buffer content and last sequence for initial replay.
func (s *Stream) Attach() (replayB64 string, lastSeq uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.attached = true
	// Flush any pending
	s.flushLocked()

	replayBytes := s.ring.Bytes()
	replayB64 = base64.StdEncoding.EncodeToString(replayBytes)
	lastSeq = atomic.LoadUint64(&s.seq)
	return replayB64, lastSeq
}

// Detach pauses event emission and resets backpressure so background reads don't block.
func (s *Stream) Detach() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.attached = false
	s.unackBytes = 0
	s.seqBytes = make(map[uint64]int)
	s.cond.Broadcast()
}

// Close terminates the stream and releases waiting goroutines.
func (s *Stream) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	if s.flushTimer != nil {
		s.flushTimer.Stop()
	}
	s.cond.Broadcast()
}
