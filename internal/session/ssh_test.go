package session

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Suvukaraz/leanterm/internal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// testSSHServer accepts password logins and runs a tiny echo "shell".
func testSSHServer(t *testing.T, password string) (addr string, hostKey ssh.PublicKey) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == "alice" && string(pass) == password {
				return nil, nil
			}
			return nil, io.EOF
		},
	}
	cfg.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			nc, err := ln.Accept()
			if err != nil {
				return
			}
			go serveConn(nc, cfg)
		}
	}()
	return ln.Addr().String(), signer.PublicKey()
}

func serveConn(nc net.Conn, cfg *ssh.ServerConfig) {
	_, chans, reqs, err := ssh.NewServerConn(nc, cfg)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)
	for nch := range chans {
		if nch.ChannelType() != "session" {
			_ = nch.Reject(ssh.UnknownChannelType, "unsupported")
			continue
		}
		ch, chReqs, err := nch.Accept()
		if err != nil {
			continue
		}
		go func() {
			for req := range chReqs {
				_ = req.Reply(req.Type == "pty-req" || req.Type == "shell" || req.Type == "env" || req.Type == "window-change", nil)
				if req.Type == "shell" {
					go func() {
						_, _ = ch.Write([]byte("welcome\r\n"))
						_, _ = io.Copy(ch, ch) // echo
					}()
				}
			}
		}()
	}
}

type recordingEmitter struct {
	mu     sync.Mutex
	states []string
}

func (e *recordingEmitter) Emit(event string, data any) {
	if event != "session:state" {
		return
	}
	m, _ := data.(map[string]any)
	e.mu.Lock()
	e.states = append(e.states, m["state"].(string))
	e.mu.Unlock()
}

func (e *recordingEmitter) last() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.states) == 0 {
		return ""
	}
	return e.states[len(e.states)-1]
}

type fakePrompter struct {
	mu    sync.Mutex
	kinds []string
	reply func(req PromptRequest) PromptResponse
}

func (p *fakePrompter) Prompt(_ context.Context, req PromptRequest) (PromptResponse, error) {
	p.mu.Lock()
	p.kinds = append(p.kinds, req.Kind)
	p.mu.Unlock()
	return p.reply(req), nil
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

func TestSSHSessionAsksForPasswordAndHostKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SSH_AUTH_SOCK", "")

	addr, _ := testSSHServer(t, "s3cret")
	host, portStr, _ := net.SplitHostPort(addr)
	var port int
	for _, c := range portStr {
		port = port*10 + int(c-'0')
	}

	prompter := &fakePrompter{reply: func(req PromptRequest) PromptResponse {
		switch req.Kind {
		case "hostkey-unknown":
			return PromptResponse{Choice: "Accept and save"}
		case "password":
			return PromptResponse{Choice: "ok", Values: map[string]string{"password": "s3cret"}, Remember: true}
		}
		return PromptResponse{Cancelled: true}
	}}

	var saved sync.Map
	em := &recordingEmitter{}
	sess := NewSSHSession(SSHSessionOptions{
		Conn: config.Connection{
			ID: "c1", Title: "test", Method: "ssh", Host: host, Port: port,
			Auth: config.ConnectionAuth{Type: "password", User: "alice", Exclusive: true},
		},
		Emitter:    em,
		Prompter:   prompter,
		SaveSecret: func(field, value string) { saved.Store(field, value) },
	})
	t.Cleanup(func() { _ = sess.Close() })

	if err := sess.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if sess.State() != "connected" {
		t.Fatalf("state = %s, want connected", sess.State())
	}
	if v, _ := saved.Load("password"); v != "s3cret" {
		t.Errorf("remembered password not saved, got %v", v)
	}
	prompter.mu.Lock()
	kinds := strings.Join(prompter.kinds, ",")
	prompter.mu.Unlock()
	if kinds != "hostkey-unknown,password" {
		t.Errorf("prompts = %s", kinds)
	}

	// The accepted key was written to known_hosts, so a reconnect does not ask again.
	data, err := os.ReadFile(filepath.Join(home, ".ssh", "known_hosts"))
	if err != nil || !strings.Contains(string(data), "ssh-ed25519") {
		t.Fatalf("known_hosts not written: %v %q", err, data)
	}

	// Disconnect, then Enter reconnects asynchronously without blocking Write.
	_ = sess.Disconnect()
	if em.last() != "disconnected" {
		t.Fatalf("last state = %s", em.last())
	}
	if err := sess.Write([]byte("\r")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return sess.State() == "connected" })
}

func TestSSHSessionStateStaysResponsiveWhileConnecting(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SSH_AUTH_SOCK", "")

	// A server that accepts TCP but never speaks SSH keeps the handshake pending.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close()
		}
	}()
	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port := 0
	for _, c := range portStr {
		port = port*10 + int(c-'0')
	}

	sess := NewSSHSession(SSHSessionOptions{
		Conn: config.Connection{
			ID: "c2", Title: "slow", Method: "ssh", Host: "127.0.0.1", Port: port,
			Auth: config.ConnectionAuth{Type: "password", User: "bob", Exclusive: true},
		},
		Password: "x",
	})
	done := make(chan struct{})
	go func() { _ = sess.Start(); close(done) }()
	waitFor(t, func() bool { return sess.State() == "connecting" })

	// These must not block on the pending handshake.
	start := time.Now()
	_ = sess.Resize(100, 30)
	_ = sess.State()
	if time.Since(start) > 200*time.Millisecond {
		t.Fatal("State/Resize blocked while connecting")
	}

	_ = sess.Close()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not abort the pending handshake")
	}
}

func TestHostKeyReplaceRemovesOffendingEntries(t *testing.T) {
	dir := t.TempDir()
	khPath := filepath.Join(dir, "known_hosts")

	_, oldPriv, _ := ed25519.GenerateKey(rand.Reader)
	oldSigner, _ := ssh.NewSignerFromKey(oldPriv)
	_, newPriv, _ := ed25519.GenerateKey(rand.Reader)
	newSigner, _ := ssh.NewSignerFromKey(newPriv)

	addr := knownhosts.Normalize("example.org:2222")
	hashed := knownhosts.HashHostname(addr)
	content := knownhosts.Line([]string{addr}, oldSigner.PublicKey()) + "\n" +
		hashed + " " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(oldSigner.PublicKey()))) + "\n" +
		knownhosts.Line([]string{"other.host"}, oldSigner.PublicKey()) + "\n"
	if err := os.WriteFile(khPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	hkm := NewHostKeyManager(khPath, true, nil)
	if err := hkm.replace(addr, newSigner.PublicKey()); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(khPath)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected other.host + new key, got:\n%s", data)
	}
	if !strings.HasPrefix(lines[0], "other.host ") {
		t.Errorf("unrelated host entry was removed:\n%s", data)
	}

	cb, err := knownhosts.New(khPath)
	if err != nil {
		t.Fatal(err)
	}
	remote := &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 2222}
	if err := cb("example.org:2222", remote, newSigner.PublicKey()); err != nil {
		t.Errorf("new key not accepted: %v", err)
	}
}
