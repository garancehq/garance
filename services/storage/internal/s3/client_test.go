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
