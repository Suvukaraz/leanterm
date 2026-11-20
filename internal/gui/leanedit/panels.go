package leanedit

import (
	"fmt"
	"image"
	"image/color"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	gvcolor "github.com/oligo/gvcode/color"
	"github.com/oligo/gvcode/textstyle/decoration"

	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

// ---- search panel (CodeMirror's search panel, at the top) --------------------------------------------

type searchPanel struct {
	open       bool
	find, repl widget.Editor
	matchCase  widget.Bool
	regex      widget.Bool
	byWord     widget.Bool
	next, prev widget.Clickable
	all, close widget.Clickable
	replB      widget.Clickable
	replAll    widget.Clickable
	focusFind  bool
	query      string // the query the matches were computed for
	doc        string
	tab        int
	matches    []Sel
	invalid    bool
	wasCase    bool
	wasRegex   bool
	wasWord    bool
}

const searchSource = "leanedit-search"

func (a *App) openSearch(gtx layout.Context) {
	t := a.Active()
	if t == nil || !t.ready {
		return
	}
	sp := &a.search
	sp.open, sp.focusFind = true, true
	if s := t.ed.SelectedText(); s != "" && !strings.Contains(s, "\n") {
		sp.find.SetText(s)
	}
	sp.find.SetCaret(0, sp.find.Len())
}

func (a *App) closeSearch() {
	sp := &a.search
	sp.open = false
	if t := a.tab(sp.tab); t != nil {
		t.ed.ClearDecorations(searchSource)
	}
	sp.matches, sp.query = nil, ""
	a.ui_.focusEditor = true
}

// searchRegexp builds the matcher of the query (nil: none or invalid).
func (sp *searchPanel) searchRegexp() *regexp.Regexp {
	q := sp.find.Text()
	if q == "" {
		return nil
	}
	pat := q
	if !sp.regex.Value {
		pat = regexp.QuoteMeta(q)
	}
	if sp.byWord.Value {
		pat = `\b(?:` + pat + `)\b`
	}
	if !sp.matchCase.Value {
		pat = "(?i)" + pat
	}
	re, err := regexp.Compile("(?m)" + pat)
	if err != nil {
		sp.invalid = true
		return nil
	}
	sp.invalid = false
	return re
}

// refresh recomputes the matches when the query, options or text changed.
func (a *App) refreshSearch(t *Tab) {
	sp := &a.search
	text := t.ed.Text()
	q := sp.find.Text()
	if q == sp.query && text == sp.doc && t.ID == sp.tab && sp.wasCase == sp.matchCase.Value && sp.wasRegex == sp.regex.Value && sp.wasWord == sp.byWord.Value {
		return
	}
	if old := a.tab(sp.tab); old != nil {
		old.ed.ClearDecorations(searchSource)
	}
	sp.query, sp.doc, sp.tab = q, text, t.ID
	sp.wasCase, sp.wasRegex, sp.wasWord = sp.matchCase.Value, sp.regex.Value, sp.byWord.Value
	sp.matches = nil
	re := sp.searchRegexp()
	if re == nil {
		return
	}
	// Byte offsets to rune offsets.
	idx := re.FindAllStringIndex(text, 5000)
	if len(idx) == 0 {
		return
	}
	runeAt := runeOffsets(text)
	var decos []decoration.Decoration
	bg := &decoration.Background{Color: gvcolor.MakeColor(a.ui_.th.Pal.EditorSearchMatch)}
	for _, m := range idx {
		if m[0] == m[1] {
			continue
		}
		s := Sel{runeAt(m[0]), runeAt(m[1])}
		sp.matches = append(sp.matches, s)
		decos = append(decos, decoration.Decoration{Source: searchSource, Start: s.Anchor, End: s.Head, Background: bg})
	}
	_ = t.ed.AddDecorations(decos...)
}

func runeOffsets(s string) func(int) int {
	// Byte offset → rune offset, walking forward (offsets come sorted).
	pos, runes := 0, 0
	return func(b int) int {
		if b < pos {
			pos, runes = 0, 0
		}
		for pos < b && pos < len(s) {
			_, n := decodeRune(s[pos:])
			pos += n
			runes++
		}
		return runes
	}
}

func decodeRune(s string) (rune, int) {
	for i, r := range s {
		if i == 0 {
			n := len(string(r))
			return r, n
		}
	}
	return 0, 1
}

// findNext selects the next (dir 1) or previous match after the cursor.
func (a *App) findNext(dir int) {
	t := a.Active()
	if t == nil || !t.ready {
		return
	}
	sp := &a.search
	a.refreshSearch(t)
	if len(sp.matches) == 0 {
		return
	}
	s := t.sel()
	var pick Sel
	if dir > 0 {
		i := sort.Search(len(sp.matches), func(i int) bool { return sp.matches[i].Anchor >= s.to() })
		if i == len(sp.matches) {
			i = 0
		}
		pick = sp.matches[i]
	} else {
		i := sort.Search(len(sp.matches), func(i int) bool { return sp.matches[i].Anchor >= s.from() }) - 1
		if i < 0 {
			i = len(sp.matches) - 1
		}
		pick = sp.matches[i]
	}
	from := a.currentLocation()
	t.setSel(Sel{pick.Anchor, pick.Head})
	a.recordJump(from)
}

func (a *App) selectAllMatches() {
	// One cursor: select the whole range from the first to the last match.
	t := a.Active()
	if t == nil || len(a.search.matches) == 0 {
		return
	}
	m := a.search.matches
	t.setSel(Sel{m[0].Anchor, m[len(m)-1].Head})
}

func (a *App) replaceOne() {
	t := a.Active()
	if t == nil {
		return
	}
	sp := &a.search
	a.refreshSearch(t)
	s := t.sel()
	for _, m := range sp.matches {
		if m.Anchor == s.from() && m.Head == s.to() {
			rep := sp.replacement(t.ed.ReadTextBetween(m.Anchor, m.Head))
			end := m.Anchor + len([]rune(rep))
			a.applyEdit(t, []Change{{m.Anchor, m.Head, rep}}, &Sel{end, end})
			a.refreshSearch(t)
			a.findNext(1)
			return
		}
	}
	a.findNext(1)
}

func (a *App) replaceAll() {
	t := a.Active()
	if t == nil {
		return
	}
	sp := &a.search
	a.refreshSearch(t)
	var ch []Change
	for _, m := range sp.matches {
		ch = append(ch, Change{m.Anchor, m.Head, sp.replacement(t.ed.ReadTextBetween(m.Anchor, m.Head))})
	}
	a.applyEdit(t, ch, nil)
	a.refreshSearch(t)
}

// replacement is the text for a match ($1 etc. with a regular expression).
func (sp *searchPanel) replacement(match string) string {
	r := sp.repl.Text()
	if !sp.regex.Value {
		return r
	}
	re := sp.searchRegexp()
	if re == nil {
		return r
	}
	// $& is the whole match in CodeMirror; Go uses ${0}.
	r = strings.ReplaceAll(r, "$&", "${0}")
	return re.ReplaceAllString(match, r)
}

func (a *App) searchPanelLayout(gtx layout.Context, th *gtk.Theme, t *Tab) layout.Dimensions {
	sp := &a.search
	pal := th.Pal
	// Keys of the find field: Enter next, Shift+Enter previous, Escape closes.
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &sp.find, Name: key.NameReturn, Optional: key.ModShift},
			key.Filter{Focus: &sp.find, Name: key.NameEnter, Optional: key.ModShift},
			key.Filter{Focus: &sp.find, Name: key.NameEscape},
			key.Filter{Focus: &sp.repl, Name: key.NameReturn},
			key.Filter{Focus: &sp.repl, Name: key.NameEnter},
			key.Filter{Focus: &sp.repl, Name: key.NameEscape},
		)
		if !ok {
			break
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		switch {
		case e.Name == key.NameEscape:
			a.closeSearch()
			return layout.Dimensions{}
		case gtx.Focused(&sp.repl):
			a.replaceOne()
		case e.Modifiers.Contain(key.ModShift):
			a.findNext(-1)
		default:
			a.findNext(1)
		}
	}
	if sp.next.Clicked(gtx) {
		a.findNext(1)
	}
	if sp.prev.Clicked(gtx) {
		a.findNext(-1)
	}
	if sp.all.Clicked(gtx) {
		a.selectAllMatches()
	}
	if sp.replB.Clicked(gtx) {
		a.replaceOne()
	}
	if sp.replAll.Clicked(gtx) {
		a.replaceAll()
	}
	if sp.close.Clicked(gtx) {
		a.closeSearch()
		return layout.Dimensions{}
	}
	sp.find.SingleLine, sp.repl.SingleLine = true, true
	a.refreshSearch(t)
	if sp.focusFind {
		sp.focusFind = false
		gtx.Execute(key.FocusCmd{Tag: &sp.find})
	}
	btn := func(c *widget.Clickable, label string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions { return gtk.Button{Label: label}.Layout(gtx, th, c) }
	}
	check := func(b *widget.Bool, label string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions { return th.CheckButton(gtx, b, label, false) }
	}
	entry := func(e *widget.Editor, ph string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return gtk.Entry{Placeholder: ph, Width: 200}.Layout(gtx, th, e)
		}
	}
	row := func(kids ...layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			var fl []layout.FlexChild
			for i, k := range kids {
				if i > 0 {
					fl = append(fl, layout.Rigid(layout.Spacer{Width: 4}.Layout))
				}
				fl = append(fl, layout.Rigid(k))
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, fl...)
		}
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	m := op.Record(gtx.Ops)
	d := layout.Inset{Top: 4, Bottom: 4, Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		count := ""
		switch {
		case sp.invalid:
			count = "invalid pattern"
		case sp.find.Text() != "":
			count = fmt.Sprintf("%d matches", len(sp.matches))
		}
		return layout.Flex{}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(row(entry(&sp.find, "Find"), btn(&sp.next, "next"), btn(&sp.prev, "previous"), btn(&sp.all, "all"),
						check(&sp.matchCase, "match case"), check(&sp.regex, "regexp"), check(&sp.byWord, "by word"),
						gtk.Text{S: count, Size: gtk.Em(0.9), Color: pal.MutedFg}.Widget(th))),
					layout.Rigid(layout.Spacer{Height: 4}.Layout),
					layout.Rigid(row(entry(&sp.repl, "Replace"), btn(&sp.replB, "replace"), btn(&sp.replAll, "replace all"))),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return sp.close.Layout(gtx, gtk.Text{S: "×", Size: gtk.Em(1.3), Color: pal.FgColor}.Widget(th))
			}),
		)
	})
	call := m.Stop()
	d.Size.X = gtx.Constraints.Max.X
	gtk.FillRect(gtx, image.Rectangle{Max: d.Size}, pal.HeaderBg)
	gtk.FillRect(gtx, image.Rect(0, d.Size.Y-gtx.Dp(1), d.Size.X, d.Size.Y), pal.BorderColor)
	call.Add(gtx.Ops)
	return d
}

