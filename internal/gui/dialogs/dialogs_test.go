package dialogs

import (
	"errors"
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/Suvukaraz/leanterm/internal/cluster"
	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/services"
)

func draw(t *testing.T, scheme string, w func(gtx layout.Context, th *gtk.Theme)) {
	t.Helper()
	var r input.Router
	var ops op.Ops
	th := gtk.New(scheme)
	for i := 0; i < 2; i++ {
		ops.Reset()
		gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(1000, 800)}, Source: r.Source(), Now: time.Now()}
		w(gtx, th)
		r.Frame(&ops)
	}
}

func TestQuickConnect(t *testing.T) {
	var got []any
	q := &QuickConnect{Connect: func(title, method, host string, port int, user, pw string) error {
		got = []any{title, method, host, port, user, pw}
		return nil
	}}
	q.Open()
	q.submit()
	if q.err != "Please enter a target host." || got != nil {
		t.Fatalf("empty host: %q %v", q.err, got)
	}
	q.host, q.title, q.user, q.passwd = " srv ", " ", "me", "pw"
	q.submit()
	deadline := time.Now().Add(time.Second)
	for q.open && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		q.mb.drain()
	}
	if q.open || len(got) != 6 || got[0] != "srv" || got[2] != "srv" || got[3] != 22 {
		t.Errorf("connect: open=%v %v", q.open, got)
	}
	for _, s := range []string{"dark", "light"} {
		q.Open()
		q.err = "boom"
		draw(t, s, q.Layout)
	}
}

func TestEditVariablesKeepsTenRows(t *testing.T) {
	var saved services.ConnectionEdit
	e := &EditVariables{
		Load: func(id string) (services.ConnectionEdit, error) {
			c := services.ConnectionEdit{}
			c.Variables = []config.UserVariable{{ID: "abc", Value: "one", Hidden: true}}
			return c, nil
		},
		Save: func(id string, c services.ConnectionEdit) error { saved = c; return nil },
	}
	e.Open("c1")
	if e.rows[0].ID != "abc" || !e.rows[0].Hidden || e.rows[3].ID != "var_3" {
		t.Fatalf("rows: %+v", e.rows)
	}
	e.rows[3].Value = "four"
	draw(t, "dark", e.Layout)
	e.edit.Variables = append([]config.UserVariable(nil), e.rows[:]...)
	if err := e.Save(e.id, e.edit); err != nil || len(saved.Variables) != 10 || saved.Variables[3].Value != "four" {
		t.Errorf("save: %v %+v", err, saved.Variables)
	}
}

func TestBulkEditRules(t *testing.T) {
	var rules []config.BulkEditRule
	applied := false
	b := &BulkEdit{
		Apply: func(ids []string, r []config.BulkEditRule) (config.BulkEditResult, error) {
			rules = r
			return config.BulkEditResult{ModifiedCount: len(ids)}, nil
		},
		Applied: func() { applied = true },
	}
	b.Open([]string{"a", "b"})
	b.applyNow()
	if b.status != "Please select at least one field to change." {
		t.Errorf("no rule: %q", b.status)
	}
	b.rows[1].change, b.rows[1].value, b.rows[1].regex, b.rows[1].match = true, "10.0.$1", true, `^192\.168\.(.*)`
	b.applyNow()
	if len(rules) != 1 || rules[0].Field != "host" || !rules[0].UseRegex || b.status != "Modified 2 connection(s)." || !applied {
		t.Errorf("apply: %+v %q %v", rules, b.status, applied)
	}
	b.Apply = func([]string, []config.BulkEditRule) (config.BulkEditResult, error) {
		return config.BulkEditResult{}, errors.New("bad regex")
	}
	b.applyNow()
	if b.status != "Error applying bulk edit: bad regex" {
		t.Errorf("error: %q", b.status)
	}
	draw(t, "light", b.Layout)
}

type fakeClusters struct {
	m    map[string][]string
	sent []string
}

func (f *fakeClusters) fns() *Clusters {
	return &Clusters{
		List: func() []cluster.ClusterInfo {
			var out []cluster.ClusterInfo
			for n, s := range f.m {
				out = append(out, cluster.ClusterInfo{Name: n, Sessions: s})
			}
			return out
		},
		Sessions: func() []Session {
			return []Session{{TmpID: "t1", Title: "web", Kind: "ssh"}, {TmpID: "t2", Title: "db", Kind: "ssh"}}
		},
		Create: func(n string) error { f.m[n] = nil; return nil },
		Delete: func(n string) { delete(f.m, n) },
		Add:    func(id, n string) { f.m[n] = append(f.m[n], id) },
		Remove: func(id string) {
			for n, s := range f.m {
				for i, x := range s {
					if x == id {
						f.m[n] = append(s[:i:i], s[i+1:]...)
					}
				}
			}
		},
		Send: func(n, text string, o services.PCCOptions) error {
			f.sent = append(f.sent, n+":"+text)
			return nil
		},
	}
}

func TestClusterWindowsDraw(t *testing.T) {
	f := &fakeClusters{m: map[string][]string{"prod": {"t1"}}}
	c := f.fns()
	a := &ClusterAdmin{C: c}
	a.Open()
	if a.selected != "prod" {
		t.Fatalf("selected %q", a.selected)
	}
	for _, s := range []string{"dark", "light"} {
		for tab := 0; tab < 3; tab++ {
			a.tabs.Selected = tab
			draw(t, s, a.Layout)
		}
	}
	p := &PCC{C: c}
	p.Open()
	p.sendText("uptime")
	if len(f.sent) != 1 || f.sent[0] != "prod:uptime" || p.status != "Sent: uptime" {
		t.Errorf("pcc: %v %q", f.sent, p.status)
	}
	p.multi = true
	draw(t, "dark", p.Layout)
	w := &WakeOnLan{}
	w.Open("host", "")
	draw(t, "dark", w.Layout)
}
