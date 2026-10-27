//go:generate go run ../../cmd/gen-config-doc/main.go ../../docs/config-reference.md

package config

// SchemaVersion defines the configuration version.
const CurrentSchemaVersion = 2

// Built-in prompt defaults from lib/PACUtils.pm:234-239
const (
	DefaultCommandPrompt              = `(([#%:>~\$\] ])(?!\g{-1})){3,4}|(\w[@\/]\w|sftp).*?[#%>~\$\]]|([\w\-\.]+)[%>\$\]]( |\033)|^[#%\$>\:\]~] *$`
	DefaultUsernamePrompt             = `([lL]ogin|[uU]suario|([uU]ser-?)*[nN]ame.*|[uU]ser)\s*:\s*$`
	DefaultPasswordPrompt             = `([pP]ass|[pP]ass[wW]or[dt](\s+for\s+|\w+@[\w\-\.]+)*|[cC]ontrase.a|Enter passphrase for key '.+')\s*:\s*$`
	DefaultHostKeyVerificationPrompt  = `^.+ontinue connecting \(([^/]+)\/([^/]+)(?:[^)]+)?\)\?\s*$`
	DefaultRemoteHostKeyChangedPrompt = `.*ffending .*key in (.+?)\:(\d+).*`
	DefaultPressAnyKeyPrompt          = `.*(any key to continue|tecla para continuar).*`
)

// FontConfig represents a font family and size
type FontConfig struct {
	Family string `toml:"family" comment:"Font family name"`
	Size   int    `toml:"size" comment:"Font size in points"`
}

// GeneralConfig represents application general behaviour
type GeneralConfig struct {
	Layout                 string `toml:"layout" comment:"Main Window Layout: traditional | compact"`
	Theme                  string `toml:"theme" comment:"Icons Theme: default | asbru-color | asbru-dark | system"`
	ColorScheme            string `toml:"color_scheme" comment:"Colour scheme of the user interface: dark | light"`
	StartMainMaximized     bool   `toml:"start_main_maximized" comment:"At start: Maximized"`
	RememberMainSize       bool   `toml:"remember_main_size" comment:"At start: With last size"`
	StartIconified         bool   `toml:"start_iconified" comment:"Iconified in 'systray'"`
	StartAtLogin           bool   `toml:"start_at_login" comment:"Start at session startup"`
	CloseToTray            bool   `toml:"close_to_tray" comment:"Hide to tray instead of closing"`
	ConfirmExit            bool   `toml:"confirm_exit" comment:"Confirm exit"`
	SaveOnExit             bool   `toml:"save_on_exit" comment:"Auto save on exit"`
	AutoSave               bool   `toml:"auto_save" comment:"Automatically save every configuration change"`
	ProtectWithPassword    bool   `toml:"protect_with_password" comment:"Protect with password"`
	PasswordOnTrayRestore  bool   `toml:"password_on_tray_restore" comment:"Ask to restore from Tray"`
	ForceSplit50           bool   `toml:"force_split_50" comment:"Force split tabs to 50%"`
	HideOnConnect          bool   `toml:"hide_on_connect" comment:"Hide on connection"`
	AutoAcceptHostKeys     bool   `toml:"auto_accept_host_keys" comment:"Auto-accept or delete unknown or offending SSH keys"`
	AppendGroupNameToTitle bool   `toml:"append_group_name_to_title" comment:"Automatically append group name to title for new connections"`
	AllowMultipleInstances bool   `toml:"allow_multiple_instances" comment:"Allow more than one instance to run at the same time"`
	SortGroupsFirst        bool   `toml:"sort_groups_first" comment:"Sort groups first in tree"`
	HelpURL                string `toml:"help_url" comment:"Target of all 'Help' link buttons"`
}

