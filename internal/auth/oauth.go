package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

// HTTPClient abstracts HTTP calls for testability. The standard *http.Client
// satisfies this interface.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// OAuthUserInfo contains the user information returned by an OAuth2 provider.
type OAuthUserInfo struct {
	ProviderID string // unique ID from the provider
	Email      string
	Name       string
	AvatarURL  string
}

// OAuthProvider holds the configuration for an OAuth2 identity provider.
type OAuthProvider struct {
	Name         string
	ClientID     string
	ClientSecret string //nolint:gosec // G117: OAuth provider config
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Scopes       []string
	RedirectURL  string

	// HTTPClient overrides the default HTTP client used for user info requests.
	// When nil, the OAuth2-token-bearing client is used (production default).
	HTTPClient HTTPClient

	// oauthConfig is the compiled oauth2.Config.
	oauthConfig *oauth2.Config
}

// NewGoogleProvider returns an OAuth2 configuration for Google.
func NewGoogleProvider(clientID, clientSecret, redirectURL string) *OAuthProvider {
	p := &OAuthProvider{
		Name:         "google",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      google.Endpoint.AuthURL,
		TokenURL:     google.Endpoint.TokenURL,
		UserInfoURL:  "https://www.googleapis.com/oauth2/v2/userinfo",
		Scopes:       []string{"openid", "email", "profile"},
		RedirectURL:  redirectURL,
	}
	p.oauthConfig = p.buildConfig()
	return p
}

// NewGitHubProvider returns an OAuth2 configuration for GitHub.
func NewGitHubProvider(clientID, clientSecret, redirectURL string) *OAuthProvider {
	p := &OAuthProvider{
		Name:         "github",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      github.Endpoint.AuthURL,
		TokenURL:     github.Endpoint.TokenURL,
		UserInfoURL:  "https://api.github.com/user",
		Scopes:       []string{"read:user", "user:email"},
		RedirectURL:  redirectURL,
	}
	p.oauthConfig = p.buildConfig()
	return p
}

func (p *OAuthProvider) buildConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  p.AuthURL,
			TokenURL: p.TokenURL,
		},
		Scopes:      p.Scopes,
		RedirectURL: p.RedirectURL,
	}
}

// AuthorizationURL returns the OAuth2 authorization URL with the given state parameter.
func (p *OAuthProvider) AuthorizationURL(state string) string {
	return p.oauthConfig.AuthCodeURL(state)
}

// ExchangeCode exchanges an authorization code for tokens and fetches user info.
// Returns an OAuthUserInfo with the provider-side user ID, email, display name,
// and avatar URL.
func (p *OAuthProvider) ExchangeCode(ctx context.Context, code string) (*OAuthUserInfo, error) {
	token, err := p.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("auth.ExchangeCode: %w", err)
	}

	// Use injected HTTPClient if set (tests), otherwise use the OAuth2
	// token-bearing client (production).
	var client HTTPClient
	if p.HTTPClient != nil {
		client = p.HTTPClient
	} else {
		client = p.oauthConfig.Client(ctx, token)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.UserInfoURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("auth.ExchangeCode: creating request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth.ExchangeCode: fetching user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth.ExchangeCode: user info returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("auth.ExchangeCode: reading user info: %w", err)
	}

	switch p.Name {
	case "google":
		return parseGoogleUserInfo(body)
	case "github":
		return parseGitHubUserInfo(body)
	default:
		return nil, fmt.Errorf("auth.ExchangeCode: unsupported provider %q", p.Name)
	}
}

type googleUserInfoResp struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func parseGoogleUserInfo(data []byte) (*OAuthUserInfo, error) {
	var info googleUserInfoResp
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("auth.parseGoogleUserInfo: %w", err)
	}

	return &OAuthUserInfo{
		ProviderID: info.ID,
		Email:      info.Email,
		Name:       info.Name,
		AvatarURL:  info.Picture,
	}, nil
}

type gitHubUserInfoResp struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

func parseGitHubUserInfo(data []byte) (*OAuthUserInfo, error) {
	var info gitHubUserInfoResp
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("auth.parseGitHubUserInfo: %w", err)
	}

	displayName := info.Name
	if displayName == "" {
		displayName = info.Login
	}

	return &OAuthUserInfo{
		ProviderID: strconv.Itoa(info.ID),
		Email:      info.Email,
		Name:       displayName,
		AvatarURL:  info.AvatarURL,
	}, nil
}
