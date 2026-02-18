package backends

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/gosuda/aira/internal/agent"
)

const (
	acpImage       = "ghcr.io/gosuda/aira-acp:latest"
	acpInitTimeout = 60 * time.Second
	acpIdleTimeout = 15 * time.Minute

	acpDefaultEndpoint = "http://localhost:8090"
)

type acpSession struct {
	containerID string
	cancel      context.CancelFunc
}

// ACPBackend implements agent.AgentBackend for ACP-compliant agent servers.
// It runs a thin ACP client inside a Docker container that:
// 1. Connects to the configured ACP endpoint
// 2. Sends prompts via JSON-RPC
// 3. Streams responses back to stdout.
type ACPBackend struct {
	runtime   *agent.DockerRuntime
	transport *ACPTransport
	handler   agent.MessageHandler
	sessions  map[agent.SessionID]*acpSession
	mu        sync.RWMutex
}

func NewACPBackend(runtime *agent.DockerRuntime) (agent.AgentBackend, error) {
	return &ACPBackend{
		runtime:   runtime,
		transport: &ACPTransport{},
		sessions:  make(map[agent.SessionID]*acpSession),
	}, nil
}

func (b *ACPBackend) StartSession(ctx context.Context, opts agent.SessionOptions) (agent.SessionID, error) {
	sessionID := opts.SessionID
	if sessionID == uuid.Nil {
		sessionID = uuid.New()
	}

	endpoint := acpDefaultEndpoint
	if ep, ok := opts.Environment["ACP_ENDPOINT"]; ok && ep != "" {
		endpoint = ep
	}

	cmd := []string{
		"acp-client",
		"--endpoint", endpoint,
		"--json", opts.Prompt,
	}

	containerID, err := b.runtime.CreateContainer(ctx, agent.ContainerOptions{
		SessionID:   sessionID,
		Image:       acpImage,
		VolumeName:  opts.ProjectDir,
		ProjectDir:  "/repo",
		WorkDir:     opts.WorkDir,
		BranchName:  opts.BranchName,
		Environment: opts.Environment,
		Cmd:         cmd,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("agent.ACPBackend.StartSession: %w", err)
	}

	sessionCtx, cancel := context.WithCancel(ctx)

	b.mu.Lock()
	b.sessions[sessionID] = &acpSession{
		containerID: containerID,
		cancel:      cancel,
	}
	b.mu.Unlock()

	err = b.runtime.StartContainer(ctx, containerID)
	if err != nil {
		cancel()
		b.mu.Lock()
		delete(b.sessions, sessionID)
		b.mu.Unlock()
		if rmErr := b.runtime.RemoveContainer(ctx, containerID); rmErr != nil {
			log.Error().Err(rmErr).Str("container_id", containerID).Msg("agent.ACPBackend: failed to remove container after start failure")
		}
		return uuid.Nil, fmt.Errorf("agent.ACPBackend.StartSession: %w", err)
	}

	// Stream logs in background goroutine.
	go b.streamOutput(sessionCtx, sessionID, containerID)

	return sessionID, nil
}

func (b *ACPBackend) SendPrompt(ctx context.Context, sessionID agent.SessionID, prompt string) error {
	b.mu.RLock()
	sess, ok := b.sessions[sessionID]
	b.mu.RUnlock()

	if !ok {
		return fmt.Errorf("agent.ACPBackend.SendPrompt: session %s not found", sessionID)
	}

	data := []byte(prompt + "\n")
	cmd := []string{"sh", "-c", "cat > /proc/1/fd/0"}

	err := b.runtime.ExecInContainer(ctx, sess.containerID, cmd, data)
	if err != nil {
		return fmt.Errorf("agent.ACPBackend.SendPrompt: %w", err)
	}

	return nil
}

func (b *ACPBackend) Cancel(ctx context.Context, sessionID agent.SessionID) error {
	b.mu.RLock()
	sess, ok := b.sessions[sessionID]
	b.mu.RUnlock()

	if !ok {
		return fmt.Errorf("agent.ACPBackend.Cancel: session %s not found", sessionID)
	}

	sess.cancel()

	err := b.runtime.StopContainer(ctx, sess.containerID)
	if err != nil {
		return fmt.Errorf("agent.ACPBackend.Cancel: %w", err)
	}

	return nil
}

func (b *ACPBackend) OnMessage(handler agent.MessageHandler) {
	b.mu.Lock()
	b.handler = handler
	b.mu.Unlock()
}

func (b *ACPBackend) Dispose(ctx context.Context) error {
	b.mu.Lock()
	sessions := make(map[agent.SessionID]*acpSession, len(b.sessions))
	maps.Copy(sessions, b.sessions)
	b.mu.Unlock()

	var firstErr error
	for sid, sess := range sessions {
		sess.cancel()

		err := b.runtime.StopContainer(ctx, sess.containerID)
		if err != nil && firstErr == nil {
			firstErr = err
		}

		err = b.runtime.RemoveContainer(ctx, sess.containerID)
		if err != nil && firstErr == nil {
			firstErr = err
		}

		b.mu.Lock()
		delete(b.sessions, sid)
		b.mu.Unlock()
	}

	if firstErr != nil {
		return fmt.Errorf("agent.ACPBackend.Dispose: %w", firstErr)
	}
	return nil
}

// streamOutput reads container logs and dispatches parsed messages to the handler.
func (b *ACPBackend) streamOutput(ctx context.Context, sessionID agent.SessionID, containerID string) {
	reader, err := b.runtime.StreamLogs(ctx, containerID)
	if err != nil {
		b.emitMessage(agent.Message{
			Type:      agent.MessageTypeError,
			SessionID: sessionID,
			Content:   fmt.Sprintf("failed to stream logs: %v", err),
			Timestamp: time.Now(),
		})
		return
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		filtered, keep := b.transport.FilterOutput(line)
		if !keep {
			continue
		}

		msg := b.parseLine(sessionID, filtered)
		b.emitMessage(msg)
	}

	if scanErr := scanner.Err(); scanErr != nil {
		select {
		case <-ctx.Done():
			// Context cancelled, expected.
		default:
			b.emitMessage(agent.Message{
				Type:      agent.MessageTypeError,
				SessionID: sessionID,
				Content:   fmt.Sprintf("log stream error: %v", scanErr),
				Timestamp: time.Now(),
			})
		}
	}
}

func (b *ACPBackend) parseLine(sessionID agent.SessionID, line string) agent.Message {
	var raw json.RawMessage
	if json.Unmarshal([]byte(line), &raw) == nil {
		tc, err := b.transport.ParseToolCall(raw)
		if err == nil && tc.Name != "" {
			return agent.Message{
				Type:      agent.MessageTypeToolCall,
				SessionID: sessionID,
				Content:   tc.Name,
				Raw:       raw,
				Timestamp: time.Now(),
			}
		}

		return agent.Message{
			Type:      agent.MessageTypeOutput,
			SessionID: sessionID,
			Content:   line,
			Raw:       raw,
			Timestamp: time.Now(),
		}
	}

	return agent.Message{
		Type:      agent.MessageTypeOutput,
		SessionID: sessionID,
		Content:   line,
		Timestamp: time.Now(),
	}
}

func (b *ACPBackend) emitMessage(msg agent.Message) {
	b.mu.RLock()
	handler := b.handler
	b.mu.RUnlock()

	if handler != nil {
		handler(msg)
	}
}

// ACPTransport implements agent.TransportHandler for ACP-compliant agent servers.
type ACPTransport struct{}

func (t *ACPTransport) AgentName() string          { return "acp" }
func (t *ACPTransport) InitTimeout() time.Duration { return acpInitTimeout }
func (t *ACPTransport) IdleTimeout() time.Duration { return acpIdleTimeout }

func (t *ACPTransport) FilterOutput(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", false
	}

	// Strip Docker log header (8-byte prefix for multiplexed streams).
	if trimmed != "" && trimmed[0] < 0x20 {
		if len(trimmed) > 8 {
			trimmed = trimmed[8:]
		} else {
			return "", false
		}
	}

	// Filter debug lines from the ACP client.
	if strings.HasPrefix(strings.ToLower(trimmed), "acp: debug") {
		return "", false
	}

	return trimmed, true
}

func (t *ACPTransport) ParseToolCall(raw json.RawMessage) (agent.ToolCall, error) {
	// ACP emits tool_call events with JSON-RPC structure:
	// {"method":"tool_call","params":{"tool":"name","arguments":{...},"id":"..."}}
	var envelope struct {
		Method string `json:"method"`
		Params struct {
			Tool      string          `json:"tool"`
			Arguments json.RawMessage `json:"arguments"`
			ID        string          `json:"id"`
		} `json:"params"`
	}

	err := json.Unmarshal(raw, &envelope)
	if err != nil {
		return agent.ToolCall{}, fmt.Errorf("agent.ACPTransport.ParseToolCall: %w", err)
	}

	if envelope.Method != "tool_call" {
		return agent.ToolCall{}, fmt.Errorf("agent.ACPTransport.ParseToolCall: not a tool_call event (method=%q)", envelope.Method)
	}

	return agent.ToolCall{
		Name:   envelope.Params.Tool,
		Input:  envelope.Params.Arguments,
		CallID: envelope.Params.ID,
	}, nil
}
