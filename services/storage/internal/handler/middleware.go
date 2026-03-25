package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserIDKey contextKey = "user_id"

type JWTClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// OptionalAuth extracts the user ID from JWT if present, but doesn't require it.
// Used for public bucket access where auth is optional.
func OptionalAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				next.ServeHTTP(w, r)
				return
			}

			token, err := jwt.ParseWithClaims(parts[1], &JWTClaims{}, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method")
				}
				return []byte(secret), nil
			})
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
				ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth rejects requests without valid JWT.
func RequireAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, "UNAUTHORIZED", "missing authorization header", 401)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				writeError(w, "UNAUTHORIZED", "invalid authorization format", 401)
				return
			}

			token, err := jwt.ParseWithClaims(parts[1], &JWTClaims{}, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method")
				}
				return []byte(secret), nil
			})
			if err != nil {
				writeError(w, "UNAUTHORIZED", "invalid or expired token", 401)
				return
			}

			claims, ok := token.Claims.(*JWTClaims)
			if !ok || !token.Valid {
				writeError(w, "UNAUTHORIZED", "invalid token", 401)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
