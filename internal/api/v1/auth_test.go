package v1_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/gosuda/aira/internal/api/v1"
	"github.com/gosuda/aira/internal/auth"
	"github.com/gosuda/aira/internal/domain"
)

// ---------------------------------------------------------------------------
// POST /auth/register
// ---------------------------------------------------------------------------

func TestRegister(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	now := time.Now()

	fixtureTenant := &domain.Tenant{
		ID:        tenantID,
		Name:      "Acme",
		Slug:      "acme",
		CreatedAt: now,
		UpdatedAt: now,
	}

	fixtureUser := &domain.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "alice@acme.io",
		Name:     "Alice",
		Role:     "member",
	}

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tenants: &mockTenantRepo{
				getBySlugFunc: func(_ context.Context, slug string) (*domain.Tenant, error) {
					require.Equal(t, "acme", slug)
					return fixtureTenant, nil
				},
			},
		}
		authSvc := &mockAuthService{
			registerFunc: func(_ context.Context, tid uuid.UUID, email, password, name string) (*domain.User, error) {
				assert.Equal(t, tenantID, tid)
				assert.Equal(t, "alice@acme.io", email)
				assert.Equal(t, "secretpw1", password)
				assert.Equal(t, "Alice", name)
				return fixtureUser, nil
			},
			loginFunc: func(_ context.Context, tid uuid.UUID, _, _ string) (string, string, error) {
				assert.Equal(t, tenantID, tid)
				return "access-tok", "refresh-tok", nil
			},
		}

		v1.RegisterAuthRoutes(api, store, authSvc)

		resp := api.Post("/auth/register", map[string]any{
			"tenant_slug": "acme",
			"email":       "alice@acme.io",
			"password":    "secretpw1",
			"name":        "Alice",
		})

		require.Equal(t, http.StatusOK, resp.Code)

		var body struct {
			User         *domain.User `json:"user"`
			AccessToken  string       `json:"access_token"`
			RefreshToken string       `json:"refresh_token"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, fixtureUser.Email, body.User.Email)
		assert.Empty(t, body.User.PasswordHash, "password hash must be stripped")
		assert.Equal(t, "access-tok", body.AccessToken)
		assert.Equal(t, "refresh-tok", body.RefreshToken)
	})

	t.Run("tenant_not_found", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tenants: &mockTenantRepo{
				getBySlugFunc: func(_ context.Context, _ string) (*domain.Tenant, error) {
					return nil, fmt.Errorf("repo.GetBySlug: %w", domain.ErrNotFound)
				},
			},
		}
		authSvc := &mockAuthService{}

		v1.RegisterAuthRoutes(api, store, authSvc)

		resp := api.Post("/auth/register", map[string]any{
			"tenant_slug": "no-such-tenant",
			"email":       "bob@test.io",
			"password":    "password123",
			"name":        "Bob",
		})

		assert.Equal(t, http.StatusNotFound, resp.Code)

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.EqualValues(t, http.StatusNotFound, errBody["status"])
	})

	t.Run("user_already_exists", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tenants: &mockTenantRepo{
				getBySlugFunc: func(_ context.Context, _ string) (*domain.Tenant, error) {
					return fixtureTenant, nil
				},
			},
		}
		authSvc := &mockAuthService{
			registerFunc: func(_ context.Context, _ uuid.UUID, _, _, _ string) (*domain.User, error) {
				return nil, fmt.Errorf("auth.Register: %w", auth.ErrUserAlreadyExists)
			},
		}

		v1.RegisterAuthRoutes(api, store, authSvc)

		resp := api.Post("/auth/register", map[string]any{
			"tenant_slug": "acme",
			"email":       "alice@acme.io",
			"password":    "password123",
			"name":        "Alice",
		})

		assert.Equal(t, http.StatusConflict, resp.Code)

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.EqualValues(t, http.StatusConflict, errBody["status"])
	})

	t.Run("login_after_register_fails", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tenants: &mockTenantRepo{
				getBySlugFunc: func(_ context.Context, _ string) (*domain.Tenant, error) {
					return fixtureTenant, nil
				},
			},
		}
		authSvc := &mockAuthService{
			registerFunc: func(_ context.Context, _ uuid.UUID, _, _, _ string) (*domain.User, error) {
				return fixtureUser, nil
			},
			loginFunc: func(_ context.Context, _ uuid.UUID, _, _ string) (string, string, error) {
				return "", "", errors.New("auth.Login: token issuance failed")
			},
		}

		v1.RegisterAuthRoutes(api, store, authSvc)

		resp := api.Post("/auth/register", map[string]any{
			"tenant_slug": "acme",
			"email":       "alice@acme.io",
			"password":    "password123",
			"name":        "Alice",
		})

		assert.Equal(t, http.StatusInternalServerError, resp.Code)

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.EqualValues(t, http.StatusInternalServerError, errBody["status"])
	})
}

// ---------------------------------------------------------------------------
// POST /auth/login
// ---------------------------------------------------------------------------

func TestLogin(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	now := time.Now()

	fixtureTenant := &domain.Tenant{
		ID:        tenantID,
		Name:      "Acme",
		Slug:      "acme",
		CreatedAt: now,
		UpdatedAt: now,
	}

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tenants: &mockTenantRepo{
				getBySlugFunc: func(_ context.Context, slug string) (*domain.Tenant, error) {
					require.Equal(t, "acme", slug)
					return fixtureTenant, nil
				},
			},
		}
		authSvc := &mockAuthService{
			loginFunc: func(_ context.Context, tid uuid.UUID, email, password string) (string, string, error) {
				assert.Equal(t, tenantID, tid)
				assert.Equal(t, "alice@acme.io", email)
				assert.Equal(t, "secretpw1", password)
				return "access-tok", "refresh-tok", nil
			},
		}

		v1.RegisterAuthRoutes(api, store, authSvc)

		resp := api.Post("/auth/login", map[string]any{
			"tenant_slug": "acme",
			"email":       "alice@acme.io",
			"password":    "secretpw1",
		})

		require.Equal(t, http.StatusOK, resp.Code)

		var body struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, "access-tok", body.AccessToken)
		assert.Equal(t, "refresh-tok", body.RefreshToken)
	})

	t.Run("tenant_not_found", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tenants: &mockTenantRepo{
				getBySlugFunc: func(_ context.Context, _ string) (*domain.Tenant, error) {
					return nil, fmt.Errorf("repo.GetBySlug: %w", domain.ErrNotFound)
				},
			},
		}
		authSvc := &mockAuthService{}

		v1.RegisterAuthRoutes(api, store, authSvc)

		resp := api.Post("/auth/login", map[string]any{
			"tenant_slug": "ghost",
			"email":       "bob@test.io",
			"password":    "password123",
		})

		assert.Equal(t, http.StatusNotFound, resp.Code)

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.EqualValues(t, http.StatusNotFound, errBody["status"])
	})

	t.Run("invalid_credentials", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tenants: &mockTenantRepo{
				getBySlugFunc: func(_ context.Context, _ string) (*domain.Tenant, error) {
					return fixtureTenant, nil
				},
			},
		}
		authSvc := &mockAuthService{
			loginFunc: func(_ context.Context, _ uuid.UUID, _, _ string) (string, string, error) {
				return "", "", fmt.Errorf("auth.Login: %w", auth.ErrInvalidCredentials)
			},
		}

		v1.RegisterAuthRoutes(api, store, authSvc)

		resp := api.Post("/auth/login", map[string]any{
			"tenant_slug": "acme",
			"email":       "alice@acme.io",
			"password":    "wrong-pw",
		})

		assert.Equal(t, http.StatusUnauthorized, resp.Code)

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.EqualValues(t, http.StatusUnauthorized, errBody["status"])
	})
}

// ---------------------------------------------------------------------------
// POST /auth/refresh
// ---------------------------------------------------------------------------

func TestRefreshToken(t *testing.T) {
	t.Parallel()

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tenants: &mockTenantRepo{},
		}
		authSvc := &mockAuthService{
			refreshTokenFunc: func(_ context.Context, rt string) (string, error) {
				require.Equal(t, "valid-refresh-tok", rt)
				return "new-access-tok", nil
			},
		}

		v1.RegisterAuthRoutes(api, store, authSvc)

		resp := api.Post("/auth/refresh", map[string]any{
			"refresh_token": "valid-refresh-tok",
		})

		require.Equal(t, http.StatusOK, resp.Code)

		var body struct {
			AccessToken string `json:"access_token"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, "new-access-tok", body.AccessToken)
	})

	t.Run("invalid_token", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tenants: &mockTenantRepo{},
		}
		authSvc := &mockAuthService{
			refreshTokenFunc: func(_ context.Context, _ string) (string, error) {
				return "", errors.New("auth.RefreshToken: token expired")
			},
		}

		v1.RegisterAuthRoutes(api, store, authSvc)

		resp := api.Post("/auth/refresh", map[string]any{
			"refresh_token": "expired-tok",
		})

		assert.Equal(t, http.StatusUnauthorized, resp.Code)

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.EqualValues(t, http.StatusUnauthorized, errBody["status"])
	})
}
