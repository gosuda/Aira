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
// TestCreateTask
// ---------------------------------------------------------------------------

func TestCreateTask(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	projectID := uuid.New()
	adrID := uuid.New()

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		var createCalled bool
		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{
				getByIDFunc: func(_ context.Context, tid, pid uuid.UUID) (*domain.Project, error) {
					assert.Equal(t, tenantID, tid)
					assert.Equal(t, projectID, pid)
					return &domain.Project{ID: projectID, TenantID: tenantID}, nil
				},
			},
			tasks: &mockTaskRepo{
				createFunc: func(_ context.Context, task *domain.Task) error {
					createCalled = true
					assert.Equal(t, tenantID, task.TenantID)
					assert.Equal(t, projectID, task.ProjectID)
					assert.Equal(t, "Implement login", task.Title)
					assert.Equal(t, domain.TaskStatusBacklog, task.Status)
					return nil
				},
			},
			adrs: &mockADRRepo{},
		}
		v1.RegisterTaskRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.PostCtx(ctx, "/tasks", map[string]any{
			"project_id":  projectID.String(),
			"title":       "Implement login",
			"description": "Add OAuth2 login flow",
			"priority":    1,
		})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.True(t, createCalled, "store.Tasks().Create must be invoked")

		var body domain.Task
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, "Implement login", body.Title)
		assert.Equal(t, "Add OAuth2 login flow", body.Description)
		assert.Equal(t, domain.TaskStatusBacklog, body.Status)
		assert.Equal(t, 1, body.Priority)
		assert.Equal(t, tenantID, body.TenantID)
		assert.Equal(t, projectID, body.ProjectID)
		assert.NotEqual(t, uuid.Nil, body.ID)
	})

	t.Run("happy_path_with_adr", func(t *testing.T) {
		t.Parallel()

		var adrLookedUp, createCalled bool
		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Project, error) {
					return &domain.Project{ID: projectID, TenantID: tenantID}, nil
				},
			},
			adrs: &mockADRRepo{
				getByIDFunc: func(_ context.Context, tid, aid uuid.UUID) (*domain.ADR, error) {
					adrLookedUp = true
					assert.Equal(t, tenantID, tid)
					assert.Equal(t, adrID, aid)
					return &domain.ADR{ID: adrID}, nil
				},
			},
			tasks: &mockTaskRepo{
				createFunc: func(_ context.Context, task *domain.Task) error {
					createCalled = true
					require.NotNil(t, task.ADRID)
					assert.Equal(t, adrID, *task.ADRID)
					return nil
				},
			},
		}
		v1.RegisterTaskRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.PostCtx(ctx, "/tasks", map[string]any{
			"project_id": projectID.String(),
			"title":      "ADR task",
			"adr_id":     adrID.String(),
		})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.True(t, adrLookedUp, "ADR must be looked up when adr_id is provided")
		assert.True(t, createCalled, "store.Tasks().Create must be invoked")
	})

	t.Run("missing_tenant", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{},
			tasks:    &mockTaskRepo{},
			adrs:     &mockADRRepo{},
		}
		v1.RegisterTaskRoutes(api, store)

		// No tenant in context.
		resp := api.PostCtx(context.Background(), "/tasks", map[string]any{
			"project_id": projectID.String(),
			"title":      "No tenant",
		})

		assert.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("project_not_found", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Project, error) {
					return nil, domain.ErrNotFound
				},
			},
			tasks: &mockTaskRepo{},
			adrs:  &mockADRRepo{},
		}
		v1.RegisterTaskRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.PostCtx(ctx, "/tasks", map[string]any{
			"project_id": projectID.String(),
			"title":      "Task for missing project",
		})

		assert.Equal(t, http.StatusNotFound, resp.Code)

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.Contains(t, errBody["detail"], "project not found")
	})

	t.Run("adr_not_found", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Project, error) {
					return &domain.Project{ID: projectID}, nil
				},
			},
			adrs: &mockADRRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ADR, error) {
					return nil, domain.ErrNotFound
				},
			},
			tasks: &mockTaskRepo{},
		}
		v1.RegisterTaskRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.PostCtx(ctx, "/tasks", map[string]any{
			"project_id": projectID.String(),
			"title":      "Task with missing ADR",
			"adr_id":     uuid.New().String(),
		})

		assert.Equal(t, http.StatusNotFound, resp.Code)

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.Contains(t, errBody["detail"], "ADR not found")
	})

	t.Run("store_error", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			projects: &mockProjectRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Project, error) {
					return &domain.Project{ID: projectID}, nil
				},
			},
			tasks: &mockTaskRepo{
				createFunc: func(_ context.Context, _ *domain.Task) error {
					return errors.New("db connection lost")
				},
			},
			adrs: &mockADRRepo{},
		}
		v1.RegisterTaskRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.PostCtx(ctx, "/tasks", map[string]any{
			"project_id": projectID.String(),
			"title":      "Will fail to persist",
		})

		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})
}

