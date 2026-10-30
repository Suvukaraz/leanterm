package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type AuthOptions struct {
	AuthType      string // "password", "publickey", "manual"
	User          string
	Host          string
	Password      string
	KeyFile       string
	KeyPassphrase string
	Exclusive     bool

	// Interactive support (optional). Without a prompter only stored secrets are used.
	Ctx        context.Context
	Prompter   Prompter
	TmpID      string
	SaveSecret func(field, value string) // persists a secret the user asked to remember
}

// passwordSource hands out the stored password once and asks the user afterwards.
type passwordSource struct {
	opts    *AuthOptions
	usedPwd bool
	asked   int
}

func (p *passwordSource) next(prompt string) (string, error) {
	if !p.usedPwd && p.opts.Password != "" {
		p.usedPwd = true
		return p.opts.Password, nil
	}
	p.usedPwd = true
	p.asked++
	msg := fmt.Sprintf("Enter password for '%s@%s'", p.opts.User, p.opts.Host)
	if p.asked > 1 || p.opts.Password != "" {
		msg = "Permission denied, please try again.\n" + msg
	}
	if prompt != "" && !strings.EqualFold(strings.TrimSpace(prompt), "password:") {
		msg = strings.TrimSpace(prompt)
	}
	ctx := p.opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	resp, err := ask(ctx, p.opts.Prompter, PromptRequest{
		TmpID:    p.opts.TmpID,
		Kind:     "password",
		Title:    "Password requested",
		Message:  msg,
		Fields:   []PromptField{{Name: "password", Label: "Password", Secret: true}},
		Remember: p.opts.SaveSecret != nil,
	})
	if err != nil {
		return "", err
	}
	pwd := resp.Values["password"]
	if resp.Remember && p.opts.SaveSecret != nil {
		p.opts.SaveSecret("password", pwd)
	}
	p.opts.Password = pwd
	return pwd, nil
}

// BuildAuthMethods prepares the slice of SSH auth methods based on connection config.
func BuildAuthMethods(opts AuthOptions) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	interactive := opts.Prompter != nil
	pw := &passwordSource{opts: &opts}

	passwordMethods := func() []ssh.AuthMethod {
		if !interactive {
			if opts.Password == "" {
				return nil
			}
			return []ssh.AuthMethod{ssh.Password(opts.Password), createKeyboardInteractive(opts.Password)}
		}
		return []ssh.AuthMethod{
			ssh.RetryableAuthMethod(ssh.PasswordCallback(func() (string, error) { return pw.next("") }), 3),
			ssh.RetryableAuthMethod(ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				return keyboardInteractiveAnswers(&opts, pw, instruction, questions, echos)
			}), 3),
		}
	}

	switch opts.AuthType {
	case "publickey":
		if opts.KeyFile != "" {
			signer, err := loadKeyInteractive(&opts)
			if err != nil {
				if errors.Is(err, ErrPromptCancelled) || opts.Exclusive {
					return nil, err
				}
			} else {
				methods = append(methods, ssh.PublicKeys(signer))
			}
		}
	case "password", "manual":
		methods = append(methods, passwordMethods()...)
	}

	if !opts.Exclusive {
		// 1. SSH agent
		if conn, err := dialAgent(); err == nil {
			agClient := agent.NewClient(conn)
			methods = append(methods, ssh.PublicKeysCallback(agClient.Signers))
		}

		// 2. Default unencrypted keys in ~/.ssh
		if home, _ := os.UserHomeDir(); home != "" {
			for _, k := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
				p := filepath.Join(home, ".ssh", k)
				if p == opts.KeyFile {
					continue
				}
				if signer, err := loadPrivateKey(p, ""); err == nil {
					methods = append(methods, ssh.PublicKeys(signer))
				}
			}
		}

		// 3. Password / keyboard-interactive as last resort
		if opts.AuthType == "publickey" {
			methods = append(methods, passwordMethods()...)
		}
	}

	if len(methods) == 0 {
		return nil, errors.New("no authentication methods available (no password stored and no usable key)")
	}
	return methods, nil
}

