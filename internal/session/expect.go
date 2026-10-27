package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/secrets"
	"github.com/Suvukaraz/leanterm/internal/vars"
)

// ExpectOptions configures the execution of the automated Expect engine.
type ExpectOptions struct {
	TmpID          string
	ConnID         string
	Entries        []config.ExpectEntry
	DefaultTimeout int // seconds (from timeout_command)
	SendDelayMs    int
	LineEnding     string
	SubstCtx       vars.SubstContext
	Vault          *secrets.Vault
	SendFunc       func([]byte) error
	StatusFunc     func(string)
	ExpectProgFunc func(index int, total int, state string)
	CloseFunc      func(reason string)
}

// ExpectRunner executes expect entries sequentially with jump, timeout, and substitution support.
type ExpectRunner struct {
	opts       ExpectOptions
	mu         sync.Mutex
	dataCh     chan []byte
	closed     bool
	cancelFunc context.CancelFunc
}

// NewExpectRunner creates a new ExpectRunner.
func NewExpectRunner(opts ExpectOptions) *ExpectRunner {
	if opts.LineEnding == "" {
		opts.LineEnding = "\n"
	}
	return &ExpectRunner{
		opts:   opts,
		dataCh: make(chan []byte, 100),
	}
}

// Feed pushes incoming terminal output bytes to the expect runner.
func (r *ExpectRunner) Feed(p []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || len(p) == 0 {
		return
	}
	// Defensive copy to prevent race
	cp := make([]byte, len(p))
	copy(cp, p)

	select {
	case r.dataCh <- cp:
	default:
		// Drop or queue
	}
}

// Close closes the data channel.
func (r *ExpectRunner) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed {
		r.closed = true
		close(r.dataCh)
	}
}

