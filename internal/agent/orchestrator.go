package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/gosuda/aira/internal/domain"
)

// ErrSessionNotFound is returned when a session is not found.
var ErrSessionNotFound = errors.New("agent: session not found") //nolint:gochecknoglobals // sentinel error

// ErrInvalidSessionState is returned when an operation is invalid for the current session state.
var ErrInvalidSessionState = errors.New("agent: invalid session state") //nolint:gochecknoglobals // sentinel error

// PubSubPublisher abstracts the Redis pub/sub publish operation.
type PubSubPublisher interface {
	Publish(ctx context.Context, channel string, payload []byte) error
}

// Orchestrator coordinates the full agent session lifecycle:
// task pickup -> container creation -> agent execution -> HITL -> completion.
type Orchestrator struct {
	registry *Registry
	runtime  *DockerRuntime
	volumes  *VolumeManager
	sessions domain.AgentSessionRepository
	tasks    domain.TaskRepository
	projects domain.ProjectRepository
	adrs     domain.ADRRepository
	pubsub   PubSubPublisher

	// backends tracks active backends by session ID for HITL and cancellation.
	backends map[SessionID]AgentBackend
	mu       sync.RWMutex

	done chan struct{}
}

func NewOrchestrator(
	registry *Registry,
	runtime *DockerRuntime,
	volumes *VolumeManager,
	sessions domain.AgentSessionRepository,
	tasks domain.TaskRepository,
	projects domain.ProjectRepository,
	adrs domain.ADRRepository,
	pubsub PubSubPublisher,
) *Orchestrator {
	return &Orchestrator{
		registry: registry,
		runtime:  runtime,
		volumes:  volumes,
		sessions: sessions,
		tasks:    tasks,
		projects: projects,
		adrs:     adrs,
		pubsub:   pubsub,
		backends: make(map[SessionID]AgentBackend),
		done:     make(chan struct{}),
	}
}

// Shutdown signals all background goroutines to stop.
func (o *Orchestrator) Shutdown() {
	close(o.done)
}

