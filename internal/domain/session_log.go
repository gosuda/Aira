package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SessionLogEntry records a single message or event from an agent session,
// used for replay and audit.
type SessionLogEntry struct {
	ID        uuid.UUID
	SessionID uuid.UUID
	TenantID  uuid.UUID
	EntryType string // "output", "tool_call", "tool_result", "error", "hitl_question", "hitl_answer"
	Content   string // the actual text/JSON payload
	CreatedAt time.Time
}

// SessionLogRepository stores and retrieves ordered log entries per session.
type SessionLogRepository interface {
	Append(ctx context.Context, entry *SessionLogEntry) error
	ListBySession(ctx context.Context, tenantID, sessionID uuid.UUID, limit, offset int) ([]*SessionLogEntry, error)
	CountBySession(ctx context.Context, tenantID, sessionID uuid.UUID) (int64, error)
}
