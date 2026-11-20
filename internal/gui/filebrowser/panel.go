package filebrowser

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/sftpsvc"
)

// Panel is SftpPanel.svelte: header, toolbar, path bar, the file list,
// the transfer queue and the footer. Menus and dialogs are drawn by
// [Panel.LayoutOverlays] over the whole window.
type Panel struct {
	B *Browser
	// Connected reports whether the session is connected (else the list is
	// covered).
	Connected func() bool
	// PointerPos is where context menus open (window coordinates).
	PointerPos func() image.Point
	// TerminalInput types text into the terminal (copy path, cd).
	TerminalInput func(s string)
	// ChooseFile asks for a file or folder (upload, copy to, move to).
	ChooseFile func(title, dir string, folder bool) (string, error)
	// OnClose hides the panel (× and "Hide browser").
	OnClose func()
	// OpenInEditor opens a file in LeanEdit ("Open in Editor"); nil hides
	// the item. OpensInEditor decides what a double click opens there.
	OpenInEditor  func(path string)
	OpensInEditor func(path string) bool

	sortBy  SortBy
	sortAsc bool
	list    widget.List
	rows    map[string]*rowState
	up      widget.Clickable
	listTag int
	typed   string
	typedAt time.Time

	close                                                  widget.Clickable
	back, fwd, upBtn, home, refresh, newDir, upl, dl, more widget.Clickable
	heads                                                  [4]widget.Clickable
	pathEd                                                 widget.Editor
	pathFor                                                string
	histBtn                                                widget.Clickable
	follow                                                 widget.Bool
	queueHead                                              widget.Clickable
	queueOpen                                              bool
	queueBtns                                              map[string]*widget.Clickable

	menu       gtk.Menu // toolbar ≡, list and history menus
	menuKind   string
	menuTarget *sftpsvc.FileEntry

	dlg     string // the open dialog: newdir, newfile, rename, delete, perms, conflict, error
	dlgBox  gtk.Dialog
	msg     gtk.MessageDialog
	nameEd  widget.Editor
	ok      widget.Clickable
	cancel  widget.Clickable
	skip    widget.Clickable
	paths   []string
	perm    perms
	pending func(overwrite, skip bool)

	errMu  sync.Mutex
	errMsg string
	// upload back: a file opened with its application changed.
	backLocal, backRemote string
}

type rowState struct {
	click widget.Clickable
	area  int
}

func (p *Panel) init() {
	if p.rows == nil {
		p.rows = map[string]*rowState{}
		p.queueBtns = map[string]*widget.Clickable{}
		p.sortBy, p.sortAsc = SortName, true
		p.list.Axis = layout.Vertical
		p.pathEd.SingleLine, p.pathEd.Submit = true, true
		p.B.OnError = func(err error) {
			p.errMu.Lock()
			p.errMsg = err.Error()
			p.errMu.Unlock()
			if p.B.Changed != nil {
				p.B.Changed()
			}
		}
	}
}

// FileModified asks to upload a file changed in its application back
// (events.SFTPFileModified).
func (p *Panel) FileModified(localPath, remotePath string) {
	p.backLocal, p.backRemote = localPath, remotePath
}

func (p *Panel) title() string {
	if p.B.Backend.Kind() == KindLocal {
		return "File Browser"
	}
	return "SFTP Browser"
}

// Layout draws the panel into the constraints.
func (p *Panel) Layout(gtx layout.Context, th *gtk.Theme) layout.Dimensions {
	p.init()
	st := p.B.Snapshot()
	p.actions(gtx, st)
	pal := th.Pal
	gtk.Fill(gtx, pal.BgColor)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.header(gtx, th) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.toolbar(gtx, th, st) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.pathBar(gtx, th, st) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if st.Err == "" {
				return layout.Dimensions{}
			}
			return bar(gtx, pal.ErrorBg, pal.ErrorBorder, layout.Inset{Top: 4, Bottom: 4, Left: 8, Right: 8}, func(gtx layout.Context) layout.Dimensions {
				return gtk.Text{S: st.Err, Size: 11, Color: pal.ErrorFg, MaxLines: 1}.Layout(gtx, th)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return p.fileList(gtx, th, st) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.queue(gtx, th, st) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.footer(gtx, th, st) }),
	)
}

// bar fills the full width with bg, draws w with padding and a bottom
// border.
func bar(gtx layout.Context, bg, border color.NRGBA, in layout.Inset, w layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	m := op.Record(gtx.Ops)
	d := in.Layout(gtx, w)
	call := m.Stop()
	d.Size.X = gtx.Constraints.Max.X
	d.Size.Y += max(1, gtx.Dp(1))
	gtk.FillRect(gtx, image.Rectangle{Max: d.Size}, bg)
	call.Add(gtx.Ops)
	gtk.FillRect(gtx, image.Rect(0, d.Size.Y-max(1, gtx.Dp(1)), d.Size.X, d.Size.Y), border)
	return d
}

// header: the title in bold 12px and × (padding 4px 8px).
func (p *Panel) header(gtx layout.Context, th *gtk.Theme) layout.Dimensions {
	pal := th.Pal
	if p.close.Clicked(gtx) && p.OnClose != nil {
		p.OnClose()
	}
	return bar(gtx, pal.HeaderBg, pal.BorderColor, layout.Inset{Top: 4, Bottom: 4, Left: 8, Right: 8}, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, gtk.FillWidth(gtk.Text{S: p.title(), Size: 12, Weight: font.Bold, Color: pal.FgColor}.Widget(th))),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.close.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					c := pal.MutedFg
					if p.close.Hovered() {
						gtk.FillRect(gtx, image.Rect(0, 0, gtx.Dp(16), gtx.Dp(18)), pal.RowHover)
						c = pal.FgColor
					}
					return layout.Inset{Left: 4, Right: 4}.Layout(gtx, gtk.Text{S: "×", Size: 16, Color: c}.Widget(th))
				})
			}),
		)
	})
}

