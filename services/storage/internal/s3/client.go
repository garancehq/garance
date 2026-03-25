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
