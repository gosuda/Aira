package auth_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gosuda/aira/internal/auth"
	"github.com/gosuda/aira/internal/domain"
)

// --- mock UserRepository for apikey tests ---

type mockUserRepo struct {
	createAPIKeyErr error
	createdKeys     []*domain.APIKey

	getByPrefixResult *domain.APIKey
	getByPrefixErr    error

	getByIDResult *domain.User
	getByIDErr    error

	updateLastUsedErr error
}

func (m *mockUserRepo) Create(context.Context, *domain.User) error { return nil }
func (m *mockUserRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*domain.User, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	return m.getByIDResult, nil
}
func (m *mockUserRepo) GetByEmail(context.Context, uuid.UUID, string) (*domain.User, error) {
	return nil, errors.New("not found")
}
func (m *mockUserRepo) Update(context.Context, *domain.User) error { return nil }
func (m *mockUserRepo) List(context.Context, uuid.UUID) ([]*domain.User, error) {
	return nil, nil
}

func (m *mockUserRepo) CreateOAuthLink(context.Context, *domain.UserOAuthLink) error { return nil }
func (m *mockUserRepo) GetOAuthLink(context.Context, string, string) (*domain.UserOAuthLink, error) {
	return nil, nil
}
func (m *mockUserRepo) DeleteOAuthLink(context.Context, uuid.UUID) error { return nil }

func (m *mockUserRepo) CreateMessengerLink(context.Context, *domain.UserMessengerLink) error {
	return nil
}
func (m *mockUserRepo) GetMessengerLink(context.Context, uuid.UUID, string, string) (*domain.UserMessengerLink, error) {
	return nil, nil
}
func (m *mockUserRepo) ListMessengerLinks(context.Context, uuid.UUID) ([]*domain.UserMessengerLink, error) {
	return nil, nil
}
func (m *mockUserRepo) DeleteMessengerLink(context.Context, uuid.UUID) error { return nil }

func (m *mockUserRepo) CreateAPIKey(_ context.Context, key *domain.APIKey) error {
	if m.createAPIKeyErr != nil {
		return m.createAPIKeyErr
	}
	m.createdKeys = append(m.createdKeys, key)
	return nil
}
func (m *mockUserRepo) GetAPIKeyByPrefix(_ context.Context, _ uuid.UUID, _ string) (*domain.APIKey, error) {
	if m.getByPrefixErr != nil {
		return nil, m.getByPrefixErr
	}
	return m.getByPrefixResult, nil
}
func (m *mockUserRepo) ListAPIKeys(context.Context, uuid.UUID, uuid.UUID) ([]*domain.APIKey, error) {
	return nil, nil
}
func (m *mockUserRepo) DeleteAPIKey(context.Context, uuid.UUID) error { return nil }
func (m *mockUserRepo) UpdateAPIKeyLastUsed(_ context.Context, _ uuid.UUID) error {
	return m.updateLastUsedErr
}

func TestGenerateAPIKey(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()

	t.Run("returns key with aira_ prefix", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockUserRepo{}
		svc := auth.NewService(repo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		rawKey, apiKey, err := svc.GenerateAPIKey(ctx, tenantID, userID, "test key", []string{"read"})

		require.NoError(t, err)
		require.NotNil(t, apiKey)
		assert.True(t, strings.HasPrefix(rawKey, "aira_"), "key should have aira_ prefix, got: %s", rawKey)
	})

	t.Run("returns hash that is SHA-256 of key", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockUserRepo{}
		svc := auth.NewService(repo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		rawKey, apiKey, err := svc.GenerateAPIKey(ctx, tenantID, userID, "hash test", nil)

		require.NoError(t, err)

		expectedHash := sha256.Sum256([]byte(rawKey))
		expectedHex := hex.EncodeToString(expectedHash[:])

		assert.Equal(t, expectedHex, apiKey.KeyHash)
	})

	t.Run("key format: aira_ + 32 hex chars", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockUserRepo{}
		svc := auth.NewService(repo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		rawKey, _, err := svc.GenerateAPIKey(ctx, tenantID, userID, "format test", nil)

		require.NoError(t, err)
		// "aira_" (5 chars) + 32 hex chars = 37 total
		assert.Len(t, rawKey, 37)
	})

	t.Run("prefix stored is first 8 chars", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockUserRepo{}
		svc := auth.NewService(repo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		rawKey, apiKey, err := svc.GenerateAPIKey(ctx, tenantID, userID, "prefix test", nil)

		require.NoError(t, err)
		assert.Equal(t, rawKey[:8], apiKey.Prefix)
	})

	t.Run("stored fields correct", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockUserRepo{}
		svc := auth.NewService(repo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		_, apiKey, err := svc.GenerateAPIKey(ctx, tenantID, userID, "my key", []string{"read", "write"})

		require.NoError(t, err)
		assert.Equal(t, tenantID, apiKey.TenantID)
		assert.Equal(t, userID, apiKey.UserID)
		assert.Equal(t, "my key", apiKey.Name)
		assert.Equal(t, []string{"read", "write"}, apiKey.Scopes)
		assert.NotEqual(t, uuid.Nil, apiKey.ID)
		require.Len(t, repo.createdKeys, 1)
	})

	t.Run("CreateAPIKey error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockUserRepo{createAPIKeyErr: errors.New("db error")}
		svc := auth.NewService(repo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		rawKey, apiKey, err := svc.GenerateAPIKey(ctx, tenantID, userID, "fail key", nil)

		require.Error(t, err)
		assert.Empty(t, rawKey)
		assert.Nil(t, apiKey)
	})
}