// AppearanceConfig represents look and feel options
type AppearanceConfig struct {
	StartTreeOn                string     `toml:"start_tree_on" comment:"Start on: connections | favourites | history | clusters"`
	ShowScreenshots            bool       `toml:"show_screenshots" comment:"Show screenshots section in Info tab"`
	ShowStatistics             bool       `toml:"show_statistics" comment:"Show statistics section in Info tab"`
	ShowTrayIcon               bool       `toml:"show_tray_icon" comment:"Always show Tray icon"`
	MonochromeTrayIcon         bool       `toml:"monochrome_tray_icon" comment:"Use Black & White tray icon"`
	SelectionToClipboard       bool       `toml:"selection_to_clipboard" comment:"Mouse selection copies content to global clipboard"`
	TreePosition               string     `toml:"tree_position" comment:"Show connections on: left | right"`
	ButtonBarPosition          string     `toml:"button_bar_position" comment:"Main button bar position: top | bottom"`
	ShowTreeTooltips           bool       `toml:"show_tree_tooltips" comment:"Show tooltips in connection tree"`
	ShowTreeLines              bool       `toml:"show_tree_lines" comment:"Show lines in connections tree"`
	TreeOverlayScrolling       bool       `toml:"tree_overlay_scrolling" comment:"Enable overlay scrolling for connections tree"`
	ShowTreeTitles             bool       `toml:"show_tree_titles" comment:"Show tree titles"`
	InfoFont                   FontConfig `toml:"info_font" comment:"Font for Info tab"`
	TreeFont                   FontConfig `toml:"tree_font" comment:"Connection list font"`
	ProtectedColor             string     `toml:"protected_color" comment:"Protected nodes color (#rrggbb)"`
	ProtectedColorTarget       string     `toml:"protected_color_target" comment:"foreground | background"`
	UnprotectedColor           string     `toml:"unprotected_color" comment:"Unprotected nodes color (#rrggbb)"`
	UnprotectedColorTarget     string     `toml:"unprotected_color_target" comment:"foreground | background"`
	AutoHideConnectionsList    bool       `toml:"auto_hide_connections_list" comment:"Hide connections list on terminal open"`
	AutoHideButtonBar          bool       `toml:"auto_hide_button_bar" comment:"Auto hide main button bar on terminal open"`
	ShowTreeOnHover            bool       `toml:"show_tree_on_hover" comment:"Auto show/hide when entering/leaving connections list"`
	TabsInMainWindow           bool       `toml:"tabs_in_main_window" comment:"Put Terminal TABS in Main Window"`
	StatusBarForwards          bool       `toml:"status_bar_forwards" comment:"Show port forwarding information in status bar"`
	StatusBarForwardsFullNames bool       `toml:"status_bar_forwards_full_names" comment:"Use full names instead of abbreviations (-L, -R, -D)"`
	ScreenshotViewer           string     `toml:"screenshot_viewer" comment:"When opening screenshots: internal | external"`
	ScreenshotViewerCommand    string     `toml:"screenshot_viewer_command" comment:"External screenshot viewer command"`
}

// PromptsConfig contains regular expressions for prompt detection
type PromptsConfig struct {
	Command              string `toml:"command" comment:"Command prompt regexp (empty = built-in default)"`
	Username             string `toml:"username" comment:"USERNAME prompt regexp (empty = built-in default)"`
	Password             string `toml:"password" comment:"PASSWORD prompt regexp (empty = built-in default)"`
	HostKeyVerification  string `toml:"host_key_verification" comment:"Host-key verification regexp"`
	RemoteHostKeyChanged string `toml:"remote_host_key_changed" comment:"Remote host key changed regexp"`
	PressAnyKey          string `toml:"press_any_key" comment:"Press any key to continue regexp"`
}

// TerminalColorsConfig defines status colors for tabs
type TerminalColorsConfig struct {
	Connected    string `toml:"connected" comment:"Connected color (#rrggbb)"`
	Disconnected string `toml:"disconnected" comment:"Disconnected color (#rrggbb)"`
	NewData      string `toml:"new_data" comment:"New data color (#rrggbb)"`
}

