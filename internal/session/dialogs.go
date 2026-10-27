package session

import (
	"context"
	"errors"
)

// PromptField is one input of a backend-initiated dialog.
type PromptField struct {
	Name   string `json:"name"`
	Label  string `json:"label"`
	Secret bool   `json:"secret"`
	Value  string `json:"value,omitempty"`
}

// PromptRequest describes a dialog the backend needs answered by the user
// (password, passphrase, keyboard-interactive questions, host key decisions).
type PromptRequest struct {
	ID       string        `json:"id"`
	TmpID    string        `json:"tmpID,omitempty"`
	Kind     string        `json:"kind"` // "password" | "passphrase" | "credentials" | "keyboard-interactive" | "hostkey-unknown" | "hostkey-changed" | "ask" | "confirm"
	Title    string        `json:"title"`
	Message  string        `json:"message"`
	Fields   []PromptField `json:"fields,omitempty"`
	Options  []string      `json:"options,omitempty"`
	Remember bool          `json:"remember,omitempty"` // offer a "Remember" checkbox
}

// PromptResponse is the user's answer.
type PromptResponse struct {
	Cancelled bool              `json:"cancelled"`
	Choice    string            `json:"choice"`
	Values    map[string]string `json:"values,omitempty"`
	Remember  bool              `json:"remember,omitempty"`
}

// Prompter shows dialogs in the UI and blocks until they are answered or ctx ends.
type Prompter interface {
	Prompt(ctx context.Context, req PromptRequest) (PromptResponse, error)
}

// ErrPromptCancelled is returned when the user dismissed a dialog.
var ErrPromptCancelled = errors.New("input cancelled by user")

// ask is a nil-safe helper: without a prompter every question counts as cancelled.
func ask(ctx context.Context, p Prompter, req PromptRequest) (PromptResponse, error) {
	if p == nil {
		return PromptResponse{Cancelled: true}, ErrPromptCancelled
	}
	resp, err := p.Prompt(ctx, req)
	if err != nil {
		return PromptResponse{Cancelled: true}, err
	}
	if resp.Cancelled {
		return resp, ErrPromptCancelled
	}
	return resp, nil
}
