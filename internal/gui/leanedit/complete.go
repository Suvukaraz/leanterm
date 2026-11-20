package leanedit

import (
	"image"
	"image/color"
	"unicode/utf8"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/oligo/gvcode"

	"github.com/Suvukaraz/leanterm/internal/gui/completion"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/gui/picker"
)

// ---- automatic word completion (KTextEditor's word completion) -----------------------------------
//
// The completion package indexes the document's words in the background
// (without the cursor line, which is counted live without the word being
// typed, so a new word never suggests itself) and ranks words and the
// language's keywords with sahilm/fuzzy. Here the popup opens below the
// word after three typed characters (or on Ctrl+Space), follows the typing
// and takes Up/Down/Tab/Enter/Escape from the editor through gvcode's
// command stack while it is open; the editor keeps the keyboard focus.

const (
	compVisibleRows = 8
	compMaxWidth    = unit.Dp(420)
	compMinWidth    = unit.Dp(180)
)

type compState struct {
	idx  *completion.Indexer
	gen  uint64
	text string // the text of the last frame
	line int    // the cursor line the index leaves out

	open      bool
	manual    bool // opened with Ctrl+Space (Enter accepts)
	navigated bool // Up/Down used (Enter accepts)
	start     int  // where the word starts (rune offset)
	prefix    string
	items     []completion.Item
	sel       int
	first     int // the first visible row
	accepted  string
	request   bool // Ctrl+Space was pressed
	edited    bool // a completion was inserted this frame
	clicks    []widget.Clickable
}

// closeCompletion stops the tab's indexer (the tab closes).
func (t *Tab) closeCompletion() {
	if t.comp.idx != nil {
		t.comp.idx.Close()
		t.comp.idx = nil
	}
}

// CompletionOpen reports whether the completion popup is shown.
func (t *Tab) CompletionOpen() bool { return t.comp.open }

// CompletionItems are the shown completions and the chosen one.
func (t *Tab) CompletionItems() ([]completion.Item, int) { return t.comp.items, t.comp.sel }

// requestCompletion is Ctrl+Space.
func (t *Tab) requestCompletion() { t.comp.request = true }

// syncCompletion runs before the editor is laid out (after its events):
// it keeps the index current and opens, updates or closes the popup.
func (t *Tab) syncCompletion(gtx layout.Context) {
	c := &t.comp
	if !t.ready {
		return
	}
	if c.idx == nil {
		c.idx = completion.NewIndexer(t.invalidate)
	}
	text := t.ed.Text()
	caret, anchor := t.ed.Selection()
	d := t.lineIndex()
	line := d.lineAt(caret).n - 1
	typed := false
	if text != c.text || line != c.line {
		if text != c.text {
			typed = utf8.RuneCountInString(text) == utf8.RuneCountInString(c.text)+1
		}
		c.gen++
		c.idx.Update(c.gen, text, line)
		c.text, c.line = text, line
	}
	if c.accepted != "" && text != c.accepted {
		c.accepted = ""
	}
	request := c.request
	c.request = false

	prefix, start := completion.PrefixAt(d.r, caret)
	n := utf8.RuneCountInString(prefix)
	switch {
	case c.open:
		_, _, ok := t.compAnchor()
		if anchor != caret || !gtx.Focused(t.ed) || start != c.start || prefix == "" ||
			(!c.manual && n < completion.MinWordLen) || !ok {
			t.closeCompletionPopup()
			break
		}
		if prefix != c.prefix || request {
			t.fillCompletion(prefix, start)
			if len(c.items) == 0 {
				t.closeCompletionPopup()
			}
		}
	case request && anchor == caret:
		t.fillCompletion(prefix, start)
		c.open, c.manual, c.navigated = len(c.items) > 0, true, false
	case typed && anchor == caret && n >= completion.MinWordLen && text != c.accepted && caret > 0 && completion.IsWordRune(d.r[caret-1]):
		t.fillCompletion(prefix, start)
		c.open, c.manual, c.navigated = len(c.items) > 0, false, false
	}
	t.registerCompletionKeys()
}

