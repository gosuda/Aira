package backends_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gosuda/aira/internal/agent"
	"github.com/gosuda/aira/internal/agent/backends"
)

// ---------------------------------------------------------------------------
// Claude Transport tests
// ---------------------------------------------------------------------------

func TestClaudeTransport_AgentName(t *testing.T) {
	t.Parallel()

	transport := &backends.ClaudeTransport{}
	assert.Equal(t, "claude", transport.AgentName())
}

func TestClaudeTransport_Timeouts(t *testing.T) {
	t.Parallel()

	transport := &backends.ClaudeTransport{}
	assert.Equal(t, 30*time.Second, transport.InitTimeout())
	assert.Equal(t, 5*time.Minute, transport.IdleTimeout())
}

func TestClaudeTransport_FilterOutput(t *testing.T) {
	t.Parallel()

	transport := &backends.ClaudeTransport{}

	tests := []struct {
		name     string
		input    string
		wantOut  string
		wantKeep bool
	}{
		{
			name:     "empty line filtered",
			input:    "",
			wantOut:  "",
			wantKeep: false,
		},
		{
			name:     "whitespace only filtered",
			input:    "   \t  ",
			wantOut:  "",
			wantKeep: false,
		},
		{
			name:     "normal line kept",
			input:    `{"type":"output","content":"hello"}`,
			wantOut:  `{"type":"output","content":"hello"}`,
			wantKeep: true,
		},
		{
			name:     "docker header stripped",
			input:    "\x01\x00\x00\x00\x00\x00\x00\x2a" + `{"type":"output"}`,
			wantOut:  `{"type":"output"}`,
			wantKeep: true,
		},
		{
			name:     "short binary prefix filtered",
			input:    "\x01\x02\x03",
			wantOut:  "",
			wantKeep: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, keep := transport.FilterOutput(tt.input)
			assert.Equal(t, tt.wantKeep, keep)
			if keep {
				assert.Equal(t, tt.wantOut, out)
			}
		})
	}
}

func TestClaudeTransport_ParseToolCall(t *testing.T) {
	t.Parallel()

	transport := &backends.ClaudeTransport{}

	t.Run("valid tool_use parsed", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{"type":"tool_use","id":"call_123","name":"read_file","input":{"path":"/foo"}}`)
		tc, err := transport.ParseToolCall(raw)

		require.NoError(t, err)
		assert.Equal(t, "read_file", tc.Name)
		assert.Equal(t, "call_123", tc.CallID)
		assert.JSONEq(t, `{"path":"/foo"}`, string(tc.Input))
	})

	t.Run("non-tool_use rejected", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{"type":"output","content":"hello"}`)
		_, err := transport.ParseToolCall(raw)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a tool_use event")
	})

	t.Run("invalid JSON rejected", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{not valid json}`)
		_, err := transport.ParseToolCall(raw)

		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// Codex Transport tests
// ---------------------------------------------------------------------------

func TestCodexTransport_AgentName(t *testing.T) {
	t.Parallel()

	transport := &backends.CodexTransport{}
	assert.Equal(t, "codex", transport.AgentName())
}

func TestCodexTransport_Timeouts(t *testing.T) {
	t.Parallel()

	transport := &backends.CodexTransport{}
	assert.Equal(t, 45*time.Second, transport.InitTimeout())
	assert.Equal(t, 10*time.Minute, transport.IdleTimeout())
}

func TestCodexTransport_FilterOutput(t *testing.T) {
	t.Parallel()

	transport := &backends.CodexTransport{}

	tests := []struct {
		name     string
		input    string
		wantOut  string
		wantKeep bool
	}{
		{
			name:     "empty line filtered",
			input:    "",
			wantOut:  "",
			wantKeep: false,
		},
		{
			name:     "whitespace only filtered",
			input:    "   \t  ",
			wantOut:  "",
			wantKeep: false,
		},
		{
			name:     "normal line kept",
			input:    `{"type":"output","content":"hello"}`,
			wantOut:  `{"type":"output","content":"hello"}`,
			wantKeep: true,
		},
		{
			name:     "docker header stripped",
			input:    "\x01\x00\x00\x00\x00\x00\x00\x2a" + `{"type":"output"}`,
			wantOut:  `{"type":"output"}`,
			wantKeep: true,
		},
		{
			name:     "short binary prefix filtered",
			input:    "\x01\x02\x03",
			wantOut:  "",
			wantKeep: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, keep := transport.FilterOutput(tt.input)
			assert.Equal(t, tt.wantKeep, keep)
			if keep {
				assert.Equal(t, tt.wantOut, out)
			}
		})
	}
}

func TestCodexTransport_ParseToolCall(t *testing.T) {
	t.Parallel()

	transport := &backends.CodexTransport{}

	t.Run("valid function_call parsed", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{"type":"function_call","id":"fc_456","name":"write_file","arguments":{"path":"/bar","content":"data"}}`)
		tc, err := transport.ParseToolCall(raw)

		require.NoError(t, err)
		assert.Equal(t, "write_file", tc.Name)
		assert.Equal(t, "fc_456", tc.CallID)
		assert.JSONEq(t, `{"path":"/bar","content":"data"}`, string(tc.Input))
	})

	t.Run("non-function_call rejected", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{"type":"output","content":"hello"}`)
		_, err := transport.ParseToolCall(raw)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a function_call event")
	})

	t.Run("invalid JSON rejected", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{not valid json}`)
		_, err := transport.ParseToolCall(raw)

		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// OpenCode Transport tests