// ---- go to line (CodeMirror's gotoLine panel) --------------------------------------------------------

type gotoPanel struct {
	open  bool
	ed    widget.Editor
	go_   widget.Clickable
	focus bool
}

func (a *App) openGotoLine() {
	if t := a.Active(); t == nil || !t.ready {
		return
	}
	a.gotoLn.open, a.gotoLn.focus = true, true
	a.gotoLn.ed.SetText("")
}

// parseGoto reads "line", "line:col", "+n", "-n" or "n%" (CodeMirror's syntax).
func parseGoto(s string, cur, lines int) (line, col int, ok bool) {
	s = strings.TrimSpace(s)
	m := regexp.MustCompile(`^([+-])?(\d+)?(:\d+)?(%)?$`).FindStringSubmatch(s)
	if m == nil || (m[2] == "" && m[3] == "") {
		return 0, 0, false
	}
	line = cur
	if m[2] != "" {
		n, _ := strconv.Atoi(m[2])
		switch {
		case m[4] != "":
			line = int(float64(n)/100*float64(lines) + 0.5)
			if m[1] != "" {
				line = cur + map[string]int{"+": 1, "-": -1}[m[1]]*int(float64(n)/100*float64(lines)+0.5)
			}
		case m[1] == "+":
			line = cur + n
		case m[1] == "-":
			line = cur - n
		default:
			line = n
		}
	}
	if m[3] != "" {
		col, _ = strconv.Atoi(m[3][1:])
	}
	return max(1, min(lines, line)), col, true
}

