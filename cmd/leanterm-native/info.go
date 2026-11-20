package main

import (
	"fmt"
	imdraw "golang.org/x/image/draw"
	"image"
	"image/draw"
	"os"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	_ "image/jpeg"
	_ "image/png"

	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

// infoPage is the Info tab (ConnectionPanel.svelte's info page).
func (u *ui) infoPage(gtx layout.Context) layout.Dimensions {
	p := u.th.Pal
	gtk.Fill(gtx, p.BaseColor)

	// Save the notes of the current node once typing has settled.
	u.flushNotes()

	sel := u.singleSelected()
	list := layout.List{Axis: layout.Vertical}
	d := list.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		kids := []layout.FlexChild{}
		if sel == nil {
			kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return u.welcomePage(gtx)
			}))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
		}
		kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.nodeInfoPage(gtx, sel.ID)
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	})
	_ = p
	return d
}

// flushNotes saves the notes text to the config when it settled for >500ms.
func (u *ui) flushNotes() {
	if u.notesNode == "" {
		return
	}
	txt := u.notes.Text()
	if txt != u.notesLast {
		u.notesLast = txt
		u.notesChanged = time.Now()
		u.notesDirty = true
		return
	}
	if u.notesDirty && time.Since(u.notesChanged) > 500*time.Millisecond {
		u.notesDirty = false
		if err := u.configSvc.SetDescription(u.notesNode, txt); err != nil {
			u.dropMsg = err.Error()
		}
	}
}

// welcomePage shows the welcome box (nothing or the root selected).
func (u *ui) welcomePage(gtx layout.Context) layout.Dimensions {
	p := u.th.Pal
	return layout.Inset{Top: 48, Left: 16, Right: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return gtk.Text{S: "Welcome to LeanTerm", Size: gtk.Em(2.0), Color: p.FgColor, Weight: font.Bold}.Layout(gtx, u.th)
			}),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return gtk.Text{S: "LeanTerm Connection Manager is ready for your sessions.", Size: gtk.Em(0.9), Color: p.MutedFg}.Layout(gtx, u.th)
			}),
			layout.Rigid(layout.Spacer{Height: 24}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints = layout.Constraints{Min: image.Pt(0, 0), Max: image.Pt(gtx.Dp(128), gtx.Dp(128))}
				lg := gtk.Logo{}
				return lg.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: 24}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					kids := []layout.FlexChild{}
					for _, s := range []string{
						"Select or create a connection in the left tree and click Connect or double-click to start.",
						"Use Local shell to open an instant terminal.",
						"Click Settings to customize colors, keys, fonts, and security settings.",
					} {
						it := s
						kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: 2, Bottom: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return gtk.Text{S: "•  " + it, Size: gtk.Em(0.9), Color: p.FgColor, LineHeight: 1.4}.Layout(gtx, u.th)
							})
						}))
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
				})
			}),
		)
	})
}

// nodeInfoPage shows the notes and the statistics of the selected node.
func (u *ui) nodeInfoPage(gtx layout.Context, id string) layout.Dimensions {
	p := u.th.Pal
	n := u.byID[id]
	showStatistics := u.cfg == nil || u.cfg.Appearance.ShowStatistics
	showScreenshots := u.cfg != nil && u.cfg.Appearance.ShowScreenshots

	// The notes belong to the node the user is looking at; reload them
	// when the selection changed.
	if u.notesNode != id {
		u.notesNode = id
		u.notesLast = ""
		u.notes.SetText(n.Description)
		u.notesLast = u.notes.Text()
		u.notesDirty = false
	}

	kids := []layout.FlexChild{}

	// Node header.
	kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: 12, Right: 12, Top: 12, Bottom: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			icon := fonts.IconFolder
			if !n.IsGroup {
				icon = methodIcon(n.Method)
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return u.th.IconBox(gtx, icon, 20, p.FgColor)
				}),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					kids := []layout.FlexChild{}
					kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return gtk.Text{S: n.Name, Size: gtk.Em(1.1), Color: p.FgColor, Weight: font.Bold}.Layout(gtx, u.th)
					}))
					if n.Title != "" && n.Title != n.Name {
						kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return gtk.Text{S: "(" + n.Title + ")", Size: gtk.Em(0.9), Color: p.MutedFg}.Layout(gtx, u.th)
						}))
					}
					return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, kids...)
				}),
			)
		})
	}))

	// Notes & description.
	kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: 12, Right: 12, Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return gtk.Text{S: "Notes & Description:", Size: gtk.Em(0.9), Color: p.FgColor, Weight: font.Bold}.Layout(gtx, u.th)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(140)))
					return gtk.TextArea{Placeholder: "Insert your comments for this connection here ...", Mono: true}.Layout(gtx, u.th, &u.notes)
				}),
			)
		})
	}))

	// Statistics.
	if showStatistics {
		kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 12, Right: 12, Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				stat := u.sessionSvc.GetStats(id)
				return u.th.Frame(gtx, "STATISTICS", func(gtx layout.Context) layout.Dimensions {
					kids := []layout.FlexChild{}
					statRow := func(name, value string) layout.FlexChild {
						return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: 1, Bottom: 1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(170), gtx.Dp(170)
										return gtk.Text{S: name, Color: p.MutedFg}.Layout(gtx, u.th)
									}),
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										return gtk.Text{S: value, Color: p.FgColor}.Layout(gtx, u.th)
									}),
								)
							})
						})
					}
					kids = append(kids,
						statRow("Total connections:", fmt.Sprint(stat.TotalConnections)),
						statRow("Total connected time:", formatDuration(stat.TotalSeconds)),
						statRow("Last connection:", formatLastConn(stat.LastStart)),
					)
					kids = append(kids, layout.Rigid(layout.Spacer{Height: 6}.Layout))
					kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							b := gtk.Button{Label: "Reset Statistics", Icon: fonts.IconUndo}
							return b.Layout(gtx, u.th, &u.statsReset)
						})
					}))
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
				})
			})
		}))
	}

	// Screenshots.
	if showScreenshots {
		kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 12, Right: 12, Top: 8, Bottom: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				conn, ok := u.core.FindConnection(id)
				return u.th.Frame(gtx, "SCREENSHOTS", func(gtx layout.Context) layout.Dimensions {
					if !ok || len(conn.Screenshots) == 0 {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return gtk.Text{S: "No screenshots saved yet. Use 'Take Screenshot' in terminal context menu.", Size: gtk.Em(0.85), Color: p.MutedFg}.Layout(gtx, u.th)
							}),
							layout.Rigid(layout.Spacer{Height: 6}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return u.openShotsButton(gtx)
							}),
						)
					}
					kids := []layout.FlexChild{}
					kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.screenshotGrid(gtx, conn.Screenshots)
					}))
					kids = append(kids, layout.Rigid(layout.Spacer{Height: 6}.Layout))
					kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.openShotsButton(gtx)
					}))
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
				})
			})
		}))
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
}

