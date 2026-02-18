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

	"github.com/gosuda/aira/internal/agent"
	v1 "github.com/gosuda/aira/internal/api/v1"
	"github.com/gosuda/aira/internal/domain"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newAgentTestAPI(t *testing.T) (humatest.TestAPI, *mockDataStore, *mockAgentOrchestrator) {
	t.Helper()

	_, api := humatest.New(t)
	store := &mockDataStore{}
	orch := &mockAgentOrchestrator{}

	v1.RegisterAgentRoutes(api, store, orch)

	return api, store, orch
}

func makeSession(tenantID, projectID uuid.UUID) *domain.AgentSession {
	now := time.Now()
	taskID := uuid.New()
	return &domain.AgentSession{
		ID:        uuid.New(),
		TenantID:  tenantID,
		ProjectID: projectID,
		TaskID:    &taskID,
		AgentType: "claude",
		Status:    domain.AgentStatusRunning,
		StartedAt: &now,
		CreatedAt: now,
	}
}

// parseErrorBody decodes the RFC 9457 problem detail from the response body.
func parseErrorBody(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	return body
}

// ---------------------------------------------------------------------------
// POST /agents/trigger
// ---------------------------------------------------------------------------

func TestTriggerAgent(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	projectID := uuid.New()

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		api, _, orch := newAgentTestAPI(t)
		session := makeSession(tenantID, projectID)

		orch.startTaskFunc = func(_ context.Context, tid, taskID uuid.UUID, agentType string) (*domain.AgentSession, error) {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, *session.TaskID, taskID)
			assert.Equal(t, "claude", agentType)
			return session, nil
		}

		ctx := tenantCtx(tenantID)
		resp := api.PostCtx(ctx, "/agents/trigger", map[string]any{
			"task_id":    session.TaskID.String(),
			"agent_type": "claude",
		})

		require.Equal(t, http.StatusOK, resp.Code)

		var body domain.AgentSession
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
		assert.Equal(t, session.ID, body.ID)
		assert.Equal(t, session.AgentType, body.AgentType)
		assert.Equal(t, domain.AgentStatusRunning, body.Status)
	})

	t.Run("missing_tenant", func(t *testing.T) {
		t.Parallel()

		api, _, _ := newAgentTestAPI(t)

		// Use bare context -- no tenant injected.
		resp := api.PostCtx(context.Background(), "/agents/trigger", map[string]any{
			"task_id":    uuid.New().String(),
			"agent_type": "claude",
		})

		assert.Equal(t, http.StatusForbidden, resp.Code)
		body := parseErrorBody(t, resp.Body.Bytes())
		assert.Contains(t, body["detail"], "missing tenant context")
	})

	t.Run("task_not_found", func(t *testing.T) {
		t.Parallel()

		api, _, orch := newAgentTestAPI(t)

		orch.startTaskFunc = func(_ context.Context, _, _ uuid.UUID, _ string) (*domain.AgentSession, error) {
			return nil, fmt.Errorf("orchestrator.StartTask: %w", domain.ErrNotFound)
		}

		ctx := tenantCtx(tenantID)
		resp := api.PostCtx(ctx, "/agents/trigger", map[string]any{
			"task_id":    uuid.New().String(),
			"agent_type": "claude",
		})

		assert.Equal(t, http.StatusNotFound, resp.Code)
		body := parseErrorBody(t, resp.Body.Bytes())
		assert.Contains(t, body["detail"], "task not found")
	})

	t.Run("unknown_agent_type", func(t *testing.T) {
		t.Parallel()

		api, _, orch := newAgentTestAPI(t)

		orch.startTaskFunc = func(_ context.Context, _, _ uuid.UUID, _ string) (*domain.AgentSession, error) {
			return nil, fmt.Errorf("orchestrator.StartTask: %w", agent.ErrUnknownAgent)
		}

		ctx := tenantCtx(tenantID)
		resp := api.PostCtx(ctx, "/agents/trigger", map[string]any{
			"task_id":    uuid.New().String(),
			"agent_type": "bogus",
		})

		assert.Equal(t, http.StatusBadRequest, resp.Code)
		body := parseErrorBody(t, resp.Body.Bytes())
		assert.Contains(t, body["detail"], "unknown agent type")
	})

	t.Run("invalid_session_state", func(t *testing.T) {
		t.Parallel()

		api, _, orch := newAgentTestAPI(t)

		orch.startTaskFunc = func(_ context.Context, _, _ uuid.UUID, _ string) (*domain.AgentSession, error) {
			return nil, fmt.Errorf("orchestrator.StartTask: %w", agent.ErrInvalidSessionState)
		}

		ctx := tenantCtx(tenantID)
		resp := api.PostCtx(ctx, "/agents/trigger", map[string]any{
			"task_id":    uuid.New().String(),
			"agent_type": "claude",
		})

		assert.Equal(t, http.StatusBadRequest, resp.Code)
		body := parseErrorBody(t, resp.Body.Bytes())
		assert.Contains(t, body["detail"], "not in an eligible state")
	})

	t.Run("orchestrator_error", func(t *testing.T) {
		t.Parallel()

		api, _, orch := newAgentTestAPI(t)

		orch.startTaskFunc = func(_ context.Context, _, _ uuid.UUID, _ string) (*domain.AgentSession, error) {
			return nil, errors.New("docker daemon unreachable")
		}

		ctx := tenantCtx(tenantID)
		resp := api.PostCtx(ctx, "/agents/trigger", map[string]any{
			"task_id":    uuid.New().String(),
			"agent_type": "claude",
		})

		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})
}