func (a *App) gotoPanelLayout(gtx layout.Context, th *gtk.Theme, t *Tab) layout.Dimensions {
	g := &a.gotoLn
	pal := th.Pal
	submit := false
	for {
		ev, ok := gtx.Event(key.Filter{Focus: &g.ed, Name: key.NameReturn}, key.Filter{Focus: &g.ed, Name: key.NameEnter}, key.Filter{Focus: &g.ed, Name: key.NameEscape})
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			if e.Name == key.NameEscape {
				g.open = false
				a.ui_.focusEditor = true
				return layout.Dimensions{}
			}
			submit = true
		}
	}
	if g.go_.Clicked(gtx) {
		submit = true
	}
	if submit {
		d := t.lineIndex()
		if line, col, ok := parseGoto(g.ed.Text(), d.lineAt(t.sel().Head).n, len(d.lines)); ok {
			a.goToLine(t, line)
			if col > 0 {
				l := d.line(line)
				p := l.from + min(col-1, l.to-l.from)
				t.setSel(Sel{p, p})
			}
			g.open = false
			a.ui_.focusEditor = true
			return layout.Dimensions{}
		}
	}
	g.ed.SingleLine = true
	if g.focus {
		g.focus = false
		gtx.Execute(key.FocusCmd{Tag: &g.ed})
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	m := op.Record(gtx.Ops)
	d := layout.Inset{Top: 4, Bottom: 4, Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(gtk.Text{S: "Go to line", Color: pal.FgColor}.Widget(th)),
			layout.Rigid(layout.Spacer{Width: 6}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return gtk.Entry{Width: 120}.Layout(gtx, th, &g.ed) }),
			layout.Rigid(layout.Spacer{Width: 4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return gtk.Button{Label: "go"}.Layout(gtx, th, &g.go_) }),
		)
	})
	call := m.Stop()
	d.Size.X = gtx.Constraints.Max.X
	gtk.FillRect(gtx, image.Rectangle{Max: d.Size}, pal.HeaderBg)
	gtk.FillRect(gtx, image.Rect(0, d.Size.Y-gtx.Dp(1), d.Size.X, d.Size.Y), pal.BorderColor)
	call.Add(gtx.Ops)
	return d
}

