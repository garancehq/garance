// services/auth/internal/oauth/gitlab.go
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type GitLabProvider struct {
	clientID     string
	clientSecret string
	scopes       string
}

func NewGitLabProvider(clientID, clientSecret, scopes string) *GitLabProvider {
	if scopes == "" {
		scopes = "read_user"
	}
	return &GitLabProvider{clientID: clientID, clientSecret: clientSecret, scopes: scopes}
}

func (g *GitLabProvider) AuthorizeURL(state, redirectURI string) string {
	params := url.Values{
		"client_id":     {g.clientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {g.scopes},
		"state":         {state},
	}
	return "https://gitlab.com/oauth/authorize?" + params.Encode()
}

func (g *GitLabProvider) ExchangeCode(ctx context.Context, code, redirectURI string) (*OAuthToken, error) {
	data := url.Values{
		"code":          {code},
		"client_id":     {g.clientID},
		"client_secret": {g.clientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	}
	resp, err := http.PostForm("https://gitlab.com/oauth/token", data)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	var token OAuthToken
	json.NewDecoder(resp.Body).Decode(&token)
	return &token, nil
}

func (g *GitLabProvider) GetUserProfile(ctx context.Context, accessToken string) (*OAuthProfile, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://gitlab.com/api/v4/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch profile: %w", err)
	}
	defer resp.Body.Close()

	var raw map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&raw)

	return &OAuthProfile{
		ProviderUserID: fmt.Sprintf("%.0f", raw["id"]),
		Email:          strings.ToLower(fmt.Sprintf("%v", raw["email"])),
		Name:           fmt.Sprintf("%v", raw["name"]),
		AvatarURL:      fmt.Sprintf("%v", raw["avatar_url"]),
		Raw:            raw,
	}, nil
}
