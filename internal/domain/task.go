package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type TaskStatus string

const (
	TaskStatusBacklog    TaskStatus = "backlog"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusReview     TaskStatus = "review"
	TaskStatusDone       TaskStatus = "done"
)

// ValidTransition checks if a task state transition is allowed.
// Allowed: backlog->in_progress, in_progress->review, review->done, review->in_progress (rework).
func (s TaskStatus) ValidTransition(to TaskStatus) bool {
	switch s {
	case TaskStatusBacklog:
		return to == TaskStatusInProgress
	case TaskStatusInProgress:
		return to == TaskStatusReview
	case TaskStatusReview:
		return to == TaskStatusDone || to == TaskStatusInProgress
	default:
		return false
	}
}

type Task struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	ProjectID      uuid.UUID
	ADRID          *uuid.UUID // nullable, parent ADR
	Title          string
	Description    string
	Status         TaskStatus
	Priority       int
	AssignedTo     *uuid.UUID
	AgentSessionID *uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

var ErrInvalidTransition = errors.New("task: invalid state transition")

type TaskRepository interface {
	Create(ctx context.Context, t *Task) error
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*Task, error)
	ListByProject(ctx context.Context, tenantID, projectID uuid.UUID) ([]*Task, error)
	ListByStatus(ctx context.Context, tenantID, projectID uuid.UUID, status TaskStatus) ([]*Task, error)
	UpdateStatus(ctx context.Context, tenantID, id uuid.UUID, status TaskStatus) error
	Update(ctx context.Context, t *Task) error
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}
