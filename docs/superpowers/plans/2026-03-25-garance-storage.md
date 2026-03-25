# Garance Storage Service — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Garance Storage Service in Go — manages file uploads/downloads, S3-compatible storage backend, bucket management, signed URLs, and file metadata in PostgreSQL.

**Architecture:** Single Go module in `services/storage/` with packages: `handler` (HTTP routes), `service` (business logic), `store` (PostgreSQL metadata), `s3` (S3 client abstraction). File binaries go to S3 (MinIO for dev/self-host, Scaleway for SaaS). Metadata (name, size, mime type, bucket, owner) is stored in the `garance_storage` PostgreSQL schema.

**Tech Stack:** Go 1.25+, net/http (HTTP), pgx v5 (PostgreSQL), minio-go v7 (S3 client), testcontainers-go (integration tests)

**Spec:** `docs/superpowers/specs/2026-03-25-garance-baas-design.md` (sections 7, 13)

---

## Task 1: Go Module & S3 Client

**Files:**
- Create: `services/storage/go.mod`
- Create: `services/storage/main.go`
- Create: `services/storage/internal/config/config.go`
- Create: `services/storage/internal/s3/client.go`
- Create: `services/storage/internal/s3/client_test.go`
- Modify: `services/go.work` — add `./storage`

- [ ] **Step 1: Initialize Go module**

```bash
mkdir -p /Users/jh3ady/Development/Projects/garance/services/storage/internal/config
mkdir -p /Users/jh3ady/Development/Projects/garance/services/storage/internal/s3
cd /Users/jh3ady/Development/Projects/garance/services/storage
go mod init github.com/garancehq/garance/services/storage
```

- [ ] **Step 2: Update Go workspace**

```go
// services/go.work
go 1.25

use (
    ./auth
    ./storage
)
```

Adjust the go version to match what's already in go.work.

- [ ] **Step 3: Write config**

```go
// services/storage/internal/config/config.go
package config

import "os"

type Config struct {
	DatabaseURL   string
	ListenAddr    string
	S3Endpoint    string
	S3Region      string
	S3Bucket      string
	S3AccessKey   string
	S3SecretKey   string
	S3UseSSL      bool
	BaseURL       string
	JWTSecret     string
}

func Load() *Config {
	return &Config{
		DatabaseURL:   getEnv("DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/postgres"),
		ListenAddr:    getEnv("LISTEN_ADDR", "0.0.0.0:4002"),
		S3Endpoint:    getEnv("S3_ENDPOINT", "localhost:9000"),
		S3Region:      getEnv("S3_REGION", "us-east-1"),
		S3Bucket:      getEnv("S3_BUCKET", "garance"),
		S3AccessKey:   getEnv("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey:   getEnv("S3_SECRET_KEY", "minioadmin"),
		S3UseSSL:      getEnv("S3_USE_SSL", "false") == "true",
		BaseURL:       getEnv("BASE_URL", "http://localhost:4002"),
		JWTSecret:     getEnv("JWT_SECRET", "dev-secret-change-me"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
```

- [ ] **Step 4: Write S3 client abstraction**

```go
// services/storage/internal/s3/client.go
package s3

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	mc     *minio.Client
	bucket string
}

func NewClient(endpoint, accessKey, secretKey, bucket, region string, useSSL bool) (*Client, error) {
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}
	return &Client{mc: mc, bucket: bucket}, nil
}

// EnsureBucket creates the root bucket if it doesn't exist.
func (c *Client) EnsureBucket(ctx context.Context) error {
	exists, err := c.mc.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket: %w", err)
	}
	if !exists {
		if err := c.mc.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
	}
	return nil
}

// Upload stores a file in S3. The key is prefixed with the bucket name for namespacing.
// e.g., "avatars/user-123/photo.jpg"
func (c *Client) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	_, err := c.mc.PutObject(ctx, c.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}
	return nil
}

// Download retrieves a file from S3.
func (c *Client) Download(ctx context.Context, key string) (io.ReadCloser, string, error) {
	obj, err := c.mc.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get object: %w", err)
	}
	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, "", fmt.Errorf("failed to stat object: %w", err)
	}
	return obj, info.ContentType, nil
}

// Delete removes a file from S3.
func (c *Client) Delete(ctx context.Context, key string) error {
	err := c.mc.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// SignedURL generates a pre-signed URL for temporary access.
func (c *Client) SignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	reqParams := make(url.Values)
	presignedURL, err := c.mc.PresignedGetObject(ctx, c.bucket, key, expiry, reqParams)
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %w", err)
	}
	return presignedURL.String(), nil
}

// Exists checks if an object exists in S3.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	_, err := c.mc.StatObject(ctx, c.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
```

- [ ] **Step 5: Write S3 integration test**

