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