// TerminalLookConfig defines visual aspects of terminals
type TerminalLookConfig struct {
	Font            FontConfig `toml:"font" comment:"Terminal font"`
	Foreground      string     `toml:"foreground" comment:"Text color"`
	Background      string     `toml:"background" comment:"Back color"`
	Bold            string     `toml:"bold" comment:"Bold Color"`
	BoldLikeText    bool       `toml:"bold_like_text" comment:"Bold color same as text"`
	BoldIsBright    bool       `toml:"bold_is_bright" comment:"Bold color also bright"`
	CursorShape     string     `toml:"cursor_shape" comment:"Cursor shape: block | ibeam | underline"`
	ScrollbackLines int        `toml:"scrollback_lines" comment:"Scrollback lines (-1 = unlimited, 0 = off)"`
	Transparency    float64    `toml:"transparency" comment:"Transparency 0.0 - 1.0 (0 = disabled)"`
	WordChars       string     `toml:"word_chars" comment:"Select-by-word characters"`
	Encoding        string     `toml:"encoding" comment:"Character encoding (UTF-8, ISO-8859-15, etc.)"`
	Backspace       string     `toml:"backspace" comment:"Backspace: auto | ascii-backspace | ascii-delete | delete-sequence | tty"`
	Palette         []string   `toml:"palette" comment:"16 ANSI color palette entries"`
}

// SessionLogsConfig defines auto save session logs
type SessionLogsConfig struct {
	Enabled           bool   `toml:"enabled" comment:"Enable auto save session logs"`
	Folder            string `toml:"folder" comment:"Folder for session logs (empty = default)"`
	Pattern           string `toml:"pattern" comment:"Log file name pattern"`
	Keep              int    `toml:"keep" comment:"Files to keep (0 = infinite)"`
	StripControlChars bool   `toml:"strip_control_chars" comment:"Remove CONTROL characters"`
	Timestamps        bool   `toml:"timestamps" comment:"Add timestamp to log lines"`
}

// TerminalConfig represents global terminal behaviour and styling
type TerminalConfig struct {
	CloseOnDisconnect      bool                 `toml:"close_on_disconnect" comment:"Close Terminal on disconnection"`
	UnsplitDisconnected    bool                 `toml:"unsplit_disconnected" comment:"Auto unsplit disconnected Terminals"`
	TimeoutConnect         int                  `toml:"timeout_connect" comment:"Time to connect in seconds (0 = infinite)"`
	TimeoutCommand         int                  `toml:"timeout_command" comment:"Expect Cmd exec timeout in seconds"`
	MaxReconnectAttempts   int                  `toml:"max_reconnect_attempts" comment:"Maximum reconnect attempts"`
	WhenLastTabClosed      string               `toml:"when_last_tab_closed" comment:"When closing last TAB: nothing | close | hide"`
	OpenNewIn              string               `toml:"open_new_in" comment:"Open NEW connections on: tab | window"`
	WindowWidth            int                  `toml:"window_width" comment:"Default terminal window width"`
	WindowHeight           int                  `toml:"window_height" comment:"Default terminal window height"`
	StartMaximized         bool                 `toml:"start_maximized" comment:"Start Terminals maximized"`
	ShowStatusBar          bool                 `toml:"show_status_bar" comment:"Show status bar on every terminal"`
	ChangeMainTitle        bool                 `toml:"change_main_title" comment:"Change main window title with terminal title"`
	MacrosBar              string               `toml:"macros_bar" comment:"Show MACROS: never | combo | buttons"`
	ShowGlobalMacros       bool                 `toml:"show_global_macros" comment:"Show GLOBAL commands in terminals"`
	TabsPosition           string               `toml:"tabs_position" comment:"Tabs position: top | bottom | left | right"`
	CaptureXtermTitle      bool                 `toml:"capture_xterm_title" comment:"Capture xterm OSC title"`
	AudibleBell            bool                 `toml:"audible_bell" comment:"Audible Bell"`
	HideConnectionsSubmenu bool                 `toml:"hide_connections_submenu" comment:"Hide Connections submenu in context menu"`
	ConfirmChains          bool                 `toml:"confirm_chains" comment:"Confirm Chains selection / execution"`
	SkipFirstChainExpect   bool                 `toml:"skip_first_chain_expect" comment:"Skip first Chain expect"`
	Debug                  bool                 `toml:"debug" comment:"Expect DEBUG"`
	Colors                 TerminalColorsConfig `toml:"colors" comment:"Tab label status colors"`
	Look                   TerminalLookConfig   `toml:"look" comment:"Terminal UI visual configuration"`
	SessionLogs            SessionLogsConfig    `toml:"session_logs" comment:"Session logs auto save configuration"`
}