// cp is a .button-cp toolbar button: transparent until hovered, padding
// 3px 5px, 40% opacity when disabled.
func cp(gtx layout.Context, th *gtk.Theme, c *widget.Clickable, icon rune, iconColor color.NRGBA, disabled, active bool) layout.Dimensions {
	pal := th.Pal
	if disabled {
		gtx = gtx.Disabled()
		iconColor.A = 102
	}
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		d := layout.Inset{Top: 4, Bottom: 4, Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return th.IconBox(gtx, icon, 16, iconColor)
		})
		call := m.Stop()
		if !disabled && (c.Hovered() || active) {
			r := image.Rectangle{Max: d.Size}
			paint.FillShape(gtx.Ops, pal.BorderColor, clip.UniformRRect(r, gtx.Dp(3)).Op(gtx.Ops))
			paint.FillShape(gtx.Ops, pal.RowHover, clip.UniformRRect(r.Inset(max(1, gtx.Dp(1))), gtx.Dp(2)).Op(gtx.Ops))
		}
		call.Add(gtx.Ops)
		if !disabled {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		return d
	})
}

// iconGray is the light gray of the web GUI's outline icons.
func iconGray(pal *gtk.Palette) color.NRGBA {
	c := pal.FgColor
	c.A = 0xd0
	return c
}

// historyButton is .path-dropdown-btn: a square joined to the entry with
// a 9px ▼ (the web view's button colors fall back to #e8e8e8 there).
func (p *Panel) historyButton(gtx layout.Context, th *gtk.Theme) layout.Dimensions {
	return p.histBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		sz := image.Pt(gtx.Dp(22), gtx.Dp(26))
		bg := color.NRGBA{R: 0xe8, G: 0xe8, B: 0xe8, A: 0xff}
		if p.histBtn.Hovered() || p.menu.Opened() && p.menuKind == "history" {
			bg = color.NRGBA{R: 0xd8, G: 0xd8, B: 0xd8, A: 0xff}
		}
		r := image.Rectangle{Max: sz}
		paint.FillShape(gtx.Ops, th.Pal.BorderColor, clip.RRect{Rect: r, NE: gtx.Dp(3), SE: gtx.Dp(3)}.Op(gtx.Ops))
		in := max(1, gtx.Dp(1))
		paint.FillShape(gtx.Ops, bg, clip.RRect{Rect: image.Rect(0, in, sz.X-in, sz.Y-in), NE: gtx.Dp(2), SE: gtx.Dp(2)}.Op(gtx.Ops))
		g := gtx
		g.Constraints = layout.Exact(sz)
		layout.Center.Layout(g, gtk.Text{S: "▼", Size: 9, Color: color.NRGBA{R: 0x55, G: 0x57, B: 0x53, A: 0xff}}.Widget(th))
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: sz}
	})
}

// toolbar: back, forward, up, home, refresh | new folder, upload, download
// | ≡ (SftpToolbar.svelte).
func (p *Panel) toolbar(gtx layout.Context, th *gtk.Theme, st State) layout.Dimensions {
	pal := th.Pal
	sftp := p.B.Backend.Kind() == KindSFTP
	fg := iconGray(pal)
	sep := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: 3, Right: 3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			h := gtx.Dp(18)
			gtk.FillRect(gtx, image.Rect(0, 0, max(1, gtx.Dp(1)), h), pal.BorderColor)
			return layout.Dimensions{Size: image.Pt(max(1, gtx.Dp(1)), h)}
		})
	})
	gap := layout.Rigid(layout.Spacer{Width: 3}.Layout)
	b := func(c *widget.Clickable, icon rune, col color.NRGBA, disabled bool) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions { return cp(gtx, th, c, icon, col, disabled, false) })
	}
	_, canUp := ParentPath(st.Current, st.Win)
	kids := []layout.FlexChild{
		b(&p.back, fonts.IconChevLeft, fg, st.HistoryIdx <= 0), gap,
		b(&p.fwd, fonts.IconChevRight, fg, st.HistoryIdx >= len(st.History)-1), gap,
		b(&p.upBtn, fonts.IconChevUp, fg, !canUp), gap,
		b(&p.home, fonts.IconHomeO, fg, false), gap,
		b(&p.refresh, fonts.IconRedo, fg, false), sep,
		b(&p.newDir, fonts.IconFolderO, fg, false),
	}
	if sftp {
		kids = append(kids, gap, b(&p.upl, fonts.IconUpload, fg, false), gap, b(&p.dl, fonts.IconDownload, fg, len(st.Selected) == 0))
	}
	kids = append(kids, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
	}), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return cp(gtx, th, &p.more, fonts.IconBars, fg, false, p.menu.Opened() && p.menuKind == "more")
	}))
	return bar(gtx, pal.HeaderBg, pal.BorderColor, layout.Inset{Top: 3, Bottom: 3, Left: 6, Right: 6}, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
	})
}

// pathBar: the path entry (monospace 12px) with the history button, and
// "Follow terminal folder" (SftpPathBar.svelte).
func (p *Panel) pathBar(gtx layout.Context, th *gtk.Theme, st State) layout.Dimensions {
	pal := th.Pal
	if p.pathFor != st.Current && !gtx.Focused(&p.pathEd) {
		p.pathEd.SetText(st.Current)
		p.pathFor = st.Current
	}
	label := "Follow terminal folder"
	if p.B.Backend.Kind() == KindSFTP {
		label += " (OSC 7)"
	}
	p.follow.Value = st.Follow
	return bar(gtx, pal.BgColor, pal.BorderColor, layout.Inset{Top: 4, Bottom: 4, Left: 6, Right: 6}, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return gtk.Entry{Width: -1, Mono: true, Size: 12}.Layout(gtx, th, &p.pathEd)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.historyButton(gtx, th) }),
				)
			}),
			layout.Rigid(layout.Spacer{Height: 4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return th.CheckButton(gtx, &p.follow, label, false)
			}),
		)
	})
}

// columns: Size 60px, Modified 105px, Perm. 80px; Name takes the rest.
var colWidths = [4]unit.Dp{0, 60, 105, 80}

func (p *Panel) columns(gtx layout.Context, cells [4]layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return cells[0](gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return fixed(gtx, colWidths[1], cells[1]) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return fixed(gtx, colWidths[2], cells[2]) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return fixed(gtx, colWidths[3], cells[3]) }),
	)
}

func fixed(gtx layout.Context, w unit.Dp, cell layout.Widget) layout.Dimensions {
	px := gtx.Dp(w)
	gtx.Constraints.Min.X, gtx.Constraints.Max.X = px, px
	d := cell(gtx)
	d.Size.X = px
	return d
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04")
}

