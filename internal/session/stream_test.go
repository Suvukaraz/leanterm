package session

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSink struct {
	mu      sync.Mutex
	packets []StreamPacket
}

func (m *mockSink) Send(seq uint64, b64 string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.packets = append(m.packets, StreamPacket{Seq: seq, B64: b64})
}

func (m *mockSink) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.packets)
}

func TestRingBuffer(t *testing.T) {
	rb := NewRingBuffer(10)

	rb.Write([]byte("hello"))
	assert.Equal(t, []byte("hello"), rb.Bytes())

	rb.Write([]byte("world"))
	assert.Equal(t, []byte("helloworld"), rb.Bytes())

	// Wrap around
	rb.Write([]byte("123"))
	assert.Equal(t, []byte("loworld123"), rb.Bytes())

	// Write more than capacity
	rb.Write([]byte("0123456789ABCDEF"))
	assert.Equal(t, []byte("6789ABCDEF"), rb.Bytes())
}

func TestStreamBatching(t *testing.T) {
	sink := &mockSink{}
	stream := NewStream("test-tmp-1", sink)
	defer stream.Close()

	// Write a small chunk < 64KB
	_, err := stream.Write([]byte("Hello World\n"))
	require.NoError(t, err)

	// Before timer fires, sink should have 0 packets
	assert.Equal(t, 0, sink.count())

	// Wait 25ms for the 16ms timer to flush
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, 1, sink.count())

	// Write large chunk >= 64KB -> flushes immediately
	largeData := bytes.Repeat([]byte("A"), 70*1024)
	_, err = stream.Write(largeData)
	require.NoError(t, err)

	assert.Equal(t, 2, sink.count())
}

func TestStreamBackpressureAndAck(t *testing.T) {
	sink := &mockSink{}
	stream := NewStream("test-tmp-2", sink)
	defer stream.Close()

	// Write 1.2 MB without Ack -> will block at 1 MB
	done := make(chan bool)
	go func() {
		chunk := bytes.Repeat([]byte("X"), 64*1024) // 64 KB
		for i := 0; i < 20; i++ {                   // 20 * 64 KB = 1280 KB (> 1 MB)
			_, _ = stream.Write(chunk)
		}
		done <- true
	}()

	select {
	case <-done:
		t.Fatal("expected stream.Write to block due to backpressure")
	case <-time.After(50 * time.Millisecond):
		// Stream blocked as expected
	}

	// Now acknowledge received sequence up to current
	stream.mu.Lock()
	seq := stream.seq
	stream.mu.Unlock()

	stream.Ack(seq)

	// Goroutine should now unblock and complete
	select {
	case <-done:
		// Succeeded
	case <-time.After(500 * time.Millisecond):
		t.Fatal("stream failed to unblock after Ack")
	}
}

func TestStreamAttachReplay(t *testing.T) {
	sink := &mockSink{}
	stream := NewStream("test-tmp-3", sink)
	defer stream.Close()

	_, err := stream.Write([]byte("Initial Greeting\n"))
	require.NoError(t, err)

	replayB64, lastSeq := stream.Attach()
	assert.NotEmpty(t, replayB64)
	assert.GreaterOrEqual(t, lastSeq, uint64(1))
}

func BenchmarkStreamThroughput(t *testing.B) {
	sink := &mockSink{}
	stream := NewStream("bench-stream", sink)
	defer stream.Close()

	chunk := bytes.Repeat([]byte("A"), 32*1024) // 32 KB chunk
	t.SetBytes(int64(len(chunk)))
	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		_, err := stream.Write(chunk)
		if err != nil {
			t.Fatal(err)
		}
		// Acknowledge periodically to avoid blocking benchmark
		if i%10 == 0 {
			stream.Ack(stream.seq)
		}
	}
}

func TestRingBufferWrapKeepsNewestBytes(t *testing.T) {
	r := NewRingBuffer(8)
	r.Write([]byte("abcde"))
	r.Write([]byte("fghij")) // wraps
	assert.Equal(t, "cdefghij", string(r.Bytes()))
	r.Write([]byte("k"))
	assert.Equal(t, "defghijk", string(r.Bytes()))
	r.Write([]byte("0123456789")) // larger than capacity
	assert.Equal(t, "23456789", string(r.Bytes()))
}

func TestStreamListenerFromMarkSeesBacklogOnce(t *testing.T) {
	stream := NewStream("mark", &mockSink{})
	defer stream.Close()
	_, _ = stream.Write([]byte("before|"))
	mark := stream.Mark()
	_, _ = stream.Write([]byte("banner|"))

	var got bytes.Buffer
	id := stream.AddListenerFrom(func(p []byte) { got.Write(p) }, mark)
	defer stream.RemoveListener(id)
	_, _ = stream.Write([]byte("after"))

	assert.Equal(t, "banner|after", got.String())
}