// ---------------------------------------------------------------------------
// TestListTasks
// ---------------------------------------------------------------------------

func TestListTasks(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	projectID := uuid.New()
	now := time.Now()

	// Factory creates fresh task slices per subtest to avoid shared-pointer races with t.Parallel().
	makeSampleTasks := func() []*domain.Task {
		return []*domain.Task{
			{ID: uuid.New(), TenantID: tenantID, ProjectID: projectID, Title: "Task A", Status: domain.TaskStatusBacklog, CreatedAt: now, UpdatedAt: now},
			{ID: uuid.New(), TenantID: tenantID, ProjectID: projectID, Title: "Task B", Status: domain.TaskStatusInProgress, CreatedAt: now, UpdatedAt: now},
		}
	}

	t.Run("happy_path_all", func(t *testing.T) {
		t.Parallel()

		var listCalled bool
		tasks := makeSampleTasks()
		_, api := humatest.New(t)
		store := &mockDataStore{
			tasks: &mockTaskRepo{
				listByProjectPaginatedFunc: func(_ context.Context, tid, pid uuid.UUID, limit, offset int) ([]*domain.Task, error) {
					listCalled = true
					assert.Equal(t, tenantID, tid)
					assert.Equal(t, projectID, pid)
					assert.Equal(t, 50, limit, "default limit must be 50")
					assert.Equal(t, 0, offset, "default offset must be 0")
					return tasks, nil
				},
			},
		}
		v1.RegisterTaskRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.GetCtx(ctx, "/tasks?project_id="+projectID.String())

		require.Equal(t, http.StatusOK, resp.Code)
		assert.True(t, listCalled, "ListByProjectPaginated must be invoked")

		var body []*domain.Task
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Len(t, body, 2)
		assert.Equal(t, "Task A", body[0].Title)
		assert.Equal(t, "Task B", body[1].Title)
	})

	t.Run("filtered_by_status", func(t *testing.T) {
		t.Parallel()

		var listByStatusCalled bool
		tasks := makeSampleTasks()
		_, api := humatest.New(t)
		store := &mockDataStore{
			tasks: &mockTaskRepo{
				listByStatusFunc: func(_ context.Context, tid, pid uuid.UUID, status domain.TaskStatus) ([]*domain.Task, error) {
					listByStatusCalled = true
					assert.Equal(t, tenantID, tid)
					assert.Equal(t, projectID, pid)
					assert.Equal(t, domain.TaskStatusBacklog, status)
					return []*domain.Task{tasks[0]}, nil
				},
			},
		}
		v1.RegisterTaskRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.GetCtx(ctx, "/tasks?project_id="+projectID.String()+"&status=backlog")

		require.Equal(t, http.StatusOK, resp.Code)
		assert.True(t, listByStatusCalled, "ListByStatus must be invoked when status filter is set")

		var body []*domain.Task
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Len(t, body, 1)
		assert.Equal(t, domain.TaskStatusBacklog, body[0].Status)
	})

	t.Run("store_error", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tasks: &mockTaskRepo{
				listByProjectPaginatedFunc: func(_ context.Context, _, _ uuid.UUID, _, _ int) ([]*domain.Task, error) {
					return nil, errors.New("db timeout")
				},
			},
		}
		v1.RegisterTaskRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.GetCtx(ctx, "/tasks?project_id="+projectID.String())

		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})
}

// ---------------------------------------------------------------------------
// TestGetTask
// ---------------------------------------------------------------------------