// fileList is SftpFileList.svelte: the sortable header and 22px rows.
func (p *Panel) fileList(gtx layout.Context, th *gtk.Theme, st State) layout.Dimensions {
	pal := th.Pal
	entries := Sorted(st.Entries, p.sortBy, p.sortAsc)
	p.listKeys(gtx, st, entries)
	size := gtx.Constraints.Max
	gtk.Fill(gtx, pal.BaseColor)
	// The list's own area (focus, keys, context clicks on empty space)
	// comes before the rows, so these are its children.
	area := clip.Rect{Max: size}.Push(gtx.Ops)
	event.Op(gtx.Ops, &p.listTag)
	area.Pop()
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &p.listTag, Kinds: pointer.Press})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok {
			gtx.Execute(key.FocusCmd{Tag: &p.listTag})
			if e.Buttons.Contain(pointer.ButtonSecondary) {
				p.openMenu("empty", nil, st)
			}
		}
	}

	header := func(gtx layout.Context) layout.Dimensions {
		labels := [4]string{"Name", "Size", "Modified", "Perm."}
		cols := [4]SortBy{SortName, SortSize, SortMod, SortPerms}
		var cells [4]layout.Widget
		for i := range cells {
			i := i
			if p.heads[i].Clicked(gtx) {
				if p.sortBy == cols[i] {
					p.sortAsc = !p.sortAsc
				} else {
					p.sortBy, p.sortAsc = cols[i], true
				}
			}
			cells[i] = func(gtx layout.Context) layout.Dimensions {
				return p.heads[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					d := layout.Inset{Top: 4, Bottom: 4, Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						s := labels[i]
						if p.sortBy == cols[i] {
							s += map[bool]string{true: " ▲", false: " ▼"}[p.sortAsc]
						}
						return layout.N.Layout(gtx, gtk.Text{S: s, Size: 11, Weight: font.SemiBold, Color: pal.FgColor, MaxLines: 1}.Widget(th))
					})
					d.Size.X = gtx.Constraints.Max.X
					gtk.FillRect(gtx, image.Rect(d.Size.X-max(1, gtx.Dp(1)), 0, d.Size.X, d.Size.Y), pal.BorderColor)
					return d
				})
			}
		}
		return bar(gtx, pal.HeaderBg, pal.BorderColor, layout.Inset{}, func(gtx layout.Context) layout.Dimensions {
			return p.columns(gtx, cells)
		})
	}

	_, hasParent := ParentPath(st.Current, st.Win)
	n := len(entries)
	if hasParent {
		n++
	}
	body := func(gtx layout.Context) layout.Dimensions {
		if len(entries) == 0 && !st.Loading {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !hasParent {
						return layout.Dimensions{}
					}
					return p.parentRow(gtx, th)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(24).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.N.Layout(gtx, gtk.Text{S: "Directory is empty", Size: 12, Color: pal.MutedFg}.Widget(th))
					})
				}),
			)
		}
		return p.list.Layout(gtx, n, func(gtx layout.Context, i int) layout.Dimensions {
			if hasParent {
				if i == 0 {
					return p.parentRow(gtx, th)
				}
				i--
			}
			return p.row(gtx, th, st, entries, entries[i])
		})
	}
	d := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(header),
		layout.Flexed(1, body),
	)
	if st.Loading {
		overlay(gtx, th, size, "Loading...", 12, false)
	}
	if p.Connected != nil && !p.Connected() {
		overlay(gtx, th, size, "Disconnected", 14, true)
	}
	return d
}

// overlay covers the list with the background at 85% and a centered text.
func overlay(gtx layout.Context, th *gtk.Theme, size image.Point, s string, sz unit.Sp, bold bool) {
	bg := th.Pal.BgColor
	bg.A = 217
	gtk.FillRect(gtx, image.Rectangle{Max: size}, bg)
	g := gtx
	g.Constraints = layout.Exact(size)
	w := font.Normal
	if bold {
		w = font.Bold
	}
	layout.Center.Layout(g, gtk.Text{S: s, Size: sz, Weight: w, Color: th.Pal.MutedFg}.Widget(th))
}

func (p *Panel) rowFrame(gtx layout.Context, th *gtk.Theme, c *widget.Clickable, selected, broken bool, cells [4]layout.Widget) layout.Dimensions {
	pal := th.Pal
	h := gtx.Dp(22)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		switch {
		case selected:
			gtk.FillRect(gtx, image.Rect(0, 0, w, h), pal.SelectedBg)
		case c.Hovered():
			gtk.FillRect(gtx, image.Rect(0, 0, w, h), pal.RowHover)
		}
		gtk.FillRect(gtx, image.Rect(0, h-max(1, gtx.Dp(1)), w, h), pal.BorderColor)
		g := gtx
		g.Constraints = layout.Exact(image.Pt(w, h))
		layout.W.Layout(g, func(gtx layout.Context) layout.Dimensions { return p.columns(gtx, cells) })
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: image.Pt(w, h)}
	})
}

func cellText(th *gtk.Theme, s string, c color.NRGBA, mono bool, size unit.Sp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: 6, Right: 6}.Layout(gtx, gtk.Text{S: s, Size: size, Color: c, Mono: mono, MaxLines: 1}.Widget(th))
	}
}

func (p *Panel) nameCell(th *gtk.Theme, e sftpsvc.FileEntry, fg color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			icon := fonts.IconFileO
			if e.IsDir {
				icon = fonts.IconFolderO
			}
			ic := fg
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					d := th.IconBox(gtx, icon, 16, ic)
					if e.IsLink {
						o := op.Offset(image.Pt(gtx.Dp(8), gtx.Dp(7))).Push(gtx.Ops)
						th.Icon(gtx, fonts.IconLinkArrow, 8, fg)
						o.Pop()
					}
					return d
				}),
				layout.Rigid(layout.Spacer{Width: 6}.Layout),
				layout.Flexed(1, gtk.FillWidth(gtk.Text{S: e.Name, Size: 11, Color: fg, MaxLines: 1}.Widget(th))),
			)
		})
	}
}

func (p *Panel) parentRow(gtx layout.Context, th *gtk.Theme) layout.Dimensions {
	for {
		c, ok := p.up.Update(gtx)
		if !ok {
			break
		}
		if c.NumClicks >= 2 {
			p.B.Up()
		}
	}
	fg := th.Pal.FgColor
	none := func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }
	return p.rowFrame(gtx, th, &p.up, false, false, [4]layout.Widget{
		p.nameCell(th, sftpsvc.FileEntry{Name: "..", IsDir: true}, fg), none, none, none,
	})
}