// SFTPBrowserConfig represents settings for the embedded SFTP browser
type SFTPBrowserConfig struct {
	AutoOpen             bool   `toml:"auto_open" comment:"Open SFTP browser automatically on SSH connections"`
	Width                int    `toml:"width" comment:"Panel width in pixels"`
	Position             string `toml:"position" comment:"Panel position: left | right"`
	FollowTerminalFolder bool   `toml:"follow_terminal_folder" comment:"Follow terminal folder (OSC 7)"`
	ShowHidden           bool   `toml:"show_hidden" comment:"Show hidden files"`
	ConfirmDelete        bool   `toml:"confirm_delete" comment:"Confirm before deleting"`
	PreserveTimestamps   bool   `toml:"preserve_timestamps" comment:"Preserve timestamps on transfer"`
	PreservePermissions  bool   `toml:"preserve_permissions" comment:"Preserve permissions on transfer"`
	UsePartFiles         bool   `toml:"use_part_files" comment:"Write to temporary .part files and rename when complete"`
	ParallelTransfers    int    `toml:"parallel_transfers" comment:"Number of parallel transfers (1-8)"`
	DownloadDir          string `toml:"download_dir" comment:"Default download folder (empty = XDG Downloads)"`
	PollSeconds          int    `toml:"poll_seconds" comment:"Polling interval in seconds (0 = disabled)"`
}

// LocalShellConfig represents local shell preferences
type LocalShellConfig struct {
	Autostart bool                `toml:"autostart" comment:"Start a shell upon startup"`
	Binary    string              `toml:"binary" comment:"Shell Binary (empty = $SHELL)"`
	Options   string              `toml:"options" comment:"Shell Options"`
	Directory string              `toml:"directory" comment:"Initial Directory (empty = $HOME)"`
	Look      *TerminalLookConfig `toml:"look,omitempty" comment:"Optional look override for local shell"`
}

// ProxySocksConfig represents SOCKS proxy configuration
type ProxySocksConfig struct {
	Host string `toml:"host" comment:"Proxy host"`
	Port int    `toml:"port" comment:"Proxy port"`
	User string `toml:"user" comment:"Proxy username"`
}

// ProxyJumpConfig represents SSH jump host configuration
type ProxyJumpConfig struct {
	Host    string `toml:"host" comment:"Jump server host"`
	Port    int    `toml:"port" comment:"Jump server port"`
	User    string `toml:"user" comment:"Jump server user"`
	KeyFile string `toml:"key_file" comment:"Jump server private key file"`
}

// NetworkConfig represents global proxy and jump host settings
type NetworkConfig struct {
	Mode  string           `toml:"mode" comment:"Mode: direct | socks | jump"`
	Socks ProxySocksConfig `toml:"socks" comment:"Global SOCKS proxy"`
	Jump  ProxyJumpConfig  `toml:"jump" comment:"Global Jump server"`
}

// VariableConfig represents a global or connection variable
type VariableConfig struct {
	Name   string `toml:"name" comment:"Variable name, referenced as <GV:name>"`
	Value  string `toml:"value" comment:"Variable value (empty if hidden)"`
	Hidden bool   `toml:"hidden" comment:"True if value is stored encrypted in secrets.age"`
}

// MacroConfig represents a local or remote command macro
type MacroConfig struct {
	Command      string `toml:"command" comment:"Command to execute"`
	Description  string `toml:"description" comment:"Description of the command"`
	Confirm      bool   `toml:"confirm" comment:"Ask confirmation before running"`
	AppendReturn bool   `toml:"append_return,omitempty" comment:"Append Enter / Return after sending"`
}

// MacrosConfig holds remote and local macros
type MacrosConfig struct {
	Local  []MacroConfig `toml:"local,omitempty" comment:"Local macros"`
	Remote []MacroConfig `toml:"remote,omitempty" comment:"Remote macros"`
}

// AutoClusterConfig represents regex matching rules for automatic clustering
type AutoClusterConfig struct {
	Name             string `toml:"name" comment:"Cluster name"`
	MatchName        string `toml:"match_name" comment:"Regex for connection name"`
	MatchTitle       string `toml:"match_title" comment:"Regex for connection title"`
	MatchHost        string `toml:"match_host" comment:"Regex for host"`
	MatchDescription string `toml:"match_description" comment:"Regex for description"`
}

