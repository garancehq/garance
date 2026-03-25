package proxy

import (
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/garancehq/garance/services/gateway/internal/handler"
	"github.com/garancehq/garance/services/gateway/internal/middleware"
)

// injectAuth extracts auth context from JWT middleware and passes it to the setter.
func injectAuth(r *http.Request, setter func(uid, pid, role string)) {
	uid, _ := r.Context().Value(middleware.UserIDKey).(string)
	pid, _ := r.Context().Value(middleware.ProjectIDKey).(string)
	role, _ := r.Context().Value(middleware.RoleKey).(string)
	setter(uid, pid, role)
}

// writeGRPCError maps gRPC status errors to HTTP error responses.
func writeGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		handler.WriteError(w, "INTERNAL_ERROR", "internal error", 500)
		return
	}
	switch st.Code() {
	case codes.NotFound:
		handler.WriteError(w, "NOT_FOUND", st.Message(), 404)
	case codes.InvalidArgument:
		handler.WriteError(w, "VALIDATION_ERROR", st.Message(), 400)
	case codes.Unauthenticated:
		handler.WriteError(w, "UNAUTHORIZED", st.Message(), 401)
	case codes.PermissionDenied:
		handler.WriteError(w, "PERMISSION_DENIED", st.Message(), 403)
	case codes.AlreadyExists:
		handler.WriteError(w, "CONFLICT", st.Message(), 409)
	default:
		handler.WriteError(w, "INTERNAL_ERROR", "internal error", 500)
	}
}

// requireAuth checks for authentication and calls fn if authenticated.
func requireAuth(r *http.Request, w http.ResponseWriter, fn func()) {
	if _, ok := r.Context().Value(middleware.UserIDKey).(string); !ok {
		handler.WriteError(w, "UNAUTHORIZED", "authentication required", 401)
		return
	}
	fn()
}
