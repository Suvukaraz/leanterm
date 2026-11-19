package dialogs

import (
	"context"
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/Suvukaraz/leanterm/internal/gui/theme"
	"github.com/Suvukaraz/leanterm/internal/session"
)

type harness struct {
	r   input.Router
	ops op.Ops
	th  *theme.Theme
	p   *Prompter
}

func (h *harness) frame() {
	h.ops.Reset()
	gtx := layout.Context{Ops: &h.ops, Constraints: layout.Exact(image.Pt(900, 600)), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Source: h.r.Source(), Now: time.Now()}
	h.p.Layout(gtx, h.th)
	h.r.Frame(&h.ops)
}

type result struct {
	resp session.PromptResponse
	err  error
}

func ask(p *Prompter, ctx context.Context, req session.PromptRequest) chan result {
	ch := make(chan result, 1)
	go func() {
		r, err := p.Prompt(ctx, req)
		ch <- result{r, err}
	}()
	deadline := time.Now().Add(2 * time.Second)
	for !p.Active() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	return ch
}

func wait(t *testing.T, ch chan result) result {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(3 * time.Second):
		t.Fatal("prompt not answered")
	}
	return result{}
}

func TestPasswordTypedAndSubmittedWithEnter(t *testing.T) {
	h := &harness{th: theme.New(), p: NewPrompter(func() {})}
	ch := ask(h.p, context.Background(), session.PromptRequest{
		ID: "1", Kind: "password", Title: "Password", Message: "Password for alice@host",
		Fields: []session.PromptField{{Name: "password", Label: "Password", Secret: true}}, Remember: true,
	})
	h.frame() // shows the dialog and focuses the field
	h.frame()
	h.r.Queue(key.EditEvent{Text: "s3cret"})
	h.frame()
	h.r.Queue(key.Event{Name: key.NameReturn, State: key.Press})
	h.frame()
	r := wait(t, ch)
	if r.err != nil || r.resp.Choice != "ok" || r.resp.Values["password"] != "s3cret" {
		t.Fatalf("answer %+v, %v", r.resp, r.err)
	}
	if h.p.Active() {
		t.Fatal("dialog still shown")
	}
}

func TestEscapeCancelsAndQueueContinues(t *testing.T) {
	h := &harness{th: theme.New(), p: NewPrompter(func() {})}
	first := ask(h.p, context.Background(), session.PromptRequest{ID: "1", Kind: "hostkey-unknown", Message: "Unknown host key", Options: []string{"Accept and save", "Accept once", "Reject"}})
	second := ask(h.p, context.Background(), session.PromptRequest{ID: "2", Kind: "ask", Message: "Pick", Options: []string{"a", "b"}})
	h.frame()
	h.r.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	h.frame()
	if r := wait(t, first); r.err != session.ErrPromptCancelled || !r.resp.Cancelled {
		t.Fatalf("first: %+v %v", r.resp, r.err)
	}
	if !h.p.Active() {
		t.Fatal("second dialog not shown")
	}
	h.frame()
	h.r.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	h.frame()
	if r := wait(t, second); !r.resp.Cancelled {
		t.Fatalf("second: %+v", r.resp)
	}
}

func TestAbandonedPromptDisappears(t *testing.T) {
	h := &harness{th: theme.New(), p: NewPrompter(func() {})}
	ctx, cancel := context.WithCancel(context.Background())
	ch := ask(h.p, ctx, session.PromptRequest{ID: "1", Kind: "password", Message: "pw", Fields: []session.PromptField{{Name: "p", Secret: true}}})
	h.frame()
	cancel()
	if r := wait(t, ch); r.err == nil {
		t.Fatal("no error after cancel")
	}
	h.frame()
	if h.p.Active() {
		t.Fatal("abandoned dialog still shown")
	}
}

func TestEnterPicksTheDefaultHostKeyButton(t *testing.T) {
	h := &harness{th: theme.New(), p: NewPrompter(func() {})}
	opts := []string{"Accept and save", "Accept once", "Reject"}
	ch := ask(h.p, context.Background(), session.PromptRequest{ID: "1", Kind: "hostkey-unknown", Message: "Unknown host key", Options: opts})
	h.frame()
	h.r.Queue(key.Event{Name: key.NameReturn, State: key.Press})
	h.frame()
	if r := wait(t, ch); r.err != nil || r.resp.Choice != "Accept and save" {
		t.Fatalf("answer %+v %v", r.resp, r.err)
	}
	// A changed key has no default: Enter does nothing.
	ch = ask(h.p, context.Background(), session.PromptRequest{ID: "2", Kind: "hostkey-changed", Message: "Changed!", Options: []string{"Replace key and connect", "Cancel"}})
	h.frame()
	h.r.Queue(key.Event{Name: key.NameReturn, State: key.Press})
	h.frame()
	select {
	case r := <-ch:
		t.Fatalf("Enter answered a changed host key: %+v", r.resp)
	case <-time.After(100 * time.Millisecond):
	}
	h.r.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	h.frame()
	wait(t, ch)
}
