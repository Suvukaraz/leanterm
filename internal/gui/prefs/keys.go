package prefs

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/Suvukaraz/leanterm/internal/gui/form"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

// keybindings is KeybindingsPage.svelte's reference table.
var keybindings = []struct{ window, action, desc, key string }{
	{"main", "find", "Find connection", "Ctrl+f"},
	{"main", "quit", "Quit application", "Ctrl+q"},
	{"main", "localshell", "Open local shell", "Ctrl+Shift+T"},
	{"main", "showconnections", "Toggle connections tree", "Alt+n"},
	{"tree_connections", "edit_node", "Edit node", "Alt+e"},
	{"tree_connections", "add_favourite", "Add favourite", "Alt+f"},
	{"tree_connections", "del_favourite", "Remove favourite", "Alt+Shift+F"},
	{"tree_connections", "find", "Find in tree", "Ctrl+f"},
	{"tree_connections", "expand_all", "Expand all", "Ctrl+r"},
	{"tree_connections", "collapse_all", "Collapse all", "Ctrl+t"},
	{"tree_connections", "clone", "Clone connection", "Ctrl+d"},
	{"tree_connections", "copy", "Copy node", "Ctrl+c"},
	{"tree_connections", "cut", "Cut node", "Ctrl+x"},
	{"tree_connections", "paste", "Paste node", "Ctrl+v"},
	{"tree_connections", "protection", "Toggle node protection", "Alt+r"},
	{"tree_connections", "rename", "Rename node", "F2"},
	{"tree_connections", "connect_node", "Connect node", "Alt+c"},
	{"tabs", "close", "Close current tab", "Ctrl+F4"},
	{"tabs", "next", "Next tab", "Ctrl+Page_Down"},
	{"tabs", "previous", "Previous tab", "Ctrl+Page_Up"},
	{"terminal", "fullscreen", "Toggle fullscreen", "F11"},
	{"terminal", "start", "Start session", "Ctrl+Return"},
	{"terminal", "copy", "Copy selection", "Ctrl+Shift+C"},
	{"terminal", "paste", "Paste clipboard", "Ctrl+Shift+V"},
	{"terminal", "paste_password", "Paste password from vault", "Ctrl+p"},
	{"terminal", "disconnect", "Disconnect session", "Ctrl+Shift+W"},
	{"terminal", "close", "Close tab", "Ctrl+w"},
	{"terminal", "restart", "Restart connection", "Ctrl+Shift+R"},
	{"terminal", "find_terminal", "Find in terminal output", "Ctrl+F3"},
	{"terminal", "paste_primary", "Paste selection", "Shift+Insert"},
	{"terminal", "zoom_in", "Zoom in", "Ctrl+plus"},
	{"terminal", "zoom_out", "Zoom out", "Ctrl+minus"},
	{"terminal", "zoom_reset", "Zoom reset", "Ctrl+0"},
	{"terminal", "toggle_sftp_browser", "Toggle file browser panel", "Alt+Ctrl+s"},
	{"sftp_browser", "refresh", "Refresh remote files", "F5"},
	{"sftp_browser", "parent", "Navigate to parent folder", "Backspace"},
	{"sftp_browser", "new_folder", "Create new remote folder", "F7"},
	{"sftp_browser", "toggle_hidden", "Toggle hidden files", "Ctrl+h"},
	{"sftp_browser", "edit_path", "Focus remote path bar", "Ctrl+l"},
}

