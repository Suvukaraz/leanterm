//go:build windows

package session

import (
	"io"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

// openSSHAgentPipe is the pipe of the Windows OpenSSH agent service (ssh-agent).
const openSSHAgentPipe = `\\.\pipe\openssh-ssh-agent`

// dialAgent connects to the SSH agent: a named pipe in SSH_AUTH_SOCK (e.g. set
// up for Pageant or KeeAgent), else the Windows OpenSSH agent.
func dialAgent() (io.ReadWriteCloser, error) {
	pipe := openSSHAgentPipe
	if sock := os.Getenv("SSH_AUTH_SOCK"); strings.HasPrefix(sock, `\\.\pipe\`) {
		pipe = sock
	}
	name, err := windows.UTF16PtrFromString(pipe)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: pipe, Err: err}
	}
	return os.NewFile(uintptr(h), pipe), nil
}
