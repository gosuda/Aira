package v1_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/gosuda/aira/internal/api/v1"
	"github.com/gosuda/aira/internal/domain"
	"github.com/gosuda/aira/internal/server/middleware"
)

// ---------------------------------------------------------------------------
// POST /tenants
// ---------------------------------------------------------------------------

func TestCreateTenant(t *testing.T) {
	t.Parallel()

	t.Run("happy_path_admin", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tenants: &mockTenantRepo{
				createFunc: func(_ context.Context, tenant *domain.Tenant) error {
					assert.Equal(t, "Acme Corp", tenant.Name)
					assert.Equal(t, "acme-corp", tenant.Slug)
					assert.NotEmpty(t, tenant.ID, "ID should be generated")
					assert.False(t, tenant.CreatedAt.IsZero(), "CreatedAt should be set")
					return nil
				},
			},
		}

		v1.RegisterTenantRoutes(api, store)

		ctx := adminCtx(fixedTenantID())
		resp := api.PostCtx(ctx, "/tenants", map[string]any{
			"name": "Acme Corp",
			"slug": "acme-corp",
		})

		require.Equal(t, http.StatusOK, resp.Code)

		var body domain.Tenant
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, "Acme Corp", body.Name)
		assert.Equal(t, "acme-corp", body.Slug)
		assert.NotEmpty(t, body.ID)
	})

	t.Run("non_admin_forbidden", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tenants: &mockTenantRepo{},
		}

		v1.RegisterTenantRoutes(api, store)

		// Context with role "member" instead of "admin".
		ctx := context.Background()
		ctx = context.WithValue(ctx, middleware.ContextKeyTenantID, fixedTenantID())
		ctx = context.WithValue(ctx, middleware.ContextKeyUserRole, "member")

		resp := api.PostCtx(ctx, "/tenants", map[string]any{
			"name": "Evil Corp",
			"slug": "evil-corp",
		})

		assert.Equal(t, http.StatusForbidden, resp.Code)

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.EqualValues(t, http.StatusForbidden, errBody["status"])
	})

	t.Run("missing_role_forbidden", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tenants: &mockTenantRepo{},
		}

		v1.RegisterTenantRoutes(api, store)

		// Context with no role at all.
		ctx := tenantCtx(fixedTenantID())

		resp := api.PostCtx(ctx, "/tenants", map[string]any{
			"name": "No Role Inc",
			"slug": "no-role",
		})

		assert.Equal(t, http.StatusForbidden, resp.Code)

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.EqualValues(t, http.StatusForbidden, errBody["status"])
	})

	t.Run("store_error", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tenants: &mockTenantRepo{
				createFunc: func(_ context.Context, _ *domain.Tenant) error {
					return errors.New("pg: connection refused")
				},
			},
		}

		v1.RegisterTenantRoutes(api, store)

		ctx := adminCtx(fixedTenantID())
		resp := api.PostCtx(ctx, "/tenants", map[string]any{
			"name": "Broken Corp",
			"slug": "broken-corp",
		})

		assert.Equal(t, http.StatusInternalServerError, resp.Code)

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.EqualValues(t, http.StatusInternalServerError, errBody["status"])
	})
}

// ---------------------------------------------------------------------------
// GET /tenants
// ---------------------------------------------------------------------------

func TestListTenants(t *testing.T) {
	t.Parallel()

	t.Run("happy_path_admin", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		expected := []*domain.Tenant{
			{ID: fixedTenantID(), Name: "Alpha", Slug: "alpha"},
			{ID: fixedTenantID2(), Name: "Beta", Slug: "beta"},
		}
		store := &mockDataStore{
			tenants: &mockTenantRepo{
				listFunc: func(_ context.Context) ([]*domain.Tenant, error) {
					return expected, nil
				},
			},
		}

		v1.RegisterTenantRoutes(api, store)

		ctx := adminCtx(fixedTenantID())
		resp := api.GetCtx(ctx, "/tenants")

		require.Equal(t, http.StatusOK, resp.Code)

		var body []*domain.Tenant
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		require.Len(t, body, 2)
		assert.Equal(t, "Alpha", body[0].Name)
		assert.Equal(t, "Beta", body[1].Name)
	})

	t.Run("non_admin_forbidden", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tenants: &mockTenantRepo{},
		}

		v1.RegisterTenantRoutes(api, store)

		// Context with role "viewer" instead of "admin".
		ctx := context.Background()
		ctx = context.WithValue(ctx, middleware.ContextKeyTenantID, fixedTenantID())
		ctx = context.WithValue(ctx, middleware.ContextKeyUserRole, "viewer")

		resp := api.GetCtx(ctx, "/tenants")

		assert.Equal(t, http.StatusForbidden, resp.Code)

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.EqualValues(t, http.StatusForbidden, errBody["status"])
	})
}

// ---------------------------------------------------------------------------
// Deterministic UUIDs for stable tests
// ---------------------------------------------------------------------------

func fixedTenantID() uuid.UUID {
	return uuid.MustParse("00000000-0000-0000-0000-000000000001")
}

func fixedTenantID2() uuid.UUID {
	return uuid.MustParse("00000000-0000-0000-0000-000000000002")
}
