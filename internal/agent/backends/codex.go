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
	codexImage       = "ghcr.io/gosuda/aira-codex:latest"
	codexInitTimeout = 45 * time.Second
	codexIdleTimeout = 10 * time.Minute
)

type codexSession struct {
	containerID string
	cancel      context.CancelFunc
}

// CodexBackend implements agent.AgentBackend for the OpenAI Codex CLI.
// Structure is identical to ClaudeBackend — only image, command, and transport differ.
type CodexBackend struct {
	runtime   *agent.DockerRuntime
	transport *CodexTransport
	handler   agent.MessageHandler
	sessions  map[agent.SessionID]*codexSession
	mu        sync.RWMutex
}

func NewCodexBackend(runtime *agent.DockerRuntime) (agent.AgentBackend, error) {
	return &CodexBackend{
		runtime:   runtime,
		transport: &CodexTransport{},
		sessions:  make(map[agent.SessionID]*codexSession),
	}, nil
}

func (b *CodexBackend) StartSession(ctx context.Context, opts agent.SessionOptions) (agent.SessionID, error) {
	sessionID := opts.SessionID
	if sessionID == uuid.Nil {
		sessionID = uuid.New()
	}

	cmd := []string{
		"codex",
		"--full-auto",
		"--json",
		"-q", opts.Prompt,
	}

	containerID, err := b.runtime.CreateContainer(ctx, agent.ContainerOptions{
		SessionID:   sessionID,
		Image:       codexImage,
		VolumeName:  opts.ProjectDir,
		ProjectDir:  "/repo",
		WorkDir:     opts.WorkDir,
		BranchName:  opts.BranchName,
		Environment: opts.Environment,
		Cmd:         cmd,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("agent.CodexBackend.StartSession: %w", err)
	}

	sessionCtx, cancel := context.WithCancel(ctx)

	b.mu.Lock()
	b.sessions[sessionID] = &codexSession{
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
			log.Error().Err(rmErr).Str("container_id", containerID).Msg("agent.CodexBackend: failed to remove container after start failure")
		}
		return uuid.Nil, fmt.Errorf("agent.CodexBackend.StartSession: %w", err)
	}

	// Stream logs in background goroutine.
	go b.streamOutput(sessionCtx, sessionID, containerID)

	return sessionID, nil
}

func (b *CodexBackend) SendPrompt(ctx context.Context, sessionID agent.SessionID, prompt string) error {
	b.mu.RLock()
	sess, ok := b.sessions[sessionID]
	b.mu.RUnlock()

	if !ok {
		return fmt.Errorf("agent.CodexBackend.SendPrompt: session %s not found", sessionID)
	}

	data := []byte(prompt + "\n")
	cmd := []string{"sh", "-c", "cat > /proc/1/fd/0"}

	err := b.runtime.ExecInContainer(ctx, sess.containerID, cmd, data)
	if err != nil {
		return fmt.Errorf("agent.CodexBackend.SendPrompt: %w", err)
	}

	return nil
}

func (b *CodexBackend) Cancel(ctx context.Context, sessionID agent.SessionID) error {
	b.mu.RLock()
	sess, ok := b.sessions[sessionID]
	b.mu.RUnlock()

	if !ok {
		return fmt.Errorf("agent.CodexBackend.Cancel: session %s not found", sessionID)
	}

	sess.cancel()

	err := b.runtime.StopContainer(ctx, sess.containerID)
	if err != nil {
		return fmt.Errorf("agent.CodexBackend.Cancel: %w", err)
	}

	return nil
}

func (b *CodexBackend) OnMessage(handler agent.MessageHandler) {
	b.mu.Lock()
	b.handler = handler
	b.mu.Unlock()
}

func (b *CodexBackend) Dispose(ctx context.Context) error {
	b.mu.Lock()
	sessions := make(map[agent.SessionID]*codexSession, len(b.sessions))
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
		return fmt.Errorf("agent.CodexBackend.Dispose: %w", firstErr)
	}
	return nil
}

// streamOutput reads container logs and dispatches parsed messages to the handler.
func (b *CodexBackend) streamOutput(ctx context.Context, sessionID agent.SessionID, containerID string) {
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

func (b *CodexBackend) parseLine(sessionID agent.SessionID, line string) agent.Message {
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

func (b *CodexBackend) emitMessage(msg agent.Message) {
	b.mu.RLock()
	handler := b.handler
	b.mu.RUnlock()

	if handler != nil {
		handler(msg)
	}
}

// CodexTransport implements agent.TransportHandler for the OpenAI Codex CLI.
type CodexTransport struct{}

func (t *CodexTransport) AgentName() string          { return "codex" }
func (t *CodexTransport) InitTimeout() time.Duration { return codexInitTimeout }
func (t *CodexTransport) IdleTimeout() time.Duration { return codexIdleTimeout }

func (t *CodexTransport) FilterOutput(line string) (string, bool) {
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

func (t *CodexTransport) ParseToolCall(raw json.RawMessage) (agent.ToolCall, error) {
	// Codex emits function_call events with structure:
	// {"type":"function_call","id":"...","name":"...","arguments":{...}}
	var envelope struct {
		Type      string          `json:"type"`
		ID        string          `json:"id"`
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	err := json.Unmarshal(raw, &envelope)
	if err != nil {
		return agent.ToolCall{}, fmt.Errorf("agent.CodexTransport.ParseToolCall: %w", err)
	}

	if envelope.Type != "function_call" {
		return agent.ToolCall{}, fmt.Errorf("agent.CodexTransport.ParseToolCall: not a function_call event (type=%q)", envelope.Type)
	}

	return agent.ToolCall{
		Name:   envelope.Name,
		Input:  envelope.Arguments,
		CallID: envelope.ID,
	}, nil
}
