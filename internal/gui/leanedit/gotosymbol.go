package leanedit

import (
	"strconv"

	"gioui.org/layout"

	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/gui/picker"
	"github.com/Suvukaraz/leanterm/internal/gui/symbols"
)

// ---- Go to Symbol (Ctrl+Shift+O) ------------------------------------------------------------------
//
// The symbols come from the folding parser's tree (one parse for both);
// languages without a grammar have none. The generic picker filters them
// with sahilm/fuzzy (package symbols) and jumps to the chosen one,
// unfolding the folds around it.

// Symbols are the active document's symbols from its last parse.
func (t *Tab) Symbols() []symbols.Symbol { return t.fold.syms }

func (a *App) setupSymbolPicker() {
	p := &a.symPick
	if p.Items != nil {
		return
	}
	p.Placeholder = "Go to symbol"
	p.Empty = "No symbols available"
	p.NoMatch = "No matching symbol"
	p.Items = func(q string) []picker.Item {
		t := a.Active()
		if t == nil {
			return nil
		}
		ms := symbols.Filter(t.Symbols(), q)
		out := make([]picker.Item, len(ms))
		for i, m := range ms {
			detail := m.Container
			out[i] = picker.Item{Badge: m.Kind.Badge(), Label: m.Name, Matched: m.Matched, Detail: detail,
				Right: strconv.Itoa(m.Line + 1), Value: m.Symbol}
		}
		return out
	}
	p.OnSelect = func(it picker.Item) {
		a.ui_.focusEditor = true
		if t := a.Active(); t != nil {
			if s, ok := it.Value.(symbols.Symbol); ok {
				a.jumpToSymbol(t, s)
			}
		}
	}
	p.OnClose = func() { a.ui_.focusEditor = true }
}

// openGotoSymbol opens the picker with the symbol at or before the cursor
// chosen.
func (a *App) openGotoSymbol() {
	t := a.Active()
	if t == nil {
		return
	}
	a.setupSymbolPicker()
	syms := t.Symbols()
	line := t.lineIndex().lineAt(t.sel().Head).n - 1
	a.symPick.Open(symbols.Nearest(syms, line))
}

// jumpToSymbol moves the caret to the symbol, unfolds the folds that hide
// its line and centers it.
func (a *App) jumpToSymbol(t *Tab, s symbols.Symbol) {
	d := t.lineIndex()
	if s.Line < 0 || s.Line >= len(d.lines) {
		return
	}
	t.fold.m.Reveal(s.Line)
	t.applyFolds()
	a.goToLine(t, s.Line+1)
	l := d.line(s.Line + 1)
	p := l.from + min(s.Col, l.to-l.from)
	t.setSel(Sel{p, p})
	t.fold.caret = p
	t.center = true
}

// centerCaret scrolls the caret line to the middle of the view.
func (t *Tab) centerCaret(gtx layout.Context) {
	tv := t.ed.TextView()
	pos := tv.CaretCoords()
	vp := tv.Viewport()
	tv.ScrollRel(0, int(pos.Y)-vp.Dy()/2)
	if t.invalidate != nil {
		t.invalidate()
	}
}

// symbolPickerLayout draws the picker over the editor area.
func (a *App) symbolPickerLayout(gtx layout.Context, th *gtk.Theme) {
	a.symPick.Layout(gtx, th)
}
