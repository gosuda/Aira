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
// GET /boards/{projectID}
// ---------------------------------------------------------------------------

func TestGetBoard(t *testing.T) {
	t.Parallel()

	t.Run("happy_path_sorts_into_columns", func(t *testing.T) {
		t.Parallel()

		tid := uuid.New()
		pid := uuid.New()
		now := time.Now().Truncate(time.Second)

		tasks := []*domain.Task{
			{ID: uuid.New(), TenantID: tid, ProjectID: pid, Title: "task-backlog-1", Status: domain.TaskStatusBacklog, CreatedAt: now, UpdatedAt: now},
			{ID: uuid.New(), TenantID: tid, ProjectID: pid, Title: "task-backlog-2", Status: domain.TaskStatusBacklog, CreatedAt: now, UpdatedAt: now},
			{ID: uuid.New(), TenantID: tid, ProjectID: pid, Title: "task-in-progress", Status: domain.TaskStatusInProgress, CreatedAt: now, UpdatedAt: now},
			{ID: uuid.New(), TenantID: tid, ProjectID: pid, Title: "task-review", Status: domain.TaskStatusReview, CreatedAt: now, UpdatedAt: now},
			{ID: uuid.New(), TenantID: tid, ProjectID: pid, Title: "task-done-1", Status: domain.TaskStatusDone, CreatedAt: now, UpdatedAt: now},
			{ID: uuid.New(), TenantID: tid, ProjectID: pid, Title: "task-done-2", Status: domain.TaskStatusDone, CreatedAt: now, UpdatedAt: now},
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

		assert.Len(t, board.Backlog, 2, "backlog should have 2 tasks")
		assert.Len(t, board.InProgress, 1, "in_progress should have 1 task")
		assert.Len(t, board.Review, 1, "review should have 1 task")
		assert.Len(t, board.Done, 2, "done should have 2 tasks")

		assert.Equal(t, "task-backlog-1", board.Backlog[0].Title)
		assert.Equal(t, "task-in-progress", board.InProgress[0].Title)
		assert.Equal(t, "task-review", board.Review[0].Title)
		assert.Equal(t, "task-done-1", board.Done[0].Title)
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

		_, api := humatest.New(t)
		store := &mockDataStore{
			tasks: &mockTaskRepo{},
		}
		v1.RegisterBoardRoutes(api, store)

		resp := api.GetCtx(context.Background(), "/boards/"+pid.String())

		assert.Equal(t, http.StatusForbidden, resp.Code)
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
