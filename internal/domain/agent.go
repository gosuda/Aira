package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type AgentSessionStatus string

const (
	AgentStatusPending     AgentSessionStatus = "pending"
	AgentStatusRunning     AgentSessionStatus = "running"
	AgentStatusWaitingHITL AgentSessionStatus = "waiting_hitl"
	AgentStatusCompleted   AgentSessionStatus = "completed"
	AgentStatusFailed      AgentSessionStatus = "failed"
	AgentStatusCancelled   AgentSessionStatus = "cancelled"
)

type AgentSession struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	ProjectID   uuid.UUID
	TaskID      *uuid.UUID
	AgentType   string // "claude", "codex", "opencode", "acp"
	Status      AgentSessionStatus
	ContainerID string
	BranchName  string // aira/<session-id>
	StartedAt   *time.Time
	CompletedAt *time.Time
	Error       string
	Metadata    map[string]any
	CreatedAt   time.Time
}

// GenerateBranchName returns the isolated branch name for this session.
func (s *AgentSession) GenerateBranchName() string {
	return "aira/" + s.ID.String()
}

type AgentSessionRepository interface {
	Create(ctx context.Context, s *AgentSession) error
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*AgentSession, error)
	UpdateStatus(ctx context.Context, tenantID, id uuid.UUID, status AgentSessionStatus) error
	UpdateContainer(ctx context.Context, id uuid.UUID, containerID, branchName string) error
	SetCompleted(ctx context.Context, id uuid.UUID, err string) error
	ListByTask(ctx context.Context, tenantID, taskID uuid.UUID) ([]*AgentSession, error)
	ListByProject(ctx context.Context, tenantID, projectID uuid.UUID) ([]*AgentSession, error)
}
