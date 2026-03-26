package main

import (
	"context"
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

	realtimeProxy := proxy.NewRealtimeProxy()

	engineProxy.RegisterRoutes(mux)
	authProxy.RegisterRoutes(mux)
	storageProxy.RegisterRoutes(mux)
	realtimeProxy.RegisterRoutes(mux)

	// Middleware chain: Logging -> CORS -> JWT extraction -> routes
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
		server.Shutdown(context.Background())
	}()

	log.Printf("garance gateway listening on %s", cfg.ListenAddr)
	log.Printf("  engine: %s | auth: %s | storage: %s | realtime: ws", cfg.EngineGRPCAddr, cfg.AuthGRPCAddr, cfg.StorageGRPCAddr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
