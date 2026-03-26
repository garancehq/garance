// services/auth/internal/service/oauth.go
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/garancehq/garance/services/auth/internal/crypto"
	"github.com/garancehq/garance/services/auth/internal/oauth"
	"github.com/garancehq/garance/services/auth/internal/store"
)

var (
	ErrOAuthProviderNotConfigured = errors.New("OAuth provider not configured or disabled")
	ErrOAuthNoEmail               = errors.New("email is required — please make your email public in your provider settings")
	ErrInvalidRedirectURI         = errors.New("redirect_uri is not allowed")
	ErrInvalidOAuthState          = errors.New("invalid or expired OAuth state")
)

// OAuthAuthorize initiates the OAuth flow — returns the provider's authorize URL.
func (s *AuthService) OAuthAuthorize(ctx context.Context, providerName, redirectURI, baseURL string, encryptionKey []byte) (string, error) {
	// Validate redirect URI
	if !isValidRedirectURI(redirectURI, baseURL) {
		return "", ErrInvalidRedirectURI
	}

	// Get provider config from DB
	providerConfig, err := s.db.GetProvider(ctx, providerName)
	if err != nil || !providerConfig.Enabled {
		return "", ErrOAuthProviderNotConfigured
	}

	// Decrypt client secret
	clientSecret, err := crypto.Decrypt(providerConfig.ClientSecretEncrypted, encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt client secret: %w", err)
	}

	// Create provider
	provider, err := oauth.NewProvider(providerName, providerConfig.ClientID, clientSecret, providerConfig.Scopes)
	if err != nil {
		return "", err
	}

	// Generate state
	stateBytes := make([]byte, 32)
	rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)

	// Store state
	callbackURI := fmt.Sprintf("%s/auth/v1/oauth/%s/callback", baseURL, providerName)
	if err := s.db.CreateOAuthState(ctx, state, providerName, redirectURI); err != nil {
		return "", fmt.Errorf("failed to create state: %w", err)
	}

	return provider.AuthorizeURL(state, callbackURI), nil
}

// OAuthCallback handles the provider callback — exchanges code, creates/links user, returns tokens.
func (s *AuthService) OAuthCallback(ctx context.Context, providerName, code, state, baseURL, userAgent, ip string, encryptionKey []byte) (*AuthResult, string, error) {
	// Verify and consume state
	oauthState, err := s.db.GetAndConsumeOAuthState(ctx, state)
	if err != nil {
		return nil, "", ErrInvalidOAuthState
	}
	if oauthState.Provider != providerName {
		return nil, "", ErrInvalidOAuthState
	}

	// Get provider config
	providerConfig, err := s.db.GetProvider(ctx, providerName)
	if err != nil || !providerConfig.Enabled {
		return nil, "", ErrOAuthProviderNotConfigured
	}

	clientSecret, err := crypto.Decrypt(providerConfig.ClientSecretEncrypted, encryptionKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decrypt: %w", err)
	}

	provider, _ := oauth.NewProvider(providerName, providerConfig.ClientID, clientSecret, providerConfig.Scopes)

	// Exchange code for tokens
	callbackURI := fmt.Sprintf("%s/auth/v1/oauth/%s/callback", baseURL, providerName)
	oauthToken, err := provider.ExchangeCode(ctx, code, callbackURI)
	if err != nil {
		return nil, "", fmt.Errorf("code exchange failed: %w", err)
	}

	// Get user profile
	profile, err := provider.GetUserProfile(ctx, oauthToken.AccessToken)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get profile: %w", err)
	}

	if profile.Email == "" {
		return nil, "", ErrOAuthNoEmail
	}

	// Normalize email
	profile.Email = strings.ToLower(profile.Email)

	// Find or create user
	user, err := s.findOrCreateOAuthUser(ctx, providerName, profile)
	if err != nil {
		return nil, "", err
	}

	// Check if banned
	if user.BannedAt != nil {
		return nil, "", ErrUserBanned
	}

	// Generate token pair
	pair, err := s.createTokenPair(ctx, user, userAgent, ip)
	if err != nil {
		return nil, "", err
	}

	return &AuthResult{User: user, TokenPair: pair}, oauthState.RedirectURI, nil
}

func (s *AuthService) findOrCreateOAuthUser(ctx context.Context, providerName string, profile *oauth.OAuthProfile) (*store.User, error) {
	providerData, _ := json.Marshal(profile.Raw)

	// Check if identity already exists
	identity, err := s.db.GetIdentityByProvider(ctx, providerName, profile.ProviderUserID)
	if err == nil {
		// Returning user — update provider data
		s.db.UpdateIdentityProviderData(ctx, providerName, profile.ProviderUserID, providerData)
		return s.db.GetUserByID(ctx, identity.UserID)
	}

	// Check if email already exists
	user, err := s.db.GetUserByEmail(ctx, profile.Email)
	if err == nil {
		// Link provider to existing account
		s.db.CreateIdentity(ctx, user.ID, providerName, profile.ProviderUserID, providerData)
		return user, nil
	}

	// Create new user (no password, email verified)
	user, err = s.db.CreateUser(ctx, profile.Email, nil)
	if errors.Is(err, store.ErrEmailAlreadyTaken) {
		// Race condition — retry
		user, err = s.db.GetUserByEmail(ctx, profile.Email)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	// Verify email immediately for OAuth users
	s.db.VerifyUserEmail(ctx, user.ID)

	// Create identity
	s.db.CreateIdentity(ctx, user.ID, providerName, profile.ProviderUserID, providerData)

	// Re-fetch user (with email_verified = true)
	return s.db.GetUserByID(ctx, user.ID)
}

func isValidRedirectURI(redirectURI, baseURL string) bool {
	if redirectURI == "" {
		return false
	}
	redirect, err := url.Parse(redirectURI)
	if err != nil {
		return false
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return false
	}

	// In dev mode (localhost), accept any localhost origin
	if base.Hostname() == "localhost" || base.Hostname() == "127.0.0.1" {
		return redirect.Hostname() == "localhost" || redirect.Hostname() == "127.0.0.1"
	}

	// In production, must match the base URL origin
	return redirect.Scheme == base.Scheme && redirect.Host == base.Host
}
