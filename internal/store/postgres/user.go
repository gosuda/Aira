package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gosuda/aira/internal/domain"
)

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

// --- Users ---

func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, email, password_hash, name, role, avatar_url, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		u.ID, u.TenantID,
		nilIfEmpty(u.Email), nilIfEmpty(u.PasswordHash),
		u.Name, u.Role, nilIfEmpty(u.AvatarURL),
		u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("userRepo.Create: %w", err)
	}

	return nil
}

func (r *UserRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.User, error) {
	var u domain.User
	var email, passwordHash, avatarURL *string

	err := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, email, password_hash, name, role, avatar_url, created_at, updated_at
		 FROM users WHERE tenant_id = $1 AND id = $2`,
		tenantID, id,
	).Scan(&u.ID, &u.TenantID, &email, &passwordHash, &u.Name, &u.Role, &avatarURL, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("userRepo.GetByID: %w", domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("userRepo.GetByID: %w", err)
	}

	u.Email = derefStr(email)
	u.PasswordHash = derefStr(passwordHash)
	u.AvatarURL = derefStr(avatarURL)

	return &u, nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*domain.User, error) {
	var u domain.User
	var dbEmail, passwordHash, avatarURL *string

	err := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, email, password_hash, name, role, avatar_url, created_at, updated_at
		 FROM users WHERE tenant_id = $1 AND email = $2`,
		tenantID, email,
	).Scan(&u.ID, &u.TenantID, &dbEmail, &passwordHash, &u.Name, &u.Role, &avatarURL, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("userRepo.GetByEmail: %w", domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("userRepo.GetByEmail: %w", err)
	}

	u.Email = derefStr(dbEmail)
	u.PasswordHash = derefStr(passwordHash)
	u.AvatarURL = derefStr(avatarURL)

	return &u, nil
}

func (r *UserRepo) Update(ctx context.Context, u *domain.User) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET email = $1, password_hash = $2, name = $3, role = $4, avatar_url = $5, updated_at = now()
		 WHERE tenant_id = $6 AND id = $7`,
		nilIfEmpty(u.Email), nilIfEmpty(u.PasswordHash),
		u.Name, u.Role, nilIfEmpty(u.AvatarURL),
		u.TenantID, u.ID,
	)
	if err != nil {
		return fmt.Errorf("userRepo.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("userRepo.Update: %w", domain.ErrNotFound)
	}

	return nil
}

func (r *UserRepo) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.User, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, email, password_hash, name, role, avatar_url, created_at, updated_at
		 FROM users WHERE tenant_id = $1 ORDER BY created_at, id
		 LIMIT 500`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("userRepo.List: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		var u domain.User
		var email, passwordHash, avatarURL *string

		err = rows.Scan(&u.ID, &u.TenantID, &email, &passwordHash, &u.Name, &u.Role, &avatarURL, &u.CreatedAt, &u.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("userRepo.List: scan: %w", err)
		}

		u.Email = derefStr(email)
		u.PasswordHash = derefStr(passwordHash)
		u.AvatarURL = derefStr(avatarURL)
		users = append(users, &u)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("userRepo.List: rows: %w", err)
	}

	return users, nil
}

// --- OAuth Links ---

func (r *UserRepo) CreateOAuthLink(ctx context.Context, link *domain.UserOAuthLink) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO user_oauth_links (id, user_id, provider, provider_id, access_token, refresh_token, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		link.ID, link.UserID, link.Provider, link.ProviderID,
		link.AccessToken, link.RefreshToken, link.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("userRepo.CreateOAuthLink: %w", err)
	}

	return nil
}

func (r *UserRepo) GetOAuthLink(ctx context.Context, provider, providerID string) (*domain.UserOAuthLink, error) {
	var link domain.UserOAuthLink

	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, provider, provider_id, access_token, refresh_token, created_at
		 FROM user_oauth_links WHERE provider = $1 AND provider_id = $2`,
		provider, providerID,
	).Scan(&link.ID, &link.UserID, &link.Provider, &link.ProviderID,
		&link.AccessToken, &link.RefreshToken, &link.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("userRepo.GetOAuthLink: %w", domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("userRepo.GetOAuthLink: %w", err)
	}

	return &link, nil
}

func (r *UserRepo) DeleteOAuthLink(ctx context.Context, tenantID, userID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM user_oauth_links
		 WHERE id = $1 AND user_id IN (
		     SELECT id FROM users WHERE tenant_id = $2 AND id = $3
		 )`,
		id, tenantID, userID,
	)
	if err != nil {
		return fmt.Errorf("userRepo.DeleteOAuthLink: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("userRepo.DeleteOAuthLink: %w", domain.ErrNotFound)
	}

	return nil
}

// --- Messenger Links ---

func (r *UserRepo) CreateMessengerLink(ctx context.Context, link *domain.UserMessengerLink) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO user_messenger_links (id, user_id, tenant_id, platform, external_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		link.ID, link.UserID, link.TenantID, link.Platform, link.ExternalID, link.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("userRepo.CreateMessengerLink: %w", err)
	}

	return nil
}