```go
// services/storage/internal/s3/client_test.go
package s3_test

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	s3client "github.com/garancehq/garance/services/storage/internal/s3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupMinio(t *testing.T) *s3client.Client {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "minio/minio",
		ExposedPorts: []string{"9000/tcp"},
		Cmd:          []string{"server", "/data"},
		Env: map[string]string{
			"MINIO_ROOT_USER":     "minioadmin",
			"MINIO_ROOT_PASSWORD": "minioadmin",
		},
		WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start minio: %v", err)
	}
	t.Cleanup(func() { container.Terminate(ctx) })

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "9000")

	client, err := s3client.NewClient(
		host+":"+port.Port(),
		"minioadmin", "minioadmin",
		"test-bucket", "us-east-1", false,
	)
	if err != nil {
		t.Fatalf("failed to create S3 client: %v", err)
	}

	if err := client.EnsureBucket(ctx); err != nil {
		t.Fatalf("failed to ensure bucket: %v", err)
	}

	return client
}

func TestUploadAndDownload(t *testing.T) {
	client := setupMinio(t)
	ctx := context.Background()

	content := []byte("hello garance storage")
	err := client.Upload(ctx, "test/hello.txt", bytes.NewReader(content), int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	reader, contentType, err := client.Download(ctx, "test/hello.txt")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	defer reader.Close()

	data, _ := io.ReadAll(reader)
	if string(data) != "hello garance storage" {
		t.Errorf("expected 'hello garance storage', got '%s'", string(data))
	}
	if contentType != "text/plain" {
		t.Errorf("expected content-type text/plain, got %s", contentType)
	}
}

func TestDelete(t *testing.T) {
	client := setupMinio(t)
	ctx := context.Background()

	content := []byte("to delete")
	client.Upload(ctx, "test/delete-me.txt", bytes.NewReader(content), int64(len(content)), "text/plain")

	exists, _ := client.Exists(ctx, "test/delete-me.txt")
	if !exists {
		t.Fatal("file should exist before delete")
	}

	err := client.Delete(ctx, "test/delete-me.txt")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	exists, _ = client.Exists(ctx, "test/delete-me.txt")
	if exists {
		t.Error("file should not exist after delete")
	}
}

func TestSignedURL(t *testing.T) {
	client := setupMinio(t)
	ctx := context.Background()

	content := []byte("signed content")
	client.Upload(ctx, "test/signed.txt", bytes.NewReader(content), int64(len(content)), "text/plain")

	url, err := client.SignedURL(ctx, "test/signed.txt", 1*time.Hour)
	if err != nil {
		t.Fatalf("SignedURL: %v", err)
	}
	if url == "" {
		t.Error("signed URL should not be empty")
	}
}

func TestExistsNonexistent(t *testing.T) {
	client := setupMinio(t)
	ctx := context.Background()

	exists, err := client.Exists(ctx, "nonexistent/file.txt")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Error("nonexistent file should return false")
	}
}
```

- [ ] **Step 6: Add dependencies and main.go stub**

```bash
cd /Users/jh3ady/Development/Projects/garance/services/storage
go get github.com/minio/minio-go/v7
go get github.com/jackc/pgx/v5
go get github.com/google/uuid
go get github.com/golang-jwt/jwt/v5
go get github.com/testcontainers/testcontainers-go
```

```go
// services/storage/main.go
package main

import (
	"log"
	"net/http"

	"github.com/garancehq/garance/services/storage/internal/config"
)

func main() {
	cfg := config.Load()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	log.Printf("garance storage service listening on %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 7: Verify build and tests**

Run: `cd /Users/jh3ady/Development/Projects/garance/services/storage && go build ./...`
Run: `go test ./internal/s3/ -v -count=1` — 4 tests pass (requires Docker)

- [ ] **Step 8: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add services/
git commit -m ":tada: feat(storage): initialize Go module with S3 client and integration tests"
```

---

## Task 2: Database Schema & File Metadata Store

**Files:**
- Create: `services/storage/migrations/001_initial.sql`
- Create: `services/storage/internal/store/db.go`
- Create: `services/storage/internal/store/bucket.go`
- Create: `services/storage/internal/store/file.go`
- Create: `services/storage/internal/store/store_test.go`

- [ ] **Step 1: Write migration SQL**

```sql
-- services/storage/migrations/001_initial.sql
CREATE SCHEMA IF NOT EXISTS garance_storage;

CREATE TABLE garance_storage.buckets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL,
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    max_file_size BIGINT, -- in bytes, NULL = no limit
    allowed_mime_types TEXT[], -- NULL = all types allowed
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE garance_storage.files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bucket_id UUID NOT NULL REFERENCES garance_storage.buckets(id) ON DELETE CASCADE,
    name TEXT NOT NULL, -- full path within bucket, e.g. "user-123/photo.jpg"
    size BIGINT NOT NULL,
    mime_type TEXT NOT NULL,
    owner_id UUID, -- user who uploaded the file
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (bucket_id, name)
);

CREATE INDEX idx_files_bucket_id ON garance_storage.files(bucket_id);
CREATE INDEX idx_files_owner_id ON garance_storage.files(owner_id);
```

