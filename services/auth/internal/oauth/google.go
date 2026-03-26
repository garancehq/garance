// services/auth/internal/oauth/google.go
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type GoogleProvider struct {
	clientID     string
	clientSecret string
	scopes       string
}

func NewGoogleProvider(clientID, clientSecret, scopes string) *GoogleProvider {
	if scopes == "" {
		scopes = "email profile"
	}
	return &GoogleProvider{clientID: clientID, clientSecret: clientSecret, scopes: scopes}
}

func (g *GoogleProvider) AuthorizeURL(state, redirectURI string) string {
	params := url.Values{
		"client_id":     {g.clientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {g.scopes},
		"state":         {state},
		"access_type":   {"offline"},
	}
	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
}

func (g *GoogleProvider) ExchangeCode(ctx context.Context, code, redirectURI string) (*OAuthToken, error) {
	data := url.Values{
		"code":          {code},
		"client_id":     {g.clientID},
		"client_secret": {g.clientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	}
	resp, err := http.PostForm("https://oauth2.googleapis.com/token", data)
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

func (g *GoogleProvider) GetUserProfile(ctx context.Context, accessToken string) (*OAuthProfile, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch profile: %w", err)
	}
	defer resp.Body.Close()

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode profile: %w", err)
	}

	return &OAuthProfile{
		ProviderUserID: fmt.Sprintf("%v", raw["id"]),
		Email:          strings.ToLower(fmt.Sprintf("%v", raw["email"])),
		Name:           fmt.Sprintf("%v", raw["name"]),
		AvatarURL:      fmt.Sprintf("%v", raw["picture"]),
		Raw:            raw,
	}, nil
}
