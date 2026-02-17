package agent

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SessionID uniquely identifies an agent session.
type SessionID = uuid.UUID

// MessageType categorizes agent output messages.
type MessageType string

const (
	MessageTypeOutput     MessageType = "output"
	MessageTypeToolCall   MessageType = "tool_call"
	MessageTypeToolResult MessageType = "tool_result"
	MessageTypeError      MessageType = "error"
	MessageTypeStatus     MessageType = "status"
)

// Message represents an agent output message.
type Message struct {
	Type      MessageType     `json:"type"`
	SessionID SessionID       `json:"session_id"`
	Content   string          `json:"content"`
	Raw       json.RawMessage `json:"raw,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// ToolCall represents a tool invocation by an agent.
type ToolCall struct {
	Name   string          `json:"name"`
	Input  json.RawMessage `json:"input"`
	CallID string          `json:"call_id"`
}

// MessageHandler processes agent messages.
type MessageHandler func(msg Message)

// SessionOptions configures a new agent session.
type SessionOptions struct {
	SessionID   SessionID
	ProjectDir  string            // path to repo inside container
	BranchName  string            // isolated branch
	Prompt      string            // initial task prompt
	Environment map[string]string // extra env vars
	AgentType   string            // "claude", "codex", etc.
}

// AgentBackend is the universal interface for AI agent integrations.
type AgentBackend interface {
	// StartSession initializes an agent session in a Docker container.
	StartSession(ctx context.Context, opts SessionOptions) (SessionID, error)

	// SendPrompt sends a user prompt to an active session (for HITL responses).
	SendPrompt(ctx context.Context, sessionID SessionID, prompt string) error

	// Cancel gracefully stops an agent session.
	Cancel(ctx context.Context, sessionID SessionID) error

	// OnMessage registers a handler for agent output messages.
	OnMessage(handler MessageHandler)

	// Dispose cleans up all resources.
	Dispose(ctx context.Context) error
}
