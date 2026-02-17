package v1

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/gosuda/aira/internal/domain"
	"github.com/gosuda/aira/internal/server/middleware"
	"github.com/gosuda/aira/internal/store/postgres"
)

type TriggerAgentInput struct {
	Body struct {
		TaskID    uuid.UUID `json:"task_id" doc:"Task ID to run agent for"`
		AgentType string    `json:"agent_type" minLength:"1" maxLength:"50" doc:"Agent type (claude, codex, opencode, acp)"`
	}
}

type TriggerAgentOutput struct {
	Body *domain.AgentSession
}

type GetAgentSessionInput struct {
	ID uuid.UUID `path:"id" doc:"Agent session ID"`
}

type GetAgentSessionOutput struct {
	Body *domain.AgentSession
}

type CancelAgentInput struct {
	ID uuid.UUID `path:"id" doc:"Agent session ID"`
}

type CancelAgentOutput struct {
	Body *domain.AgentSession
}

type ListAgentSessionsInput struct {
	ProjectID uuid.UUID `query:"project_id" required:"true" doc:"Project ID"`
}

type ListAgentSessionsOutput struct {
	Body []*domain.AgentSession
}

func RegisterAgentRoutes(api huma.API, store *postgres.Store) {
	huma.Register(api, huma.Operation{
		OperationID: "trigger-agent",
		Method:      http.MethodPost,
		Path:        "/agents/trigger",
		Summary:     "Trigger an agent session for a task",
		Tags:        []string{"Agents"},
	}, func(ctx context.Context, input *TriggerAgentInput) (*TriggerAgentOutput, error) {
		tenantID, ok := middleware.TenantIDFromContext(ctx)
		if !ok {
			return nil, huma.Error403Forbidden("missing tenant context")
		}

		// Look up the task to get the project ID.
		task, err := store.Tasks().GetByID(ctx, tenantID, input.Body.TaskID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, huma.Error404NotFound("task not found")
			}
			return nil, huma.Error500InternalServerError("failed to look up task", err)
		}

		now := time.Now()
		session := &domain.AgentSession{
			ID:        uuid.New(),
			TenantID:  tenantID,
			ProjectID: task.ProjectID,
			TaskID:    &input.Body.TaskID,
			AgentType: input.Body.AgentType,
			Status:    domain.AgentStatusPending,
			CreatedAt: now,
		}
		session.BranchName = session.GenerateBranchName()

		err = store.AgentSessions().Create(ctx, session)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to create agent session", err)
		}

		return &TriggerAgentOutput{Body: session}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-agent-session",
		Method:      http.MethodGet,
		Path:        "/agents/{id}",
		Summary:     "Get an agent session by ID",
		Tags:        []string{"Agents"},
	}, func(ctx context.Context, input *GetAgentSessionInput) (*GetAgentSessionOutput, error) {
		tenantID, ok := middleware.TenantIDFromContext(ctx)
		if !ok {
			return nil, huma.Error403Forbidden("missing tenant context")
		}

		session, err := store.AgentSessions().GetByID(ctx, tenantID, input.ID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, huma.Error404NotFound("agent session not found")
			}
			return nil, huma.Error500InternalServerError("failed to get agent session", err)
		}

		return &GetAgentSessionOutput{Body: session}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "cancel-agent-session",
		Method:      http.MethodPost,
		Path:        "/agents/{id}/cancel",
		Summary:     "Cancel an agent session",
		Tags:        []string{"Agents"},
	}, func(ctx context.Context, input *CancelAgentInput) (*CancelAgentOutput, error) {
		tenantID, ok := middleware.TenantIDFromContext(ctx)
		if !ok {
			return nil, huma.Error403Forbidden("missing tenant context")
		}

		session, err := store.AgentSessions().GetByID(ctx, tenantID, input.ID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, huma.Error404NotFound("agent session not found")
			}
			return nil, huma.Error500InternalServerError("failed to get agent session", err)
		}

		if session.Status == domain.AgentStatusCompleted ||
			session.Status == domain.AgentStatusFailed ||
			session.Status == domain.AgentStatusCancelled {
			return nil, huma.Error400BadRequest("agent session is already in terminal state: " + string(session.Status))
		}

		err = store.AgentSessions().UpdateStatus(ctx, tenantID, input.ID, domain.AgentStatusCancelled)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to cancel agent session", err)
		}

		session.Status = domain.AgentStatusCancelled

		return &CancelAgentOutput{Body: session}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-agent-sessions",
		Method:      http.MethodGet,
		Path:        "/agents",
		Summary:     "List agent sessions for a project",
		Tags:        []string{"Agents"},
	}, func(ctx context.Context, input *ListAgentSessionsInput) (*ListAgentSessionsOutput, error) {
		tenantID, ok := middleware.TenantIDFromContext(ctx)
		if !ok {
			return nil, huma.Error403Forbidden("missing tenant context")
		}

		sessions, err := store.AgentSessions().ListByProject(ctx, tenantID, input.ProjectID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list agent sessions", err)
		}

		return &ListAgentSessionsOutput{Body: sessions}, nil
	})
}
