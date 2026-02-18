package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/gosuda/aira/internal/auth"
)

// --- Auth URL tests ---

func TestNewGoogleProvider_AuthURL(t *testing.T) {
	t.Parallel()

	p := auth.NewGoogleProvider("google-client-id", "google-secret", "https://example.com/callback")
	authURL := p.AuthorizationURL("test-state")

	require.NotEmpty(t, authURL)
	assert.Contains(t, authURL, "accounts.google.com")
	assert.Contains(t, authURL, "client_id=google-client-id")
	assert.Contains(t, authURL, "state=test-state")
	assert.Contains(t, authURL, "redirect_uri="+url.QueryEscape("https://example.com/callback"))
	assert.Contains(t, authURL, "response_type=code")
}

func TestNewGitHubProvider_AuthURL(t *testing.T) {
	t.Parallel()

	p := auth.NewGitHubProvider("github-client-id", "github-secret", "https://example.com/gh-callback")
	authURL := p.AuthorizationURL("gh-state")

	require.NotEmpty(t, authURL)
	assert.Contains(t, authURL, "github.com/login/oauth/authorize")
	assert.Contains(t, authURL, "client_id=github-client-id")
	assert.Contains(t, authURL, "state=gh-state")
}

func TestGoogleProvider_AuthURL_ContainsScopes(t *testing.T) {
	t.Parallel()

	p := auth.NewGoogleProvider("cid", "csec", "https://example.com/cb")
	authURL := p.AuthorizationURL("s")

	assert.Contains(t, authURL, "scope=")
	assert.Contains(t, authURL, "openid")
	assert.Contains(t, authURL, "email")
	assert.Contains(t, authURL, "profile")
}

func TestGitHubProvider_AuthURL_ContainsScopes(t *testing.T) {
	t.Parallel()

	p := auth.NewGitHubProvider("cid", "csec", "https://example.com/cb")
	authURL := p.AuthorizationURL("s")

	assert.Contains(t, authURL, "scope=")
	assert.Contains(t, authURL, "read")
	assert.Contains(t, authURL, "user")
}

func TestGoogleProvider_NameIsGoogle(t *testing.T) {
	t.Parallel()

	p := auth.NewGoogleProvider("id", "sec", "https://example.com/cb")
	assert.Equal(t, "google", p.Name)
}

func TestGitHubProvider_NameIsGitHub(t *testing.T) {
	t.Parallel()

	p := auth.NewGitHubProvider("id", "sec", "https://example.com/cb")
	assert.Equal(t, "github", p.Name)
}

// --- ExchangeCode tests ---
//
// ExchangeCode does two HTTP calls:
//   1. Token exchange (POST to TokenURL) -- handled by oauth2 library.
//   2. User info fetch (GET to UserInfoURL) -- handled by OAuthProvider.
//
// Strategy:
//   - For (1): Use httptest.NewServer as a fake token endpoint. The oauth2
//     library supports context-based HTTP client injection via oauth2.HTTPClient.
//     We use a custom RoundTripper that redirects all requests to the test server.
//   - For (2): Inject a mock HTTPClient into OAuthProvider.HTTPClient.

// mockHTTPClient implements auth.HTTPClient for testing user info responses.
type mockHTTPClient struct {
	handler http.Handler
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	m.handler.ServeHTTP(rec, req)
	return rec.Result(), nil
}

// tokenRedirectTransport redirects all HTTP requests to a test server.
type tokenRedirectTransport struct {
	targetBaseURL string
}

func (tr *tokenRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newURL := tr.targetBaseURL + req.URL.Path
	if req.URL.RawQuery != "" {
		newURL += "?" + req.URL.RawQuery
	}

	newReq, err := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
	if err != nil {
		return nil, err
	}
	newReq.Header = req.Header

	return http.DefaultTransport.RoundTrip(newReq)
}

// oauthCtx returns a context with an HTTP client that routes all oauth2 token
// exchange requests to the given test server URL.
func oauthCtx(t *testing.T, tokenServerURL string) context.Context {
	t.Helper()
	transport := &tokenRedirectTransport{targetBaseURL: tokenServerURL}
	client := &http.Client{Transport: transport}
	return context.WithValue(t.Context(), oauth2.HTTPClient, client)
}

