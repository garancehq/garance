package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrOAuthStateNotFound = errors.New("oauth state not found or expired")

type OAuthState struct {
	State       string    `json:"state"`
	Provider    string    `json:"provider"`
	RedirectURI string    `json:"redirect_uri"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func (db *DB) CreateOAuthState(ctx context.Context, state, provider, redirectURI string) error {
	_, err := db.Pool.Exec(ctx,
		`INSERT INTO garance_auth.oauth_states (state, provider, redirect_uri)
		 VALUES ($1, $2, $3)`,
		state, provider, redirectURI,
	)
	return err
}

func (db *DB) GetAndConsumeOAuthState(ctx context.Context, state string) (*OAuthState, error) {
	// Lazy cleanup of expired states
	db.Pool.Exec(ctx, `DELETE FROM garance_auth.oauth_states WHERE expires_at < now()`)

	var s OAuthState
	err := db.Pool.QueryRow(ctx,
		`DELETE FROM garance_auth.oauth_states WHERE state = $1 AND expires_at > now()
		 RETURNING state, provider, redirect_uri, created_at, expires_at`,
		state,
	).Scan(&s.State, &s.Provider, &s.RedirectURI, &s.CreatedAt, &s.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOAuthStateNotFound
	}
	return &s, err
}
