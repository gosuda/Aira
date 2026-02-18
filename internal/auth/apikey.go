package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/gosuda/aira/internal/domain"
)

// ErrInvalidAPIKey is returned when an API key is not found or the hash does not match.
var ErrInvalidAPIKey = errors.New("auth: invalid API key")

const (
	apiKeyPrefix    = "aira_"
	apiKeyRandLen   = 16 // 16 bytes = 32 hex chars
	apiKeyPrefixLen = 8  // first 8 chars of the full key used for lookup
)

// GenerateAPIKey creates a new API key, stores the SHA-256 hash, and returns
// the raw key (shown to the user once). Key format: "aira_" + 32 random hex chars.
func (s *Service) GenerateAPIKey(ctx context.Context, tenantID, userID uuid.UUID, name string, scopes []string) (string, *domain.APIKey, error) {
	raw := make([]byte, apiKeyRandLen)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("auth.GenerateAPIKey: %w", err)
	}

	rawKey := apiKeyPrefix + hex.EncodeToString(raw)

	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	prefix := rawKey[:apiKeyPrefixLen]

	now := time.Now()
	key := &domain.APIKey{
		ID:        uuid.New(),
		TenantID:  tenantID,
		UserID:    userID,
		Name:      name,
		KeyHash:   keyHash,
		Prefix:    prefix,
		Scopes:    scopes,
		CreatedAt: now,
	}

	if err := s.userRepo.CreateAPIKey(ctx, key); err != nil {
		return "", nil, fmt.Errorf("auth.GenerateAPIKey: %w", err)
	}

	return rawKey, key, nil
}

// ValidateAPIKey checks an API key by looking up the prefix (first 8 chars)
// and comparing the SHA-256 hash. Returns the associated user and API key record.
func (s *Service) ValidateAPIKey(ctx context.Context, rawKey string) (*domain.User, *domain.APIKey, error) {
	if len(rawKey) < apiKeyPrefixLen {
		return nil, nil, fmt.Errorf("auth.ValidateAPIKey: %w", ErrInvalidAPIKey)
	}

	prefix := rawKey[:apiKeyPrefixLen]

	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	// Pass uuid.Nil to search across all tenants (consistent with middleware behavior).
	apiKey, err := s.userRepo.GetAPIKeyByPrefix(ctx, uuid.Nil, prefix)
	if err != nil {
		return nil, nil, fmt.Errorf("auth.ValidateAPIKey: %w", ErrInvalidAPIKey)
	}

	if apiKey.KeyHash != keyHash {
		return nil, nil, fmt.Errorf("auth.ValidateAPIKey: %w", ErrInvalidAPIKey)
	}

	// Check expiration.
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, nil, fmt.Errorf("auth.ValidateAPIKey: key expired: %w", ErrInvalidAPIKey)
	}

	user, err := s.userRepo.GetByID(ctx, apiKey.TenantID, apiKey.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("auth.ValidateAPIKey: %w", err)
	}

	// Update last used timestamp (fire and forget).
	if updateErr := s.userRepo.UpdateAPIKeyLastUsed(ctx, apiKey.ID); updateErr != nil {
		log.Warn().Err(updateErr).Str("api_key_id", apiKey.ID.String()).Msg("auth.ValidateAPIKey: failed to update last_used_at")
	}

	return user, apiKey, nil
}
