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

// collectIDs extracts IDs from a slice of tasks for order-independent comparison.
func collectIDs(tasks []domain.Task) []uuid.UUID {
	ids := make([]uuid.UUID, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	return ids
}

// ---------------------------------------------------------------------------
// GET /boards/{projectID}
// ---------------------------------------------------------------------------

func TestGetBoard(t *testing.T) {
	t.Parallel()

	t.Run("happy_path_sorts_into_columns", func(t *testing.T) {
		t.Parallel()

		tid := uuid.New()
		pid := uuid.New()
		now := time.Now().Truncate(time.Second)

		backlog1 := uuid.New()
		backlog2 := uuid.New()
		inProg := uuid.New()
		review := uuid.New()
		done1 := uuid.New()
		done2 := uuid.New()

		tasks := []*domain.Task{
			{ID: backlog1, TenantID: tid, ProjectID: pid, Title: "task-backlog-1", Status: domain.TaskStatusBacklog, CreatedAt: now, UpdatedAt: now},
			{ID: backlog2, TenantID: tid, ProjectID: pid, Title: "task-backlog-2", Status: domain.TaskStatusBacklog, CreatedAt: now, UpdatedAt: now},
			{ID: inProg, TenantID: tid, ProjectID: pid, Title: "task-in-progress", Status: domain.TaskStatusInProgress, CreatedAt: now, UpdatedAt: now},
			{ID: review, TenantID: tid, ProjectID: pid, Title: "task-review", Status: domain.TaskStatusReview, CreatedAt: now, UpdatedAt: now},
			{ID: done1, TenantID: tid, ProjectID: pid, Title: "task-done-1", Status: domain.TaskStatusDone, CreatedAt: now, UpdatedAt: now},
			{ID: done2, TenantID: tid, ProjectID: pid, Title: "task-done-2", Status: domain.TaskStatusDone, CreatedAt: now, UpdatedAt: now},
		}

		_, api := humatest.New(t)
		store := &mockDataStore{
			tasks: &mockTaskRepo{
				listByProjectFunc: func(_ context.Context, tenantID, projectID uuid.UUID) ([]*domain.Task, error) {
					assert.Equal(t, tid, tenantID)
					assert.Equal(t, pid, projectID)
					return tasks, nil
				},
			},
		}
		v1.RegisterBoardRoutes(api, store)

		resp := api.GetCtx(tenantCtx(tid), "/boards/"+pid.String())

		require.Equal(t, http.StatusOK, resp.Code)

		var board struct {
			Backlog    []domain.Task `json:"backlog"`
			InProgress []domain.Task `json:"in_progress"`
			Review     []domain.Task `json:"review"`
			Done       []domain.Task `json:"done"`
		}
		err := json.Unmarshal(resp.Body.Bytes(), &board)
		require.NoError(t, err)

		// Assert column sizes.
		assert.Len(t, board.Backlog, 2, "backlog should have 2 tasks")
		assert.Len(t, board.InProgress, 1, "in_progress should have 1 task")
		assert.Len(t, board.Review, 1, "review should have 1 task")
		assert.Len(t, board.Done, 2, "done should have 2 tasks")

		// Assert by IDs (order-independent within columns).
		assert.ElementsMatch(t, []uuid.UUID{backlog1, backlog2}, collectIDs(board.Backlog))
		assert.ElementsMatch(t, []uuid.UUID{inProg}, collectIDs(board.InProgress))
		assert.ElementsMatch(t, []uuid.UUID{review}, collectIDs(board.Review))
		assert.ElementsMatch(t, []uuid.UUID{done1, done2}, collectIDs(board.Done))
	})

	t.Run("unknown_status_silently_dropped", func(t *testing.T) {
		t.Parallel()

		tid := uuid.New()
		pid := uuid.New()
		now := time.Now().Truncate(time.Second)

		knownID := uuid.New()
		unknownID := uuid.New()

		tasks := []*domain.Task{
			{ID: knownID, TenantID: tid, ProjectID: pid, Title: "known", Status: domain.TaskStatusBacklog, CreatedAt: now, UpdatedAt: now},
			{ID: unknownID, TenantID: tid, ProjectID: pid, Title: "unknown-status", Status: domain.TaskStatus("archived"), CreatedAt: now, UpdatedAt: now},
		}

		_, api := humatest.New(t)
		store := &mockDataStore{
			tasks: &mockTaskRepo{
				listByProjectFunc: func(_ context.Context, _, _ uuid.UUID) ([]*domain.Task, error) {
					return tasks, nil
				},
			},
		}
		v1.RegisterBoardRoutes(api, store)

		resp := api.GetCtx(tenantCtx(tid), "/boards/"+pid.String())

		require.Equal(t, http.StatusOK, resp.Code)

		var board struct {
			Backlog    []domain.Task `json:"backlog"`
			InProgress []domain.Task `json:"in_progress"`
			Review     []domain.Task `json:"review"`
			Done       []domain.Task `json:"done"`
		}
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &board))

		// The known task appears in backlog; the unknown-status task is dropped.
		assert.Len(t, board.Backlog, 1)
		assert.Equal(t, knownID, board.Backlog[0].ID)
		assert.Empty(t, board.InProgress)
		assert.Empty(t, board.Review)
		assert.Empty(t, board.Done)
	})

	t.Run("empty_board", func(t *testing.T) {
		t.Parallel()

		tid := uuid.New()
		pid := uuid.New()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tasks: &mockTaskRepo{
				listByProjectFunc: func(_ context.Context, _, _ uuid.UUID) ([]*domain.Task, error) {
					return []*domain.Task{}, nil
				},
			},
		}
		v1.RegisterBoardRoutes(api, store)

		resp := api.GetCtx(tenantCtx(tid), "/boards/"+pid.String())

		require.Equal(t, http.StatusOK, resp.Code)

		var board struct {
			Backlog    []domain.Task `json:"backlog"`
			InProgress []domain.Task `json:"in_progress"`
			Review     []domain.Task `json:"review"`
			Done       []domain.Task `json:"done"`
		}
		err := json.Unmarshal(resp.Body.Bytes(), &board)
		require.NoError(t, err)

		assert.Empty(t, board.Backlog)
		assert.Empty(t, board.InProgress)
		assert.Empty(t, board.Review)
		assert.Empty(t, board.Done)
	})

	t.Run("missing_tenant_context", func(t *testing.T) {
		t.Parallel()

		pid := uuid.New()

		var storeCalled bool
		_, api := humatest.New(t)
		store := &mockDataStore{
			tasks: &mockTaskRepo{
				listByProjectFunc: func(_ context.Context, _, _ uuid.UUID) ([]*domain.Task, error) {
					storeCalled = true
					return nil, nil
				},
			},
		}
		v1.RegisterBoardRoutes(api, store)

		resp := api.GetCtx(context.Background(), "/boards/"+pid.String())

		assert.Equal(t, http.StatusForbidden, resp.Code)
		assert.False(t, storeCalled, "store must NOT be accessed without tenant context")
	})

	t.Run("invalid_project_id", func(t *testing.T) {
		t.Parallel()

		tid := uuid.New()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tasks: &mockTaskRepo{},
		}
		v1.RegisterBoardRoutes(api, store)

		resp := api.GetCtx(tenantCtx(tid), "/boards/not-a-uuid")

		// Huma returns 422 for unparseable path parameters.
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("store_error", func(t *testing.T) {
		t.Parallel()

		tid := uuid.New()
		pid := uuid.New()

		_, api := humatest.New(t)
		store := &mockDataStore{
			tasks: &mockTaskRepo{
				listByProjectFunc: func(_ context.Context, _, _ uuid.UUID) ([]*domain.Task, error) {
					return nil, errors.New("db: connection lost")
				},
			},
		}
		v1.RegisterBoardRoutes(api, store)

		resp := api.GetCtx(tenantCtx(tid), "/boards/"+pid.String())

		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})
}
