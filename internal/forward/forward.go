package forward

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"

	"github.com/Suvukaraz/leanterm/internal/config"
	"golang.org/x/crypto/ssh"
)

type Forwarder struct {
	mu        sync.Mutex
	listeners []io.Closer
	cancel    context.CancelFunc
	ctx       context.Context
	summary   []string
	fullNames []string
}

func NewForwarder() *Forwarder {
	return &Forwarder{}
}

func (f *Forwarder) Summary(fullNames bool) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if fullNames {
		return fmt.Sprintf("%v", f.fullNames)
	}
	res := ""
	for _, s := range f.summary {
		if res != "" {
			res += " "
		}
		res += s
	}
	return res
}

func (f *Forwarder) Start(
	ctx context.Context,
	client *ssh.Client,
	opts config.SSHOptions,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.ctx, f.cancel = context.WithCancel(ctx)

	// 1. Local forwards (-L)
	for _, lf := range opts.LocalForwards {
		bindAddr := lf.BindAddress
		if bindAddr == "" {
			if opts.GatewayPorts {
				bindAddr = "0.0.0.0"
			} else {
				bindAddr = "127.0.0.1"
			}
		}
		localEndpoint := net.JoinHostPort(bindAddr, strconv.Itoa(lf.Port))
		remoteEndpoint := net.JoinHostPort(lf.Host, strconv.Itoa(lf.HostPort))

		listener, err := net.Listen("tcp", localEndpoint)
		if err != nil {
			return fmt.Errorf("local forward listen %s failed: %w", localEndpoint, err)
		}
		f.listeners = append(f.listeners, listener)
		f.summary = append(f.summary, fmt.Sprintf("-L %d:%s:%d", lf.Port, lf.Host, lf.HostPort))
		f.fullNames = append(f.fullNames, fmt.Sprintf("Local Forwarding: %d:%s:%d", lf.Port, lf.Host, lf.HostPort))

		go f.handleLocalListener(listener, client, remoteEndpoint)
	}

	// 2. Remote forwards (-R)
	for _, rf := range opts.RemoteForwards {
		remoteBind := rf.BindAddress
		if remoteBind == "" {
			remoteBind = "127.0.0.1"
		}
		remoteEndpoint := net.JoinHostPort(remoteBind, strconv.Itoa(rf.Port))
		localEndpoint := net.JoinHostPort(rf.Host, strconv.Itoa(rf.HostPort))

		listener, err := client.Listen("tcp", remoteEndpoint)
		if err != nil {
			return fmt.Errorf("remote forward listen %s failed: %w", remoteEndpoint, err)
		}
		f.listeners = append(f.listeners, listener)
		f.summary = append(f.summary, fmt.Sprintf("-R %d:%s:%d", rf.Port, rf.Host, rf.HostPort))
		f.fullNames = append(f.fullNames, fmt.Sprintf("Remote Forwarding: %d:%s:%d", rf.Port, rf.Host, rf.HostPort))

		go f.handleRemoteListener(listener, localEndpoint)
	}

	// 3. Dynamic forwards (-D / SOCKS5)
	for _, df := range opts.DynamicForwards {
		bindAddr := df.BindAddress
		if bindAddr == "" {
			if opts.GatewayPorts {
				bindAddr = "0.0.0.0"
			} else {
				bindAddr = "127.0.0.1"
			}
		}
		endpoint := net.JoinHostPort(bindAddr, strconv.Itoa(df.Port))
		listener, err := net.Listen("tcp", endpoint)
		if err != nil {
			return fmt.Errorf("dynamic forward listen %s failed: %w", endpoint, err)
		}
		f.listeners = append(f.listeners, listener)
		f.summary = append(f.summary, fmt.Sprintf("-D %d", df.Port))
		f.fullNames = append(f.fullNames, fmt.Sprintf("Dynamic Forwarding: %d", df.Port))

		go f.handleSocks5Listener(listener, client)
	}

	return nil
}

func (f *Forwarder) handleLocalListener(listener net.Listener, client *ssh.Client, remoteEndpoint string) {
	for {
		localConn, err := listener.Accept()
		if err != nil {
			return
		}
		go func(lConn net.Conn) {
			defer lConn.Close()
			rConn, err := client.Dial("tcp", remoteEndpoint)
			if err != nil {
				return
			}
			defer rConn.Close()
			pipe(lConn, rConn)
		}(localConn)
	}
}

func (f *Forwarder) handleRemoteListener(listener net.Listener, localEndpoint string) {
	for {
		remoteConn, err := listener.Accept()
		if err != nil {
			return
		}
		go func(rConn net.Conn) {
			defer rConn.Close()
			lConn, err := net.Dial("tcp", localEndpoint)
			if err != nil {
				return
			}
			defer lConn.Close()
			pipe(rConn, lConn)
		}(remoteConn)
	}
}

// Minimal SOCKS5 RFC 1928 server proxying through sshClient
func (f *Forwarder) handleSocks5Listener(listener net.Listener, client *ssh.Client) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go f.handleSocks5Conn(conn, client)
	}
}

func (f *Forwarder) handleSocks5Conn(conn net.Conn, client *ssh.Client) {
	defer conn.Close()

	buf := make([]byte, 256)
	// Handshake: read version and methods
	if _, err := io.ReadFull(conn, buf[:2]); err != nil || buf[0] != 0x05 {
		return
	}
	nMethods := int(buf[1])
	if _, err := io.ReadFull(conn, buf[:nMethods]); err != nil {
		return
	}
	// Reply: NO AUTHENTICATION REQUIRED (0x00)
	if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
		return
	}

	// Request details: VER, CMD, RSV, ATYP
	if _, err := io.ReadFull(conn, buf[:4]); err != nil || buf[0] != 0x05 || buf[1] != 0x01 {
		// Only CONNECT (0x01) supported
		_, _ = conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	var targetHost string
	switch buf[3] {
	case 0x01: // IPv4
		if _, err := io.ReadFull(conn, buf[:4]); err != nil {
			return
		}
		targetHost = net.IP(buf[:4]).String()
	case 0x03: // Domain name
		if _, err := io.ReadFull(conn, buf[:1]); err != nil {
			return
		}
		dLen := int(buf[0])
		if _, err := io.ReadFull(conn, buf[:dLen]); err != nil {
			return
		}
		targetHost = string(buf[:dLen])
	case 0x04: // IPv6
		if _, err := io.ReadFull(conn, buf[:16]); err != nil {
			return
		}
		targetHost = net.IP(buf[:16]).String()
	default:
		return
	}

	// Port
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return
	}
	targetPort := (int(buf[0]) << 8) | int(buf[1])
	targetEndpoint := net.JoinHostPort(targetHost, strconv.Itoa(targetPort))

	// Dial target through SSH
	remoteConn, err := client.Dial("tcp", targetEndpoint)
	if err != nil {
		_, _ = conn.Write([]byte{0x05, 0x04, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) // Host unreachable
		return
	}
	defer remoteConn.Close()

	// Reply SUCCESS
	if _, err := conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 127, 0, 0, 1, 0, 0}); err != nil {
		return
	}

	pipe(conn, remoteConn)
}

func pipe(conn1, conn2 net.Conn) {
	pipeStreams(conn1, conn2)
}

func (f *Forwarder) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.cancel != nil {
		f.cancel()
	}
	var errs []error
	for _, l := range f.listeners {
		if err := l.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	f.listeners = nil
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
