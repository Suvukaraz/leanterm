package leanedit

import (
	"fmt"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"github.com/oligo/gvcode"
)

// Selection and Go menus and the editing commands (commands.ts on gvcode).

// sel is the tab's selection (gvcode: caret and the other end).
func (t *Tab) sel() Sel {
	caret, other := t.ed.Selection()
	return Sel{Anchor: other, Head: caret}
}

func (t *Tab) setSel(s Sel) { t.ed.SetCaret(s.Head, s.Anchor) }

// applyEdit applies changes as one edit (one undo step): it replaces the
// smallest span that covers them, then sets the selection (nil: the old
// one, moved along).
func (a *App) applyEdit(t *Tab, changes []Change, ns *Sel) bool {
	if len(changes) == 0 {
		if ns != nil {
			t.setSel(*ns)
		}
		return false
	}
	old := t.ed.Text()
	next, mapPos := apply(old, changes)
	s := t.sel()
	if ns == nil {
		m := Sel{mapPos(s.Anchor, 1), mapPos(s.Head, 1)}
		ns = &m
	}
	or, nr := []rune(old), []rune(next)
	pre := 0
	for pre < len(or) && pre < len(nr) && or[pre] == nr[pre] {
		pre++
	}
	suf := 0
	for suf < len(or)-pre && suf < len(nr)-pre && or[len(or)-1-suf] == nr[len(nr)-1-suf] {
		suf++
	}
	t.ed.SetCaret(len(or)-suf, pre)
	if mid := string(nr[pre : len(nr)-suf]); mid != "" {
		t.ed.Insert(mid)
	} else {
		t.ed.Delete(1) // deletes the selection
	}
	t.setSel(*ns)
	a.recordEdit(t)
	a.markDirty(t)
	t.retoken = true
	return true
}

// cmd runs a text command on the active document.
func (a *App) cmd(fn func(t *Tab, text string, s Sel) ([]Change, *Sel)) {
	a.run(func(t *Tab) {
		ch, ns := fn(t, t.ed.Text(), t.sel())
		a.applyEdit(t, ch, ns)
	})
}

func lines(fn func(text string, s Sel) []Change) func(t *Tab, text string, s Sel) ([]Change, *Sel) {
	return func(t *Tab, text string, s Sel) ([]Change, *Sel) { return fn(text, s), nil }
}

func (a *App) caseCmd(mode string) {
	a.cmd(func(t *Tab, text string, s Sel) ([]Change, *Sel) {
		ch, ns := changeCase(text, s, mode)
		return ch, &ns
	})
}

func (a *App) tabSize() int { return a.look().tabWidth }

func (a *App) indentUnit() string {
	if a.settings.Editor.IndentWithTabs {
		return "\t"
	}
	return strings.Repeat(" ", a.tabSize())
}

func (a *App) indent(dir int) {
	a.cmd(lines(func(text string, s Sel) []Change { return indentChanges(text, s, dir, a.indentUnit(), a.tabSize()) }))
}

// ---- navigation --------------------------------------------------------------------------------

// goToLine moves the cursor to line n (clamped) and records the jump.
func (a *App) goToLine(t *Tab, n int) {
	d := t.lineIndex()
	n = max(1, min(len(d.lines), n))
	from := a.currentLocation()
	p := d.line(n).from
	t.setSel(Sel{p, p})
	a.recordJump(from)
}

func (a *App) location(t *Tab, pos int) location {
	mk, _ := t.ed.CreateMarker(pos, gvcode.BiasBackward)
	return location{tab: t.ID, mk: mk, pos: pos}
}

func (a *App) currentLocation() *location {
	t := a.Active()
	if t == nil || !t.ready {
		return nil
	}
	l := a.location(t, t.sel().Head)
	return &l
}

func (a *App) sameLine(x, y location) bool {
	if x.tab != y.tab {
		return false
	}
	t := a.tab(x.tab)
	if t == nil {
		return x.offset() == y.offset()
	}
	d := t.lineIndex()
	return d.lineAt(x.offset()).n == d.lineAt(y.offset()).n
}

