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
	ErrFileNotFound      = errors.New("file not found")
	ErrFileAlreadyExists = errors.New("file already exists")
)

type File struct {
	ID        uuid.UUID  `json:"id"`
	BucketID  uuid.UUID  `json:"bucket_id"`
	Name      string     `json:"name"`
	Size      int64      `json:"size"`
	MimeType  string     `json:"mime_type"`
	OwnerID   *uuid.UUID `json:"owner_id,omitempty"`
	Metadata  []byte     `json:"metadata"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (db *DB) CreateFile(ctx context.Context, bucketID uuid.UUID, name string, size int64, mimeType string, ownerID *uuid.UUID) (*File, error) {
	var file File
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO garance_storage.files (bucket_id, name, size, mime_type, owner_id)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, bucket_id, name, size, mime_type, owner_id, metadata, created_at, updated_at`,
		bucketID, name, size, mimeType, ownerID,
	).Scan(&file.ID, &file.BucketID, &file.Name, &file.Size, &file.MimeType,
		&file.OwnerID, &file.Metadata, &file.CreatedAt, &file.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrFileAlreadyExists
		}
		return nil, err
	}
	return &file, nil
}

func (db *DB) GetFile(ctx context.Context, bucketID uuid.UUID, name string) (*File, error) {
	var file File
	err := db.Pool.QueryRow(ctx,
		`SELECT id, bucket_id, name, size, mime_type, owner_id, metadata, created_at, updated_at
		 FROM garance_storage.files WHERE bucket_id = $1 AND name = $2`,
		bucketID, name,
	).Scan(&file.ID, &file.BucketID, &file.Name, &file.Size, &file.MimeType,
		&file.OwnerID, &file.Metadata, &file.CreatedAt, &file.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrFileNotFound
	}
	return &file, err
}

func (db *DB) ListFiles(ctx context.Context, bucketID uuid.UUID, prefix string, limit, offset int) ([]File, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, bucket_id, name, size, mime_type, owner_id, metadata, created_at, updated_at
		 FROM garance_storage.files
		 WHERE bucket_id = $1 AND ($2 = '' OR name LIKE $2 || '%')
		 ORDER BY name
		 LIMIT $3 OFFSET $4`,
		bucketID, prefix, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.BucketID, &f.Name, &f.Size, &f.MimeType,
			&f.OwnerID, &f.Metadata, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

func (db *DB) DeleteFile(ctx context.Context, bucketID uuid.UUID, name string) error {
	tag, err := db.Pool.Exec(ctx,
		`DELETE FROM garance_storage.files WHERE bucket_id = $1 AND name = $2`,
		bucketID, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrFileNotFound
	}
	return nil
}
