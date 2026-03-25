# Garance API Gateway — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Garance API Gateway in Go — the single entry point for all client requests. Translates HTTP/REST to gRPC calls to backend services (Engine, Auth, Storage). Handles CORS, JWT verification, and request routing. Also add gRPC servers to the existing Engine (Rust/tonic), Auth (Go), and Storage (Go) services.

**Architecture:** The Gateway is a Go HTTP server that receives all client requests and proxies them via gRPC to the appropriate backend service. Proto definitions are shared in `proto/` at the repo root. Each service implements its own gRPC server alongside its existing HTTP server (both listen on different ports).

**Tech Stack:** Go 1.25+ (Gateway), tonic 0.12 + prost 0.13 (Engine gRPC), google.golang.org/grpc (Auth/Storage gRPC), protoc + protoc-gen-go + protoc-gen-go-grpc (code generation)

**Spec:** `docs/superpowers/specs/2026-03-25-garance-baas-design.md` (sections 3, 13)

---

## Task 1: Proto Definitions

**Files:**
- Create: `proto/engine/v1/engine.proto`
- Create: `proto/auth/v1/auth.proto`
- Create: `proto/storage/v1/storage.proto`
- Create: `proto/buf.yaml`
- Create: `proto/buf.gen.yaml`

- [ ] **Step 1: Write engine proto**

```protobuf
// proto/engine/v1/engine.proto
syntax = "proto3";

package engine.v1;

option go_package = "github.com/garancehq/garance/proto/engine/v1;enginev1";

service EngineService {
  rpc ListRows(ListRowsRequest) returns (ListRowsResponse);
  rpc GetRow(GetRowRequest) returns (GetRowResponse);
  rpc InsertRow(InsertRowRequest) returns (InsertRowResponse);
  rpc UpdateRow(UpdateRowRequest) returns (UpdateRowResponse);
  rpc DeleteRow(DeleteRowRequest) returns (DeleteRowResponse);
}

message ListRowsRequest {
  string table = 1;
  map<string, string> filters = 2; // key=column, value="operator.value"
  string select = 3;               // comma-separated columns
  string order = 4;                // e.g. "name.asc,age.desc"
  int64 limit = 5;
  int64 offset = 6;
  // Auth context (injected by Gateway from JWT)
  string user_id = 10;
  string project_id = 11;
  string role = 12;
}

message ListRowsResponse {
  // JSON-encoded array of rows
  bytes rows_json = 1;
  int64 count = 2;
}

message GetRowRequest {
  string table = 1;
  string id = 2;
  string user_id = 10;
  string project_id = 11;
  string role = 12;
}

message GetRowResponse {
  bytes row_json = 1;
  bool found = 2;
}

message InsertRowRequest {
  string table = 1;
  bytes body_json = 2; // JSON object of column values
  string user_id = 10;
  string project_id = 11;
  string role = 12;
}

message InsertRowResponse {
  bytes row_json = 1;
}

message UpdateRowRequest {
  string table = 1;
  string id = 2;
  bytes body_json = 3;
  string user_id = 10;
  string project_id = 11;
  string role = 12;
}

message UpdateRowResponse {
  bytes row_json = 1;
  bool found = 2;
}

message DeleteRowRequest {
  string table = 1;
  string id = 2;
  string user_id = 10;
  string project_id = 11;
  string role = 12;
}

message DeleteRowResponse {
  bool found = 1;
}
```

- [ ] **Step 2: Write auth proto**

```protobuf
// proto/auth/v1/auth.proto
syntax = "proto3";

package auth.v1;

option go_package = "github.com/garancehq/garance/proto/auth/v1;authv1";

service AuthService {
  rpc SignUp(SignUpRequest) returns (AuthResponse);
  rpc SignIn(SignInRequest) returns (AuthResponse);
  rpc RefreshToken(RefreshTokenRequest) returns (AuthResponse);
  rpc SignOut(SignOutRequest) returns (SignOutResponse);
  rpc GetUser(GetUserRequest) returns (UserResponse);
  rpc DeleteUser(DeleteUserRequest) returns (DeleteUserResponse);
}

message SignUpRequest {
  string email = 1;
  string password = 2;
  string user_agent = 3;
  string ip_address = 4;
}

message SignInRequest {
  string email = 1;
  string password = 2;
  string user_agent = 3;
  string ip_address = 4;
}

message RefreshTokenRequest {
  string refresh_token = 1;
  string user_agent = 2;
  string ip_address = 3;
}

message SignOutRequest {
  string refresh_token = 1;
}

message SignOutResponse {}

message GetUserRequest {
  string user_id = 1;
}

message DeleteUserRequest {
  string user_id = 1;
}

message DeleteUserResponse {}

message AuthResponse {
  UserResponse user = 1;
  TokenPair token_pair = 2;
}

message TokenPair {
  string access_token = 1;
  string refresh_token = 2;
  int64 expires_in = 3;
  string token_type = 4;
}

message UserResponse {
  string id = 1;
  string email = 2;
  bool email_verified = 3;
  string role = 4;
  string created_at = 5;
  string updated_at = 6;
}
```

- [ ] **Step 3: Write storage proto**

