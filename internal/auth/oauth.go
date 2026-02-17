package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

// OAuthProvider holds the configuration for an OAuth2 identity provider.
type OAuthProvider struct {
	Name         string
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Scopes       []string
	RedirectURL  string

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
// Returns the provider-side user ID, email, display name, and avatar URL.
func (p *OAuthProvider) ExchangeCode(ctx context.Context, code string) (providerID, email, name, avatarURL string, err error) {
	token, err := p.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return "", "", "", "", fmt.Errorf("auth.ExchangeCode: %w", err)
	}

	client := p.oauthConfig.Client(ctx, token)

	resp, err := client.Get(p.UserInfoURL)
	if err != nil {
		return "", "", "", "", fmt.Errorf("auth.ExchangeCode: fetching user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", "", "", fmt.Errorf("auth.ExchangeCode: user info returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", "", fmt.Errorf("auth.ExchangeCode: reading user info: %w", err)
	}

	switch p.Name {
	case "google":
		return parseGoogleUserInfo(body)
	case "github":
		return parseGitHubUserInfo(body)
	default:
		return "", "", "", "", fmt.Errorf("auth.ExchangeCode: unsupported provider %q", p.Name)
	}
}

type googleUserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func parseGoogleUserInfo(data []byte) (providerID, email, name, avatarURL string, err error) {
	var info googleUserInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return "", "", "", "", fmt.Errorf("auth.parseGoogleUserInfo: %w", err)
	}

	return info.ID, info.Email, info.Name, info.Picture, nil
}

type gitHubUserInfo struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

func parseGitHubUserInfo(data []byte) (providerID, email, name, avatarURL string, err error) {
	var info gitHubUserInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return "", "", "", "", fmt.Errorf("auth.parseGitHubUserInfo: %w", err)
	}

	displayName := info.Name
	if displayName == "" {
		displayName = info.Login
	}

	return fmt.Sprintf("%d", info.ID), info.Email, displayName, info.AvatarURL, nil
}