func (a *App) histories() {
	if a.jumps == nil {
		a.jumps, a.edits = newHistory(a.sameLine), newHistory(a.sameLine)
		drop := func(l location) {
			if t := a.tab(l.tab); t != nil && l.mk != nil {
				t.ed.RemoveMarker(l.mk)
			}
		}
		a.jumps.drop, a.edits.drop = drop, drop
	}
}

// forgetTab drops a tab's locations (closed or reloaded).
func (a *App) forgetTab(id int) {
	if a.jumps != nil {
		a.jumps.removeTab(id)
		a.edits.removeTab(id)
	}
}

func (a *App) recordJump(from *location) {
	a.histories()
	to := a.currentLocation()
	if from != nil && to != nil && !a.sameLine(*from, *to) {
		a.jumps.jump(*from, *to)
		return
	}
	for _, l := range []*location{from, to} {
		if l != nil && l.mk != nil {
			if t := a.tab(l.tab); t != nil {
				t.ed.RemoveMarker(l.mk)
			}
		}
	}
}

func (a *App) recordEdit(t *Tab) {
	a.histories()
	a.edits.edit(a.location(t, t.sel().Head))
}

func (a *App) goToLocation(l *location) {
	if l == nil {
		return
	}
	t := a.tab(l.tab)
	if t == nil {
		return
	}
	a.activate(t.ID)
	p := min(l.offset(), t.ed.Len())
	t.setSel(Sel{p, p})
}

func (a *App) gotoBookmark(dir int) {
	a.run(func(t *Tab) {
		cur := t.lineIndex().lineAt(t.sel().Head).n
		if n := nextLineIn(t.bookmarkLines(), cur, dir, true); n > 0 {
			a.goToLine(t, n)
		}
	})
}

func (a *App) gotoModified(dir int) {
	a.run(func(t *Tab) {
		cur := t.lineIndex().lineAt(t.sel().Head).n
		set := t.modifiedSet()
		start := cur
		for set[start+dir] {
			start += dir
		}
		if n := nextLineIn(t.modifiedList(), start, dir, false); n > 0 {
			a.goToLine(t, n)
		}
	})
}

func (a *App) matchingBracket(selectTo bool) {
	a.run(func(t *Tab) {
		s := t.sel()
		m, at, ok := matchingBracket(t.ed.Text(), s.Head)
		if !ok {
			return
		}
		if !selectTo {
			// The cursor goes to the other side of the matching bracket.
			p := m
			if m > at {
				p = m + 1
			}
			t.setSel(Sel{p, p})
			return
		}
		lo, hi := min(at, m), max(at, m)+1
		t.setSel(Sel{lo, hi})
	})
}

// ---- menus ---------------------------------------------------------------------------------------

func (a *App) selectionItems(hasDoc bool) []item {
	act := func(fn func()) func(layout.Context) { return func(layout.Context) { fn() } }
	t := a.Active()
	selected := t != nil && t.ready && t.ed.SelectionLen() > 0
	return []item{
		{label: "Select All", shortcut: "Ctrl+A", disabled: !hasDoc, action: act(func() { a.run(func(t *Tab) { t.setSel(Sel{0, t.ed.Len()}) }) })},
		{label: "Deselect", shortcut: "Ctrl+Shift+A", disabled: !hasDoc || !selected, action: act(a.deselect)},
		// gvcode has no block selection (see the docs).
		{label: "Block Selection Mode", shortcut: "Ctrl+Shift+B", disabled: true},
		sep(),
		{label: "Toggle Comment", shortcut: "Ctrl+/", disabled: !hasDoc, action: act(a.toggleComment)},
		{label: "Join Lines", shortcut: "Ctrl+J", disabled: !hasDoc, action: act(func() { a.cmd(lines(joinLines)) })},
		{label: "Capitalisation", disabled: !hasDoc, sub: []item{
			{label: "Uppercase", shortcut: "Ctrl+U", action: act(func() { a.caseCmd("upper") })},
			{label: "Lowercase", shortcut: "Ctrl+Shift+U", action: act(func() { a.caseCmd("lower") })},
			{label: "Capitalize", shortcut: "Ctrl+Alt+U", action: act(func() { a.caseCmd("capitalize") })},
		}},
		sep(),
		{label: "Clean Indentation", disabled: !hasDoc, action: act(func() { a.convertIndent(a.settings.Editor.IndentWithTabs) })},
		{label: "Convert Indentation to Spaces", disabled: !hasDoc, action: act(func() { a.convertIndent(false) })},
		{label: "Convert Indentation to Tabs", disabled: !hasDoc, action: act(func() { a.convertIndent(true) })},
		{label: "Format Indentation", disabled: !hasDoc, action: act(func() { a.convertIndent(a.settings.Editor.IndentWithTabs) })},
		{label: "Align On…", disabled: !hasDoc, action: act(func() { a.openAlign() })},
		{label: "Apply Word Wrap", disabled: !hasDoc, action: act(func() {
			a.cmd(lines(func(text string, s Sel) []Change { return applyWordWrap(text, s, 80, a.tabSize()) }))
		})},
		sep(),
		// gvcode has a single cursor: no extra carets (see the docs).
		{label: "Add Caret above Cursor", shortcut: "Ctrl+Alt+Up", disabled: true},
		{label: "Add Caret below Cursor", shortcut: "Ctrl+Alt+Down", disabled: true},
		{label: "Add Cursors to Line Ends", shortcut: "Alt+Shift+I", disabled: true},
		{label: "Find and Select Next Occurrence", shortcut: "Alt+J", disabled: !hasDoc, action: act(a.selectNext)},
		{label: "Find and Select All Occurrences", shortcut: "Ctrl+Alt+Shift+J", disabled: true},
	}
}