```protobuf
// proto/storage/v1/storage.proto
syntax = "proto3";

package storage.v1;

option go_package = "github.com/garancehq/garance/proto/storage/v1;storagev1";

service StorageService {
  rpc CreateBucket(CreateBucketRequest) returns (BucketResponse);
  rpc ListBuckets(ListBucketsRequest) returns (ListBucketsResponse);
  rpc DeleteBucket(DeleteBucketRequest) returns (DeleteBucketResponse);
  rpc Upload(UploadRequest) returns (FileResponse);
  rpc Download(DownloadRequest) returns (DownloadResponse);
  rpc DeleteFile(DeleteFileRequest) returns (DeleteFileResponse);
  rpc ListFiles(ListFilesRequest) returns (ListFilesResponse);
  rpc CreateSignedURL(CreateSignedURLRequest) returns (SignedURLResponse);
}

message CreateBucketRequest {
  string name = 1;
  bool is_public = 2;
  int64 max_file_size = 3; // 0 = no limit
  repeated string allowed_mime_types = 4;
}

message BucketResponse {
  string id = 1;
  string name = 2;
  bool is_public = 3;
  int64 max_file_size = 4;
  repeated string allowed_mime_types = 5;
  string created_at = 6;
}

message ListBucketsRequest {}

message ListBucketsResponse {
  repeated BucketResponse buckets = 1;
}

message DeleteBucketRequest {
  string name = 1;
}

message DeleteBucketResponse {}

message UploadRequest {
  string bucket = 1;
  string file_name = 2;
  bytes content = 3;
  string mime_type = 4;
  string owner_id = 5; // from JWT
}

message FileResponse {
  string id = 1;
  string bucket = 2;
  string name = 3;
  int64 size = 4;
  string mime_type = 5;
  string owner_id = 6;
  string created_at = 7;
}

message DownloadRequest {
  string bucket = 1;
  string file_name = 2;
}

message DownloadResponse {
  bytes content = 1;
  string mime_type = 2;
  int64 size = 3;
}

message DeleteFileRequest {
  string bucket = 1;
  string file_name = 2;
}

message DeleteFileResponse {}

message ListFilesRequest {
  string bucket = 1;
  string prefix = 2;
  int32 limit = 3;
  int32 offset = 4;
}

message ListFilesResponse {
  repeated FileResponse files = 1;
}

message CreateSignedURLRequest {
  string bucket = 1;
  string file_name = 2;
  int32 expires_in = 3; // seconds
}

message SignedURLResponse {
  string signed_url = 1;
}
```

- [ ] **Step 4: Create buf config for code generation**

```yaml
# proto/buf.yaml
version: v2
lint:
  use:
    - STANDARD
breaking:
  use:
    - FILE
```

```yaml
# proto/buf.gen.yaml
version: v2
managed:
  enabled: true
plugins:
  # Go code generation
  - remote: buf.build/protocolbuffers/go
    out: gen/go
    opt: paths=source_relative
  - remote: buf.build/grpc/go
    out: gen/go
    opt: paths=source_relative
```

- [ ] **Step 5: Generate Go code**

```bash
cd /Users/jh3ady/Development/Projects/garance/proto
# Install buf if not present
go install github.com/bufbuild/buf/cmd/buf@latest
# Generate
buf generate
```

This creates:
- `proto/gen/go/engine/v1/engine.pb.go`
- `proto/gen/go/engine/v1/engine_grpc.pb.go`
- `proto/gen/go/auth/v1/auth.pb.go`
- `proto/gen/go/auth/v1/auth_grpc.pb.go`
- `proto/gen/go/storage/v1/storage.pb.go`
- `proto/gen/go/storage/v1/storage_grpc.pb.go`

- [ ] **Step 6: Generate Rust code for Engine**

For the Rust Engine, we use tonic-build. Add to `engine/crates/garance-engine/Cargo.toml`:

```toml
[dependencies]
# ... existing deps ...
tonic = "0.12"
prost = "0.13"

[build-dependencies]
tonic-build = "0.12"
```

Create `engine/crates/garance-engine/build.rs`:

```rust
fn main() -> Result<(), Box<dyn std::error::Error>> {
    tonic_build::configure()
        .build_server(true)
        .build_client(false)
        .compile_protos(
            &["../../../proto/engine/v1/engine.proto"],
            &["../../../proto"],
        )?;
    Ok(())
}
```

Run: `cd /Users/jh3ady/Development/Projects/garance/engine && cargo build -p garance-engine`

- [ ] **Step 7: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add proto/ engine/
git commit -m ":sparkles: feat(proto): add gRPC proto definitions for engine, auth, and storage"
```

---

## Task 2: Auth gRPC Server

**Files:**
- Create: `services/auth/internal/grpcserver/server.go`
- Modify: `services/auth/main.go` — add gRPC listener alongside HTTP
- Modify: `services/auth/go.mod` — add grpc deps

- [ ] **Step 1: Add gRPC dependencies**

```bash
cd /Users/jh3ady/Development/Projects/garance/services/auth
go get google.golang.org/grpc
go get google.golang.org/protobuf
```

The auth service needs to import the generated proto code. Create a local replace or use the generated code from `proto/gen/go/`. Add to `go.mod`:

```
require github.com/garancehq/garance/proto v0.0.0
replace github.com/garancehq/garance/proto => ../../proto
```

And create `proto/go.mod`:

```
module github.com/garancehq/garance/proto

go 1.25
```

Run `go mod tidy` in proto/ and auth/ to resolve dependencies.

Also update `services/go.work` to include `../../proto`:

```go
use (
    ./auth
    ./storage
    ../../proto  // if go.work is in services/
)
```

Or more practically, move go.work to repo root. This will be adjusted during implementation.

- [ ] **Step 2: Write gRPC server for auth**

```go
// services/auth/internal/grpcserver/server.go
package grpcserver

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authv1 "github.com/garancehq/garance/proto/gen/go/auth/v1"
	"github.com/garancehq/garance/services/auth/internal/service"
	"github.com/garancehq/garance/services/auth/internal/store"
)

type AuthGRPCServer struct {
	authv1.UnimplementedAuthServiceServer
	auth *service.AuthService
}

func NewAuthGRPCServer(auth *service.AuthService) *AuthGRPCServer {
	return &AuthGRPCServer{auth: auth}
}

func (s *AuthGRPCServer) Register(srv *grpc.Server) {
	authv1.RegisterAuthServiceServer(srv, s)
}

func (s *AuthGRPCServer) SignUp(ctx context.Context, req *authv1.SignUpRequest) (*authv1.AuthResponse, error) {
	result, err := s.auth.SignUp(ctx, req.Email, req.Password, req.UserAgent, req.IpAddress)
	if err != nil {
		return nil, mapAuthError(err)
	}
	return toAuthResponse(result), nil
}

func (s *AuthGRPCServer) SignIn(ctx context.Context, req *authv1.SignInRequest) (*authv1.AuthResponse, error) {
	result, err := s.auth.SignIn(ctx, req.Email, req.Password, req.UserAgent, req.IpAddress)
	if err != nil {
		return nil, mapAuthError(err)
	}
	return toAuthResponse(result), nil
}

func (s *AuthGRPCServer) RefreshToken(ctx context.Context, req *authv1.RefreshTokenRequest) (*authv1.AuthResponse, error) {
	result, err := s.auth.RefreshToken(ctx, req.RefreshToken, req.UserAgent, req.IpAddress)
	if err != nil {
		return nil, mapAuthError(err)
	}
	return toAuthResponse(result), nil
}

