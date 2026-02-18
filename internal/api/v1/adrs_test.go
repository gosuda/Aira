package v1_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/gosuda/aira/internal/api/v1"
	"github.com/gosuda/aira/internal/domain"
)

// ---------------------------------------------------------------------------
// TestListADRs
// ---------------------------------------------------------------------------

func TestListADRs(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	projectID := uuid.New()
	now := time.Now()

	sampleADRs := []*domain.ADR{
		{
			ID: uuid.New(), TenantID: tenantID, ProjectID: projectID,
			Sequence: 1, Title: "ADR-0001 Use PostgreSQL",
			Status: domain.ADRStatusAccepted, Context: "Need a DB",
			Decision: "Use PostgreSQL", CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: uuid.New(), TenantID: tenantID, ProjectID: projectID,
			Sequence: 2, Title: "ADR-0002 Use Redis",
			Status: domain.ADRStatusDraft, Context: "Need caching",
			Decision: "Use Redis", CreatedAt: now, UpdatedAt: now,
		},
	}

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			adrs: &mockADRRepo{
				listByProjectFunc: func(_ context.Context, tid, pid uuid.UUID) ([]*domain.ADR, error) {
					assert.Equal(t, tenantID, tid)
					assert.Equal(t, projectID, pid)
					return sampleADRs, nil
				},
			},
		}
		v1.RegisterADRRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.GetCtx(ctx, "/adrs?project_id="+projectID.String())

		require.Equal(t, http.StatusOK, resp.Code)

		var body []*domain.ADR
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Len(t, body, 2)
		assert.Equal(t, "ADR-0001 Use PostgreSQL", body[0].Title)
		assert.Equal(t, domain.ADRStatusAccepted, body[0].Status)
		assert.Equal(t, "ADR-0002 Use Redis", body[1].Title)
		assert.Equal(t, domain.ADRStatusDraft, body[1].Status)
	})

	t.Run("store_error", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			adrs: &mockADRRepo{
				listByProjectFunc: func(_ context.Context, _, _ uuid.UUID) ([]*domain.ADR, error) {
					return nil, errors.New("db connection refused")
				},
			},
		}
		v1.RegisterADRRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.GetCtx(ctx, "/adrs?project_id="+projectID.String())

		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})
}

// ---------------------------------------------------------------------------
// TestGetADR
// ---------------------------------------------------------------------------

func TestGetADR(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	adrID := uuid.New()
	now := time.Now()

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		adr := &domain.ADR{
			ID: adrID, TenantID: tenantID, ProjectID: uuid.New(),
			Sequence: 3, Title: "ADR-0003 Use gRPC",
			Status:   domain.ADRStatusProposed,
			Context:  "Need inter-service communication",
			Decision: "Use gRPC for service mesh",
			Drivers:  []string{"performance", "type safety"},
			Options:  []string{"REST", "gRPC", "GraphQL"},
			Consequences: domain.ADRConsequences{
				Good:    []string{"type-safe contracts"},
				Bad:     []string{"learning curve"},
				Neutral: []string{"need protobuf tooling"},
			},
			CreatedAt: now, UpdatedAt: now,
		}
		store := &mockDataStore{
			adrs: &mockADRRepo{
				getByIDFunc: func(_ context.Context, tid, id uuid.UUID) (*domain.ADR, error) {
					assert.Equal(t, tenantID, tid)
					assert.Equal(t, adrID, id)
					return adr, nil
				},
			},
		}
		v1.RegisterADRRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.GetCtx(ctx, "/adrs/"+adrID.String())

		require.Equal(t, http.StatusOK, resp.Code)

		var body domain.ADR
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, adrID, body.ID)
		assert.Equal(t, "ADR-0003 Use gRPC", body.Title)
		assert.Equal(t, domain.ADRStatusProposed, body.Status)
		assert.Equal(t, 3, body.Sequence)
		assert.Equal(t, "Use gRPC for service mesh", body.Decision)
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			adrs: &mockADRRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ADR, error) {
					return nil, domain.ErrNotFound
				},
			},
		}
		v1.RegisterADRRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.GetCtx(ctx, "/adrs/"+uuid.New().String())

		assert.Equal(t, http.StatusNotFound, resp.Code)

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.Contains(t, errBody["detail"], "ADR not found")
	})
}

// ---------------------------------------------------------------------------
// TestUpdateADRStatus
// ---------------------------------------------------------------------------

