package middleware_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gosuda/aira/internal/auth"
	"github.com/gosuda/aira/internal/domain"
	"github.com/gosuda/aira/internal/server/middleware"
)

// errNotFound is a sentinel used by mock repos when no API key matches.
var errNotFound = errors.New("api key not found")

// ---------------------------------------------------------------------------
// Mock UserRepository
// ---------------------------------------------------------------------------

// mockUserRepo implements domain.UserRepository with only the methods needed
// for API key authentication. All other methods panic if called.
type mockUserRepo struct {
	getAPIKeyByPrefixFunc    func(ctx context.Context, tenantID uuid.UUID, prefix string) (*domain.APIKey, error)
	updateAPIKeyLastUsedFunc func(ctx context.Context, id uuid.UUID) error
}

func (m *mockUserRepo) GetAPIKeyByPrefix(ctx context.Context, tenantID uuid.UUID, prefix string) (*domain.APIKey, error) {
	return m.getAPIKeyByPrefixFunc(ctx, tenantID, prefix)
}

func (m *mockUserRepo) UpdateAPIKeyLastUsed(ctx context.Context, id uuid.UUID) error {
	return m.updateAPIKeyLastUsedFunc(ctx, id)
}

// Stub methods — not exercised by Auth middleware.

func (m *mockUserRepo) Create(_ context.Context, _ *domain.User) error { panic("not implemented") }
func (m *mockUserRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*domain.User, error) {
	panic("not implemented")
}
func (m *mockUserRepo) GetByEmail(_ context.Context, _ uuid.UUID, _ string) (*domain.User, error) {
	panic("not implemented")
}
func (m *mockUserRepo) Update(_ context.Context, _ *domain.User) error { panic("not implemented") }
func (m *mockUserRepo) List(_ context.Context, _ uuid.UUID) ([]*domain.User, error) {
	panic("not implemented")
}
func (m *mockUserRepo) CreateOAuthLink(_ context.Context, _ *domain.UserOAuthLink) error {
	panic("not implemented")
}
func (m *mockUserRepo) GetOAuthLink(_ context.Context, _, _ string) (*domain.UserOAuthLink, error) {
	panic("not implemented")
}
func (m *mockUserRepo) DeleteOAuthLink(_ context.Context, _ uuid.UUID) error {
	panic("not implemented")
}
func (m *mockUserRepo) CreateMessengerLink(_ context.Context, _ *domain.UserMessengerLink) error {
	panic("not implemented")
}
func (m *mockUserRepo) GetMessengerLink(_ context.Context, _ uuid.UUID, _, _ string) (*domain.UserMessengerLink, error) {
	panic("not implemented")
}
func (m *mockUserRepo) ListMessengerLinks(_ context.Context, _ uuid.UUID) ([]*domain.UserMessengerLink, error) {
	panic("not implemented")
}
func (m *mockUserRepo) DeleteMessengerLink(_ context.Context, _ uuid.UUID) error {
	panic("not implemented")
}
func (m *mockUserRepo) CreateAPIKey(_ context.Context, _ *domain.APIKey) error {
	panic("not implemented")
}
func (m *mockUserRepo) ListAPIKeys(_ context.Context, _, _ uuid.UUID) ([]*domain.APIKey, error) {
	panic("not implemented")
}
func (m *mockUserRepo) DeleteAPIKey(_ context.Context, _ uuid.UUID) error {
	panic("not implemented")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// contextHandler captures context values set by middleware so tests can
// assert that the correct tenant, user, and role were injected.
type contextHandler struct {
	tenantID uuid.UUID
	userID   uuid.UUID
	role     string
	called   bool
}

func (h *contextHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.called = true
	h.tenantID, _ = middleware.TenantIDFromContext(r.Context())
	h.userID, _ = middleware.UserIDFromContext(r.Context())
	h.role, _ = middleware.RoleFromContext(r.Context())
	w.WriteHeader(http.StatusOK)
}

// hashKey returns the hex-encoded SHA-256 hash of rawKey.
func hashKey(rawKey string) string {
	h := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(h[:])
}

// setTenant injects a tenant ID into the request context.
func setTenant(r *http.Request, tenantID uuid.UUID) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.ContextKeyTenantID, tenantID)
	return r.WithContext(ctx)
}

