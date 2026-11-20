package leanedit

import (
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/transfer"
	"gioui.org/layout"
)

// encodings are the encodings LeanEdit offers (EditorApp.svelte's ENCODINGS).
var encodings = []struct{ id, label string }{
	{"utf-8", "UTF-8"},
	{"utf-8-bom", "UTF-8 with BOM"},
	{"windows-1252", "Windows-1252 (Western)"},
	{"iso-8859-1", "ISO-8859-1 (Latin-1)"},
	{"iso-8859-15", "ISO-8859-15 (Latin-9)"},
}

// ---- actions -----------------------------------------------------------------------------------

// Save saves the active tab.
func (a *App) Save() {
	if t := a.Active(); t != nil && t.ready {
		id := t.ID
		a.flow(func() { a.saveTab(id, false) })
	}
}

// SaveAs saves the active tab under another name.
func (a *App) SaveAs() {
	if t := a.Active(); t != nil && t.ready {
		id := t.ID
		a.flow(func() { a.saveTabAs(id) })
	}
}

// run applies fn to the active document and gives it the focus back.
func (a *App) run(fn func(t *Tab)) {
	if t := a.Active(); t != nil && t.ready {
		fn(t)
		a.ui_.focusEditor = true
		a.markDirty(t)
	}
}

func (a *App) copySelection(gtx layout.Context, cut bool) {
	t := a.Active()
	if t == nil || !t.ready {
		return
	}
	s := t.ed.SelectedText()
	if s == "" {
		return
	}
	gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: readCloser(s)})
	if cut {
		t.ed.Insert("")
		a.markDirty(t)
	}
	a.ui_.focusEditor = true
}

// paste asks for the clipboard; gvcode inserts it when it arrives.
func (a *App) paste(gtx layout.Context) {
	t := a.Active()
	if t == nil || !t.ready {
		return
	}
	gtx.Execute(clipboard.ReadCmd{Tag: t.ed})
	a.ui_.focusEditor = true
}

func (a *App) setLineEnding(le string) {
	if t := a.Active(); t != nil && t.LineEnding != le {
		t.LineEnding = le
		a.markDirty(t)
	}
}

func (a *App) toggleLineEnding() {
	if t := a.Active(); t != nil {
		if t.LineEnding == "crlf" {
			a.setLineEnding("lf")
		} else {
			a.setLineEnding("crlf")
		}
	}
}

func (a *App) setEncoding(enc string) {
	if t := a.Active(); t != nil && t.Encoding != enc {
		t.Encoding = enc
		a.markDirty(t)
	}
}

func (a *App) setLanguage(id string) {
	if t := a.Active(); t != nil {
		t.LanguageID = id
		t.lexer = lexerFor(id)
		t.retoken = true
	}
}

func (a *App) toggleHighlight() {
	if t := a.Active(); t != nil {
		t.Highlight = !t.Highlight
		t.retoken = true
	}
}

func (a *App) zoom(delta float32, reset bool) {
	base := float32(a.settings.Editor.Font.Size)
	if base <= 0 {
		base = 11
	}
	if reset {
		a.zoomPt = 0
		return
	}
	a.zoomPt = max(6-base, min(40-base, a.zoomPt+delta))
}

func (a *App) cycleTab(dir int) {
	if len(a.tabs) < 2 {
		return
	}
	for i, t := range a.tabs {
		if t.ID == a.active {
			a.activate(a.tabs[(i+dir+len(a.tabs))%len(a.tabs)].ID)
			return
		}
	}
}

func (a *App) newWindow() {
	a.flow(func() {
		if err := a.Svc.NewWindow(); err != nil {
			a.showError("A new LeanEdit window could not be opened.", err)
		}
	})
}

func (a *App) closeActive() {
	if t := a.Active(); t != nil {
		id := t.ID
		a.flow(func() { a.closeTab(id) })
	}
}

// Overwrite and block selection: gvcode has neither (see the docs); the
// status bar shows the modes the web GUI shows.
func (a *App) insLabel() string   { return "INS" }
func (a *App) blockLabel() string { return "LINE" }
func (a *App) toggleOverwrite()   {}
func (a *App) toggleBlock()       {}

// openSearch opens the find/replace bar (stage 7b).
func (a *App) openSearch(gtx layout.Context) {}

func (a *App) encodingItems(reload bool) []item {
	t := a.Active()
	var out []item
	for _, e := range encodings {
		e := e
		it := item{label: e.label}
		if reload {
			it.action = func(layout.Context) {
				if t != nil {
					id := t.ID
					a.flow(func() { a.reloadTab(id, e.id) })
				}
			}
		} else {
			it.checked = t != nil && t.Encoding == e.id
			it.action = func(layout.Context) { a.setEncoding(e.id) }
		}
		out = append(out, it)
	}
	return out
}

