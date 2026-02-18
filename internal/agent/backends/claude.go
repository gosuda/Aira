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
	claudeImage       = "ghcr.io/gosuda/aira-claude:latest"
	claudeInitTimeout = 30 * time.Second
	claudeIdleTimeout = 5 * time.Minute
)

type claudeSession struct {
	containerID string
	cancel      context.CancelFunc
}

// ClaudeBackend implements agent.AgentBackend for the Claude Code CLI.
// It runs Claude Code inside a Docker container and communicates via stdin/stdout.
type ClaudeBackend struct {
	runtime   *agent.DockerRuntime
	transport *ClaudeTransport
	handler   agent.MessageHandler
	sessions  map[agent.SessionID]*claudeSession
	mu        sync.RWMutex
}

func NewClaudeBackend(runtime *agent.DockerRuntime) (agent.AgentBackend, error) {
	return &ClaudeBackend{
		runtime:   runtime,
		transport: &ClaudeTransport{},
		sessions:  make(map[agent.SessionID]*claudeSession),
	}, nil
}

func (b *ClaudeBackend) StartSession(ctx context.Context, opts agent.SessionOptions) (agent.SessionID, error) {
	sessionID := opts.SessionID
	if sessionID == uuid.Nil {
		sessionID = uuid.New()
	}

	cmd := []string{
		"claude",
		"--output-format", "stream-json",
		"--verbose",
		"-p", opts.Prompt,
	}

	containerID, err := b.runtime.CreateContainer(ctx, agent.ContainerOptions{
		SessionID:   sessionID,
		Image:       claudeImage,
		VolumeName:  opts.ProjectDir,
		ProjectDir:  "/repo",
		WorkDir:     opts.WorkDir,
		BranchName:  opts.BranchName,
		Environment: opts.Environment,
		Cmd:         cmd,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("agent.ClaudeBackend.StartSession: %w", err)
	}

	sessionCtx, cancel := context.WithCancel(context.Background())

	b.mu.Lock()
	b.sessions[sessionID] = &claudeSession{
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
			log.Error().Err(rmErr).Str("container_id", containerID).Msg("agent.ClaudeBackend: failed to remove container after start failure")
		}
		return uuid.Nil, fmt.Errorf("agent.ClaudeBackend.StartSession: %w", err)
	}

	// Stream logs in background goroutine.
	go b.streamOutput(sessionCtx, sessionID, containerID)

	return sessionID, nil
}

func (b *ClaudeBackend) SendPrompt(ctx context.Context, sessionID agent.SessionID, prompt string) error {
	b.mu.RLock()
	sess, ok := b.sessions[sessionID]
	b.mu.RUnlock()

	if !ok {
		return fmt.Errorf("agent.ClaudeBackend.SendPrompt: session %s not found", sessionID)
	}

	// Write the prompt followed by a newline to the container's stdin via exec.
	data := []byte(prompt + "\n")
	cmd := []string{"sh", "-c", "cat > /proc/1/fd/0"}

	err := b.runtime.ExecInContainer(ctx, sess.containerID, cmd, data)
	if err != nil {
		return fmt.Errorf("agent.ClaudeBackend.SendPrompt: %w", err)
	}

	return nil
}

func (b *ClaudeBackend) Cancel(ctx context.Context, sessionID agent.SessionID) error {
	b.mu.RLock()
	sess, ok := b.sessions[sessionID]
	b.mu.RUnlock()

	if !ok {
		return fmt.Errorf("agent.ClaudeBackend.Cancel: session %s not found", sessionID)
	}

	sess.cancel()

	err := b.runtime.StopContainer(ctx, sess.containerID)
	if err != nil {
		return fmt.Errorf("agent.ClaudeBackend.Cancel: %w", err)
	}

	return nil
}

func (b *ClaudeBackend) OnMessage(handler agent.MessageHandler) {
	b.mu.Lock()
	b.handler = handler
	b.mu.Unlock()
}

func (b *ClaudeBackend) Dispose(ctx context.Context) error {
	b.mu.Lock()
	sessions := make(map[agent.SessionID]*claudeSession, len(b.sessions))
	maps.Copy(sessions, b.sessions)
	b.mu.Unlock()

	var firstErr error
	for sid, sess := range sessions {
		sess.cancel()

		err := b.runtime.StopContainer(ctx, sess.containerID)
		if err != nil {
			log.Error().Err(err).Str("session_id", sid.String()).Str("container_id", sess.containerID).Msg("agent.ClaudeBackend.Dispose: failed to stop container")
			if firstErr == nil {
				firstErr = err
			}
		}

		err = b.runtime.RemoveContainer(ctx, sess.containerID)
		if err != nil {
			log.Error().Err(err).Str("session_id", sid.String()).Str("container_id", sess.containerID).Msg("agent.ClaudeBackend.Dispose: failed to remove container")
			if firstErr == nil {
				firstErr = err
			}
		}

		b.mu.Lock()
		delete(b.sessions, sid)
		b.mu.Unlock()
	}

	if firstErr != nil {
		return fmt.Errorf("agent.ClaudeBackend.Dispose: %w", firstErr)
	}
	return nil
}

// streamOutput reads container logs and dispatches parsed messages to the handler.
func (b *ClaudeBackend) streamOutput(ctx context.Context, sessionID agent.SessionID, containerID string) {
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
	// Set a larger buffer for long JSON lines.
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

func (b *ClaudeBackend) parseLine(sessionID agent.SessionID, line string) agent.Message {
	// Try to parse as JSON (stream-json output).
	var raw json.RawMessage
	if json.Unmarshal([]byte(line), &raw) == nil {
		// Check if it's a tool call.
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

func (b *ClaudeBackend) emitMessage(msg agent.Message) {
	b.mu.RLock()
	handler := b.handler
	b.mu.RUnlock()

	if handler != nil {
		handler(msg)
	}
}

// ClaudeTransport implements agent.TransportHandler for Claude Code CLI.
type ClaudeTransport struct{}

func (t *ClaudeTransport) AgentName() string          { return "claude" }
func (t *ClaudeTransport) InitTimeout() time.Duration { return claudeInitTimeout }
func (t *ClaudeTransport) IdleTimeout() time.Duration { return claudeIdleTimeout }

func (t *ClaudeTransport) FilterOutput(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", false
	}

	// Strip Docker log header (8-byte prefix for multiplexed streams).
	// Docker log lines from ContainerLogs with timestamps look like:
	// <8-byte-header><timestamp> <content>
	// We detect and skip the binary header.
	if trimmed != "" && trimmed[0] < 0x20 {
		// Binary prefix from Docker multiplexed stream.
		if len(trimmed) > 8 {
			trimmed = trimmed[8:]
		} else {
			return "", false
		}
	}

	return trimmed, true
}

func (t *ClaudeTransport) ParseToolCall(raw json.RawMessage) (agent.ToolCall, error) {
	// Claude stream-json emits tool_use events with structure:
	// {"type":"tool_use","id":"...","name":"...","input":{...}}
	var envelope struct {
		Type  string          `json:"type"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	}

	err := json.Unmarshal(raw, &envelope)
	if err != nil {
		return agent.ToolCall{}, fmt.Errorf("agent.ClaudeTransport.ParseToolCall: %w", err)
	}

	if envelope.Type != "tool_use" {
		return agent.ToolCall{}, fmt.Errorf("agent.ClaudeTransport.ParseToolCall: not a tool_use event (type=%q)", envelope.Type)
	}

	return agent.ToolCall{
		Name:   envelope.Name,
		Input:  envelope.Input,
		CallID: envelope.ID,
	}, nil
}