// KeePassConfig represents KeePassXC integration settings
type KeePassConfig struct {
	Enabled  bool   `toml:"enabled" comment:"Enable KeePassXC integration"`
	Database string `toml:"database" comment:"Path to KeePassXC database file"`
	KeyFile  string `toml:"key_file" comment:"Path to KeePassXC key file"`
}

// KeybindingsConfig contains per-window keybinding overrides
type KeybindingsConfig struct {
	Main            map[string]string `toml:"main,omitempty"`
	TreeConnections map[string]string `toml:"tree_connections,omitempty"`
	TreeFavourites  map[string]string `toml:"tree_favourites,omitempty"`
	TreeHistory     map[string]string `toml:"tree_history,omitempty"`
	TreeClusters    map[string]string `toml:"tree_clusters,omitempty"`
	Tabs            map[string]string `toml:"tabs,omitempty"`
	Terminal        map[string]string `toml:"terminal,omitempty"`
	SFTPBrowser     map[string]string `toml:"sftp_browser,omitempty"`
}

// Group represents a folder node in the connection tree
type Group struct {
	ID          string   `toml:"id" comment:"UUID v4"`
	Parent      string   `toml:"parent" comment:"Parent group ID, empty string for root"`
	Name        string   `toml:"name" comment:"Group name"`
	Description string   `toml:"description" comment:"Group description"`
	Protected   bool     `toml:"protected" comment:"Protected from modification"`
	Screenshots []string `toml:"screenshots,omitempty" comment:"List of screenshot file paths"`
}

// ConnectionAuth holds authentication settings for a connection
type ConnectionAuth struct {
	Type      string `toml:"type" comment:"Auth type: password | publickey | manual"`
	User      string `toml:"user" comment:"Login username"`
	KeyFile   string `toml:"key_file" comment:"Private key file path"`
	KeyUser   string `toml:"key_user" comment:"Username next to key file (empty = user)"`
	Exclusive bool   `toml:"exclusive" comment:"Use selected auth exclusively (no fallback)"`
}

// ConnectionNetwork holds network and proxy settings for a connection
type ConnectionNetwork struct {
	Mode       string           `toml:"mode" comment:"Mode: global | direct | socks | jump"`
	Socks      ProxySocksConfig `toml:"socks" comment:"Connection-specific SOCKS proxy"`
	Jump       ProxyJumpConfig  `toml:"jump" comment:"Connection-specific Jump host"`
	PseudoJump bool             `toml:"pseudo_jump" comment:"Pseudo jump server (SSH only)"`
}

// LocalForward represents a local (-L) or remote (-R) port forward
type LocalForward struct {
	BindAddress string `toml:"bind_address" comment:"Local / Bind IP"`
	Port        int    `toml:"port" comment:"Local / Listen Port"`
	Host        string `toml:"host" comment:"Remote host IP"`
	HostPort    int    `toml:"host_port" comment:"Remote Port"`
}

// DynamicForward represents a dynamic (-D) SOCKS port forward
type DynamicForward struct {
	BindAddress string `toml:"bind_address" comment:"Bind IP address"`
	Port        int    `toml:"port" comment:"Listen port"`
}

// SSHOptions holds connection-specific SSH options
type SSHOptions struct {
	IPVersion         string            `toml:"ip_version" comment:"IP Protocol: any | 4 | 6"`
	ForwardX11        bool              `toml:"forward_x11" comment:"Forward X"`
	GatewayPorts      bool              `toml:"gateway_ports" comment:"Allow Remote Port Connect"`
	ForwardAgent      bool              `toml:"forward_agent" comment:"Forward Agent"`
	NoRemoteCommand   bool              `toml:"no_remote_command" comment:"Do not execute remote command"`
	RandomSOCKSTunnel bool              `toml:"random_socks_tunnel" comment:"Create random SOCKS tunnel"`
	Options           map[string]string `toml:"options,omitempty" comment:"ssh_config options"`
	LocalForwards     []LocalForward    `toml:"local_forwards,omitempty" comment:"Local Port Forwarding (-L)"`
	RemoteForwards    []LocalForward    `toml:"remote_forwards,omitempty" comment:"Remote Port Forwarding (-R)"`
	DynamicForwards   []DynamicForward  `toml:"dynamic_forwards,omitempty" comment:"Dynamic Socks Proxy (-D)"`
}