// Run executes the expect state machine according to Section 8.5.
func (r *ExpectRunner) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	r.cancelFunc = cancel
	defer cancel()

	// 1. Filter enabled entries
	var active []config.ExpectEntry
	for _, e := range r.opts.Entries {
		if e.Enabled {
			active = append(active, e)
		}
	}
	if len(active) == 0 {
		return
	}

	total := len(active)
	jumps := 0
	i := 0
	var accBuf []byte

	for i >= 0 && i < total {
		select {
		case <-ctx.Done():
			return
		default:
		}

		entry := active[i]
		ruleNum := i + 1

		// Empty pattern: send immediately
		if entry.Pattern == "" {
			if r.opts.ExpectProgFunc != nil {
				r.opts.ExpectProgFunc(ruleNum, total, "sending")
			}
			r.sendEntry(ctx, entry, i)
			accBuf = nil // reset accumulated buffer

			nextI, stop := r.handleMatchGoto(entry.OnMatch, entry.OnMatchGoto, i, total, &jumps)
			if stop {
				return
			}
			i = nextI
			continue
		}

		// Compile regex
		re, err := regexp2.Compile(entry.Pattern, regexp2.Multiline)
		if err != nil {
			if r.opts.StatusFunc != nil {
				r.opts.StatusFunc(fmt.Sprintf("EXPECT:ERROR:Invalid regex '%s': %v", entry.Pattern, err))
			}
			nextI, stop := r.handleFailGoto(entry.OnFail, entry.OnFailGoto, i, total, &jumps, 0, entry.Pattern)
			if stop {
				return
			}
			i = nextI
			continue
		}

		// Determine timeout
		timeoutSec := entry.Timeout
		if timeoutSec <= 0 {
			timeoutSec = r.opts.DefaultTimeout
		}

		if r.opts.StatusFunc != nil {
			r.opts.StatusFunc(fmt.Sprintf("EXPECT:WAITING:%s", entry.Pattern))
		}
		if r.opts.ExpectProgFunc != nil {
			r.opts.ExpectProgFunc(ruleNum, total, "waiting")
		}

		var timer *time.Timer
		var timerCh <-chan time.Time
		if timeoutSec > 0 {
			timer = time.NewTimer(time.Duration(timeoutSec) * time.Second)
			timerCh = timer.C
			defer timer.Stop()
		}

		// Check if accumulated buffer from prior output already matches
		if len(accBuf) > 0 {
			matched, _ := re.MatchString(string(accBuf))
			if matched {
				if timer != nil {
					timer.Stop()
				}
				if r.opts.ExpectProgFunc != nil {
					r.opts.ExpectProgFunc(ruleNum, total, "matched")
				}
				accBuf = nil
				r.sendEntry(ctx, entry, i)
				nextI, stop := r.handleMatchGoto(entry.OnMatch, entry.OnMatchGoto, i, total, &jumps)
				if stop {
					return
				}
				i = nextI
				continue
			}
		}

		// Wait loop for incoming chunks or timeout
		matched := false
	waitLoop:
		for {
			select {
			case <-ctx.Done():
				if timer != nil {
					timer.Stop()
				}
				return

			case chunk, ok := <-r.dataCh:
				if !ok {
					// EOF
					if timer != nil {
						timer.Stop()
					}
					if r.opts.StatusFunc != nil {
						r.opts.StatusFunc("CLOSE:Connection ended by remote peer!!")
					}
					if r.opts.CloseFunc != nil {
						r.opts.CloseFunc("Connection ended by remote peer!!")
					}
					return
				}

				accBuf = append(accBuf, chunk...)

				// Restart timeout timer on every received byte
				if timer != nil {
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(time.Duration(timeoutSec) * time.Second)
				}

				isMatch, _ := re.MatchString(string(accBuf))
				if isMatch {
					if timer != nil {
						timer.Stop()
					}
					matched = true
					if r.opts.ExpectProgFunc != nil {
						r.opts.ExpectProgFunc(ruleNum, total, "matched")
					}
					accBuf = nil // reset accumulated buffer
					r.sendEntry(ctx, entry, i)
					nextI, stop := r.handleMatchGoto(entry.OnMatch, entry.OnMatchGoto, i, total, &jumps)
					if stop {
						return
					}
					i = nextI
					break waitLoop
				}

			case <-timerCh:
				// Timeout!
				if r.opts.ExpectProgFunc != nil {
					r.opts.ExpectProgFunc(ruleNum, total, "timeout")
				}
				if r.opts.StatusFunc != nil {
					r.opts.StatusFunc(fmt.Sprintf("EXPECT:ON_FAIL:Timeout waiting for '%s'", entry.Pattern))
				}
				nextI, stop := r.handleFailGoto(entry.OnFail, entry.OnFailGoto, i, total, &jumps, timeoutSec, entry.Pattern)
				if stop {
					return
				}
				i = nextI
				break waitLoop
			}
		}

		if !matched && i == ruleNum-1 {
			// Protection against stalling if state wasn't updated
			i++
		}
	}
}

func (r *ExpectRunner) sendEntry(ctx context.Context, entry config.ExpectEntry, idx int) {
	var sendText string
	if entry.Hidden {
		if r.opts.Vault != nil && r.opts.ConnID != "" {
			if entry.ID != "" {
				sendText = r.opts.Vault.GetConnection(r.opts.ConnID, entry.ID)
			}
			if sendText == "" {
				sendText = r.opts.Vault.GetConnection(r.opts.ConnID, fmt.Sprintf("expect_%d", idx))
			}
		}
		if r.opts.StatusFunc != nil {
			r.opts.StatusFunc("EXPECT:SENDING:<<HIDDEN STRING>>")
		}
	} else {
		res := vars.Subst(entry.Send, r.opts.SubstCtx)
		sendText = res.Text
		if r.opts.StatusFunc != nil {
			if res.HasHidden {
				r.opts.StatusFunc("EXPECT:SENDING:<<HIDDEN STRING>>")
			} else {
				r.opts.StatusFunc(fmt.Sprintf("EXPECT:SENDING:%s", sendText))
			}
		}
	}

	if entry.AppendReturn {
		sendText += r.opts.LineEnding
	}

	if r.opts.SendFunc != nil && len(sendText) > 0 {
		_ = sendWithDelay(ctx, r.opts.SendFunc, sendText, r.opts.SendDelayMs)
	}
}