func (a *App) languageItems() []item {
	cur := ""
	if t := a.Active(); t != nil {
		cur = t.LanguageID
	}
	items := []item{{label: "Plain Text", checked: cur == "", action: func(layout.Context) { a.setLanguage("") }}, sep()}
	for _, g := range languageGroups() {
		var sub []item
		for _, l := range g.langs {
			l := l
			sub = append(sub, item{label: l.name, checked: cur == l.id, action: func(layout.Context) { a.setLanguage(l.id) }})
		}
		items = append(items, item{label: g.label, sub: sub})
	}
	return items
}

// ---- menus (EditorApp.svelte's menus) ------------------------------------------------------------

func (a *App) menus() []topMenu {
	t := a.Active()
	hasDoc := t != nil && t.ready
	hasFile := t != nil && t.File != nil
	act := func(fn func()) func(layout.Context) { return func(layout.Context) { fn() } }
	ed := func(fn func(t *Tab)) func(layout.Context) { return func(layout.Context) { a.run(fn) } }
	return []topMenu{
		{label: "File", items: []item{
			{label: "New", icon: iconNew, shortcut: "Ctrl+N", action: act(a.NewDocument)},
			{label: "Open…", icon: iconOpen, shortcut: "Ctrl+O", action: act(a.ChooseAndOpen)},
			{label: "New Window", shortcut: "Ctrl+Shift+N", action: act(a.newWindow)},
			sep(),
			{label: "Save", icon: iconSave, shortcut: "Ctrl+S", disabled: !hasDoc, action: act(a.Save)},
			{label: "Save As…", icon: iconSaveAs, shortcut: "Ctrl+Shift+S", disabled: !hasDoc, action: act(a.SaveAs)},
			{label: "Save All", disabled: !a.anyDirty(), action: act(func() { a.flow(func() { a.saveAll() }) })},
			sep(),
			{label: "Reload", icon: iconRefresh, shortcut: "F5", disabled: !hasFile, action: act(a.reloadActive)},
			{label: "Reload with Encoding", disabled: !hasFile, sub: a.encodingItems(true)},
			sep(),
			{label: "Close", icon: iconClose, shortcut: "Ctrl+W", disabled: t == nil, action: act(a.closeActive)},
			{label: "Close All", disabled: len(a.tabs) == 0, action: act(func() { a.flow(func() { a.closeAll() }) })},
			sep(),
			{label: "Quit Editor", icon: iconQuit, shortcut: "Ctrl+Q", action: act(a.RequestClose)},
		}},
		{label: "Edit", items: []item{
			{label: "Undo", icon: iconUndo, shortcut: "Ctrl+Z", disabled: !hasDoc, action: ed(func(t *Tab) { t.ed.Undo() })},
			{label: "Redo", icon: iconRedo, shortcut: "Ctrl+Shift+Z", disabled: !hasDoc, action: ed(func(t *Tab) { t.ed.Redo() })},
			sep(),
			{label: "Cut", icon: iconCut, shortcut: "Ctrl+X", disabled: !hasDoc, action: func(gtx layout.Context) { a.copySelection(gtx, true) }},
			{label: "Copy", icon: iconCopy, shortcut: "Ctrl+C", disabled: !hasDoc, action: func(gtx layout.Context) { a.copySelection(gtx, false) }},
			{label: "Paste", icon: iconPaste, shortcut: "Ctrl+V", disabled: !hasDoc, action: a.paste},
			sep(),
			{label: "Find…", icon: iconFind, shortcut: "Ctrl+F", disabled: !hasDoc, action: a.openSearch},
			{label: "Replace…", icon: iconEdit, shortcut: "Ctrl+R", disabled: !hasDoc, action: a.openSearch},
			{label: "Find Next", shortcut: "F3", disabled: !hasDoc, action: func(layout.Context) { a.findNext(1) }},
			{label: "Find Previous", shortcut: "Shift+F3", disabled: !hasDoc, action: func(layout.Context) { a.findNext(-1) }},
		}},
		{label: "Selection", items: a.selectionItems(hasDoc)},
		{label: "View", items: a.viewItems(hasDoc)},
		{label: "Go", items: a.goItems(hasDoc)},
		{label: "Tools", items: []item{
			{label: "Line Endings", disabled: !hasDoc, sub: []item{
				{label: "Unix (LF)", checked: t != nil && t.LineEnding == "lf", action: act(func() { a.setLineEnding("lf") })},
				{label: "Windows (CRLF)", checked: t != nil && t.LineEnding == "crlf", action: act(func() { a.setLineEnding("crlf") })},
			}},
			{label: "Encoding", disabled: !hasDoc, sub: a.encodingItems(false)},
			{label: "Highlighting", disabled: !hasDoc, sub: a.languageItems()},
			sep(),
			{label: "Indent", shortcut: "Tab", disabled: !hasDoc, action: ed(func(t *Tab) { indentLines(t, 1) })},
			{label: "Unindent", shortcut: "Shift+Tab", disabled: !hasDoc, action: ed(func(t *Tab) { indentLines(t, -1) })},
		}},
		// Opens LeanEdit's settings window right away (no drop-down menu).
		{label: "Settings", action: func(layout.Context) {
			a.flow(func() { _ = a.Svc.ShowPreferences() })
		}},
	}
}

