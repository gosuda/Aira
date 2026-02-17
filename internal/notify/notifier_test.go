package notify_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gosuda/aira/internal/domain"
	"github.com/gosuda/aira/internal/messenger"
	"github.com/gosuda/aira/internal/notify"
)

// --- mocks ---

type mockMessenger struct {
	platform      string
	notifications []sentNotification
	notifyErr     error
}

type sentNotification struct {
	externalID string
	text       string
}

func (m *mockMessenger) SendMessage(context.Context, string, string) (messenger.MessageID, error) {
	return "", nil
}

func (m *mockMessenger) CreateThread(context.Context, string, messenger.MessageID, string, []messenger.QuestionOption) (messenger.ThreadID, error) {
	return "", nil
}

func (m *mockMessenger) UpdateMessage(context.Context, string, messenger.MessageID, string) error {
	return nil
}

func (m *mockMessenger) SendNotification(_ context.Context, externalID, text string) error {
	if m.notifyErr != nil {
		return m.notifyErr
	}
	m.notifications = append(m.notifications, sentNotification{externalID: externalID, text: text})
	return nil
}

func (m *mockMessenger) Platform() string { return m.platform }

type mockRegistry struct {
	messengers map[string]messenger.Messenger
}

func (r *mockRegistry) Get(platform string) (messenger.Messenger, bool) {
	m, ok := r.messengers[platform]
	return m, ok
}

type mockUserLinks struct {
	links []*domain.UserMessengerLink
	err   error
}

func (m *mockUserLinks) ListMessengerLinks(context.Context, uuid.UUID) ([]*domain.UserMessengerLink, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.links, nil
}

// --- Notify tests ---

func TestNotify(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	t.Run("happy path sends via first available link", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		slackMsg := &mockMessenger{platform: "slack"}
		reg := &mockRegistry{messengers: map[string]messenger.Messenger{"slack": slackMsg}}
		links := &mockUserLinks{
			links: []*domain.UserMessengerLink{
				{Platform: "slack", ExternalID: "U123"},
			},
		}

		n := notify.New(reg, links)
		err := n.Notify(ctx, userID, "hello")

		require.NoError(t, err)
		require.Len(t, slackMsg.notifications, 1)
		assert.Equal(t, "U123", slackMsg.notifications[0].externalID)
		assert.Equal(t, "hello", slackMsg.notifications[0].text)
	})

	t.Run("no links falls back to log without error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		reg := &mockRegistry{messengers: map[string]messenger.Messenger{}}
		links := &mockUserLinks{links: nil}

		n := notify.New(reg, links)
		err := n.Notify(ctx, userID, "hello")

		require.NoError(t, err)
	})

	t.Run("ListMessengerLinks error propagates", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		reg := &mockRegistry{messengers: map[string]messenger.Messenger{}}
		links := &mockUserLinks{err: errors.New("db error")}

		n := notify.New(reg, links)
		err := n.Notify(ctx, userID, "hello")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "list links")
	})

	t.Run("SendNotification failure returns error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		slackMsg := &mockMessenger{platform: "slack", notifyErr: errors.New("api down")}
		reg := &mockRegistry{messengers: map[string]messenger.Messenger{"slack": slackMsg}}
		links := &mockUserLinks{
			links: []*domain.UserMessengerLink{
				{Platform: "slack", ExternalID: "U123"},
			},
		}

		n := notify.New(reg, links)
		err := n.Notify(ctx, userID, "hello")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "all links failed")
	})

	t.Run("falls through to second link on first failure", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		slackMsg := &mockMessenger{platform: "slack", notifyErr: errors.New("slack down")}
		discordMsg := &mockMessenger{platform: "discord"}
		reg := &mockRegistry{messengers: map[string]messenger.Messenger{
			"slack":   slackMsg,
			"discord": discordMsg,
		}}
		links := &mockUserLinks{
			links: []*domain.UserMessengerLink{
				{Platform: "slack", ExternalID: "U123"},
				{Platform: "discord", ExternalID: "D456"},
			},
		}

		n := notify.New(reg, links)
		err := n.Notify(ctx, userID, "hello")

		require.NoError(t, err)
		assert.Empty(t, slackMsg.notifications)
		require.Len(t, discordMsg.notifications, 1)
		assert.Equal(t, "D456", discordMsg.notifications[0].externalID)
	})
}

// --- NotifyVia tests ---

func TestNotifyVia(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		slackMsg := &mockMessenger{platform: "slack"}
		reg := &mockRegistry{messengers: map[string]messenger.Messenger{"slack": slackMsg}}
		links := &mockUserLinks{}

		n := notify.New(reg, links)
		err := n.NotifyVia(ctx, "slack", "U123", "hello")

		require.NoError(t, err)
		require.Len(t, slackMsg.notifications, 1)
		assert.Equal(t, "U123", slackMsg.notifications[0].externalID)
		assert.Equal(t, "hello", slackMsg.notifications[0].text)
	})

	t.Run("unknown platform returns ErrPlatformNotFound", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		reg := &mockRegistry{messengers: map[string]messenger.Messenger{}}
		links := &mockUserLinks{}

		n := notify.New(reg, links)
		err := n.NotifyVia(ctx, "unknown", "U123", "hello")

		require.Error(t, err)
		assert.ErrorIs(t, err, notify.ErrPlatformNotFound)
	})

	t.Run("SendNotification error wraps", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		slackMsg := &mockMessenger{platform: "slack", notifyErr: errors.New("timeout")}
		reg := &mockRegistry{messengers: map[string]messenger.Messenger{"slack": slackMsg}}
		links := &mockUserLinks{}

		n := notify.New(reg, links)
		err := n.NotifyVia(ctx, "slack", "U123", "hello")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "send")
	})
}
