package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dlclark/regexp2"
)

var hexColorRegex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// ValidationError represents a specific configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (v ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", v.Field, v.Message)
}

// ValidationResult holds errors and warnings from a validation pass
type ValidationResult struct {
	Errors   []ValidationError
	Warnings []string
}

func (vr *ValidationResult) AddError(field, message string) {
	vr.Errors = append(vr.Errors, ValidationError{Field: field, Message: message})
}

func (vr *ValidationResult) AddWarning(msg string) {
	vr.Warnings = append(vr.Warnings, msg)
}

func (vr ValidationResult) HasErrors() bool {
	return len(vr.Errors) > 0
}

func (vr ValidationResult) Error() error {
	if !vr.HasErrors() {
		return nil
	}
	var sb strings.Builder
	sb.WriteString("configuration validation errors:\n")
	for _, e := range vr.Errors {
		sb.WriteString(fmt.Sprintf("  - %s\n", e.Error()))
	}
	return fmt.Errorf("%s", sb.String())
}

// Validate checks all configuration values, enums, ports, colors, regexes, and tree integrity.
func Validate(cfg *Config) ValidationResult {
	res := ValidationResult{}

	// General enums
	switch cfg.General.Layout {
	case "traditional", "compact":
	default:
		res.AddError("general.layout", fmt.Sprintf("invalid layout '%s', must be 'traditional' or 'compact'", cfg.General.Layout))
	}

	switch cfg.General.Theme {
	case "default", "asbru-color", "asbru-dark", "system":
	default:
		res.AddError("general.theme", fmt.Sprintf("invalid theme '%s'", cfg.General.Theme))
	}

	switch cfg.General.ColorScheme {
	case "dark", "light":
	default:
		res.AddError("general.color_scheme", fmt.Sprintf("invalid color_scheme '%s', must be 'dark' or 'light'", cfg.General.ColorScheme))
	}

	// Appearance enums
	switch cfg.Appearance.StartTreeOn {
	case "connections", "favourites", "history", "clusters":
	default:
		res.AddError("appearance.start_tree_on", fmt.Sprintf("invalid start_tree_on '%s'", cfg.Appearance.StartTreeOn))
	}

	switch cfg.Appearance.TreePosition {
	case "left", "right":
	default:
		res.AddError("appearance.tree_position", fmt.Sprintf("invalid tree_position '%s'", cfg.Appearance.TreePosition))
	}

	switch cfg.SFTPBrowser.Position {
	case "left", "right":
	default:
		res.AddError("sftp_browser.position", fmt.Sprintf("invalid position '%s'", cfg.SFTPBrowser.Position))
	}

	switch cfg.Appearance.ButtonBarPosition {
	case "top", "bottom":
	default:
		res.AddError("appearance.button_bar_position", fmt.Sprintf("invalid button_bar_position '%s'", cfg.Appearance.ButtonBarPosition))
	}

	switch cfg.Appearance.ProtectedColorTarget {
	case "foreground", "background":
	default:
		res.AddError("appearance.protected_color_target", "must be 'foreground' or 'background'")
	}

	switch cfg.Appearance.UnprotectedColorTarget {
	case "foreground", "background":
	default:
		res.AddError("appearance.unprotected_color_target", "must be 'foreground' or 'background'")
	}

	if cfg.Appearance.ProtectedColor != "" && !hexColorRegex.MatchString(cfg.Appearance.ProtectedColor) {
		res.AddError("appearance.protected_color", fmt.Sprintf("invalid hex color '%s'", cfg.Appearance.ProtectedColor))
	}
	if cfg.Appearance.UnprotectedColor != "" && !hexColorRegex.MatchString(cfg.Appearance.UnprotectedColor) {
		res.AddError("appearance.unprotected_color", fmt.Sprintf("invalid hex color '%s'", cfg.Appearance.UnprotectedColor))
	}

	switch cfg.Appearance.ScreenshotViewer {
	case "internal", "external":
	default:
		res.AddError("appearance.screenshot_viewer", "must be 'internal' or 'external'")
	}

	// Prompts regex validation
	validateRegex(&res, "prompts.command", cfg.Prompts.Command)
	validateRegex(&res, "prompts.username", cfg.Prompts.Username)
	validateRegex(&res, "prompts.password", cfg.Prompts.Password)
	validateRegex(&res, "prompts.host_key_verification", cfg.Prompts.HostKeyVerification)
	validateRegex(&res, "prompts.remote_host_key_changed", cfg.Prompts.RemoteHostKeyChanged)
	validateRegex(&res, "prompts.press_any_key", cfg.Prompts.PressAnyKey)

	// Terminal enums
	switch cfg.Terminal.WhenLastTabClosed {
	case "nothing", "close", "hide":
	default:
		res.AddError("terminal.when_last_tab_closed", "must be 'nothing', 'close', or 'hide'")
	}

	switch cfg.Terminal.OpenNewIn {
	case "tab", "window":
	default:
		res.AddError("terminal.open_new_in", "must be 'tab' or 'window'")
	}

	switch cfg.Terminal.MacrosBar {
	case "never", "combo", "buttons":
	default:
		res.AddError("terminal.macros_bar", "must be 'never', 'combo', or 'buttons'")
	}

	switch cfg.Terminal.TabsPosition {
	case "top", "bottom", "left", "right":
	default:
		res.AddError("terminal.tabs_position", "must be 'top', 'bottom', 'left', or 'right'")
	}

	validateColor(&res, "terminal.colors.connected", cfg.Terminal.Colors.Connected)
	validateColor(&res, "terminal.colors.disconnected", cfg.Terminal.Colors.Disconnected)
	validateColor(&res, "terminal.colors.new_data", cfg.Terminal.Colors.NewData)

	validateLook(&res, "terminal.look", &cfg.Terminal.Look)

	// SFTP browser
	if cfg.SFTPBrowser.Width < 200 {
		res.AddError("sftp_browser.width", "width must be at least 200")
	}
	if cfg.SFTPBrowser.ParallelTransfers < 1 || cfg.SFTPBrowser.ParallelTransfers > 8 {
		res.AddError("sftp_browser.parallel_transfers", "parallel_transfers must be between 1 and 8")
	}

	// Network
	switch cfg.Network.Mode {
	case "direct", "socks", "jump":
	default:
		res.AddError("network.mode", "must be 'direct', 'socks', or 'jump'")
	}
	if cfg.Network.Socks.Host != "" {
		validatePort(&res, "network.socks.port", cfg.Network.Socks.Port)
	}
	if cfg.Network.Jump.Host != "" {
		validatePort(&res, "network.jump.port", cfg.Network.Jump.Port)
	}

	// Auto clusters regex validation
	for i, ac := range cfg.AutoClusters {
		prefix := fmt.Sprintf("auto_clusters[%d]", i)
		validateRegex(&res, prefix+".match_name", ac.MatchName)
		validateRegex(&res, prefix+".match_title", ac.MatchTitle)
		validateRegex(&res, prefix+".match_host", ac.MatchHost)
		validateRegex(&res, prefix+".match_description", ac.MatchDescription)
	}

	// Tree integrity
	validateTreeIntegrity(&res, cfg)

	return res
}