// ===========================================================================
// 1. Context helpers
// ===========================================================================

func TestTenantIDFromContext(t *testing.T) {
	t.Parallel()

	t.Run("present", func(t *testing.T) {
		t.Parallel()

		want := uuid.New()
		ctx := context.WithValue(context.Background(), middleware.ContextKeyTenantID, want)

		got, ok := middleware.TenantIDFromContext(ctx)

		require.True(t, ok)
		assert.Equal(t, want, got)
	})

	t.Run("absent", func(t *testing.T) {
		t.Parallel()

		got, ok := middleware.TenantIDFromContext(context.Background())

		assert.False(t, ok)
		assert.Equal(t, uuid.Nil, got)
	})

	t.Run("wrong type", func(t *testing.T) {
		t.Parallel()

		// Store a string instead of uuid.UUID.
		ctx := context.WithValue(context.Background(), middleware.ContextKeyTenantID, "not-a-uuid")

		got, ok := middleware.TenantIDFromContext(ctx)

		assert.False(t, ok)
		assert.Equal(t, uuid.Nil, got)
	})
}

func TestUserIDFromContext(t *testing.T) {
	t.Parallel()

	t.Run("present", func(t *testing.T) {
		t.Parallel()

		want := uuid.New()
		ctx := context.WithValue(context.Background(), middleware.ContextKeyUserID, want)

		got, ok := middleware.UserIDFromContext(ctx)

		require.True(t, ok)
		assert.Equal(t, want, got)
	})

	t.Run("absent", func(t *testing.T) {
		t.Parallel()

		got, ok := middleware.UserIDFromContext(context.Background())

		assert.False(t, ok)
		assert.Equal(t, uuid.Nil, got)
	})

	t.Run("wrong type", func(t *testing.T) {
		t.Parallel()

		ctx := context.WithValue(context.Background(), middleware.ContextKeyUserID, 42)

		got, ok := middleware.UserIDFromContext(ctx)

		assert.False(t, ok)
		assert.Equal(t, uuid.Nil, got)
	})
}

func TestRoleFromContext(t *testing.T) {
	t.Parallel()

	t.Run("present", func(t *testing.T) {
		t.Parallel()

		ctx := context.WithValue(context.Background(), middleware.ContextKeyUserRole, "admin")

		got, ok := middleware.RoleFromContext(ctx)

		require.True(t, ok)
		assert.Equal(t, "admin", got)
	})

	t.Run("absent", func(t *testing.T) {
		t.Parallel()

		got, ok := middleware.RoleFromContext(context.Background())

		assert.False(t, ok)
		assert.Empty(t, got)
	})

	t.Run("wrong type", func(t *testing.T) {
		t.Parallel()

		ctx := context.WithValue(context.Background(), middleware.ContextKeyUserRole, 123)

		got, ok := middleware.RoleFromContext(ctx)

		assert.False(t, ok)
		assert.Empty(t, got)
	})
}

// ===========================================================================
// 2. RequireTenant middleware
// ===========================================================================

