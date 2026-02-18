package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/gosuda/aira/internal/messenger"
)

// TelegramAPI abstracts the subset of the Telegram Bot API used by TelegramMessenger.
// This allows testing without real HTTP calls.
type TelegramAPI interface {
	SendMessage(chatID, text string) (messageID string, err error)
	EditMessageText(chatID, messageID, text string) error
	SendReply(chatID, replyToMessageID, text string) (messageID string, err error)
}

// TelegramMessenger implements messenger.Messenger for Telegram.
type TelegramMessenger struct {
	api TelegramAPI
}

// Compile-time interface check.
var _ messenger.Messenger = (*TelegramMessenger)(nil) //nolint:gochecknoglobals // compile-time check

// NewTelegramMessenger creates a TelegramMessenger with the given API client.
func NewTelegramMessenger(api TelegramAPI) *TelegramMessenger {
	return &TelegramMessenger{api: api}
}

// SendMessage posts a text message to a Telegram chat and returns the message ID.
func (m *TelegramMessenger) SendMessage(_ context.Context, channelID, text string) (messenger.MessageID, error) {
	msgID, err := m.api.SendMessage(channelID, text)
	if err != nil {
		return "", fmt.Errorf("telegram.TelegramMessenger.SendMessage: %w", err)
	}

	return messenger.MessageID(msgID), nil
}

// CreateThread starts a threaded reply under a parent message using Telegram's reply mechanism.
// If options are provided, they are formatted as a numbered list appended to the message text.
func (m *TelegramMessenger) CreateThread(_ context.Context, channelID string, parentID messenger.MessageID, text string, options []messenger.QuestionOption) (messenger.ThreadID, error) {
	var b strings.Builder
	b.WriteString(text)
	if len(options) > 0 {
		b.WriteString("\n")
		for i, opt := range options {
			fmt.Fprintf(&b, "\n%d. %s", i+1, opt.Label)
		}
	}
	body := b.String()

	replyID, err := m.api.SendReply(channelID, string(parentID), body)
	if err != nil {
		return "", fmt.Errorf("telegram.TelegramMessenger.CreateThread: %w", err)
	}

	return messenger.ThreadID(replyID), nil
}

// UpdateMessage edits an existing Telegram message.
func (m *TelegramMessenger) UpdateMessage(_ context.Context, channelID string, messageID messenger.MessageID, text string) error {
	err := m.api.EditMessageText(channelID, string(messageID), text)
	if err != nil {
		return fmt.Errorf("telegram.TelegramMessenger.UpdateMessage: %w", err)
	}

	return nil
}

// SendNotification sends a direct message to a Telegram user.
// Telegram uses the chat ID directly for DMs, so no separate channel creation is needed.
func (m *TelegramMessenger) SendNotification(_ context.Context, userExternalID, text string) error {
	_, err := m.api.SendMessage(userExternalID, text)
	if err != nil {
		return fmt.Errorf("telegram.TelegramMessenger.SendNotification: %w", err)
	}

	return nil
}

// Platform returns the messenger platform identifier.
func (m *TelegramMessenger) Platform() string {
	return "telegram"
}
