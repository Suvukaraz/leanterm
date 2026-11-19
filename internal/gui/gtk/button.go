package gtk

import (
	"image"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

// ButtonVariant is a button's style: Normal (gtk.css "button"), CP (the
// borderless toolbar buttons, ".button-cp") or Flat (no frame until hovered).
type ButtonVariant uint8

const (
	Normal ButtonVariant = iota
	CP
	Flat
)

// Button describes a push button (Button.svelte).
type Button struct {
	Label string
	// Icon is a Nerd Fonts icon before the label (0 = none).
	Icon rune
	// Default marks the dialog's default button (accent border, weight 500).
	Default  bool
	Disabled bool
	Variant  ButtonVariant
	// Active shows a toggle button pressed (ToggleButton.svelte).
	Active bool
	// MinWidth widens the button (dialog buttons).
	MinWidth unit.Dp
}

// Layout draws the button; clicks are read from c as usual.
func (b Button) Layout(gtx layout.Context, th *Theme, c *widget.Clickable) layout.Dimensions {
	if b.Disabled {
		gtx = gtx.Disabled()
	}
	// CSS: margin 1px around the button.
	return layout.UniformInset(1).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return b.layout(gtx, th, c)
	})
}

func (b Button) layout(gtx layout.Context, th *Theme, c *widget.Clickable) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		p := th.Pal
		fg := p.FgColor
		weight := font.Normal
		if b.Default {
			weight = font.Medium
		}
		// CSS: padding 3px 8px (cp: 3px), gap 4px, line-height 1.3, border 1px.
		padX, padY := unit.Dp(8), unit.Dp(3)
		if b.Variant == CP {
			padX = 3
		}
		content := func(gtx layout.Context) layout.Dimensions {
			var kids []layout.FlexChild
			if b.Icon != 0 {
				kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return th.IconBox(gtx, b.Icon, 16, fg)
				}))
			}
			if b.Label != "" {
				if b.Icon != 0 {
					kids = append(kids, layout.Rigid(layout.Spacer{Width: 4}.Layout))
				}
				kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return Text{S: b.Label, Color: fg, Weight: weight, LineHeight: 1.3, MaxLines: 1}.Layout(gtx, th)
				}))
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
		}
		gtx.Constraints.Min = image.Point{}
		call, cd := record(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: padX + 1, Right: padX + 1, Top: padY + 1, Bottom: padY + 1}.Layout(gtx, content)
		})
		size := cd.Size
		size.X = max(size.X, gtx.Dp(b.MinWidth))

		bg, border := p.BtnBg, p.BtnBorder
		if b.Default {
			border = p.Accent
		}
		hovered := c.Hovered() && !b.Disabled
		switch {
		case b.Active:
			bg, border = p.BtnFocusBg, p.BorderColor
		case c.Pressed() && !b.Disabled:
			bg = p.BtnActiveBg
		case hovered:
			bg, border = p.BtnHoverBg, p.BtnHoverBorder
		}
		radius := gtx.Dp(4)
		switch b.Variant {
		case CP, Flat:
			radius = gtx.Dp(3)
			if !hovered && !b.Active {
				bg.A, border.A = 0, 0
			}
		}
		if b.Disabled {
			bg, border, fg = faded(bg, 0.5), faded(border, 0.5), faded(fg, 0.5)
			// Redraw the content faded.
			call, _ = record(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: padX + 1, Right: padX + 1, Top: padY + 1, Bottom: padY + 1}.Layout(gtx, content)
			})
		}
		// :focus-visible draws a 1px focus ring on the border.
		if gtx.Focused(c) && !b.Disabled {
			border = p.FocusRing
		}
		box(gtx, size, radius, bg, border)
		off := op.Offset(image.Pt((size.X-cd.Size.X)/2, 0)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		off.Pop()
		if !b.Disabled {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		return layout.Dimensions{Size: size, Baseline: cd.Baseline}
	})
}