func (s *AuthGRPCServer) SignOut(ctx context.Context, req *authv1.SignOutRequest) (*authv1.SignOutResponse, error) {
	_ = s.auth.SignOut(ctx, req.RefreshToken)
	return &authv1.SignOutResponse{}, nil
}

func (s *AuthGRPCServer) GetUser(ctx context.Context, req *authv1.GetUserRequest) (*authv1.UserResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}
	user, err := s.auth.GetUser(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "internal error")
	}
	return toUserResponse(user), nil
}

func (s *AuthGRPCServer) DeleteUser(ctx context.Context, req *authv1.DeleteUserRequest) (*authv1.DeleteUserResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}
	if err := s.auth.DeleteUser(ctx, userID); err != nil {
		return nil, status.Error(codes.Internal, "failed to delete user")
	}
	return &authv1.DeleteUserResponse{}, nil
}

func toAuthResponse(result *service.AuthResult) *authv1.AuthResponse {
	return &authv1.AuthResponse{
		User: toUserResponse(result.User),
		TokenPair: &authv1.TokenPair{
			AccessToken:  result.TokenPair.AccessToken,
			RefreshToken: result.TokenPair.RefreshToken,
			ExpiresIn:    result.TokenPair.ExpiresIn,
			TokenType:    result.TokenPair.TokenType,
		},
	}
}

func toUserResponse(user *store.User) *authv1.UserResponse {
	return &authv1.UserResponse{
		Id:            user.ID.String(),
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		Role:          user.Role,
		CreatedAt:     user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     user.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func mapAuthError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, service.ErrUserBanned):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, store.ErrEmailAlreadyTaken):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, service.ErrPasswordRequired), errors.Is(err, service.ErrWeakPassword):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
```

- [ ] **Step 3: Update auth main.go to serve both HTTP and gRPC**

Add to main.go, after the HTTP server setup:

```go
// gRPC server
import (
	"net"
	"google.golang.org/grpc"
	"github.com/garancehq/garance/services/auth/internal/grpcserver"
)

// In main(), after HTTP setup:
grpcAddr := getEnv("GRPC_ADDR", "0.0.0.0:5001")
grpcSrv := grpc.NewServer()
authGRPC := grpcserver.NewAuthGRPCServer(authService)
authGRPC.Register(grpcSrv)

lis, err := net.Listen("tcp", grpcAddr)
if err != nil {
	log.Fatalf("failed to listen for gRPC: %v", err)
}

go func() {
	log.Printf("garance auth gRPC listening on %s", grpcAddr)
	if err := grpcSrv.Serve(lis); err != nil {
		log.Fatalf("gRPC server error: %v", err)
	}
}()

// In shutdown handler, add:
grpcSrv.GracefulStop()
```

- [ ] **Step 4: Verify**

Run: `cd /Users/jh3ady/Development/Projects/garance/services/auth && go build ./...`
Run: `go test ./... -count=1` — existing tests still pass

- [ ] **Step 5: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add services/auth/ proto/
git commit -m ":sparkles: feat(auth): add gRPC server alongside HTTP"
```

---

## Task 3: Storage gRPC Server

**Files:**
- Create: `services/storage/internal/grpcserver/server.go`
- Modify: `services/storage/main.go` — add gRPC listener
- Modify: `services/storage/go.mod` — add grpc deps + proto module

- [ ] **Step 1: Add gRPC dependencies**

```bash
cd /Users/jh3ady/Development/Projects/garance/services/storage
go get google.golang.org/grpc
go get google.golang.org/protobuf
```

Add the same proto module replace as auth:
```
require github.com/garancehq/garance/proto v0.0.0
replace github.com/garancehq/garance/proto => ../../proto
```

- [ ] **Step 2: Write gRPC server for storage**

```go
// services/storage/internal/grpcserver/server.go
package grpcserver

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
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
```

- [ ] **Step 3: Update storage main.go** — same pattern as auth (dual HTTP + gRPC)

gRPC on port 5002, HTTP on port 4002.

- [ ] **Step 4: Verify**

Run: `cd /Users/jh3ady/Development/Projects/garance/services/storage && go build ./...`
Run: `go test ./... -count=1` — existing tests still pass

- [ ] **Step 5: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add services/storage/ proto/
git commit -m ":sparkles: feat(storage): add gRPC server alongside HTTP"
```

---

## Task 4: Engine gRPC Server (Rust/tonic)

**Files:**
- Create: `engine/crates/garance-engine/src/grpc/mod.rs`
- Create: `engine/crates/garance-engine/src/grpc/server.rs`
- Create: `engine/crates/garance-engine/build.rs`
- Modify: `engine/crates/garance-engine/Cargo.toml` — add tonic, prost, tonic-build
- Modify: `engine/crates/garance-engine/src/lib.rs` — add `pub mod grpc;`
- Modify: `engine/crates/garance-engine/src/main.rs` — add gRPC listener

- [ ] **Step 1: Update Cargo.toml**

Add to `engine/Cargo.toml` workspace dependencies:
```toml
tonic = "0.12"
prost = "0.13"
```

Add to `engine/crates/garance-engine/Cargo.toml`:
```toml
[dependencies]
# ... existing ...
tonic.workspace = true
prost.workspace = true

[build-dependencies]
tonic-build = "0.12"
```

- [ ] **Step 2: Create build.rs for proto compilation**

```rust
// engine/crates/garance-engine/build.rs
fn main() -> Result<(), Box<dyn std::error::Error>> {
    tonic_build::configure()
        .build_server(true)
        .build_client(false)
        .compile_protos(
            &["../../../proto/engine/v1/engine.proto"],
            &["../../../proto"],
        )?;
    Ok(())
}
```

- [ ] **Step 3: Write gRPC server**

```rust
// engine/crates/garance-engine/src/grpc/mod.rs
pub mod server;
```

```rust
// engine/crates/garance-engine/src/grpc/server.rs
use std::sync::Arc;
use tokio::sync::RwLock;
use tonic::{Request, Response, Status};
use serde_json::{Map, Value};

