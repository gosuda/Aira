package secrets

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validKey(t *testing.T) []byte {
	t.Helper()

	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)

	return key
}

func TestNewVault_ValidKey(t *testing.T) {
	t.Parallel()

	v, err := NewVault(validKey(t))
	require.NoError(t, err)
	assert.NotNil(t, v)
}

func TestNewVault_InvalidKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		keyLen int
	}{
		{name: "too short", keyLen: 16},
		{name: "too long", keyLen: 64},
		{name: "empty", keyLen: 0},
		{name: "one byte", keyLen: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			key := make([]byte, tt.keyLen)
			v, err := NewVault(key)
			assert.Nil(t, v)
			assert.ErrorIs(t, err, ErrInvalidKey)
		})
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	t.Parallel()

	v, err := NewVault(validKey(t))
	require.NoError(t, err)

	tests := []struct {
		name      string
		plaintext string
	}{
		{name: "simple string", plaintext: "hello world"},
		{name: "empty string", plaintext: ""},
		{name: "api key", plaintext: "sk-ant-api03-XXXXXXXXXXXXXXXXXXXXX"},
		{name: "unicode", plaintext: "secret value with unicode chars"},
		{name: "long value", plaintext: string(make([]byte, 4096))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			encrypted, encErr := v.Encrypt(tt.plaintext)
			require.NoError(t, encErr)
			assert.NotEmpty(t, encrypted)
			assert.NotEqual(t, tt.plaintext, encrypted)

			decrypted, decErr := v.Decrypt(encrypted)
			require.NoError(t, decErr)
			assert.Equal(t, tt.plaintext, decrypted)
		})
	}
}

func TestEncrypt_DifferentCiphertexts(t *testing.T) {
	t.Parallel()

	v, err := NewVault(validKey(t))
	require.NoError(t, err)

	plaintext := "same-secret-value"

	ct1, err := v.Encrypt(plaintext)
	require.NoError(t, err)

	ct2, err := v.Encrypt(plaintext)
	require.NoError(t, err)

	// Different ciphertexts due to random nonces.
	assert.NotEqual(t, ct1, ct2)

	// Both must decrypt to the same plaintext.
	d1, err := v.Decrypt(ct1)
	require.NoError(t, err)

	d2, err := v.Decrypt(ct2)
	require.NoError(t, err)

	assert.Equal(t, plaintext, d1)
	assert.Equal(t, plaintext, d2)
}

func TestDecrypt_InvalidCiphertext(t *testing.T) {
	t.Parallel()

	v, err := NewVault(validKey(t))
	require.NoError(t, err)

	tests := []struct {
		name       string
		ciphertext string
	}{
		{name: "not base64", ciphertext: "!!!not-base64!!!"},
		{name: "empty base64", ciphertext: base64.StdEncoding.EncodeToString([]byte{})},
		{name: "too short for nonce", ciphertext: base64.StdEncoding.EncodeToString([]byte("short"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, decErr := v.Decrypt(tt.ciphertext)
			require.Error(t, decErr)
			assert.Empty(t, result)
		})
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	t.Parallel()

	v, err := NewVault(validKey(t))
	require.NoError(t, err)

	encrypted, err := v.Encrypt("original secret")
	require.NoError(t, err)

	// Decode, tamper, re-encode.
	data, err := base64.StdEncoding.DecodeString(encrypted)
	require.NoError(t, err)

	// Flip a byte in the ciphertext portion (after the 12-byte nonce).
	if len(data) > 13 {
		data[13] ^= 0xFF
	}

	tampered := base64.StdEncoding.EncodeToString(data)

	result, decErr := v.Decrypt(tampered)
	require.Error(t, decErr)
	assert.Empty(t, result)
}

func TestDecryptSecrets_HappyPath(t *testing.T) {
	t.Parallel()

	v, err := NewVault(validKey(t))
	require.NoError(t, err)

	// Encrypt several secrets.
	enc1, err := v.Encrypt("sk-ant-api-key")
	require.NoError(t, err)

	enc2, err := v.Encrypt("ghp_token_12345")
	require.NoError(t, err)

	enc3, err := v.Encrypt("postgres://user:pass@host/db")
	require.NoError(t, err)

	secrets := []*Secret{
		{ID: uuid.New(), Name: "ANTHROPIC_API_KEY", Value: enc1},
		{ID: uuid.New(), Name: "GITHUB_TOKEN", Value: enc2},
		{ID: uuid.New(), Name: "DATABASE_URL", Value: enc3},
	}

	result, err := v.DecryptSecrets(secrets)
	require.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, "sk-ant-api-key", result["ANTHROPIC_API_KEY"])
	assert.Equal(t, "ghp_token_12345", result["GITHUB_TOKEN"])
	assert.Equal(t, "postgres://user:pass@host/db", result["DATABASE_URL"])
}

func TestDecryptSecrets_OneFailsReturnsError(t *testing.T) {
	t.Parallel()

	v, err := NewVault(validKey(t))
	require.NoError(t, err)

	enc1, err := v.Encrypt("valid-secret")
	require.NoError(t, err)

	secrets := []*Secret{
		{ID: uuid.New(), Name: "VALID_KEY", Value: enc1},
		{ID: uuid.New(), Name: "BAD_KEY", Value: "not-valid-base64!!!"},
	}

	result, decErr := v.DecryptSecrets(secrets)
	require.Error(t, decErr)
	assert.Nil(t, result)
	assert.Contains(t, decErr.Error(), "BAD_KEY")
}
