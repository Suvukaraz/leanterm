//go:build windows

package session

import (
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"github.com/charmbracelet/x/xpty"
	"golang.org/x/sys/windows"
)

// killOnClose: see xptyShell.Close.
const killOnClose = true

func checkPtySupport() error {
	if err := windows.NewLazySystemDLL("kernel32.dll").NewProc("CreatePseudoConsole").Find(); err != nil {
		return errors.New("this Windows version has no pseudo console support (Windows 10 1809 or later is required)")
	}
	return nil
}

func prepareCmd(*exec.Cmd) {}
func afterStart(xpty.Pty)  {}

// defaultShell is used when no shell is configured: PowerShell 7, Windows
// PowerShell, else the command interpreter (%ComSpec%).
func defaultShell() string {
	for _, name := range []string{"pwsh.exe", "powershell.exe"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	if cs := os.Getenv("ComSpec"); cs != "" {
		return cs
	}
	return "cmd.exe"
}

// psCwdPrompt wraps the existing PowerShell prompt (profile, oh-my-posh, ...) so
// every prompt reports the current folder as OSC 9;9, like for Windows Terminal.
const psCwdPrompt = `$global:__LeanTermPrompt = $function:prompt
function global:prompt {
  $loc = $executionContext.SessionState.Path.CurrentLocation
  $out = & $global:__LeanTermPrompt
  if ($loc.Provider.Name -eq 'FileSystem') { "$([char]27)]9;9;$($loc.ProviderPath)$([char]27)\" + $out } else { $out }
}`

// withCwdReporting lets PowerShell and cmd report their folder (OSC 9;9) when
// they run without own options, so the file browser can follow them.
func withCwdReporting(shell string, args, env []string) ([]string, []string) {
	if len(args) > 0 {
		return args, env
	}
	switch strings.ToLower(strings.TrimSuffix(filepath.Base(shell), filepath.Ext(shell))) {
	case "pwsh", "powershell":
		return []string{"-NoExit", "-EncodedCommand", encodePowerShell(psCwdPrompt)}, env
	case "cmd":
		prompt := "$P$G"
		out := make([]string, 0, len(env)+1)
		for _, kv := range env {
			if len(kv) > 7 && strings.EqualFold(kv[:7], "PROMPT=") {
				prompt = kv[7:]
				continue
			}
			out = append(out, kv)
		}
		return args, append(out, `PROMPT=$E]9;9;$P$E\`+prompt)
	}
	return args, env
}

// encodePowerShell encodes a script for -EncodedCommand (Base64 of UTF-16LE).
func encodePowerShell(script string) string {
	u := utf16.Encode([]rune(script))
	b := make([]byte, 2*len(u))
	for i, c := range u {
		b[2*i] = byte(c)
		b[2*i+1] = byte(c >> 8)
	}
	return base64.StdEncoding.EncodeToString(b)
}
