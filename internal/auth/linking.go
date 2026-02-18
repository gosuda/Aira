package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/gosuda/aira/internal/domain"
)

// ErrLinkTokenExpired is returned when a link token is not found or has expired.
var ErrLinkTokenExpired = errors.New("auth: link token expired or not found")

const (
	linkTokenBytes  = 32
	linkTokenExpiry = 15 * time.Minute
)

// LinkToken represents a temporary token for linking a messenger account to a user.
type LinkToken struct {
	Token      string
	TenantID   uuid.UUID
	Platform   string // "slack", "discord", "telegram"
	ExternalID string // messenger user ID
	ExpiresAt  time.Time
}

// GenerateLinkToken creates a cryptographically random link token and stores it
// in memory with a 15-minute expiration.
func (s *Service) GenerateLinkToken(tenantID uuid.UUID, platform, externalID string) (*LinkToken, error) {
	raw := make([]byte, linkTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("auth.GenerateLinkToken: %w", err)
	}

	lt := &LinkToken{
		Token:      hex.EncodeToString(raw),
		TenantID:   tenantID,
		Platform:   platform,
		ExternalID: externalID,
		ExpiresAt:  time.Now().Add(linkTokenExpiry),
	}

	s.linkTokens.Store(lt.Token, lt)

	return lt, nil
}

// VerifyAndLink validates a link token and creates a UserMessengerLink binding.
// The token is consumed (deleted) on successful verification.
func (s *Service) VerifyAndLink(ctx context.Context, token string, userID uuid.UUID) error {
	val, ok := s.linkTokens.LoadAndDelete(token)
	if !ok {
		return fmt.Errorf("auth.VerifyAndLink: %w", ErrLinkTokenExpired)
	}

	lt, ok := val.(*LinkToken)
	if !ok {
		return fmt.Errorf("auth.VerifyAndLink: %w", ErrLinkTokenExpired)
	}

	if time.Now().After(lt.ExpiresAt) {
		return fmt.Errorf("auth.VerifyAndLink: %w", ErrLinkTokenExpired)
	}

	link := &domain.UserMessengerLink{
		ID:         uuid.New(),
		UserID:     userID,
		TenantID:   lt.TenantID,
		Platform:   lt.Platform,
		ExternalID: lt.ExternalID,
		CreatedAt:  time.Now(),
	}

	if err := s.userRepo.CreateMessengerLink(ctx, link); err != nil {
		return fmt.Errorf("auth.VerifyAndLink: %w", err)
	}

	return nil
}
