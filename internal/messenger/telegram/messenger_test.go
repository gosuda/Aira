package telegram_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gosuda/aira/internal/messenger"
	airatelegram "github.com/gosuda/aira/internal/messenger/telegram"
)

// --- mock TelegramAPI ---

type mockTelegramAPI struct {
	// SendMessage
	sendChatID string
	sendText   string
	sendMsgID  string
	sendErr    error

	// EditMessageText
	editChatID string
	editMsgID  string
	editText   string
	editErr    error

	// SendReply
	replyChatID   string
	replyToMsgID  string
	replyText     string
	replyResultID string
	replyErr      error
}

func (m *mockTelegramAPI) SendMessage(chatID, text string) (string, error) {
	m.sendChatID = chatID
	m.sendText = text
	if m.sendErr != nil {
		return "", m.sendErr
	}
	return m.sendMsgID, nil
}

func (m *mockTelegramAPI) EditMessageText(chatID, messageID, text string) error {
	m.editChatID = chatID
	m.editMsgID = messageID
	m.editText = text
	return m.editErr
}

func (m *mockTelegramAPI) SendReply(chatID, replyToMessageID, text string) (string, error) {
	m.replyChatID = chatID
	m.replyToMsgID = replyToMessageID
	m.replyText = text
	if m.replyErr != nil {
		return "", m.replyErr
	}
	return m.replyResultID, nil
}

// --- TelegramMessenger tests ---

func TestTelegramMessenger_SendMessage(t *testing.T) {
	t.Parallel()

	t.Run("success returns MessageID", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockTelegramAPI{sendMsgID: "42"}
		m := airatelegram.NewTelegramMessenger(api)

		msgID, err := m.SendMessage(ctx, "chat-123", "hello world")

		require.NoError(t, err)
		assert.Equal(t, messenger.MessageID("42"), msgID)
		assert.Equal(t, "chat-123", api.sendChatID)
		assert.Equal(t, "hello world", api.sendText)
	})

	t.Run("error wraps Telegram API error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockTelegramAPI{sendErr: errors.New("chat_not_found")}
		m := airatelegram.NewTelegramMessenger(api)

		msgID, err := m.SendMessage(ctx, "chat-999", "hello")

		require.Error(t, err)
		assert.Empty(t, msgID)
		assert.Contains(t, err.Error(), "telegram.TelegramMessenger.SendMessage")
		assert.Contains(t, err.Error(), "chat_not_found")
	})
}

func TestTelegramMessenger_CreateThread(t *testing.T) {
	t.Parallel()

	t.Run("success with no options sends plain reply", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockTelegramAPI{replyResultID: "reply-001"}
		m := airatelegram.NewTelegramMessenger(api)

		threadID, err := m.CreateThread(ctx, "chat-123", messenger.MessageID("msg-parent"), "pick a database", nil)

		require.NoError(t, err)
		assert.Equal(t, messenger.ThreadID("reply-001"), threadID)
		assert.Equal(t, "chat-123", api.replyChatID)
		assert.Equal(t, "msg-parent", api.replyToMsgID)
		assert.Equal(t, "pick a database", api.replyText)
	})

	t.Run("success with options formats numbered list", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockTelegramAPI{replyResultID: "reply-002"}
		m := airatelegram.NewTelegramMessenger(api)

		opts := []messenger.QuestionOption{
			{Label: "PostgreSQL", Value: "pg"},
			{Label: "MySQL", Value: "mysql"},
		}

		threadID, err := m.CreateThread(ctx, "chat-123", messenger.MessageID("msg-parent"), "pick a database", opts)

		require.NoError(t, err)
		assert.Equal(t, messenger.ThreadID("reply-002"), threadID)
		assert.Contains(t, api.replyText, "pick a database")
		assert.Contains(t, api.replyText, "1. PostgreSQL")
		assert.Contains(t, api.replyText, "2. MySQL")
	})

	t.Run("error wraps Telegram API error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockTelegramAPI{replyErr: errors.New("reply_failed")}
		m := airatelegram.NewTelegramMessenger(api)

		threadID, err := m.CreateThread(ctx, "chat-123", messenger.MessageID("msg-parent"), "question", nil)

		require.Error(t, err)
		assert.Empty(t, threadID)
		assert.Contains(t, err.Error(), "telegram.TelegramMessenger.CreateThread")
		assert.Contains(t, err.Error(), "reply_failed")
	})
}

func TestTelegramMessenger_UpdateMessage(t *testing.T) {
	t.Parallel()

	t.Run("success calls EditMessageText with correct params", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockTelegramAPI{}
		m := airatelegram.NewTelegramMessenger(api)

		err := m.UpdateMessage(ctx, "chat-123", messenger.MessageID("42"), "updated text")

		require.NoError(t, err)
		assert.Equal(t, "chat-123", api.editChatID)
		assert.Equal(t, "42", api.editMsgID)
		assert.Equal(t, "updated text", api.editText)
	})

	t.Run("error wraps Telegram API error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockTelegramAPI{editErr: errors.New("cant_edit_message")}
		m := airatelegram.NewTelegramMessenger(api)

		err := m.UpdateMessage(ctx, "chat-123", messenger.MessageID("42"), "new text")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "telegram.TelegramMessenger.UpdateMessage")
		assert.Contains(t, err.Error(), "cant_edit_message")
	})
}

func TestTelegramMessenger_SendNotification(t *testing.T) {
	t.Parallel()

	t.Run("success sends message directly to user chat ID", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockTelegramAPI{sendMsgID: "notif-001"}
		m := airatelegram.NewTelegramMessenger(api)

		err := m.SendNotification(ctx, "user-123", "you have a new task")

		require.NoError(t, err)
		assert.Equal(t, "user-123", api.sendChatID)
		assert.Equal(t, "you have a new task", api.sendText)
	})

	t.Run("error wraps Telegram API error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		api := &mockTelegramAPI{sendErr: errors.New("user_not_found")}
		m := airatelegram.NewTelegramMessenger(api)

		err := m.SendNotification(ctx, "user-999", "notification")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "telegram.TelegramMessenger.SendNotification")
		assert.Contains(t, err.Error(), "user_not_found")
	})
}

func TestTelegramMessenger_Platform(t *testing.T) {
	t.Parallel()

	api := &mockTelegramAPI{}
	m := airatelegram.NewTelegramMessenger(api)

	assert.Equal(t, "telegram", m.Platform())
}
