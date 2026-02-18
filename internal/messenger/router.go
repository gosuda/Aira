package messenger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/google/uuid"

	"github.com/gosuda/aira/internal/domain"
)

// ErrQuestionNotFound is returned when a HITL question cannot be located.
var ErrQuestionNotFound = errors.New("messenger: question not found") //nolint:gochecknoglobals // sentinel error

// ErrQuestionAlreadyAnswered is returned when attempting to answer a question that is not pending.
var ErrQuestionAlreadyAnswered = errors.New("messenger: question already answered") //nolint:gochecknoglobals // sentinel error

// HITLCallback is called when a human answers a HITL question so the orchestrator
// can resume the agent session.
type HITLCallback func(ctx context.Context, sessionID, tenantID uuid.UUID, answer string) error

// EscalationConfig configures who to notify when a HITL question times out.
type EscalationConfig struct {
	Platform  string // messenger platform for escalation
	ChannelID string // channel or user ID to escalate to
	Enabled   bool
}

// Router routes AI-generated questions to messenger threads and collects human responses.
// It bridges the agent orchestrator and the chat platform.
type Router struct {
	questions    HITLQuestionRepository
	messenger    Messenger
	callback     HITLCallback
	pollInterval time.Duration
	escalation   EscalationConfig
}

// HITLQuestionRepository is a subset of domain.HITLQuestionRepository used by the router.
type HITLQuestionRepository interface {
	Create(ctx context.Context, q *domain.HITLQuestion) error
	GetByThreadID(ctx context.Context, tenantID uuid.UUID, platform, threadID string) (*domain.HITLQuestion, error)
	Answer(ctx context.Context, tenantID, id uuid.UUID, answer string, answeredBy uuid.UUID) error
	ListExpired(ctx context.Context) ([]*domain.HITLQuestion, error)
	Cancel(ctx context.Context, tenantID, id uuid.UUID) error
}

// RouterOption configures optional Router parameters.
type RouterOption func(*Router)

// WithPollInterval sets the interval at which the timeout watcher checks for expired questions.
func WithPollInterval(d time.Duration) RouterOption {
	return func(r *Router) {
		r.pollInterval = d
	}
}

// WithEscalation configures timeout escalation notifications.
func WithEscalation(cfg EscalationConfig) RouterOption {
	return func(r *Router) {
		r.escalation = cfg
	}
}

// NewRouter creates a Router with the required dependencies.
func NewRouter(
	questions HITLQuestionRepository,
	msg Messenger,
	callback HITLCallback,
	opts ...RouterOption,
) *Router {
	r := &Router{
		questions:    questions,
		messenger:    msg,
		callback:     callback,
		pollInterval: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// AskQuestion creates a HITL question in the database and sends it to a messenger thread.
// It returns the created question or an error.
func (r *Router) AskQuestion(
	ctx context.Context,
	tenantID uuid.UUID,
	session *domain.AgentSession,
	channelID string,
	question string,
	options []string,
	timeout time.Duration,
) (*domain.HITLQuestion, error) {
	// Build messenger options from string options.
	messengerOpts := make([]QuestionOption, 0, len(options))
	for _, opt := range options {
		messengerOpts = append(messengerOpts, QuestionOption{
			Label: opt,
			Value: opt,
		})
	}

	// Send a parent message, then create a thread for the question.
	parentMsg := fmt.Sprintf("Agent needs input for session `%s`", session.ID.String())
	msgID, err := r.messenger.SendMessage(ctx, channelID, parentMsg)
	if err != nil {
		return nil, fmt.Errorf("messenger.Router.AskQuestion: send message: %w", err)
	}

	threadID, err := r.messenger.CreateThread(ctx, channelID, msgID, question, messengerOpts)
	if err != nil {
		return nil, fmt.Errorf("messenger.Router.AskQuestion: create thread: %w", err)
	}

	// Compute timeout deadline.
	var timeoutAt *time.Time
	if timeout > 0 {
		t := time.Now().Add(timeout)
		timeoutAt = &t
	}

	now := time.Now()
	q := &domain.HITLQuestion{
		ID:                uuid.New(),
		TenantID:          tenantID,
		AgentSessionID:    session.ID,
		Question:          question,
		Options:           options,
		MessengerThreadID: string(threadID),
		MessengerPlatform: r.messenger.Platform(),
		Status:            domain.HITLStatusPending,
		TimeoutAt:         timeoutAt,
		CreatedAt:         now,
	}

	createErr := r.questions.Create(ctx, q)
	if createErr != nil {
		return nil, fmt.Errorf("messenger.Router.AskQuestion: create question: %w", createErr)
	}

	return q, nil
}

// HandleResponse processes a human answer received from the messenger platform.
// It looks up the question by thread ID, records the answer, and calls back to
// the orchestrator to resume the agent.
func (r *Router) HandleResponse(
	ctx context.Context,
	tenantID uuid.UUID,
	platform string,
	threadID string,
	answer string,
	answeredBy uuid.UUID,
) error {
	q, err := r.questions.GetByThreadID(ctx, tenantID, platform, threadID)
	if err != nil {
		return fmt.Errorf("messenger.Router.HandleResponse: get by thread: %w", err)
	}

	if q.Status != domain.HITLStatusPending {
		return fmt.Errorf("messenger.Router.HandleResponse: status %q: %w", q.Status, ErrQuestionAlreadyAnswered)
	}

	answerErr := r.questions.Answer(ctx, tenantID, q.ID, answer, answeredBy)
	if answerErr != nil {
		return fmt.Errorf("messenger.Router.HandleResponse: record answer: %w", answerErr)
	}

	// Resume the agent session via the orchestrator callback.
	callbackErr := r.callback(ctx, q.AgentSessionID, tenantID, answer)
	if callbackErr != nil {
		return fmt.Errorf("messenger.Router.HandleResponse: resume session: %w", callbackErr)
	}

	return nil
}

// StartTimeoutWatcher launches a background goroutine that polls for expired HITL questions
// and cancels them. It blocks until the context is cancelled.
func (r *Router) StartTimeoutWatcher(ctx context.Context) {
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.processExpiredQuestions(ctx)
		}
	}
}

// processExpiredQuestions finds and cancels all expired HITL questions.
func (r *Router) processExpiredQuestions(ctx context.Context) {
	expired, err := r.questions.ListExpired(ctx)
	if err != nil {
		log.Error().Err(err).Msg("list expired questions")
		return
	}

	for _, q := range expired {
		cancelErr := r.questions.Cancel(ctx, q.TenantID, q.ID)
		if cancelErr != nil {
			log.Error().Err(cancelErr).Str("question_id", q.ID.String()).Msg("cancel expired question")
			continue
		}

		// Best-effort: update the original thread with a timeout message.
		timeoutMsg := "Question timed out: " + q.Question
		updateErr := r.messenger.UpdateMessage(ctx, q.MessengerThreadID, MessageID(q.MessengerThreadID), timeoutMsg)
		if updateErr != nil {
			log.Error().Err(updateErr).Str("thread_id", q.MessengerThreadID).Msg("update thread with timeout")
		}

		// Escalation: send notification to configured escalation channel.
		if r.escalation.Enabled {
			escMsg := fmt.Sprintf("HITL question timed out (session %s): %s", q.AgentSessionID, q.Question)
			_, escErr := r.messenger.SendMessage(ctx, r.escalation.ChannelID, escMsg)
			if escErr != nil {
				log.Error().Err(escErr).Str("question_id", q.ID.String()).Msg("escalation failed")
			}
		}

		log.Warn().Str("question_id", q.ID.String()).Str("question", q.Question).Msg("question timed out")
	}
}
