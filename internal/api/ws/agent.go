package ws

import (
	"time"

	"github.com/google/uuid"
)

// AgentEvent represents a real-time agent session update.
type AgentEvent struct {
	Type      string    `json:"type"` // "output", "status_change", "tool_call", "error"
	SessionID uuid.UUID `json:"session_id"`
	Data      any       `json:"data,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
