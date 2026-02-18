package v1

import (
	"context"

	"github.com/google/uuid"

	"github.com/gosuda/aira/internal/domain"
)

// DataStore abstracts the repository accessor pattern for handler testing.
// *postgres.Store satisfies this interface.
type DataStore interface {
	Tenants() domain.TenantRepository
	Projects() domain.ProjectRepository
	Tasks() domain.TaskRepository
	ADRs() domain.ADRRepository
	AgentSessions() domain.AgentSessionRepository
}

// AuthService abstracts authentication operations for handler testing.
// *auth.Service satisfies this interface.
type AuthService interface {
	Register(ctx context.Context, tenantID uuid.UUID, email, password, name string) (*domain.User, error)
	Login(ctx context.Context, tenantID uuid.UUID, email, password string) (accessToken, refreshToken string, err error)
	RefreshToken(ctx context.Context, refreshToken string) (string, error)
}

// AgentOrchestrator abstracts agent lifecycle operations for handler testing.
// *agent.Orchestrator satisfies this interface.
type AgentOrchestrator interface {
	StartTask(ctx context.Context, tenantID, taskID uuid.UUID, agentType string) (*domain.AgentSession, error)
	CancelSession(ctx context.Context, tenantID, sessionID uuid.UUID) error
}
