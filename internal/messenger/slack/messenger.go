package slack

import (
	"context"
	"fmt"

	slacklib "github.com/slack-go/slack"

	"github.com/gosuda/aira/internal/messenger"
)

// SlackAPI abstracts the subset of the Slack client used by SlackMessenger.
// This allows testing without real HTTP calls.
type SlackAPI interface {
	PostMessage(channelID string, options ...slacklib.MsgOption) (string, string, error)
	UpdateMessage(channelID, timestamp string, options ...slacklib.MsgOption) (string, string, string, error)
	PostEphemeral(channelID, userID string, options ...slacklib.MsgOption) (string, error)
}

// SlackMessenger implements messenger.Messenger for Slack.
type SlackMessenger struct {
	api SlackAPI
}

// Compile-time interface check.
var _ messenger.Messenger = (*SlackMessenger)(nil) //nolint:gochecknoglobals // compile-time check

// NewSlackMessenger creates a SlackMessenger with the given API client.
func NewSlackMessenger(api SlackAPI) *SlackMessenger {
	return &SlackMessenger{api: api}
}

// SendMessage posts a text message to a Slack channel and returns the message timestamp as MessageID.
func (m *SlackMessenger) SendMessage(_ context.Context, channelID, text string) (messenger.MessageID, error) {
	_, ts, err := m.api.PostMessage(channelID, slacklib.MsgOptionText(text, false))
	if err != nil {
		return "", fmt.Errorf("slack.SlackMessenger.SendMessage: %w", err)
	}

	return messenger.MessageID(ts), nil
}

// CreateThread starts a threaded reply under a parent message. If options are provided,
// Block Kit buttons are included in the thread message.
func (m *SlackMessenger) CreateThread(_ context.Context, channelID string, parentID messenger.MessageID, text string, options []messenger.QuestionOption) (messenger.ThreadID, error) {
	msgOpts := []slacklib.MsgOption{
		slacklib.MsgOptionTS(string(parentID)),
		slacklib.MsgOptionText(text, false),
	}

	if len(options) > 0 {
		blocks := BuildQuestionBlocks(text, options)
		msgOpts = append(msgOpts, slacklib.MsgOptionBlocks(blocks...))
	}

	_, ts, err := m.api.PostMessage(channelID, msgOpts...)
	if err != nil {
		return "", fmt.Errorf("slack.SlackMessenger.CreateThread: %w", err)
	}

	return messenger.ThreadID(ts), nil
}

// UpdateMessage edits an existing Slack message.
func (m *SlackMessenger) UpdateMessage(_ context.Context, channelID string, messageID messenger.MessageID, text string) error {
	_, _, _, err := m.api.UpdateMessage(channelID, string(messageID), slacklib.MsgOptionText(text, false))
	if err != nil {
		return fmt.Errorf("slack.SlackMessenger.UpdateMessage: %w", err)
	}

	return nil
}

// SendNotification sends an ephemeral notification to a Slack user.
// For Phase 1C, this posts an ephemeral message to the user directly.
// The userExternalID is used as both the channel and user ID for DM-style ephemeral delivery.
func (m *SlackMessenger) SendNotification(_ context.Context, userExternalID, text string) error {
	_, err := m.api.PostEphemeral(userExternalID, userExternalID, slacklib.MsgOptionText(text, false))
	if err != nil {
		return fmt.Errorf("slack.SlackMessenger.SendNotification: %w", err)
	}

	return nil
}

// Platform returns the messenger platform identifier.
func (m *SlackMessenger) Platform() string {
	return "slack"
}
