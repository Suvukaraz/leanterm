package vars

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/platform"
	"github.com/Suvukaraz/leanterm/internal/secrets"
)

// AskHandler is a callback for interactive variable substitution (<ASK:...>)
type AskHandler func(prompt string, options []string, masked bool) (val string, cancelled bool, err error)

// KeePassResolver defines the interface for resolving KeePassXC tags.
type KeePassResolver interface {
	SubstKeePass(ctx context.Context, text string) (result string, hasHidden bool, err error)
	IsEnabled() bool
}

// SubstContext provides all connection and environment values needed for substitution
type SubstContext struct {
	UUID       string
	Socks5Port int
	Timestamp  time.Time
	Name       string
	Title      string
	IP         string
	Port       int
	User       string
	Pass       string
	Variables  []config.UserVariable
	Globals    []config.VariableConfig
	Vault      *secrets.Vault
	ConnID     string
	AskHandler AskHandler
	KeePass    KeePassResolver
}

// SubstResult contains the substituted text and extracted control metadata
type SubstResult struct {
	Text      string
	Ctrl      string
	Tee       string
	Pipes     []string
	Prompt    string
	Pos       int
	Cancelled bool
	HasHidden bool
}

var (
	reGV    = regexp.MustCompile(`<GV:([^>]+)>`)
	reV     = regexp.MustCompile(`<V:([0-9]+)>`)
	reEnv   = regexp.MustCompile(`<ENV:([A-Za-z0-9_]+)>`)
	reAsk   = regexp.MustCompile(`<ASK:([^>]+)>`)
	reCmd   = regexp.MustCompile(`<CMD:([^>]+)>`)
	reCtrl  = regexp.MustCompile(`<CTRL_X:([^>]+)>`)
	reTee   = regexp.MustCompile(`<TEE:([^>]+)>`)
	rePipe  = regexp.MustCompile(`<PIPE:([^>]+)>`)
)

