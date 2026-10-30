package session

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/platform"
	"github.com/Suvukaraz/leanterm/internal/vars"
)

// PrePostAskHandler prompts the user to select which commands to run.
// Returns the selected entries and whether the user chose to skip execution.
type PrePostAskHandler func(phase string, entries []config.PrePostExec) (selected []config.PrePostExec, skip bool)

// PrePostOptions configures pre/post execution.
type PrePostOptions struct {
	Phase      string // "PRE_EXEC" or "POST_EXEC"
	Entries    []config.PrePostExec
	SubstCtx   vars.SubstContext
	AskHandler PrePostAskHandler
	Stream     *Stream
	StatusFunc func(string)
}

// ExecutePrePost runs pre-exec or post-exec commands according to Section 8.7.
func ExecutePrePost(ctx context.Context, opts PrePostOptions) error {
	if len(opts.Entries) == 0 {
		return nil
	}

	var hasAsk bool
	for _, e := range opts.Entries {
		if e.Ask {
			hasAsk = true
			break
		}
	}

	var toRun []config.PrePostExec

	if hasAsk && opts.AskHandler != nil {
		selected, skip := opts.AskHandler(opts.Phase, opts.Entries)
		if skip {
			if opts.StatusFunc != nil {
				opts.StatusFunc(fmt.Sprintf("%s:Skipped by user", opts.Phase))
			}
			return nil
		}
		toRun = selected
	} else {
		for _, e := range opts.Entries {
			if e.Default {
				toRun = append(toRun, e)
			}
		}
	}

	for _, entry := range toRun {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if strings.TrimSpace(entry.Command) == "" {
			continue
		}

		substRes := vars.Subst(entry.Command, opts.SubstCtx)
		cmdStr := substRes.Text

		if opts.StatusFunc != nil {
			if substRes.HasHidden {
				opts.StatusFunc(fmt.Sprintf("%s:<<HIDDEN COMMAND>>", opts.Phase))
			} else {
				opts.StatusFunc(fmt.Sprintf("%s:%s", opts.Phase, cmdStr))
			}
		}

		// Grey terminal header
		if opts.Stream != nil {
			header := fmt.Sprintf("\r\n\x1b[90m[%s] Executing: %s\x1b[0m\r\n", opts.Phase, cmdStr)
			_, _ = opts.Stream.Write([]byte(header))
		}

		// Execute with 60s timeout per command
		cmdCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		cmd := platform.ShellCommand(cmdCtx, cmdStr)

		var outBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &outBuf

		err := cmd.Run()
		cancel()

		// Stream grey output lines to terminal
		if opts.Stream != nil && outBuf.Len() > 0 {
			scanner := bufio.NewScanner(&outBuf)
			for scanner.Scan() {
				line := scanner.Text()
				greyLine := fmt.Sprintf("\x1b[90m%s\x1b[0m\r\n", line)
				_, _ = opts.Stream.Write([]byte(greyLine))
			}
		}

		if err != nil {
			if opts.StatusFunc != nil {
				opts.StatusFunc(fmt.Sprintf("%s:ERROR:%v", opts.Phase, err))
			}
			if opts.Stream != nil {
				errLine := fmt.Sprintf("\x1b[31m[%s] Failed: %v\x1b[0m\r\n", opts.Phase, err)
				_, _ = opts.Stream.Write([]byte(errLine))
			}
		} else {
			if opts.StatusFunc != nil {
				opts.StatusFunc(fmt.Sprintf("%s:SUCCESS", opts.Phase))
			}
		}
	}

	return nil
}

// GreyWriter wraps an io.Writer to format every line in grey ANSI (90m).
type GreyWriter struct {
	w io.Writer
}

func NewGreyWriter(w io.Writer) *GreyWriter {
	return &GreyWriter{w: w}
}

func (gw *GreyWriter) Write(p []byte) (n int, err error) {
	lines := strings.Split(string(p), "\n")
	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			continue
		}
		formatted := fmt.Sprintf("\x1b[90m%s\x1b[0m\r\n", line)
		if _, err := gw.w.Write([]byte(formatted)); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}
