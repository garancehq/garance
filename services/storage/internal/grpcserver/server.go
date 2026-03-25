package grpcserver

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	storagev1 "github.com/garancehq/garance/proto/gen/go/storage/v1"
	"github.com/garancehq/garance/services/storage/internal/service"
	"github.com/garancehq/garance/services/storage/internal/store"
)

type StorageGRPCServer struct {
	storagev1.UnimplementedStorageServiceServer
	svc *service.StorageService
}

func NewStorageGRPCServer(svc *service.StorageService) *StorageGRPCServer {
	return &StorageGRPCServer{svc: svc}
}

func (s *StorageGRPCServer) Register(srv *grpc.Server) {
	storagev1.RegisterStorageServiceServer(srv, s)
}

func (s *StorageGRPCServer) CreateBucket(ctx context.Context, req *storagev1.CreateBucketRequest) (*storagev1.BucketResponse, error) {
	var maxSize *int64
	if req.MaxFileSize > 0 {
		maxSize = &req.MaxFileSize
	}
	bucket, err := s.svc.CreateBucket(ctx, req.Name, req.IsPublic, maxSize, req.AllowedMimeTypes)
	if err != nil {
		if errors.Is(err, store.ErrBucketAlreadyExists) {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to create bucket")
	}
	return toBucketResponse(bucket), nil
}

func (s *StorageGRPCServer) ListBuckets(ctx context.Context, _ *storagev1.ListBucketsRequest) (*storagev1.ListBucketsResponse, error) {
	buckets, err := s.svc.ListBuckets(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list buckets")
	}
	resp := &storagev1.ListBucketsResponse{}
	for _, b := range buckets {
		resp.Buckets = append(resp.Buckets, toBucketResponse(&b))
	}
	return resp, nil
}

func (s *StorageGRPCServer) DeleteBucket(ctx context.Context, req *storagev1.DeleteBucketRequest) (*storagev1.DeleteBucketResponse, error) {
	if err := s.svc.DeleteBucket(ctx, req.Name); err != nil {
		if errors.Is(err, store.ErrBucketNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to delete bucket")
	}
	return &storagev1.DeleteBucketResponse{}, nil
}

func (s *StorageGRPCServer) Upload(ctx context.Context, req *storagev1.UploadRequest) (*storagev1.FileResponse, error) {
	var ownerID *uuid.UUID
	if req.OwnerId != "" {
		parsed, err := uuid.Parse(req.OwnerId)
		if err == nil {
			ownerID = &parsed
		}
	}

	reader := bytes.NewReader(req.Content)
	file, err := s.svc.Upload(ctx, req.Bucket, req.FileName, reader, int64(len(req.Content)), req.MimeType, ownerID)
	if err != nil {
		return nil, mapStorageError(err)
	}
	return toFileResponse(file, req.Bucket), nil
}

func (s *StorageGRPCServer) Download(ctx context.Context, req *storagev1.DownloadRequest) (*storagev1.DownloadResponse, error) {
	reader, fileMeta, err := s.svc.Download(ctx, req.Bucket, req.FileName)
	if err != nil {
		return nil, mapStorageError(err)
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to read file")
	}

	return &storagev1.DownloadResponse{
		Content:  content,
		MimeType: fileMeta.MimeType,
		Size:     fileMeta.Size,
	}, nil
}

func (s *StorageGRPCServer) DeleteFile(ctx context.Context, req *storagev1.DeleteFileRequest) (*storagev1.DeleteFileResponse, error) {
	if err := s.svc.Delete(ctx, req.Bucket, req.FileName); err != nil {
		return nil, mapStorageError(err)
	}
	return &storagev1.DeleteFileResponse{}, nil
}

func (s *StorageGRPCServer) ListFiles(ctx context.Context, req *storagev1.ListFilesRequest) (*storagev1.ListFilesResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}
	files, err := s.svc.ListFiles(ctx, req.Bucket, req.Prefix, limit, int(req.Offset))
	if err != nil {
		return nil, mapStorageError(err)
	}
	resp := &storagev1.ListFilesResponse{}
	for _, f := range files {
		resp.Files = append(resp.Files, toFileResponse(&f, req.Bucket))
	}
	return resp, nil
}

func (s *StorageGRPCServer) CreateSignedURL(ctx context.Context, req *storagev1.CreateSignedURLRequest) (*storagev1.SignedURLResponse, error) {
	expiry := time.Duration(req.ExpiresIn) * time.Second
	if expiry <= 0 {
		expiry = time.Hour
	}
	url, err := s.svc.SignedURL(ctx, req.Bucket, req.FileName, expiry)
	if err != nil {
		return nil, mapStorageError(err)
	}
	return &storagev1.SignedURLResponse{SignedUrl: url}, nil
}

func toBucketResponse(b *store.Bucket) *storagev1.BucketResponse {
	var maxSize int64
	if b.MaxFileSize != nil {
		maxSize = *b.MaxFileSize
	}
	return &storagev1.BucketResponse{
		Id:               b.ID.String(),
		Name:             b.Name,
		IsPublic:         b.IsPublic,
		MaxFileSize:      maxSize,
		AllowedMimeTypes: b.AllowedMimeTypes,
		CreatedAt:        b.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func toFileResponse(f *store.File, bucket string) *storagev1.FileResponse {
	var ownerID string
	if f.OwnerID != nil {
		ownerID = f.OwnerID.String()
	}
	return &storagev1.FileResponse{
		Id:        f.ID.String(),
		Bucket:    bucket,
		Name:      f.Name,
		Size:      f.Size,
		MimeType:  f.MimeType,
		OwnerId:   ownerID,
		CreatedAt: f.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func mapStorageError(err error) error {
	switch {
	case errors.Is(err, store.ErrBucketNotFound), errors.Is(err, store.ErrFileNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, store.ErrBucketAlreadyExists), errors.Is(err, store.ErrFileAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, service.ErrFileTooLarge):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, service.ErrMimeTypeNotAllowed):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
