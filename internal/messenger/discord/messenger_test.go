package discord_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gosuda/aira/internal/messenger"
	airadiscord "github.com/gosuda/aira/internal/messenger/discord"
)

// --- mock DiscordAPI ---

type mockDiscordAPI struct {
	// ChannelMessageSend
	sendChannel string
	sendContent string
	sendMsgID   string
	sendErr     error

	// ChannelMessageEdit
	editChannel string
	editMsgID   string
	editContent string
	editErr     error

	// MessageThreadStartComplex
	threadChannel  string
	threadMsgID    string
	threadName     string
	threadResultID string
	threadErr      error

	// ChannelMessageSendComplex
	complexChannel  string
	complexContent  string
	complexThreadID string
	complexMsgID    string
	complexErr      error

	// UserChannelCreate
	dmUserID    string
	dmChannelID string
	dmErr       error
}

func (m *mockDiscordAPI) ChannelMessageSend(channelID, content string) (string, error) {
	m.sendChannel = channelID
	m.sendContent = content
	if m.sendErr != nil {
		return "", m.sendErr
	}
	return m.sendMsgID, nil
}

func (m *mockDiscordAPI) ChannelMessageEdit(channelID, messageID, content string) error {
	m.editChannel = channelID
	m.editMsgID = messageID
	m.editContent = content
	return m.editErr
}

func (m *mockDiscordAPI) MessageThreadStartComplex(channelID, messageID, threadName string) (string, error) {
	m.threadChannel = channelID
	m.threadMsgID = messageID
	m.threadName = threadName
	if m.threadErr != nil {
		return "", m.threadErr
	}
	return m.threadResultID, nil
}

func (m *mockDiscordAPI) ChannelMessageSendComplex(channelID, content, threadID string) (string, error) {
	m.complexChannel = channelID
	m.complexContent = content
	m.complexThreadID = threadID
	if m.complexErr != nil {
		return "", m.complexErr
	}
	return m.complexMsgID, nil
}

func (m *mockDiscordAPI) UserChannelCreate(userID string) (string, error) {
	m.dmUserID = userID
	if m.dmErr != nil {
		return "", m.dmErr
	}
	return m.dmChannelID, nil
}

// --- DiscordMessenger tests ---

func TestDiscordMessenger_SendMessage(t *testing.T) {
	t.Parallel()

	t.Run("success returns MessageID", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockDiscordAPI{sendMsgID: "msg-001"}
		m := airadiscord.NewDiscordMessenger(api)

		msgID, err := m.SendMessage(ctx, "ch-123", "hello world")

		require.NoError(t, err)
		assert.Equal(t, messenger.MessageID("msg-001"), msgID)
		assert.Equal(t, "ch-123", api.sendChannel)
		assert.Equal(t, "hello world", api.sendContent)
	})

	t.Run("error wraps Discord API error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockDiscordAPI{sendErr: errors.New("channel_not_found")}
		m := airadiscord.NewDiscordMessenger(api)

		msgID, err := m.SendMessage(ctx, "ch-999", "hello")

		require.Error(t, err)
		assert.Empty(t, msgID)
		assert.Contains(t, err.Error(), "discord.DiscordMessenger.SendMessage")
		assert.Contains(t, err.Error(), "channel_not_found")
	})
}

