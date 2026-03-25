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
	ErrFileTooLarge       = errors.New("file exceeds maximum allowed size")
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
