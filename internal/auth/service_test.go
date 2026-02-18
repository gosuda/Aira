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

// --- configurable mock UserRepository for service tests ---

// mockServiceRepo is a configurable mock implementing domain.UserRepository.
// It captures calls and returns preconfigured responses for service-level tests.
type mockServiceRepo struct {
	// GetByEmail behavior.
	getByEmailUser *domain.User
	getByEmailErr  error

	// GetByID behavior.
	getByIDUser *domain.User
	getByIDErr  error

	// Create behavior.
	createErr   error
	createdUser *domain.User // captures the user passed to Create.

	// Update behavior.
	updateErr error
}

func (m *mockServiceRepo) Create(_ context.Context, u *domain.User) error {
	m.createdUser = u
	return m.createErr
}

func (m *mockServiceRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (*domain.User, error) {
	return m.getByIDUser, m.getByIDErr
}

func (m *mockServiceRepo) GetByEmail(context.Context, uuid.UUID, string) (*domain.User, error) {
	return m.getByEmailUser, m.getByEmailErr
}

func (m *mockServiceRepo) Update(context.Context, *domain.User) error {
	return m.updateErr
}

func (m *mockServiceRepo) List(context.Context, uuid.UUID) ([]*domain.User, error) {
	return nil, nil
}

func (m *mockServiceRepo) CreateOAuthLink(context.Context, *domain.UserOAuthLink) error {
	return nil
}

func (m *mockServiceRepo) GetOAuthLink(context.Context, string, string) (*domain.UserOAuthLink, error) {
	return nil, nil
}

func (m *mockServiceRepo) DeleteOAuthLink(context.Context, uuid.UUID, uuid.UUID) error { return nil }

func (m *mockServiceRepo) CreateMessengerLink(context.Context, *domain.UserMessengerLink) error {
	return nil
}

func (m *mockServiceRepo) GetMessengerLink(context.Context, uuid.UUID, string, string) (*domain.UserMessengerLink, error) {
	return nil, nil
}

func (m *mockServiceRepo) ListMessengerLinks(context.Context, uuid.UUID) ([]*domain.UserMessengerLink, error) {
	return nil, nil
}

func (m *mockServiceRepo) DeleteMessengerLink(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *mockServiceRepo) CreateAPIKey(context.Context, *domain.APIKey) error { return nil }

func (m *mockServiceRepo) GetAPIKeyByPrefix(context.Context, uuid.UUID, string) (*domain.APIKey, error) {
	return nil, nil
}

func (m *mockServiceRepo) ListAPIKeys(context.Context, uuid.UUID, uuid.UUID) ([]*domain.APIKey, error) {
	return nil, nil
}

func (m *mockServiceRepo) DeleteAPIKey(context.Context, uuid.UUID, uuid.UUID) error { return nil }

func (m *mockServiceRepo) UpdateAPIKeyLastUsed(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

// --- test constants ---

const (
	testJWTSecret = "test-secret-key-for-unit-tests"
	testEmail     = "alice@example.com"
	testPassword  = "correct-horse-battery-staple"
	testUserName  = "Alice"
)

var (
	testAccessTTL  = 15 * time.Minute
	testRefreshTTL = 7 * 24 * time.Hour
)

// newTestService creates a Service with the given mock and standard test config.
func newTestService(repo *mockServiceRepo) *auth.Service {
	return auth.NewService(repo, testJWTSecret, testAccessTTL, testRefreshTTL)
}

// --- Register tests ---

func TestRegister(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()

	t.Run("happy path creates user with correct fields", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockServiceRepo{
			getByEmailErr: domain.ErrNotFound,
		}
		svc := newTestService(repo)

		user, err := svc.Register(ctx, tenantID, testEmail, testPassword, testUserName)

		require.NoError(t, err)
		require.NotNil(t, user)
		assert.Equal(t, tenantID, user.TenantID)
		assert.Equal(t, testEmail, user.Email)
		assert.Equal(t, testUserName, user.Name)
		assert.Equal(t, "member", user.Role, "default role must be member")
		assert.NotEqual(t, uuid.Nil, user.ID, "user ID must be generated")
		assert.False(t, user.CreatedAt.IsZero(), "CreatedAt must be set")
		assert.False(t, user.UpdatedAt.IsZero(), "UpdatedAt must be set")
	})

	t.Run("password is hashed not stored as plaintext", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockServiceRepo{
			getByEmailErr: domain.ErrNotFound,
		}
		svc := newTestService(repo)

		user, err := svc.Register(ctx, tenantID, testEmail, testPassword, testUserName)

		require.NoError(t, err)
		require.NotNil(t, user)
		assert.NotEqual(t, testPassword, user.PasswordHash, "password must not be stored as plaintext")
		assert.NotEmpty(t, user.PasswordHash, "password hash must not be empty")
		assert.Contains(t, user.PasswordHash, "$", "argon2id hash must contain salt$hash separator")
	})

	t.Run("user already exists returns ErrUserAlreadyExists", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		existingUser := &domain.User{
			ID:       uuid.New(),
			TenantID: tenantID,
			Email:    testEmail,
		}
		repo := &mockServiceRepo{
			getByEmailUser: existingUser,
			getByEmailErr:  nil,
		}
		svc := newTestService(repo)

		user, err := svc.Register(ctx, tenantID, testEmail, testPassword, testUserName)

		require.Error(t, err)
		assert.Nil(t, user)
		assert.ErrorIs(t, err, auth.ErrUserAlreadyExists)
	})

	t.Run("repo Create error is propagated", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repoErr := errors.New("database connection refused")
		repo := &mockServiceRepo{
			getByEmailErr: domain.ErrNotFound,
			createErr:     repoErr,
		}
		svc := newTestService(repo)

		user, err := svc.Register(ctx, tenantID, testEmail, testPassword, testUserName)

		require.Error(t, err)
		assert.Nil(t, user)
		assert.ErrorIs(t, err, repoErr)
	})

	t.Run("created user is passed to repo with hashed password", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockServiceRepo{
			getByEmailErr: domain.ErrNotFound,
		}
		svc := newTestService(repo)

		_, err := svc.Register(ctx, tenantID, testEmail, testPassword, testUserName)

		require.NoError(t, err)
		require.NotNil(t, repo.createdUser, "repo.Create must have been called")
		assert.Equal(t, testEmail, repo.createdUser.Email)
		assert.Equal(t, testUserName, repo.createdUser.Name)
		assert.Equal(t, "member", repo.createdUser.Role)
		assert.NotEqual(t, testPassword, repo.createdUser.PasswordHash)
	})
}

