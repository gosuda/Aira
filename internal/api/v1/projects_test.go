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
// POST /projects
// ---------------------------------------------------------------------------

func TestCreateProject(t *testing.T) {
	t.Parallel()

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		tid := uuid.New()
		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{
				createFunc: func(_ context.Context, _ *domain.Project) error {
					return nil
				},
			},
		}
		v1.RegisterProjectRoutes(api, store)

		resp := api.PostCtx(tenantCtx(tid), "/projects", map[string]any{
			"name":     "my-project",
			"repo_url": "https://github.com/org/repo",
			"branch":   "develop",
		})

		require.Equal(t, http.StatusOK, resp.Code)

		var body domain.Project
		err := json.Unmarshal(resp.Body.Bytes(), &body)
		require.NoError(t, err)
		assert.Equal(t, "my-project", body.Name)
		assert.Equal(t, "https://github.com/org/repo", body.RepoURL)
		assert.Equal(t, "develop", body.Branch)
		assert.Equal(t, tid, body.TenantID)
		assert.NotEqual(t, uuid.Nil, body.ID)
	})

	t.Run("missing_tenant_context", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{},
		}
		v1.RegisterProjectRoutes(api, store)

		resp := api.PostCtx(context.Background(), "/projects", map[string]any{
			"name":     "my-project",
			"repo_url": "https://github.com/org/repo",
		})

		assert.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("invalid_repo_url_scheme", func(t *testing.T) {
		t.Parallel()

		tid := uuid.New()
		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{},
		}
		v1.RegisterProjectRoutes(api, store)

		resp := api.PostCtx(tenantCtx(tid), "/projects", map[string]any{
			"name":     "bad-url-project",
			"repo_url": "http://github.com/org/repo",
		})

		assert.Equal(t, http.StatusBadRequest, resp.Code)
	})

	t.Run("store_error", func(t *testing.T) {
		t.Parallel()

		tid := uuid.New()
		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{
				createFunc: func(_ context.Context, _ *domain.Project) error {
					return errors.New("db: connection refused")
				},
			},
		}
		v1.RegisterProjectRoutes(api, store)

		resp := api.PostCtx(tenantCtx(tid), "/projects", map[string]any{
			"name":     "failing-project",
			"repo_url": "https://github.com/org/repo",
		})

		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})
}

// ---------------------------------------------------------------------------
// GET /projects
// ---------------------------------------------------------------------------

func TestListProjects(t *testing.T) {
	t.Parallel()

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		tid := uuid.New()
		now := time.Now().Truncate(time.Second)
		projects := []*domain.Project{
			{ID: uuid.New(), TenantID: tid, Name: "alpha", RepoURL: "https://github.com/org/alpha", Branch: "main", Settings: json.RawMessage("{}"), CreatedAt: now},
			{ID: uuid.New(), TenantID: tid, Name: "beta", RepoURL: "git@github.com:org/beta.git", Branch: "develop", Settings: json.RawMessage("{}"), CreatedAt: now},
		}

		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{
				listFunc: func(_ context.Context, tenantID uuid.UUID) ([]*domain.Project, error) {
					assert.Equal(t, tid, tenantID)
					return projects, nil
				},
			},
		}
		v1.RegisterProjectRoutes(api, store)

		resp := api.GetCtx(tenantCtx(tid), "/projects")

		require.Equal(t, http.StatusOK, resp.Code)

		var body []domain.Project
		err := json.Unmarshal(resp.Body.Bytes(), &body)
		require.NoError(t, err)
		require.Len(t, body, 2)
		assert.Equal(t, "alpha", body[0].Name)
		assert.Equal(t, "beta", body[1].Name)
	})

	t.Run("store_error", func(t *testing.T) {
		t.Parallel()

		tid := uuid.New()
		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{
				listFunc: func(_ context.Context, _ uuid.UUID) ([]*domain.Project, error) {
					return nil, errors.New("db: timeout")
				},
			},
		}
		v1.RegisterProjectRoutes(api, store)

		resp := api.GetCtx(tenantCtx(tid), "/projects")

		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})
}

// ---------------------------------------------------------------------------
// GET /projects/{id}
// ---------------------------------------------------------------------------

