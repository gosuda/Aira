package messenger_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gosuda/aira/internal/domain"
	"github.com/gosuda/aira/internal/messenger"
)

// --- mock Messenger ---

type mockMessenger struct {
	sendMsgID  messenger.MessageID
	sendMsgErr error
	threadID   messenger.ThreadID
	threadErr  error
	platform   string
}

func (m *mockMessenger) SendMessage(_ context.Context, _, _ string) (messenger.MessageID, error) {
	if m.sendMsgErr != nil {
		return "", m.sendMsgErr
	}
	return m.sendMsgID, nil
}

func (m *mockMessenger) CreateThread(_ context.Context, _ string, _ messenger.MessageID, _ string, _ []messenger.QuestionOption) (messenger.ThreadID, error) {
	if m.threadErr != nil {
		return "", m.threadErr
	}
	return m.threadID, nil
}

func (m *mockMessenger) UpdateMessage(context.Context, string, messenger.MessageID, string) error {
	return nil
}

func (m *mockMessenger) SendNotification(context.Context, string, string) error { return nil }

func (m *mockMessenger) Platform() string { return m.platform }

// --- mock HITLQuestionRepository ---

type mockHITLQuestionRepo struct {
	createErr error
	created   []*domain.HITLQuestion

	getByThreadResult *domain.HITLQuestion
	getByThreadErr    error

	answerErr error

	listExpiredResult []*domain.HITLQuestion
	listExpiredErr    error

	cancelErr    error
	cancelledIDs []uuid.UUID
}

func (m *mockHITLQuestionRepo) Create(_ context.Context, q *domain.HITLQuestion) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = append(m.created, q)
	return nil
}

func (m *mockHITLQuestionRepo) GetByThreadID(_ context.Context, _ uuid.UUID, _, _ string) (*domain.HITLQuestion, error) {
	if m.getByThreadErr != nil {
		return nil, m.getByThreadErr
	}
	return m.getByThreadResult, nil
}

func (m *mockHITLQuestionRepo) Answer(_ context.Context, _, _ uuid.UUID, _ string, _ uuid.UUID) error {
	return m.answerErr
}

func (m *mockHITLQuestionRepo) ListExpired(context.Context) ([]*domain.HITLQuestion, error) {
	if m.listExpiredErr != nil {
		return nil, m.listExpiredErr
	}
	return m.listExpiredResult, nil
}

func (m *mockHITLQuestionRepo) Cancel(_ context.Context, _, id uuid.UUID) error {
	if m.cancelErr != nil {
		return m.cancelErr
	}
	m.cancelledIDs = append(m.cancelledIDs, id)
	return nil
}

// --- AskQuestion tests ---