func validateColor(res *ValidationResult, field, val string) {
	if val != "" && !hexColorRegex.MatchString(val) {
		res.AddError(field, fmt.Sprintf("invalid hex color '%s', must be #rrggbb", val))
	}
}

func validatePort(res *ValidationResult, field string, port int) {
	if port < 1 || port > 65535 {
		res.AddError(field, fmt.Sprintf("invalid port %d, must be 1-65535", port))
	}
}

func validateRegex(res *ValidationResult, field, pattern string) {
	if pattern == "" {
		return
	}
	if _, err := regexp2.Compile(pattern, regexp2.Multiline); err != nil {
		res.AddError(field, fmt.Sprintf("invalid regex '%s': %v", pattern, err))
	}
}

func validateLook(res *ValidationResult, prefix string, look *TerminalLookConfig) {
	if look == nil {
		return
	}
	validateColor(res, prefix+".foreground", look.Foreground)
	validateColor(res, prefix+".background", look.Background)
	validateColor(res, prefix+".bold", look.Bold)

	switch look.CursorShape {
	case "block", "ibeam", "underline":
	default:
		res.AddError(prefix+".cursor_shape", "must be 'block', 'ibeam', or 'underline'")
	}

	switch look.Backspace {
	case "auto", "ascii-backspace", "ascii-delete", "delete-sequence", "tty":
	default:
		res.AddError(prefix+".backspace", "invalid backspace binding")
	}

	for i, c := range look.Palette {
		validateColor(res, fmt.Sprintf("%s.palette[%d]", prefix, i), c)
	}
}