- [ ] **Step 2: Write database connection**

```go
// services/storage/internal/store/db.go
package store

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func NewDB(ctx context.Context, databaseURL string) (*DB, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return &DB{Pool: pool}, nil
}

func (db *DB) Close() {
	db.Pool.Close()
}

func (db *DB) RunMigrationsFromDir(ctx context.Context, dir string) error {
	sql, err := os.ReadFile(dir + "/001_initial.sql")
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}
	_, err = db.Pool.Exec(ctx, string(sql))
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Write bucket store**

```go
// services/storage/internal/store/bucket.go
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
```

- [ ] **Step 4: Write file metadata store**

```go
// services/storage/internal/store/file.go
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
```

- [ ] **Step 5: Write integration test**

```go
// services/storage/internal/store/store_test.go
package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/garancehq/garance/services/storage/internal/store"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupDB(t *testing.T) *store.DB {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start postgres: %v", err)
	}
	t.Cleanup(func() { pgContainer.Terminate(ctx) })

	connStr, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")
	db, err := store.NewDB(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.RunMigrationsFromDir(ctx, "../../migrations"); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return db
}

func TestBucketCRUD(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	bucket, err := db.CreateBucket(ctx, "avatars", true, nil, nil)
	if err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	if bucket.Name != "avatars" {
		t.Errorf("expected name avatars, got %s", bucket.Name)
	}
	if !bucket.IsPublic {
		t.Error("expected public bucket")
	}

	found, err := db.GetBucketByName(ctx, "avatars")
	if err != nil {
		t.Fatalf("GetBucketByName: %v", err)
	}
	if found.ID != bucket.ID {
		t.Error("bucket IDs don't match")
	}

	buckets, err := db.ListBuckets(ctx)
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	if len(buckets) != 1 {
		t.Errorf("expected 1 bucket, got %d", len(buckets))
	}

	err = db.DeleteBucket(ctx, "avatars")
	if err != nil {
		t.Fatalf("DeleteBucket: %v", err)
	}

	_, err = db.GetBucketByName(ctx, "avatars")
	if err != store.ErrBucketNotFound {
		t.Errorf("expected ErrBucketNotFound, got %v", err)
	}
}

func TestBucketDuplicate(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	db.CreateBucket(ctx, "photos", false, nil, nil)
	_, err := db.CreateBucket(ctx, "photos", false, nil, nil)
	if err != store.ErrBucketAlreadyExists {
		t.Errorf("expected ErrBucketAlreadyExists, got %v", err)
	}
}

func TestBucketWithConstraints(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	maxSize := int64(5 * 1024 * 1024) // 5MB
	mimeTypes := []string{"image/jpeg", "image/png"}
	bucket, err := db.CreateBucket(ctx, "images", true, &maxSize, mimeTypes)
	if err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	if bucket.MaxFileSize == nil || *bucket.MaxFileSize != maxSize {
		t.Error("max_file_size mismatch")
	}
	if len(bucket.AllowedMimeTypes) != 2 {
		t.Errorf("expected 2 allowed mime types, got %d", len(bucket.AllowedMimeTypes))
	}
}

func TestFileCRUD(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	bucket, _ := db.CreateBucket(ctx, "docs", false, nil, nil)
	ownerID := uuid.New()

	file, err := db.CreateFile(ctx, bucket.ID, "report.pdf", 1024, "application/pdf", &ownerID)
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	if file.Name != "report.pdf" {
		t.Errorf("expected name report.pdf, got %s", file.Name)
	}
	if file.Size != 1024 {
		t.Errorf("expected size 1024, got %d", file.Size)
	}

	found, err := db.GetFile(ctx, bucket.ID, "report.pdf")
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if found.ID != file.ID {
		t.Error("file IDs don't match")
	}

	files, err := db.ListFiles(ctx, bucket.ID, "", 100, 0)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}

	err = db.DeleteFile(ctx, bucket.ID, "report.pdf")
	if err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}

	_, err = db.GetFile(ctx, bucket.ID, "report.pdf")
	if err != store.ErrFileNotFound {
		t.Errorf("expected ErrFileNotFound, got %v", err)
	}
}

func TestFileDuplicate(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	bucket, _ := db.CreateBucket(ctx, "dup-test", false, nil, nil)
	db.CreateFile(ctx, bucket.ID, "same.txt", 100, "text/plain", nil)
	_, err := db.CreateFile(ctx, bucket.ID, "same.txt", 200, "text/plain", nil)
	if err != store.ErrFileAlreadyExists {
		t.Errorf("expected ErrFileAlreadyExists, got %v", err)
	}
}

