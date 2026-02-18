package domain_test

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gosuda/aira/internal/domain"
)

// ---------------------------------------------------------------------------
// 1. TaskStatus.ValidTransition — full 4x4 state-machine matrix.
// ---------------------------------------------------------------------------

func TestTaskStatus_ValidTransition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		from domain.TaskStatus
		to   domain.TaskStatus
		want bool
	}{
		// From backlog.
		{domain.TaskStatusBacklog, domain.TaskStatusInProgress, true},
		{domain.TaskStatusBacklog, domain.TaskStatusReview, false},
		{domain.TaskStatusBacklog, domain.TaskStatusDone, false},
		{domain.TaskStatusBacklog, domain.TaskStatusBacklog, false},

		// From in_progress.
		{domain.TaskStatusInProgress, domain.TaskStatusReview, true},
		{domain.TaskStatusInProgress, domain.TaskStatusDone, false},
		{domain.TaskStatusInProgress, domain.TaskStatusBacklog, false},
		{domain.TaskStatusInProgress, domain.TaskStatusInProgress, false},

		// From review.
		{domain.TaskStatusReview, domain.TaskStatusDone, true},
		{domain.TaskStatusReview, domain.TaskStatusInProgress, true}, // rework
		{domain.TaskStatusReview, domain.TaskStatusBacklog, false},
		{domain.TaskStatusReview, domain.TaskStatusReview, false},

		// From done (terminal).
		{domain.TaskStatusDone, domain.TaskStatusBacklog, false},
		{domain.TaskStatusDone, domain.TaskStatusInProgress, false},
		{domain.TaskStatusDone, domain.TaskStatusReview, false},
		{domain.TaskStatusDone, domain.TaskStatusDone, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			t.Parallel()

			got := tt.from.ValidTransition(tt.to)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestTaskStatus_ValidTransition_UnknownStatus verifies that an unrecognised
// status always returns false regardless of destination.
func TestTaskStatus_ValidTransition_UnknownStatus(t *testing.T) {
	t.Parallel()

	unknown := domain.TaskStatus("archived")
	targets := []domain.TaskStatus{
		domain.TaskStatusBacklog,
		domain.TaskStatusInProgress,
		domain.TaskStatusReview,
		domain.TaskStatusDone,
	}

	for _, to := range targets {
		t.Run("archived->"+string(to), func(t *testing.T) {
			t.Parallel()

			assert.False(t, unknown.ValidTransition(to))
		})
	}
}

// TestTaskStatus_ValidTransition_UnknownTarget verifies that transitioning to
// an unrecognised status always returns false.
func TestTaskStatus_ValidTransition_UnknownTarget(t *testing.T) {
	t.Parallel()

	unknown := domain.TaskStatus("archived")
	sources := []domain.TaskStatus{
		domain.TaskStatusBacklog,
		domain.TaskStatusInProgress,
		domain.TaskStatusReview,
		domain.TaskStatusDone,
	}

	for _, from := range sources {
		t.Run(string(from)+"->archived", func(t *testing.T) {
			t.Parallel()

			assert.False(t, from.ValidTransition(unknown))
		})
	}
}

// ---------------------------------------------------------------------------
// 2. AgentSession.GenerateBranchName.
// ---------------------------------------------------------------------------

func TestAgentSession_GenerateBranchName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   uuid.UUID
		want string
	}{
		{
			name: "normal uuid",
			id:   uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
			want: "aira/550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name: "zero uuid",
			id:   uuid.UUID{},
			want: "aira/00000000-0000-0000-0000-000000000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			session := &domain.AgentSession{ID: tt.id}
			got := session.GenerateBranchName()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestAgentSession_GenerateBranchName_Prefix ensures the branch always starts
// with the "aira/" prefix.
func TestAgentSession_GenerateBranchName_Prefix(t *testing.T) {
	t.Parallel()

	session := &domain.AgentSession{ID: uuid.New()}
	got := session.GenerateBranchName()
	assert.Contains(t, got, "aira/")
	assert.Equal(t, "aira/", got[:5])
}

// ---------------------------------------------------------------------------
// 3. Sentinel errors — identity, distinctness, and wrapping.
// ---------------------------------------------------------------------------

func TestSentinelErrors_Identity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{"ErrNotFound", domain.ErrNotFound},
		{"ErrConflict", domain.ErrConflict},
		{"ErrUnauthorized", domain.ErrUnauthorized},
		{"ErrForbidden", domain.ErrForbidden},
		{"ErrInvalidTransition", domain.ErrInvalidTransition},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Error(t, tt.err, "sentinel error should not be nil")
			assert.NotEmpty(t, tt.err.Error(), "error message should not be empty")
		})
	}
}