func validateTreeIntegrity(res *ValidationResult, cfg *Config) {
	seenIDs := make(map[string]bool)
	allGroupIDs := make(map[string]bool)

	// Check group IDs
	for i := range cfg.Groups {
		g := &cfg.Groups[i]
		if g.ID == "" {
			res.AddError(fmt.Sprintf("groups[%d].id", i), "group ID cannot be empty")
			continue
		}
		if seenIDs[g.ID] {
			res.AddError(fmt.Sprintf("groups[%d].id", i), fmt.Sprintf("duplicate ID '%s'", g.ID))
		}
		seenIDs[g.ID] = true
		allGroupIDs[g.ID] = true
	}

	// Check connection IDs and details
	for i := range cfg.Connections {
		c := &cfg.Connections[i]
		if c.ID == "" {
			res.AddError(fmt.Sprintf("connections[%d].id", i), "connection ID cannot be empty")
			continue
		}
		if seenIDs[c.ID] {
			res.AddError(fmt.Sprintf("connections[%d].id", i), fmt.Sprintf("duplicate ID '%s'", c.ID))
		}
		seenIDs[c.ID] = true

		switch c.Method {
		case "ssh", "telnet", "sftp":
		default:
			res.AddError(fmt.Sprintf("connections[%d].method", i), fmt.Sprintf("unknown method '%s'", c.Method))
		}

		if c.Port != 0 {
			validatePort(res, fmt.Sprintf("connections[%d].port", i), c.Port)
		}

		switch c.SFTPBrowser {
		case "global", "open", "closed":
		default:
			res.AddError(fmt.Sprintf("connections[%d].sftp_browser", i), "must be 'global', 'open', or 'closed'")
		}

		switch c.Auth.Type {
		case "password", "publickey", "manual":
		default:
			res.AddError(fmt.Sprintf("connections[%d].auth.type", i), "must be 'password', 'publickey', or 'manual'")
		}

		switch c.Network.Mode {
		case "global", "direct", "socks", "jump":
		default:
			res.AddError(fmt.Sprintf("connections[%d].network.mode", i), "must be 'global', 'direct', 'socks', or 'jump'")
		}

		if c.Network.Socks.Host != "" {
			validatePort(res, fmt.Sprintf("connections[%d].network.socks.port", i), c.Network.Socks.Port)
		}
		if c.Network.Jump.Host != "" {
			validatePort(res, fmt.Sprintf("connections[%d].network.jump.port", i), c.Network.Jump.Port)
		}

		// SSH forwards validation
		seenLocalPorts := make(map[string]bool)
		for j, lf := range c.SSH.LocalForwards {
			validatePort(res, fmt.Sprintf("connections[%d].ssh.local_forwards[%d].port", i, j), lf.Port)
			validatePort(res, fmt.Sprintf("connections[%d].ssh.local_forwards[%d].host_port", i, j), lf.HostPort)
			key := fmt.Sprintf("%s:%d", lf.BindAddress, lf.Port)
			if seenLocalPorts[key] {
				res.AddError(fmt.Sprintf("connections[%d].ssh.local_forwards[%d]", i, j),
					fmt.Sprintf("CONFIG ERROR: Local Port %d on Local IP '%s' defined in Local Port Forwarding is already in use!", lf.Port, lf.BindAddress))
			}
			seenLocalPorts[key] = true
		}

		seenRemotePorts := make(map[int]bool)
		for j, rf := range c.SSH.RemoteForwards {
			validatePort(res, fmt.Sprintf("connections[%d].ssh.remote_forwards[%d].port", i, j), rf.Port)
			validatePort(res, fmt.Sprintf("connections[%d].ssh.remote_forwards[%d].host_port", i, j), rf.HostPort)
			if seenRemotePorts[rf.Port] {
				res.AddWarning(fmt.Sprintf("WARNING: Same Remote Port %d defined more than once in connection '%s'!", rf.Port, c.Name))
			}
			seenRemotePorts[rf.Port] = true
		}

		for j, df := range c.SSH.DynamicForwards {
			validatePort(res, fmt.Sprintf("connections[%d].ssh.dynamic_forwards[%d].port", i, j), df.Port)
		}

		// Expect list regexes
		for j, exp := range c.Expect {
			validateRegex(res, fmt.Sprintf("connections[%d].expect[%d].pattern", i, j), exp.Pattern)
			switch exp.OnMatch {
			case "next", "stop", "goto":
			default:
				res.AddError(fmt.Sprintf("connections[%d].expect[%d].on_match", i, j), "must be 'next', 'stop', or 'goto'")
			}
			switch exp.OnFail {
			case "close", "stop", "goto":
			default:
				res.AddError(fmt.Sprintf("connections[%d].expect[%d].on_fail", i, j), "must be 'close', 'stop', or 'goto'")
			}
		}

		// Terminal overrides
		if c.Terminal.UsePersonal {
			validateColor(res, fmt.Sprintf("connections[%d].terminal.tab_color", i), c.Terminal.TabColor)
			validateRegex(res, fmt.Sprintf("connections[%d].terminal.prompts.command", i), c.Terminal.Prompts.Command)
			validateRegex(res, fmt.Sprintf("connections[%d].terminal.prompts.username", i), c.Terminal.Prompts.Username)
			validateRegex(res, fmt.Sprintf("connections[%d].terminal.prompts.password", i), c.Terminal.Prompts.Password)
			validateLook(res, fmt.Sprintf("connections[%d].terminal.look", i), c.Terminal.Look)
		}
	}

	// Check orphans & cycles for groups
	parentMap := make(map[string]string)
	for i := range cfg.Groups {
		g := &cfg.Groups[i]
		if g.Parent != "" && !allGroupIDs[g.Parent] {
			res.AddWarning(fmt.Sprintf("Group '%s' (ID %s) has non-existent parent '%s'; reattached to root", g.Name, g.ID, g.Parent))
			g.Parent = ""
		}
		parentMap[g.ID] = g.Parent
	}

	// Detect cycles in groups
	for i := range cfg.Groups {
		g := &cfg.Groups[i]
		visited := make(map[string]bool)
		curr := g.ID
		for curr != "" {
			if visited[curr] {
				res.AddWarning(fmt.Sprintf("Cycle detected in parent hierarchy of group '%s' (ID %s); breaking cycle by reattaching to root", g.Name, g.ID))
				g.Parent = ""
				parentMap[g.ID] = ""
				break
			}
			visited[curr] = true
			curr = parentMap[curr]
		}
	}

	// Check connections parent orphans
	for i := range cfg.Connections {
		c := &cfg.Connections[i]
		if c.Parent != "" && !allGroupIDs[c.Parent] {
			res.AddWarning(fmt.Sprintf("Connection '%s' (ID %s) has non-existent parent '%s'; reattached to root", c.Name, c.ID, c.Parent))
			c.Parent = ""
		}
	}
}