func TestListFilesWithPrefix(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	bucket, _ := db.CreateBucket(ctx, "prefix-test", false, nil, nil)
	db.CreateFile(ctx, bucket.ID, "photos/a.jpg", 100, "image/jpeg", nil)
	db.CreateFile(ctx, bucket.ID, "photos/b.jpg", 200, "image/jpeg", nil)
	db.CreateFile(ctx, bucket.ID, "docs/c.pdf", 300, "application/pdf", nil)

	files, err := db.ListFiles(ctx, bucket.ID, "photos/", 100, 0)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files with prefix photos/, got %d", len(files))
	}
}
```

- [ ] **Step 6: Run tests**

Run: `cd /Users/jh3ady/Development/Projects/garance/services/storage && go test ./internal/store/ -v -count=1`
Expected: 6 tests pass.

- [ ] **Step 7: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add services/
git commit -m ":sparkles: feat(storage): add database schema, bucket and file metadata store"
```

---

## Task 3: Storage Service Layer

**Files:**
- Create: `services/storage/internal/service/storage.go`
- Create: `services/storage/internal/service/storage_test.go`

- [ ] **Step 1: Write storage service**

```go
// services/storage/internal/service/storage.go
package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/garancehq/garance/services/storage/internal/s3"
	"github.com/garancehq/garance/services/storage/internal/store"
	"github.com/google/uuid"
)

var (
	ErrFileTooLarge     = errors.New("file exceeds maximum allowed size")
	ErrMimeTypeNotAllowed = errors.New("file type not allowed")
)

type StorageService struct {
	db *store.DB
	s3 *s3.Client
}

func NewStorageService(db *store.DB, s3Client *s3.Client) *StorageService {
	return &StorageService{db: db, s3: s3Client}
}

// CreateBucket creates a new storage bucket.
func (s *StorageService) CreateBucket(ctx context.Context, name string, isPublic bool, maxFileSize *int64, allowedMimeTypes []string) (*store.Bucket, error) {
	return s.db.CreateBucket(ctx, name, isPublic, maxFileSize, allowedMimeTypes)
}

// ListBuckets returns all buckets.
func (s *StorageService) ListBuckets(ctx context.Context) ([]store.Bucket, error) {
	return s.db.ListBuckets(ctx)
}

// GetBucket returns a bucket by name.
func (s *StorageService) GetBucket(ctx context.Context, name string) (*store.Bucket, error) {
	return s.db.GetBucketByName(ctx, name)
}

// DeleteBucket removes a bucket and all its files.
func (s *StorageService) DeleteBucket(ctx context.Context, name string) error {
	// TODO: also delete S3 objects for this bucket
	return s.db.DeleteBucket(ctx, name)
}

// Upload stores a file in S3 and records metadata in PG.
func (s *StorageService) Upload(ctx context.Context, bucketName, fileName string, reader io.Reader, size int64, mimeType string, ownerID *uuid.UUID) (*store.File, error) {
	bucket, err := s.db.GetBucketByName(ctx, bucketName)
	if err != nil {
		return nil, err
	}

	// Validate constraints
	if bucket.MaxFileSize != nil && size > *bucket.MaxFileSize {
		return nil, ErrFileTooLarge
	}
	if len(bucket.AllowedMimeTypes) > 0 {
		allowed := false
		for _, mt := range bucket.AllowedMimeTypes {
			if mt == mimeType {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, ErrMimeTypeNotAllowed
		}
	}

	// Upload to S3
	s3Key := fmt.Sprintf("%s/%s", bucketName, fileName)
	if err := s.s3.Upload(ctx, s3Key, reader, size, mimeType); err != nil {
		return nil, fmt.Errorf("failed to upload to S3: %w", err)
	}

	// Record metadata in PG
	file, err := s.db.CreateFile(ctx, bucket.ID, fileName, size, mimeType, ownerID)
	if err != nil {
		// Best effort cleanup: delete from S3 if metadata insert fails
		s.s3.Delete(ctx, s3Key)
		return nil, err
	}

	return file, nil
}

// Download retrieves a file from S3.
func (s *StorageService) Download(ctx context.Context, bucketName, fileName string) (io.ReadCloser, *store.File, error) {
	bucket, err := s.db.GetBucketByName(ctx, bucketName)
	if err != nil {
		return nil, nil, err
	}

	file, err := s.db.GetFile(ctx, bucket.ID, fileName)
	if err != nil {
		return nil, nil, err
	}

	s3Key := fmt.Sprintf("%s/%s", bucketName, fileName)
	reader, _, err := s.s3.Download(ctx, s3Key)
	if err != nil {
		return nil, nil, err
	}

	return reader, file, nil
}

// Delete removes a file from S3 and PG.
func (s *StorageService) Delete(ctx context.Context, bucketName, fileName string) error {
	bucket, err := s.db.GetBucketByName(ctx, bucketName)
	if err != nil {
		return err
	}

	s3Key := fmt.Sprintf("%s/%s", bucketName, fileName)
	if err := s.s3.Delete(ctx, s3Key); err != nil {
		return err
	}

	return s.db.DeleteFile(ctx, bucket.ID, fileName)
}

// ListFiles lists files in a bucket with optional prefix.
func (s *StorageService) ListFiles(ctx context.Context, bucketName, prefix string, limit, offset int) ([]store.File, error) {
	bucket, err := s.db.GetBucketByName(ctx, bucketName)
	if err != nil {
		return nil, err
	}
	return s.db.ListFiles(ctx, bucket.ID, prefix, limit, offset)
}

// SignedURL generates a temporary download URL.
func (s *StorageService) SignedURL(ctx context.Context, bucketName, fileName string, expiry time.Duration) (string, error) {
	bucket, err := s.db.GetBucketByName(ctx, bucketName)
	if err != nil {
		return "", err
	}

	// Verify file exists
	if _, err := s.db.GetFile(ctx, bucket.ID, fileName); err != nil {
		return "", err
	}

	s3Key := fmt.Sprintf("%s/%s", bucketName, fileName)
	return s.s3.SignedURL(ctx, s3Key, expiry)
}

// PublicURL returns the public URL for a file in a public bucket.
func (s *StorageService) PublicURL(bucketName, fileName, baseURL string) string {
	return fmt.Sprintf("%s/storage/v1/%s/%s", baseURL, bucketName, fileName)
}
```