// fillCompletion ranks the candidates for prefix.
func (t *Tab) fillCompletion(prefix string, start int) {
	c := &t.comp
	words := map[string]int(nil)
	d := t.lineIndex()
	caret, _ := t.ed.Selection()
	l := d.lineAt(caret)
	lw := completion.LineWords(d.r[l.from:l.to], caret-l.from)
	if s := c.idx.Snapshot(); s != nil {
		words = s.Words
		// A snapshot taken on another line (the next one is debounced) left
		// that line out: count it from the current text.
		if s.Line != l.n-1 && s.Line >= 0 && s.Line < len(d.lines) {
			completion.CountWords(d.lines[s.Line].text, lw)
		}
	}
	c.items = completion.Complete(prefix, completion.Candidates(words, lw, completion.Keywords(t.LanguageID)), completion.MaxItems)
	c.prefix, c.start = prefix, start
	c.sel, c.first = 0, 0
}

func (t *Tab) closeCompletionPopup() {
	c := &t.comp
	c.open, c.manual, c.navigated = false, false, false
	c.items = nil
	c.prefix = ""
}

// registerCompletionKeys puts the popup's keys on top of gvcode's own
// commands while it is open (re-declared every frame). Enter accepts only
// after Up/Down or Ctrl+Space; otherwise it stays gvcode's line break and
// the popup closes with the word.
func (t *Tab) registerCompletionKeys() {
	c := &t.comp
	tag := &t.comp
	t.ed.RemoveCommands(tag)
	t.ed.RegisterCommand(tag, key.Filter{Name: key.NameSpace, Required: key.ModShortcut},
		func(layout.Context, key.Event) gvcode.EditorEvent {
			t.requestCompletion()
			t.invalidateNow()
			return nil
		})
	if !c.open {
		return
	}
	move := func(dlt int) gvcode.CommandHandler {
		return func(layout.Context, key.Event) gvcode.EditorEvent {
			t.moveCompletion(dlt)
			return nil
		}
	}
	accept := func(layout.Context, key.Event) gvcode.EditorEvent {
		if t.acceptCompletion(t.comp.sel) {
			return gvcode.ChangeEvent{}
		}
		return nil
	}
	t.ed.RegisterCommand(tag, key.Filter{Name: key.NameUpArrow}, move(-1))
	t.ed.RegisterCommand(tag, key.Filter{Name: key.NameDownArrow}, move(1))
	t.ed.RegisterCommand(tag, key.Filter{Name: key.NameTab}, accept)
	t.ed.RegisterCommand(tag, key.Filter{Name: key.NameEscape}, func(layout.Context, key.Event) gvcode.EditorEvent {
		t.closeCompletionPopup()
		t.invalidateNow()
		return nil
	})
	if c.navigated || c.manual {
		t.ed.RegisterCommand(tag, key.Filter{Name: key.NameReturn}, accept)
		t.ed.RegisterCommand(tag, key.Filter{Name: key.NameEnter}, accept)
	}
}

func (t *Tab) invalidateNow() {
	if t.invalidate != nil {
		t.invalidate()
	}
}

// moveCompletion moves the choice (wrapping around).
func (t *Tab) moveCompletion(d int) {
	c := &t.comp
	if n := len(c.items); n > 0 {
		c.sel = ((c.sel+d)%n + n) % n
		c.navigated = true
		if c.sel < c.first {
			c.first = c.sel
		} else if c.sel >= c.first+compVisibleRows {
			c.first = c.sel - compVisibleRows + 1
		}
	}
	t.invalidateNow()
}

// acceptCompletion replaces the word's start up to the caret with item i
// (one undo step); what follows the caret stays.
func (t *Tab) acceptCompletion(i int) bool {
	c := &t.comp
	if i < 0 || i >= len(c.items) {
		return false
	}
	word := c.items[i].Text
	caret, _ := t.ed.Selection()
	start := c.start
	t.closeCompletionPopup()
	if start > caret {
		return false
	}
	t.ed.SetCaret(caret, start)
	t.ed.Insert(word) // replaces the selection: one undoable edit
	c.accepted = t.ed.Text()
	c.edited = true
	t.retoken = true
	t.invalidateNow()
	return true
}

