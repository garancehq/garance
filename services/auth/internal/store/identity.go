package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrIdentityNotFound = errors.New("identity not found")

type Identity struct {
	ID             uuid.UUID `json:"id"`
	UserID         uuid.UUID `json:"user_id"`
	Provider       string    `json:"provider"`
	ProviderUserID string    `json:"provider_user_id"`
	ProviderData   []byte    `json:"provider_data"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (db *DB) CreateIdentity(ctx context.Context, userID uuid.UUID, provider, providerUserID string, providerData []byte) (*Identity, error) {
	var identity Identity
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO garance_auth.identities (user_id, provider, provider_user_id, provider_data)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, provider, provider_user_id, provider_data, created_at, updated_at`,
		userID, provider, providerUserID, providerData,
	).Scan(&identity.ID, &identity.UserID, &identity.Provider, &identity.ProviderUserID,
		&identity.ProviderData, &identity.CreatedAt, &identity.UpdatedAt)
	return &identity, err
}

func (db *DB) GetIdentityByProvider(ctx context.Context, provider, providerUserID string) (*Identity, error) {
	var identity Identity
	err := db.Pool.QueryRow(ctx,
		`SELECT id, user_id, provider, provider_user_id, provider_data, created_at, updated_at
		 FROM garance_auth.identities WHERE provider = $1 AND provider_user_id = $2`,
		provider, providerUserID,
	).Scan(&identity.ID, &identity.UserID, &identity.Provider, &identity.ProviderUserID,
		&identity.ProviderData, &identity.CreatedAt, &identity.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrIdentityNotFound
	}
	return &identity, err
}

func (db *DB) UpdateIdentityProviderData(ctx context.Context, provider, providerUserID string, providerData []byte) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE garance_auth.identities SET provider_data = $1, updated_at = now()
		 WHERE provider = $2 AND provider_user_id = $3`,
		providerData, provider, providerUserID)
	return err
}