func TestAskQuestion(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	tenantID := uuid.New()
	projectID := uuid.New()
	session := &domain.AgentSession{
		ID:        sessionID,
		TenantID:  tenantID,
		ProjectID: projectID,
	}

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		msg := &mockMessenger{
			sendMsgID: "msg-123",
			threadID:  "thread-456",
			platform:  "slack",
		}
		repo := &mockHITLQuestionRepo{}
		callbackFn := func(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }

		router := messenger.NewRouter(repo, msg, callbackFn)

		q, err := router.AskQuestion(ctx, tenantID, session, "general", "Which database?", []string{"pg", "mysql"}, 5*time.Minute)

		require.NoError(t, err)
		require.NotNil(t, q)
		assert.Equal(t, tenantID, q.TenantID)
		assert.Equal(t, sessionID, q.AgentSessionID)
		assert.Equal(t, "Which database?", q.Question)
		assert.Equal(t, []string{"pg", "mysql"}, q.Options)
		assert.Equal(t, "thread-456", q.MessengerThreadID)
		assert.Equal(t, "slack", q.MessengerPlatform)
		assert.Equal(t, domain.HITLStatusPending, q.Status)
		assert.NotNil(t, q.TimeoutAt)
		require.Len(t, repo.created, 1)
	})

	t.Run("zero timeout sets nil TimeoutAt", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		msg := &mockMessenger{sendMsgID: "msg-1", threadID: "thread-1", platform: "slack"}
		repo := &mockHITLQuestionRepo{}
		callbackFn := func(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }

		router := messenger.NewRouter(repo, msg, callbackFn)

		q, err := router.AskQuestion(ctx, tenantID, session, "ch", "question?", nil, 0)

		require.NoError(t, err)
		assert.Nil(t, q.TimeoutAt)
	})

	t.Run("SendMessage error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		msg := &mockMessenger{sendMsgErr: errors.New("send failed")}
		repo := &mockHITLQuestionRepo{}
		callbackFn := func(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }

		router := messenger.NewRouter(repo, msg, callbackFn)

		q, err := router.AskQuestion(ctx, tenantID, session, "ch", "question?", nil, time.Minute)

		require.Error(t, err)
		assert.Nil(t, q)
		assert.Contains(t, err.Error(), "send message")
	})

	t.Run("CreateThread error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		msg := &mockMessenger{
			sendMsgID: "msg-1",
			threadErr: errors.New("thread failed"),
		}
		repo := &mockHITLQuestionRepo{}
		callbackFn := func(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }

		router := messenger.NewRouter(repo, msg, callbackFn)

		q, err := router.AskQuestion(ctx, tenantID, session, "ch", "question?", nil, time.Minute)

		require.Error(t, err)
		assert.Nil(t, q)
		assert.Contains(t, err.Error(), "create thread")
	})

	t.Run("Create error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		msg := &mockMessenger{sendMsgID: "msg-1", threadID: "thread-1", platform: "slack"}
		repo := &mockHITLQuestionRepo{createErr: errors.New("db error")}
		callbackFn := func(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }

		router := messenger.NewRouter(repo, msg, callbackFn)

		q, err := router.AskQuestion(ctx, tenantID, session, "ch", "question?", nil, time.Minute)

		require.Error(t, err)
		assert.Nil(t, q)
		assert.Contains(t, err.Error(), "create question")
	})
}

// --- HandleResponse tests ---

func TestHandleResponse(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	sessionID := uuid.New()
	questionID := uuid.New()
	answeredByID := uuid.New()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		var callbackCalled bool
		callbackFn := func(_ context.Context, gotSessionID, gotTenantID uuid.UUID, gotAnswer string) error {
			callbackCalled = true
			assert.Equal(t, sessionID, gotSessionID)
			assert.Equal(t, tenantID, gotTenantID)
			assert.Equal(t, "use PostgreSQL", gotAnswer)
			return nil
		}

		repo := &mockHITLQuestionRepo{
			getByThreadResult: &domain.HITLQuestion{
				ID:             questionID,
				TenantID:       tenantID,
				AgentSessionID: sessionID,
				Status:         domain.HITLStatusPending,
			},
		}
		msg := &mockMessenger{platform: "slack"}

		router := messenger.NewRouter(repo, msg, callbackFn)

		err := router.HandleResponse(ctx, tenantID, "slack", "thread-1", "use PostgreSQL", answeredByID)

		require.NoError(t, err)
		assert.True(t, callbackCalled)
	})

	t.Run("GetByThreadID error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		callbackFn := func(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }
		repo := &mockHITLQuestionRepo{getByThreadErr: errors.New("not found")}
		msg := &mockMessenger{platform: "slack"}

		router := messenger.NewRouter(repo, msg, callbackFn)

		err := router.HandleResponse(ctx, tenantID, "slack", "thread-1", "answer", answeredByID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "get by thread")
	})

	t.Run("already answered", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		callbackFn := func(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }
		repo := &mockHITLQuestionRepo{
			getByThreadResult: &domain.HITLQuestion{
				ID:     questionID,
				Status: domain.HITLStatusAnswered,
			},
		}
		msg := &mockMessenger{platform: "slack"}

		router := messenger.NewRouter(repo, msg, callbackFn)

		err := router.HandleResponse(ctx, tenantID, "slack", "thread-1", "answer", answeredByID)

		require.Error(t, err)
		assert.ErrorIs(t, err, messenger.ErrQuestionAlreadyAnswered)
	})

	t.Run("Answer error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		callbackFn := func(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }
		repo := &mockHITLQuestionRepo{
			getByThreadResult: &domain.HITLQuestion{
				ID:     questionID,
				Status: domain.HITLStatusPending,
			},
			answerErr: errors.New("db failure"),
		}
		msg := &mockMessenger{platform: "slack"}

		router := messenger.NewRouter(repo, msg, callbackFn)

		err := router.HandleResponse(ctx, tenantID, "slack", "thread-1", "answer", answeredByID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "record answer")
	})

	t.Run("callback error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		callbackFn := func(context.Context, uuid.UUID, uuid.UUID, string) error {
			return errors.New("callback failed")
		}
		repo := &mockHITLQuestionRepo{
			getByThreadResult: &domain.HITLQuestion{
				ID:             questionID,
				TenantID:       tenantID,
				AgentSessionID: sessionID,
				Status:         domain.HITLStatusPending,
			},
		}
		msg := &mockMessenger{platform: "slack"}

		router := messenger.NewRouter(repo, msg, callbackFn)

		err := router.HandleResponse(ctx, tenantID, "slack", "thread-1", "answer", answeredByID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "resume session")
	})
}