func TestSentinelErrors_Distinct(t *testing.T) {
	t.Parallel()

	sentinels := []error{
		domain.ErrNotFound,
		domain.ErrConflict,
		domain.ErrUnauthorized,
		domain.ErrForbidden,
		domain.ErrInvalidTransition,
	}

	for i, a := range sentinels {
		for j, b := range sentinels {
			if i == j {
				continue
			}

			t.Run(a.Error()+"!="+b.Error(), func(t *testing.T) {
				t.Parallel()

				assert.NotErrorIs(t, a, b, "sentinel errors must be distinct")
			})
		}
	}
}

func TestSentinelErrors_WrappingPreservesIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{"ErrNotFound", domain.ErrNotFound},
		{"ErrConflict", domain.ErrConflict},
		{"ErrUnauthorized", domain.ErrUnauthorized},
		{"ErrForbidden", domain.ErrForbidden},
		{"ErrInvalidTransition", domain.ErrInvalidTransition},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wrapped := fmt.Errorf("outer: %w", tt.err)
			require.ErrorIs(t, wrapped, tt.err, "wrapped error should preserve identity")

			doubleWrapped := fmt.Errorf("outer2: %w", wrapped)
			require.ErrorIs(t, doubleWrapped, tt.err, "double-wrapped error should preserve identity")
		})
	}
}

// ---------------------------------------------------------------------------
// 4. Status constants — string value regression guards.
// ---------------------------------------------------------------------------

func TestTaskStatusConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  domain.TaskStatus
		want string
	}{
		{"backlog", domain.TaskStatusBacklog, "backlog"},
		{"in_progress", domain.TaskStatusInProgress, "in_progress"},
		{"review", domain.TaskStatusReview, "review"},
		{"done", domain.TaskStatusDone, "done"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, string(tt.got))
		})
	}
}

func TestAgentSessionStatusConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  domain.AgentSessionStatus
		want string
	}{
		{"pending", domain.AgentStatusPending, "pending"},
		{"running", domain.AgentStatusRunning, "running"},
		{"waiting_hitl", domain.AgentStatusWaitingHITL, "waiting_hitl"},
		{"completed", domain.AgentStatusCompleted, "completed"},
		{"failed", domain.AgentStatusFailed, "failed"},
		{"cancelled", domain.AgentStatusCancelled, "cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, string(tt.got))
		})
	}
}

func TestHITLStatusConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  domain.HITLStatus
		want string
	}{
		{"pending", domain.HITLStatusPending, "pending"},
		{"answered", domain.HITLStatusAnswered, "answered"},
		{"timeout", domain.HITLStatusTimeout, "timeout"},
		{"cancelled", domain.HITLStatusCancelled, "cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, string(tt.got))
		})
	}
}

func TestADRStatusConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  domain.ADRStatus
		want string
	}{
		{"draft", domain.ADRStatusDraft, "draft"},
		{"proposed", domain.ADRStatusProposed, "proposed"},
		{"accepted", domain.ADRStatusAccepted, "accepted"},
		{"rejected", domain.ADRStatusRejected, "rejected"},
		{"deprecated", domain.ADRStatusDeprecated, "deprecated"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, string(tt.got))
		})
	}
}
