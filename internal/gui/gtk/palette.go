// The palettes mirror the --gtk-* color tokens of the web GUI's theme
// (frontend/src/lib/theme/gtk.css); palette_test.go checks that they match,
// so both GUIs look the same while they coexist.

package gtk

import "image/color"

// Palette holds the color tokens of LeanTerm's GTK theme (--gtk-* in gtk.css).
type Palette struct {
	Name                string
	BgColor             color.NRGBA // --gtk-bg-color
	FgColor             color.NRGBA // --gtk-fg-color
	BaseColor           color.NRGBA // --gtk-base-color
	TextColor           color.NRGBA // --gtk-text-color
	MutedFg             color.NRGBA // --gtk-muted-fg
	DisabledFg          color.NRGBA // --gtk-disabled-fg
	BorderColor         color.NRGBA // --gtk-border-color
	HeaderBg            color.NRGBA // --gtk-header-bg
	MenuBg              color.NRGBA // --gtk-menu-bg
	RowHover            color.NRGBA // --gtk-row-hover
	RowAlt              color.NRGBA // --gtk-row-alt
	BtnBg               color.NRGBA // --gtk-btn-bg
	BtnBorder           color.NRGBA // --gtk-btn-border
	BtnHoverBorder      color.NRGBA // --gtk-btn-hover-border
	BtnHoverBg          color.NRGBA // --gtk-btn-hover-bg
	BtnActiveBg         color.NRGBA // --gtk-btn-active-bg
	BtnFocusBg          color.NRGBA // --gtk-btn-focus-bg
	FocusRing           color.NRGBA // --gtk-focus-ring
	Accent              color.NRGBA // --gtk-accent
	SelectedBg          color.NRGBA // --gtk-selected-bg
	SelectedFg          color.NRGBA // --gtk-selected-fg
	PanedHandle         color.NRGBA // --gtk-paned-handle
	PanedHandleHover    color.NRGBA // --gtk-paned-handle-hover
	TooltipBg           color.NRGBA // --gtk-tooltip-bg
	TooltipFg           color.NRGBA // --gtk-tooltip-fg
	Shadow              color.NRGBA // --gtk-shadow
	Backdrop            color.NRGBA // --gtk-backdrop
	ErrorBg             color.NRGBA // --gtk-error-bg
	ErrorFg             color.NRGBA // --gtk-error-fg
	ErrorBorder         color.NRGBA // --gtk-error-border
	WarningBg           color.NRGBA // --gtk-warning-bg
	WarningFg           color.NRGBA // --gtk-warning-fg
	SuccessFg           color.NRGBA // --gtk-success-fg
	LinkFg              color.NRGBA // --gtk-link-fg
	Scrollbar           color.NRGBA // --gtk-scrollbar
	ScrollbarHover      color.NRGBA // --gtk-scrollbar-hover
	TerminalBg          color.NRGBA // --gtk-terminal-bg
	SyntaxKeyword       color.NRGBA // --gtk-syntax-keyword
	SyntaxString        color.NRGBA // --gtk-syntax-string
	SyntaxComment       color.NRGBA // --gtk-syntax-comment
	SyntaxNumber        color.NRGBA // --gtk-syntax-number
	SyntaxType          color.NRGBA // --gtk-syntax-type
	SyntaxFunction      color.NRGBA // --gtk-syntax-function
	SyntaxConstant      color.NRGBA // --gtk-syntax-constant
	SyntaxOperator      color.NRGBA // --gtk-syntax-operator
	SyntaxTag           color.NRGBA // --gtk-syntax-tag
	SyntaxAttribute     color.NRGBA // --gtk-syntax-attribute
	SyntaxProperty      color.NRGBA // --gtk-syntax-property
	SyntaxMeta          color.NRGBA // --gtk-syntax-meta
	SyntaxInvalid       color.NRGBA // --gtk-syntax-invalid
	EditorSelection     color.NRGBA // --gtk-editor-selection
	EditorActiveLine    color.NRGBA // --gtk-editor-active-line
	EditorMinimapSlider color.NRGBA // --gtk-editor-minimap-slider
	EditorBookmark      color.NRGBA // --gtk-editor-bookmark
	EditorModified      color.NRGBA // --gtk-editor-modified
	EditorSearchMatch   color.NRGBA // --gtk-editor-search-match
}

func c(rgb uint32, a uint8) color.NRGBA {
	return color.NRGBA{R: uint8(rgb >> 16), G: uint8(rgb >> 8), B: uint8(rgb), A: a}
}

