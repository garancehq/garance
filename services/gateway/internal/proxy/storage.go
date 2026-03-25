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