use crate::api::routes::row_to_json; // reuse existing serialization
use crate::query::filter::parse_query_params;
use crate::query::builder::*;
use crate::schema::types::Schema;
use garance_pooler::GarancePool;

// Include generated proto code
pub mod engine_proto {
    tonic::include_proto!("engine.v1");
}

use engine_proto::engine_service_server::{EngineService, EngineServiceServer};
use engine_proto::*;

pub struct EngineGrpcService {
    pool: Arc<GarancePool>,
    schema: Arc<RwLock<Schema>>,
}

impl EngineGrpcService {
    pub fn new(pool: Arc<GarancePool>, schema: Arc<RwLock<Schema>>) -> Self {
        Self { pool, schema }
    }

    pub fn into_service(self) -> EngineServiceServer<Self> {
        EngineServiceServer::new(self)
    }
}

#[tonic::async_trait]
impl EngineService for EngineGrpcService {
    async fn list_rows(&self, request: Request<ListRowsRequest>) -> Result<Response<ListRowsResponse>, Status> {
        let req = request.into_inner();
        let schema = self.schema.read().await;
        let table = schema.tables.get(&req.table)
            .ok_or_else(|| Status::not_found(format!("table '{}' not found", req.table)))?;

        let mut params: Vec<(String, String)> = req.filters.into_iter().collect();
        if !req.select.is_empty() {
            params.push(("select".into(), req.select));
        }
        if !req.order.is_empty() {
            params.push(("order".into(), req.order));
        }
        if req.limit > 0 {
            params.push(("limit".into(), req.limit.to_string()));
        }
        if req.offset > 0 {
            params.push(("offset".into(), req.offset.to_string()));
        }

        let qp = parse_query_params(&params).map_err(|e| Status::invalid_argument(e.to_string()))?;
        let sql_query = build_select(table, &qp).map_err(|e| Status::invalid_argument(e.to_string()))?;

        let client = self.pool.get().await.map_err(|e| Status::internal(e.to_string()))?;
        let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> =
            sql_query.params.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();

        let rows = client.query(&sql_query.sql, &params_refs).await
            .map_err(|e| Status::internal(e.to_string()))?;

        let results: Vec<Value> = rows.iter().map(row_to_json).collect();
        let json_bytes = serde_json::to_vec(&results).map_err(|e| Status::internal(e.to_string()))?;

        Ok(Response::new(ListRowsResponse {
            rows_json: json_bytes,
            count: results.len() as i64,
        }))
    }

    async fn get_row(&self, request: Request<GetRowRequest>) -> Result<Response<GetRowResponse>, Status> {
        let req = request.into_inner();
        let schema = self.schema.read().await;
        let table = schema.tables.get(&req.table)
            .ok_or_else(|| Status::not_found(format!("table '{}' not found", req.table)))?;

        let sql_query = build_select_by_id(table, &req.id).map_err(|e| Status::invalid_argument(e.to_string()))?;
        let client = self.pool.get().await.map_err(|e| Status::internal(e.to_string()))?;
        let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> =
            sql_query.params.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();

        let row = client.query_opt(&sql_query.sql, &params_refs).await
            .map_err(|e| Status::internal(e.to_string()))?;

        match row {
            Some(row) => {
                let json_bytes = serde_json::to_vec(&row_to_json(&row)).map_err(|e| Status::internal(e.to_string()))?;
                Ok(Response::new(GetRowResponse { row_json: json_bytes, found: true }))
            }
            None => Ok(Response::new(GetRowResponse { row_json: vec![], found: false })),
        }
    }

    async fn insert_row(&self, request: Request<InsertRowRequest>) -> Result<Response<InsertRowResponse>, Status> {
        let req = request.into_inner();
        let schema = self.schema.read().await;
        let table = schema.tables.get(&req.table)
            .ok_or_else(|| Status::not_found(format!("table '{}' not found", req.table)))?;

        let body: Map<String, Value> = serde_json::from_slice(&req.body_json)
            .map_err(|e| Status::invalid_argument(format!("invalid JSON body: {}", e)))?;

        let columns: Vec<String> = body.keys().cloned().collect();
        let values: Vec<String> = body.values().map(|v| match v {
            Value::String(s) => s.clone(),
            other => other.to_string(),
        }).collect();

        let sql_query = build_insert(table, &columns).map_err(|e| Status::invalid_argument(e.to_string()))?;
        let client = self.pool.get().await.map_err(|e| Status::internal(e.to_string()))?;
        let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> =
            values.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();

        let row = client.query_one(&sql_query.sql, &params_refs).await
            .map_err(|e| Status::internal(e.to_string()))?;

        let json_bytes = serde_json::to_vec(&row_to_json(&row)).map_err(|e| Status::internal(e.to_string()))?;
        Ok(Response::new(InsertRowResponse { row_json: json_bytes }))
    }

    async fn update_row(&self, request: Request<UpdateRowRequest>) -> Result<Response<UpdateRowResponse>, Status> {
        let req = request.into_inner();
        let schema = self.schema.read().await;
        let table = schema.tables.get(&req.table)
            .ok_or_else(|| Status::not_found(format!("table '{}' not found", req.table)))?;

        let body: Map<String, Value> = serde_json::from_slice(&req.body_json)
            .map_err(|e| Status::invalid_argument(format!("invalid JSON body: {}", e)))?;

        let columns: Vec<String> = body.keys().cloned().collect();
        let values: Vec<String> = body.values().map(|v| match v {
            Value::String(s) => s.clone(),
            other => other.to_string(),
        }).collect();

        let sql_query = build_update(table, &req.id, &columns).map_err(|e| Status::invalid_argument(e.to_string()))?;
        let client = self.pool.get().await.map_err(|e| Status::internal(e.to_string()))?;

        let mut all_params: Vec<String> = values;
        all_params.push(req.id);
        let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> =
            all_params.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();

        let row = client.query_opt(&sql_query.sql, &params_refs).await
            .map_err(|e| Status::internal(e.to_string()))?;

        match row {
            Some(row) => {
                let json_bytes = serde_json::to_vec(&row_to_json(&row)).map_err(|e| Status::internal(e.to_string()))?;
                Ok(Response::new(UpdateRowResponse { row_json: json_bytes, found: true }))
            }
            None => Ok(Response::new(UpdateRowResponse { row_json: vec![], found: false })),
        }
    }