func (a *App) reloadActive() {
	if t := a.Active(); t != nil && t.File != nil {
		id := t.ID
		a.flow(func() { a.reloadTab(id, "") })
	}
}

func (a *App) viewItems(hasDoc bool) []item {
	t := a.Active()
	return []item{
		{label: "Word Wrap", checked: a.wrap(), action: func(layout.Context) { v := !a.wrap(); a.wrapOverride = &v }},
		{label: "Show Whitespace", checked: a.whitespace(), action: func(layout.Context) { v := !a.whitespace(); a.whitespaceOverride = &v }},
		{label: "Show Scrollbar Minimap", checked: a.minimap(), action: func(layout.Context) { v := !a.minimap(); a.minimapOverride = &v }},
		{label: "Syntax Highlighting", checked: t != nil && t.Highlight, disabled: !hasDoc, action: func(layout.Context) { a.toggleHighlight() }},
		sep(),
		{label: "Fold All", disabled: true},
		{label: "Unfold All", disabled: true},
		sep(),
		{label: "Zoom In", shortcut: "Ctrl++", action: func(layout.Context) { a.zoom(1, false) }},
		{label: "Zoom Out", shortcut: "Ctrl+-", action: func(layout.Context) { a.zoom(-1, false) }},
		{label: "Reset Zoom", shortcut: "Ctrl+0", action: func(layout.Context) { a.zoom(0, true) }},
		sep(),
		{label: "Show Toolbar", checked: a.toolbar, action: func(layout.Context) { a.toolbar = !a.toolbar }},
	}
}

func (a *App) whitespace() bool {
	if a.whitespaceOverride != nil {
		return *a.whitespaceOverride
	}
	return a.settings.Editor.ShowWhitespace
}

func (a *App) minimap() bool {
	if a.minimapOverride != nil {
		return *a.minimapOverride
	}
	return a.settings.Editor.ShowMinimap
}

// ---- keyboard shortcuts (handleKeyDown, handleGoKeys) ---------------------------------------------

type shortcut struct {
	name  key.Name
	mods  key.Modifiers
	run   func(gtx layout.Context)
	focus bool // also while the editor has the focus (its own keys come first otherwise)
}

func (a *App) shortcuts(gtx layout.Context) {
	if a.question != nil {
		return
	}
	ctrl, shift, alt := key.ModShortcut, key.ModShift, key.ModAlt
	list := []shortcut{
		{"N", ctrl, func(layout.Context) { a.NewDocument() }, true},
		{"N", ctrl | shift, func(layout.Context) { a.newWindow() }, true},
		{"O", ctrl, func(layout.Context) { a.ChooseAndOpen() }, true},
		{"S", ctrl, func(layout.Context) { a.Save() }, true},
		{"S", ctrl | shift, func(layout.Context) { a.SaveAs() }, true},
		{"W", ctrl, func(layout.Context) { a.closeActive() }, true},
		{"Q", ctrl, func(layout.Context) { a.RequestClose() }, true},
		{key.NameTab, ctrl, func(layout.Context) { a.cycleTab(1) }, true},
		{key.NameTab, ctrl | shift, func(layout.Context) { a.cycleTab(-1) }, true},
		{key.NamePageDown, ctrl, func(layout.Context) { a.cycleTab(1) }, true},
		{key.NamePageUp, ctrl, func(layout.Context) { a.cycleTab(-1) }, true},
		{key.NameRightArrow, alt, func(layout.Context) { a.cycleTab(1) }, true},
		{key.NameLeftArrow, alt, func(layout.Context) { a.cycleTab(-1) }, true},
		{"+", ctrl, func(layout.Context) { a.zoom(1, false) }, true},
		{"=", ctrl, func(layout.Context) { a.zoom(1, false) }, true},
		{"-", ctrl, func(layout.Context) { a.zoom(-1, false) }, true},
		{"0", ctrl, func(layout.Context) { a.zoom(0, true) }, true},
		{key.NameF5, 0, func(layout.Context) { a.reloadActive() }, true},
		{"F", ctrl, a.openSearch, true},
		{"R", ctrl, a.openSearch, true},
	}
	list = append(list, a.commandShortcuts()...)
	t := a.Active()
	for _, s := range list {
		filters := []event.Filter{key.Filter{Name: s.name, Required: s.mods}}
		if s.focus && t != nil {
			filters = append(filters, key.Filter{Focus: t.ed, Name: s.name, Required: s.mods})
		}
		for {
			ev, ok := gtx.Event(filters...)
			if !ok {
				break
			}
			if e, ok := ev.(key.Event); ok && e.State == key.Press && e.Modifiers == s.mods {
				s.run(gtx)
			}
		}
	}
}

// ---- helpers ------------------------------------------------------------------------------------

type stringReader struct{ *strings.Reader }

func (stringReader) Close() error { return nil }

func readCloser(s string) stringReader { return stringReader{strings.NewReader(s)} }

var _ transfer.DataEvent
