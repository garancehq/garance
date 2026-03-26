// services/auth/internal/service/auth.go
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/garancehq/garance/services/auth/internal/crypto"
	"github.com/garancehq/garance/services/auth/internal/store"
	"github.com/garancehq/garance/services/auth/internal/token"
	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserBanned         = errors.New("user is banned")
	ErrEmailNotVerified   = errors.New("email not verified")
	ErrPasswordRequired   = errors.New("password is required")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
)

type AuthService struct {
	db     *store.DB
	tokens *token.Manager
}

func NewAuthService(db *store.DB, tokens *token.Manager) *AuthService {
	return &AuthService{db: db, tokens: tokens}
}

// DB returns the underlying store for admin operations.
func (s *AuthService) DB() *store.DB {
	return s.db
}

type AuthResult struct {
	User      *store.User      `json:"user"`
	TokenPair *token.TokenPair `json:"token_pair"`
}

func (s *AuthService) SignUp(ctx context.Context, email, password, userAgent, ip string) (*AuthResult, error) {
	if password == "" {
		return nil, ErrPasswordRequired
	}
	if len(password) < 8 {
		return nil, ErrWeakPassword
	}

	hash, err := crypto.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user, err := s.db.CreateUser(ctx, email, &hash)
	if err != nil {
		return nil, err
	}

	pair, err := s.createTokenPair(ctx, user, userAgent, ip)
	if err != nil {
		return nil, err
	}

	return &AuthResult{User: user, TokenPair: pair}, nil
}

func (s *AuthService) SignIn(ctx context.Context, email, password, userAgent, ip string) (*AuthResult, error) {
	user, err := s.db.GetUserByEmail(ctx, email)
	if errors.Is(err, store.ErrUserNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	if user.BannedAt != nil {
		return nil, ErrUserBanned
	}

	if user.EncryptedPassword == nil {
		return nil, ErrInvalidCredentials
	}

	ok, err := crypto.VerifyPassword(password, *user.EncryptedPassword)
	if err != nil || !ok {
		return nil, ErrInvalidCredentials
	}

	pair, err := s.createTokenPair(ctx, user, userAgent, ip)
	if err != nil {
		return nil, err
	}

	return &AuthResult{User: user, TokenPair: pair}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshTokenStr, userAgent, ip string) (*AuthResult, error) {
	session, err := s.db.GetSessionByRefreshToken(ctx, refreshTokenStr)
	if errors.Is(err, store.ErrSessionNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	// Rotate: revoke old session
	if err := s.db.RevokeSession(ctx, session.ID); err != nil {
		return nil, err
	}

	user, err := s.db.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	if user.BannedAt != nil {
		return nil, ErrUserBanned
	}

	pair, err := s.createTokenPair(ctx, user, userAgent, ip)
	if err != nil {
		return nil, err
	}

	return &AuthResult{User: user, TokenPair: pair}, nil
}

func (s *AuthService) SignOut(ctx context.Context, refreshTokenStr string) error {
	session, err := s.db.GetSessionByRefreshToken(ctx, refreshTokenStr)
	if err != nil {
		return nil // Silent — don't leak whether token was valid
	}
	return s.db.RevokeSession(ctx, session.ID)
}

func (s *AuthService) GetUser(ctx context.Context, userID uuid.UUID) (*store.User, error) {
	return s.db.GetUserByID(ctx, userID)
}

func (s *AuthService) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	return s.db.DeleteUser(ctx, userID)
}

func (s *AuthService) createTokenPair(ctx context.Context, user *store.User, userAgent, ip string) (*token.TokenPair, error) {
	accessToken, err := s.tokens.GenerateAccessToken(user.ID, "", user.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := token.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	_, err = s.db.CreateSession(ctx, user.ID, refreshToken, userAgent, ip)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &token.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900, // 15 minutes in seconds
		TokenType:    "Bearer",
	}, nil
}
