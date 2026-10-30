//go:build windows

package session

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// conptyShell runs the shell in a Windows pseudo console (ConPTY, Windows 10 1809+).
// Output and input go through two anonymous pipes; ConPTY turns the console
// API calls of the shell into VT sequences for the terminal engine.
type conptyShell struct {
	hpc     windows.Handle
	process windows.Handle
	pid     int
	in      *os.File // our end of the input pipe
	out     *os.File // our end of the output pipe

	closeOnce sync.Once
	waitOnce  sync.Once
	waitErr   error
}

func startShellOS(ctx context.Context, spec shellSpec) (shellProcess, error) {
	if err := windows.NewLazySystemDLL("kernel32.dll").NewProc("CreatePseudoConsole").Find(); err != nil {
		return nil, errors.New("this Windows version has no pseudo console support (Windows 10 1809 or later is required)")
	}

	var inR, inW, outR, outW windows.Handle
	if err := windows.CreatePipe(&inR, &inW, nil, 0); err != nil {
		return nil, fmt.Errorf("create input pipe: %w", err)
	}
	if err := windows.CreatePipe(&outR, &outW, nil, 0); err != nil {
		closeHandles(inR, inW)
		return nil, fmt.Errorf("create output pipe: %w", err)
	}

	var hpc windows.Handle
	size := windows.Coord{X: int16(clampDim(spec.Cols)), Y: int16(clampDim(spec.Rows))}
	if err := windows.CreatePseudoConsole(size, inR, outW, 0, &hpc); err != nil {
		closeHandles(inR, inW, outR, outW)
		return nil, fmt.Errorf("create pseudo console: %w", err)
	}
	// The pseudo console holds its own copies of these ends.
	closeHandles(inR, outW)

	s := &conptyShell{
		hpc: hpc,
		in:  os.NewFile(uintptr(inW), "conpty-input"),
		out: os.NewFile(uintptr(outR), "conpty-output"),
	}
	if err := s.spawn(spec); err != nil {
		windows.ClosePseudoConsole(hpc)
		_ = s.in.Close()
		_ = s.out.Close()
		return nil, err
	}

	go func() {
		<-ctx.Done()
		_ = s.Close()
	}()
	return s, nil
}

func (s *conptyShell) spawn(spec shellSpec) error {
	path := spec.Path
	if resolved, err := exec.LookPath(path); err == nil {
		path = resolved
	}

	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return fmt.Errorf("attribute list: %w", err)
	}
	defer attrs.Delete()
	// The attribute value is the HPCON itself, not a pointer to it.
	hpc := s.hpc
	if err := attrs.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, *(*unsafe.Pointer)(unsafe.Pointer(&hpc)), unsafe.Sizeof(hpc)); err != nil {
		return fmt.Errorf("attach pseudo console: %w", err)
	}

	si := windows.StartupInfoEx{ProcThreadAttributeList: attrs.List()}
	si.Cb = uint32(unsafe.Sizeof(si))
	// No inherited standard handles: all I/O goes through the pseudo console.
	si.Flags = windows.STARTF_USESTDHANDLES

	cmdLine, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{path}, spec.Args...)))
	if err != nil {
		return err
	}
	var dir *uint16
	if spec.Dir != "" {
		if dir, err = windows.UTF16PtrFromString(spec.Dir); err != nil {
			return err
		}
	}
	env := environmentBlock(spec.Env)

	var pi windows.ProcessInformation
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT)
	if err := windows.CreateProcess(nil, cmdLine, nil, nil, false, flags, &env[0], dir, &si.StartupInfo, &pi); err != nil {
		return fmt.Errorf("start %s: %w", spec.Path, err)
	}
	_ = windows.CloseHandle(pi.Thread)
	s.process = pi.Process
	s.pid = int(pi.ProcessId)
	return nil
}

func (s *conptyShell) Read(p []byte) (int, error)  { return s.out.Read(p) }
func (s *conptyShell) Write(p []byte) (int, error) { return s.in.Write(p) }
func (s *conptyShell) Pid() int                    { return s.pid }

func (s *conptyShell) Resize(cols, rows int) error {
	return windows.ResizePseudoConsole(s.hpc, windows.Coord{X: int16(clampDim(cols)), Y: int16(clampDim(rows))})
}

func (s *conptyShell) Wait() error {
	s.waitOnce.Do(func() {
		if _, err := windows.WaitForSingleObject(s.process, windows.INFINITE); err != nil {
			s.waitErr = err
			return
		}
		var code uint32
		if err := windows.GetExitCodeProcess(s.process, &code); err == nil && code != 0 {
			s.waitErr = fmt.Errorf("exit status %d", code)
		}
	})
	return s.waitErr
}

// Close ends the shell and the pseudo console. ClosePseudoConsole waits until its
// output is read on older Windows versions, so the output pipe is closed after it
// in the background (the read loop keeps draining it meanwhile).
func (s *conptyShell) Close() error {
	s.closeOnce.Do(func() {
		_ = windows.TerminateProcess(s.process, 1)
		_ = s.in.Close()
		go func() {
			windows.ClosePseudoConsole(s.hpc)
			_ = s.out.Close()
			// Wait must finish before the process handle goes away.
			_ = s.Wait()
			_ = windows.CloseHandle(s.process)
		}()
	})
	return nil
}

func clampDim(n int) int {
	if n < 1 {
		return 1
	}
	if n > 32767 {
		return 32767
	}
	return n
}

func closeHandles(hs ...windows.Handle) {
	for _, h := range hs {
		_ = windows.CloseHandle(h)
	}
}

// environmentBlock builds a CreateProcess environment: "k=v\0k=v\0\0" in UTF-16.
func environmentBlock(env []string) []uint16 {
	var block []uint16
	for _, kv := range env {
		block = append(block, utf16.Encode([]rune(kv))...)
		block = append(block, 0)
	}
	if len(block) == 0 {
		block = append(block, 0)
	}
	return append(block, 0)
}

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
