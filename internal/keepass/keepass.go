package keepass

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Suvukaraz/leanterm/internal/config"
)

var (
	ErrDisabled       = errors.New("KeePassXC integration is disabled")
	ErrDatabaseNotSet = errors.New("KeePassXC database path is not configured")
	ErrEntryNotFound  = errors.New("KeePassXC entry or attribute not found")

	reKeePassTag = regexp.MustCompile(`<([A-Za-z0-9_.:-]+)\|([^>]+)>`)
)

type cacheItem struct {
	value     string
	expiresAt time.Time
}

// CmdRunner abstracts command execution for testability.
type CmdRunner func(ctx context.Context, stdin string, name string, args ...string) ([]byte, error)

func defaultCmdRunner(ctx context.Context, stdin string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errStr := strings.TrimSpace(stderr.String())
		if errStr != "" {
			return nil, fmt.Errorf("%w: %s", err, errStr)
		}
		return nil, err
	}

	return stdout.Bytes(), nil
}

// Client manages interactions with keepassxc-cli.
type Client struct {
	mu             sync.RWMutex
	enabled        bool
	database       string
	keyFile        string
	masterPassword string
	cache          map[string]cacheItem
	ttl            time.Duration
	runner         CmdRunner
}

// Options configures the KeePass Client.
type Options struct {
	Config         config.KeePassConfig
	MasterPassword string
	TTL            time.Duration
	Runner         CmdRunner
}

// NewClient initializes a KeePassXC client.
func NewClient(opts Options) *Client {
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	runner := opts.Runner
	if runner == nil {
		runner = defaultCmdRunner
	}

	return &Client{
		enabled:        opts.Config.Enabled,
		database:       opts.Config.Database,
		keyFile:        opts.Config.KeyFile,
		masterPassword: opts.MasterPassword,
		cache:          make(map[string]cacheItem),
		ttl:            ttl,
		runner:         runner,
	}
}

// IsEnabled returns true if KeePassXC integration is enabled and configured.
func (c *Client) IsEnabled() bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled && c.database != ""
}

// SetMasterPassword updates the master password in memory.
func (c *Client) SetMasterPassword(password string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.masterPassword = password
	// Invalidate cache on password change
	c.cache = make(map[string]cacheItem)
}

// ClearCache flushes the in-memory cache.
func (c *Client) ClearCache() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]cacheItem)
}

// GetField queries an attribute of an entry from the KeePass database.
func (c *Client) GetField(ctx context.Context, entry, field string) (string, error) {
	if c == nil || !c.enabled {
		return "", ErrDisabled
	}
	if c.database == "" {
		return "", ErrDatabaseNotSet
	}

	cacheKey := entry + "::" + strings.ToLower(field)

	// Check cache
	c.mu.RLock()
	if item, found := c.cache[cacheKey]; found && time.Now().Before(item.expiresAt) {
		c.mu.RUnlock()
		return item.value, nil
	}
	c.mu.RUnlock()

	// Build CLI arguments: keepassxc-cli show -s [-k keyfile] database entry -a attribute
	args := []string{"show", "-s"}
	if c.keyFile != "" {
		args = append(args, "-k", c.keyFile)
	}
	args = append(args, c.database, entry, "-a", field)

	stdin := ""
	if c.masterPassword != "" {
		stdin = c.masterPassword + "\n"
	}

	outBytes, err := c.runner(ctx, stdin, "keepassxc-cli", args...)
	if err != nil {
		return "", fmt.Errorf("keepassxc-cli failed for '%s|%s': %w", field, entry, err)
	}

	val := strings.TrimRight(string(outBytes), "\r\n")

	// Store in cache
	c.mu.Lock()
	c.cache[cacheKey] = cacheItem{
		value:     val,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	return val, nil
}

// SubstKeePass replaces KeePass masks `<field|entry>` in text.
// If any sensitive field (Password) is substituted, hasHidden is returned as true.
func (c *Client) SubstKeePass(ctx context.Context, text string) (result string, hasHidden bool, err error) {
	if c == nil || !c.IsEnabled() {
		return text, false, nil
	}

	result = text
	var firstErr error

	result = reKeePassTag.ReplaceAllStringFunc(text, func(match string) string {
		sub := reKeePassTag.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		field := strings.TrimSpace(sub[1])
		entry := strings.TrimSpace(sub[2])

		val, err := c.GetField(ctx, entry, field)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return match
		}

		if strings.EqualFold(field, "password") || strings.EqualFold(field, "pass") {
			hasHidden = true
		}

		return val
	})

	return result, hasHidden, firstErr
}