func (p *Panel) row(gtx layout.Context, th *gtk.Theme, st State, entries []sftpsvc.FileEntry, e sftpsvc.FileEntry) layout.Dimensions {
	pal := th.Pal
	rs := p.rows[e.Path]
	if rs == nil {
		rs = &rowState{}
		p.rows[e.Path] = rs
	}
	selected := contains(st.Selected, e.Path)
	for {
		c, ok := rs.click.Update(gtx)
		if !ok {
			break
		}
		gtx.Execute(key.FocusCmd{Tag: &p.listTag})
		switch {
		case c.NumClicks >= 2:
			p.activate(e)
		case c.Modifiers.Contain(key.ModShortcut):
			if selected {
				p.B.SetSelected(remove(st.Selected, e.Path))
			} else {
				p.B.SetSelected(append(st.Selected, e.Path))
			}
		case c.Modifiers.Contain(key.ModShift) && len(st.Selected) > 0:
			from, to := index(entries, st.Selected[len(st.Selected)-1]), index(entries, e.Path)
			if from >= 0 && to >= 0 {
				sel := append([]string(nil), st.Selected...)
				for i := min(from, to); i <= max(from, to); i++ {
					if !contains(sel, entries[i].Path) {
						sel = append(sel, entries[i].Path)
					}
				}
				p.B.SetSelected(sel)
			}
		default:
			p.B.SetSelected([]string{e.Path})
		}
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &rs.area, Kinds: pointer.Press})
		if !ok {
			break
		}
		if pe, ok := ev.(pointer.Event); ok && pe.Buttons.Contain(pointer.ButtonSecondary) {
			if !selected {
				p.B.SetSelected([]string{e.Path})
			}
			ee := e
			p.openMenu("item", &ee, st)
		}
	}
	fg, muted := pal.FgColor, pal.FgColor
	if selected {
		fg, muted = pal.SelectedFg, pal.SelectedFg
	}
	if e.LinkBroken {
		fg.A, muted.A = 128, 128
	}
	size := ""
	if !e.IsDir {
		size = FormatBytes(e.Size)
	}
	d := p.rowFrame(gtx, th, &rs.click, selected, e.LinkBroken, [4]layout.Widget{
		p.nameCell(th, e, fg),
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.E.Layout(gtx, gtk.Text{S: size, Size: 11, Color: muted, MaxLines: 1}.Widget(th))
			})
		},
		cellText(th, formatDate(e.ModTime), muted, false, 11),
		cellText(th, e.Permissions, muted, true, 10),
	})
	// The row's own area for right clicks, over the clickable.
	a := clip.Rect{Max: d.Size}.Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &rs.area)
	pass.Pop()
	a.Pop()
	return d
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func remove(s []string, v string) []string {
	var out []string
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

func index(es []sftpsvc.FileEntry, path string) int {
	for i, e := range es {
		if e.Path == path {
			return i
		}
	}
	return -1
}

// activate opens a folder, or a file in its application.
func (p *Panel) activate(e sftpsvc.FileEntry) {
	if e.IsDir {
		p.B.Navigate(e.Path)
		return
	}
	if p.OpenInEditor != nil && p.OpensInEditor != nil && p.OpensInEditor(e.Path) {
		p.OpenInEditor(e.Path)
		return
	}
	p.B.OpenWithDefaultApp(e.Path)
}

// listKeys: F5, F2, F7, Delete, Enter, Backspace / Alt+Up and type-ahead.
func (p *Panel) listKeys(gtx layout.Context, st State, entries []sftpsvc.FileEntry) {
	for {
		ev, ok := gtx.Event(
			key.FocusFilter{Target: &p.listTag},
			key.Filter{Focus: &p.listTag, Name: "", Optional: key.ModShift | key.ModAlt},
		)
		if !ok {
			break
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		switch {
		case e.Name == key.NameF5:
			p.B.Refresh()
		case e.Name == key.NameF2 && len(st.Selected) == 1:
			p.openNameDialog("rename", BaseName(st.Selected[0]), st.Selected)
		case e.Name == key.NameF7:
			p.openNameDialog("newdir", "", nil)
		case e.Name == key.NameDeleteForward && len(st.Selected) > 0:
			p.askDelete(st.Selected)
		case e.Name == key.NameReturn && len(st.Selected) == 1:
			if i := index(entries, st.Selected[0]); i >= 0 {
				p.activate(entries[i])
			}
		case e.Name == key.NameDeleteBackward, e.Name == key.NameUpArrow && e.Modifiers.Contain(key.ModAlt):
			p.B.Up()
		case len([]rune(string(e.Name))) == 1 && !e.Modifiers.Contain(key.ModAlt):
			if gtx.Now.Sub(p.typedAt) > 700*time.Millisecond {
				p.typed = ""
			}
			p.typed += strings.ToLower(string(e.Name))
			p.typedAt = gtx.Now
			for _, en := range entries {
				if strings.HasPrefix(strings.ToLower(en.Name), p.typed) {
					p.B.SetSelected([]string{en.Path})
					break
				}
			}
		}
	}
}

// queue is SftpTransferQueue.svelte (collapsible).
func (p *Panel) queue(gtx layout.Context, th *gtk.Theme, st State) layout.Dimensions {
	pal := th.Pal
	if len(st.Transfers) == 0 {
		return layout.Dimensions{}
	}
	if p.queueHead.Clicked(gtx) {
		p.queueOpen = !p.queueOpen
	}
	running := 0
	for _, t := range st.Transfers {
		if t.Status == "running" {
			running++
		}
	}
	gtk.FillRect(gtx, image.Rect(0, 0, gtx.Constraints.Max.X, max(1, gtx.Dp(1))), pal.BorderColor)
	kids := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return p.queueHead.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			arrow := "▶"
			if p.queueOpen {
				arrow = "▼"
			}
			return layout.Inset{Top: 4, Bottom: 4, Left: 8, Right: 8}.Layout(gtx,
				gtk.Text{S: fmt.Sprintf("%s  Transfers (%d running, %d total)", arrow, running, len(st.Transfers)), Size: 11, Weight: font.SemiBold, Color: pal.FgColor}.Widget(th))
		})
	})}
	if p.queueOpen {
		for _, t := range st.Transfers {
			t := t
			btn := p.queueBtns[t.ID]
			if btn == nil {
				btn = new(widget.Clickable)
				p.queueBtns[t.ID] = btn
			}
			if btn.Clicked(gtx) {
				if t.Status == "running" {
					p.B.Cancel(t.ID)
				} else {
					p.B.Dismiss(t.ID)
				}
			}
			kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 3, Bottom: 3, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					arrow := map[string]string{"upload": "⬆", "download": "⬇", "move": "➜", "copy": "⧉"}[t.Direction]
					status := t.Status
					if status == "running" {
						status = t.Speed
					}
					sc := map[string]color.NRGBA{"completed": pal.SuccessFg, "failed": pal.ErrorFg}[t.Status]
					if sc.A == 0 {
						sc = pal.MutedFg
					}
					rows := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(gtk.Text{S: arrow + " ", Size: 11, Color: pal.FgColor}.Widget(th)),
							layout.Flexed(1, gtk.FillWidth(gtk.Text{S: t.Name, Size: 11, Color: pal.FgColor, MaxLines: 1}.Widget(th))),
							layout.Rigid(gtk.Text{S: status, Size: 10, Color: sc, MaxLines: 1}.Widget(th)),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: 6}.Layout(gtx, gtk.Text{S: "✕", Size: 11, Color: pal.MutedFg}.Widget(th))
								})
							}),
						)
					})}
					if t.Status == "running" {
						rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: 3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return th.ProgressBar(gtx, float32(t.Progress)/100, unit.Dp(float32(gtx.Constraints.Max.X)/gtx.Metric.PxPerDp))
							})
						}))
					}
					if t.Err != "" {
						rows = append(rows, layout.Rigid(gtk.Text{S: t.Err, Size: 10, Color: pal.ErrorFg, MaxLines: 1}.Widget(th)))
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
				})
			}))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
}

