package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"

	"github.com/gosuda/aira/internal/domain"
)

// Sentinel errors for the auth package.
var (
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	ErrUserAlreadyExists  = errors.New("auth: user already exists")
	ErrUserNotFound       = errors.New("auth: user not found")
)

// argon2id parameters following OWASP recommendations.
const (
	argonTime    = 1
	argonMemory  = 64 * 1024 // 64 MiB
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

// Service provides authentication and authorization operations.
type Service struct {
	userRepo   domain.UserRepository
	jwtSecret  string
	accessTTL  time.Duration
	refreshTTL time.Duration

	// linkTokens stores temporary link tokens in memory (token string -> *LinkToken).
	linkTokens sync.Map
}

// NewService creates a new auth service.
func NewService(userRepo domain.UserRepository, jwtSecret string, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{
		userRepo:   userRepo,
		jwtSecret:  jwtSecret,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// Register creates a new user with email/password. Returns the created user.
// The password is hashed with argon2id before storage.
func (s *Service) Register(ctx context.Context, tenantID uuid.UUID, email, password, name string) (*domain.User, error) {
	// Check if user already exists.
	existing, err := s.userRepo.GetByEmail(ctx, tenantID, email)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("auth.Register: %w", ErrUserAlreadyExists)
	}

	hash, err := hashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("auth.Register: %w", err)
	}

	now := time.Now()
	user := &domain.User{
		ID:           uuid.New(),
		TenantID:     tenantID,
		Email:        email,
		PasswordHash: hash,
		Name:         name,
		Role:         "member",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("auth.Register: %w", err)
	}

	return user, nil
}

// Login validates email/password and returns access + refresh JWT tokens.
func (s *Service) Login(ctx context.Context, tenantID uuid.UUID, email, password string) (accessToken, refreshToken string, err error) {
	user, err := s.userRepo.GetByEmail(ctx, tenantID, email)
	if err != nil {
		return "", "", fmt.Errorf("auth.Login: %w", ErrInvalidCredentials)
	}

	if !verifyPassword(password, user.PasswordHash) {
		return "", "", fmt.Errorf("auth.Login: %w", ErrInvalidCredentials)
	}

	accessToken, err = IssueAccessToken(s.jwtSecret, user.TenantID, user.ID, user.Role, s.accessTTL)
	if err != nil {
		return "", "", fmt.Errorf("auth.Login: %w", err)
	}

	refreshToken, err = IssueRefreshToken(s.jwtSecret, user.TenantID, user.ID, user.Role, s.refreshTTL)
	if err != nil {
		return "", "", fmt.Errorf("auth.Login: %w", err)
	}

	return accessToken, refreshToken, nil
}

// RefreshToken validates a refresh token and issues a new access token.
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	claims, err := ValidateToken(s.jwtSecret, refreshToken)
	if err != nil {
		return "", fmt.Errorf("auth.RefreshToken: %w", err)
	}

	if claims.TokenType != tokenTypeRefresh {
		return "", fmt.Errorf("auth.RefreshToken: %w", ErrInvalidToken)
	}

	tenantID, err := uuid.Parse(claims.TenantID)
	if err != nil {
		return "", fmt.Errorf("auth.RefreshToken: invalid tenant id: %w", err)
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return "", fmt.Errorf("auth.RefreshToken: invalid user id: %w", err)
	}

	// Verify the user still exists and fetch current role.
	user, err := s.userRepo.GetByID(ctx, tenantID, userID)
	if err != nil {
		return "", fmt.Errorf("auth.RefreshToken: %w", ErrUserNotFound)
	}

	newAccess, err := IssueAccessToken(s.jwtSecret, user.TenantID, user.ID, user.Role, s.accessTTL)
	if err != nil {
		return "", fmt.Errorf("auth.RefreshToken: %w", err)
	}

	return newAccess, nil
}

// GetUser returns a user by ID (for middleware use).
func (s *Service) GetUser(ctx context.Context, tenantID, userID uuid.UUID) (*domain.User, error) {
	user, err := s.userRepo.GetByID(ctx, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("auth.GetUser: %w", err)
	}

	return user, nil
}

// hashPassword generates an argon2id hash with a random salt.
// Format: hex(salt) + "$" + hex(hash)
func hashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	return hex.EncodeToString(salt) + "$" + hex.EncodeToString(hash), nil
}

// verifyPassword checks a password against an argon2id hash.
func verifyPassword(password, encoded string) bool {
	// Split salt$hash
	var saltHex, hashHex string
	for i := range len(encoded) {
		if encoded[i] == '$' {
			saltHex = encoded[:i]
			hashHex = encoded[i+1:]
			break
		}
	}

	if saltHex == "" || hashHex == "" {
		return false
	}

	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false
	}

	expectedHash, err := hex.DecodeString(hashHex)
	if err != nil {
		return false
	}

	computed := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	// Constant-time comparison to prevent timing attacks.
	if len(computed) != len(expectedHash) {
		return false
	}

	var diff byte
	for i := range computed {
		diff |= computed[i] ^ expectedHash[i]
	}

	return diff == 0
}
