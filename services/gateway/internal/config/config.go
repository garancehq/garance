package config

import "os"

type Config struct {
	ListenAddr      string
	EngineGRPCAddr  string
	AuthGRPCAddr    string
	StorageGRPCAddr string
	JWTSecret       string
	AllowedOrigins  string
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
