package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims holds the JWT token payload. Field types and JSON tags are compatible
// with the middleware's jwtClaims so tokens issued here are parsed correctly.
type Claims struct {
	jwt.RegisteredClaims
	TenantID  string `json:"tid"`
	UserID    string `json:"uid"`
	Role      string `json:"role"`
	TokenType string `json:"typ"` // "access" or "refresh"
}

const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
)

// ErrInvalidToken is returned when a JWT cannot be parsed or has expired.
var ErrInvalidToken = errors.New("auth: invalid or expired token")

// IssueAccessToken creates a signed JWT access token.
func IssueAccessToken(secret string, tenantID, userID uuid.UUID, role string, ttl time.Duration) (string, error) {
	return issueToken(secret, tenantID, userID, role, tokenTypeAccess, ttl)
}

// IssueRefreshToken creates a signed JWT refresh token.
func IssueRefreshToken(secret string, tenantID, userID uuid.UUID, role string, ttl time.Duration) (string, error) {
	return issueToken(secret, tenantID, userID, role, tokenTypeRefresh, ttl)
}

func issueToken(secret string, tenantID, userID uuid.UUID, role, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "aira",
		},
		TenantID:  tenantID.String(),
		UserID:    userID.String(),
		Role:      role,
		TokenType: tokenType,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("auth.issueToken: %w", err)
	}

	return signed, nil
}

// ValidateToken parses and validates a JWT token string. Returns the embedded claims.
func ValidateToken(secret, tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(_ *jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, fmt.Errorf("auth.ValidateToken: %w", ErrInvalidToken)
	}

	if !token.Valid {
		return nil, fmt.Errorf("auth.ValidateToken: %w", ErrInvalidToken)
	}

	return claims, nil
}
