package ws

import "github.com/google/uuid"

// BoardEvent represents a real-time kanban board update.
type BoardEvent struct {
	Type      string    `json:"type"` // "task_created", "task_updated", "task_moved", "task_deleted"
	TaskID    uuid.UUID `json:"task_id"`
	ProjectID uuid.UUID `json:"project_id"`
	Data      any       `json:"data,omitempty"`
}