func TestGetTask(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	taskID := uuid.New()
	now := time.Now()

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		task := &domain.Task{
			ID: taskID, TenantID: tenantID, ProjectID: uuid.New(),
			Title: "Found task", Status: domain.TaskStatusReview,
			CreatedAt: now, UpdatedAt: now,
		}
		store := &mockDataStore{
			tasks: &mockTaskRepo{
				getByIDFunc: func(_ context.Context, tid, id uuid.UUID) (*domain.Task, error) {
					assert.Equal(t, tenantID, tid)
					assert.Equal(t, taskID, id)
					return task, nil
				},
			},
		}
		v1.RegisterTaskRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.GetCtx(ctx, "/tasks/"+taskID.String())

		require.Equal(t, http.StatusOK, resp.Code)

		var body domain.Task
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, taskID, body.ID)
		assert.Equal(t, "Found task", body.Title)
		assert.Equal(t, domain.TaskStatusReview, body.Status)
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tasks: &mockTaskRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Task, error) {
					return nil, domain.ErrNotFound
				},
			},
		}
		v1.RegisterTaskRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.GetCtx(ctx, "/tasks/"+uuid.New().String())

		assert.Equal(t, http.StatusNotFound, resp.Code)

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.Contains(t, errBody["detail"], "task not found")
	})
}

// ---------------------------------------------------------------------------
// TestUpdateTask
// ---------------------------------------------------------------------------

func TestUpdateTask(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	taskID := uuid.New()
	projectID := uuid.New()
	now := time.Now()

	baseTask := func() *domain.Task {
		return &domain.Task{
			ID: taskID, TenantID: tenantID, ProjectID: projectID,
			Title: "Original", Description: "Original desc",
			Status: domain.TaskStatusBacklog, Priority: 1,
			CreatedAt: now, UpdatedAt: now,
		}
	}

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		var updated *domain.Task
		store := &mockDataStore{
			tasks: &mockTaskRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Task, error) {
					return baseTask(), nil
				},
				updateFunc: func(_ context.Context, task *domain.Task) error {
					updated = task
					return nil
				},
			},
		}
		v1.RegisterTaskRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.PutCtx(ctx, "/tasks/"+taskID.String(), map[string]any{
			"title":       "Updated title",
			"description": "Updated desc",
			"priority":    5,
		})

		require.Equal(t, http.StatusOK, resp.Code)
		require.NotNil(t, updated)
		assert.Equal(t, "Updated title", updated.Title)
		assert.Equal(t, "Updated desc", updated.Description)
		assert.Equal(t, 5, updated.Priority)

		var body domain.Task
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, "Updated title", body.Title)
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tasks: &mockTaskRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Task, error) {
					return nil, domain.ErrNotFound
				},
			},
		}
		v1.RegisterTaskRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.PutCtx(ctx, "/tasks/"+taskID.String(), map[string]any{
			"title": "Won't apply",
		})

		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("partial_updates", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		var updated *domain.Task
		store := &mockDataStore{
			tasks: &mockTaskRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Task, error) {
					return baseTask(), nil
				},
				updateFunc: func(_ context.Context, task *domain.Task) error {
					updated = task
					return nil
				},
			},
		}
		v1.RegisterTaskRoutes(api, store)

		// Only update title, leave description and priority unchanged.
		ctx := tenantCtx(tenantID)
		resp := api.PutCtx(ctx, "/tasks/"+taskID.String(), map[string]any{
			"title": "Only title changed",
		})

		require.Equal(t, http.StatusOK, resp.Code)
		require.NotNil(t, updated)
		assert.Equal(t, "Only title changed", updated.Title)
		assert.Equal(t, "Original desc", updated.Description, "description should be preserved")
		assert.Equal(t, 1, updated.Priority, "priority should be preserved")
	})
}

// ---------------------------------------------------------------------------
// TestTransitionTaskStatus
// ---------------------------------------------------------------------------

