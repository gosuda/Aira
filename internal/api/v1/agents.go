package v1

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/gosuda/aira/internal/agent"
	"github.com/gosuda/aira/internal/domain"
	"github.com/gosuda/aira/internal/server/middleware"
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
	Limit     int       `query:"limit" minimum:"1" maximum:"200" default:"50" doc:"Max results"`
	Offset    int       `query:"offset" minimum:"0" default:"0" doc:"Offset for pagination"`
}

type ListAgentSessionsOutput struct {
	Body []*domain.AgentSession
}

func RegisterAgentRoutes(api huma.API, store DataStore, orchestrator AgentOrchestrator) {
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

		session, err := orchestrator.StartTask(ctx, tenantID, input.Body.TaskID, input.Body.AgentType)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, huma.Error404NotFound("task not found")
			}
			if errors.Is(err, agent.ErrUnknownAgent) {
				return nil, huma.Error400BadRequest("unknown agent type: " + input.Body.AgentType)
			}
			if errors.Is(err, agent.ErrInvalidSessionState) {
				return nil, huma.Error400BadRequest("task is not in an eligible state for agent execution")
			}
			return nil, huma.Error500InternalServerError("failed to start agent session", err)
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

		err := orchestrator.CancelSession(ctx, tenantID, input.ID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, huma.Error404NotFound("agent session not found")
			}
			if errors.Is(err, agent.ErrInvalidSessionState) {
				return nil, huma.Error400BadRequest("agent session is already in a terminal state")
			}
			return nil, huma.Error500InternalServerError("failed to cancel agent session", err)
		}

		session, err := store.AgentSessions().GetByID(ctx, tenantID, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to get cancelled session", err)
		}

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

		sessions, err := store.AgentSessions().ListByProjectPaginated(ctx, tenantID, input.ProjectID, input.Limit, input.Offset)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list agent sessions", err)
		}

		return &ListAgentSessionsOutput{Body: sessions}, nil
	})
}
