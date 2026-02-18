package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/gosuda/aira/internal/domain"
)

type jwtClaims struct {
	jwt.RegisteredClaims
	TenantID string `json:"tid"`
	UserID   string `json:"uid"`
	Role     string `json:"role"`
}

func Auth(jwtSecret string, userRepo domain.UserRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try Bearer token first.
			if tok := extractBearer(r); tok != "" {
				ctx, ok := authenticateJWT(r.Context(), tok, jwtSecret)
				if ok {
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// Try API key.
			if key := r.Header.Get("X-API-Key"); key != "" {
				ctx, ok := authenticateAPIKey(r.Context(), key, userRepo)
				if ok {
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			http.Error(w, `{"title":"Unauthorized","status":401,"detail":"missing or invalid credentials"}`, http.StatusUnauthorized)
		})
	}
}

func extractBearer(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if len(auth) > 7 && strings.EqualFold(auth[:7], "bearer ") {
		return auth[7:]
	}
	return ""
}

func authenticateJWT(ctx context.Context, tokenStr, secret string) (context.Context, bool) {
	claims := &jwtClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(_ *jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid {
		return ctx, false
	}

	tenantID, err := uuid.Parse(claims.TenantID)
	if err != nil {
		return ctx, false
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return ctx, false
	}

	ctx = context.WithValue(ctx, ContextKeyTenantID, tenantID)
	ctx = context.WithValue(ctx, ContextKeyUserID, userID)
	ctx = context.WithValue(ctx, ContextKeyUserRole, claims.Role)
	return ctx, true
}

func authenticateAPIKey(ctx context.Context, rawKey string, userRepo domain.UserRepository) (context.Context, bool) {
	if len(rawKey) < 8 {
		return ctx, false
	}
	prefix := rawKey[:8]

	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	// Look up by prefix; there is no tenant context yet so we pass uuid.Nil.
	// GetAPIKeyByPrefix takes tenantID but we need to search across tenants
	// for API key auth. We pass uuid.Nil and match by hash.
	// The repo implementation should handle uuid.Nil as "any tenant".
	apiKey, err := userRepo.GetAPIKeyByPrefix(ctx, uuid.Nil, prefix)
	if err != nil {
		return ctx, false
	}

	if apiKey.KeyHash != keyHash {
		return ctx, false
	}

	// Check expiration.
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return ctx, false
	}

	// Update last used timestamp (fire and forget).
	if updateErr := userRepo.UpdateAPIKeyLastUsed(ctx, apiKey.ID); updateErr != nil {
		log.Warn().Err(updateErr).Str("api_key_id", apiKey.ID.String()).Msg("auth: failed to update api key last_used_at")
	}

	ctx = context.WithValue(ctx, ContextKeyTenantID, apiKey.TenantID)
	ctx = context.WithValue(ctx, ContextKeyUserID, apiKey.UserID)
	ctx = context.WithValue(ctx, ContextKeyUserRole, "member")
	return ctx, true
}
