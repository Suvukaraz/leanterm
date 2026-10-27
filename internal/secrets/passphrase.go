package secrets

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"filippo.io/age"
)

const DefaultWorkFactor = 18

var ErrWrongPassword = errors.New("ERROR: Wrong password!!")

// EncryptWithPassphrase encrypts data using age scrypt recipient with work factor 18.
func EncryptWithPassphrase(data []byte, password string) ([]byte, error) {
	if password == "" {
		return nil, errors.New("master password cannot be empty")
	}

	recipient, err := age.NewScryptRecipient(password)
	if err != nil {
		return nil, fmt.Errorf("failed to create scrypt recipient: %w", err)
	}
	recipient.SetWorkFactor(DefaultWorkFactor)

	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipient)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize age encryption: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// DecryptWithPassphrase decrypts data using age scrypt identity.
func DecryptWithPassphrase(encrypted []byte, password string) ([]byte, error) {
	identity, err := age.NewScryptIdentity(password)
	if err != nil {
		return nil, fmt.Errorf("failed to create scrypt identity: %w", err)
	}

	r, err := age.Decrypt(bytes.NewReader(encrypted), identity)
	if err != nil {
		return nil, ErrWrongPassword
	}

	decrypted, err := io.ReadAll(r)
	if err != nil {
		return nil, ErrWrongPassword
	}

	return decrypted, nil
}