// TelnetOptions holds connection-specific Telnet options
type TelnetOptions struct {
	IPVersion    string `toml:"ip_version" comment:"IP Protocol: any | 4 | 6"`
	BindAddress  string `toml:"bind_address" comment:"Bind Address"`
	EscapeChar   string `toml:"escape_char" comment:"Escape Character (default ^])"`
	NAWS         bool   `toml:"naws" comment:"Send window size (NAWS)"`
	TerminalType string `toml:"terminal_type" comment:"Terminal type (default XTERM-256COLOR)"`
	LineEnding   string `toml:"line_ending" comment:"Line ending: crlf | crnul | lf"`
}

// SendStringOptions defines periodic string sending
type SendStringOptions struct {
	Enabled      bool   `toml:"enabled" comment:"Programatically send string"`
	Text         string `toml:"text" comment:"Text to send"`
	AppendReturn bool   `toml:"append_return" comment:"Send Return with text"`
	Interval     int    `toml:"interval" comment:"Interval in seconds"`
	OnlyWhenIdle bool   `toml:"only_when_idle" comment:"Only send when terminal is idle"`
}

// PrePostExec holds pre or post execution command entries
type PrePostExec struct {
	Command string `toml:"command" comment:"Command to execute"`
	Ask     bool   `toml:"ask" comment:"Ask before running"`
	Default bool   `toml:"default" comment:"Default selection in ask dialog"`
}

// ExpectEntry represents an automated expect step
type ExpectEntry struct {
	ID           string `toml:"id" comment:"Stable short ID linking secret in secrets.age"`
	Enabled      bool   `toml:"enabled" comment:"Enabled"`
	Pattern      string `toml:"pattern" comment:"Expect regexp"`
	Send         string `toml:"send" comment:"Send string (empty if hidden)"`
	Hidden       bool   `toml:"hidden" comment:"Hide string in secrets.age"`
	AppendReturn bool   `toml:"append_return" comment:"Append Return after sending"`
	OnMatch      string `toml:"on_match" comment:"If match: next | stop | goto"`
	OnMatchGoto  int    `toml:"on_match_goto" comment:"Index to jump to if match"`
	OnFail       string `toml:"on_fail" comment:"If fail: close | stop | goto"`
	OnFailGoto   int    `toml:"on_fail_goto" comment:"Index to jump to if fail"`
	Timeout      int    `toml:"timeout" comment:"Timeout in seconds (0 = use terminal default)"`
}

// UserVariable holds a connection-specific user variable
type UserVariable struct {
	ID     string `toml:"id" comment:"Stable ID linking secret in secrets.age"`
	Value  string `toml:"value" comment:"Value (empty if hidden)"`
	Hidden bool   `toml:"hidden" comment:"Hide string in secrets.age"`
}

// ConnectionTerminal holds connection-specific terminal overrides
type ConnectionTerminal struct {
	UsePersonal    bool                `toml:"use_personal" comment:"Use these personal options"`
	OpenIn         string              `toml:"open_in" comment:"Open NEW connection on: tab | window"`
	WindowWidth    int                 `toml:"window_width" comment:"Window width"`
	WindowHeight   int                 `toml:"window_height" comment:"Window height"`
	TimeoutConnect int                 `toml:"timeout_connect" comment:"Connection timeout (sec)"`
	TimeoutCommand int                 `toml:"timeout_command" comment:"Expect Cmd exec timeout (sec)"`
	TabColor       string              `toml:"tab_color" comment:"Tab background color (#rrggbb)"`
	Prompts        PromptsConfig       `toml:"prompts" comment:"Prompt regex overrides"`
	Look           *TerminalLookConfig `toml:"look,omitempty" comment:"Personal look override"`
}

