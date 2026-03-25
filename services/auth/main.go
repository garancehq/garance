package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/garancehq/garance/services/auth/internal/config"
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
	authHandler := handler.NewAuthHandler(authService, tokenMgr)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	authHandler.RegisterRoutes(mux)

	server := &http.Server{Addr: cfg.ListenAddr, Handler: mux}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		server.Shutdown(ctx)
	}()

	log.Printf("garance auth service listening on %s", cfg.ListenAddr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