- [ ] **Step 2: Write integration test** (requires both MinIO and PostgreSQL)

```go
// services/storage/internal/service/storage_test.go
package service_test

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	s3client "github.com/garancehq/garance/services/storage/internal/s3"
	"github.com/garancehq/garance/services/storage/internal/service"
	"github.com/garancehq/garance/services/storage/internal/store"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setup(t *testing.T) *service.StorageService {
	t.Helper()
	ctx := context.Background()

	// Start PostgreSQL
	pgContainer, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("postgres: %v", err)
	}
	t.Cleanup(func() { pgContainer.Terminate(ctx) })

	connStr, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")
	db, err := store.NewDB(ctx, connStr)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.RunMigrationsFromDir(ctx, "../../migrations"); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	// Start MinIO
	minioReq := testcontainers.ContainerRequest{
		Image:        "minio/minio",
		ExposedPorts: []string{"9000/tcp"},
		Cmd:          []string{"server", "/data"},
		Env:          map[string]string{"MINIO_ROOT_USER": "minioadmin", "MINIO_ROOT_PASSWORD": "minioadmin"},
		WaitingFor:   wait.ForHTTP("/minio/health/live").WithPort("9000").WithStartupTimeout(30 * time.Second),
	}
	minioContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: minioReq, Started: true,
	})
	if err != nil {
		t.Fatalf("minio: %v", err)
	}
	t.Cleanup(func() { minioContainer.Terminate(ctx) })

	host, _ := minioContainer.Host(ctx)
	port, _ := minioContainer.MappedPort(ctx, "9000")

	s3, err := s3client.NewClient(host+":"+port.Port(), "minioadmin", "minioadmin", "test-bucket", "us-east-1", false)
	if err != nil {
		t.Fatalf("s3: %v", err)
	}
	if err := s3.EnsureBucket(ctx); err != nil {
		t.Fatalf("ensure bucket: %v", err)
	}

	return service.NewStorageService(db, s3)
}

func TestUploadAndDownload(t *testing.T) {
	svc := setup(t)
	ctx := context.Background()

	svc.CreateBucket(ctx, "avatars", true, nil, nil)
	ownerID := uuid.New()

	content := []byte("avatar image data")
	file, err := svc.Upload(ctx, "avatars", "user-123/photo.jpg", bytes.NewReader(content), int64(len(content)), "image/jpeg", &ownerID)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if file.Name != "user-123/photo.jpg" {
		t.Errorf("expected name user-123/photo.jpg, got %s", file.Name)
	}

	reader, fileMeta, err := svc.Download(ctx, "avatars", "user-123/photo.jpg")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	defer reader.Close()

	data, _ := io.ReadAll(reader)
	if string(data) != "avatar image data" {
		t.Errorf("content mismatch")
	}
	if fileMeta.MimeType != "image/jpeg" {
		t.Errorf("expected mime type image/jpeg, got %s", fileMeta.MimeType)
	}
}

func TestUploadFileTooLarge(t *testing.T) {
	svc := setup(t)
	ctx := context.Background()

	maxSize := int64(100)
	svc.CreateBucket(ctx, "small", false, &maxSize, nil)

	content := make([]byte, 200)
	_, err := svc.Upload(ctx, "small", "big.txt", bytes.NewReader(content), int64(len(content)), "text/plain", nil)
	if err != service.ErrFileTooLarge {
		t.Errorf("expected ErrFileTooLarge, got %v", err)
	}
}

func TestUploadMimeTypeNotAllowed(t *testing.T) {
	svc := setup(t)
	ctx := context.Background()

	mimeTypes := []string{"image/jpeg", "image/png"}
	svc.CreateBucket(ctx, "images-only", false, nil, mimeTypes)

	content := []byte("not an image")
	_, err := svc.Upload(ctx, "images-only", "file.txt", bytes.NewReader(content), int64(len(content)), "text/plain", nil)
	if err != service.ErrMimeTypeNotAllowed {
		t.Errorf("expected ErrMimeTypeNotAllowed, got %v", err)
	}
}

func TestDeleteFile(t *testing.T) {
	svc := setup(t)
	ctx := context.Background()

	svc.CreateBucket(ctx, "delete-test", false, nil, nil)
	content := []byte("delete me")
	svc.Upload(ctx, "delete-test", "temp.txt", bytes.NewReader(content), int64(len(content)), "text/plain", nil)

	err := svc.Delete(ctx, "delete-test", "temp.txt")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, _, err = svc.Download(ctx, "delete-test", "temp.txt")
	if err != store.ErrFileNotFound {
		t.Errorf("expected ErrFileNotFound, got %v", err)
	}
}

func TestSignedURL(t *testing.T) {
	svc := setup(t)
	ctx := context.Background()

	svc.CreateBucket(ctx, "signed-test", false, nil, nil)
	content := []byte("signed content")
	svc.Upload(ctx, "signed-test", "doc.pdf", bytes.NewReader(content), int64(len(content)), "application/pdf", nil)

	url, err := svc.SignedURL(ctx, "signed-test", "doc.pdf", 1*time.Hour)
	if err != nil {
		t.Fatalf("SignedURL: %v", err)
	}
	if url == "" {
		t.Error("signed URL should not be empty")
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/jh3ady/Development/Projects/garance/services/storage && go test ./internal/service/ -v -count=1`
Expected: 5 tests pass.

