package notify

import (
	"context"
	"log"

	"github.com/google/uuid"
)

// Notifier dispatches push notifications to users. Currently a placeholder
// that logs notifications; in Phase 1C this will route to the appropriate
// messenger (Slack, Telegram, etc.).
type Notifier struct{}

// New creates a new Notifier.
func New() *Notifier {
	return &Notifier{}
}

// Notify sends a notification to a user. For now, logs the notification.
func (n *Notifier) Notify(_ context.Context, userID uuid.UUID, message string) error {
	log.Printf("notify: user=%s message=%s", userID, message)
	return nil
}