// Subst executes variable substitution in the exact order specified in section 8.6:
// 1. Built-in variables (<UUID>, <SOCKS5_PORT>, <TIMESTAMP>, <DATE_*>, <TIME_*>, <NAME>, <TITLE>, <IP>, <PORT>, <USER>, <PASS>)
// 2. <GV:name> from global variables
// 3. <V:n> from connection variables
// 4. <ENV:NAME> from environment
// 5. Interactive <ASK:...>
// 6. <CMD:command> from local execution
// 7. Control tags (<CTRL_X:...>, <TEE:...>, <PIPE:...>)
// Unknown tags remain unchanged. There is no recursive substitution.
func Subst(input string, ctx SubstContext) SubstResult {
	result := SubstResult{
		Text: input,
	}

	if input == "" {
		return result
	}

	ts := ctx.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	// 1. Built-in variables
	replacements := map[string]string{
		"<UUID>":        ctx.UUID,
		"<SOCKS5_PORT>": strconv.Itoa(ctx.Socks5Port),
		"<TIMESTAMP>":   strconv.FormatInt(ts.Unix(), 10),
		"<DATE_Y>":      ts.Format("2006"),
		"<DATE_M>":      ts.Format("01"),
		"<DATE_D>":      ts.Format("02"),
		"<TIME_H>":      ts.Format("15"),
		"<TIME_M>":      ts.Format("04"),
		"<TIME_S>":      ts.Format("05"),
		"<NAME>":        ctx.Name,
		"<TITLE>":       ctx.Title,
		"<IP>":          ctx.IP,
		"<PORT>":        strconv.Itoa(ctx.Port),
		"<USER>":        ctx.User,
	}

	// Replace built-ins except PASS first
	for k, v := range replacements {
		result.Text = strings.ReplaceAll(result.Text, k, v)
	}

	// PASS replacement
	if strings.Contains(result.Text, "<PASS>") {
		result.Text = strings.ReplaceAll(result.Text, "<PASS>", ctx.Pass)
		result.HasHidden = true
	}

	// 2. <GV:name> from global variables
	result.Text = reGV.ReplaceAllStringFunc(result.Text, func(match string) string {
		sub := reGV.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		varName := sub[1]
		for _, gv := range ctx.Globals {
			if gv.Name == varName {
				if gv.Hidden {
					result.HasHidden = true
					if ctx.Vault != nil {
						return ctx.Vault.GetGlobal(varName)
					}
					return ""
				}
				return gv.Value
			}
		}
		return match // Unknown stays unchanged
	})

	// 3. <V:n> from connection variables
	result.Text = reV.ReplaceAllStringFunc(result.Text, func(match string) string {
		sub := reV.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		idx, err := strconv.Atoi(sub[1])
		if err != nil || idx < 0 || idx >= len(ctx.Variables) {
			return match
		}
		v := ctx.Variables[idx]
		if v.Hidden {
			result.HasHidden = true
			if ctx.Vault != nil && ctx.ConnID != "" {
				key := fmt.Sprintf("var_%d", idx)
				val := ctx.Vault.GetConnection(ctx.ConnID, key)
				if val == "" && v.ID != "" {
					val = ctx.Vault.GetConnection(ctx.ConnID, v.ID)
				}
				return val
			}
			return ""
		}
		return v.Value
	})

	// 4. <ENV:NAME> from process environment
	result.Text = reEnv.ReplaceAllStringFunc(result.Text, func(match string) string {
		sub := reEnv.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		envName := sub[1]
		if val, ok := os.LookupEnv(envName); ok {
			return val
		}
		return match
	})

	// 5. Interactive <ASK:...>
	if ctx.AskHandler != nil && strings.Contains(result.Text, "<ASK:") {
		var askErr error
		var wasCancelled bool

		result.Text = reAsk.ReplaceAllStringFunc(result.Text, func(match string) string {
			if wasCancelled || askErr != nil {
				return match
			}
			sub := reAsk.FindStringSubmatch(match)
			if len(sub) < 2 {
				return match
			}
			content := sub[1]

			var prompt string
			var options []string

			if strings.Contains(content, "|") {
				// Combo options: desc|opt1|opt2...
				parts := strings.Split(content, "|")
				prompt = parts[0]
				options = parts[1:]
			} else if _, err := strconv.Atoi(content); err == nil {
				// Numbered: <ASK:n>
				prompt = fmt.Sprintf("Variable substitution '%s'", content)
			} else {
				// Text: <ASK:text>
				prompt = fmt.Sprintf("Please, enter a value for: '%s'", content)
			}

			val, cancelled, err := ctx.AskHandler(prompt, options, false)
			if err != nil {
				askErr = err
				return match
			}
			if cancelled {
				wasCancelled = true
				return match
			}
			return val
		})

		if wasCancelled {
			result.Cancelled = true
			return result
		}
	}

	// 6. <CMD:command> from local execution
	result.Text = reCmd.ReplaceAllStringFunc(result.Text, func(match string) string {
		sub := reCmd.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		cmdStr := sub[1]
		out, err := executeLocalCmd(cmdStr, 30*time.Second)
		if err != nil {
			return match
		}
		// If multi-line, wrap as echo "<output>"
		if strings.Contains(out, "\n") {
			return fmt.Sprintf("echo %q", out)
		}
		return out
	})

	// 7. Control tags (removed from Text, returned as metadata)
	// <CTRL_X:cmd>
	if loc := reCtrl.FindStringSubmatch(result.Text); len(loc) >= 2 {
		result.Ctrl = loc[1]
		result.Text = reCtrl.ReplaceAllString(result.Text, "")
	}

	// <TEE:file>
	if loc := reTee.FindStringSubmatch(result.Text); len(loc) >= 2 {
		result.Tee = loc[1]
		result.Text = reTee.ReplaceAllString(result.Text, "")
	}

	// <PIPE:cmd> or <PIPE:cmd:prompt>
	result.Text = rePipe.ReplaceAllStringFunc(result.Text, func(match string) string {
		sub := rePipe.FindStringSubmatch(match)
		if len(sub) >= 2 {
			pipeSpec := sub[1]
			parts := strings.SplitN(pipeSpec, ":", 2)
			result.Pipes = append(result.Pipes, parts[0])
			if len(parts) == 2 {
				result.Prompt = parts[1]
			}
		}
		return ""
	})

	// 8. KeePass masks <field|entry>
	if ctx.KeePass != nil && ctx.KeePass.IsEnabled() {
		kpOut, hasHidden, err := ctx.KeePass.SubstKeePass(context.Background(), result.Text)
		if err == nil {
			result.Text = kpOut
			if hasHidden {
				result.HasHidden = true
			}
		}
	}

	return result
}

func executeLocalCmd(command string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := platform.ShellCommand(ctx, command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	out := strings.TrimRight(stdout.String(), "\r\n")
	return out, nil
}