// ---------------------------------------------------------------------------

func TestOpenCodeTransport_AgentName(t *testing.T) {
	t.Parallel()

	transport := &backends.OpenCodeTransport{}
	assert.Equal(t, "opencode", transport.AgentName())
}

func TestOpenCodeTransport_Timeouts(t *testing.T) {
	t.Parallel()

	transport := &backends.OpenCodeTransport{}
	assert.Equal(t, 30*time.Second, transport.InitTimeout())
	assert.Equal(t, 5*time.Minute, transport.IdleTimeout())
}

func TestOpenCodeTransport_FilterOutput(t *testing.T) {
	t.Parallel()

	transport := &backends.OpenCodeTransport{}

	tests := []struct {
		name     string
		input    string
		wantOut  string
		wantKeep bool
	}{
		{
			name:     "empty line filtered",
			input:    "",
			wantOut:  "",
			wantKeep: false,
		},
		{
			name:     "whitespace only filtered",
			input:    "   \t  ",
			wantOut:  "",
			wantKeep: false,
		},
		{
			name:     "normal line kept",
			input:    `{"type":"output","content":"hello"}`,
			wantOut:  `{"type":"output","content":"hello"}`,
			wantKeep: true,
		},
		{
			name:     "docker header stripped",
			input:    "\x01\x00\x00\x00\x00\x00\x00\x2a" + `{"type":"output"}`,
			wantOut:  `{"type":"output"}`,
			wantKeep: true,
		},
		{
			name:     "short binary prefix filtered",
			input:    "\x01\x02\x03",
			wantOut:  "",
			wantKeep: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, keep := transport.FilterOutput(tt.input)
			assert.Equal(t, tt.wantKeep, keep)
			if keep {
				assert.Equal(t, tt.wantOut, out)
			}
		})
	}
}

func TestOpenCodeTransport_ParseToolCall(t *testing.T) {
	t.Parallel()

	transport := &backends.OpenCodeTransport{}

	t.Run("valid tool_call parsed", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{"type":"tool_call","tool_id":"tc_789","tool_name":"exec_cmd","params":{"cmd":"ls -la"}}`)
		tc, err := transport.ParseToolCall(raw)

		require.NoError(t, err)
		assert.Equal(t, "exec_cmd", tc.Name)
		assert.Equal(t, "tc_789", tc.CallID)
		assert.JSONEq(t, `{"cmd":"ls -la"}`, string(tc.Input))
	})

	t.Run("non-tool_call rejected", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{"type":"output","content":"hello"}`)
		_, err := transport.ParseToolCall(raw)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a tool_call event")
	})

	t.Run("invalid JSON rejected", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{not valid json}`)
		_, err := transport.ParseToolCall(raw)

		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// Registry integration tests
// ---------------------------------------------------------------------------

// stubBackend satisfies agent.AgentBackend for registry tests.
type stubBackend struct{}

func (s *stubBackend) StartSession(context.Context, agent.SessionOptions) (agent.SessionID, error) {
	return agent.SessionID{}, nil
}
func (s *stubBackend) SendPrompt(context.Context, agent.SessionID, string) error { return nil }
func (s *stubBackend) Cancel(context.Context, agent.SessionID) error             { return nil }
func (s *stubBackend) OnMessage(agent.MessageHandler)                            {}
func (s *stubBackend) Dispose(context.Context) error                             { return nil }

func TestRegistry_AllBackendsRegistered(t *testing.T) {
	t.Parallel()

	registry := agent.NewRegistry()
	registry.Register("claude", backends.NewClaudeBackend)
	registry.Register("codex", backends.NewCodexBackend)
	registry.Register("opencode", backends.NewOpenCodeBackend)

	available := registry.Available()
	assert.Equal(t, []string{"claude", "codex", "opencode"}, available)
}

func TestRegistry_CreateCodex(t *testing.T) {
	t.Parallel()

	registry := agent.NewRegistry()
	registry.Register("codex", func(_ *agent.DockerRuntime) (agent.AgentBackend, error) {
		return &stubBackend{}, nil
	})

	backend, err := registry.Create("codex", nil)

	require.NoError(t, err)
	assert.NotNil(t, backend)
}

func TestRegistry_CreateOpenCode(t *testing.T) {
	t.Parallel()

	registry := agent.NewRegistry()
	registry.Register("opencode", func(_ *agent.DockerRuntime) (agent.AgentBackend, error) {
		return &stubBackend{}, nil
	})

	backend, err := registry.Create("opencode", nil)

	require.NoError(t, err)
	assert.NotNil(t, backend)
}