func TestValidateAPIKey(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()

	t.Run("returns true for correct key", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		// Generate a key first, then validate it.
		genRepo := &mockUserRepo{}
		svc := auth.NewService(genRepo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		rawKey, generatedKey, err := svc.GenerateAPIKey(ctx, tenantID, userID, "valid key", nil)
		require.NoError(t, err)

		// Set up the validate mock with the generated key data.
		validateRepo := &mockUserRepo{
			getByPrefixResult: generatedKey,
			getByIDResult: &domain.User{
				ID:       userID,
				TenantID: tenantID,
				Email:    "test@example.com",
			},
		}
		validateSvc := auth.NewService(validateRepo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		user, apiKey, err := validateSvc.ValidateAPIKey(ctx, rawKey)

		require.NoError(t, err)
		require.NotNil(t, user)
		require.NotNil(t, apiKey)
		assert.Equal(t, userID, user.ID)
	})

	t.Run("returns false for wrong key", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		// Create a stored key with a known hash.
		storedHash := sha256.Sum256([]byte("aira_correctkeycorrectkeycorrectk"))
		storedKey := &domain.APIKey{
			ID:       uuid.New(),
			TenantID: tenantID,
			UserID:   userID,
			KeyHash:  hex.EncodeToString(storedHash[:]),
			Prefix:   "aira_cor",
		}

		repo := &mockUserRepo{
			getByPrefixResult: storedKey,
		}
		svc := auth.NewService(repo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		// Try to validate with a different key that has the same prefix.
		user, apiKey, err := svc.ValidateAPIKey(ctx, "aira_corWRONGKEYWRONGKEYWRONGKEYWR")

		require.Error(t, err)
		assert.Nil(t, user)
		assert.Nil(t, apiKey)
		assert.ErrorIs(t, err, auth.ErrInvalidAPIKey)
	})

	t.Run("key too short rejected", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockUserRepo{}
		svc := auth.NewService(repo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		user, apiKey, err := svc.ValidateAPIKey(ctx, "short")

		require.Error(t, err)
		assert.Nil(t, user)
		assert.Nil(t, apiKey)
		assert.ErrorIs(t, err, auth.ErrInvalidAPIKey)
	})

	t.Run("prefix not found rejected", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockUserRepo{getByPrefixErr: errors.New("not found")}
		svc := auth.NewService(repo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		user, apiKey, err := svc.ValidateAPIKey(ctx, "aira_0123456789abcdef0123456789abcdef")

		require.Error(t, err)
		assert.Nil(t, user)
		assert.Nil(t, apiKey)
		assert.ErrorIs(t, err, auth.ErrInvalidAPIKey)
	})

	t.Run("expired key rejected", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		// Create a key that we know the raw value of.
		rawKey := "aira_0123456789abcdef0123456789ab"
		hash := sha256.Sum256([]byte(rawKey))
		past := time.Now().Add(-1 * time.Hour)

		storedKey := &domain.APIKey{
			ID:        uuid.New(),
			TenantID:  tenantID,
			UserID:    userID,
			KeyHash:   hex.EncodeToString(hash[:]),
			Prefix:    rawKey[:8],
			ExpiresAt: &past,
		}

		repo := &mockUserRepo{getByPrefixResult: storedKey}
		svc := auth.NewService(repo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		user, apiKey, err := svc.ValidateAPIKey(ctx, rawKey)

		require.Error(t, err)
		assert.Nil(t, user)
		assert.Nil(t, apiKey)
		assert.ErrorIs(t, err, auth.ErrInvalidAPIKey)
	})
}