func TestGetProject(t *testing.T) {
	t.Parallel()

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		tid := uuid.New()
		pid := uuid.New()
		project := &domain.Project{
			ID:        pid,
			TenantID:  tid,
			Name:      "my-project",
			RepoURL:   "https://github.com/org/repo",
			Branch:    "main",
			Settings:  json.RawMessage("{}"),
			CreatedAt: time.Now().Truncate(time.Second),
		}

		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{
				getByIDFunc: func(_ context.Context, tenantID, id uuid.UUID) (*domain.Project, error) {
					assert.Equal(t, tid, tenantID)
					assert.Equal(t, pid, id)
					return project, nil
				},
			},
		}
		v1.RegisterProjectRoutes(api, store)

		resp := api.GetCtx(tenantCtx(tid), "/projects/"+pid.String())

		require.Equal(t, http.StatusOK, resp.Code)

		var body domain.Project
		err := json.Unmarshal(resp.Body.Bytes(), &body)
		require.NoError(t, err)
		assert.Equal(t, pid, body.ID)
		assert.Equal(t, "my-project", body.Name)
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		tid := uuid.New()
		pid := uuid.New()

		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Project, error) {
					return nil, domain.ErrNotFound
				},
			},
		}
		v1.RegisterProjectRoutes(api, store)

		resp := api.GetCtx(tenantCtx(tid), "/projects/"+pid.String())

		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("store_error", func(t *testing.T) {
		t.Parallel()

		tid := uuid.New()
		pid := uuid.New()

		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Project, error) {
					return nil, errors.New("db: something broke")
				},
			},
		}
		v1.RegisterProjectRoutes(api, store)

		resp := api.GetCtx(tenantCtx(tid), "/projects/"+pid.String())

		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})
}

// ---------------------------------------------------------------------------
// PUT /projects/{id}
// ---------------------------------------------------------------------------

func TestUpdateProject(t *testing.T) {
	t.Parallel()

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		tid := uuid.New()
		pid := uuid.New()
		existing := &domain.Project{
			ID:        pid,
			TenantID:  tid,
			Name:      "old-name",
			RepoURL:   "https://github.com/org/old",
			Branch:    "main",
			Settings:  json.RawMessage("{}"),
			CreatedAt: time.Now().Truncate(time.Second),
		}

		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Project, error) {
					return existing, nil
				},
				updateFunc: func(_ context.Context, p *domain.Project) error {
					assert.Equal(t, "new-name", p.Name)
					assert.Equal(t, "https://github.com/org/new", p.RepoURL)
					return nil
				},
			},
		}
		v1.RegisterProjectRoutes(api, store)

		resp := api.PutCtx(tenantCtx(tid), "/projects/"+pid.String(), map[string]any{
			"name":     "new-name",
			"repo_url": "https://github.com/org/new",
		})

		require.Equal(t, http.StatusOK, resp.Code)

		var body domain.Project
		err := json.Unmarshal(resp.Body.Bytes(), &body)
		require.NoError(t, err)
		assert.Equal(t, "new-name", body.Name)
		assert.Equal(t, "https://github.com/org/new", body.RepoURL)
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		tid := uuid.New()
		pid := uuid.New()

		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Project, error) {
					return nil, domain.ErrNotFound
				},
			},
		}
		v1.RegisterProjectRoutes(api, store)

		resp := api.PutCtx(tenantCtx(tid), "/projects/"+pid.String(), map[string]any{
			"name": "updated",
		})

		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("partial_update_fields", func(t *testing.T) {
		t.Parallel()

		tid := uuid.New()
		pid := uuid.New()
		existing := &domain.Project{
			ID:        pid,
			TenantID:  tid,
			Name:      "original-name",
			RepoURL:   "https://github.com/org/repo",
			Branch:    "main",
			Settings:  json.RawMessage(`{"key":"value"}`),
			CreatedAt: time.Now().Truncate(time.Second),
		}

		var updated *domain.Project
		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Project, error) {
					return existing, nil
				},
				updateFunc: func(_ context.Context, p *domain.Project) error {
					updated = p
					return nil
				},
			},
		}
		v1.RegisterProjectRoutes(api, store)

		// Only send branch -- name, repo_url, settings should remain unchanged.
		resp := api.PutCtx(tenantCtx(tid), "/projects/"+pid.String(), map[string]any{
			"branch": "develop",
		})

		require.Equal(t, http.StatusOK, resp.Code)
		require.NotNil(t, updated)
		assert.Equal(t, "original-name", updated.Name, "name should remain unchanged")
		assert.Equal(t, "https://github.com/org/repo", updated.RepoURL, "repo_url should remain unchanged")
		assert.Equal(t, "develop", updated.Branch, "branch should be updated")
	})
}

// ---------------------------------------------------------------------------
// DELETE /projects/{id}
// ---------------------------------------------------------------------------

func TestDeleteProject(t *testing.T) {
	t.Parallel()

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		tid := uuid.New()
		pid := uuid.New()

		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{
				deleteFunc: func(_ context.Context, tenantID, id uuid.UUID) error {
					assert.Equal(t, tid, tenantID)
					assert.Equal(t, pid, id)
					return nil
				},
			},
		}
		v1.RegisterProjectRoutes(api, store)

		resp := api.DeleteCtx(tenantCtx(tid), "/projects/"+pid.String())

		assert.Equal(t, http.StatusNoContent, resp.Code)
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		tid := uuid.New()
		pid := uuid.New()

		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{
				deleteFunc: func(_ context.Context, _, _ uuid.UUID) error {
					return domain.ErrNotFound
				},
			},
		}
		v1.RegisterProjectRoutes(api, store)

		resp := api.DeleteCtx(tenantCtx(tid), "/projects/"+pid.String())

		assert.Equal(t, http.StatusNotFound, resp.Code)
	})
}
