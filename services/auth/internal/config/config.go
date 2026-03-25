package config

import "os"

type Config struct {
	DatabaseURL string
	ListenAddr  string
	JWTSecret   string
	SMTPHost    string
	SMTPPort    string
	SMTPUser    string
	SMTPPass    string
	SMTPFrom    string
	BaseURL     string
}

func Load() *Config {
	return &Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/postgres"),
		ListenAddr:  getEnv("LISTEN_ADDR", "0.0.0.0:4001"),
		JWTSecret:   getEnv("JWT_SECRET", "dev-secret-change-me"),
		SMTPHost:    getEnv("SMTP_HOST", "localhost"),
		SMTPPort:    getEnv("SMTP_PORT", "1025"),
		SMTPUser:    getEnv("SMTP_USER", ""),
		SMTPPass:    getEnv("SMTP_PASS", ""),
		SMTPFrom:    getEnv("SMTP_FROM", "noreply@garance.io"),
		BaseURL:     getEnv("BASE_URL", "http://localhost:4001"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