func (d *Dialog) keybindingsPage(gtx layout.Context, th *gtk.Theme) layout.Dimensions {
	pal := th.Pal
	cell := func(gtx layout.Context, w int, fill bool, c layout.Widget) layout.Dimensions {
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
		return layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if fill {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
			}
			return layout.W.Layout(gtx, c)
		})
	}
	// row lays out the three columns (120px, the rest, 160px) on a line.
	row := func(gtx layout.Context, bg color.NRGBA, a, b, c layout.Widget) layout.Dimensions {
		total := gtx.Constraints.Max.X
		// The window column is at least 120px; the table widens it for
		// the longest name.
		w1, w3 := max(gtx.Dp(120), d.keyWinW+gtx.Dp(16)), gtx.Dp(160)
		w2 := max(0, total-w1-w3)
		m := op.Record(gtx.Ops)
		var hs [3]int
		var calls [3]op.CallOp
		for i, part := range []struct {
			w int
			c layout.Widget
		}{{w1, a}, {w2, b}, {w3, c}} {
			mm := op.Record(gtx.Ops)
			dims := cell(gtx, part.w, true, part.c)
			calls[i] = mm.Stop()
			hs[i] = dims.Size.Y
		}
		m.Stop()
		h := max(hs[0], hs[1], hs[2])
		size := image.Pt(total, h+gtx.Dp(1))
		if bg.A != 0 {
			gtk.FillRect(gtx, image.Rectangle{Max: size}, bg)
		}
		x := 0
		for i, w := range []int{w1, w2, w3} {
			o := op.Offset(image.Pt(x, (h-hs[i])/2)).Push(gtx.Ops)
			calls[i].Add(gtx.Ops)
			o.Pop()
			x += w
		}
		gtk.FillRect(gtx, image.Rect(0, h, total, size.Y), pal.BorderColor)
		return layout.Dimensions{Size: size}
	}
	txt := func(s string, c color.NRGBA, w font.Weight, mono bool) layout.Widget {
		return gtk.Text{S: s, Size: gtk.Em(0.9), Color: c, Weight: w, Mono: mono, MaxLines: 1}.Widget(th)
	}
	if d.keyWinW == 0 {
		for _, k := range keybindings {
			m := op.Record(gtx.Ops)
			dims := txt(k.window, pal.MutedFg, font.Normal, true)(gtx)
			m.Stop()
			d.keyWinW = max(d.keyWinW, dims.Size.X)
		}
	}
	if len(d.keyHover) != len(keybindings) {
		d.keyHover = make([]gtk.Hover, len(keybindings))
	}
	table := func(gtx layout.Context) layout.Dimensions {
		// The table box: border 1px, radius 3px, base color; the header row
		// stays on top while the rows scroll.
		gtx.Constraints.Min = gtx.Constraints.Max
		size := gtx.Constraints.Max
		gtk.Box(gtx, size, 3, pal.BaseColor, pal.BorderColor)
		b := max(1, gtx.Dp(1))
		o := op.Offset(image.Pt(b, b)).Push(gtx.Ops)
		gtx.Constraints = layout.Exact(size.Sub(image.Pt(2*b, 2*b)))
		layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				h := func(s string) layout.Widget { return txt(s, pal.FgColor, font.Bold, false) }
				return row(gtx, pal.HeaderBg, h("Window"), h("Action / Description"), h("Keybinding"))
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				l := &d.lists[15]
				l.Axis = layout.Vertical
				return l.Layout(gtx, len(keybindings), func(gtx layout.Context, i int) layout.Dimensions {
					k := keybindings[i]
					hv := &d.keyHover[i]
					bg := color.NRGBA{}
					if hv.Update(gtx) {
						bg = pal.RowAlt
					}
					dims := row(gtx, bg,
						txt(k.window, pal.MutedFg, font.Normal, true),
						form.RowGap(0, txt(k.action, pal.FgColor, font.Bold, false), txt(": "+k.desc, pal.FgColor, font.Normal, false)),
						func(gtx layout.Context) layout.Dimensions {
							return boxB(gtx, pal.HeaderBg, pal.BorderColor, 3, layout.Inset{Top: 2, Bottom: 2, Left: 6, Right: 6},
								gtk.Text{S: k.key, Size: gtk.Em(0.9 * 0.85), Color: pal.FgColor, Mono: true, MaxLines: 1}.Widget(th))
						})
					area := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
					hv.Add(gtx)
					area.Pop()
					return dims
				})
			}),
		)
		o.Pop()
		return layout.Dimensions{Size: size}
	}
	return layout.UniformInset(10).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(d.note(th, seg{s: "Keyboard shortcut reference by window. Changing shortcuts is not supported yet."})),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Flexed(1, table),
		)
	})
}