func TestUpdateADRStatus(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	adrID := uuid.New()
	now := time.Now()

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			adrs: &mockADRRepo{
				getByIDFunc: func(_ context.Context, tid, id uuid.UUID) (*domain.ADR, error) {
					assert.Equal(t, tenantID, tid)
					assert.Equal(t, adrID, id)
					return &domain.ADR{
						ID: adrID, TenantID: tenantID, ProjectID: uuid.New(),
						Sequence: 1, Title: "ADR-0001",
						Status:    domain.ADRStatusDraft,
						CreatedAt: now, UpdatedAt: now,
					}, nil
				},
				updateStatusFunc: func(_ context.Context, tid, id uuid.UUID, status domain.ADRStatus) error {
					assert.Equal(t, tenantID, tid)
					assert.Equal(t, adrID, id)
					assert.Equal(t, domain.ADRStatusProposed, status)
					return nil
				},
			},
		}
		v1.RegisterADRRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.PatchCtx(ctx, "/adrs/"+adrID.String()+"/status", map[string]any{
			"status": "proposed",
		})

		require.Equal(t, http.StatusOK, resp.Code)

		var body domain.ADR
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, domain.ADRStatusProposed, body.Status)
		assert.Equal(t, adrID, body.ID)
	})

	t.Run("proposed_to_accepted", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			adrs: &mockADRRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ADR, error) {
					return &domain.ADR{
						ID: adrID, TenantID: tenantID,
						Status:    domain.ADRStatusProposed,
						CreatedAt: now, UpdatedAt: now,
					}, nil
				},
				updateStatusFunc: func(_ context.Context, _, _ uuid.UUID, status domain.ADRStatus) error {
					assert.Equal(t, domain.ADRStatusAccepted, status)
					return nil
				},
			},
		}
		v1.RegisterADRRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.PatchCtx(ctx, "/adrs/"+adrID.String()+"/status", map[string]any{
			"status": "accepted",
		})

		require.Equal(t, http.StatusOK, resp.Code)

		var body domain.ADR
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, domain.ADRStatusAccepted, body.Status)
	})

	t.Run("accepted_to_deprecated", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			adrs: &mockADRRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ADR, error) {
					return &domain.ADR{
						ID: adrID, TenantID: tenantID,
						Status:    domain.ADRStatusAccepted,
						CreatedAt: now, UpdatedAt: now,
					}, nil
				},
				updateStatusFunc: func(_ context.Context, _, _ uuid.UUID, status domain.ADRStatus) error {
					assert.Equal(t, domain.ADRStatusDeprecated, status)
					return nil
				},
			},
		}
		v1.RegisterADRRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.PatchCtx(ctx, "/adrs/"+adrID.String()+"/status", map[string]any{
			"status": "deprecated",
		})

		require.Equal(t, http.StatusOK, resp.Code)

		var body domain.ADR
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, domain.ADRStatusDeprecated, body.Status)
	})

	t.Run("invalid_status", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			adrs: &mockADRRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ADR, error) {
					return &domain.ADR{
						ID: adrID, TenantID: tenantID,
						Status:    domain.ADRStatusDraft,
						CreatedAt: now, UpdatedAt: now,
					}, nil
				},
			},
		}
		v1.RegisterADRRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.PatchCtx(ctx, "/adrs/"+adrID.String()+"/status", map[string]any{
			"status": "nonexistent",
		})

		assert.Equal(t, http.StatusBadRequest, resp.Code)

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.Contains(t, errBody["detail"], "unknown ADR status")
	})

	t.Run("invalid_transition", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			adrs: &mockADRRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ADR, error) {
					return &domain.ADR{
						ID: adrID, TenantID: tenantID,
						Status:    domain.ADRStatusDraft,
						CreatedAt: now, UpdatedAt: now,
					}, nil
				},
			},
		}
		v1.RegisterADRRoutes(api, store)

		// draft -> accepted is not allowed; must go draft -> proposed first.
		ctx := tenantCtx(tenantID)
		resp := api.PatchCtx(ctx, "/adrs/"+adrID.String()+"/status", map[string]any{
			"status": "accepted",
		})

		assert.Equal(t, http.StatusBadRequest, resp.Code)

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.Contains(t, errBody["detail"], "invalid status transition")
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			adrs: &mockADRRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ADR, error) {
					return nil, domain.ErrNotFound
				},
			},
		}
		v1.RegisterADRRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.PatchCtx(ctx, "/adrs/"+uuid.New().String()+"/status", map[string]any{
			"status": "proposed",
		})

		assert.Equal(t, http.StatusNotFound, resp.Code)

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.Contains(t, errBody["detail"], "ADR not found")
	})
}
