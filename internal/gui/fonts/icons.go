package fonts

// Icons of the GUI: code points of the Nerd Fonts symbol font ([Symbols]).
// Names follow the Nerd Fonts cheat sheet (nf-<set>-<name>).
const (
	IconTerminal    = '\uf489' // nf-oct-terminal
	IconFileCode    = '\uf1c9' // nf-fa-file_code
	IconFolder      = '\uf07b' // nf-fa-folder
	IconFolderOpen  = '\uf07c' // nf-fa-folder_open
	IconLock        = '\uf023' // nf-fa-lock
	IconUnlock      = '\uf09c' // nf-fa-unlock
	IconPlug        = '\uf1e6' // nf-fa-plug
	IconServer      = '\uf233' // nf-fa-server
	IconStar        = '\uf005' // nf-fa-star
	IconHistory     = '\uf1da' // nf-fa-history
	IconCog         = '\uf013' // nf-fa-cog
	IconInfo        = '\uf05a' // nf-fa-info_circle
	IconClose       = '\uf00d' // nf-fa-close
	IconCheck       = '\uf00c' // nf-fa-check
	IconWarning     = '\uf071' // nf-fa-warning
	IconSearch      = '\uf002' // nf-fa-search
	IconUpload      = '\uf093' // nf-fa-upload
	IconDownload    = '\uf019' // nf-fa-download
	IconRefresh     = '\uf021' // nf-fa-refresh
	IconHome        = '\uf015' // nf-fa-home
	IconArrowLeft   = '\uf060' // nf-fa-arrow_left
	IconArrowRight  = '\uf061' // nf-fa-arrow_right
	IconArrowUp     = '\uf062' // nf-fa-arrow_up
	IconPlus        = '\uf067' // nf-fa-plus
	IconTrash       = '\uf1f8' // nf-fa-trash
	IconPencil      = '\uf040' // nf-fa-pencil
	IconLinux       = '\uf17c' // nf-fa-linux
	IconWindows     = '\uf17a' // nf-fa-windows
	IconPowerOff    = '\uf011' // nf-fa-power_off
	IconBolt        = '\uf0e7' // nf-fa-bolt
	IconCircle      = '\uf111' // nf-fa-circle
	IconSitemap     = '\uf0e8' // nf-fa-sitemap
	IconPowerlineLR = '\ue0b0' // nf-pl-left_hard_divider
)

// AllIcons lists the icons above (for tests and icon pickers).
var AllIcons = []rune{
	IconTerminal, IconFileCode, IconFolder, IconFolderOpen, IconLock, IconUnlock,
	IconPlug, IconServer, IconStar, IconHistory, IconCog, IconInfo, IconClose,
	IconCheck, IconWarning, IconSearch, IconUpload, IconDownload, IconRefresh,
	IconHome, IconArrowLeft, IconArrowRight, IconArrowUp, IconPlus, IconTrash,
	IconPencil, IconLinux, IconWindows, IconPowerOff, IconBolt, IconCircle,
	IconSitemap, IconPowerlineLR,
}
