package server

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/gosuda/aira/internal/domain"
	"github.com/gosuda/aira/internal/messenger"
)

// slackResponseAdapter bridges the Slack handler's ResponseHandler interface to
// the messenger.Router's HandleResponse method. It resolves Slack user IDs to
// internal user IDs via messenger links for attribution.
type slackResponseAdapter struct {
	router   *messenger.Router
	userRepo domain.UserRepository
}

// HandleSlackResponse implements slack.ResponseHandler.
func (a *slackResponseAdapter) HandleSlackResponse(ctx context.Context, tenantID uuid.UUID, threadTS, answer, slackUserID string) error {
	// Look up user by Slack external ID to attribute the answer.
	// If the user is not linked, use uuid.Nil (graceful degradation).
	var answeredBy uuid.UUID

	link, err := a.userRepo.GetMessengerLink(ctx, tenantID, "slack", slackUserID)
	if err == nil && link != nil {
		answeredBy = link.UserID
	}

	if routerErr := a.router.HandleResponse(ctx, tenantID, "slack", threadTS, answer, answeredBy); routerErr != nil {
		return fmt.Errorf("slackResponseAdapter.HandleSlackResponse: %w", routerErr)
	}

	return nil
}
