package messenger

import "context"

// MessageID uniquely identifies a message within a messenger platform.
type MessageID string

// ThreadID uniquely identifies a conversation thread within a messenger platform.
type ThreadID string

// QuestionOption represents a structured choice presented to a human in a HITL interaction.
type QuestionOption struct {
	Label string `json:"label"` // display text
	Value string `json:"value"` // machine-readable value returned on selection
}

// Messenger abstracts communication with a chat platform (Slack, Discord, Telegram, etc.).
// Implementations handle platform-specific API calls; the interface is platform-agnostic.
type Messenger interface {
	// SendMessage posts a text message to a channel and returns its platform message ID.
	SendMessage(ctx context.Context, channelID, text string) (MessageID, error)

	// CreateThread starts a new thread under a parent message, optionally presenting
	// structured options for the recipient to choose from.
	CreateThread(ctx context.Context, channelID string, parentID MessageID, text string, options []QuestionOption) (ThreadID, error)

	// UpdateMessage edits an existing message in a channel.
	UpdateMessage(ctx context.Context, channelID string, messageID MessageID, text string) error

	// SendNotification sends a direct/ephemeral notification to a user by their
	// external platform ID (e.g. Slack user ID).
	SendNotification(ctx context.Context, userExternalID, text string) error

	// Platform returns the messenger platform identifier (e.g. "slack", "discord").
	Platform() string
}