func TestDiscordMessenger_CreateThread(t *testing.T) {
	t.Parallel()

	t.Run("success with no options creates thread and sends message", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockDiscordAPI{
			threadResultID: "thread-001",
			complexMsgID:   "msg-002",
		}
		m := airadiscord.NewDiscordMessenger(api)

		threadID, err := m.CreateThread(ctx, "ch-123", messenger.MessageID("msg-parent"), "pick a database", nil)

		require.NoError(t, err)
		assert.Equal(t, messenger.ThreadID("thread-001"), threadID)
		assert.Equal(t, "ch-123", api.threadChannel)
		assert.Equal(t, "msg-parent", api.threadMsgID)
		assert.Equal(t, "pick a database", api.threadName)
		assert.Equal(t, "pick a database", api.complexContent)
		assert.Equal(t, "thread-001", api.complexThreadID)
	})

	t.Run("success with options formats numbered list", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockDiscordAPI{
			threadResultID: "thread-002",
			complexMsgID:   "msg-003",
		}
		m := airadiscord.NewDiscordMessenger(api)

		opts := []messenger.QuestionOption{
			{Label: "PostgreSQL", Value: "pg"},
			{Label: "MySQL", Value: "mysql"},
		}

		threadID, err := m.CreateThread(ctx, "ch-123", messenger.MessageID("msg-parent"), "pick a database", opts)

		require.NoError(t, err)
		assert.Equal(t, messenger.ThreadID("thread-002"), threadID)
		assert.Contains(t, api.complexContent, "pick a database")
		assert.Contains(t, api.complexContent, "1. PostgreSQL")
		assert.Contains(t, api.complexContent, "2. MySQL")
	})

	t.Run("thread name truncated to 100 chars", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		longText := "This is a very long question that exceeds the Discord thread name limit of one hundred characters and should be truncated"
		api := &mockDiscordAPI{
			threadResultID: "thread-003",
			complexMsgID:   "msg-004",
		}
		m := airadiscord.NewDiscordMessenger(api)

		_, err := m.CreateThread(ctx, "ch-123", messenger.MessageID("msg-parent"), longText, nil)

		require.NoError(t, err)
		assert.Len(t, api.threadName, 100)
		assert.Equal(t, longText[:100], api.threadName)
		// Full text still sent in the message body.
		assert.Equal(t, longText, api.complexContent)
	})

	t.Run("thread start error wraps", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockDiscordAPI{threadErr: errors.New("cannot_create_thread")}
		m := airadiscord.NewDiscordMessenger(api)

		threadID, err := m.CreateThread(ctx, "ch-123", messenger.MessageID("msg-parent"), "question", nil)

		require.Error(t, err)
		assert.Empty(t, threadID)
		assert.Contains(t, err.Error(), "discord.DiscordMessenger.CreateThread")
		assert.Contains(t, err.Error(), "cannot_create_thread")
	})

	t.Run("message send in thread error wraps", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockDiscordAPI{
			threadResultID: "thread-004",
			complexErr:     errors.New("send_failed"),
		}
		m := airadiscord.NewDiscordMessenger(api)

		threadID, err := m.CreateThread(ctx, "ch-123", messenger.MessageID("msg-parent"), "question", nil)

		require.Error(t, err)
		assert.Empty(t, threadID)
		assert.Contains(t, err.Error(), "discord.DiscordMessenger.CreateThread")
		assert.Contains(t, err.Error(), "send_failed")
	})
}

func TestDiscordMessenger_UpdateMessage(t *testing.T) {
	t.Parallel()

	t.Run("success calls ChannelMessageEdit with correct params", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockDiscordAPI{}
		m := airadiscord.NewDiscordMessenger(api)

		err := m.UpdateMessage(ctx, "ch-123", messenger.MessageID("msg-001"), "updated text")

		require.NoError(t, err)
		assert.Equal(t, "ch-123", api.editChannel)
		assert.Equal(t, "msg-001", api.editMsgID)
		assert.Equal(t, "updated text", api.editContent)
	})

	t.Run("error wraps Discord API error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockDiscordAPI{editErr: errors.New("cant_update_message")}
		m := airadiscord.NewDiscordMessenger(api)

		err := m.UpdateMessage(ctx, "ch-123", messenger.MessageID("msg-001"), "new text")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "discord.DiscordMessenger.UpdateMessage")
		assert.Contains(t, err.Error(), "cant_update_message")
	})
}

func TestDiscordMessenger_SendNotification(t *testing.T) {
	t.Parallel()

	t.Run("success creates DM channel then sends message", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockDiscordAPI{
			dmChannelID: "dm-ch-001",
			sendMsgID:   "msg-notif-001",
		}
		m := airadiscord.NewDiscordMessenger(api)

		err := m.SendNotification(ctx, "user-123", "you have a new task")

		require.NoError(t, err)
		assert.Equal(t, "user-123", api.dmUserID)
		assert.Equal(t, "dm-ch-001", api.sendChannel)
		assert.Equal(t, "you have a new task", api.sendContent)
	})

	t.Run("DM channel creation error wraps", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockDiscordAPI{dmErr: errors.New("user_not_found")}
		m := airadiscord.NewDiscordMessenger(api)

		err := m.SendNotification(ctx, "user-999", "notification")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "discord.DiscordMessenger.SendNotification")
		assert.Contains(t, err.Error(), "user_not_found")
	})

	t.Run("message send error wraps", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockDiscordAPI{
			dmChannelID: "dm-ch-002",
			sendErr:     errors.New("cannot_send_dm"),
		}
		m := airadiscord.NewDiscordMessenger(api)

		err := m.SendNotification(ctx, "user-123", "notification")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "discord.DiscordMessenger.SendNotification")
		assert.Contains(t, err.Error(), "cannot_send_dm")
	})
}

func TestDiscordMessenger_Platform(t *testing.T) {
	t.Parallel()

	api := &mockDiscordAPI{}
	m := airadiscord.NewDiscordMessenger(api)

	assert.Equal(t, "discord", m.Platform())
}
