package agent_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gosuda/aira/internal/agent"
)

// --- stub AgentBackend for registry tests ---

type stubAgentBackend struct {
	agentType string
}

func (s *stubAgentBackend) StartSession(context.Context, agent.SessionOptions) (agent.SessionID, error) {
	return uuid.New(), nil
}
func (s *stubAgentBackend) SendPrompt(context.Context, agent.SessionID, string) error { return nil }
func (s *stubAgentBackend) Cancel(context.Context, agent.SessionID) error             { return nil }
func (s *stubAgentBackend) OnMessage(agent.MessageHandler)                            {}
func (s *stubAgentBackend) Dispose(context.Context) error                             { return nil }

func TestRegistry_RegisterAndCreate(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		reg := agent.NewRegistry()
		reg.Register("claude", func(_ *agent.DockerRuntime) (agent.AgentBackend, error) {
			return &stubAgentBackend{agentType: "claude"}, nil
		})

		backend, err := reg.Create("claude", nil)

		require.NoError(t, err)
		require.NotNil(t, backend)
	})

	t.Run("unknown agent type returns ErrUnknownAgent", func(t *testing.T) {
		t.Parallel()

		reg := agent.NewRegistry()

		backend, err := reg.Create("nonexistent", nil)

		require.Error(t, err)
		assert.Nil(t, backend)
		assert.ErrorIs(t, err, agent.ErrUnknownAgent)
	})

	t.Run("factory error propagated", func(t *testing.T) {
		t.Parallel()

		reg := agent.NewRegistry()
		reg.Register("broken", func(_ *agent.DockerRuntime) (agent.AgentBackend, error) {
			return nil, errors.New("factory boom")
		})

		backend, err := reg.Create("broken", nil)

		require.Error(t, err)
		assert.Nil(t, backend)
		assert.Contains(t, err.Error(), "factory boom")
	})

	t.Run("Available returns sorted names", func(t *testing.T) {
		t.Parallel()

		reg := agent.NewRegistry()
		reg.Register("codex", func(_ *agent.DockerRuntime) (agent.AgentBackend, error) {
			return &stubAgentBackend{}, nil
		})
		reg.Register("acp", func(_ *agent.DockerRuntime) (agent.AgentBackend, error) {
			return &stubAgentBackend{}, nil
		})
		reg.Register("claude", func(_ *agent.DockerRuntime) (agent.AgentBackend, error) {
			return &stubAgentBackend{}, nil
		})

		available := reg.Available()

		assert.Equal(t, []string{"acp", "claude", "codex"}, available)
	})
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	reg := agent.NewRegistry()

	// Pre-register one backend.
	reg.Register("claude", func(_ *agent.DockerRuntime) (agent.AgentBackend, error) {
		return &stubAgentBackend{agentType: "claude"}, nil
	})

	var wg sync.WaitGroup

	// Concurrent registers.
	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := "agent-" + uuid.New().String()[:8]
			_ = idx
			reg.Register(name, func(_ *agent.DockerRuntime) (agent.AgentBackend, error) {
				return &stubAgentBackend{agentType: name}, nil
			})
		}(i)
	}

	// Concurrent creates.
	for range 10 {
		wg.Go(func() {
			backend, err := reg.Create("claude", nil)
			require.NoError(t, err)
			require.NotNil(t, backend)
		})
	}

	// Concurrent Available calls.
	for range 5 {
		wg.Go(func() {
			_ = reg.Available()
		})
	}

	wg.Wait()

	// After all goroutines complete, "claude" plus 10 agent-* should be registered.
	available := reg.Available()
	assert.GreaterOrEqual(t, len(available), 11)
}