// compAnchor is where the popup hangs: the word start's position in the
// editor (x, baseline) and whether it is in view.
func (t *Tab) compAnchor() (x, y int, ok bool) {
	tv := t.ed.TextView()
	pos := tv.RuneCoords(t.comp.start)
	vp := tv.Viewport()
	if pos.Y < 0 || int(pos.Y) > vp.Dy() {
		return 0, 0, false
	}
	return t.ed.GutterWidth() + int(pos.X), int(pos.Y), true
}

// layoutCompletion draws the popup over the editor (after it is laid out).
func (t *Tab) layoutCompletion(gtx layout.Context, th *gtk.Theme, sh *text.Shaper) {
	c := &t.comp
	if !c.open || len(c.items) == 0 {
		return
	}
	// The editor's shaper has its font (the theme's may not).
	if sh != nil {
		mt := *th.Theme
		mt.Shaper = sh
		th = &gtk.Theme{Theme: &mt, Pal: th.Pal}
	}
	ax, ay, ok := t.compAnchor()
	if !ok {
		return
	}
	pal := th.Pal
	lh := t.ed.TextView().GetLineHeight().Ceil()
	rowH := max(lh, gtx.Dp(20))
	rows := min(len(c.items), compVisibleRows)
	if len(c.clicks) < len(c.items) {
		c.clicks = append(c.clicks, make([]widget.Clickable, len(c.items)-len(c.clicks))...)
	}
	for i := range c.items {
		if c.clicks[i].Clicked(gtx) {
			if t.acceptCompletion(i) {
				c.edited = true
			}
			gtx.Execute(op.InvalidateCmd{})
			return
		}
	}
	// The width: the longest word, within limits.
	w := gtx.Dp(compMinWidth)
	for _, it := range c.items {
		w = max(w, gtx.Sp(unit.Sp(float32(utf8.RuneCountInString(it.Text))*8))+gtx.Dp(72))
	}
	w = min(w, gtx.Dp(compMaxWidth), gtx.Constraints.Max.X)
	h := rows*rowH + 2*gtx.Dp(3)
	// Below the line; above it when there is no room; never past the right edge.
	desc := lh / 3
	top := ay + desc + gtx.Dp(2)
	if top+h > gtx.Constraints.Max.Y && ay-lh-h >= 0 {
		top = ay - lh + desc - h - gtx.Dp(2)
	}
	x := min(ax-gtx.Dp(4), gtx.Constraints.Max.X-w)
	x = max(0, x)
	defer op.Offset(image.Pt(x, top)).Push(gtx.Ops).Pop()
	box := image.Pt(w, h)
	gtk.Box(gtx, box, 4, pal.MenuBg, pal.BorderColor)
	defer clip.Rect{Max: box}.Push(gtx.Ops).Pop()
	pad := gtx.Dp(3)
	for r := 0; r < rows; r++ {
		i := c.first + r
		if i >= len(c.items) {
			break
		}
		it := c.items[i]
		active := i == c.sel
		st := op.Offset(image.Pt(pad, pad+r*rowH)).Push(gtx.Ops)
		g := gtx
		g.Constraints = layout.Exact(image.Pt(w-2*pad, rowH))
		c.clicks[i].Layout(g, func(gtx layout.Context) layout.Dimensions {
			sz := gtx.Constraints.Max
			if active {
				gtk.Box(gtx, sz, 3, pal.SelectedBg, color.NRGBA{})
			}
			fg, accent := pal.TextColor, pal.Accent
			if active {
				fg, accent = pal.SelectedFg, pal.SelectedFg
			}
			layout.Inset{Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = gtx.Constraints.Max
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = 0
						d := picker.Highlight(gtx, th, it.Text, it.Matched, fg, accent, t.typeface)
						d.Size.X = gtx.Constraints.Max.X // the badge goes to the right
						return d
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return picker.Badge(gtx, th, it.Kind.Badge(), active)
					}),
				)
			})
			return layout.Dimensions{Size: sz}
		})
		st.Pop()
	}
	// A thin scroll indicator when there are more rows.
	if len(c.items) > rows {
		th := max(gtx.Dp(8), h*rows/len(c.items))
		ty := (h - th) * c.first / max(1, len(c.items)-rows)
		paint.FillShape(gtx.Ops, pal.Scrollbar, clip.Rect(image.Rect(w-gtx.Dp(3), ty, w-gtx.Dp(1), ty+th)).Op())
	}
}