// ---- quick open (QuickOpen.svelte) -----------------------------------------------------------------

type quickOpen struct {
	open   bool
	ed     widget.Editor
	index  int
	clicks map[int]*widget.Clickable
	list   widget.List
	focus  bool
	last   string
	back   int
}

func (a *App) openQuickOpen() {
	if len(a.tabs) == 0 {
		return
	}
	q := &a.quick
	q.open, q.focus, q.index = true, true, 0
	q.ed.SetText("")
}

// score is QuickOpen.svelte's fuzzy match (-1: no match; lower is better).
func score(text, q string) int {
	if q == "" {
		return 0
	}
	t := strings.ToLower(text)
	if i := strings.Index(t, q); i >= 0 {
		return len([]rune(t[:i]))
	}
	tr := []rune(t)
	pos, gaps := -1, 0
	for _, ch := range q {
		next := -1
		for i := pos + 1; i < len(tr); i++ {
			if tr[i] == ch {
				next = i
				break
			}
		}
		if next < 0 {
			return -1
		}
		gaps += next - pos - 1
		pos = next
	}
	return 1000 + gaps
}

type qoItem struct {
	id            int
	title, detail string
}

func (a *App) quickItems() []qoItem {
	q := strings.ToLower(strings.TrimSpace(a.quick.ed.Text()))
	type scored struct {
		it qoItem
		s  int
	}
	var out []scored
	for _, t := range a.tabs {
		title := t.Title
		if t.Dirty {
			title += " *"
		}
		detail := ""
		if t.File != nil {
			detail = displayPath(*t.File)
		}
		it := qoItem{t.ID, title, detail}
		s := score(title, q)
		if s < 0 {
			if d := score(detail, q); d >= 0 {
				s = 2000 + d
			}
		}
		if s >= 0 {
			out = append(out, scored{it, s})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].s < out[j].s })
	items := make([]qoItem, len(out))
	for i, s := range out {
		items[i] = s.it
	}
	return items
}