// StartTask picks up a task, creates an agent session, prepares the repo volume,
// creates an isolated branch, starts the agent, and manages the lifecycle.
func (o *Orchestrator) StartTask(ctx context.Context, tenantID, taskID uuid.UUID, agentType string) (*domain.AgentSession, error) {
	// 1. Get task, verify eligible status.
	task, err := o.tasks.GetByID(ctx, tenantID, taskID)
	if err != nil {
		return nil, fmt.Errorf("agent.Orchestrator.StartTask: get task: %w", err)
	}

	if task.Status != domain.TaskStatusBacklog && task.Status != domain.TaskStatusInProgress {
		return nil, fmt.Errorf("agent.Orchestrator.StartTask: task status %q not eligible: %w", task.Status, ErrInvalidSessionState)
	}

	// 2. Create AgentSession in DB (status: pending).
	now := time.Now()
	session := &domain.AgentSession{
		ID:        uuid.New(),
		TenantID:  tenantID,
		ProjectID: task.ProjectID,
		TaskID:    &taskID,
		AgentType: agentType,
		Status:    domain.AgentStatusPending,
		StartedAt: &now,
		CreatedAt: now,
	}
	session.BranchName = session.GenerateBranchName()

	err = o.sessions.Create(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("agent.Orchestrator.StartTask: create session: %w", err)
	}

	// 3. Get project -> ensure volume -> git fetch -> create branch.
	project, err := o.projects.GetByID(ctx, tenantID, task.ProjectID)
	if err != nil {
		o.failSession(ctx, session.ID, tenantID, "failed to get project: "+err.Error())
		return nil, fmt.Errorf("agent.Orchestrator.StartTask: get project: %w", err)
	}

	volumeName := "aira-repo-" + project.ID.String()

	err = o.volumes.EnsureVolume(ctx, volumeName)
	if err != nil {
		o.failSession(ctx, session.ID, tenantID, "failed to ensure volume: "+err.Error())
		return nil, fmt.Errorf("agent.Orchestrator.StartTask: ensure volume: %w", err)
	}

	err = o.volumes.CloneRepo(ctx, volumeName, project.RepoURL)
	if err != nil {
		o.failSession(ctx, session.ID, tenantID, "failed to clone repo: "+err.Error())
		return nil, fmt.Errorf("agent.Orchestrator.StartTask: clone repo: %w", err)
	}

	err = o.volumes.FetchRepo(ctx, volumeName)
	if err != nil {
		o.failSession(ctx, session.ID, tenantID, "failed to fetch repo: "+err.Error())
		return nil, fmt.Errorf("agent.Orchestrator.StartTask: fetch repo: %w", err)
	}

	baseBranch := project.Branch
	if baseBranch == "" {
		baseBranch = "main"
	}

	err = o.volumes.CreateBranch(ctx, volumeName, session.BranchName, "origin/"+baseBranch)
	if err != nil {
		o.failSession(ctx, session.ID, tenantID, "failed to create branch: "+err.Error())
		return nil, fmt.Errorf("agent.Orchestrator.StartTask: create branch: %w", err)
	}

	// 4. Create backend via registry.
	backend, err := o.registry.Create(agentType, o.runtime)
	if err != nil {
		o.failSession(ctx, session.ID, tenantID, "failed to create backend: "+err.Error())
		return nil, fmt.Errorf("agent.Orchestrator.StartTask: %w", err)
	}

	// 5. Register message handler before starting.
	backend.OnMessage(func(msg Message) {
		o.handleMessage(session.ID, session.TenantID, msg)
	})

	// 6. Start session via backend.
	// WorkDir points to the isolated worktree so multiple agents can run concurrently.
	prompt := buildPrompt(task)
	worktreeDir := "/repo/.worktrees/" + session.BranchName

	_, err = backend.StartSession(ctx, SessionOptions{
		SessionID:   session.ID,
		ProjectDir:  volumeName,
		WorkDir:     worktreeDir,
		BranchName:  session.BranchName,
		Prompt:      prompt,
		Environment: nil,
		AgentType:   agentType,
	})
	if err != nil {
		if disposeErr := backend.Dispose(ctx); disposeErr != nil {
			log.Error().Err(disposeErr).Str("session_id", session.ID.String()).Msg("agent.StartTask: failed to dispose backend after start failure")
		}
		o.failSession(ctx, session.ID, tenantID, "failed to start session: "+err.Error())
		return nil, fmt.Errorf("agent.Orchestrator.StartTask: start session: %w", err)
	}

	// Track backend for HITL and cancellation.
	o.mu.Lock()
	o.backends[session.ID] = backend
	o.mu.Unlock()

	// 7. Update session status to running.
	err = o.sessions.UpdateStatus(ctx, tenantID, session.ID, domain.AgentStatusRunning)
	if err != nil {
		return nil, fmt.Errorf("agent.Orchestrator.StartTask: update session running: %w", err)
	}
	session.Status = domain.AgentStatusRunning

	// 8. Update task status to in_progress if in backlog.
	if task.Status == domain.TaskStatusBacklog {
		err = o.tasks.UpdateStatus(ctx, tenantID, taskID, domain.TaskStatusInProgress)
		if err != nil {
			return nil, fmt.Errorf("agent.Orchestrator.StartTask: update task status: %w", err)
		}
	}

	// 9. Start background goroutine to wait for completion.
	go o.waitForCompletion(session.ID, session.TenantID)

	return session, nil
}

// HandleHITLResponse resumes an agent session after a human answers a question.
func (o *Orchestrator) HandleHITLResponse(ctx context.Context, sessionID, tenantID uuid.UUID, answer string) error {
	session, err := o.sessions.GetByID(ctx, tenantID, sessionID)
	if err != nil {
		return fmt.Errorf("agent.Orchestrator.HandleHITLResponse: %w", err)
	}

	if session.Status != domain.AgentStatusWaitingHITL {
		return fmt.Errorf("agent.Orchestrator.HandleHITLResponse: session status %q: %w", session.Status, ErrInvalidSessionState)
	}

	o.mu.RLock()
	backend, ok := o.backends[sessionID]
	o.mu.RUnlock()

	if !ok {
		return fmt.Errorf("agent.Orchestrator.HandleHITLResponse: %w", ErrSessionNotFound)
	}

	err = backend.SendPrompt(ctx, sessionID, answer)
	if err != nil {
		return fmt.Errorf("agent.Orchestrator.HandleHITLResponse: %w", err)
	}

	err = o.sessions.UpdateStatus(ctx, tenantID, sessionID, domain.AgentStatusRunning)
	if err != nil {
		return fmt.Errorf("agent.Orchestrator.HandleHITLResponse: update status: %w", err)
	}

	return nil
}