- [ ] **Step 4: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add services/
git commit -m ":sparkles: feat(storage): add storage service with upload, download, signed URLs, and bucket constraints"
```

---

## Task 4: HTTP Handlers & Dockerfile

**Files:**
- Create: `services/storage/internal/handler/handler.go`
- Create: `services/storage/internal/handler/response.go`
- Create: `services/storage/internal/handler/middleware.go`
- Modify: `services/storage/main.go`
- Create: `services/storage/Dockerfile`

- [ ] **Step 1: Write response helpers** (same pattern as auth)

```go
// services/storage/internal/handler/response.go
package handler

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Status  int         `json:"status"`
	Details interface{} `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, code string, message string, status int) {
	writeJSON(w, status, ErrorResponse{
		Error: ErrorBody{Code: code, Message: message, Status: status},
	})
}
```

- [ ] **Step 2: Write auth middleware** (JWT validation, same as auth service)

```go
// services/storage/internal/handler/middleware.go
package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserIDKey contextKey = "user_id"

type JWTClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// OptionalAuth extracts the user ID from JWT if present, but doesn't require it.
// Used for public bucket access where auth is optional.
func OptionalAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				next.ServeHTTP(w, r)
				return
			}

			token, err := jwt.ParseWithClaims(parts[1], &JWTClaims{}, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method")
				}
				return []byte(secret), nil
			})
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
				ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth rejects requests without valid JWT.
func RequireAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, "UNAUTHORIZED", "missing authorization header", 401)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				writeError(w, "UNAUTHORIZED", "invalid authorization format", 401)
				return
			}

			token, err := jwt.ParseWithClaims(parts[1], &JWTClaims{}, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method")
				}
				return []byte(secret), nil
			})
			if err != nil {
				writeError(w, "UNAUTHORIZED", "invalid or expired token", 401)
				return
			}

			claims, ok := token.Claims.(*JWTClaims)
			if !ok || !token.Valid {
				writeError(w, "UNAUTHORIZED", "invalid token", 401)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Unused but required to prevent compiler error
var _ = time.Now
```

- [ ] **Step 3: Write storage handlers**

