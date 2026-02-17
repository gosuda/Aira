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

	"github.com/gosuda/aira/internal/agent"
)

const (
	opencodeImage       = "ghcr.io/gosuda/aira-opencode:latest"
	opencodeInitTimeout = 30 * time.Second
	opencodeIdleTimeout = 5 * time.Minute
)

type opencodeSession struct {
	containerID string
	cancel      context.CancelFunc
}

// OpenCodeBackend implements agent.AgentBackend for the OpenCode CLI.
// Structure is identical to ClaudeBackend — only image, command, and transport differ.
type OpenCodeBackend struct {
	runtime   *agent.DockerRuntime
	transport *OpenCodeTransport
	handler   agent.MessageHandler
	sessions  map[agent.SessionID]*opencodeSession
	mu        sync.RWMutex
}

func NewOpenCodeBackend(runtime *agent.DockerRuntime) (agent.AgentBackend, error) {
	return &OpenCodeBackend{
		runtime:   runtime,
		transport: &OpenCodeTransport{},
		sessions:  make(map[agent.SessionID]*opencodeSession),
	}, nil
}

func (b *OpenCodeBackend) StartSession(ctx context.Context, opts agent.SessionOptions) (agent.SessionID, error) {
	sessionID := opts.SessionID
	if sessionID == uuid.Nil {
		sessionID = uuid.New()
	}

	cmd := []string{
		"opencode",
		"run",
		"--output", "json",
		opts.Prompt,
	}

	containerID, err := b.runtime.CreateContainer(ctx, agent.ContainerOptions{
		SessionID:   sessionID,
		Image:       opencodeImage,
		VolumeName:  opts.ProjectDir,
		ProjectDir:  "/repo",
		WorkDir:     opts.WorkDir,
		BranchName:  opts.BranchName,
		Environment: opts.Environment,
		Cmd:         cmd,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("agent.OpenCodeBackend.StartSession: %w", err)
	}

	sessionCtx, cancel := context.WithCancel(ctx)

	b.mu.Lock()
	b.sessions[sessionID] = &opencodeSession{
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
		_ = b.runtime.RemoveContainer(ctx, containerID)
		return uuid.Nil, fmt.Errorf("agent.OpenCodeBackend.StartSession: %w", err)
	}

	// Stream logs in background goroutine.
	go b.streamOutput(sessionCtx, sessionID, containerID)

	return sessionID, nil
}

func (b *OpenCodeBackend) SendPrompt(ctx context.Context, sessionID agent.SessionID, prompt string) error {
	b.mu.RLock()
	sess, ok := b.sessions[sessionID]
	b.mu.RUnlock()

	if !ok {
		return fmt.Errorf("agent.OpenCodeBackend.SendPrompt: session %s not found", sessionID)
	}

	data := []byte(prompt + "\n")
	cmd := []string{"sh", "-c", "cat > /proc/1/fd/0"}

	err := b.runtime.ExecInContainer(ctx, sess.containerID, cmd, data)
	if err != nil {
		return fmt.Errorf("agent.OpenCodeBackend.SendPrompt: %w", err)
	}

	return nil
}

func (b *OpenCodeBackend) Cancel(ctx context.Context, sessionID agent.SessionID) error {
	b.mu.RLock()
	sess, ok := b.sessions[sessionID]
	b.mu.RUnlock()

	if !ok {
		return fmt.Errorf("agent.OpenCodeBackend.Cancel: session %s not found", sessionID)
	}

	sess.cancel()

	err := b.runtime.StopContainer(ctx, sess.containerID)
	if err != nil {
		return fmt.Errorf("agent.OpenCodeBackend.Cancel: %w", err)
	}

	return nil
}

func (b *OpenCodeBackend) OnMessage(handler agent.MessageHandler) {
	b.mu.Lock()
	b.handler = handler
	b.mu.Unlock()
}

func (b *OpenCodeBackend) Dispose(ctx context.Context) error {
	b.mu.Lock()
	sessions := make(map[agent.SessionID]*opencodeSession, len(b.sessions))
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
		return fmt.Errorf("agent.OpenCodeBackend.Dispose: %w", firstErr)
	}
	return nil
}

// streamOutput reads container logs and dispatches parsed messages to the handler.
func (b *OpenCodeBackend) streamOutput(ctx context.Context, sessionID agent.SessionID, containerID string) {
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

func (b *OpenCodeBackend) parseLine(sessionID agent.SessionID, line string) agent.Message {
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

func (b *OpenCodeBackend) emitMessage(msg agent.Message) {
	b.mu.RLock()
	handler := b.handler
	b.mu.RUnlock()

	if handler != nil {
		handler(msg)
	}
}

// OpenCodeTransport implements agent.TransportHandler for the OpenCode CLI.
type OpenCodeTransport struct{}

func (t *OpenCodeTransport) AgentName() string          { return "opencode" }
func (t *OpenCodeTransport) InitTimeout() time.Duration { return opencodeInitTimeout }
func (t *OpenCodeTransport) IdleTimeout() time.Duration { return opencodeIdleTimeout }

func (t *OpenCodeTransport) FilterOutput(line string) (string, bool) {
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

	return trimmed, true
}

func (t *OpenCodeTransport) ParseToolCall(raw json.RawMessage) (agent.ToolCall, error) {
	// OpenCode emits tool_call events with structure:
	// {"type":"tool_call","tool_id":"...","tool_name":"...","params":{...}}
	var envelope struct {
		Type     string          `json:"type"`
		ToolID   string          `json:"tool_id"`
		ToolName string          `json:"tool_name"`
		Params   json.RawMessage `json:"params"`
	}

	err := json.Unmarshal(raw, &envelope)
	if err != nil {
		return agent.ToolCall{}, fmt.Errorf("agent.OpenCodeTransport.ParseToolCall: %w", err)
	}

	if envelope.Type != "tool_call" {
		return agent.ToolCall{}, fmt.Errorf("agent.OpenCodeTransport.ParseToolCall: not a tool_call event (type=%q)", envelope.Type)
	}

	return agent.ToolCall{
		Name:   envelope.ToolName,
		Input:  envelope.Params,
		CallID: envelope.ToolID,
	}, nil
}