// --- processExpiredQuestions tests ---
// processExpiredQuestions is unexported, tested indirectly via StartTimeoutWatcher.

func TestStartTimeoutWatcher_ProcessesExpiredQuestions(t *testing.T) {
	t.Parallel()

	q1ID := uuid.New()
	q2ID := uuid.New()
	tenant1 := uuid.New()
	tenant2 := uuid.New()

	t.Run("no expired questions", func(t *testing.T) {
		t.Parallel()

		repo := &mockHITLQuestionRepo{listExpiredResult: nil}
		msg := &mockMessenger{platform: "slack"}
		callbackFn := func(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }

		// Use a very short poll interval so the watcher fires quickly.
		router := messenger.NewRouter(repo, msg, callbackFn, messenger.WithPollInterval(10*time.Millisecond))

		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		router.StartTimeoutWatcher(ctx)

		// No cancellations should have happened.
		assert.Empty(t, repo.cancelledIDs)
	})

	t.Run("expired questions get cancelled", func(t *testing.T) {
		t.Parallel()

		repo := &mockHITLQuestionRepo{
			listExpiredResult: []*domain.HITLQuestion{
				{ID: q1ID, TenantID: tenant1, Question: "q1"},
				{ID: q2ID, TenantID: tenant2, Question: "q2"},
			},
		}
		msg := &mockMessenger{platform: "slack"}
		callbackFn := func(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }

		router := messenger.NewRouter(repo, msg, callbackFn, messenger.WithPollInterval(10*time.Millisecond))

		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		router.StartTimeoutWatcher(ctx)

		// At least one tick should have fired and cancelled both questions.
		assert.Contains(t, repo.cancelledIDs, q1ID)
		assert.Contains(t, repo.cancelledIDs, q2ID)
	})

	t.Run("cancel error continues to next", func(t *testing.T) {
		t.Parallel()

		// cancelErr affects ALL cancels, but the loop continues (logs + continues).
		repo := &mockHITLQuestionRepo{
			listExpiredResult: []*domain.HITLQuestion{
				{ID: q1ID, TenantID: tenant1, Question: "q1"},
				{ID: q2ID, TenantID: tenant2, Question: "q2"},
			},
			cancelErr: errors.New("cancel failed"),
		}
		msg := &mockMessenger{platform: "slack"}
		callbackFn := func(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }

		router := messenger.NewRouter(repo, msg, callbackFn, messenger.WithPollInterval(10*time.Millisecond))

		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		// Should not panic even though Cancel returns errors.
		router.StartTimeoutWatcher(ctx)

		// No IDs should be in cancelledIDs because all cancels failed.
		assert.Empty(t, repo.cancelledIDs)
	})
}
