// services/auth/internal/handler/handler.go
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/garancehq/garance/services/auth/internal/service"
	"github.com/garancehq/garance/services/auth/internal/store"
	"github.com/garancehq/garance/services/auth/internal/token"
	"github.com/google/uuid"
)

type AuthHandler struct {
	auth   *service.AuthService
	tokens *token.Manager
}

func NewAuthHandler(auth *service.AuthService, tokens *token.Manager) *AuthHandler {
	return &AuthHandler{auth: auth, tokens: tokens}
}

func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/v1/signup", h.SignUp)
	mux.HandleFunc("POST /auth/v1/signin", h.SignIn)
	mux.HandleFunc("POST /auth/v1/token/refresh", h.RefreshToken)
	mux.HandleFunc("POST /auth/v1/signout", h.SignOut)

	// Protected routes
	protected := http.NewServeMux()
	protected.HandleFunc("GET /auth/v1/user", h.GetUser)
	protected.HandleFunc("DELETE /auth/v1/user", h.DeleteUser)

	middleware := AuthMiddleware(h.tokens)
	mux.Handle("GET /auth/v1/user", middleware(protected))
	mux.Handle("DELETE /auth/v1/user", middleware(protected))
}

type signUpRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) SignUp(w http.ResponseWriter, r *http.Request) {
	var req signUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}

	if req.Email == "" {
		writeError(w, "VALIDATION_ERROR", "email is required", 400)
		return
	}

	result, err := h.auth.SignUp(r.Context(), req.Email, req.Password, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		h.handleAuthError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

type signInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) SignIn(w http.ResponseWriter, r *http.Request) {
	var req signInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}

	result, err := h.auth.SignIn(r.Context(), req.Email, req.Password, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		h.handleAuthError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}

	result, err := h.auth.RefreshToken(r.Context(), req.RefreshToken, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		h.handleAuthError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

type signOutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) SignOut(w http.ResponseWriter, r *http.Request) {
	var req signOutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}

	h.auth.SignOut(r.Context(), req.RefreshToken)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		writeError(w, "UNAUTHORIZED", "missing user context", 401)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, "UNAUTHORIZED", "invalid user id", 401)
		return
	}

	user, err := h.auth.GetUser(r.Context(), userID)
	if err != nil {
		writeError(w, "NOT_FOUND", "user not found", 404)
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (h *AuthHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		writeError(w, "UNAUTHORIZED", "missing user context", 401)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, "UNAUTHORIZED", "invalid user id", 401)
		return
	}

	if err := h.auth.DeleteUser(r.Context(), userID); err != nil {
		writeError(w, "INTERNAL_ERROR", "failed to delete user", 500)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) handleAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		writeError(w, "UNAUTHORIZED", err.Error(), 401)
	case errors.Is(err, service.ErrUserBanned):
		writeError(w, "PERMISSION_DENIED", err.Error(), 403)
	case errors.Is(err, store.ErrEmailAlreadyTaken):
		writeError(w, "CONFLICT", err.Error(), 409)
	case errors.Is(err, service.ErrPasswordRequired), errors.Is(err, service.ErrWeakPassword):
		writeError(w, "VALIDATION_ERROR", err.Error(), 400)
	default:
		writeError(w, "INTERNAL_ERROR", "internal server error", 500)
	}
}
