package config

import "os"

type Config struct {
	DatabaseURL string
	ListenAddr  string
	S3Endpoint  string
	S3Region    string
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string
	S3UseSSL    bool
	BaseURL     string
	JWTSecret   string
}

func Load() *Config {
	return &Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/postgres"),
		ListenAddr:  getEnv("LISTEN_ADDR", "0.0.0.0:4002"),
		S3Endpoint:  getEnv("S3_ENDPOINT", "localhost:9000"),
		S3Region:    getEnv("S3_REGION", "us-east-1"),
		S3Bucket:    getEnv("S3_BUCKET", "garance"),
		S3AccessKey: getEnv("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey: getEnv("S3_SECRET_KEY", "minioadmin"),
		S3UseSSL:    getEnv("S3_USE_SSL", "false") == "true",
		BaseURL:     getEnv("BASE_URL", "http://localhost:4002"),
		JWTSecret:   getEnv("JWT_SECRET", "dev-secret-change-me"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
