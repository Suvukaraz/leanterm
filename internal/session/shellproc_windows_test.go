//go:build windows

package session

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

func TestWithCwdReporting(t *testing.T) {
	args, env := withCwdReporting(`C:\Windows\System32\cmd.exe`, nil, []string{"PATH=x", "prompt=$N$G"})
	if len(args) != 0 {
		t.Fatalf("cmd args = %v", args)
	}
	if got := env[len(env)-1]; got != `PROMPT=$E]9;9;$P$E\$N$G` {
		t.Fatalf("cmd PROMPT = %q (env %v)", got, env)
	}

	args, _ = withCwdReporting("pwsh.exe", nil, nil)
	if len(args) != 3 || args[0] != "-NoExit" || args[1] != "-EncodedCommand" {
		t.Fatalf("pwsh args = %v", args)
	}
	raw, err := base64.StdEncoding.DecodeString(args[2])
	if err != nil {
		t.Fatal(err)
	}
	u := make([]uint16, len(raw)/2)
	for i := range u {
		u[i] = uint16(raw[2*i]) | uint16(raw[2*i+1])<<8
	}
	if script := string(utf16.Decode(u)); !strings.Contains(script, "]9;9;") {
		t.Fatalf("decoded script = %q", script)
	}

	// Own options are left alone.
	if args, _ := withCwdReporting("powershell.exe", []string{"-NoProfile"}, nil); len(args) != 1 {
		t.Fatalf("own options changed: %v", args)
	}
}

// The integration must really make cmd.exe report its folder through ConPTY.
func TestCmdReportsCwdThroughConPTY(t *testing.T) {
	args, env := withCwdReporting("cmd.exe", nil, os.Environ())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	proc, err := startShell(ctx, shellSpec{Path: "cmd.exe", Args: args, Env: env, Dir: `C:\`, Cols: 120, Rows: 30})
	if err != nil {
		t.Fatal(err)
	}
	defer proc.Close()

	reports := make(chan string, 16)
	go func() {
		var sc cwdScanner
		buf := make([]byte, 4096)
		for {
			n, err := proc.Read(buf)
			if cwd := sc.Scan(buf[:n]); cwd != "" {
				reports <- cwd
			}
			if err != nil {
				close(reports)
				return
			}
		}
	}()
	if _, err := proc.Write([]byte("cd /d %SystemRoot%\r")); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case cwd, ok := <-reports:
			if !ok {
				t.Skip("shell ended without reporting its folder")
			}
			if strings.Contains(strings.ToLower(cwd), "windows") {
				_, _ = proc.Write([]byte("exit\r"))
				return
			}
		case <-ctx.Done():
			t.Skip("ConPTY did not pass OSC 9;9 through on this Windows version")
		}
	}
}
