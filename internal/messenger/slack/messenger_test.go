package slack_test

import (
	"errors"
	"testing"

	slacklib "github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gosuda/aira/internal/messenger"
	airaslack "github.com/gosuda/aira/internal/messenger/slack"
)

// --- mock SlackAPI ---

type mockSlackAPI struct {
	postMsgChannel string
	postMsgTS      string
	postMsgErr     error
	postMsgOpts    []slacklib.MsgOption

	updateChannel  string
	updateTS       string
	updateErr      error
	updateRespCh   string
	updateRespTS   string
	updateRespText string

	ephemeralChannel string
	ephemeralUser    string
	ephemeralTS      string
	ephemeralErr     error
}

func (m *mockSlackAPI) PostMessage(channelID string, options ...slacklib.MsgOption) (ch, ts string, err error) {
	m.postMsgChannel = channelID
	m.postMsgOpts = options
	if m.postMsgErr != nil {
		return "", "", m.postMsgErr
	}
	return m.postMsgChannel, m.postMsgTS, nil
}

func (m *mockSlackAPI) UpdateMessage(channelID, timestamp string, _ ...slacklib.MsgOption) (ch, ts, text string, err error) {
	m.updateChannel = channelID
	m.updateTS = timestamp
	if m.updateErr != nil {
		return "", "", "", m.updateErr
	}
	return m.updateRespCh, m.updateRespTS, m.updateRespText, nil
}

func (m *mockSlackAPI) PostEphemeral(channelID, userID string, _ ...slacklib.MsgOption) (string, error) {
	m.ephemeralChannel = channelID
	m.ephemeralUser = userID
	if m.ephemeralErr != nil {
		return "", m.ephemeralErr
	}
	return m.ephemeralTS, nil
}

// --- SlackMessenger tests ---

func TestSlackMessenger_SendMessage(t *testing.T) {
	t.Parallel()

	t.Run("success returns message timestamp as MessageID", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockSlackAPI{postMsgTS: "1234567890.123456"}
		m := airaslack.NewSlackMessenger(api)

		msgID, err := m.SendMessage(ctx, "C123", "hello world")

		require.NoError(t, err)
		assert.Equal(t, messenger.MessageID("1234567890.123456"), msgID)
		assert.Equal(t, "C123", api.postMsgChannel)
	})

	t.Run("error wraps Slack API error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockSlackAPI{postMsgErr: errors.New("channel_not_found")}
		m := airaslack.NewSlackMessenger(api)

		msgID, err := m.SendMessage(ctx, "C999", "hello")

		require.Error(t, err)
		assert.Empty(t, msgID)
		assert.Contains(t, err.Error(), "slack.SlackMessenger.SendMessage")
		assert.Contains(t, err.Error(), "channel_not_found")
	})
}

func TestSlackMessenger_CreateThread(t *testing.T) {
	t.Parallel()

	t.Run("success with no options sends threaded reply", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockSlackAPI{postMsgTS: "1234567890.654321"}
		m := airaslack.NewSlackMessenger(api)

		threadID, err := m.CreateThread(ctx, "C123", messenger.MessageID("1234567890.000000"), "pick a database", nil)

		require.NoError(t, err)
		assert.Equal(t, messenger.ThreadID("1234567890.654321"), threadID)
		assert.Equal(t, "C123", api.postMsgChannel)
	})

	t.Run("success with options sends Block Kit buttons", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockSlackAPI{postMsgTS: "1234567890.789012"}
		m := airaslack.NewSlackMessenger(api)

		opts := []messenger.QuestionOption{
			{Label: "PostgreSQL", Value: "pg"},
			{Label: "MySQL", Value: "mysql"},
		}

		threadID, err := m.CreateThread(ctx, "C123", messenger.MessageID("1234567890.000000"), "pick a database", opts)

		require.NoError(t, err)
		assert.Equal(t, messenger.ThreadID("1234567890.789012"), threadID)
	})

	t.Run("error wraps Slack API error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockSlackAPI{postMsgErr: errors.New("thread_not_found")}
		m := airaslack.NewSlackMessenger(api)

		threadID, err := m.CreateThread(ctx, "C123", messenger.MessageID("1.0"), "question", nil)

		require.Error(t, err)
		assert.Empty(t, threadID)
		assert.Contains(t, err.Error(), "slack.SlackMessenger.CreateThread")
	})
}

func TestSlackMessenger_UpdateMessage(t *testing.T) {
	t.Parallel()

	t.Run("success calls UpdateMessage with correct params", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockSlackAPI{updateRespCh: "C123", updateRespTS: "1.0", updateRespText: "updated"}
		m := airaslack.NewSlackMessenger(api)

		err := m.UpdateMessage(ctx, "C123", messenger.MessageID("1234567890.000000"), "new text")

		require.NoError(t, err)
		assert.Equal(t, "C123", api.updateChannel)
		assert.Equal(t, "1234567890.000000", api.updateTS)
	})

	t.Run("error wraps Slack API error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockSlackAPI{updateErr: errors.New("cant_update_message")}
		m := airaslack.NewSlackMessenger(api)

		err := m.UpdateMessage(ctx, "C123", messenger.MessageID("1.0"), "new text")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "slack.SlackMessenger.UpdateMessage")
	})
}

func TestSlackMessenger_SendNotification(t *testing.T) {
	t.Parallel()

	t.Run("success calls PostEphemeral", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockSlackAPI{ephemeralTS: "1234567890.111111"}
		m := airaslack.NewSlackMessenger(api)

		err := m.SendNotification(ctx, "U123", "you have a new task")

		require.NoError(t, err)
		assert.Equal(t, "U123", api.ephemeralUser)
	})

	t.Run("error wraps Slack API error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockSlackAPI{ephemeralErr: errors.New("user_not_found")}
		m := airaslack.NewSlackMessenger(api)

		err := m.SendNotification(ctx, "U999", "notification")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "slack.SlackMessenger.SendNotification")
	})
}

func TestSlackMessenger_Platform(t *testing.T) {
	t.Parallel()

	api := &mockSlackAPI{}
	m := airaslack.NewSlackMessenger(api)

	assert.Equal(t, "slack", m.Platform())
}
