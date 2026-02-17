package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type AuditEntry struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	ActorType  string // "user", "agent", "system"
	ActorID    string
	Action     string
	Resource   string // "task", "adr", "agent_session", etc.
	ResourceID uuid.UUID
	Details    map[string]any
	CreatedAt  time.Time
}

type AuditRepository interface {
	Record(ctx context.Context, entry *AuditEntry) error
	ListByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*AuditEntry, error)
	ListByResource(ctx context.Context, tenantID uuid.UUID, resource string, resourceID uuid.UUID) ([]*AuditEntry, error)
}