func (a *App) deselect() {
	a.run(func(t *Tab) {
		h := t.sel().Head
		t.setSel(Sel{h, h})
	})
}

func (a *App) convertIndent(tabs bool) {
	a.cmd(lines(func(text string, s Sel) []Change { return convertIndentation(text, s, tabs, a.tabSize()) }))
}

func (a *App) toggleComment() {
	a.cmd(func(t *Tab, text string, s Sel) ([]Change, *Sel) { return toggleComment(text, s, t.LanguageID), nil })
}

func (a *App) selectNext() {
	a.run(func(t *Tab) {
		if ns, ok := nextOccurrence(t.ed.Text(), t.sel()); ok {
			t.setSel(ns)
		}
	})
}

func (a *App) goItems(hasDoc bool) []item {
	act := func(fn func()) func(layout.Context) { return func(layout.Context) { fn() } }
	return []item{
		{label: "Quick Open", shortcut: "Ctrl+Alt+O", disabled: len(a.tabs) == 0, action: act(a.openQuickOpen)},
		sep(),
		{label: "Go to Line…", shortcut: "Ctrl+G", disabled: !hasDoc, action: act(a.openGotoLine)},
		sep(),
		{label: "Next Tab", shortcut: "Alt+Right", disabled: len(a.tabs) < 2, action: act(func() { a.cycleTab(1) })},
		{label: "Previous Tab", shortcut: "Alt+Left", disabled: len(a.tabs) < 2, action: act(func() { a.cycleTab(-1) })},
		sep(),
		{label: "Go to Previous Location", shortcut: "Ctrl+1", disabled: !hasDoc, action: act(a.prevLocation)},
		{label: "Go to Next Location", shortcut: "Ctrl+Shift+1", disabled: !hasDoc, action: act(a.nextLocation)},
		sep(),
		{label: "Go to Previous Editing Location", shortcut: "Ctrl+E", disabled: !hasDoc, action: act(a.prevEdit)},
		{label: "Go to Next Editing Location", shortcut: "Ctrl+Shift+E", disabled: !hasDoc, action: act(a.nextEdit)},
		sep(),
		{label: "Go to Previous Modified Line", disabled: !hasDoc, action: act(func() { a.gotoModified(-1) })},
		{label: "Go to Next Modified Line", disabled: !hasDoc, action: act(func() { a.gotoModified(1) })},
		sep(),
		{label: "Go to Matching Bracket", shortcut: "Ctrl+6", disabled: !hasDoc, action: act(func() { a.matchingBracket(false) })},
		{label: "Select to Matching Bracket", shortcut: "Ctrl+Shift+6", disabled: !hasDoc, action: act(func() { a.matchingBracket(true) })},
		sep(),
		{label: "Bookmarks", disabled: !hasDoc, sub: a.bookmarkItems()},
	}
}