func (a *App) quickOpenLayout(gtx layout.Context, th *gtk.Theme) {
	q := &a.quick
	pal := th.Pal
	items := a.quickItems()
	if q.ed.Text() != q.last {
		q.last, q.index = q.ed.Text(), 0
	}
	choose := func(id int) {
		q.open = false
		a.activate(id)
	}
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &q.ed, Name: key.NameEscape},
			key.Filter{Focus: &q.ed, Name: key.NameUpArrow},
			key.Filter{Focus: &q.ed, Name: key.NameDownArrow},
			key.Filter{Focus: &q.ed, Name: key.NameReturn},
			key.Filter{Focus: &q.ed, Name: key.NameEnter},
		)
		if !ok {
			break
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		switch e.Name {
		case key.NameEscape:
			q.open = false
			a.ui_.focusEditor = true
			return
		case key.NameDownArrow:
			q.index = min(len(items)-1, q.index+1)
		case key.NameUpArrow:
			q.index = max(0, q.index-1)
		default:
			if q.index < len(items) {
				choose(items[q.index].id)
				return
			}
		}
	}
	if q.clicks == nil {
		q.clicks = map[int]*widget.Clickable{}
	}
	for _, it := range items {
		if c := q.clicks[it.id]; c != nil && c.Clicked(gtx) {
			choose(it.id)
			return
		}
	}
	// A click outside closes it (the backdrop).
	for {
		_, ok := gtx.Event(pointerPress(&q.back))
		if !ok {
			break
		}
		q.open = false
		return
	}
	q.ed.SingleLine = true
	if q.focus {
		q.focus = false
		gtx.Execute(key.FocusCmd{Tag: &q.ed})
	}
	backdrop(gtx, &q.back)
	// .quick-open: 64px from the top, centered, min(620px, 90vw), padding 6px.
	w := min(gtx.Dp(620), gtx.Constraints.Max.X*9/10)
	x := (gtx.Constraints.Max.X - w) / 2
	defer op.Offset(image.Pt(x, gtx.Dp(64))).Push(gtx.Ops).Pop()
	g := gtx
	g.Constraints = layout.Constraints{Min: image.Pt(w, 0), Max: image.Pt(w, gtx.Constraints.Max.Y)}
	m := op.Record(gtx.Ops)
	d := layout.UniformInset(6).Layout(g, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return gtk.Entry{Placeholder: "Quick Open: type to filter open documents", Width: -1}.Layout(gtx, th, &q.ed)
			}),
			layout.Rigid(layout.Spacer{Height: 6}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if len(items) == 0 {
					return layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8}.Layout(gtx, gtk.Text{S: "No matching document", Color: pal.MutedFg}.Widget(th))
				}
				gtx.Constraints.Max.Y = gtx.Dp(320)
				q.list.Axis = layout.Vertical
				return q.list.Layout(gtx, len(items), func(gtx layout.Context, i int) layout.Dimensions {
					it := items[i]
					c := q.clicks[it.id]
					if c == nil {
						c = new(widget.Clickable)
						q.clicks[it.id] = c
					}
					active := i == q.index
					fg := pal.FgColor
					if active {
						fg = pal.SelectedFg
					}
					return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						m := op.Record(gtx.Ops)
						d := layout.Inset{Top: 4, Bottom: 4, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
								layout.Rigid(gtk.Text{S: it.title, Color: fg, MaxLines: 1}.Widget(th)),
								layout.Rigid(layout.Spacer{Width: 10}.Layout),
								layout.Flexed(1, gtk.Text{S: it.detail, Size: gtk.Em(0.85), Color: mixColor(fg, pal.MenuBg, 0.7), MaxLines: 1}.Widget(th)),
							)
						})
						call := m.Stop()
						d.Size.X = gtx.Constraints.Max.X
						if active {
							gtk.Box(gtx, d.Size, 3, pal.SelectedBg, color.NRGBA{})
						}
						call.Add(gtx.Ops)
						return d
					})
				})
			}),
		)
	})
	call := m.Stop()
	gtk.Box(gtx, d.Size, 6, pal.MenuBg, pal.BorderColor)
	call.Add(gtx.Ops)
}