// keyboardInteractiveAnswers answers a single echo-off question with the password and
// asks the user for everything else (e.g. OTP codes).
func keyboardInteractiveAnswers(opts *AuthOptions, pw *passwordSource, instruction string, questions []string, echos []bool) ([]string, error) {
	if len(questions) == 0 {
		return nil, nil
	}
	if len(questions) == 1 && !echos[0] && looksLikePassword(questions[0]) {
		pwd, err := pw.next(questions[0])
		if err != nil {
			return nil, err
		}
		return []string{pwd}, nil
	}

	fields := make([]PromptField, len(questions))
	for i, q := range questions {
		fields[i] = PromptField{Name: fmt.Sprintf("q%d", i), Label: strings.TrimSpace(q), Secret: !echos[i]}
	}
	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	msg := strings.TrimSpace(instruction)
	if msg == "" {
		msg = fmt.Sprintf("The server %s requests additional authentication.", opts.Host)
	}
	resp, err := ask(ctx, opts.Prompter, PromptRequest{
		TmpID:   opts.TmpID,
		Kind:    "keyboard-interactive",
		Title:   "Authentication",
		Message: msg,
		Fields:  fields,
	})
	if err != nil {
		return nil, err
	}
	answers := make([]string, len(questions))
	for i := range questions {
		answers[i] = resp.Values[fmt.Sprintf("q%d", i)]
	}
	return answers, nil
}

// looksLikePassword reports whether a keyboard-interactive question asks for the
// account password (and not e.g. for a one-time code).
func looksLikePassword(q string) bool {
	q = strings.ToLower(q)
	for _, w := range []string{"password", "passwort", "kennwort", "contraseña"} {
		if strings.Contains(q, w) {
			return true
		}
	}
	return false
}

// loadKeyInteractive loads the configured key and asks for the passphrase when needed.
func loadKeyInteractive(opts *AuthOptions) (ssh.Signer, error) {
	path := expandHome(opts.KeyFile)
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read private key %s: %w", path, err)
	}
	if opts.KeyPassphrase != "" {
		if signer, err := ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(opts.KeyPassphrase)); err == nil {
			return signer, nil
		}
	} else {
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err == nil {
			return signer, nil
		}
		var missing *ssh.PassphraseMissingError
		if !errors.As(err, &missing) {
			return nil, fmt.Errorf("cannot parse private key %s: %w", path, err)
		}
	}

	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	for attempt := 0; attempt < 3; attempt++ {
		msg := fmt.Sprintf("Enter passphrase for key '%s'", path)
		if attempt > 0 {
			msg = "Wrong passphrase, please try again.\n" + msg
		}
		resp, err := ask(ctx, opts.Prompter, PromptRequest{
			TmpID:    opts.TmpID,
			Kind:     "passphrase",
			Title:    "Passphrase requested",
			Message:  msg,
			Fields:   []PromptField{{Name: "passphrase", Label: "Passphrase", Secret: true}},
			Remember: opts.SaveSecret != nil,
		})
		if err != nil {
			return nil, err
		}
		pass := resp.Values["passphrase"]
		signer, perr := ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(pass))
		if perr == nil {
			if resp.Remember && opts.SaveSecret != nil {
				opts.SaveSecret("key_passphrase", pass)
			}
			return signer, nil
		}
	}
	return nil, fmt.Errorf("wrong passphrase for key %s", path)
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func loadPrivateKey(path, passphrase string) (ssh.Signer, error) {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if passphrase != "" {
		return ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(passphrase))
	}
	return ssh.ParsePrivateKey(keyBytes)
}

func createKeyboardInteractive(password string) ssh.AuthMethod {
	return ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
		answers := make([]string, len(questions))
		for i := range questions {
			answers[i] = password
		}
		return answers, nil
	})
}
