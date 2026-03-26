// services/auth/internal/oauth/github.go
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type GitHubProvider struct {
	clientID     string
	clientSecret string
	scopes       string
}

func NewGitHubProvider(clientID, clientSecret, scopes string) *GitHubProvider {
	if scopes == "" {
		scopes = "user:email"
	}
	return &GitHubProvider{clientID: clientID, clientSecret: clientSecret, scopes: scopes}
}

func (g *GitHubProvider) AuthorizeURL(state, redirectURI string) string {
	params := url.Values{
		"client_id":    {g.clientID},
		"redirect_uri": {redirectURI},
		"scope":        {g.scopes},
		"state":        {state},
	}
	return "https://github.com/login/oauth/authorize?" + params.Encode()
}

func (g *GitHubProvider) ExchangeCode(ctx context.Context, code, redirectURI string) (*OAuthToken, error) {
	data := url.Values{
		"code":          {code},
		"client_id":     {g.clientID},
		"client_secret": {g.clientSecret},
		"redirect_uri":  {redirectURI},
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	var token OAuthToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("failed to decode token: %w", err)
	}
	return &token, nil
}

func (g *GitHubProvider) GetUserProfile(ctx context.Context, accessToken string) (*OAuthProfile, error) {
	// Get user profile
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch profile: %w", err)
	}
	defer resp.Body.Close()

	var raw map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&raw)

	email := ""
	if e, ok := raw["email"].(string); ok && e != "" {
		email = strings.ToLower(e)
	}

	// If email is private, fetch from /user/emails
	if email == "" {
		email, _ = g.fetchPrimaryEmail(ctx, accessToken)
	}

	return &OAuthProfile{
		ProviderUserID: fmt.Sprintf("%.0f", raw["id"]),
		Email:          email,
		Name:           fmt.Sprintf("%v", raw["name"]),
		AvatarURL:      fmt.Sprintf("%v", raw["avatar_url"]),
		Raw:            raw,
	}, nil
}

func (g *GitHubProvider) fetchPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user/emails", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	json.NewDecoder(resp.Body).Decode(&emails)

	for _, e := range emails {
		if e.Primary && e.Verified {
			return strings.ToLower(e.Email), nil
		}
	}
	return "", fmt.Errorf("no verified primary email found")
}