// CancelSession cancels an agent session.
func (o *Orchestrator) CancelSession(ctx context.Context, tenantID, sessionID uuid.UUID) error {
	session, err := o.sessions.GetByID(ctx, tenantID, sessionID)
	if err != nil {
		return fmt.Errorf("agent.Orchestrator.CancelSession: %w", err)
	}

	if session.Status == domain.AgentStatusCompleted ||
		session.Status == domain.AgentStatusFailed ||
		session.Status == domain.AgentStatusCancelled {
		return fmt.Errorf("agent.Orchestrator.CancelSession: session in terminal state %q: %w", session.Status, ErrInvalidSessionState)
	}

	o.mu.RLock()
	backend, ok := o.backends[sessionID]
	o.mu.RUnlock()

	if ok {
		err = backend.Cancel(ctx, sessionID)
		if err != nil {
			return fmt.Errorf("agent.Orchestrator.CancelSession: %w", err)
		}
	}

	err = o.sessions.UpdateStatus(ctx, tenantID, sessionID, domain.AgentStatusCancelled)
	if err != nil {
		return fmt.Errorf("agent.Orchestrator.CancelSession: update status: %w", err)
	}

	o.cleanupBackend(sessionID)
	o.cleanupWorktree(ctx, sessionID, tenantID)

	return nil
}

// completeSession handles agent completion (called when container exits).
func (o *Orchestrator) completeSession(ctx context.Context, sessionID, tenantID uuid.UUID, exitErr string) {
	if exitErr == "" {
		if err := o.sessions.SetCompleted(ctx, sessionID, ""); err != nil {
			log.Error().Err(err).Str("session_id", sessionID.String()).Msg("agent.completeSession: failed to set completed")
		}

		// Try to transition task to review.
		session, err := o.sessions.GetByID(ctx, tenantID, sessionID)
		if err == nil && session.TaskID != nil {
			if statusErr := o.tasks.UpdateStatus(ctx, tenantID, *session.TaskID, domain.TaskStatusReview); statusErr != nil {
				log.Error().Err(statusErr).Str("task_id", session.TaskID.String()).Msg("agent.completeSession: failed to update task status")
			}
		}
	} else {
		if err := o.sessions.SetCompleted(ctx, sessionID, exitErr); err != nil {
			log.Error().Err(err).Str("session_id", sessionID.String()).Msg("agent.completeSession: failed to set completed with error")
		}
	}

	o.cleanupBackend(sessionID)
	o.cleanupWorktree(ctx, sessionID, tenantID)

	// Publish completion event.
	evt := map[string]string{
		"type":       "session_completed",
		"session_id": sessionID.String(),
		"error":      exitErr,
	}
	payload, err := json.Marshal(evt)
	if err == nil {
		channel := "agent:" + sessionID.String()
		if pubErr := o.pubsub.Publish(ctx, channel, payload); pubErr != nil {
			log.Error().Err(pubErr).Str("channel", channel).Msg("agent.completeSession: failed to publish completion event")
		}
	}
}

// handleMessage dispatches agent messages to Redis pub/sub.
func (o *Orchestrator) handleMessage(sessionID, tenantID uuid.UUID, msg Message) {
	_ = tenantID // reserved for tenant-scoped routing

	// Skip publish if shutting down.
	select {
	case <-o.done:
		return
	default:
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}

	channel := "agent:" + sessionID.String()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if pubErr := o.pubsub.Publish(ctx, channel, payload); pubErr != nil {
		log.Error().Err(pubErr).Str("channel", channel).Msg("agent.handleMessage: failed to publish message")
	}
}