// footer: items, selection and free space (30px, 11px muted).
func (p *Panel) footer(gtx layout.Context, th *gtk.Theme, st State) layout.Dimensions {
	pal := th.Pal
	s := fmt.Sprintf("%d items", len(st.Entries))
	if n := len(st.Selected); n > 0 {
		var bytes int64
		for _, e := range st.Entries {
			if !e.IsDir && contains(st.Selected, e.Path) {
				bytes += e.Size
			}
		}
		s += fmt.Sprintf(", %d selected (%s)", n, FormatBytes(bytes))
	}
	if st.VFS != nil && st.VFS.FreeSpace > 0 {
		s += " · free: " + FormatBytes(int64(st.VFS.FreeSpace))
	}
	w, h := gtx.Constraints.Max.X, gtx.Dp(30)
	gtk.FillRect(gtx, image.Rect(0, 0, w, h), pal.HeaderBg)
	gtk.FillRect(gtx, image.Rect(0, 0, w, max(1, gtx.Dp(1))), pal.BorderColor)
	g := gtx
	g.Constraints = layout.Exact(image.Pt(w, h))
	layout.Inset{Left: 6, Right: 6}.Layout(g, func(gtx layout.Context) layout.Dimensions {
		return layout.W.Layout(gtx, gtk.Text{S: s, Size: 11, Color: pal.MutedFg, MaxLines: 1}.Widget(th))
	})
	return layout.Dimensions{Size: image.Pt(w, h)}
}

// ---- actions ------------------------------------------------------------------------

func (p *Panel) actions(gtx layout.Context, st State) {
	b := p.B
	if p.back.Clicked(gtx) {
		b.Back()
	}
	if p.fwd.Clicked(gtx) {
		b.Forward()
	}
	if p.upBtn.Clicked(gtx) {
		b.Up()
	}
	if p.home.Clicked(gtx) {
		b.Home()
	}
	if p.refresh.Clicked(gtx) {
		b.Refresh()
	}
	if p.newDir.Clicked(gtx) {
		p.openNameDialog("newdir", "", nil)
	}
	if p.upl.Clicked(gtx) {
		p.upload(st)
	}
	if p.dl.Clicked(gtx) && len(st.Selected) > 0 {
		b.Download(st.Selected, "")
	}
	if p.more.Clicked(gtx) {
		p.openMenu("more", nil, st)
	}
	if p.histBtn.Clicked(gtx) {
		p.openMenu("history", nil, st)
	}
	if p.follow.Update(gtx) {
		b.SetFollow(p.follow.Value)
	}
	for {
		ev, ok := p.pathEd.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			b.Navigate(p.pathEd.Text())
			p.pathFor = ""
		}
	}
	for {
		ev, ok := gtx.Event(key.Filter{Focus: &p.pathEd, Name: key.NameEscape})
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			p.pathEd.SetText(st.Current) // Esc reverts
		}
	}
}

func (p *Panel) upload(st State) {
	if p.ChooseFile == nil {
		return
	}
	go func() {
		path, err := p.ChooseFile("Upload file", "", false)
		if err != nil {
			p.B.fail(err)
			return
		}
		if path != "" {
			p.transfer("upload", []string{path}, st.Current)
		}
	}()
}

func (p *Panel) copyOrMoveTo(op string, paths []string, st State) {
	if p.ChooseFile == nil || len(paths) == 0 {
		return
	}
	title := map[string]string{"copy": "Copy to folder", "move": "Move to folder"}[op]
	go func() {
		dir, err := p.ChooseFile(title, st.Current, true)
		if err != nil {
			p.B.fail(err)
			return
		}
		if dir != "" {
			p.transfer(op, paths, dir)
		}
	}()
}

// transfer uploads (SFTP) or copies/moves (local) after asking about
// names that already exist in dstDir.
func (p *Panel) transfer(op string, paths []string, dstDir string) {
	run := func(paths []string, overwrite bool) {
		if op == "upload" {
			p.B.Upload(paths)
		} else {
			p.B.CopyOrMove(op, paths, dstDir, overwrite)
		}
	}
	names := p.B.Conflicts(paths, dstDir)
	if len(names) == 0 {
		run(paths, false)
		return
	}
	p.paths = names
	p.pending = func(overwrite, skip bool) {
		if skip {
			var keep []string
			for _, q := range paths {
				if !contains(names, BaseName(q)) {
					keep = append(keep, q)
				}
			}
			paths = keep
		}
		if len(paths) > 0 {
			run(paths, overwrite)
		}
	}
	p.dlg = "conflict"
	p.dlgBox = gtk.Dialog{Title: "File already exists", Width: 420, Closable: true}
	if p.B.Changed != nil {
		p.B.Changed()
	}
}

