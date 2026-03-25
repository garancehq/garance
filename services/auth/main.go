package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/garancehq/garance/services/auth/internal/config"
)

func main() {
	cfg := config.Load()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	log.Printf("garance auth service listening on %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		log.Fatal(err)
	}
	_ = fmt.Sprintf("base url: %s", cfg.BaseURL)
}
