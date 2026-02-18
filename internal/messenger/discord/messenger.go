package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/gosuda/aira/internal/messenger"
)

// DiscordAPI abstracts the subset of the Discord client used by DiscordMessenger.
// This allows testing without real HTTP calls.
type DiscordAPI interface {
	ChannelMessageSend(channelID, content string) (messageID string, err error)
	ChannelMessageEdit(channelID, messageID, content string) error
	MessageThreadStartComplex(channelID, messageID string, threadName string) (threadID string, err error)
	ChannelMessageSendComplex(channelID string, content string, threadID string) (messageID string, err error)
	UserChannelCreate(userID string) (channelID string, err error)
}

// DiscordMessenger implements messenger.Messenger for Discord.
type DiscordMessenger struct {
	api DiscordAPI
}

// Compile-time interface check.
var _ messenger.Messenger = (*DiscordMessenger)(nil) //nolint:gochecknoglobals // compile-time check

// NewDiscordMessenger creates a DiscordMessenger with the given API client.
func NewDiscordMessenger(api DiscordAPI) *DiscordMessenger {
	return &DiscordMessenger{api: api}
}

// SendMessage posts a text message to a Discord channel and returns the message ID.
func (m *DiscordMessenger) SendMessage(_ context.Context, channelID, text string) (messenger.MessageID, error) {
	msgID, err := m.api.ChannelMessageSend(channelID, text)
	if err != nil {
		return "", fmt.Errorf("discord.DiscordMessenger.SendMessage: %w", err)
	}

	return messenger.MessageID(msgID), nil
}

// CreateThread starts a new Discord thread under a parent message. If options are provided,
// they are formatted as a numbered list in the thread message.
func (m *DiscordMessenger) CreateThread(_ context.Context, channelID string, parentID messenger.MessageID, text string, options []messenger.QuestionOption) (messenger.ThreadID, error) {
	threadName := text
	if len(threadName) > 100 {
		threadName = threadName[:100]
	}

	threadID, err := m.api.MessageThreadStartComplex(channelID, string(parentID), threadName)
	if err != nil {
		return "", fmt.Errorf("discord.DiscordMessenger.CreateThread: %w", err)
	}

	var b strings.Builder
	b.WriteString(text)
	if len(options) > 0 {
		b.WriteString("\n")
		for i, opt := range options {
			fmt.Fprintf(&b, "\n%d. %s", i+1, opt.Label)
		}
	}
	body := b.String()

	_, err = m.api.ChannelMessageSendComplex(channelID, body, threadID)
	if err != nil {
		return "", fmt.Errorf("discord.DiscordMessenger.CreateThread: %w", err)
	}

	return messenger.ThreadID(threadID), nil
}

// UpdateMessage edits an existing Discord message.
func (m *DiscordMessenger) UpdateMessage(_ context.Context, channelID string, messageID messenger.MessageID, text string) error {
	err := m.api.ChannelMessageEdit(channelID, string(messageID), text)
	if err != nil {
		return fmt.Errorf("discord.DiscordMessenger.UpdateMessage: %w", err)
	}

	return nil
}

// SendNotification sends a direct message to a Discord user.
// It first opens a DM channel, then sends the message.
func (m *DiscordMessenger) SendNotification(_ context.Context, userExternalID, text string) error {
	dmChannelID, err := m.api.UserChannelCreate(userExternalID)
	if err != nil {
		return fmt.Errorf("discord.DiscordMessenger.SendNotification: %w", err)
	}

	_, err = m.api.ChannelMessageSend(dmChannelID, text)
	if err != nil {
		return fmt.Errorf("discord.DiscordMessenger.SendNotification: %w", err)
	}

	return nil
}

// Platform returns the messenger platform identifier.
func (m *DiscordMessenger) Platform() string {
	return "discord"
}