    async fn delete_row(&self, request: Request<DeleteRowRequest>) -> Result<Response<DeleteRowResponse>, Status> {
        let req = request.into_inner();
        let schema = self.schema.read().await;
        let table = schema.tables.get(&req.table)
            .ok_or_else(|| Status::not_found(format!("table '{}' not found", req.table)))?;

        let sql_query = build_delete(table, &req.id).map_err(|e| Status::invalid_argument(e.to_string()))?;
        let client = self.pool.get().await.map_err(|e| Status::internal(e.to_string()))?;
        let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> =
            sql_query.params.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();

        let affected = client.execute(&sql_query.sql, &params_refs).await
            .map_err(|e| Status::internal(e.to_string()))?;

        Ok(Response::new(DeleteRowResponse { found: affected > 0 }))
    }
}
```

Note: `row_to_json` needs to be made `pub` in `api/routes.rs` for the grpc module to reuse it.

- [ ] **Step 4: Update lib.rs and main.rs**

In `lib.rs` add `pub mod grpc;`.

In `main.rs`, add gRPC server on port 5000 alongside the HTTP server on port 4000.

- [ ] **Step 5: Verify**

Run: `cd /Users/jh3ady/Development/Projects/garance/engine && cargo build -p garance-engine`
Run: `cargo test -p garance-engine` — existing tests still pass

Note: `protoc` must be installed on the system for tonic-build. Install via `brew install protobuf`.

- [ ] **Step 6: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add engine/
git commit -m ":sparkles: feat(engine): add gRPC server with tonic"
```

---

## Task 5: Gateway Service

**Files:**
- Create: `services/gateway/go.mod`
- Create: `services/gateway/main.go`
- Create: `services/gateway/internal/config/config.go`
- Create: `services/gateway/internal/proxy/engine.go`
- Create: `services/gateway/internal/proxy/auth.go`
- Create: `services/gateway/internal/proxy/storage.go`
- Create: `services/gateway/internal/middleware/cors.go`
- Create: `services/gateway/internal/middleware/jwt.go`
- Create: `services/gateway/internal/middleware/logging.go`
- Create: `services/gateway/internal/handler/response.go`
- Modify: `services/go.work` — add `./gateway`

- [ ] **Step 1: Initialize Go module**

```bash
mkdir -p /Users/jh3ady/Development/Projects/garance/services/gateway/internal/{config,proxy,middleware,handler}
cd /Users/jh3ady/Development/Projects/garance/services/gateway
go mod init github.com/garancehq/garance/services/gateway
```

Update `services/go.work`:
```go
use (
    ./auth
    ./storage
    ./gateway
)
```

- [ ] **Step 2: Write config**

```go
// services/gateway/internal/config/config.go
package config

import "os"

type Config struct {
	ListenAddr     string
	EngineGRPCAddr string
	AuthGRPCAddr   string
	StorageGRPCAddr string
	JWTSecret      string
	AllowedOrigins string
}

func Load() *Config {
	return &Config{
		ListenAddr:      getEnv("LISTEN_ADDR", "0.0.0.0:8080"),
		EngineGRPCAddr:  getEnv("ENGINE_GRPC_ADDR", "localhost:5000"),
		AuthGRPCAddr:    getEnv("AUTH_GRPC_ADDR", "localhost:5001"),
		StorageGRPCAddr: getEnv("STORAGE_GRPC_ADDR", "localhost:5002"),
		JWTSecret:       getEnv("JWT_SECRET", "dev-secret-change-me"),
		AllowedOrigins:  getEnv("ALLOWED_ORIGINS", "*"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
```

- [ ] **Step 3: Write CORS middleware**

```go
// services/gateway/internal/middleware/cors.go
package middleware

import (
	"net/http"
	"strings"
)

func CORS(allowedOrigins string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			if allowedOrigins == "*" || containsOrigin(allowedOrigins, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func containsOrigin(allowed, origin string) bool {
	for _, o := range strings.Split(allowed, ",") {
		if strings.TrimSpace(o) == origin {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Write JWT middleware**

```go
// services/gateway/internal/middleware/jwt.go
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	UserIDKey    contextKey = "user_id"
	ProjectIDKey contextKey = "project_id"
	RoleKey      contextKey = "role"
)

type JWTClaims struct {
	jwt.RegisteredClaims
	UserID    string `json:"user_id"`
	ProjectID string `json:"project_id,omitempty"`
	Role      string `json:"role"`
}

