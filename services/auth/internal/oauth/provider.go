// services/auth/internal/oauth/provider.go
package oauth

import (
	"context"
	"fmt"
)

type OAuthToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

type OAuthProfile struct {
	ProviderUserID string
	Email          string
	Name           string
	AvatarURL      string
	Raw            map[string]interface{}
}

type Provider interface {
	AuthorizeURL(state, redirectURI string) string
	ExchangeCode(ctx context.Context, code, redirectURI string) (*OAuthToken, error)
	GetUserProfile(ctx context.Context, accessToken string) (*OAuthProfile, error)
}

func NewProvider(name, clientID, clientSecret, scopes string) (Provider, error) {
	switch name {
	case "google":
		return NewGoogleProvider(clientID, clientSecret, scopes), nil
	case "github":
		return NewGitHubProvider(clientID, clientSecret, scopes), nil
	case "gitlab":
		return NewGitLabProvider(clientID, clientSecret, scopes), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
}
