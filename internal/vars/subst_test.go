package vars

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestSubstBuiltins(t *testing.T) {
	fixedTime := time.Date(2026, 10, 27, 15, 30, 45, 0, time.UTC)
	ctx := SubstContext{
		UUID:       "uuid-1234",
		Socks5Port: 1085,
		Timestamp:  fixedTime,
		Name:       "Production DB",
		Title:      "Prod Server",
		IP:         "192.168.1.50",
		Port:       22,
		User:       "admin",
		Pass:       "supersecret",
	}

	input := "ssh <USER>:<PASS>@<IP>:<PORT> - <NAME> (<TITLE>) [<UUID>] Socks:<SOCKS5_PORT> Date:<DATE_Y>-<DATE_M>-<DATE_D> Time:<TIME_H>:<TIME_M>:<TIME_S> TS:<TIMESTAMP>"
	res := Subst(input, ctx)

	expected := "ssh admin:supersecret@192.168.1.50:22 - Production DB (Prod Server) [uuid-1234] Socks:1085 Date:2026-10-27 Time:15:30:45 TS:1793115045"
	assert.Equal(t, expected, res.Text)
	assert.True(t, res.HasHidden)
}

func TestSubstGlobalVariables(t *testing.T) {
	ctx := SubstContext{
		Globals: []config.VariableConfig{
			{Name: "DOMAIN", Value: "example.com", Hidden: false},
			{Name: "API_KEY", Value: "raw-key-123", Hidden: false},
		},
	}

	input := "curl https://<GV:DOMAIN>/api -H 'Key: <GV:API_KEY>' and <GV:UNKNOWN>"
	res := Subst(input, ctx)

	expected := "curl https://example.com/api -H 'Key: raw-key-123' and <GV:UNKNOWN>"
	assert.Equal(t, expected, res.Text)
}

func TestSubstUserVariables(t *testing.T) {
	ctx := SubstContext{
		Variables: []config.UserVariable{
			{ID: "v0", Value: "val0", Hidden: false},
			{ID: "v1", Value: "val1", Hidden: false},
		},
	}

	input := "connect <V:0> and <V:1> and <V:99>"
	res := Subst(input, ctx)

	expected := "connect val0 and val1 and <V:99>"
	assert.Equal(t, expected, res.Text)
}

func TestSubstEnvironment(t *testing.T) {
	_ = os.Setenv("LEANTERM_TEST_VAR", "my_test_value")
	defer os.Unsetenv("LEANTERM_TEST_VAR")

	ctx := SubstContext{}
	input := "env is <ENV:LEANTERM_TEST_VAR> and <ENV:DOES_NOT_EXIST_XYZ>"
	res := Subst(input, ctx)

	expected := "env is my_test_value and <ENV:DOES_NOT_EXIST_XYZ>"
	assert.Equal(t, expected, res.Text)
}

func TestSubstAskInteractive(t *testing.T) {
	calls := 0
	handler := func(prompt string, options []string, masked bool) (string, bool, error) {
		calls++
		if prompt == "Variable substitution '1'" {
			return "user_input_1", false, nil
		}
		if prompt == "Choose environment" && len(options) == 2 {
			return options[1], false, nil // select "prod"
		}
		if prompt == "Please, enter a value for: 'target host'" {
			return "host.internal", false, nil
		}
		return "fallback", false, nil
	}

	ctx := SubstContext{
		AskHandler: handler,
	}

	input := "Deploy <ASK:1> to <ASK:Choose environment|staging|prod> at <ASK:target host>"
	res := Subst(input, ctx)

	assert.False(t, res.Cancelled)
	assert.Equal(t, "Deploy user_input_1 to prod at host.internal", res.Text)
	assert.Equal(t, 3, calls)
}

func TestSubstAskCancelled(t *testing.T) {
	handler := func(prompt string, options []string, masked bool) (string, bool, error) {
		return "", true, nil // User clicked Cancel
	}

	ctx := SubstContext{
		AskHandler: handler,
	}

	input := "Run <ASK:password>"
	res := Subst(input, ctx)

	assert.True(t, res.Cancelled)
}

func TestSubstLocalCommand(t *testing.T) {
	ctx := SubstContext{}
	input := "echo <CMD:echo hello_from_sh>"
	res := Subst(input, ctx)

	assert.Equal(t, "echo hello_from_sh", res.Text)
}

func TestSubstControlTags(t *testing.T) {
	ctx := SubstContext{}
	input := "ssh server <CTRL_X:my_ctrl> <TEE:/tmp/session.log> <PIPE:grep error:Enter pattern>"
	res := Subst(input, ctx)

	assert.Equal(t, "ssh server   ", res.Text)
	assert.Equal(t, "my_ctrl", res.Ctrl)
	assert.Equal(t, "/tmp/session.log", res.Tee)
	assert.Equal(t, []string{"grep error"}, res.Pipes)
	assert.Equal(t, "Enter pattern", res.Prompt)
}

func TestSubstNoRecursion(t *testing.T) {
	ctx := SubstContext{
		Globals: []config.VariableConfig{
			{Name: "TRICKY", Value: "<DATE_Y>", Hidden: false},
		},
	}

	input := "Value: <GV:TRICKY>"
	res := Subst(input, ctx)

	// Result should contain the literal string "<DATE_Y>", NOT 2026
	assert.Equal(t, "Value: <DATE_Y>", res.Text)
}

type mockKPResolver struct{}

func (m *mockKPResolver) IsEnabled() bool { return true }
func (m *mockKPResolver) SubstKeePass(ctx context.Context, text string) (string, bool, error) {
	if strings.Contains(text, "<Password|srv1>") {
		return strings.ReplaceAll(text, "<Password|srv1>", "mysecret"), true, nil
	}
	return text, false, nil
}

func TestSubstKeePass(t *testing.T) {
	ctx := SubstContext{
		KeePass: &mockKPResolver{},
	}
	res := Subst("Connecting with <Password|srv1>", ctx)
	if res.Text != "Connecting with mysecret" {
		t.Errorf("Subst KeePass text = %q, want 'Connecting with mysecret'", res.Text)
	}
	if !res.HasHidden {
		t.Errorf("expected HasHidden = true")
	}
}