// Drop handles files dropped on the panel: uploads (SFTP) or copies
// (local) into the current folder.
func (p *Panel) Drop(paths []string) {
	st := p.B.Snapshot()
	if len(paths) == 0 || st.Current == "" {
		return
	}
	op := "copy"
	if p.B.Backend.Kind() == KindSFTP {
		if p.Connected != nil && !p.Connected() {
			return
		}
		op = "upload"
	}
	go p.transfer(op, paths, st.Current)
}

func (p *Panel) openMenu(kind string, target *sftpsvc.FileEntry, st State) {
	sftp := p.B.Backend.Kind() == KindSFTP
	var items []gtk.MenuItem
	switch kind {
	case "more":
		start := "Open start folder"
		if sftp {
			start = "Open SFTP start folder"
		}
		hide := "Hide file browser"
		if sftp {
			hide = "Hide SFTP browser"
		}
		items = []gtk.MenuItem{
			{ID: "hidden", Label: "Show hidden files", Checked: st.ShowHidden},
			{ID: "follow", Label: "Follow terminal folder", Checked: st.Follow},
			{ID: "goterm", Label: "Go to terminal folder"},
			{ID: "start", Label: start},
			{ID: "copycur", Label: "Copy current path"},
			{Separator: true},
			{ID: "hide", Label: hide},
		}
	case "history":
		seen := map[string]bool{}
		for i := len(st.History) - 1; i >= 0; i-- {
			h := st.History[i]
			if seen[h] {
				continue
			}
			seen[h] = true
			items = append(items, gtk.MenuItem{ID: "goto:" + h, Label: h, Icon: fonts.IconFolder, Checked: h == st.Current})
		}
		if len(items) == 0 {
			items = []gtk.MenuItem{{ID: "none", Label: "No history", Disabled: true}}
		}
	case "item":
		e := target
		if e.IsDir {
			items = append(items, gtk.MenuItem{ID: "open", Label: "Open"})
		} else {
			if p.OpenInEditor != nil {
				items = append(items, gtk.MenuItem{ID: "openeditor", Label: "Open in Editor"})
			}
			items = append(items, gtk.MenuItem{ID: "openapp", Label: "Open with default application"})
		}
		if sftp {
			items = append(items, gtk.MenuItem{ID: "download", Label: "Download"})
		} else {
			items = append(items, gtk.MenuItem{ID: "copyto", Label: "Copy to…"}, gtk.MenuItem{ID: "moveto", Label: "Move to…"})
		}
		items = append(items,
			gtk.MenuItem{Separator: true},
			gtk.MenuItem{ID: "rename", Label: "Rename (F2)"},
			gtk.MenuItem{ID: "delete", Label: "Delete (Del)"},
			gtk.MenuItem{ID: "newdir", Label: "New folder (F7)"},
			gtk.MenuItem{ID: "newfile", Label: "New empty file"},
		)
		if !st.Win {
			items = append(items, gtk.MenuItem{ID: "perms", Label: "Permissions..."})
		}
		items = append(items, gtk.MenuItem{Separator: true},
			gtk.MenuItem{ID: "copypath", Label: "Copy path"},
			gtk.MenuItem{ID: "pathterm", Label: "Copy path to terminal"})
		if e.IsDir {
			items = append(items, gtk.MenuItem{ID: "cdterm", Label: "cd to this folder in terminal"})
		}
		items = append(items, gtk.MenuItem{Separator: true}, gtk.MenuItem{ID: "refresh", Label: "Refresh (F5)"})
	case "empty":
		items = []gtk.MenuItem{
			{ID: "newdir", Label: "New folder"},
			{ID: "newfile", Label: "New empty file"},
		}
		if sftp {
			items = append(items, gtk.MenuItem{ID: "upload", Label: "Upload files..."})
		}
		items = append(items, gtk.MenuItem{Separator: true},
			gtk.MenuItem{ID: "refresh", Label: "Refresh (F5)"},
			gtk.MenuItem{ID: "hidden", Label: "Show hidden files", Checked: st.ShowHidden})
	}
	p.menuKind, p.menuTarget = kind, target
	p.menu.Items = items
	pos := image.Point{}
	if p.PointerPos != nil {
		pos = p.PointerPos()
	}
	p.menu.Open(pos)
}

func (p *Panel) quote(s string, win bool) string {
	if win {
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (p *Panel) menuAction(gtx layout.Context) {
	id, ok := p.menu.Activated()
	if !ok {
		return
	}
	st := p.B.Snapshot()
	b := p.B
	e := p.menuTarget
	copyText := func(s string) {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(s))})
	}
	switch {
	case strings.HasPrefix(id, "goto:"):
		b.Navigate(strings.TrimPrefix(id, "goto:"))
	case id == "hidden":
		b.ToggleHidden()
	case id == "follow":
		b.SetFollow(!st.Follow)
	case id == "goterm":
		b.GoToTerminalFolder()
	case id == "start":
		b.Navigate(st.Start)
	case id == "copycur":
		copyText(st.Current)
	case id == "hide":
		if p.OnClose != nil {
			p.OnClose()
		}
	case id == "open" && e != nil:
		b.Navigate(e.Path)
	case id == "openeditor" && e != nil && p.OpenInEditor != nil:
		p.OpenInEditor(e.Path)
	case id == "openapp" && e != nil:
		b.OpenWithDefaultApp(e.Path)
	case id == "download":
		b.Download(st.Selected, "")
	case id == "copyto":
		p.copyOrMoveTo("copy", st.Selected, st)
	case id == "moveto":
		p.copyOrMoveTo("move", st.Selected, st)
	case id == "rename" && e != nil:
		p.openNameDialog("rename", e.Name, []string{e.Path})
	case id == "delete":
		p.askDelete(st.Selected)
	case id == "newdir":
		p.openNameDialog("newdir", "", nil)
	case id == "newfile":
		p.openNameDialog("newfile", "", nil)
	case id == "perms":
		p.openPerms(st.Selected, st)
	case id == "copypath" && e != nil:
		copyText(e.Path)
	case id == "pathterm" && e != nil && p.TerminalInput != nil:
		p.TerminalInput(p.quote(e.Path, st.Win) + " ")
	case id == "cdterm" && e != nil && p.TerminalInput != nil:
		if st.Win {
			p.TerminalInput("pushd " + p.quote(e.Path, true) + "\r")
		} else {
			p.TerminalInput("cd " + p.quote(e.Path, false) + "\n")
		}
	case id == "refresh":
		b.Refresh()
	case id == "upload":
		p.upload(st)
	}
}

