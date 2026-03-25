package handler

import (
	"encoding/json"
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
