package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrProviderNotFound      = errors.New("provider not found")
	ErrProviderAlreadyExists = errors.New("provider already exists")
)

type OAuthProvider struct {
	ID                    uuid.UUID `json:"id"`
	Provider              string    `json:"provider"`
	ClientID              string    `json:"client_id"`
	ClientSecretEncrypted string    `json:"-"`
	Enabled               bool      `json:"enabled"`
	Scopes                string    `json:"scopes"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

func (db *DB) CreateProvider(ctx context.Context, provider, clientID, clientSecretEncrypted, scopes string) (*OAuthProvider, error) {
	var p OAuthProvider
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO garance_platform.oauth_providers (provider, client_id, client_secret_encrypted, scopes)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, provider, client_id, client_secret_encrypted, enabled, scopes, created_at, updated_at`,
		provider, clientID, clientSecretEncrypted, scopes,
	).Scan(&p.ID, &p.Provider, &p.ClientID, &p.ClientSecretEncrypted, &p.Enabled, &p.Scopes, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrProviderAlreadyExists
		}
		return nil, err
	}
	return &p, nil
}

func (db *DB) GetProvider(ctx context.Context, provider string) (*OAuthProvider, error) {
	var p OAuthProvider
	err := db.Pool.QueryRow(ctx,
		`SELECT id, provider, client_id, client_secret_encrypted, enabled, scopes, created_at, updated_at
		 FROM garance_platform.oauth_providers WHERE provider = $1`,
		provider,
	).Scan(&p.ID, &p.Provider, &p.ClientID, &p.ClientSecretEncrypted, &p.Enabled, &p.Scopes, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProviderNotFound
	}
	return &p, err
}

func (db *DB) ListProviders(ctx context.Context) ([]OAuthProvider, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, provider, client_id, client_secret_encrypted, enabled, scopes, created_at, updated_at
		 FROM garance_platform.oauth_providers ORDER BY provider`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []OAuthProvider
	for rows.Next() {
		var p OAuthProvider
		if err := rows.Scan(&p.ID, &p.Provider, &p.ClientID, &p.ClientSecretEncrypted, &p.Enabled, &p.Scopes, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	return providers, nil
}

func (db *DB) UpdateProvider(ctx context.Context, provider string, clientID, clientSecretEncrypted *string, enabled *bool, scopes *string) error {
	// Build dynamic UPDATE
	query := "UPDATE garance_platform.oauth_providers SET updated_at = now()"
	args := []interface{}{}
	argIdx := 1

	if clientID != nil {
		query += fmt.Sprintf(", client_id = $%d", argIdx)
		args = append(args, *clientID)
		argIdx++
	}
	if clientSecretEncrypted != nil {
		query += fmt.Sprintf(", client_secret_encrypted = $%d", argIdx)
		args = append(args, *clientSecretEncrypted)
		argIdx++
	}
	if enabled != nil {
		query += fmt.Sprintf(", enabled = $%d", argIdx)
		args = append(args, *enabled)
		argIdx++
	}
	if scopes != nil {
		query += fmt.Sprintf(", scopes = $%d", argIdx)
		args = append(args, *scopes)
		argIdx++
	}

	query += fmt.Sprintf(" WHERE provider = $%d", argIdx)
	args = append(args, provider)

	tag, err := db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrProviderNotFound
	}
	return nil
}

func (db *DB) DeleteProvider(ctx context.Context, provider string) error {
	tag, err := db.Pool.Exec(ctx, `DELETE FROM garance_platform.oauth_providers WHERE provider = $1`, provider)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrProviderNotFound
	}
	return nil
}