```go
// services/storage/internal/handler/handler.go
package handler

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/garancehq/garance/services/storage/internal/service"
	"github.com/garancehq/garance/services/storage/internal/store"
	"github.com/google/uuid"
)

type StorageHandler struct {
	svc     *service.StorageService
	baseURL string
	secret  string
}

func NewStorageHandler(svc *service.StorageService, baseURL, jwtSecret string) *StorageHandler {
	return &StorageHandler{svc: svc, baseURL: baseURL, secret: jwtSecret}
}

func (h *StorageHandler) RegisterRoutes(mux *http.ServeMux) {
	// Bucket management (requires auth)
	mux.HandleFunc("POST /storage/v1/buckets", h.requireAuth(h.CreateBucket))
	mux.HandleFunc("GET /storage/v1/buckets", h.requireAuth(h.ListBuckets))
	mux.HandleFunc("DELETE /storage/v1/buckets/{bucket}", h.requireAuth(h.DeleteBucket))

	// File operations
	mux.HandleFunc("POST /storage/v1/{bucket}/upload", h.requireAuth(h.Upload))
	mux.HandleFunc("GET /storage/v1/{bucket}/{path...}", h.optionalAuth(h.Download))
	mux.HandleFunc("DELETE /storage/v1/{bucket}/{path...}/delete", h.requireAuth(h.DeleteFile))

	// Signed URLs
	mux.HandleFunc("POST /storage/v1/{bucket}/signed-url", h.requireAuth(h.CreateSignedURL))

	// List files
	mux.HandleFunc("GET /storage/v1/{bucket}", h.requireAuth(h.ListFiles))
}

func (h *StorageHandler) requireAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		RequireAuth(h.secret)(http.HandlerFunc(handler)).ServeHTTP(w, r)
	}
}

func (h *StorageHandler) optionalAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		OptionalAuth(h.secret)(http.HandlerFunc(handler)).ServeHTTP(w, r)
	}
}

type createBucketRequest struct {
	Name             string   `json:"name"`
	IsPublic         bool     `json:"is_public"`
	MaxFileSize      *int64   `json:"max_file_size,omitempty"`
	AllowedMimeTypes []string `json:"allowed_mime_types,omitempty"`
}

func (h *StorageHandler) CreateBucket(w http.ResponseWriter, r *http.Request) {
	var req createBucketRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}
	if req.Name == "" {
		writeError(w, "VALIDATION_ERROR", "bucket name is required", 400)
		return
	}

	bucket, err := h.svc.CreateBucket(r.Context(), req.Name, req.IsPublic, req.MaxFileSize, req.AllowedMimeTypes)
	if err != nil {
		if errors.Is(err, store.ErrBucketAlreadyExists) {
			writeError(w, "CONFLICT", err.Error(), 409)
			return
		}
		writeError(w, "INTERNAL_ERROR", "failed to create bucket", 500)
		return
	}
	writeJSON(w, 201, bucket)
}

func (h *StorageHandler) ListBuckets(w http.ResponseWriter, r *http.Request) {
	buckets, err := h.svc.ListBuckets(r.Context())
	if err != nil {
		writeError(w, "INTERNAL_ERROR", "failed to list buckets", 500)
		return
	}
	if buckets == nil {
		buckets = []store.Bucket{}
	}
	writeJSON(w, 200, buckets)
}

func (h *StorageHandler) DeleteBucket(w http.ResponseWriter, r *http.Request) {
	bucketName := r.PathValue("bucket")
	if err := h.svc.DeleteBucket(r.Context(), bucketName); err != nil {
		if errors.Is(err, store.ErrBucketNotFound) {
			writeError(w, "NOT_FOUND", err.Error(), 404)
			return
		}
		writeError(w, "INTERNAL_ERROR", "failed to delete bucket", 500)
		return
	}
	w.WriteHeader(204)
}

func (h *StorageHandler) Upload(w http.ResponseWriter, r *http.Request) {
	bucketName := r.PathValue("bucket")

	// Limit upload size to 5GB
	r.Body = http.MaxBytesReader(w, r.Body, 5*1024*1024*1024)

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, "VALIDATION_ERROR", "file is required (multipart form field 'file')", 400)
		return
	}
	defer file.Close()

	fileName := r.FormValue("name")
	if fileName == "" {
		fileName = header.Filename
	}

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	var ownerID *uuid.UUID
	if uid, ok := r.Context().Value(UserIDKey).(string); ok {
		parsed, err := uuid.Parse(uid)
		if err == nil {
			ownerID = &parsed
		}
	}

	fileMeta, err := h.svc.Upload(r.Context(), bucketName, fileName, file, header.Size, mimeType, ownerID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrBucketNotFound):
			writeError(w, "NOT_FOUND", "bucket not found", 404)
		case errors.Is(err, service.ErrFileTooLarge):
			writeError(w, "VALIDATION_ERROR", err.Error(), 413)
		case errors.Is(err, service.ErrMimeTypeNotAllowed):
			writeError(w, "VALIDATION_ERROR", err.Error(), 415)
		case errors.Is(err, store.ErrFileAlreadyExists):
			writeError(w, "CONFLICT", err.Error(), 409)
		default:
			writeError(w, "INTERNAL_ERROR", "upload failed", 500)
		}
		return
	}

	writeJSON(w, 201, fileMeta)
}

func (h *StorageHandler) Download(w http.ResponseWriter, r *http.Request) {
	bucketName := r.PathValue("bucket")
	filePath := r.PathValue("path")

	reader, fileMeta, err := h.svc.Download(r.Context(), bucketName, filePath)
	if err != nil {
		if errors.Is(err, store.ErrBucketNotFound) || errors.Is(err, store.ErrFileNotFound) {
			writeError(w, "NOT_FOUND", "file not found", 404)
			return
		}
		writeError(w, "INTERNAL_ERROR", "download failed", 500)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", fileMeta.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(fileMeta.Size, 10))
	w.Header().Set("Content-Disposition", "inline")
	io.Copy(w, reader)
}

func (h *StorageHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	bucketName := r.PathValue("bucket")
	filePath := r.PathValue("path")

	if err := h.svc.Delete(r.Context(), bucketName, filePath); err != nil {
		if errors.Is(err, store.ErrBucketNotFound) || errors.Is(err, store.ErrFileNotFound) {
			writeError(w, "NOT_FOUND", "file not found", 404)
			return
		}
		writeError(w, "INTERNAL_ERROR", "delete failed", 500)
		return
	}
	w.WriteHeader(204)
}

type signedURLRequest struct {
	FileName  string `json:"file_name"`
	ExpiresIn int    `json:"expires_in"` // seconds
}

func (h *StorageHandler) CreateSignedURL(w http.ResponseWriter, r *http.Request) {
	bucketName := r.PathValue("bucket")

	var req signedURLRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}
	if req.FileName == "" {
		writeError(w, "VALIDATION_ERROR", "file_name is required", 400)
		return
	}
	if req.ExpiresIn <= 0 {
		req.ExpiresIn = 3600 // default 1 hour
	}

	url, err := h.svc.SignedURL(r.Context(), bucketName, req.FileName, time.Duration(req.ExpiresIn)*time.Second)
	if err != nil {
		if errors.Is(err, store.ErrBucketNotFound) || errors.Is(err, store.ErrFileNotFound) {
			writeError(w, "NOT_FOUND", "file not found", 404)
			return
		}
		writeError(w, "INTERNAL_ERROR", "failed to generate signed URL", 500)
		return
	}

	writeJSON(w, 200, map[string]string{"signed_url": url})
}

func (h *StorageHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	bucketName := r.PathValue("bucket")
	prefix := r.URL.Query().Get("prefix")

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil {
			offset = parsed
		}
	}

	files, err := h.svc.ListFiles(r.Context(), bucketName, prefix, limit, offset)
	if err != nil {
		if errors.Is(err, store.ErrBucketNotFound) {
			writeError(w, "NOT_FOUND", "bucket not found", 404)
			return
		}
		writeError(w, "INTERNAL_ERROR", "failed to list files", 500)
		return
	}
	if files == nil {
		files = []store.File{}
	}
	writeJSON(w, 200, files)
}

func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}
```

