package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrSessionNotFound = errors.New("session not found")

type Session struct {
	ID           uuid.UUID  `json:"id"`
	UserID       uuid.UUID  `json:"user_id"`
	RefreshToken string     `json:"-"`
	UserAgent    *string    `json:"user_agent,omitempty"`
	IPAddress    *string    `json:"ip_address,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    time.Time  `json:"expires_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
}

func (db *DB) CreateSession(ctx context.Context, userID uuid.UUID, refreshToken, userAgent, ipAddress string) (*Session, error) {
	var session Session
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO garance_auth.sessions (user_id, refresh_token, user_agent, ip_address, expires_at)
		 VALUES ($1, $2, $3, $4, now() + interval '30 days')
		 RETURNING id, user_id, refresh_token, user_agent, ip_address, created_at, expires_at, revoked_at`,
		userID, refreshToken, nilIfEmpty(userAgent), nilIfEmpty(ipAddress),
	).Scan(&session.ID, &session.UserID, &session.RefreshToken, &session.UserAgent,
		&session.IPAddress, &session.CreatedAt, &session.ExpiresAt, &session.RevokedAt)
	return &session, err
}

func (db *DB) GetSessionByRefreshToken(ctx context.Context, refreshToken string) (*Session, error) {
	var session Session
	err := db.Pool.QueryRow(ctx,
		`SELECT id, user_id, refresh_token, user_agent, ip_address, created_at, expires_at, revoked_at
		 FROM garance_auth.sessions
		 WHERE refresh_token = $1 AND revoked_at IS NULL AND expires_at > now()`,
		refreshToken,
	).Scan(&session.ID, &session.UserID, &session.RefreshToken, &session.UserAgent,
		&session.IPAddress, &session.CreatedAt, &session.ExpiresAt, &session.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	return &session, err
}

func (db *DB) RevokeSession(ctx context.Context, id uuid.UUID) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE garance_auth.sessions SET revoked_at = now() WHERE id = $1`, id)
	return err
}

func (db *DB) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE garance_auth.sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