// Dark is the dark color scheme (the default).
var Dark = Palette{
	Name:                "dark",
	BgColor:             c(0x2b2b2b, 0xff),
	FgColor:             c(0xe6e6e6, 0xff),
	BaseColor:           c(0x1f1f1f, 0xff),
	TextColor:           c(0xf2f2f2, 0xff),
	MutedFg:             c(0xa0a0a0, 0xff),
	DisabledFg:          c(0x6f6f6f, 0xff),
	BorderColor:         c(0x3c3c3c, 0xff),
	HeaderBg:            c(0x333333, 0xff),
	MenuBg:              c(0x2f2f2f, 0xff),
	RowHover:            c(0xffffff, 15),
	RowAlt:              c(0xffffff, 6),
	BtnBg:               c(0x3a3a3a, 0xff),
	BtnBorder:           c(0x4a4a4a, 0xff),
	BtnHoverBorder:      c(0x8a8a8a, 0xff),
	BtnHoverBg:          c(0x474747, 0xff),
	BtnActiveBg:         c(0x2f2f2f, 0xff),
	BtnFocusBg:          c(0x3d3a30, 0xff),
	FocusRing:           c(0x4a90d9, 0xff),
	Accent:              c(0x4a90d9, 0xff),
	SelectedBg:          c(0x2f6db3, 0xff),
	SelectedFg:          c(0xffffff, 0xff),
	PanedHandle:         c(0x3a3a3a, 0xff),
	PanedHandleHover:    c(0x5a5a5a, 0xff),
	TooltipBg:           c(0x0f0f0f, 245),
	TooltipFg:           c(0xeeeeee, 0xff),
	Shadow:              c(0x000000, 140),
	Backdrop:            c(0x000000, 128),
	ErrorBg:             c(0x4a2326, 0xff),
	ErrorFg:             c(0xffb4b4, 0xff),
	ErrorBorder:         c(0x7a3439, 0xff),
	WarningBg:           c(0x4a4220, 0xff),
	WarningFg:           c(0xf3df8a, 0xff),
	SuccessFg:           c(0x8ae234, 0xff),
	LinkFg:              c(0x78aef0, 0xff),
	Scrollbar:           c(0x555555, 0xff),
	ScrollbarHover:      c(0x777777, 0xff),
	TerminalBg:          c(0x000000, 0xff),
	SyntaxKeyword:       c(0x78aef0, 0xff),
	SyntaxString:        c(0x9ccc65, 0xff),
	SyntaxComment:       c(0x8c8c8c, 0xff),
	SyntaxNumber:        c(0xe5b567, 0xff),
	SyntaxType:          c(0x6fc3c3, 0xff),
	SyntaxFunction:      c(0xf3df8a, 0xff),
	SyntaxConstant:      c(0xe59a6f, 0xff),
	SyntaxOperator:      c(0xc8c8c8, 0xff),
	SyntaxTag:           c(0x78aef0, 0xff),
	SyntaxAttribute:     c(0xc39ac9, 0xff),
	SyntaxProperty:      c(0x9fc7f0, 0xff),
	SyntaxMeta:          c(0xb48ead, 0xff),
	SyntaxInvalid:       c(0xffb4b4, 0xff),
	EditorSelection:     c(0x4a90d9, 97),
	EditorActiveLine:    c(0xffffff, 11),
	EditorMinimapSlider: c(0xffffff, 33),
	EditorBookmark:      c(0x4a90d9, 56),
	EditorModified:      c(0xe5a50a, 0xff),
	EditorSearchMatch:   c(0xf3df8a, 71),
}

// Light is the light color scheme.
var Light = Palette{
	Name:                "light",
	BgColor:             c(0xf0f0f0, 0xff),
	FgColor:             c(0x2e3436, 0xff),
	BaseColor:           c(0xffffff, 0xff),
	TextColor:           c(0x000000, 0xff),
	MutedFg:             c(0x666666, 0xff),
	DisabledFg:          c(0xa0a0a0, 0xff),
	BorderColor:         c(0xd1d1d1, 0xff),
	HeaderBg:            c(0xe4e4e4, 0xff),
	MenuBg:              c(0xffffff, 0xff),
	RowHover:            c(0x000000, 15),
	RowAlt:              c(0x000000, 6),
	BtnBg:               c(0xf6f6f6, 0xff),
	BtnBorder:           c(0xcfcfcf, 0xff),
	BtnHoverBorder:      c(0x555555, 0xff),
	BtnHoverBg:          c(0xffffff, 0xff),
	BtnActiveBg:         c(0xe2e2e2, 0xff),
	BtnFocusBg:          c(0xffebbb, 0xff),
	FocusRing:           c(0x3584e4, 0xff),
	Accent:              c(0x3584e4, 0xff),
	SelectedBg:          c(0x3584e4, 0xff),
	SelectedFg:          c(0xffffff, 0xff),
	PanedHandle:         c(0xd0d0d0, 0xff),
	PanedHandleHover:    c(0xb0b0b0, 0xff),
	TooltipBg:           c(0x282828, 242),
	TooltipFg:           c(0xffffff, 0xff),
	Shadow:              c(0x000000, 77),
	Backdrop:            c(0x000000, 89),
	ErrorBg:             c(0xfce4e4, 0xff),
	ErrorFg:             c(0x721c24, 0xff),
	ErrorBorder:         c(0xf5c6cb, 0xff),
	WarningBg:           c(0xf6f877, 0xff),
	WarningFg:           c(0x5c4a00, 0xff),
	SuccessFg:           c(0x2e7d32, 0xff),
	LinkFg:              c(0x1e70bf, 0xff),
	Scrollbar:           c(0xc1c1c1, 0xff),
	ScrollbarHover:      c(0xa1a1a1, 0xff),
	TerminalBg:          c(0x000000, 0xff),
	SyntaxKeyword:       c(0x1e5fb3, 0xff),
	SyntaxString:        c(0x3a7d17, 0xff),
	SyntaxComment:       c(0x7a7a7a, 0xff),
	SyntaxNumber:        c(0xa35a00, 0xff),
	SyntaxType:          c(0x0e7a7a, 0xff),
	SyntaxFunction:      c(0x8a6100, 0xff),
	SyntaxConstant:      c(0xb5480f, 0xff),
	SyntaxOperator:      c(0x444444, 0xff),
	SyntaxTag:           c(0x1e5fb3, 0xff),
	SyntaxAttribute:     c(0x7b3f9e, 0xff),
	SyntaxProperty:      c(0x2b5b8a, 0xff),
	SyntaxMeta:          c(0x7b3f9e, 0xff),
	SyntaxInvalid:       c(0xc01c28, 0xff),
	EditorSelection:     c(0x3584e4, 64),
	EditorActiveLine:    c(0x000000, 10),
	EditorMinimapSlider: c(0x000000, 28),
	EditorBookmark:      c(0x3584e4, 41),
	EditorModified:      c(0xc88800, 0xff),
	EditorSearchMatch:   c(0xffc800, 89),
}