// newFakeTokenServer returns an httptest server that returns a valid OAuth2 token.
func newFakeTokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token": "fake-access-token",
			"token_type":   "Bearer",
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newErrorTokenServer returns an httptest server that returns an OAuth2 error.
func newErrorTokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "code is expired or invalid",
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestGoogleProvider_ExchangeCode_HappyPath(t *testing.T) {
	t.Parallel()

	tokenSrv := newFakeTokenServer(t)
	ctx := oauthCtx(t, tokenSrv.URL)

	mock := &mockHTTPClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"id":      "google-123",
				"email":   "alice@gmail.com",
				"name":    "Alice Smith",
				"picture": "https://photo.google.com/alice.jpg",
			})
		}),
	}

	p := auth.NewGoogleProvider("test-id", "test-secret", "https://example.com/cb")
	p.HTTPClient = mock

	info, err := p.ExchangeCode(ctx, "valid-code")

	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "google-123", info.ProviderID)
	assert.Equal(t, "alice@gmail.com", info.Email)
	assert.Equal(t, "Alice Smith", info.Name)
	assert.Equal(t, "https://photo.google.com/alice.jpg", info.AvatarURL)
}

func TestGitHubProvider_ExchangeCode_HappyPath(t *testing.T) {
	t.Parallel()

	tokenSrv := newFakeTokenServer(t)
	ctx := oauthCtx(t, tokenSrv.URL)

	mock := &mockHTTPClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":         42,
				"login":      "octocat",
				"name":       "The Octocat",
				"email":      "octocat@github.com",
				"avatar_url": "https://avatars.githubusercontent.com/u/42",
			})
		}),
	}

	p := auth.NewGitHubProvider("test-id", "test-secret", "https://example.com/cb")
	p.HTTPClient = mock

	info, err := p.ExchangeCode(ctx, "gh-valid-code")

	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "42", info.ProviderID)
	assert.Equal(t, "octocat@github.com", info.Email)
	assert.Equal(t, "The Octocat", info.Name)
	assert.Equal(t, "https://avatars.githubusercontent.com/u/42", info.AvatarURL)
}

func TestGitHubProvider_ExchangeCode_FallsBackToLogin(t *testing.T) {
	t.Parallel()

	tokenSrv := newFakeTokenServer(t)
	ctx := oauthCtx(t, tokenSrv.URL)

	mock := &mockHTTPClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":         99,
				"login":      "anonymous-dev",
				"name":       "",
				"email":      "anon@dev.io",
				"avatar_url": "",
			})
		}),
	}

	p := auth.NewGitHubProvider("test-id", "test-secret", "https://example.com/cb")
	p.HTTPClient = mock

	info, err := p.ExchangeCode(ctx, "code")

	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "anonymous-dev", info.Name, "should fall back to login when name is empty")
}

func TestExchangeCode_InvalidCode_TokenError(t *testing.T) {
	t.Parallel()

	tokenSrv := newErrorTokenServer(t)
	ctx := oauthCtx(t, tokenSrv.URL)

	p := auth.NewGoogleProvider("test-id", "test-secret", "https://example.com/cb")

	info, err := p.ExchangeCode(ctx, "bad-code")

	require.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "auth.ExchangeCode")
}

func TestExchangeCode_UserInfoHTTPError(t *testing.T) {
	t.Parallel()

	tokenSrv := newFakeTokenServer(t)
	ctx := oauthCtx(t, tokenSrv.URL)

	mock := &mockHTTPClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}),
	}

	p := auth.NewGoogleProvider("test-id", "test-secret", "https://example.com/cb")
	p.HTTPClient = mock

	info, err := p.ExchangeCode(ctx, "valid-code")

	require.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "user info returned 500")
}

func TestExchangeCode_MalformedGoogleResponse(t *testing.T) {
	t.Parallel()

	tokenSrv := newFakeTokenServer(t)
	ctx := oauthCtx(t, tokenSrv.URL)

	mock := &mockHTTPClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{not valid json`))
		}),
	}

	p := auth.NewGoogleProvider("test-id", "test-secret", "https://example.com/cb")
	p.HTTPClient = mock

	info, err := p.ExchangeCode(ctx, "valid-code")

	require.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "parseGoogleUserInfo")
}

func TestExchangeCode_MalformedGitHubResponse(t *testing.T) {
	t.Parallel()

	tokenSrv := newFakeTokenServer(t)
	ctx := oauthCtx(t, tokenSrv.URL)

	mock := &mockHTTPClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`broken-json`))
		}),
	}

	p := auth.NewGitHubProvider("test-id", "test-secret", "https://example.com/cb")
	p.HTTPClient = mock

	info, err := p.ExchangeCode(ctx, "valid-code")

	require.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "parseGitHubUserInfo")
}
