package forward

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

// DisplayInfo holds parsed X11 display information.
type DisplayInfo struct {
	Raw        string
	IsUnix     bool
	SocketPath string
	TCPAddr    string
	ScreenNum  uint32
}

type x11RequestMsg struct {
	SingleConnection bool
	AuthProtocol     string
	AuthCookie       string
	ScreenNumber     uint32
}

// ParseDisplay parses an X11 DISPLAY string (e.g. ":0", ":0.0", "localhost:10.0", "unix:0").
func ParseDisplay(d string) (*DisplayInfo, error) {
	d = strings.TrimSpace(d)
	if d == "" {
		d = ":0"
	}

	info := &DisplayInfo{Raw: d}

	// Remove leading "unix:" if present
	isExplicitUnix := false
	if strings.HasPrefix(d, "unix:") {
		isExplicitUnix = true
		d = strings.TrimPrefix(d, "unix:")
	}

	var host, rest string
	colonIdx := strings.LastIndex(d, ":")
	if colonIdx >= 0 {
		host = d[:colonIdx]
		rest = d[colonIdx+1:]
	} else if isExplicitUnix {
		host = ""
		rest = d
	} else {
		return nil, fmt.Errorf("invalid DISPLAY format: %s", info.Raw)
	}

	parts := strings.Split(rest, ".")
	displayNum, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid display number in DISPLAY '%s': %w", info.Raw, err)
	}

	screenNum := 0
	if len(parts) > 1 {
		screenNum, _ = strconv.Atoi(parts[1])
	}
	info.ScreenNum = uint32(screenNum)

	if host == "" || host == "localhost" || host == "127.0.0.1" || isExplicitUnix {
		// Prefer UNIX domain socket on Linux
		socketPath := fmt.Sprintf("/tmp/.X11-unix/X%d", displayNum)
		// Check if unix socket exists or if explicit unix or host empty
		if host == "" || isExplicitUnix {
			info.IsUnix = true
			info.SocketPath = socketPath
			return info, nil
		}
	}

	// TCP fallback or non-empty remote host
	if host == "" {
		host = "127.0.0.1"
	}
	info.IsUnix = false
	info.TCPAddr = net.JoinHostPort(host, strconv.Itoa(6000+displayNum))
	return info, nil
}

// GetX11Cookie attempts to retrieve the MIT-MAGIC-COOKIE for the display using xauth,
// falling back to a cryptographically secure random 16-byte cookie.
func GetX11Cookie(display string) (protocol string, cookieHex string, err error) {
	if display == "" {
		display = os.Getenv("DISPLAY")
		if display == "" {
			display = ":0"
		}
	}

	cmd := exec.Command("xauth", "list", display)
	out, err := cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				return fields[1], fields[2], nil
			}
		}
	}

	// Fallback: Generate random 16-byte cookie
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "MIT-MAGIC-COOKIE-1", "00000000000000000000000000000000", err
	}
	return "MIT-MAGIC-COOKIE-1", hex.EncodeToString(b), nil
}

// SetupX11 requests X11 forwarding on the SSH session and forwards incoming X11
// channel requests to the local X11 display.
func SetupX11(ctx context.Context, sess *ssh.Session, client *ssh.Client) error {
	display := os.Getenv("DISPLAY")
	if display == "" {
		display = ":0"
	}

	info, err := ParseDisplay(display)
	if err != nil {
		return err
	}

	proto, cookie, _ := GetX11Cookie(display)

	req := x11RequestMsg{
		SingleConnection: false,
		AuthProtocol:     proto,
		AuthCookie:       cookie,
		ScreenNumber:     info.ScreenNum,
	}

	ok, err := sess.SendRequest("x11-req", true, ssh.Marshal(req))
	if err != nil {
		return fmt.Errorf("x11-req error: %w", err)
	}
	if !ok {
		return errors.New("x11-req rejected by remote SSH server")
	}

	x11Chans := client.HandleChannelOpen("x11")
	if x11Chans == nil {
		return nil
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case newChan, ok := <-x11Chans:
				if !ok {
					return
				}
				go handleX11Channel(ctx, newChan, info)
			}
		}
	}()

	return nil
}

func handleX11Channel(ctx context.Context, newChan ssh.NewChannel, info *DisplayInfo) {
	ch, reqs, err := newChan.Accept()
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)

	var localConn net.Conn
	if info.IsUnix {
		localConn, err = net.Dial("unix", info.SocketPath)
		if err != nil && info.TCPAddr != "" {
			localConn, err = net.Dial("tcp", info.TCPAddr)
		}
	} else {
		localConn, err = net.Dial("tcp", info.TCPAddr)
	}

	if err != nil {
		_ = ch.Close()
		return
	}

	go func() {
		<-ctx.Done()
		_ = localConn.Close()
		_ = ch.Close()
	}()

	pipeStreams(ch, localConn)
}

type closeWriter interface {
	CloseWrite() error
}

func pipeStreams(c1, c2 io.ReadWriteCloser) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(c1, c2)
		if cw, ok := c1.(closeWriter); ok {
			_ = cw.CloseWrite()
		} else {
			_ = c1.Close()
		}
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(c2, c1)
		if cw, ok := c2.(closeWriter); ok {
			_ = cw.CloseWrite()
		} else {
			_ = c2.Close()
		}
	}()

	wg.Wait()
}
