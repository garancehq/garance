// services/auth/internal/handler/middleware.go
package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/garancehq/garance/services/auth/internal/token"
)

type contextKey string

const UserIDKey contextKey = "user_id"
const RoleKey contextKey = "role"

func AuthMiddleware(mgr *token.Manager) func(http.Handler) http.Handler {
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

			claims, err := mgr.ValidateAccessToken(parts[1])
			if err != nil {
				writeError(w, "UNAUTHORIZED", "invalid or expired token", 401)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, RoleKey, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
