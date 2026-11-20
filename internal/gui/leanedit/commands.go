package leanedit

import (
	"strings"

	"gioui.org/layout"
)

// Selection and Go menus and the editing commands of commands.ts.
// TODO(7b): the commands, search bar, go to line, quick open, bookmarks
// and location history.

func (a *App) selectionItems(hasDoc bool) []item {
	ed := func(fn func(t *Tab)) func(layout.Context) { return func(layout.Context) { a.run(fn) } }
	return []item{
		{label: "Select All", shortcut: "Ctrl+A", disabled: !hasDoc, action: ed(func(t *Tab) { t.ed.SetCaret(t.ed.Len(), 0) })},
		{label: "Deselect", shortcut: "Ctrl+Shift+A", disabled: !hasDoc, action: ed(func(t *Tab) { t.ed.ClearSelection() })},
	}
}

func (a *App) goItems(hasDoc bool) []item {
	return []item{
		{label: "Next Tab", shortcut: "Alt+Right", disabled: len(a.tabs) < 2, action: func(layout.Context) { a.cycleTab(1) }},
		{label: "Previous Tab", shortcut: "Alt+Left", disabled: len(a.tabs) < 2, action: func(layout.Context) { a.cycleTab(-1) }},
	}
}

func (a *App) commandShortcuts() []shortcut { return nil }

func (a *App) findNext(dir int) {}

// indentLines indents (dir 1) or unindents (dir -1) the selected lines.
func indentLines(t *Tab, dir int) {
	_ = strings.TrimSpace
}
