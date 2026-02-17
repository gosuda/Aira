package notify

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"

	"github.com/gosuda/aira/internal/domain"
	"github.com/gosuda/aira/internal/messenger"
)

// ErrNoMessengerLinks is returned when a user has no linked messenger accounts.
var ErrNoMessengerLinks = errors.New("notify: user has no messenger links") //nolint:gochecknoglobals // sentinel error

// ErrPlatformNotFound is returned when a messenger platform is not registered.
var ErrPlatformNotFound = errors.New("notify: platform not found") //nolint:gochecknoglobals // sentinel error

// MessengerRegistry maps platform names to Messenger implementations.
type MessengerRegistry interface {
	Get(platform string) (messenger.Messenger, bool)
}

// UserLinkResolver finds messenger links for a user.
type UserLinkResolver interface {
	ListMessengerLinks(ctx context.Context, userID uuid.UUID) ([]*domain.UserMessengerLink, error)
}

// Notifier dispatches push notifications to users through their linked messenger accounts.
type Notifier struct {
	messengers MessengerRegistry
	userLinks  UserLinkResolver
}

// New creates a new Notifier with the given messenger registry and user link resolver.
func New(messengers MessengerRegistry, userLinks UserLinkResolver) *Notifier {
	return &Notifier{
		messengers: messengers,
		userLinks:  userLinks,
	}
}

// Notify sends a notification to the user via their first available messenger link.
// Falls back to logging if no links exist.
func (n *Notifier) Notify(ctx context.Context, userID uuid.UUID, message string) error {
	links, err := n.userLinks.ListMessengerLinks(ctx, userID)
	if err != nil {
		return fmt.Errorf("notify.Notifier.Notify: list links: %w", err)
	}

	if len(links) == 0 {
		log.Printf("notify: no messenger links for user %s, message: %s", userID, message)
		return nil
	}

	// Try each link until one succeeds.
	var lastErr error
	for _, link := range links {
		sendErr := n.NotifyVia(ctx, link.Platform, link.ExternalID, message)
		if sendErr == nil {
			return nil
		}
		lastErr = sendErr
	}

	return fmt.Errorf("notify.Notifier.Notify: all links failed: %w", lastErr)
}

// NotifyVia sends a notification using a specific platform and external ID directly.
func (n *Notifier) NotifyVia(ctx context.Context, platform, externalID, message string) error {
	msg, ok := n.messengers.Get(platform)
	if !ok {
		return fmt.Errorf("notify.Notifier.NotifyVia: platform %q: %w", platform, ErrPlatformNotFound)
	}

	if err := msg.SendNotification(ctx, externalID, message); err != nil {
		return fmt.Errorf("notify.Notifier.NotifyVia: send: %w", err)
	}

	return nil
}
