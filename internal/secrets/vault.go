package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

//nolint:gochecknoglobals // sentinel error
var ErrSecretNotFound = errors.New("secrets: not found")

//nolint:gochecknoglobals // sentinel error
var ErrInvalidKey = errors.New("secrets: invalid encryption key")

// Secret represents an encrypted secret for a project.
type Secret struct {
	ID        uuid.UUID
	ProjectID uuid.UUID
	TenantID  uuid.UUID
	Name      string // human-readable name, e.g. "ANTHROPIC_API_KEY"
	Value     string // encrypted value (base64-encoded ciphertext)
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SecretRepository stores encrypted secrets.
type SecretRepository interface {
	Create(ctx context.Context, s *Secret) error
	GetByName(ctx context.Context, tenantID, projectID uuid.UUID, name string) (*Secret, error)
	ListByProject(ctx context.Context, tenantID, projectID uuid.UUID) ([]*Secret, error)
	Delete(ctx context.Context, tenantID, projectID uuid.UUID, name string) error
}

// Vault encrypts/decrypts secrets using AES-256-GCM.
type Vault struct {
	aead cipher.AEAD
}

// NewVault creates a Vault with the given 32-byte encryption key.
func NewVault(key []byte) (*Vault, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKey
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secrets.NewVault: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secrets.NewVault: %w", err)
	}

	return &Vault{aead: aead}, nil
}

// Encrypt encrypts plaintext and returns base64-encoded ciphertext.
// The output format is base64(nonce || ciphertext).
func (v *Vault) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("secrets.Encrypt: generate nonce: %w", err)
	}

	// Seal appends the encrypted data to nonce, producing nonce || ciphertext.
	sealed := v.aead.Seal(nonce, nonce, []byte(plaintext), nil)

	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts base64-encoded ciphertext and returns plaintext.
// Expects the format base64(nonce || ciphertext).
func (v *Vault) Decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("secrets.Decrypt: base64 decode: %w", err)
	}

	nonceSize := v.aead.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("secrets.Decrypt: ciphertext too short")
	}

	nonce := data[:nonceSize]
	encrypted := data[nonceSize:]

	plaintext, err := v.aead.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("secrets.Decrypt: %w", err)
	}

	return string(plaintext), nil
}

// DecryptSecrets takes a list of encrypted secrets and returns a map of name to plaintext.
func (v *Vault) DecryptSecrets(secrets []*Secret) (map[string]string, error) {
	result := make(map[string]string, len(secrets))

	for _, s := range secrets {
		plaintext, err := v.Decrypt(s.Value)
		if err != nil {
			return nil, fmt.Errorf("secrets.DecryptSecrets: decrypt %q: %w", s.Name, err)
		}

		result[s.Name] = plaintext
	}

	return result, nil
}