Add missing import in handler.go:
```go
import "encoding/json"
```

- [ ] **Step 4: Update main.go**

```go
// services/storage/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/garancehq/garance/services/storage/internal/config"
	"github.com/garancehq/garance/services/storage/internal/handler"
	s3client "github.com/garancehq/garance/services/storage/internal/s3"
	"github.com/garancehq/garance/services/storage/internal/service"
	"github.com/garancehq/garance/services/storage/internal/store"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()

	db, err := store.NewDB(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.RunMigrationsFromDir(ctx, "migrations"); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	s3, err := s3client.NewClient(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket, cfg.S3Region, cfg.S3UseSSL)
	if err != nil {
		log.Fatalf("failed to create S3 client: %v", err)
	}
	if err := s3.EnsureBucket(ctx); err != nil {
		log.Fatalf("failed to ensure S3 bucket: %v", err)
	}

	storageSvc := service.NewStorageService(db, s3)
	storageHandler := handler.NewStorageHandler(storageSvc, cfg.BaseURL, cfg.JWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	storageHandler.RegisterRoutes(mux)

	server := &http.Server{Addr: cfg.ListenAddr, Handler: mux}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		server.Shutdown(ctx)
	}()

	log.Printf("garance storage service listening on %s", cfg.ListenAddr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
```

- [ ] **Step 5: Write Dockerfile**

```dockerfile
# services/storage/Dockerfile
FROM golang:1-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /garance-storage .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /garance-storage /usr/local/bin/garance-storage
COPY migrations/ /app/migrations/
WORKDIR /app
ENV LISTEN_ADDR=0.0.0.0:4002
EXPOSE 4002
CMD ["garance-storage"]
```

- [ ] **Step 6: Verify build**

Run: `cd /Users/jh3ady/Development/Projects/garance/services/storage && go build ./...`
Run: `docker build -t garance-storage:dev .`

- [ ] **Step 7: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add services/
git commit -m ":sparkles: feat(storage): add HTTP handlers, auth middleware, and Dockerfile"
```

---

## Summary

| Task | Description | Tests |
|---|---|---|
| 1 | Go module, S3 client (MinIO), config | 4 |
| 2 | Database schema, bucket + file metadata store | 6 |
| 3 | Storage service (upload, download, signed URLs, constraints) | 5 |
| 4 | HTTP handlers, auth middleware, main.go, Dockerfile | 0 (build only) |
| **Total** | | **15** |

### Not in this plan (deferred)

- Image transformations (resize, webp, avif via libvips) — v0.2
- gRPC interface (added when Gateway is built — Plan 4)
- Bucket permissions from garance.schema.json
- Public URL serving (requires Gateway routing)
- Streaming large uploads with progress