// ExtractJWT extracts JWT claims if present, but does NOT require auth.
// Route-level enforcement is done per-proxy.
func ExtractJWT(secret string) func(http.Handler) http.Handler {
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
			if err != nil || !token.Valid {
				next.ServeHTTP(w, r)
				return
			}

			claims := token.Claims.(*JWTClaims)
			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, ProjectIDKey, claims.ProjectID)
			ctx = context.WithValue(ctx, RoleKey, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

- [ ] **Step 5: Write logging middleware**

```go
// services/gateway/internal/middleware/logging.go
package middleware

import (
	"log"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start))
	})
}
```

- [ ] **Step 6: Write response helpers**

```go
// services/gateway/internal/handler/response.go
package handler

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func WriteError(w http.ResponseWriter, code string, message string, status int) {
	WriteJSON(w, status, ErrorResponse{
		Error: ErrorBody{Code: code, Message: message, Status: status},
	})
}

func WriteRawJSON(w http.ResponseWriter, status int, jsonBytes []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(jsonBytes)
}
```

- [ ] **Step 7: Write engine proxy**

```go
// services/gateway/internal/proxy/engine.go
package proxy

import (
	"io"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	enginev1 "github.com/garancehq/garance/proto/gen/go/engine/v1"
	"github.com/garancehq/garance/services/gateway/internal/handler"
	"github.com/garancehq/garance/services/gateway/internal/middleware"
)

type EngineProxy struct {
	client enginev1.EngineServiceClient
}

func NewEngineProxy(addr string) (*EngineProxy, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &EngineProxy{client: enginev1.NewEngineServiceClient(conn)}, nil
}

func (p *EngineProxy) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/{table}", p.ListRows)
	mux.HandleFunc("POST /api/v1/{table}", p.InsertRow)
	mux.HandleFunc("GET /api/v1/{table}/{id}", p.GetRow)
	mux.HandleFunc("PATCH /api/v1/{table}/{id}", p.UpdateRow)
	mux.HandleFunc("DELETE /api/v1/{table}/{id}", p.DeleteRow)
}

func (p *EngineProxy) ListRows(w http.ResponseWriter, r *http.Request) {
	table := r.PathValue("table")
	query := r.URL.Query()

	filters := make(map[string]string)
	for key, vals := range query {
		if key != "select" && key != "order" && key != "limit" && key != "offset" {
			filters[key] = vals[0]
		}
	}

	req := &enginev1.ListRowsRequest{
		Table:   table,
		Filters: filters,
		Select:  query.Get("select"),
		Order:   query.Get("order"),
	}
	if l := query.Get("limit"); l != "" {
		var limit int64
		fmt.Sscanf(l, "%d", &limit)
		req.Limit = limit
	}
	if o := query.Get("offset"); o != "" {
		var offset int64
		fmt.Sscanf(o, "%d", &offset)
		req.Offset = offset
	}

	// Inject auth context from JWT
	if uid, ok := r.Context().Value(middleware.UserIDKey).(string); ok {
		req.UserId = uid
	}
	if pid, ok := r.Context().Value(middleware.ProjectIDKey).(string); ok {
		req.ProjectId = pid
	}
	if role, ok := r.Context().Value(middleware.RoleKey).(string); ok {
		req.Role = role
	}

	resp, err := p.client.ListRows(r.Context(), req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	handler.WriteRawJSON(w, 200, resp.RowsJson)
}

func (p *EngineProxy) GetRow(w http.ResponseWriter, r *http.Request) {
	req := &enginev1.GetRowRequest{
		Table: r.PathValue("table"),
		Id:    r.PathValue("id"),
	}
	injectAuth(r, func(uid, pid, role string) {
		req.UserId = uid
		req.ProjectId = pid
		req.Role = role
	})

	resp, err := p.client.GetRow(r.Context(), req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	if !resp.Found {
		handler.WriteError(w, "NOT_FOUND", "row not found", 404)
		return
	}
	handler.WriteRawJSON(w, 200, resp.RowJson)
}

func (p *EngineProxy) InsertRow(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		handler.WriteError(w, "VALIDATION_ERROR", "failed to read body", 400)
		return
	}

	req := &enginev1.InsertRowRequest{
		Table:    r.PathValue("table"),
		BodyJson: body,
	}
	injectAuth(r, func(uid, pid, role string) {
		req.UserId = uid
		req.ProjectId = pid
		req.Role = role
	})

	resp, err := p.client.InsertRow(r.Context(), req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	handler.WriteRawJSON(w, 201, resp.RowJson)
}

func (p *EngineProxy) UpdateRow(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		handler.WriteError(w, "VALIDATION_ERROR", "failed to read body", 400)
		return
	}

	req := &enginev1.UpdateRowRequest{
		Table:    r.PathValue("table"),
		Id:       r.PathValue("id"),
		BodyJson: body,
	}
	injectAuth(r, func(uid, pid, role string) {
		req.UserId = uid
		req.ProjectId = pid
		req.Role = role
	})

	resp, err := p.client.UpdateRow(r.Context(), req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	if !resp.Found {
		handler.WriteError(w, "NOT_FOUND", "row not found", 404)
		return
	}
	handler.WriteRawJSON(w, 200, resp.RowJson)
}

func (p *EngineProxy) DeleteRow(w http.ResponseWriter, r *http.Request) {
	req := &enginev1.DeleteRowRequest{
		Table: r.PathValue("table"),
		Id:    r.PathValue("id"),
	}
	injectAuth(r, func(uid, pid, role string) {
		req.UserId = uid
		req.ProjectId = pid
		req.Role = role
	})

	resp, err := p.client.DeleteRow(r.Context(), req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	if !resp.Found {
		handler.WriteError(w, "NOT_FOUND", "row not found", 404)
		return
	}
	w.WriteHeader(204)
}

// Helper to inject auth context from middleware into gRPC request
func injectAuth(r *http.Request, setter func(uid, pid, role string)) {
	uid, _ := r.Context().Value(middleware.UserIDKey).(string)
	pid, _ := r.Context().Value(middleware.ProjectIDKey).(string)
	role, _ := r.Context().Value(middleware.RoleKey).(string)
	setter(uid, pid, role)
}

// Map gRPC errors to HTTP errors
func writeGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		handler.WriteError(w, "INTERNAL_ERROR", "internal error", 500)
		return
	}
	switch st.Code() {
	case codes.NotFound:
		handler.WriteError(w, "NOT_FOUND", st.Message(), 404)
	case codes.InvalidArgument:
		handler.WriteError(w, "VALIDATION_ERROR", st.Message(), 400)
	case codes.Unauthenticated:
		handler.WriteError(w, "UNAUTHORIZED", st.Message(), 401)
	case codes.PermissionDenied:
		handler.WriteError(w, "PERMISSION_DENIED", st.Message(), 403)
	case codes.AlreadyExists:
		handler.WriteError(w, "CONFLICT", st.Message(), 409)
	default:
		handler.WriteError(w, "INTERNAL_ERROR", "internal error", 500)
	}
}
```

Add missing import: `"fmt"` for Sscanf.

- [ ] **Step 8: Write auth proxy**

```go
// services/gateway/internal/proxy/auth.go
package proxy

import (
	"encoding/json"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	authv1 "github.com/garancehq/garance/proto/gen/go/auth/v1"
	"github.com/garancehq/garance/services/gateway/internal/handler"
	"github.com/garancehq/garance/services/gateway/internal/middleware"
)

type AuthProxy struct {
	client authv1.AuthServiceClient
}

func NewAuthProxy(addr string) (*AuthProxy, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &AuthProxy{client: authv1.NewAuthServiceClient(conn)}, nil
}

func (p *AuthProxy) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/v1/signup", p.SignUp)
	mux.HandleFunc("POST /auth/v1/signin", p.SignIn)
	mux.HandleFunc("POST /auth/v1/token/refresh", p.RefreshToken)
	mux.HandleFunc("POST /auth/v1/signout", p.SignOut)
	mux.HandleFunc("GET /auth/v1/user", p.GetUser)
	mux.HandleFunc("DELETE /auth/v1/user", p.DeleteUser)
}

func (p *AuthProxy) SignUp(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		handler.WriteError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}

	resp, err := p.client.SignUp(r.Context(), &authv1.SignUpRequest{
		Email:     body.Email,
		Password:  body.Password,
		UserAgent: r.UserAgent(),
		IpAddress: r.RemoteAddr,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	handler.WriteJSON(w, 201, resp)
}

func (p *AuthProxy) SignIn(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		handler.WriteError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}

	resp, err := p.client.SignIn(r.Context(), &authv1.SignInRequest{
		Email:     body.Email,
		Password:  body.Password,
		UserAgent: r.UserAgent(),
		IpAddress: r.RemoteAddr,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	handler.WriteJSON(w, 200, resp)
}

func (p *AuthProxy) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		handler.WriteError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}

	resp, err := p.client.RefreshToken(r.Context(), &authv1.RefreshTokenRequest{
		RefreshToken: body.RefreshToken,
		UserAgent:    r.UserAgent(),
		IpAddress:    r.RemoteAddr,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	handler.WriteJSON(w, 200, resp)
}

func (p *AuthProxy) SignOut(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		handler.WriteError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}

	_, err := p.client.SignOut(r.Context(), &authv1.SignOutRequest{
		RefreshToken: body.RefreshToken,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	w.WriteHeader(204)
}

func (p *AuthProxy) GetUser(w http.ResponseWriter, r *http.Request) {
	uid, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || uid == "" {
		handler.WriteError(w, "UNAUTHORIZED", "authentication required", 401)
		return
	}

	resp, err := p.client.GetUser(r.Context(), &authv1.GetUserRequest{UserId: uid})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	handler.WriteJSON(w, 200, resp)
}

func (p *AuthProxy) DeleteUser(w http.ResponseWriter, r *http.Request) {
	uid, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || uid == "" {
		handler.WriteError(w, "UNAUTHORIZED", "authentication required", 401)
		return
	}

	_, err := p.client.DeleteUser(r.Context(), &authv1.DeleteUserRequest{UserId: uid})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	w.WriteHeader(204)
}
```

- [ ] **Step 9: Write storage proxy**

```go
// services/gateway/internal/proxy/storage.go
package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	storagev1 "github.com/garancehq/garance/proto/gen/go/storage/v1"
	"github.com/garancehq/garance/services/gateway/internal/handler"
	"github.com/garancehq/garance/services/gateway/internal/middleware"
)

type StorageProxy struct {
	client storagev1.StorageServiceClient
}

func NewStorageProxy(addr string) (*StorageProxy, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &StorageProxy{client: storagev1.NewStorageServiceClient(conn)}, nil
}

func (p *StorageProxy) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /storage/v1/buckets", p.CreateBucket)
	mux.HandleFunc("GET /storage/v1/buckets", p.ListBuckets)
	mux.HandleFunc("DELETE /storage/v1/buckets/{bucket}", p.DeleteBucket)
	mux.HandleFunc("POST /storage/v1/{bucket}/upload", p.Upload)
	mux.HandleFunc("GET /storage/v1/{bucket}/{path...}", p.Download)
	mux.HandleFunc("DELETE /storage/v1/{bucket}/{path...}", p.DeleteFile)
	mux.HandleFunc("POST /storage/v1/{bucket}/signed-url", p.CreateSignedURL)
	mux.HandleFunc("GET /storage/v1/{bucket}", p.ListFiles)
}

func (p *StorageProxy) CreateBucket(w http.ResponseWriter, r *http.Request) {
	requireAuth(r, w, func() {
		var body struct {
			Name             string   `json:"name"`
			IsPublic         bool     `json:"is_public"`
			MaxFileSize      int64    `json:"max_file_size"`
			AllowedMimeTypes []string `json:"allowed_mime_types"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			handler.WriteError(w, "VALIDATION_ERROR", "invalid body", 400)
			return
		}

		resp, err := p.client.CreateBucket(r.Context(), &storagev1.CreateBucketRequest{
			Name: body.Name, IsPublic: body.IsPublic,
			MaxFileSize: body.MaxFileSize, AllowedMimeTypes: body.AllowedMimeTypes,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		handler.WriteJSON(w, 201, resp)
	})
}