// --- Login tests ---

func TestLogin(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()

	// registerAndGetUser is a helper that registers a user via the service
	// and returns the captured repo user (with hashed password) for Login tests.
	registerAndGetUser := func(t *testing.T) *domain.User {
		t.Helper()

		repo := &mockServiceRepo{
			getByEmailErr: domain.ErrNotFound,
		}
		svc := newTestService(repo)

		_, err := svc.Register(t.Context(), tenantID, testEmail, testPassword, testUserName)
		require.NoError(t, err)
		require.NotNil(t, repo.createdUser)

		return repo.createdUser
	}

	t.Run("happy path returns two valid tokens", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		registeredUser := registerAndGetUser(t)
		repo := &mockServiceRepo{
			getByEmailUser: registeredUser,
		}
		svc := newTestService(repo)

		accessToken, refreshToken, err := svc.Login(ctx, tenantID, testEmail, testPassword)

		require.NoError(t, err)
		assert.NotEmpty(t, accessToken, "access token must not be empty")
		assert.NotEmpty(t, refreshToken, "refresh token must not be empty")
		assert.NotEqual(t, accessToken, refreshToken, "access and refresh tokens must differ")
	})

	t.Run("returned access token is a valid JWT with correct claims", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		registeredUser := registerAndGetUser(t)
		repo := &mockServiceRepo{
			getByEmailUser: registeredUser,
		}
		svc := newTestService(repo)

		accessToken, _, err := svc.Login(ctx, tenantID, testEmail, testPassword)

		require.NoError(t, err)

		claims, err := auth.ValidateToken(testJWTSecret, accessToken)
		require.NoError(t, err)
		assert.Equal(t, registeredUser.ID.String(), claims.UserID)
		assert.Equal(t, registeredUser.TenantID.String(), claims.TenantID)
		assert.Equal(t, "member", claims.Role)
		assert.Equal(t, "access", claims.TokenType)
	})

	t.Run("returned refresh token is a valid JWT with correct type", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		registeredUser := registerAndGetUser(t)
		repo := &mockServiceRepo{
			getByEmailUser: registeredUser,
		}
		svc := newTestService(repo)

		_, refreshToken, err := svc.Login(ctx, tenantID, testEmail, testPassword)

		require.NoError(t, err)

		claims, err := auth.ValidateToken(testJWTSecret, refreshToken)
		require.NoError(t, err)
		assert.Equal(t, "refresh", claims.TokenType)
	})

	t.Run("wrong password returns ErrInvalidCredentials", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		registeredUser := registerAndGetUser(t)
		repo := &mockServiceRepo{
			getByEmailUser: registeredUser,
		}
		svc := newTestService(repo)

		accessToken, refreshToken, err := svc.Login(ctx, tenantID, testEmail, "wrong-password")

		require.Error(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
		assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
	})

	t.Run("user not found returns ErrInvalidCredentials", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockServiceRepo{
			getByEmailErr: domain.ErrNotFound,
		}
		svc := newTestService(repo)

		accessToken, refreshToken, err := svc.Login(ctx, tenantID, "nobody@example.com", testPassword)

		require.Error(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
		assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
	})
}

// --- RefreshToken tests ---