func (r *ExpectRunner) handleMatchGoto(action string, targetGoto int, currentIdx int, total int, jumps *int) (int, bool) {
	switch action {
	case "stop":
		if r.opts.StatusFunc != nil {
			r.opts.StatusFunc("EXPECT:ON_MATCH:Stopping by config")
		}
		return currentIdx, true

	case "goto":
		*jumps++
		if *jumps > 1000 {
			if r.opts.StatusFunc != nil {
				r.opts.StatusFunc("EXPECT:ERROR:Loop guard exceeded 1000 jumps")
			}
			return currentIdx, true
		}
		target := targetGoto
		if target > 0 {
			target = target - 1 // convert 1-based UI index to 0-based
		}
		if target < 0 || target >= total {
			if r.opts.StatusFunc != nil {
				r.opts.StatusFunc(fmt.Sprintf("EXPECT:ON_MATCH:Target index %d out of bounds (1..%d), stopping", targetGoto, total))
			}
			return currentIdx, true
		}
		if r.opts.StatusFunc != nil {
			r.opts.StatusFunc(fmt.Sprintf("EXPECT:ON_MATCH:Goto %d", target+1))
		}
		return target, false

	default: // "next"
		return currentIdx + 1, false
	}
}

func (r *ExpectRunner) handleFailGoto(action string, targetGoto int, currentIdx int, total int, jumps *int, timeoutSec int, pattern string) (int, bool) {
	switch action {
	case "close":
		msg := fmt.Sprintf("CLOSE:TIMEOUT:%d seconds expecting pattern '%s'!!", timeoutSec, pattern)
		if r.opts.StatusFunc != nil {
			r.opts.StatusFunc(msg)
		}
		if r.opts.CloseFunc != nil {
			r.opts.CloseFunc(msg)
		}
		return currentIdx, true

	case "stop":
		if r.opts.StatusFunc != nil {
			r.opts.StatusFunc("EXPECT:ON_FAIL:Stopping by config")
		}
		return currentIdx, true

	case "goto":
		*jumps++
		if *jumps > 1000 {
			if r.opts.StatusFunc != nil {
				r.opts.StatusFunc("EXPECT:ERROR:Loop guard exceeded 1000 jumps")
			}
			return currentIdx, true
		}
		target := targetGoto
		if target > 0 {
			target = target - 1 // convert 1-based UI index to 0-based
		}
		if target < 0 || target >= total {
			if r.opts.StatusFunc != nil {
				r.opts.StatusFunc(fmt.Sprintf("EXPECT:ON_FAIL:Target index %d out of bounds (1..%d), stopping", targetGoto, total))
			}
			return currentIdx, true
		}
		if r.opts.StatusFunc != nil {
			r.opts.StatusFunc(fmt.Sprintf("EXPECT:ON_FAIL:Goto %d", target+1))
		}
		return target, false

	default: // "close" by default in Ásbrú
		msg := fmt.Sprintf("CLOSE:TIMEOUT:%d seconds expecting pattern '%s'!!", timeoutSec, pattern)
		if r.opts.StatusFunc != nil {
			r.opts.StatusFunc(msg)
		}
		if r.opts.CloseFunc != nil {
			r.opts.CloseFunc(msg)
		}
		return currentIdx, true
	}
}

func sendWithDelay(ctx context.Context, sendFunc func([]byte) error, text string, delayMs int) error {
	if delayMs <= 0 {
		return sendFunc([]byte(text))
	}
	for _, ch := range text {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := sendFunc([]byte(string(ch))); err != nil {
			return err
		}
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}
	return nil
}
