package events

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestOnAndPrefix(t *testing.T) {
	b := New()
	var got []string
	rec := func(tag string) Handler {
		return func(name string, data any) { got = append(got, fmt.Sprintf("%s:%s=%v", tag, name, data)) }
	}
	b.On(SessionClosed, rec("one"))
	b.OnPrefix(SessionCwdPrefix, rec("cwd"))
	all := b.OnPrefix("", rec("all"))

	b.Emit(SessionClosed, "x")
	b.Emit(Session(SessionCwdPrefix, "t1"), "/srv")
	b.Emit("other", 1)
	all()
	b.Emit(SessionClosed, "y")

	want := []string{
		"one:session:closed=x", "all:session:closed=x",
		"cwd:session:cwd:t1=/srv", "all:session:cwd:t1=/srv",
		"all:other=1",
		"one:session:closed=y",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
}

func TestHandlersMayUseTheBus(t *testing.T) {
	var b Bus // zero value
	var n int
	var unsub func()
	unsub = b.On("a", func(string, any) {
		n++
		unsub()          // unsubscribe from inside the handler
		b.Emit("b", nil) // and emit
		b.On("c", func(string, any) {})
	})
	b.On("b", func(string, any) { n += 10 })
	b.Emit("a", nil)
	b.Emit("a", nil)
	if n != 11 {
		t.Fatalf("n = %d", n)
	}
}

func TestConcurrentUse(t *testing.T) {
	b := New()
	var mu sync.Mutex
	count := 0
	b.OnPrefix("e", func(string, any) { mu.Lock(); count++; mu.Unlock() })
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				b.Emit("e", j)
				u := b.On("x", func(string, any) {})
				u()
			}
		}()
	}
	wg.Wait()
	if count != 800 {
		t.Fatalf("count = %d", count)
	}
}

func TestNilBus(t *testing.T) {
	var b *Bus
	b.Emit("x", nil) // no panic
}
