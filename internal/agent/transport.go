package agent

import (
	"encoding/json"
	"time"
)

// TransportHandler adapts per-agent protocol quirks.
type TransportHandler interface {
	// AgentName returns the agent framework name.
	AgentName() string

	// InitTimeout returns how long to wait for agent initialization.
	InitTimeout() time.Duration

	// IdleTimeout returns how long to wait before considering agent idle.
	IdleTimeout() time.Duration

	// FilterOutput filters/transforms agent output lines.
	// Returns the filtered line and whether to keep it.
	FilterOutput(line string) (string, bool)

	// ParseToolCall parses a raw tool call from agent output.
	ParseToolCall(raw json.RawMessage) (ToolCall, error)
}
