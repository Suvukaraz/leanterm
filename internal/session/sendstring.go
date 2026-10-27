package session

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/vars"
)

// SendStringController manages periodic string sending to a terminal session.
type SendStringController struct {
	opts       config.SendStringOptions
	substCtx   vars.SubstContext
	sendFunc   func([]byte) error
	lineEnding string
	delayMs    int

	mu         sync.Mutex
	running    bool
	cancelFunc context.CancelFunc
	lastActive atomic.Int64 // unix timestamp in milliseconds
}

// NewSendStringController creates a new SendStringController.
func NewSendStringController(
	opts config.SendStringOptions,
	substCtx vars.SubstContext,
	sendFunc func([]byte) error,
	lineEnding string,
	delayMs int,
) *SendStringController {
	if lineEnding == "" {
		lineEnding = "\n"
	}
	c := &SendStringController{
		opts:       opts,
		substCtx:   substCtx,
		sendFunc:   sendFunc,
		lineEnding: lineEnding,
		delayMs:    delayMs,
	}
	c.lastActive.Store(time.Now().UnixMilli())
	return c
}

// MarkActive marks the terminal as active (used by only_when_idle).
func (c *SendStringController) MarkActive() {
	c.lastActive.Store(time.Now().UnixMilli())
}

// Start begins periodic string sending if enabled.
func (c *SendStringController) Start(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running || !c.opts.Enabled || c.opts.Interval <= 0 || c.opts.Text == "" {
		return
	}

	runCtx, cancel := context.WithCancel(ctx)
	c.cancelFunc = cancel
	c.running = true

	go c.runLoop(runCtx)
}

// Stop stops periodic sending.
func (c *SendStringController) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}
	c.running = false
	if c.cancelFunc != nil {
		c.cancelFunc()
		c.cancelFunc = nil
	}
}

// IsRunning returns whether periodic sending is currently active.
func (c *SendStringController) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

func (c *SendStringController) runLoop(ctx context.Context) {
	interval := time.Duration(c.opts.Interval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if c.opts.OnlyWhenIdle {
				now := time.Now().UnixMilli()
				last := c.lastActive.Load()
				if now-last < interval.Milliseconds() {
					// Session was active within the interval; skip sending
					continue
				}
			}

			// Perform variable substitution
			substRes := vars.Subst(c.opts.Text, c.substCtx)
			text := substRes.Text
			if c.opts.AppendReturn {
				text += c.lineEnding
			}

			if c.sendFunc != nil && len(text) > 0 {
				_ = sendWithDelay(ctx, c.sendFunc, text, c.delayMs)
			}
			c.lastActive.Store(time.Now().UnixMilli())
		}
	}
}
