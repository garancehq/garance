package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrTokenNotFound = errors.New("verification token not found")

type VerificationToken struct {
	ID        uuid.UUID  `json:"id"`
	UserID    *uuid.UUID `json:"user_id,omitempty"`
	Email     string     `json:"email"`
	Token     string     `json:"-"`
	Type      string     `json:"type"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
}

func (db *DB) CreateVerificationToken(ctx context.Context, userID *uuid.UUID, email, token, tokenType string, ttl time.Duration) (*VerificationToken, error) {
	var vt VerificationToken
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO garance_auth.verification_tokens (user_id, email, token, type, expires_at)
		 VALUES ($1, $2, $3, $4, now() + $5::interval)
		 RETURNING id, user_id, email, token, type, created_at, expires_at, used_at`,
		userID, email, token, tokenType, ttl.String(),
	).Scan(&vt.ID, &vt.UserID, &vt.Email, &vt.Token, &vt.Type,
		&vt.CreatedAt, &vt.ExpiresAt, &vt.UsedAt)
	return &vt, err
}

func (db *DB) GetVerificationToken(ctx context.Context, token string) (*VerificationToken, error) {
	var vt VerificationToken
	err := db.Pool.QueryRow(ctx,
		`SELECT id, user_id, email, token, type, created_at, expires_at, used_at
		 FROM garance_auth.verification_tokens
		 WHERE token = $1 AND used_at IS NULL AND expires_at > now()`,
		token,
	).Scan(&vt.ID, &vt.UserID, &vt.Email, &vt.Token, &vt.Type,
		&vt.CreatedAt, &vt.ExpiresAt, &vt.UsedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTokenNotFound
	}
	return &vt, err
}

func (db *DB) MarkTokenUsed(ctx context.Context, id uuid.UUID) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE garance_auth.verification_tokens SET used_at = now() WHERE id = $1`, id)
	return err
}