func TestRequireTenant_PassesWithValidTenantID(t *testing.T) {
	t.Parallel()

	handler := middleware.RequireTenant()(okHandler)
	req := setTenant(httptest.NewRequest(http.MethodGet, "/", http.NoBody), uuid.New())
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireTenant_BlocksWhenTenantAbsent(t *testing.T) {
	t.Parallel()

	handler := middleware.RequireTenant()(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "valid tenant required")
}

func TestRequireTenant_BlocksNilTenantID(t *testing.T) {
	t.Parallel()

	handler := middleware.RequireTenant()(okHandler)
	req := setTenant(httptest.NewRequest(http.MethodGet, "/", http.NoBody), uuid.Nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "valid tenant required")
}

// ===========================================================================
// 3. RateLimit middleware
// ===========================================================================

func TestRateLimit_NoTenantInContext_PassesThrough(t *testing.T) {
	t.Parallel()

	handler := middleware.RateLimit(1, 1)(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRateLimit_FirstRequestWithTenant_Passes(t *testing.T) {
	t.Parallel()

	handler := middleware.RateLimit(1, 1)(okHandler)
	req := setTenant(httptest.NewRequest(http.MethodGet, "/", http.NoBody), uuid.New())
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRateLimit_BurstExceeded_Returns429(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	// Very low rate (effectively zero refill during the test) with burst of 2.
	handler := middleware.RateLimit(0.001, 2)(okHandler)

	// First two requests consume the burst.
	for i := range 2 {
		req := setTenant(httptest.NewRequest(http.MethodGet, "/", http.NoBody), tenantID)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equalf(t, http.StatusOK, rec.Code, "request %d should pass", i+1)
	}

	// Third request exceeds burst.
	req := setTenant(httptest.NewRequest(http.MethodGet, "/", http.NoBody), tenantID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Contains(t, rec.Body.String(), "rate limit exceeded")
}

func TestRateLimit_IndependentPerTenant(t *testing.T) {
	t.Parallel()

	tenantA := uuid.New()
	tenantB := uuid.New()
	handler := middleware.RateLimit(0.001, 1)(okHandler)

	// Exhaust tenant A's burst.
	reqA := setTenant(httptest.NewRequest(http.MethodGet, "/", http.NoBody), tenantA)
	recA := httptest.NewRecorder()
	handler.ServeHTTP(recA, reqA)
	require.Equal(t, http.StatusOK, recA.Code)

	// Tenant A is now exhausted.
	reqA2 := setTenant(httptest.NewRequest(http.MethodGet, "/", http.NoBody), tenantA)
	recA2 := httptest.NewRecorder()
	handler.ServeHTTP(recA2, reqA2)
	assert.Equal(t, http.StatusTooManyRequests, recA2.Code)

	// Tenant B should still be allowed.
	reqB := setTenant(httptest.NewRequest(http.MethodGet, "/", http.NoBody), tenantB)
	recB := httptest.NewRecorder()

	handler.ServeHTTP(recB, reqB)

	assert.Equal(t, http.StatusOK, recB.Code)
}

// ===========================================================================
// 4. Auth middleware
// ===========================================================================

const testJWTSecret = "test-jwt-secret-for-middleware-tests"

// newMockRepo creates a mockUserRepo that returns errNotFound for any prefix
// by default. Callers override getAPIKeyByPrefixFunc for API key tests.
func newMockRepo() *mockUserRepo {
	return &mockUserRepo{
		getAPIKeyByPrefixFunc: func(_ context.Context, _ uuid.UUID, _ string) (*domain.APIKey, error) {
			return nil, errNotFound
		},
		updateAPIKeyLastUsedFunc: func(_ context.Context, _ uuid.UUID) error {
			return nil
		},
	}
}

// --- JWT auth path ---

func TestAuth_JWT_ValidToken_PopulatesContext(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()
	role := "admin"

	token, err := auth.IssueAccessToken(testJWTSecret, tenantID, userID, role, 15*time.Minute)
	require.NoError(t, err)

	capture := &contextHandler{}
	handler := middleware.Auth(testJWTSecret, newMockRepo())(capture)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.True(t, capture.called, "inner handler must be called")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, tenantID, capture.tenantID)
	assert.Equal(t, userID, capture.userID)
	assert.Equal(t, role, capture.role)
}

func TestAuth_JWT_InvalidToken_Returns401(t *testing.T) {
	t.Parallel()

	handler := middleware.Auth(testJWTSecret, newMockRepo())(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer totally.invalid.token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Unauthorized")
}

func TestAuth_JWT_ExpiredToken_Returns401(t *testing.T) {
	t.Parallel()

	// Issue a token that expired 1 second ago.
	token, err := auth.IssueAccessToken(testJWTSecret, uuid.New(), uuid.New(), "member", -1*time.Second)
	require.NoError(t, err)

	handler := middleware.Auth(testJWTSecret, newMockRepo())(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuth_JWT_WrongSecret_Returns401(t *testing.T) {
	t.Parallel()

	token, err := auth.IssueAccessToken("correct-secret", uuid.New(), uuid.New(), "member", 15*time.Minute)
	require.NoError(t, err)

	handler := middleware.Auth("wrong-secret", newMockRepo())(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- Bearer format variations ---

func TestAuth_BearerFormat(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()
	token, err := auth.IssueAccessToken(testJWTSecret, tenantID, userID, "member", 15*time.Minute)
	require.NoError(t, err)

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{name: "uppercase Bearer", authHeader: "Bearer " + token, wantStatus: http.StatusOK},
		{name: "lowercase bearer", authHeader: "bearer " + token, wantStatus: http.StatusOK},
		{name: "mixed case BEARER", authHeader: "BEARER " + token, wantStatus: http.StatusOK},
		{name: "Basic scheme falls through to 401", authHeader: "Basic " + token, wantStatus: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := middleware.Auth(testJWTSecret, newMockRepo())(okHandler)
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.Header.Set("Authorization", tt.authHeader)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

// --- API key auth path ---

func TestAuth_APIKey_Valid_PopulatesContext(t *testing.T) {
	t.Parallel()

	rawKey := "testkey-abcdefgh1234"
	prefix := rawKey[:8]
	keyHash := hashKey(rawKey)
	tenantID := uuid.New()
	userID := uuid.New()
	keyID := uuid.New()

	repo := newMockRepo()
	repo.getAPIKeyByPrefixFunc = func(_ context.Context, _ uuid.UUID, p string) (*domain.APIKey, error) {
		if p == prefix {
			return &domain.APIKey{
				ID:        keyID,
				TenantID:  tenantID,
				UserID:    userID,
				Name:      "test-key",
				KeyHash:   keyHash,
				Prefix:    prefix,
				ExpiresAt: nil, // never expires
				CreatedAt: time.Now(),
			}, nil
		}
		return nil, errNotFound
	}

	capture := &contextHandler{}
	handler := middleware.Auth(testJWTSecret, repo)(capture)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-API-Key", rawKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.True(t, capture.called, "inner handler must be called")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, tenantID, capture.tenantID)
	assert.Equal(t, userID, capture.userID)
	assert.Equal(t, "member", capture.role, "API key auth always assigns member role")
}

func TestAuth_APIKey_ShortKey_Returns401(t *testing.T) {
	t.Parallel()

	handler := middleware.Auth(testJWTSecret, newMockRepo())(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-API-Key", "short") // < 8 chars
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuth_APIKey_HashMismatch_Returns401(t *testing.T) {
	t.Parallel()

	rawKey := "testkey-abcdefgh1234"
	prefix := rawKey[:8]
	tenantID := uuid.New()
	userID := uuid.New()

	repo := newMockRepo()
	repo.getAPIKeyByPrefixFunc = func(_ context.Context, _ uuid.UUID, p string) (*domain.APIKey, error) {
		if p == prefix {
			return &domain.APIKey{
				ID:        uuid.New(),
				TenantID:  tenantID,
				UserID:    userID,
				Name:      "test-key",
				KeyHash:   "wrong-hash-value",
				Prefix:    prefix,
				ExpiresAt: nil,
				CreatedAt: time.Now(),
			}, nil
		}
		return nil, errNotFound
	}

	handler := middleware.Auth(testJWTSecret, repo)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-API-Key", rawKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuth_APIKey_Expired_Returns401(t *testing.T) {
	t.Parallel()

	rawKey := "testkey-abcdefgh1234"
	prefix := rawKey[:8]
	keyHash := hashKey(rawKey)
	expired := time.Now().Add(-1 * time.Hour)

	repo := newMockRepo()
	repo.getAPIKeyByPrefixFunc = func(_ context.Context, _ uuid.UUID, p string) (*domain.APIKey, error) {
		if p == prefix {
			return &domain.APIKey{
				ID:        uuid.New(),
				TenantID:  uuid.New(),
				UserID:    uuid.New(),
				Name:      "expired-key",
				KeyHash:   keyHash,
				Prefix:    prefix,
				ExpiresAt: &expired,
				CreatedAt: time.Now().Add(-2 * time.Hour),
			}, nil
		}
		return nil, errNotFound
	}

	handler := middleware.Auth(testJWTSecret, repo)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-API-Key", rawKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- No credentials ---

func TestAuth_NoCredentials_Returns401(t *testing.T) {
	t.Parallel()

	handler := middleware.Auth(testJWTSecret, newMockRepo())(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing or invalid credentials")
}
