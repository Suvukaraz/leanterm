// Command gtk-gallery shows the native GTK widgets (internal/gui/gtk) laid out
// like the web GUI's reference gallery (frontend/gallery.html), to compare
// both side by side. With -out it renders one frame headless into a PNG.
package main

import (
	"flag"
	"image"
	"image/png"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

func main() {
	section := flag.String("section", "basics", "basics | tree | menu | message")
	scheme := flag.String("theme", "dark", "dark | light")
	out := flag.String("out", "", "render headless into this PNG and exit")
	width := flag.Int("width", 1010, "width (headless)")
	height := flag.Int("height", 520, "height (headless)")
	flag.Parse()

	g := newGallery(*section, gtk.New(*scheme))
	if *out != "" {
		if err := renderPNG(g, *out, *width, *height); err != nil {
			log.Fatal(err)
		}
		return
	}
	w := new(app.Window)
	w.Option(app.Title("GTK gallery"), app.Size(unit.Dp(float32(*width)), unit.Dp(float32(*height))))
	go func() {
		var ops op.Ops
		for {
			switch e := w.Event().(type) {
			case app.DestroyEvent:
				os.Exit(0)
			case app.FrameEvent:
				gtx := app.NewContext(&ops, e)
				g.Layout(gtx)
				e.Frame(gtx.Ops)
			}
		}
	}()
	app.Main()
}

// renderPNG draws a few frames (popups and measurements settle) and saves
// the last one at 1px per dp, like the reference screenshots.
func renderPNG(g *gallery, path string, w, h int) error {
	win, err := headless.NewWindow(w, h)
	if err != nil {
		return err
	}
	var router input.Router
	var ops op.Ops
	for i := 0; i < 3; i++ {
		ops.Reset()
		gtx := layout.Context{
			Ops:         &ops,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(w, h)),
			Source:      router.Source(),
		}
		g.Layout(gtx)
		router.Frame(&ops)
		if err := win.Frame(&ops); err != nil {
			return err
		}
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	if err := win.Screenshot(img); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
