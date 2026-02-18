package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/gosuda/aira/internal/domain"
)

func TestSessionLogEntry_Fields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	id := uuid.New()
	sessionID := uuid.New()
	tenantID := uuid.New()

	entry := domain.SessionLogEntry{
		ID:        id,
		SessionID: sessionID,
		TenantID:  tenantID,
		EntryType: "output",
		Content:   `{"text":"hello"}`,
		CreatedAt: now,
	}

	assert.Equal(t, id, entry.ID)
	assert.Equal(t, sessionID, entry.SessionID)
	assert.Equal(t, tenantID, entry.TenantID)
	assert.Equal(t, "output", entry.EntryType)
	assert.JSONEq(t, `{"text":"hello"}`, entry.Content)
	assert.Equal(t, now, entry.CreatedAt)
}

func TestSessionLogEntry_ZeroValue(t *testing.T) {
	t.Parallel()

	var entry domain.SessionLogEntry

	assert.Equal(t, uuid.Nil, entry.ID)
	assert.Equal(t, uuid.Nil, entry.SessionID)
	assert.Equal(t, uuid.Nil, entry.TenantID)
	assert.Empty(t, entry.EntryType)
	assert.Empty(t, entry.Content)
	assert.True(t, entry.CreatedAt.IsZero())
}

func TestSessionLogEntry_EntryTypes(t *testing.T) {
	t.Parallel()

	validTypes := []string{"output", "tool_call", "tool_result", "error", "hitl_question", "hitl_answer"}

	for _, entryType := range validTypes {
		t.Run(entryType, func(t *testing.T) {
			t.Parallel()

			entry := domain.SessionLogEntry{
				EntryType: entryType,
				Content:   "test content",
			}

			assert.Equal(t, entryType, entry.EntryType)
			assert.NotEmpty(t, entry.Content)
		})
	}
}

// Compile-time interface satisfaction check.
var _ domain.SessionLogRepository = (*sessionLogRepoStub)(nil)

type sessionLogRepoStub struct{}

func (s *sessionLogRepoStub) Append(_ context.Context, _ *domain.SessionLogEntry) error { return nil }
func (s *sessionLogRepoStub) ListBySession(_ context.Context, _, _ uuid.UUID, _, _ int) ([]*domain.SessionLogEntry, error) {
	return nil, nil
}
func (s *sessionLogRepoStub) CountBySession(_ context.Context, _, _ uuid.UUID) (int64, error) {
	return 0, nil
}