// waitForCompletion waits for the agent backend to finish (container exit)
// and then completes or fails the session.
func (o *Orchestrator) waitForCompletion(sessionID, tenantID uuid.UUID) {
	// Read the container ID from the session map in the backend.
	// We use WaitContainer on the runtime to detect when the container exits.
	o.mu.RLock()
	_, ok := o.backends[sessionID]
	o.mu.RUnlock()

	if !ok {
		return
	}

	// Poll until backend is gone (session cleaned up) or shutdown signals.
	// The backend's streamOutput goroutine will end when the container exits.
	// We detect completion by waiting for the container.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case <-o.done:
			cancel()
		case <-ctx.Done():
		}
	}()

	// Wait for container ID to be set in DB, with bounded retries.
	var session *domain.AgentSession
	for attempt := range 10 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(100+attempt*100) * time.Millisecond):
		}

		var err error
		session, err = o.sessions.GetByID(ctx, tenantID, sessionID)
		if err != nil {
			o.completeSession(ctx, sessionID, tenantID, "failed to get session for wait: "+err.Error())
			return
		}
		if session.ContainerID != "" {
			break
		}
	}

	if session == nil || session.ContainerID == "" {
		// Container ID not yet set; the backend manages its own container.
		// We need an alternative: wait for the backend's container via direct tracking.
		// For now, we do a lightweight poll.
		o.pollCompletion(ctx, sessionID, tenantID)
		return
	}

	exitCode, err := o.runtime.WaitContainer(ctx, session.ContainerID)
	if err != nil {
		o.completeSession(ctx, sessionID, tenantID, "container wait: "+err.Error())
		return
	}

	if exitCode != 0 {
		o.completeSession(ctx, sessionID, tenantID, fmt.Sprintf("agent exited with code %d", exitCode))
		return
	}

	o.completeSession(ctx, sessionID, tenantID, "")
}

// pollCompletion polls the session status until it reaches a terminal state.
// Used when container ID is not directly tracked in the DB.
func (o *Orchestrator) pollCompletion(ctx context.Context, sessionID, tenantID uuid.UUID) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.mu.RLock()
			_, active := o.backends[sessionID]
			o.mu.RUnlock()

			if !active {
				// Backend was cleaned up (cancel or dispose).
				return
			}

			session, err := o.sessions.GetByID(ctx, tenantID, sessionID)
			if err != nil {
				continue
			}

			switch session.Status {
			case domain.AgentStatusCompleted, domain.AgentStatusFailed, domain.AgentStatusCancelled:
				o.cleanupBackend(sessionID)
				return
			default:
				continue
			}
		}
	}
}

func (o *Orchestrator) failSession(ctx context.Context, sessionID, tenantID uuid.UUID, errMsg string) {
	if err := o.sessions.SetCompleted(ctx, sessionID, errMsg); err != nil {
		log.Error().Err(err).Str("session_id", sessionID.String()).Msg("agent.failSession: failed to set session failed")
	}
	o.cleanupWorktree(ctx, sessionID, tenantID)
}

func (o *Orchestrator) cleanupBackend(sessionID uuid.UUID) {
	o.mu.Lock()
	backend, ok := o.backends[sessionID]
	if ok {
		delete(o.backends, sessionID)
	}
	o.mu.Unlock()

	if ok {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if disposeErr := backend.Dispose(ctx); disposeErr != nil {
			log.Error().Err(disposeErr).Str("session_id", sessionID.String()).Msg("agent.cleanupBackend: failed to dispose backend")
		}
	}
}

// cleanupWorktree removes the git worktree for a completed/cancelled session.
func (o *Orchestrator) cleanupWorktree(ctx context.Context, sessionID, tenantID uuid.UUID) {
	session, err := o.sessions.GetByID(ctx, tenantID, sessionID)
	if err != nil || session.BranchName == "" {
		return
	}

	volumeName := "aira-repo-" + session.ProjectID.String()
	if rmErr := o.volumes.RemoveWorktree(ctx, volumeName, session.BranchName); rmErr != nil {
		log.Error().Err(rmErr).Str("volume", volumeName).Str("branch", session.BranchName).Msg("agent.cleanupWorktree: failed to remove worktree")
	}
}

func buildPrompt(task *domain.Task) string {
	var sb strings.Builder
	sb.WriteString("## Task: ")
	sb.WriteString(task.Title)
	sb.WriteString("\n\n")
	if task.Description != "" {
		sb.WriteString(task.Description)
		sb.WriteString("\n")
	}
	return sb.String()
}