func TestRefreshToken(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()

	t.Run("happy path issues new access token from valid refresh token", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		existingUser := &domain.User{
			ID:       userID,
			TenantID: tenantID,
			Role:     "member",
		}
		repo := &mockServiceRepo{
			getByIDUser: existingUser,
		}
		svc := newTestService(repo)

		refreshToken, err := auth.IssueRefreshToken(testJWTSecret, tenantID, userID, "member", testRefreshTTL)
		require.NoError(t, err)

		newAccess, err := svc.RefreshToken(ctx, refreshToken)

		require.NoError(t, err)
		assert.NotEmpty(t, newAccess)

		// Validate the new access token.
		claims, err := auth.ValidateToken(testJWTSecret, newAccess)
		require.NoError(t, err)
		assert.Equal(t, "access", claims.TokenType)
		assert.Equal(t, userID.String(), claims.UserID)
		assert.Equal(t, tenantID.String(), claims.TenantID)
		assert.Equal(t, "member", claims.Role)
	})

	t.Run("uses current role from repo not stale token role", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		// User was promoted to admin after the refresh token was issued.
		existingUser := &domain.User{
			ID:       userID,
			TenantID: tenantID,
			Role:     "admin",
		}
		repo := &mockServiceRepo{
			getByIDUser: existingUser,
		}
		svc := newTestService(repo)

		// Issue refresh token with old role "member".
		refreshToken, err := auth.IssueRefreshToken(testJWTSecret, tenantID, userID, "member", testRefreshTTL)
		require.NoError(t, err)

		newAccess, err := svc.RefreshToken(ctx, refreshToken)

		require.NoError(t, err)

		claims, err := auth.ValidateToken(testJWTSecret, newAccess)
		require.NoError(t, err)
		assert.Equal(t, "admin", claims.Role, "new access token must use current role from repo")
	})

	t.Run("access token rejected with ErrInvalidToken", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockServiceRepo{}
		svc := newTestService(repo)

		// Issue an access token (not refresh).
		accessToken, err := auth.IssueAccessToken(testJWTSecret, tenantID, userID, "member", testAccessTTL)
		require.NoError(t, err)

		newAccess, err := svc.RefreshToken(ctx, accessToken)

		require.Error(t, err)
		assert.Empty(t, newAccess)
		assert.ErrorIs(t, err, auth.ErrInvalidToken)
	})

	t.Run("expired token returns error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockServiceRepo{}
		svc := newTestService(repo)

		// Issue a refresh token that is already expired.
		expiredToken, err := auth.IssueRefreshToken(testJWTSecret, tenantID, userID, "member", -1*time.Second)
		require.NoError(t, err)

		newAccess, err := svc.RefreshToken(ctx, expiredToken)

		require.Error(t, err)
		assert.Empty(t, newAccess)
	})

	t.Run("malformed token returns error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockServiceRepo{}
		svc := newTestService(repo)

		newAccess, err := svc.RefreshToken(ctx, "not-a-valid-jwt")

		require.Error(t, err)
		assert.Empty(t, newAccess)
	})

	t.Run("user deleted after token issued returns ErrUserNotFound", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockServiceRepo{
			getByIDErr: domain.ErrNotFound,
		}
		svc := newTestService(repo)

		refreshToken, err := auth.IssueRefreshToken(testJWTSecret, tenantID, userID, "member", testRefreshTTL)
		require.NoError(t, err)

		newAccess, err := svc.RefreshToken(ctx, refreshToken)

		require.Error(t, err)
		assert.Empty(t, newAccess)
		assert.ErrorIs(t, err, auth.ErrUserNotFound)
	})
}

// --- GetUser tests ---

func TestGetUser(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()

	t.Run("happy path returns user", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		expectedUser := &domain.User{
			ID:       userID,
			TenantID: tenantID,
			Email:    testEmail,
			Name:     testUserName,
			Role:     "member",
		}
		repo := &mockServiceRepo{
			getByIDUser: expectedUser,
		}
		svc := newTestService(repo)

		user, err := svc.GetUser(ctx, tenantID, userID)

		require.NoError(t, err)
		require.NotNil(t, user)
		assert.Equal(t, expectedUser.ID, user.ID)
		assert.Equal(t, expectedUser.Email, user.Email)
		assert.Equal(t, expectedUser.Name, user.Name)
	})

	t.Run("user not found returns wrapped error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repo := &mockServiceRepo{
			getByIDErr: domain.ErrNotFound,
		}
		svc := newTestService(repo)

		user, err := svc.GetUser(ctx, tenantID, userID)

		require.Error(t, err)
		assert.Nil(t, user)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("repo error is wrapped and propagated", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		repoErr := errors.New("connection timeout")
		repo := &mockServiceRepo{
			getByIDErr: repoErr,
		}
		svc := newTestService(repo)

		user, err := svc.GetUser(ctx, tenantID, userID)

		require.Error(t, err)
		assert.Nil(t, user)
		assert.ErrorIs(t, err, repoErr)
	})
}