// ---- dialogs ------------------------------------------------------------------------

func (p *Panel) openNameDialog(kind, name string, paths []string) {
	titles := map[string]string{"newdir": "New Folder", "newfile": "New Empty File", "rename": "Rename"}
	p.dlg, p.paths = kind, paths
	p.dlgBox = gtk.Dialog{Title: titles[kind], Width: 340, Closable: true}
	p.nameEd.SingleLine, p.nameEd.Submit = true, true
	p.nameEd.SetText(name)
	p.nameEd.SetCaret(len(name), 0)
}

func (p *Panel) askDelete(paths []string, confirm ...bool) {
	if len(paths) == 0 {
		return
	}
	p.paths = paths
	text := fmt.Sprintf("Delete %d items? This cannot be undone.", len(paths))
	if len(paths) == 1 {
		text = fmt.Sprintf("Delete '%s'? This cannot be undone.", BaseName(paths[0]))
	}
	p.dlg = "delete"
	p.msg = gtk.MessageDialog{Type: gtk.MessageWarning, Title: "Confirm Delete", Primary: text, Buttons: gtk.ButtonsYesNo}
}

// ConfirmDelete is sftp_browser.confirm_delete (delete without asking when
// false).
var ConfirmDelete = func() bool { return true }

// LayoutOverlays draws the panel's menu and dialogs over the window.
func (p *Panel) LayoutOverlays(gtx layout.Context, th *gtk.Theme) {
	p.init()
	p.menuAction(gtx)
	p.menu.Layout(gtx, th)

	p.errMu.Lock()
	errMsg := p.errMsg
	p.errMu.Unlock()
	if errMsg != "" && p.dlg == "" {
		p.dlg = "error"
		p.msg = gtk.MessageDialog{Type: gtk.MessageError, Title: p.title(), Secondary: errMsg, Buttons: gtk.ButtonsOK}
	}
	if p.backRemote != "" && p.dlg == "" {
		p.dlg = "uploadback"
		p.msg = gtk.MessageDialog{Type: gtk.MessageQuestion, Title: "File Modified Externally", Primary: "Upload changes back to server?",
			Secondary: fmt.Sprintf("Remote file '%s' was modified externally. Do you want to upload the saved changes back?", p.backRemote), Buttons: gtk.ButtonsYesNo}
	}
	if p.dlg == "delete" && !ConfirmDelete() {
		p.B.Remove(p.paths)
		p.dlg = ""
	}

	switch p.dlg {
	case "newdir", "newfile", "rename":
		p.nameDialog(gtx, th)
	case "delete", "error", "uploadback":
		p.msg.Layout(gtx, th)
		if r, ok := p.msg.Response(); ok {
			switch p.dlg {
			case "delete":
				if r == "yes" {
					p.B.Remove(p.paths)
				}
			case "error":
				p.errMu.Lock()
				p.errMsg = ""
				p.errMu.Unlock()
			case "uploadback":
				if r == "yes" {
					p.B.UploadBack(p.backLocal, p.backRemote)
				}
				p.backLocal, p.backRemote = "", ""
			}
			p.dlg = ""
		}
	case "perms":
		p.permsDialog(gtx, th)
	case "conflict":
		p.conflictDialog(gtx, th)
	}
}

func (p *Panel) nameDialog(gtx layout.Context, th *gtk.Theme) {
	labels := map[string]string{"newdir": "Folder name:", "newfile": "File name:", "rename": "New name:"}
	oks := map[string]string{"newdir": "Create", "newfile": "Create", "rename": "Rename"}
	submit := false
	for {
		ev, ok := p.nameEd.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			submit = true
		}
	}
	if p.ok.Clicked(gtx) {
		submit = true
	}
	if p.cancel.Clicked(gtx) || p.dlgBox.CloseRequested() {
		p.dlg = ""
		return
	}
	if submit {
		name := strings.TrimSpace(p.nameEd.Text())
		if name == "" {
			return
		}
		switch p.dlg {
		case "newdir":
			p.B.CreateFolder(name)
		case "newfile":
			p.B.CreateFile(name)
		case "rename":
			if len(p.paths) == 1 {
				p.B.Rename(p.paths[0], name)
			}
		}
		p.dlg = ""
		return
	}
	gtx.Execute(key.FocusCmd{Tag: &p.nameEd})
	pal := th.Pal
	kind := p.dlg
	p.dlgBox.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(gtk.Text{S: labels[kind], Size: 12, Color: pal.FgColor}.Widget(th)),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return gtk.Entry{Width: -1}.Layout(gtx, th, &p.nameEd)
			}),
		)
	},
		func(gtx layout.Context) layout.Dimensions {
			return gtk.Button{Label: "Cancel"}.Layout(gtx, th, &p.cancel)
		},
		func(gtx layout.Context) layout.Dimensions {
			return gtk.Button{Label: oks[kind], Default: true}.Layout(gtx, th, &p.ok)
		},
	)
}