func TestTransitionTaskStatus(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	taskID := uuid.New()
	now := time.Now()

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		var updateStatusCount int
		_, api := humatest.New(t)
		store := &mockDataStore{
			tasks: &mockTaskRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Task, error) {
					return &domain.Task{
						ID: taskID, TenantID: tenantID, ProjectID: uuid.New(),
						Title: "Transition me", Status: domain.TaskStatusBacklog,
						CreatedAt: now, UpdatedAt: now,
					}, nil
				},
				updateStatusFunc: func(_ context.Context, tid, id uuid.UUID, status domain.TaskStatus) error {
					updateStatusCount++
					assert.Equal(t, tenantID, tid)
					assert.Equal(t, taskID, id)
					assert.Equal(t, domain.TaskStatusInProgress, status)
					return nil
				},
			},
		}
		v1.RegisterTaskRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.PatchCtx(ctx, "/tasks/"+taskID.String()+"/status", map[string]any{
			"status": "in_progress",
		})

		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, 1, updateStatusCount, "UpdateStatus must be called exactly once")

		var body domain.Task
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, domain.TaskStatusInProgress, body.Status)
	})

	t.Run("invalid_status", func(t *testing.T) {
		t.Parallel()

		var updateStatusCalled bool
		_, api := humatest.New(t)
		store := &mockDataStore{
			tasks: &mockTaskRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Task, error) {
					return &domain.Task{
						ID: taskID, TenantID: tenantID,
						Status:    domain.TaskStatusBacklog,
						CreatedAt: now, UpdatedAt: now,
					}, nil
				},
				updateStatusFunc: func(_ context.Context, _, _ uuid.UUID, _ domain.TaskStatus) error {
					updateStatusCalled = true
					return nil
				},
			},
		}
		v1.RegisterTaskRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.PatchCtx(ctx, "/tasks/"+taskID.String()+"/status", map[string]any{
			"status": "nonexistent",
		})

		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.False(t, updateStatusCalled, "UpdateStatus must NOT be called for invalid status")

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.Contains(t, errBody["detail"], "unknown task status")
	})

	t.Run("invalid_transition", func(t *testing.T) {
		t.Parallel()

		var updateStatusCalled bool
		_, api := humatest.New(t)
		store := &mockDataStore{
			tasks: &mockTaskRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Task, error) {
					return &domain.Task{
						ID: taskID, TenantID: tenantID,
						Status:    domain.TaskStatusBacklog,
						CreatedAt: now, UpdatedAt: now,
					}, nil
				},
				updateStatusFunc: func(_ context.Context, _, _ uuid.UUID, _ domain.TaskStatus) error {
					updateStatusCalled = true
					return nil
				},
			},
		}
		v1.RegisterTaskRoutes(api, store)

		// backlog -> done is not allowed.
		ctx := tenantCtx(tenantID)
		resp := api.PatchCtx(ctx, "/tasks/"+taskID.String()+"/status", map[string]any{
			"status": "done",
		})

		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.False(t, updateStatusCalled, "UpdateStatus must NOT be called for invalid transition")

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.Contains(t, errBody["detail"], "invalid status transition")
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tasks: &mockTaskRepo{
				getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Task, error) {
					return nil, domain.ErrNotFound
				},
			},
		}
		v1.RegisterTaskRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.PatchCtx(ctx, "/tasks/"+uuid.New().String()+"/status", map[string]any{
			"status": "in_progress",
		})

		assert.Equal(t, http.StatusNotFound, resp.Code)
	})
}

// ---------------------------------------------------------------------------
// TestDeleteTask
// ---------------------------------------------------------------------------

func TestDeleteTask(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	taskID := uuid.New()

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tasks: &mockTaskRepo{
				deleteFunc: func(_ context.Context, tid, id uuid.UUID) error {
					assert.Equal(t, tenantID, tid)
					assert.Equal(t, taskID, id)
					return nil
				},
			},
		}
		v1.RegisterTaskRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.DeleteCtx(ctx, "/tasks/"+taskID.String())

		assert.Equal(t, http.StatusNoContent, resp.Code)
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tasks: &mockTaskRepo{
				deleteFunc: func(_ context.Context, _, _ uuid.UUID) error {
					return domain.ErrNotFound
				},
			},
		}
		v1.RegisterTaskRoutes(api, store)

		ctx := tenantCtx(tenantID)
		resp := api.DeleteCtx(ctx, "/tasks/"+uuid.New().String())

		assert.Equal(t, http.StatusNotFound, resp.Code)

		var errBody map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
		assert.Contains(t, errBody["detail"], "task not found")
	})
}
