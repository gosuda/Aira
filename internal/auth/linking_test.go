package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gosuda/aira/internal/auth"
	"github.com/gosuda/aira/internal/domain"
)

// --- mock UserRepository for linking tests ---

type mockLinkingRepo struct {
	createMessengerLinkErr error
	createdLinks           []*domain.UserMessengerLink
}

func (m *mockLinkingRepo) Create(context.Context, *domain.User) error { return nil }
func (m *mockLinkingRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*domain.User, error) {
	return nil, errors.New("not implemented")
}
func (m *mockLinkingRepo) GetByEmail(context.Context, uuid.UUID, string) (*domain.User, error) {
	return nil, errors.New("not found")
}
func (m *mockLinkingRepo) Update(context.Context, *domain.User) error { return nil }
func (m *mockLinkingRepo) List(context.Context, uuid.UUID) ([]*domain.User, error) {
	return nil, nil
}

func (m *mockLinkingRepo) CreateOAuthLink(context.Context, *domain.UserOAuthLink) error { return nil }
func (m *mockLinkingRepo) GetOAuthLink(context.Context, string, string) (*domain.UserOAuthLink, error) {
	return nil, nil
}
func (m *mockLinkingRepo) DeleteOAuthLink(context.Context, uuid.UUID, uuid.UUID) error { return nil }

func (m *mockLinkingRepo) CreateMessengerLink(_ context.Context, link *domain.UserMessengerLink) error {
	if m.createMessengerLinkErr != nil {
		return m.createMessengerLinkErr
	}
	m.createdLinks = append(m.createdLinks, link)
	return nil
}
func (m *mockLinkingRepo) GetMessengerLink(context.Context, uuid.UUID, string, string) (*domain.UserMessengerLink, error) {
	return nil, nil
}
func (m *mockLinkingRepo) ListMessengerLinks(context.Context, uuid.UUID) ([]*domain.UserMessengerLink, error) {
	return nil, nil
}
func (m *mockLinkingRepo) DeleteMessengerLink(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *mockLinkingRepo) CreateAPIKey(context.Context, *domain.APIKey) error { return nil }
func (m *mockLinkingRepo) GetAPIKeyByPrefix(context.Context, uuid.UUID, string) (*domain.APIKey, error) {
	return nil, nil
}
func (m *mockLinkingRepo) ListAPIKeys(context.Context, uuid.UUID, uuid.UUID) ([]*domain.APIKey, error) {
	return nil, nil
}
func (m *mockLinkingRepo) DeleteAPIKey(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (m *mockLinkingRepo) UpdateAPIKeyLastUsed(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

// --- GenerateLinkToken tests ---

func TestGenerateLinkToken(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()

	t.Run("returns token with correct fields", func(t *testing.T) {
		t.Parallel()

		repo := &mockLinkingRepo{}
		svc := auth.NewService(repo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		lt, err := svc.GenerateLinkToken(tenantID, "slack", "U12345")

		require.NoError(t, err)
		require.NotNil(t, lt)
		assert.NotEmpty(t, lt.Token)
		assert.Equal(t, tenantID, lt.TenantID)
		assert.Equal(t, "slack", lt.Platform)
		assert.Equal(t, "U12345", lt.ExternalID)
		assert.False(t, lt.ExpiresAt.IsZero())
		assert.True(t, lt.ExpiresAt.After(time.Now()))
	})

	t.Run("token has correct tenant platform and external ID", func(t *testing.T) {
		t.Parallel()

		repo := &mockLinkingRepo{}
		svc := auth.NewService(repo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		lt, err := svc.GenerateLinkToken(tenantID, "discord", "D99999")

		require.NoError(t, err)
		assert.Equal(t, tenantID, lt.TenantID)
		assert.Equal(t, "discord", lt.Platform)
		assert.Equal(t, "D99999", lt.ExternalID)
	})

	t.Run("two calls produce different tokens", func(t *testing.T) {
		t.Parallel()

		repo := &mockLinkingRepo{}
		svc := auth.NewService(repo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		lt1, err1 := svc.GenerateLinkToken(tenantID, "slack", "U111")
		lt2, err2 := svc.GenerateLinkToken(tenantID, "slack", "U111")

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, lt1.Token, lt2.Token, "tokens must be unique")
	})
}

// --- VerifyAndLink tests ---

func TestVerifyAndLink(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()

	t.Run("success consumes token and creates link", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockLinkingRepo{}
		svc := auth.NewService(repo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		lt, err := svc.GenerateLinkToken(tenantID, "slack", "U12345")
		require.NoError(t, err)

		err = svc.VerifyAndLink(ctx, lt.Token, userID)

		require.NoError(t, err)
		require.Len(t, repo.createdLinks, 1)

		link := repo.createdLinks[0]
		assert.Equal(t, userID, link.UserID)
		assert.Equal(t, tenantID, link.TenantID)
		assert.Equal(t, "slack", link.Platform)
		assert.Equal(t, "U12345", link.ExternalID)
		assert.NotEqual(t, uuid.Nil, link.ID)
		assert.False(t, link.CreatedAt.IsZero())
	})

	t.Run("invalid token returns ErrLinkTokenExpired", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockLinkingRepo{}
		svc := auth.NewService(repo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		err := svc.VerifyAndLink(ctx, "nonexistent-token", userID)

		require.Error(t, err)
		require.ErrorIs(t, err, auth.ErrLinkTokenExpired)
		assert.Empty(t, repo.createdLinks)
	})

	t.Run("expired token returns ErrLinkTokenExpired", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockLinkingRepo{}
		svc := auth.NewService(repo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		// Generate a valid token, then expire it by manipulating ExpiresAt.
		lt, err := svc.GenerateLinkToken(tenantID, "slack", "U12345")
		require.NoError(t, err)

		// Expire the token by setting ExpiresAt to the past.
		lt.ExpiresAt = time.Now().Add(-1 * time.Hour)

		err = svc.VerifyAndLink(ctx, lt.Token, userID)

		require.Error(t, err)
		require.ErrorIs(t, err, auth.ErrLinkTokenExpired)
		assert.Empty(t, repo.createdLinks)
	})

	t.Run("token consumed on use second call fails", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockLinkingRepo{}
		svc := auth.NewService(repo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		lt, err := svc.GenerateLinkToken(tenantID, "slack", "U12345")
		require.NoError(t, err)

		// First call succeeds.
		err = svc.VerifyAndLink(ctx, lt.Token, userID)
		require.NoError(t, err)

		// Second call fails because token was consumed.
		err = svc.VerifyAndLink(ctx, lt.Token, userID)
		require.Error(t, err)
		require.ErrorIs(t, err, auth.ErrLinkTokenExpired)
	})

	t.Run("repo error is wrapped", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repoErr := errors.New("db connection lost")
		repo := &mockLinkingRepo{createMessengerLinkErr: repoErr}
		svc := auth.NewService(repo, "jwt-secret", 15*time.Minute, 24*time.Hour)

		lt, err := svc.GenerateLinkToken(tenantID, "slack", "U12345")
		require.NoError(t, err)

		err = svc.VerifyAndLink(ctx, lt.Token, userID)

		require.Error(t, err)
		require.ErrorIs(t, err, repoErr)
	})
}