func (p *Panel) conflictDialog(gtx layout.Context, th *gtk.Theme) {
	answer := func(overwrite, skip bool) {
		f := p.pending
		p.dlg, p.pending = "", nil
		if f != nil {
			go f(overwrite, skip)
		}
	}
	switch {
	case p.cancel.Clicked(gtx), p.dlgBox.CloseRequested():
		p.dlg, p.pending = "", nil
		return
	case p.skip.Clicked(gtx):
		answer(false, true)
		return
	case p.ok.Clicked(gtx):
		answer(true, false)
		return
	}
	pal := th.Pal
	names := p.paths
	p.dlgBox.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		var kids []layout.FlexChild
		text := fmt.Sprintf("%d items already exist in the target folder:", len(names))
		if len(names) == 1 {
			text = fmt.Sprintf("'%s' already exists in the target folder.", names[0])
		}
		kids = append(kids, layout.Rigid(gtk.Text{S: text, Size: 12, Color: pal.FgColor}.Widget(th)))
		if len(names) > 1 {
			for i, n := range names {
				if i == 10 {
					kids = append(kids, layout.Rigid(gtk.Text{S: fmt.Sprintf("  • … and %d more", len(names)-10), Size: 12, Color: pal.FgColor}.Widget(th)))
					break
				}
				kids = append(kids, layout.Rigid(gtk.Text{S: "  • " + n, Size: 12, Color: pal.FgColor}.Widget(th)))
			}
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	},
		func(gtx layout.Context) layout.Dimensions {
			return gtk.Button{Label: "Cancel"}.Layout(gtx, th, &p.cancel)
		},
		func(gtx layout.Context) layout.Dimensions {
			return gtk.Button{Label: "Skip existing"}.Layout(gtx, th, &p.skip)
		},
		func(gtx layout.Context) layout.Dimensions {
			return gtk.Button{Label: "Overwrite", Default: true}.Layout(gtx, th, &p.ok)
		},
	)
}

// ---- permissions ----------------------------------------------------------------------

// perms is SftpPermissionsDialog.svelte's state: nine bits, the octal
// text, recursion and its scope.
type perms struct {
	bits      [9]widget.Bool // owner rwx, group rwx, others rwx
	octal     widget.Editor
	recursive widget.Bool
	scope     widget.Enum
}

func (pm *perms) set(mode uint32) {
	for i := range pm.bits {
		pm.bits[i].Value = mode&(1<<(8-i)) != 0
	}
	pm.octal.SingleLine = true
	pm.octal.SetText(fmt.Sprintf("0%o", pm.mode()))
}

func (pm *perms) mode() uint32 {
	var m uint32
	for i := range pm.bits {
		if pm.bits[i].Value {
			m |= 1 << (8 - i)
		}
	}
	return m
}

func (p *Panel) openPerms(paths []string, st State) {
	if len(paths) == 0 {
		return
	}
	mode := uint32(0o755)
	for _, e := range st.Entries {
		if e.Path == paths[0] {
			mode = e.Mode
		}
	}
	p.paths = paths
	p.perm.set(mode & 0o777)
	p.perm.recursive.Value = false
	p.perm.scope.Value = "all"
	title := fmt.Sprintf("Change Permissions for %d items", len(paths))
	if len(paths) == 1 {
		title = "Change Permissions: " + BaseName(paths[0])
	}
	p.dlg = "perms"
	p.dlgBox = gtk.Dialog{Title: title, Width: 400, Closable: true}
}

func (p *Panel) permsDialog(gtx layout.Context, th *gtk.Theme) {
	pm := &p.perm
	for i := range pm.bits {
		if pm.bits[i].Update(gtx) {
			pm.octal.SetText(fmt.Sprintf("0%o", pm.mode()))
		}
	}
	for {
		ev, ok := pm.octal.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			if v, err := strconv.ParseUint(strings.TrimSpace(pm.octal.Text()), 8, 32); err == nil {
				for i := range pm.bits {
					pm.bits[i].Value = v&(1<<(8-i)) != 0
				}
			}
		}
	}
	if p.cancel.Clicked(gtx) || p.dlgBox.CloseRequested() {
		p.dlg = ""
		return
	}
	if p.ok.Clicked(gtx) {
		if v, err := strconv.ParseUint(strings.TrimSpace(pm.octal.Text()), 8, 32); err == nil {
			p.B.Chmod(p.paths, uint32(v), pm.recursive.Value, pm.scope.Value)
			p.dlg = ""
			return
		}
	}
	pal := th.Pal
	p.dlgBox.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		cell := func(w unit.Dp, wd layout.Widget) layout.FlexChild {
			return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				px := gtx.Dp(w)
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = px, px
				d := layout.Center.Layout(gtx, wd)
				d.Size.X = px
				return d
			})
		}
		head := func(s string) layout.Widget {
			return gtk.Text{S: s, Size: 13, Weight: font.SemiBold, Color: pal.FgColor}.Widget(th)
		}
		rowW := func(label string, base int) layout.FlexChild {
			return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 4, Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						cell(80, gtk.Text{S: label, Size: 13, Weight: font.Bold, Color: pal.FgColor}.Widget(th)),
						cell(80, func(gtx layout.Context) layout.Dimensions { return th.CheckButton(gtx, &pm.bits[base], "", false) }),
						cell(80, func(gtx layout.Context) layout.Dimensions { return th.CheckButton(gtx, &pm.bits[base+1], "", false) }),
						cell(80, func(gtx layout.Context) layout.Dimensions { return th.CheckButton(gtx, &pm.bits[base+2], "", false) }),
					)
				})
			})
		}
		kids := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx, cell(80, head("")), cell(80, head("Read")), cell(80, head("Write")), cell(80, head("Execute")))
			}),
			rowW("Owner", 0), rowW("Group", 3), rowW("Others", 6),
			layout.Rigid(layout.Spacer{Height: 12}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(gtk.Text{S: "Octal: ", Size: 13, Color: pal.FgColor}.Widget(th)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return gtk.Entry{Width: 80, Mono: true}.Layout(gtx, th, &pm.octal)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: 12}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return gtk.HLine(gtx, pal.BorderColor) }),
			layout.Rigid(layout.Spacer{Height: 12}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return th.CheckButton(gtx, &pm.recursive, "Apply recursively", false)
			}),
		}
		if pm.recursive.Value {
			kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 24, Top: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return th.RadioGroup(gtx, &pm.scope, []gtk.RadioOption{
						{Value: "all", Label: "Files and folders"}, {Value: "dirs", Label: "Folders only"}, {Value: "files", Label: "Files only"},
					}, true, false)
				})
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	},
		func(gtx layout.Context) layout.Dimensions {
			return gtk.Button{Label: "Cancel"}.Layout(gtx, th, &p.cancel)
		},
		func(gtx layout.Context) layout.Dimensions {
			return gtk.Button{Label: "Apply", Default: true}.Layout(gtx, th, &p.ok)
		},
	)
}

// DialogOpen reports whether a dialog of the panel is shown (the window's
// keys pause).
func (p *Panel) DialogOpen() bool { return p.dlg != "" || p.menu.Opened() }
