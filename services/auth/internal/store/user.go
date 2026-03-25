package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrEmailAlreadyTaken = errors.New("email already taken")
)

type User struct {
	ID                uuid.UUID  `json:"id"`
	Email             string     `json:"email"`
	EncryptedPassword *string    `json:"-"`
	EmailVerified     bool       `json:"email_verified"`
	Role              string     `json:"role"`
	Metadata          []byte     `json:"metadata"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	BannedAt          *time.Time `json:"banned_at,omitempty"`
}

func (db *DB) CreateUser(ctx context.Context, email string, encryptedPassword *string) (*User, error) {
	var user User
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO garance_auth.users (email, encrypted_password)
		 VALUES ($1, $2)
		 RETURNING id, email, encrypted_password, email_verified, role, metadata, created_at, updated_at, banned_at`,
		email, encryptedPassword,
	).Scan(&user.ID, &user.Email, &user.EncryptedPassword, &user.EmailVerified,
		&user.Role, &user.Metadata, &user.CreatedAt, &user.UpdatedAt, &user.BannedAt)
	if err != nil {
		if isDuplicateKey(err) {
			return nil, ErrEmailAlreadyTaken
		}
		return nil, err
	}
	return &user, nil
}

func (db *DB) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var user User
	err := db.Pool.QueryRow(ctx,
		`SELECT id, email, encrypted_password, email_verified, role, metadata, created_at, updated_at, banned_at
		 FROM garance_auth.users WHERE id = $1`, id,
	).Scan(&user.ID, &user.Email, &user.EncryptedPassword, &user.EmailVerified,
		&user.Role, &user.Metadata, &user.CreatedAt, &user.UpdatedAt, &user.BannedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &user, err
}

func (db *DB) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := db.Pool.QueryRow(ctx,
		`SELECT id, email, encrypted_password, email_verified, role, metadata, created_at, updated_at, banned_at
		 FROM garance_auth.users WHERE email = $1`, email,
	).Scan(&user.ID, &user.Email, &user.EncryptedPassword, &user.EmailVerified,
		&user.Role, &user.Metadata, &user.CreatedAt, &user.UpdatedAt, &user.BannedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &user, err
}

func (db *DB) VerifyUserEmail(ctx context.Context, id uuid.UUID) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE garance_auth.users SET email_verified = TRUE, updated_at = now() WHERE id = $1`, id)
	return err
}

func (db *DB) UpdateUserPassword(ctx context.Context, id uuid.UUID, encryptedPassword string) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE garance_auth.users SET encrypted_password = $1, updated_at = now() WHERE id = $2`,
		encryptedPassword, id)
	return err
}

func (db *DB) DeleteUser(ctx context.Context, id uuid.UUID) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM garance_auth.users WHERE id = $1`, id)
	return err
}

func isDuplicateKey(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" // unique_violation
	}
	return false
}
