package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	Email        string // may be empty for OAuth-only users
	PasswordHash string // argon2id, empty if OAuth-only
	Name         string
	Role         string // "admin", "member", or "viewer"
	AvatarURL    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserOAuthLink struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Provider     string // "google", "github", "slack", "discord"
	ProviderID   string
	AccessToken  string // encrypted
	RefreshToken string // encrypted
	CreatedAt    time.Time
}

type UserMessengerLink struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	TenantID   uuid.UUID
	Platform   string // "slack", "discord", "telegram"
	ExternalID string
	CreatedAt  time.Time
}

type APIKey struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	UserID     uuid.UUID
	Name       string
	KeyHash    string // SHA-256
	Prefix     string // first 8 chars for identification
	Scopes     []string
	LastUsedAt *time.Time // nullable
	ExpiresAt  *time.Time // nullable
	CreatedAt  time.Time
}

type UserRepository interface {
	Create(ctx context.Context, u *User) error
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*User, error)
	Update(ctx context.Context, u *User) error
	List(ctx context.Context, tenantID uuid.UUID) ([]*User, error)

	// OAuth links
	CreateOAuthLink(ctx context.Context, link *UserOAuthLink) error
	GetOAuthLink(ctx context.Context, provider, providerID string) (*UserOAuthLink, error)
	DeleteOAuthLink(ctx context.Context, id uuid.UUID) error

	// Messenger links
	CreateMessengerLink(ctx context.Context, link *UserMessengerLink) error
	GetMessengerLink(ctx context.Context, tenantID uuid.UUID, platform, externalID string) (*UserMessengerLink, error)
	ListMessengerLinks(ctx context.Context, userID uuid.UUID) ([]*UserMessengerLink, error)
	DeleteMessengerLink(ctx context.Context, id uuid.UUID) error

	// API keys
	CreateAPIKey(ctx context.Context, key *APIKey) error
	GetAPIKeyByPrefix(ctx context.Context, tenantID uuid.UUID, prefix string) (*APIKey, error)
	ListAPIKeys(ctx context.Context, tenantID, userID uuid.UUID) ([]*APIKey, error)
	DeleteAPIKey(ctx context.Context, id uuid.UUID) error
	UpdateAPIKeyLastUsed(ctx context.Context, id uuid.UUID) error
}