func (p *StorageProxy) ListBuckets(w http.ResponseWriter, r *http.Request) {
	requireAuth(r, w, func() {
		resp, err := p.client.ListBuckets(r.Context(), &storagev1.ListBucketsRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		handler.WriteJSON(w, 200, resp.Buckets)
	})
}

func (p *StorageProxy) DeleteBucket(w http.ResponseWriter, r *http.Request) {
	requireAuth(r, w, func() {
		_, err := p.client.DeleteBucket(r.Context(), &storagev1.DeleteBucketRequest{Name: r.PathValue("bucket")})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		w.WriteHeader(204)
	})
}

func (p *StorageProxy) Upload(w http.ResponseWriter, r *http.Request) {
	requireAuth(r, w, func() {
		r.Body = http.MaxBytesReader(w, r.Body, 5*1024*1024*1024)
		file, header, err := r.FormFile("file")
		if err != nil {
			handler.WriteError(w, "VALIDATION_ERROR", "file required", 400)
			return
		}
		defer file.Close()

		content, err := io.ReadAll(file)
		if err != nil {
			handler.WriteError(w, "INTERNAL_ERROR", "failed to read file", 500)
			return
		}

		fileName := r.FormValue("name")
		if fileName == "" {
			fileName = header.Filename
		}
		mimeType := header.Header.Get("Content-Type")
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		ownerID, _ := r.Context().Value(middleware.UserIDKey).(string)

		resp, err := p.client.Upload(r.Context(), &storagev1.UploadRequest{
			Bucket: r.PathValue("bucket"), FileName: fileName,
			Content: content, MimeType: mimeType, OwnerId: ownerID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		handler.WriteJSON(w, 201, resp)
	})
}

func (p *StorageProxy) Download(w http.ResponseWriter, r *http.Request) {
	resp, err := p.client.Download(r.Context(), &storagev1.DownloadRequest{
		Bucket: r.PathValue("bucket"), FileName: r.PathValue("path"),
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	w.Header().Set("Content-Type", resp.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(resp.Size, 10))
	w.Write(resp.Content)
}

func (p *StorageProxy) DeleteFile(w http.ResponseWriter, r *http.Request) {
	requireAuth(r, w, func() {
		_, err := p.client.DeleteFile(r.Context(), &storagev1.DeleteFileRequest{
			Bucket: r.PathValue("bucket"), FileName: r.PathValue("path"),
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		w.WriteHeader(204)
	})
}

func (p *StorageProxy) CreateSignedURL(w http.ResponseWriter, r *http.Request) {
	requireAuth(r, w, func() {
		var body struct {
			FileName  string `json:"file_name"`
			ExpiresIn int32  `json:"expires_in"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			handler.WriteError(w, "VALIDATION_ERROR", "invalid body", 400)
			return
		}
		if body.ExpiresIn <= 0 {
			body.ExpiresIn = 3600
		}

		resp, err := p.client.CreateSignedURL(r.Context(), &storagev1.CreateSignedURLRequest{
			Bucket: r.PathValue("bucket"), FileName: body.FileName, ExpiresIn: body.ExpiresIn,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		handler.WriteJSON(w, 200, resp)
	})
}

func (p *StorageProxy) ListFiles(w http.ResponseWriter, r *http.Request) {
	requireAuth(r, w, func() {
		limit := int32(100)
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.ParseInt(l, 10, 32); err == nil {
				limit = int32(parsed)
			}
		}
		offset := int32(0)
		if o := r.URL.Query().Get("offset"); o != "" {
			if parsed, err := strconv.ParseInt(o, 10, 32); err == nil {
				offset = int32(parsed)
			}
		}

		resp, err := p.client.ListFiles(r.Context(), &storagev1.ListFilesRequest{
			Bucket: r.PathValue("bucket"), Prefix: r.URL.Query().Get("prefix"),
			Limit: limit, Offset: offset,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		handler.WriteJSON(w, 200, resp.Files)
	})
}

func requireAuth(r *http.Request, w http.ResponseWriter, fn func()) {
	if _, ok := r.Context().Value(middleware.UserIDKey).(string); !ok {
		handler.WriteError(w, "UNAUTHORIZED", "authentication required", 401)
		return
	}
	fn()
}
```

- [ ] **Step 10: Write main.go**

```go
// services/gateway/main.go
package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/garancehq/garance/services/gateway/internal/config"
	"github.com/garancehq/garance/services/gateway/internal/middleware"
	"github.com/garancehq/garance/services/gateway/internal/proxy"
)

func main() {
	cfg := config.Load()

	// Connect to backend services via gRPC
	engineProxy, err := proxy.NewEngineProxy(cfg.EngineGRPCAddr)
	if err != nil {
		log.Fatalf("failed to connect to engine: %v", err)
	}

	authProxy, err := proxy.NewAuthProxy(cfg.AuthGRPCAddr)
	if err != nil {
		log.Fatalf("failed to connect to auth: %v", err)
	}

	storageProxy, err := proxy.NewStorageProxy(cfg.StorageGRPCAddr)
	if err != nil {
		log.Fatalf("failed to connect to storage: %v", err)
	}

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	engineProxy.RegisterRoutes(mux)
	authProxy.RegisterRoutes(mux)
	storageProxy.RegisterRoutes(mux)

	// Middleware chain: Logging → CORS → JWT extraction → routes
	var handler http.Handler = mux
	handler = middleware.ExtractJWT(cfg.JWTSecret)(handler)
	handler = middleware.CORS(cfg.AllowedOrigins)(handler)
	handler = middleware.Logging(handler)

	server := &http.Server{Addr: cfg.ListenAddr, Handler: handler}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down gateway...")
		server.Shutdown(nil)
	}()

	log.Printf("garance gateway listening on %s", cfg.ListenAddr)
	log.Printf("  engine: %s | auth: %s | storage: %s", cfg.EngineGRPCAddr, cfg.AuthGRPCAddr, cfg.StorageGRPCAddr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
```

- [ ] **Step 11: Add dependencies**

```bash
cd /Users/jh3ady/Development/Projects/garance/services/gateway
go get google.golang.org/grpc
go get google.golang.org/protobuf
go get github.com/golang-jwt/jwt/v5
# Add proto module
# require + replace in go.mod for proto module
```

- [ ] **Step 12: Verify build**

Run: `go build ./...`

- [ ] **Step 13: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add services/gateway/
git commit -m ":sparkles: feat(gateway): add API Gateway with gRPC proxies for engine, auth, and storage"
```

---

## Task 6: Gateway Dockerfile

**Files:**
- Create: `services/gateway/Dockerfile`

- [ ] **Step 1: Write Dockerfile**

```dockerfile
# services/gateway/Dockerfile
FROM golang:1-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /garance-gateway .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /garance-gateway /usr/local/bin/garance-gateway

ENV LISTEN_ADDR=0.0.0.0:8080
EXPOSE 8080

CMD ["garance-gateway"]
```

- [ ] **Step 2: Build**

Run: `cd /Users/jh3ady/Development/Projects/garance/services/gateway && docker build -t garance-gateway:dev .`

- [ ] **Step 3: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add services/gateway/Dockerfile
git commit -m ":whale: build(gateway): add Dockerfile"
```

---

## Summary

| Task | Description | Scope |
|---|---|---|
| 1 | Proto definitions (engine, auth, storage) + buf code gen + tonic build.rs | Proto + Engine |
| 2 | Auth gRPC server (alongside HTTP) | Auth service |
| 3 | Storage gRPC server (alongside HTTP) | Storage service |
| 4 | Engine gRPC server (Rust/tonic, alongside HTTP) | Engine |
| 5 | Gateway service (HTTP → gRPC proxies, CORS, JWT, logging) | New service |
| 6 | Gateway Dockerfile | New service |

### Port allocation

| Service | HTTP | gRPC |
|---|---|---|
| Engine | 4000 | 5000 |
| Auth | 4001 | 5001 |
| Storage | 4002 | 5002 |
| Gateway | 8080 | — |

### Not in this plan (deferred)

- Rate limiting (v0.2)
- Request ID / trace propagation (deferred to observability plan)
- gRPC health checking protocol
- TLS between Gateway and services (not needed in Docker network)
- Streaming upload via gRPC (current approach sends full file in memory — fine for MVP, needs streaming for large files in v1)
