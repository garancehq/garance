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
	ErrBucketNotFound      = errors.New("bucket not found")
	ErrBucketAlreadyExists = errors.New("bucket already exists")
)

type Bucket struct {
	ID               uuid.UUID `json:"id"`
	Name             string    `json:"name"`
	IsPublic         bool      `json:"is_public"`
	MaxFileSize      *int64    `json:"max_file_size,omitempty"`
	AllowedMimeTypes []string  `json:"allowed_mime_types,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (db *DB) CreateBucket(ctx context.Context, name string, isPublic bool, maxFileSize *int64, allowedMimeTypes []string) (*Bucket, error) {
	var bucket Bucket
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO garance_storage.buckets (name, is_public, max_file_size, allowed_mime_types)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, name, is_public, max_file_size, allowed_mime_types, created_at, updated_at`,
		name, isPublic, maxFileSize, allowedMimeTypes,
	).Scan(&bucket.ID, &bucket.Name, &bucket.IsPublic, &bucket.MaxFileSize,
		&bucket.AllowedMimeTypes, &bucket.CreatedAt, &bucket.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrBucketAlreadyExists
		}
		return nil, err
	}
	return &bucket, nil
}

func (db *DB) GetBucketByName(ctx context.Context, name string) (*Bucket, error) {
	var bucket Bucket
	err := db.Pool.QueryRow(ctx,
		`SELECT id, name, is_public, max_file_size, allowed_mime_types, created_at, updated_at
		 FROM garance_storage.buckets WHERE name = $1`, name,
	).Scan(&bucket.ID, &bucket.Name, &bucket.IsPublic, &bucket.MaxFileSize,
		&bucket.AllowedMimeTypes, &bucket.CreatedAt, &bucket.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBucketNotFound
	}
	return &bucket, err
}

func (db *DB) ListBuckets(ctx context.Context) ([]Bucket, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, name, is_public, max_file_size, allowed_mime_types, created_at, updated_at
		 FROM garance_storage.buckets ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []Bucket
	for rows.Next() {
		var b Bucket
		if err := rows.Scan(&b.ID, &b.Name, &b.IsPublic, &b.MaxFileSize,
			&b.AllowedMimeTypes, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		buckets = append(buckets, b)
	}
	return buckets, nil
}

func (db *DB) DeleteBucket(ctx context.Context, name string) error {
	tag, err := db.Pool.Exec(ctx, `DELETE FROM garance_storage.buckets WHERE name = $1`, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrBucketNotFound
	}
	return nil
}
