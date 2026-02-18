package auth_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gosuda/aira/internal/auth"
)

func TestJWT_IssueAndValidateRoundTrip(t *testing.T) {
	t.Parallel()

	secret := "test-secret-key-very-long-and-secure"
	tenantID := uuid.New()
	userID := uuid.New()
	role := "admin"

	t.Run("access token round-trip", func(t *testing.T) {
		t.Parallel()

		token, err := auth.IssueAccessToken(secret, tenantID, userID, role, 5*time.Minute)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		claims, err := auth.ValidateToken(secret, token)
		require.NoError(t, err)
		require.NotNil(t, claims)

		assert.Equal(t, tenantID.String(), claims.TenantID)
		assert.Equal(t, userID.String(), claims.UserID)
		assert.Equal(t, "admin", claims.Role)
		assert.Equal(t, "access", claims.TokenType)
		assert.Equal(t, "aira", claims.Issuer)
	})

	t.Run("refresh token round-trip", func(t *testing.T) {
		t.Parallel()

		token, err := auth.IssueRefreshToken(secret, tenantID, userID, role, 24*time.Hour)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		claims, err := auth.ValidateToken(secret, token)
		require.NoError(t, err)
		require.NotNil(t, claims)

		assert.Equal(t, tenantID.String(), claims.TenantID)
		assert.Equal(t, userID.String(), claims.UserID)
		assert.Equal(t, "admin", claims.Role)
		assert.Equal(t, "refresh", claims.TokenType)
	})
}

func TestJWT_ExpiredTokenRejected(t *testing.T) {
	t.Parallel()

	secret := "test-secret-key"
	tenantID := uuid.New()
	userID := uuid.New()

	// Issue a token that has already expired (negative TTL).
	token, err := auth.IssueAccessToken(secret, tenantID, userID, "member", -1*time.Second)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := auth.ValidateToken(secret, token)
	require.Error(t, err)
	assert.Nil(t, claims)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}

func TestJWT_InvalidSecretRejected(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()

	token, err := auth.IssueAccessToken("correct-secret", tenantID, userID, "member", 5*time.Minute)
	require.NoError(t, err)

	// Validate with a different secret.
	claims, err := auth.ValidateToken("wrong-secret", token)
	require.Error(t, err)
	assert.Nil(t, claims)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}

func TestJWT_ClaimsExtractedCorrectly(t *testing.T) {
	t.Parallel()

	secret := "extract-claims-secret"
	tenantID := uuid.New()
	userID := uuid.New()

	tests := []struct {
		name      string
		role      string
		issuer    func(string, uuid.UUID, uuid.UUID, string, time.Duration) (string, error)
		tokenType string
	}{
		{
			name:      "access token claims",
			role:      "admin",
			issuer:    auth.IssueAccessToken,
			tokenType: "access",
		},
		{
			name:      "refresh token claims",
			role:      "member",
			issuer:    auth.IssueRefreshToken,
			tokenType: "refresh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			token, err := tt.issuer(secret, tenantID, userID, tt.role, 10*time.Minute)
			require.NoError(t, err)

			claims, err := auth.ValidateToken(secret, token)
			require.NoError(t, err)

			assert.Equal(t, tenantID.String(), claims.TenantID)
			assert.Equal(t, userID.String(), claims.UserID)
			assert.Equal(t, tt.role, claims.Role)
			assert.Equal(t, tt.tokenType, claims.TokenType)
			assert.Equal(t, "aira", claims.Issuer)
			assert.NotNil(t, claims.IssuedAt)
			assert.NotNil(t, claims.ExpiresAt)
		})
	}
}

func TestJWT_MalformedTokenRejected(t *testing.T) {
	t.Parallel()

	claims, err := auth.ValidateToken("secret", "not.a.valid.jwt.token")
	require.Error(t, err)
	assert.Nil(t, claims)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}