func (r *UserRepo) GetMessengerLink(ctx context.Context, tenantID uuid.UUID, platform, externalID string) (*domain.UserMessengerLink, error) {
	var link domain.UserMessengerLink

	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, tenant_id, platform, external_id, created_at
		 FROM user_messenger_links WHERE tenant_id = $1 AND platform = $2 AND external_id = $3`,
		tenantID, platform, externalID,
	).Scan(&link.ID, &link.UserID, &link.TenantID, &link.Platform, &link.ExternalID, &link.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("userRepo.GetMessengerLink: %w", domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("userRepo.GetMessengerLink: %w", err)
	}

	return &link, nil
}

func (r *UserRepo) ListMessengerLinks(ctx context.Context, userID uuid.UUID) ([]*domain.UserMessengerLink, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, tenant_id, platform, external_id, created_at
		 FROM user_messenger_links WHERE user_id = $1 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("userRepo.ListMessengerLinks: %w", err)
	}
	defer rows.Close()

	var links []*domain.UserMessengerLink
	for rows.Next() {
		var link domain.UserMessengerLink
		err = rows.Scan(&link.ID, &link.UserID, &link.TenantID, &link.Platform, &link.ExternalID, &link.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("userRepo.ListMessengerLinks: scan: %w", err)
		}
		links = append(links, &link)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("userRepo.ListMessengerLinks: rows: %w", err)
	}

	return links, nil
}

func (r *UserRepo) DeleteMessengerLink(ctx context.Context, tenantID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM user_messenger_links WHERE tenant_id = $1 AND id = $2`,
		tenantID, id,
	)
	if err != nil {
		return fmt.Errorf("userRepo.DeleteMessengerLink: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("userRepo.DeleteMessengerLink: %w", domain.ErrNotFound)
	}

	return nil
}

// --- API Keys ---

func (r *UserRepo) CreateAPIKey(ctx context.Context, key *domain.APIKey) error {
	scopes, err := json.Marshal(key.Scopes)
	if err != nil {
		return fmt.Errorf("userRepo.CreateAPIKey: marshal scopes: %w", err)
	}

	_, err = r.pool.Exec(ctx,
		`INSERT INTO api_keys (id, tenant_id, user_id, name, key_hash, prefix, scopes, last_used_at, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		key.ID, key.TenantID, key.UserID, key.Name, key.KeyHash, key.Prefix,
		scopes, key.LastUsedAt, key.ExpiresAt, key.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("userRepo.CreateAPIKey: %w", err)
	}

	return nil
}

func (r *UserRepo) GetAPIKeyByPrefix(ctx context.Context, tenantID uuid.UUID, prefix string) (*domain.APIKey, error) {
	var key domain.APIKey
	var scopes []byte

	err := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, user_id, name, key_hash, prefix, scopes, last_used_at, expires_at, created_at
		 FROM api_keys WHERE tenant_id = $1 AND prefix = $2`,
		tenantID, prefix,
	).Scan(&key.ID, &key.TenantID, &key.UserID, &key.Name, &key.KeyHash, &key.Prefix,
		&scopes, &key.LastUsedAt, &key.ExpiresAt, &key.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("userRepo.GetAPIKeyByPrefix: %w", domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("userRepo.GetAPIKeyByPrefix: %w", err)
	}

	err = json.Unmarshal(scopes, &key.Scopes)
	if err != nil {
		return nil, fmt.Errorf("userRepo.GetAPIKeyByPrefix: unmarshal scopes: %w", err)
	}

	return &key, nil
}

func (r *UserRepo) ListAPIKeys(ctx context.Context, tenantID, userID uuid.UUID) ([]*domain.APIKey, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, user_id, name, key_hash, prefix, scopes, last_used_at, expires_at, created_at
		 FROM api_keys WHERE tenant_id = $1 AND user_id = $2 ORDER BY created_at`,
		tenantID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("userRepo.ListAPIKeys: %w", err)
	}
	defer rows.Close()

	var keys []*domain.APIKey
	for rows.Next() {
		var key domain.APIKey
		var scopes []byte

		err = rows.Scan(&key.ID, &key.TenantID, &key.UserID, &key.Name, &key.KeyHash, &key.Prefix,
			&scopes, &key.LastUsedAt, &key.ExpiresAt, &key.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("userRepo.ListAPIKeys: scan: %w", err)
		}
		err = json.Unmarshal(scopes, &key.Scopes)
		if err != nil {
			return nil, fmt.Errorf("userRepo.ListAPIKeys: unmarshal scopes: %w", err)
		}
		keys = append(keys, &key)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("userRepo.ListAPIKeys: rows: %w", err)
	}

	return keys, nil
}

func (r *UserRepo) DeleteAPIKey(ctx context.Context, tenantID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM api_keys WHERE tenant_id = $1 AND id = $2`,
		tenantID, id,
	)
	if err != nil {
		return fmt.Errorf("userRepo.DeleteAPIKey: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("userRepo.DeleteAPIKey: %w", domain.ErrNotFound)
	}

	return nil
}

func (r *UserRepo) UpdateAPIKeyLastUsed(ctx context.Context, tenantID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE api_keys SET last_used_at = now() WHERE tenant_id = $1 AND id = $2`,
		tenantID, id,
	)
	if err != nil {
		return fmt.Errorf("userRepo.UpdateAPIKeyLastUsed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("userRepo.UpdateAPIKeyLastUsed: %w", domain.ErrNotFound)
	}

	return nil
}

// --- Helpers ---

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