// ---------------------------------------------------------------------------
// GET /agents/{id}
// ---------------------------------------------------------------------------

func TestGetAgentSession(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	projectID := uuid.New()

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		api, store, _ := newAgentTestAPI(t)
		session := makeSession(tenantID, projectID)

		store.agentSessions = &mockAgentSessionRepo{
			getByIDFunc: func(_ context.Context, tid, id uuid.UUID) (*domain.AgentSession, error) {
				assert.Equal(t, tenantID, tid)
				assert.Equal(t, session.ID, id)
				return session, nil
			},
		}

		ctx := tenantCtx(tenantID)
		resp := api.GetCtx(ctx, "/agents/"+session.ID.String())

		require.Equal(t, http.StatusOK, resp.Code)

		var body domain.AgentSession
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
		assert.Equal(t, session.ID, body.ID)
		assert.Equal(t, session.AgentType, body.AgentType)
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		api, store, _ := newAgentTestAPI(t)
		missingID := uuid.New()

		store.agentSessions = &mockAgentSessionRepo{
			getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.AgentSession, error) {
				return nil, fmt.Errorf("repo.GetByID: %w", domain.ErrNotFound)
			},
		}

		ctx := tenantCtx(tenantID)
		resp := api.GetCtx(ctx, "/agents/"+missingID.String())

		assert.Equal(t, http.StatusNotFound, resp.Code)
		body := parseErrorBody(t, resp.Body.Bytes())
		assert.Contains(t, body["detail"], "agent session not found")
	})
}

// ---------------------------------------------------------------------------
// POST /agents/{id}/cancel
// ---------------------------------------------------------------------------

func TestCancelAgent(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	projectID := uuid.New()

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		api, store, orch := newAgentTestAPI(t)
		session := makeSession(tenantID, projectID)
		session.Status = domain.AgentStatusCancelled

		orch.cancelSessionFunc = func(_ context.Context, tid, sid uuid.UUID) error {
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, session.ID, sid)
			return nil
		}

		// After cancel succeeds, the handler fetches the session to return it.
		store.agentSessions = &mockAgentSessionRepo{
			getByIDFunc: func(_ context.Context, tid, id uuid.UUID) (*domain.AgentSession, error) {
				assert.Equal(t, tenantID, tid)
				assert.Equal(t, session.ID, id)
				return session, nil
			},
		}

		ctx := tenantCtx(tenantID)
		resp := api.PostCtx(ctx, "/agents/"+session.ID.String()+"/cancel", map[string]any{})

		require.Equal(t, http.StatusOK, resp.Code)

		var body domain.AgentSession
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
		assert.Equal(t, session.ID, body.ID)
		assert.Equal(t, domain.AgentStatusCancelled, body.Status)
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		api, _, orch := newAgentTestAPI(t)
		missingID := uuid.New()

		orch.cancelSessionFunc = func(_ context.Context, _, _ uuid.UUID) error {
			return fmt.Errorf("orchestrator.CancelSession: %w", domain.ErrNotFound)
		}

		ctx := tenantCtx(tenantID)
		resp := api.PostCtx(ctx, "/agents/"+missingID.String()+"/cancel", map[string]any{})

		assert.Equal(t, http.StatusNotFound, resp.Code)
		body := parseErrorBody(t, resp.Body.Bytes())
		assert.Contains(t, body["detail"], "agent session not found")
	})

	t.Run("already_terminal", func(t *testing.T) {
		t.Parallel()

		api, _, orch := newAgentTestAPI(t)
		sessionID := uuid.New()

		orch.cancelSessionFunc = func(_ context.Context, _, _ uuid.UUID) error {
			return fmt.Errorf("orchestrator.CancelSession: %w", agent.ErrInvalidSessionState)
		}

		ctx := tenantCtx(tenantID)
		resp := api.PostCtx(ctx, "/agents/"+sessionID.String()+"/cancel", map[string]any{})

		assert.Equal(t, http.StatusBadRequest, resp.Code)
		body := parseErrorBody(t, resp.Body.Bytes())
		assert.Contains(t, body["detail"], "already in a terminal state")
	})
}

// ---------------------------------------------------------------------------
// GET /agents?project_id=X
// ---------------------------------------------------------------------------

func TestListAgentSessions(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	projectID := uuid.New()

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		api, store, _ := newAgentTestAPI(t)
		s1 := makeSession(tenantID, projectID)
		s2 := makeSession(tenantID, projectID)

		store.agentSessions = &mockAgentSessionRepo{
			listByProjectFunc: func(_ context.Context, tid, pid uuid.UUID) ([]*domain.AgentSession, error) {
				assert.Equal(t, tenantID, tid)
				assert.Equal(t, projectID, pid)
				return []*domain.AgentSession{s1, s2}, nil
			},
		}

		ctx := tenantCtx(tenantID)
		resp := api.GetCtx(ctx, "/agents?project_id="+projectID.String())

		require.Equal(t, http.StatusOK, resp.Code)

		var body []*domain.AgentSession
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
		assert.Len(t, body, 2)
		assert.Equal(t, s1.ID, body[0].ID)
		assert.Equal(t, s2.ID, body[1].ID)
	})

	t.Run("store_error", func(t *testing.T) {
		t.Parallel()

		api, store, _ := newAgentTestAPI(t)

		store.agentSessions = &mockAgentSessionRepo{
			listByProjectFunc: func(_ context.Context, _, _ uuid.UUID) ([]*domain.AgentSession, error) {
				return nil, errors.New("connection refused")
			},
		}

		ctx := tenantCtx(tenantID)
		resp := api.GetCtx(ctx, "/agents?project_id="+projectID.String())

		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})
}
