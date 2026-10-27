package session

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/Suvukaraz/leanterm/internal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
)

type DialOptions struct {
	NetworkMode    string
	Host           string
	Port           int
	Timeout        time.Duration
	IPVersion      string // "any", "4", "6"
	BindAddress    string
	Socks          config.ProxySocksConfig
	SocksPassword  string
	Jump           config.ProxyJumpConfig
	JumpPassword   string
	GlobalNetwork  config.NetworkConfig
}

// DialTransport connects to the remote host using direct TCP, SOCKS5, or SSH jump host.
func DialTransport(ctx context.Context, opts DialOptions) (net.Conn, error) {
	network := "tcp"
	if opts.IPVersion == "4" {
		network = "tcp4"
	} else if opts.IPVersion == "6" {
		network = "tcp6"
	}

	targetAddr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	dialer := &net.Dialer{
		Timeout: opts.Timeout,
	}

	if opts.BindAddress != "" {
		ip := net.ParseIP(opts.BindAddress)
		if ip != nil {
			dialer.LocalAddr = &net.TCPAddr{IP: ip}
		}
	}

	effectiveMode := opts.NetworkMode
	if effectiveMode == "global" || effectiveMode == "" {
		effectiveMode = opts.GlobalNetwork.Mode
	}
	if effectiveMode == "" {
		effectiveMode = "direct"
	}

	switch effectiveMode {
	case "direct":
		return dialer.DialContext(ctx, network, targetAddr)

	case "socks":
		socksHost := opts.Socks.Host
		socksPort := opts.Socks.Port
		socksUser := opts.Socks.User
		socksPwd := opts.SocksPassword

		if socksHost == "" {
			socksHost = opts.GlobalNetwork.Socks.Host
			socksPort = opts.GlobalNetwork.Socks.Port
			socksUser = opts.GlobalNetwork.Socks.User
		}

		if socksHost == "" {
			return nil, fmt.Errorf("SOCKS proxy configured but host is empty")
		}
		if socksPort <= 0 {
			socksPort = 1080
		}

		socksAddr := net.JoinHostPort(socksHost, strconv.Itoa(socksPort))
		var auth *proxy.Auth
		if socksUser != "" || socksPwd != "" {
			auth = &proxy.Auth{
				User:     socksUser,
				Password: socksPwd,
			}
		}

		socksDialer, err := proxy.SOCKS5(network, socksAddr, auth, dialer)
		if err != nil {
			return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
		}

		// Connect using SOCKS dialer with context support
		type contextDialer interface {
			DialContext(ctx context.Context, network, addr string) (net.Conn, error)
		}
		if cd, ok := socksDialer.(contextDialer); ok {
			return cd.DialContext(ctx, network, targetAddr)
		}
		return socksDialer.Dial(network, targetAddr)

	case "jump":
		jumpHost := opts.Jump.Host
		jumpPort := opts.Jump.Port
		jumpUser := opts.Jump.User

		if jumpHost == "" {
			jumpHost = opts.GlobalNetwork.Jump.Host
			jumpPort = opts.GlobalNetwork.Jump.Port
			jumpUser = opts.GlobalNetwork.Jump.User
		}
		if jumpHost == "" {
			return nil, fmt.Errorf("Jump host configured but host is empty")
		}
		if jumpPort <= 0 {
			jumpPort = 22
		}

		// Auth methods for jump host
		var authMethods []ssh.AuthMethod
		if opts.JumpPassword != "" {
			authMethods = append(authMethods, ssh.Password(opts.JumpPassword))
		}

		jumpCfg := &ssh.ClientConfig{
			User:            jumpUser,
			Auth:            authMethods,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         opts.Timeout,
		}

		jumpAddr := net.JoinHostPort(jumpHost, strconv.Itoa(jumpPort))
		jumpClient, err := ssh.Dial(network, jumpAddr, jumpCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to dial jump host %s: %w", jumpAddr, err)
		}

		return jumpClient.Dial(network, targetAddr)

	default:
		return dialer.DialContext(ctx, network, targetAddr)
	}
}