func mixColor(c, bg color.NRGBA, a float32) color.NRGBA {
	m := func(x, y uint8) uint8 { return uint8(float32(y) + (float32(x)-float32(y))*a) }
	return color.NRGBA{R: m(c.R, bg.R), G: m(c.G, bg.G), B: m(c.B, bg.B), A: 0xff}
}

// ---- align on (EnterValueDialog.svelte) --------------------------------------------------------------

type alignPrompt struct {
	open       bool
	ed         widget.Editor
	dlg        gtk.Dialog
	ok, cancel widget.Clickable
	focus      bool
}

func (a *App) openAlign() {
	a.align.open, a.align.focus = true, true
	a.align.ed.SetText("")
}

func (a *App) alignLayout(gtx layout.Context, th *gtk.Theme) {
	p := &a.align
	submit := p.ok.Clicked(gtx)
	for {
		ev, ok := gtx.Event(key.Filter{Focus: &p.ed, Name: key.NameReturn}, key.Filter{Focus: &p.ed, Name: key.NameEnter})
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			submit = true
		}
	}
	if p.cancel.Clicked(gtx) || p.dlg.CloseRequested() {
		p.open = false
		a.ui_.focusEditor = true
		return
	}
	if submit {
		p.open = false
		if pat := p.ed.Text(); pat != "" {
			a.cmd(lines(func(text string, s Sel) []Change { return alignOn(text, s, pat, a.tabSize()) }))
		}
		a.ui_.focusEditor = true
		return
	}
	p.ed.SingleLine = true
	if p.focus {
		p.focus = false
		gtx.Execute(key.FocusCmd{Tag: &p.ed})
	}
	p.dlg.Title, p.dlg.Width, p.dlg.Closable = "Align On", 420, true
	pal := th.Pal
	p.dlg.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(14).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(gtk.Text{S: "Align lines on (regular expression; the first group, if any, marks the column):", Color: pal.FgColor}.Widget(th)),
				layout.Rigid(layout.Spacer{Height: 12}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return gtk.Entry{Placeholder: `e.g. =  or  :\s*(\S)`, Width: -1}.Layout(gtx, th, &p.ed)
				}),
			)
		})
	}, func(gtx layout.Context) layout.Dimensions {
		return gtk.Button{Label: "Cancel"}.Layout(gtx, th, &p.cancel)
	}, func(gtx layout.Context) layout.Dimensions {
		return gtk.Button{Label: "OK", Icon: iconOK}.Layout(gtx, th, &p.ok)
	})
}

var _ = unit.Dp(0)
var _ = clip.Rect{}

// pointerPress filters presses on tag.
func pointerPress(tag *int) event.Filter {
	return pointer.Filter{Target: tag, Kinds: pointer.Press}
}

// backdrop takes clicks on the whole window for tag (drawn below a popup).
func backdrop(gtx layout.Context, tag *int) {
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, tag)
}
