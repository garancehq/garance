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
