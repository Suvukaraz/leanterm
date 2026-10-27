package session

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// knownHostsMu serialises all reads/writes of known_hosts within this process.
var knownHostsMu sync.Mutex

type HostKeyManager struct {
	knownHostsPath string
	autoAccept     bool
	emitter        EventEmitter
	prompter       Prompter
	tmpID          string
}

func NewHostKeyManager(knownHostsPath string, autoAccept bool, emitter EventEmitter) *HostKeyManager {
	if knownHostsPath == "" {
		home, _ := os.UserHomeDir()
		knownHostsPath = filepath.Join(home, ".ssh", "known_hosts")
	}
	return &HostKeyManager{
		knownHostsPath: knownHostsPath,
		autoAccept:     autoAccept,
		emitter:        emitter,
	}
}

// WithPrompter enables interactive host key decisions for the given session.
func (hkm *HostKeyManager) WithPrompter(p Prompter, tmpID string) *HostKeyManager {
	hkm.prompter = p
	hkm.tmpID = tmpID
	return hkm
}

// FingerprintSHA256 formats the SHA256 fingerprint of a public key as "SHA256:...".
func FingerprintSHA256(pubKey ssh.PublicKey) string {
	hash := sha256.Sum256(pubKey.Marshal())
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(hash[:])
}

// HostKeyCallback verifies host keys against known_hosts. Unknown or changed keys are
// accepted automatically when auto-accept is on, otherwise the user is asked.
func (hkm *HostKeyManager) HostKeyCallback() ssh.HostKeyCallback {
	return hkm.HostKeyCallbackContext(context.Background())
}

func (hkm *HostKeyManager) HostKeyCallbackContext(ctx context.Context) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		addr := knownhosts.Normalize(hostname)

		knownHostsMu.Lock()
		checkErr := hkm.check(hostname, remote, key)
		knownHostsMu.Unlock()
		if checkErr == nil {
			return nil
		}

		var keyErr *knownhosts.KeyError
		changed := errors.As(checkErr, &keyErr) && len(keyErr.Want) > 0
		if !changed && !errors.As(checkErr, &keyErr) {
			// Unreadable known_hosts etc. – treat like an unknown key.
			changed = false
		}

		fingerprint := FingerprintSHA256(key)
		if hkm.autoAccept {
			if changed {
				hkm.status(fmt.Sprintf("HOSTKEY:deleting offending hostkey for %s by configuration", addr))
				return hkm.replace(addr, key)
			}
			hkm.status("HOSTKEY:accepted by configuration")
			return hkm.add(addr, key)
		}

		if changed {
			resp, err := ask(ctx, hkm.prompter, PromptRequest{
				TmpID: hkm.tmpID,
				Kind:  "hostkey-changed",
				Title: "WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!",
				Message: fmt.Sprintf("The %s host key of %s has changed.\nSomeone could be eavesdropping on you right now (man-in-the-middle attack), or the host key has just been changed.\n\nNew fingerprint: %s\nOffending entries in: %s",
					key.Type(), addr, fingerprint, hkm.knownHostsPath),
				Options: []string{"Replace key and connect", "Cancel"},
			})
			if err != nil || resp.Choice != "Replace key and connect" {
				return fmt.Errorf("host key verification failed for %s (key changed)", addr)
			}
			return hkm.replace(addr, key)
		}

		resp, err := ask(ctx, hkm.prompter, PromptRequest{
			TmpID: hkm.tmpID,
			Kind:  "hostkey-unknown",
			Title: "Unknown host key",
			Message: fmt.Sprintf("The authenticity of host '%s' can't be established.\n%s key fingerprint is %s.\nAre you sure you want to continue connecting?",
				addr, key.Type(), fingerprint),
			Options: []string{"Accept and save", "Accept once", "Reject"},
		})
		if err != nil {
			return fmt.Errorf("host key for %s rejected", addr)
		}
		switch resp.Choice {
		case "Accept and save":
			return hkm.add(addr, key)
		case "Accept once":
			return nil
		default:
			return fmt.Errorf("host key for %s rejected", addr)
		}
	}
}

func (hkm *HostKeyManager) check(hostname string, remote net.Addr, key ssh.PublicKey) error {
	if _, err := os.Stat(hkm.knownHostsPath); os.IsNotExist(err) {
		return &knownhosts.KeyError{}
	}
	cb, err := knownhosts.New(hkm.knownHostsPath)
	if err != nil {
		return err
	}
	return cb(hostname, remote, key)
}

func (hkm *HostKeyManager) status(text string) {
	if hkm.emitter != nil && hkm.tmpID != "" {
		hkm.emitter.Emit("session:status", map[string]any{"tmpID": hkm.tmpID, "code": "HOSTKEY", "text": text})
	}
}

// add appends a known_hosts line for addr.
func (hkm *HostKeyManager) add(addr string, key ssh.PublicKey) error {
	knownHostsMu.Lock()
	defer knownHostsMu.Unlock()

	if err := os.MkdirAll(filepath.Dir(hkm.knownHostsPath), 0700); err != nil {
		return nil // cannot persist; key is accepted for this connection only
	}
	f, err := os.OpenFile(hkm.knownHostsPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil
	}
	defer f.Close()
	_, _ = f.WriteString(knownhosts.Line([]string{addr}, key) + "\n")
	return nil
}

// appendKey is kept for callers/tests that add a key directly.
func (hkm *HostKeyManager) appendKey(addr string, key ssh.PublicKey) error {
	return hkm.add(addr, key)
}

// replace removes all entries for addr (plain or hashed) and adds the new key.
func (hkm *HostKeyManager) replace(addr string, key ssh.PublicKey) error {
	knownHostsMu.Lock()
	data, err := os.ReadFile(hkm.knownHostsPath)
	if err == nil {
		var out bytes.Buffer
		sc := bufio.NewScanner(bytes.NewReader(data))
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			if lineMatchesHost(line, addr) {
				continue
			}
			out.WriteString(line)
			out.WriteByte('\n')
		}
		tmp := hkm.knownHostsPath + ".tmp"
		if werr := os.WriteFile(tmp, out.Bytes(), 0600); werr == nil {
			_ = os.Rename(tmp, hkm.knownHostsPath)
		}
	}
	knownHostsMu.Unlock()
	return hkm.add(addr, key)
}

// lineMatchesHost reports whether a known_hosts line lists addr (plain or hashed form).
func lineMatchesHost(line, addr string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return false
	}
	fields := strings.Fields(trimmed)
	if strings.HasPrefix(fields[0], "@") { // marker such as @cert-authority
		if len(fields) < 2 {
			return false
		}
		fields = fields[1:]
	}
	for _, h := range strings.Split(fields[0], ",") {
		if h == addr {
			return true
		}
		if strings.HasPrefix(h, "|1|") && knownhosts.HashHostname(addr) != "" && hashedMatches(h, addr) {
			return true
		}
	}
	return false
}

// hashedMatches checks an OpenSSH hashed host entry "|1|salt|hash" against addr.
func hashedMatches(entry, addr string) bool {
	parts := strings.Split(entry, "|")
	if len(parts) != 4 {
		return false
	}
	salt, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	return hmacSHA1Base64(salt, addr) == parts[3]
}

func hmacSHA1Base64(key []byte, msg string) string {
	mac := hmac.New(sha1.New, key)
	mac.Write([]byte(msg))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
