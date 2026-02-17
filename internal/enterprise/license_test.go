package enterprise

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidator_NoLicense(t *testing.T) {
	t.Parallel()

	v := NewValidator(nil)
	err := v.Validate()
	assert.ErrorIs(t, err, ErrNoLicense)
}

func TestValidator_ValidLicense(t *testing.T) {
	t.Parallel()

	license := &License{
		ID:        "lic-001",
		Org:       "acme-corp",
		MaxUsers:  50,
		MaxAgents: 10,
		Features:  []string{"sso", "audit-log"},
		ExpiresAt: time.Now().Add(24 * time.Hour),
		IssuedAt:  time.Now().Add(-24 * time.Hour),
	}

	v := NewValidator(license)
	err := v.Validate()
	require.NoError(t, err)
}

func TestValidator_ExpiredLicense(t *testing.T) {
	t.Parallel()

	license := &License{
		ID:        "lic-expired",
		Org:       "acme-corp",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		IssuedAt:  time.Now().Add(-48 * time.Hour),
	}

	v := NewValidator(license)
	err := v.Validate()
	assert.ErrorIs(t, err, ErrLicenseExpired)
}

func TestHasFeature_Enabled(t *testing.T) {
	t.Parallel()

	license := &License{
		Features: []string{"sso", "audit-log", "secrets"},
	}

	v := NewValidator(license)
	assert.True(t, v.HasFeature("sso"))
	assert.True(t, v.HasFeature("audit-log"))
	assert.True(t, v.HasFeature("secrets"))
}

func TestHasFeature_Disabled(t *testing.T) {
	t.Parallel()

	license := &License{
		Features: []string{"sso"},
	}

	v := NewValidator(license)
	assert.False(t, v.HasFeature("audit-log"))
	assert.False(t, v.HasFeature("secrets"))
}

func TestHasFeature_NoLicense(t *testing.T) {
	t.Parallel()

	v := NewValidator(nil)
	assert.False(t, v.HasFeature("sso"))
}

func TestMaxUsers_WithLicense(t *testing.T) {
	t.Parallel()

	license := &License{
		MaxUsers: 100,
	}

	v := NewValidator(license)
	assert.Equal(t, 100, v.MaxUsers())
}

func TestMaxUsers_NoLicense(t *testing.T) {
	t.Parallel()

	v := NewValidator(nil)
	assert.Equal(t, 0, v.MaxUsers())
}

func TestMaxAgents_WithLicense(t *testing.T) {
	t.Parallel()

	license := &License{
		MaxAgents: 25,
	}

	v := NewValidator(license)
	assert.Equal(t, 25, v.MaxAgents())
}

func TestMaxAgents_NoLicense(t *testing.T) {
	t.Parallel()

	v := NewValidator(nil)
	assert.Equal(t, 0, v.MaxAgents())
}
