package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	"github.com/garancehq/garance/services/auth/internal/config"
	"github.com/garancehq/garance/services/auth/internal/crypto"
	"github.com/garancehq/garance/services/auth/internal/grpcserver"
	"github.com/garancehq/garance/services/auth/internal/handler"
	"github.com/garancehq/garance/services/auth/internal/service"
	"github.com/garancehq/garance/services/auth/internal/store"
	"github.com/garancehq/garance/services/auth/internal/token"
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

	tokenMgr := token.NewManager(cfg.JWTSecret)
	authService := service.NewAuthService(db, tokenMgr)

	// Derive encryption key for OAuth client secrets
	var encryptionKey []byte
	if cfg.EncryptionKey != "" {
		encryptionKey = crypto.DeriveKey(cfg.EncryptionKey)
	} else {
		log.Println("WARNING: ENCRYPTION_KEY not set, using default dev key — do NOT use in production")
		encryptionKey = crypto.DeriveKey("garance-dev-encryption-key")
	}

	authHandler := handler.NewAuthHandler(authService, tokenMgr, encryptionKey, cfg.BaseURL)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	authHandler.RegisterRoutes(mux)

	server := &http.Server{Addr: cfg.ListenAddr, Handler: mux}

	// gRPC server
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

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		grpcSrv.GracefulStop()
		server.Shutdown(ctx)
	}()

	log.Printf("garance auth service listening on %s", cfg.ListenAddr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