// Connection represents a connection node in the tree
type Connection struct {
	ID              string             `toml:"id" comment:"UUID v4"`
	Parent          string             `toml:"parent" comment:"Parent group ID, empty string for root"`
	Name            string             `toml:"name" comment:"Connection name in tree"`
	Title           string             `toml:"title" comment:"TAB / Window Title"`
	Description     string             `toml:"description" comment:"Description text"`
	Protected       bool               `toml:"protected" comment:"Protected from changes"`
	Favourite       bool               `toml:"favourite" comment:"Marked as favourite"`
	Method          string             `toml:"method" comment:"ssh | telnet | sftp | ..."`
	Host            string             `toml:"host" comment:"Host IP or hostname"`
	Port            int                `toml:"port" comment:"Port number"`
	LaunchAtStartup bool               `toml:"launch_at_startup" comment:"Launch at startup"`
	AutoReconnect   bool               `toml:"auto_reconnect" comment:"Reconnect on disconnection"`
	SendDelayMs     int                `toml:"send_delay_ms" comment:"Delay in ms for automated char sending"`
	Clusters        []string           `toml:"clusters,omitempty" comment:"Configured cluster names"`
	Screenshots     []string           `toml:"screenshots,omitempty" comment:"Screenshot paths"`
	MAC             string             `toml:"mac" comment:"MAC address for Wake on LAN"`
	SFTPBrowser     string             `toml:"sftp_browser" comment:"SFTP browser: global | open | closed"`
	SFTPStartDir    string             `toml:"sftp_start_dir" comment:"SFTP start folder"`
	Auth            ConnectionAuth     `toml:"auth" comment:"Authentication"`
	Network         ConnectionNetwork  `toml:"network" comment:"Network and proxy"`
	SSH             SSHOptions         `toml:"ssh" comment:"SSH options"`
	Telnet          TelnetOptions      `toml:"telnet" comment:"Telnet options"`
	SendString      SendStringOptions  `toml:"send_string" comment:"Send string options"`
	SessionLogs     SessionLogsConfig  `toml:"session_logs" comment:"Session log options"`
	PreExec         []PrePostExec      `toml:"pre_exec,omitempty" comment:"Pre-execution commands"`
	PostExec        []PrePostExec      `toml:"post_exec,omitempty" comment:"Post-execution commands"`
	Expect          []ExpectEntry      `toml:"expect,omitempty" comment:"Expect automation list"`
	Macros          MacrosConfig       `toml:"macros" comment:"Connection macros"`
	Variables       []UserVariable     `toml:"variables,omitempty" comment:"User variables (<V:n>)"`
	Terminal        ConnectionTerminal `toml:"terminal" comment:"Terminal options"`
}

// Config is the root configuration structure mapped to config.toml
type Config struct {
	SchemaVersion int                 `toml:"schema_version" comment:"Configuration schema version"`
	General       GeneralConfig       `toml:"general" comment:"Preferences > Main Options > Behaviour"`
	Appearance    AppearanceConfig    `toml:"appearance" comment:"Preferences > Main Options > Look and Feel"`
	Prompts       PromptsConfig       `toml:"prompts" comment:"Preferences > Main Options > Advanced"`
	Terminal      TerminalConfig      `toml:"terminal" comment:"Preferences > Terminal Options"`
	SFTPBrowser   SFTPBrowserConfig   `toml:"sftp_browser" comment:"SFTP browser side panel settings"`
	LocalShell    LocalShellConfig    `toml:"local_shell" comment:"Local Shell options"`
	Network       NetworkConfig       `toml:"network" comment:"Global Network Settings"`
	Variables     []VariableConfig    `toml:"variables,omitempty" comment:"Global Variables (<GV:name>)"`
	Macros        MacrosConfig        `toml:"macros" comment:"Global Macros"`
	AutoClusters  []AutoClusterConfig `toml:"auto_clusters,omitempty" comment:"Auto Cluster definitions"`
	KeePass       KeePassConfig       `toml:"keepass" comment:"KeePassXC settings"`
	Keybindings   KeybindingsConfig   `toml:"keybindings" comment:"Keybinding overrides"`
	Groups        []Group             `toml:"groups,omitempty" comment:"Connection groups"`
	Connections   []Connection        `toml:"connections,omitempty" comment:"Saved connections"`
}
