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
	// The main window's buttons and menus.
	IconSave      = '\uf0c7'     // nf-fa-floppy_o
	IconCopy      = '\uf0c5'     // nf-fa-copy
	IconCut       = '\uf0c4'     // nf-fa-scissors
	IconPaste     = '\uf0ea'     // nf-fa-paste
	IconPlay      = '\uf04b'     // nf-fa-play
	IconStarOff   = '\uf006'     // nf-fa-star_o
	IconList      = '\uf03a'     // nf-fa-list
	IconMinus     = '\uf068'     // nf-fa-minus
	IconEdit      = '\uf044'     // nf-fa-edit (pencil in a box)
	IconExchange  = '\uf0ec'     // nf-fa-exchange (two arrows, transfers)
	IconQuestion  = '\uf059'     // nf-fa-question_circle
	IconExternal  = '\uf08e'     // nf-fa-external_link
	IconFile      = '\uf0f6'     // nf-fa-file_text
	IconFolderAdd = '\U000F0256' // nf-md-folder_plus
	IconRedo      = '\uf01e'     // nf-fa-repeat
	IconUndo      = '\uf0e2'     // nf-fa-undo
	IconStop      = '\uf04d'     // nf-fa-stop
	IconGlobe     = '\uf0ac'     // nf-fa-globe
	IconBriefcase = '\uf0b1'     // nf-fa-briefcase (the SFTP method)
	IconTools     = '\U000F1064' // nf-md-tools (settings)
	IconBulb      = '\U000F0336' // nf-md-lightbulb_outline (the Info tab)
	IconBars      = '\uf0c9'     // nf-fa-bars (more actions)
	IconChevLeft  = '\uf053'     // nf-fa-chevron_left
	IconChevRight = '\uf054'     // nf-fa-chevron_right
	IconChevUp    = '\uf077'     // nf-fa-chevron_up
	IconChevDown  = '\uf078'     // nf-fa-chevron_down
	IconFolderO   = '\uf114'     // nf-fa-folder_o (outline)
	IconFileO     = '\uf016'     // nf-fa-file_o (outline)
	IconHomeO     = '\U000F07D1' // nf-md-home_outline
	IconLinkArrow = '\uf064'     // nf-fa-share (symbolic links)
	IconCogs      = '\uf085'     // nf-fa-cogs (gtk-execute)
	IconSliders   = '\uf1de'     // nf-fa-sliders (gtk-properties)
	IconKey       = '\uf084'     // nf-fa-key
)

// AllIcons lists the icons above (for tests and icon pickers).
var AllIcons = []rune{
	IconTerminal, IconFileCode, IconFolder, IconFolderOpen, IconLock, IconUnlock,
	IconPlug, IconServer, IconStar, IconHistory, IconCog, IconInfo, IconClose,
	IconCheck, IconWarning, IconSearch, IconUpload, IconDownload, IconRefresh,
	IconHome, IconArrowLeft, IconArrowRight, IconArrowUp, IconPlus, IconTrash,
	IconPencil, IconLinux, IconWindows, IconPowerOff, IconBolt, IconCircle,
	IconSitemap, IconPowerlineLR,
	IconSave, IconCopy, IconCut, IconPaste, IconPlay, IconStarOff,
	IconList, IconMinus, IconEdit, IconExchange, IconQuestion, IconExternal,
	IconFile, IconFolderAdd, IconRedo, IconUndo, IconStop, IconGlobe, IconBriefcase, IconTools, IconBulb, IconBars, IconLinkArrow, IconChevLeft, IconChevRight, IconChevUp, IconChevDown, IconFolderO, IconFileO, IconHomeO,
	IconCogs, IconSliders, IconKey,
}