// openShotsButton is the "Open Screenshots Folder" button.
func (u *ui) openShotsButton(gtx layout.Context) layout.Dimensions {
	return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		b := gtk.Button{Label: "Open Screenshots Folder", Icon: fonts.IconFolderOpen}
		return b.Layout(gtx, u.th, &u.shotsOpen)
	})
}

// shotCacheEntry is a decoded + scaled thumbnail, reused while the file on
// disk is unchanged.
type shotCacheEntry struct {
	mod  time.Time
	size int64
	img  image.Image
}

// screenshotGrid paints the thumbnails of the screenshots (the web GUI shows
// them side by side, 8px apart).
func (u *ui) screenshotGrid(gtx layout.Context, shots []string) layout.Dimensions {
	gtx.Constraints = layout.Constraints{Max: image.Pt(gtx.Constraints.Max.X, gtx.Dp(118))}
	list := layout.List{Axis: layout.Horizontal}
	return list.Layout(gtx, len(shots), func(gtx layout.Context, i int) layout.Dimensions {
		return layout.Inset{Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return u.screenshotThumb(gtx, shots[i])
		})
	})
}

// screenshotThumb paints one thumbnail (decoded from the file).
func (u *ui) screenshotThumb(gtx layout.Context, path string) layout.Dimensions {
	p := u.th.Pal
	img, err := u.loadScaledImage(path, gtx.Dp(110))
	if err != nil {
		return layout.Inset{Top: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return gtk.Text{S: "(unreadable)", Size: gtk.Em(0.8), Color: p.MutedFg}.Layout(gtx, u.th)
		})
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	gtx.Constraints = layout.Exact(image.Pt(w, h))
	r := image.Rectangle{Max: image.Pt(w, h)}
	cl := clip.UniformRRect(r, gtx.Dp(4)).Push(gtx.Ops)
	op := paint.NewImageOp(img)
	op.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	cl.Pop()
	return layout.Dimensions{Size: image.Pt(w, h)}
}

// loadScaledImage decodes the image and scales it to h pixels high. The
// decode+scale is cached per file content (mtime+size): this runs every
// frame, so re-reading the PNGs from disk would not do.
func (u *ui) loadScaledImage(path string, h int) (image.Image, error) {
	info, statErr := os.Stat(path)
	if statErr == nil {
		if u.shotCache == nil {
			u.shotCache = map[string]shotCacheEntry{}
		}
		if c, ok := u.shotCache[path]; ok && c.mod.Equal(info.ModTime()) && c.size == info.Size() {
			return c.img, nil
		}
	}
	img, err := loadScaledImage(path, h)
	if err != nil {
		return nil, err
	}
	if statErr == nil {
		if len(u.shotCache) > 32 { // bound the cache
			u.shotCache = map[string]shotCacheEntry{}
		}
		u.shotCache[path] = shotCacheEntry{mod: info.ModTime(), size: info.Size(), img: img}
	}
	return img, nil
}

func loadScaledImage(path string, h int) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	if b.Dy() <= 0 {
		return nil, fmt.Errorf("empty image")
	}
	w := int(float64(b.Dx()) * float64(h) / float64(b.Dy()))
	if w <= 0 {
		w = 1
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	imdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst, nil
}
