package session

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/Suvukaraz/leanterm/internal/config"
)

type PromptMatcherOptions struct {
	AuthType     string // "password", "manual", etc.
	Username     string
	Password     string
	LineEnding   string
	Prompts      config.PromptsConfig
	TimeoutSec   int
	OnSend       func(text string)
	OnStatus     func(code string)
	OnConnected  func(note string)
	OnError      func(errCode string)
}

type PromptMatcher struct {
	mu             sync.Mutex
	opts           PromptMatcherOptions
	userRe         *regexp2.Regexp
	passRe         *regexp2.Regexp
	anyKeyRe       *regexp2.Regexp
	cmdRe          *regexp2.Regexp
	buffer         []byte
	step           int // 0: waiting user, 1: waiting pass, 2: waiting cmd, 3: connected
	passSent       bool
	manualTakeover bool
	timer          *time.Timer
	stopped        bool
}

func compileRegexOrDefault(pattern, fallback string) *regexp2.Regexp {
	if strings.TrimSpace(pattern) != "" {
		if re, err := regexp2.Compile(pattern, regexp2.IgnoreCase); err == nil {
			return re
		}
	}
	if re, err := regexp2.Compile(fallback, regexp2.IgnoreCase); err == nil {
		return re
	}
	re, _ := regexp2.Compile(`(?i)([$#%>]\s*$)`, regexp2.IgnoreCase)
	return re
}

func matchRe(re *regexp2.Regexp, s string) bool {
	if re == nil {
		return false
	}
	m, err := re.MatchString(s)
	return err == nil && m
}

func NewPromptMatcher(opts PromptMatcherOptions) *PromptMatcher {
	if opts.LineEnding == "" {
		opts.LineEnding = "\r\n"
	}
	if opts.TimeoutSec <= 0 {
		opts.TimeoutSec = 40
	}

	pm := &PromptMatcher{
		opts:     opts,
		userRe:   compileRegexOrDefault(opts.Prompts.Username, config.DefaultUsernamePrompt),
		passRe:   compileRegexOrDefault(opts.Prompts.Password, config.DefaultPasswordPrompt),
		anyKeyRe: compileRegexOrDefault(opts.Prompts.PressAnyKey, config.DefaultPressAnyKeyPrompt),
		cmdRe:    compileRegexOrDefault(opts.Prompts.Command, config.DefaultCommandPrompt),
	}

	if opts.AuthType == "manual" {
		pm.step = 3
		if opts.OnConnected != nil {
			opts.OnConnected("")
		}
		return pm
	}

	// Timeout timer for automatic connection
	pm.timer = time.AfterFunc(time.Duration(opts.TimeoutSec)*time.Second, func() {
		pm.mu.Lock()
		defer pm.mu.Unlock()
		if pm.step < 3 && !pm.stopped {
			pm.step = 3
			if pm.opts.OnConnected != nil {
				pm.opts.OnConnected("no prompt detected")
			}
		}
	})

	return pm
}

func (pm *PromptMatcher) FeedOutput(data []byte) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.stopped || pm.step >= 3 || pm.manualTakeover {
		return
	}

	pm.buffer = append(pm.buffer, data...)
	bufStr := string(pm.buffer)

	switch pm.step {
	case 0:
		// Check for username prompt
		if matchRe(pm.userRe, bufStr) {
			if strings.TrimSpace(pm.opts.Username) == "" {
				pm.stopped = true
				if pm.timer != nil {
					pm.timer.Stop()
				}
				if pm.opts.OnError != nil {
					pm.opts.OnError("CLOSE:LOGIN:Empty username is not valid to login here")
				}
				return
			}

			pm.buffer = nil
			pm.step = 1
			if pm.opts.OnStatus != nil {
				pm.opts.OnStatus(fmt.Sprintf("LOGIN:%s", pm.opts.Username))
			}
			if pm.opts.OnSend != nil {
				pm.opts.OnSend(pm.opts.Username + pm.opts.LineEnding)
			}
			return
		}

		// Direct password prompt
		if matchRe(pm.passRe, bufStr) {
			pm.buffer = nil
			pm.step = 2
			pm.passSent = true
			if pm.opts.OnStatus != nil {
				pm.opts.OnStatus("PASSWORD:Sent (not shown)")
			}
			if pm.opts.OnSend != nil {
				pm.opts.OnSend(pm.opts.Password + pm.opts.LineEnding)
			}
			return
		}

		// Press any key prompt
		if matchRe(pm.anyKeyRe, bufStr) {
			pm.buffer = nil
			if pm.opts.OnStatus != nil {
				pm.opts.OnStatus("PRESSKEY:sending 'return' to continue connecting")
			}
			if pm.opts.OnSend != nil {
				pm.opts.OnSend(pm.opts.LineEnding)
			}
			return
		}

		// Command prompt
		if matchRe(pm.cmdRe, bufStr) {
			pm.finishConnectedLocked("")
			return
		}

	case 1:
		// Waiting for password prompt after username was sent
		if matchRe(pm.passRe, bufStr) {
			pm.buffer = nil
			pm.step = 2
			pm.passSent = true
			if pm.opts.OnStatus != nil {
				pm.opts.OnStatus("PASSWORD:Sent (not shown)")
			}
			if pm.opts.OnSend != nil {
				pm.opts.OnSend(pm.opts.Password + pm.opts.LineEnding)
			}
			return
		}

		if matchRe(pm.anyKeyRe, bufStr) {
			pm.buffer = nil
			if pm.opts.OnStatus != nil {
				pm.opts.OnStatus("PRESSKEY:sending 'return' to continue connecting")
			}
			if pm.opts.OnSend != nil {
				pm.opts.OnSend(pm.opts.LineEnding)
			}
			return
		}

		if matchRe(pm.cmdRe, bufStr) {
			pm.finishConnectedLocked("")
			return
		}

	case 2:
		// Waiting for command prompt after password was sent
		if matchRe(pm.cmdRe, bufStr) {
			pm.finishConnectedLocked("")
			return
		}

		// Second password prompt: rejected!
		if pm.passSent && matchRe(pm.passRe, bufStr) {
			pm.manualTakeover = true
			if pm.timer != nil {
				pm.timer.Stop()
			}
			if pm.opts.OnStatus != nil {
				pm.opts.OnStatus(fmt.Sprintf(
					"CLOSE:PASSWORD:Provided username/password '%s/<<hidden_password>>' was rejected",
					pm.opts.Username,
				))
			}
			return
		}

		if matchRe(pm.anyKeyRe, bufStr) {
			pm.buffer = nil
			if pm.opts.OnSend != nil {
				pm.opts.OnSend(pm.opts.LineEnding)
			}
			return
		}
	}
}

func (pm *PromptMatcher) finishConnectedLocked(note string) {
	pm.step = 3
	pm.buffer = nil
	if pm.timer != nil {
		pm.timer.Stop()
	}
	if pm.opts.OnConnected != nil {
		pm.opts.OnConnected(note)
	}
}

func (pm *PromptMatcher) ManualInput() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.manualTakeover = true
}

func (pm *PromptMatcher) Stop() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.stopped = true
	if pm.timer != nil {
		pm.timer.Stop()
	}
}