func (a *App) prevLocation() { a.histories(); a.goToLocation(a.jumps.back(a.currentLocation())) }
func (a *App) nextLocation() { a.histories(); a.goToLocation(a.jumps.forward()) }
func (a *App) prevEdit()     { a.histories(); a.goToLocation(a.edits.back(a.currentLocation())) }
func (a *App) nextEdit()     { a.histories(); a.goToLocation(a.edits.forward()) }

// bookmarkItems is Go → Bookmarks: the commands and one entry per bookmark.
func (a *App) bookmarkItems() []item {
	t := a.Active()
	var marks []int
	cur := 0
	if t != nil && t.ready {
		marks = t.bookmarkLines()
		cur = t.lineIndex().lineAt(t.sel().Head).n
	}
	has := func(n int) bool {
		for _, m := range marks {
			if m == n {
				return true
			}
		}
		return false
	}
	act := func(fn func()) func(layout.Context) { return func(layout.Context) { fn() } }
	items := []item{
		{label: "Set Bookmark", shortcut: "Ctrl+B", checked: has(cur), action: act(func() { a.run(func(t *Tab) { t.toggleBookmark() }) })},
		{label: "Clear All Bookmarks", disabled: len(marks) == 0, action: act(func() { a.run(func(t *Tab) { t.clearBookmarks() }) })},
		sep(),
		{label: "Next Bookmark", shortcut: "Alt+PgDown", disabled: len(marks) == 0, action: act(func() { a.gotoBookmark(1) })},
		{label: "Previous Bookmark", shortcut: "Alt+PgUp", disabled: len(marks) == 0, action: act(func() { a.gotoBookmark(-1) })},
	}
	if len(marks) > 0 {
		items = append(items, sep())
		d := t.lineIndex()
		for _, n := range marks {
			n := n
			text := strings.TrimSpace(d.line(n).text)
			if r := []rune(text); len(r) > 40 {
				text = string(r[:40]) + "…"
			}
			items = append(items, item{label: fmt.Sprintf("%d: %s", n, text), action: act(func() { a.run(func(t *Tab) { a.goToLine(t, n) }) })})
		}
	}
	return items
}

// commandShortcuts are the editing keys of setup.ts's keymap and the Go
// menu's keys (they also work while the editor has the focus).
func (a *App) commandShortcuts() []shortcut {
	ctrl, shift, alt := key.ModShortcut, key.ModShift, key.ModAlt
	f := func(fn func()) func(layout.Context) { return func(layout.Context) { fn() } }
	return []shortcut{
		{"G", ctrl, f(a.openGotoLine), true},
		{"H", ctrl, func(gtx layout.Context) { a.openSearch(gtx) }, true},
		{"A", ctrl | shift, f(a.deselect), true},
		{"J", ctrl, f(func() { a.cmd(lines(joinLines)) }), true},
		{"U", ctrl, f(func() { a.caseCmd("upper") }), true},
		{"U", ctrl | shift, f(func() { a.caseCmd("lower") }), true},
		{"U", ctrl | alt, f(func() { a.caseCmd("capitalize") }), true},
		{"J", alt, f(a.selectNext), true},
		{"/", ctrl, f(a.toggleComment), true},
		{"6", ctrl, f(func() { a.matchingBracket(false) }), true},
		{"6", ctrl | shift, f(func() { a.matchingBracket(true) }), true},
		{"B", ctrl, f(func() { a.run(func(t *Tab) { t.toggleBookmark() }) }), true},
		{key.NamePageDown, alt, f(func() { a.gotoBookmark(1) }), true},
		{key.NamePageUp, alt, f(func() { a.gotoBookmark(-1) }), true},
		{"1", ctrl, f(a.prevLocation), true},
		{"1", ctrl | shift, f(a.nextLocation), true},
		{"E", ctrl, f(a.prevEdit), true},
		{"E", ctrl | shift, f(a.nextEdit), true},
		{"O", ctrl | alt, f(a.openQuickOpen), true},
		{key.NameF3, 0, f(func() { a.findNext(1) }), true},
		{key.NameF3, shift, f(func() { a.findNext(-1) }), true},
	}
}
