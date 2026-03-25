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
