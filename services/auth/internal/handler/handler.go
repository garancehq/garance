// services/auth/internal/handler/handler.go
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/garancehq/garance/services/auth/internal/crypto"
	"github.com/garancehq/garance/services/auth/internal/service"
	"github.com/garancehq/garance/services/auth/internal/store"
	"github.com/garancehq/garance/services/auth/internal/token"
	"github.com/google/uuid"
)

type AuthHandler struct {
	auth          *service.AuthService
	tokens        *token.Manager
	encryptionKey []byte
	baseURL       string
}

func NewAuthHandler(auth *service.AuthService, tokens *token.Manager, encryptionKey []byte, baseURL string) *AuthHandler {
	return &AuthHandler{auth: auth, tokens: tokens, encryptionKey: encryptionKey, baseURL: baseURL}
}

func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/v1/signup", h.SignUp)
	mux.HandleFunc("POST /auth/v1/signin", h.SignIn)
	mux.HandleFunc("POST /auth/v1/token/refresh", h.RefreshToken)
	mux.HandleFunc("POST /auth/v1/signout", h.SignOut)

	// OAuth flow routes
	mux.HandleFunc("GET /auth/v1/oauth/{provider}", h.OAuthInitiate)
	mux.HandleFunc("GET /auth/v1/oauth/{provider}/callback", h.OAuthCallback)

	// Admin routes (internal port, no auth required)
	mux.HandleFunc("GET /auth/v1/admin/users", h.ListUsers)
	mux.HandleFunc("GET /auth/v1/admin/providers", h.ListProviders)
	mux.HandleFunc("POST /auth/v1/admin/providers", h.CreateProvider)
	mux.HandleFunc("PATCH /auth/v1/admin/providers/{provider}", h.UpdateProvider)
	mux.HandleFunc("DELETE /auth/v1/admin/providers/{provider}", h.DeleteProvider)

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

func (h *AuthHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil {
			offset = parsed
		}
	}

	users, err := h.auth.DB().ListUsers(r.Context(), limit, offset)
	if err != nil {
		writeError(w, "INTERNAL_ERROR", "failed to list users", 500)
		return
	}
	count, _ := h.auth.DB().CountUsers(r.Context())

	if users == nil {
		users = []store.User{}
	}
	writeJSON(w, 200, map[string]interface{}{"users": users, "total": count})
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

// ─── OAuth Flow ──────────────────────────────────────────────────────────────

func (h *AuthHandler) OAuthInitiate(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	redirectURI := r.URL.Query().Get("redirect_uri")
	if redirectURI == "" {
		writeError(w, "VALIDATION_ERROR", "redirect_uri is required", 400)
		return
	}

	authorizeURL, err := h.auth.OAuthAuthorize(r.Context(), provider, redirectURI, h.baseURL, h.encryptionKey)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrOAuthProviderNotConfigured):
			writeError(w, "NOT_FOUND", err.Error(), 404)
		case errors.Is(err, service.ErrInvalidRedirectURI):
			writeError(w, "VALIDATION_ERROR", err.Error(), 400)
		default:
			writeError(w, "INTERNAL_ERROR", "failed to initiate OAuth", 500)
		}
		return
	}

	http.Redirect(w, r, authorizeURL, http.StatusFound)
}

func (h *AuthHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		writeError(w, "VALIDATION_ERROR", "code and state are required", 400)
		return
	}

	result, redirectURI, err := h.auth.OAuthCallback(r.Context(), provider, code, state, h.baseURL, r.UserAgent(), r.RemoteAddr, h.encryptionKey)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidOAuthState):
			writeError(w, "UNAUTHORIZED", err.Error(), 401)
		case errors.Is(err, service.ErrOAuthProviderNotConfigured):
			writeError(w, "NOT_FOUND", err.Error(), 404)
		case errors.Is(err, service.ErrOAuthNoEmail):
			writeError(w, "VALIDATION_ERROR", err.Error(), 400)
		case errors.Is(err, service.ErrUserBanned):
			writeError(w, "PERMISSION_DENIED", err.Error(), 403)
		default:
			writeError(w, "INTERNAL_ERROR", "OAuth callback failed", 500)
		}
		return
	}

	// Redirect to the original redirect_uri with tokens in query params
	u, err := url.Parse(redirectURI)
	if err != nil {
		writeError(w, "INTERNAL_ERROR", "invalid redirect URI", 500)
		return
	}

	q := u.Query()
	q.Set("access_token", result.TokenPair.AccessToken)
	q.Set("refresh_token", result.TokenPair.RefreshToken)
	q.Set("expires_in", fmt.Sprintf("%d", result.TokenPair.ExpiresIn))
	q.Set("token_type", result.TokenPair.TokenType)
	u.RawQuery = q.Encode()

	http.Redirect(w, r, u.String(), http.StatusFound)
}

// ─── Admin API ───────────────────────────────────────────────────────────────

type providerResponse struct {
	ID        uuid.UUID `json:"id"`
	Provider  string    `json:"provider"`
	ClientID  string    `json:"client_id"`
	HasSecret bool      `json:"has_secret"`
	Enabled   bool      `json:"enabled"`
	Scopes    string    `json:"scopes"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
}

func toProviderResponse(p store.OAuthProvider) providerResponse {
	return providerResponse{
		ID:        p.ID,
		Provider:  p.Provider,
		ClientID:  p.ClientID,
		HasSecret: p.ClientSecretEncrypted != "",
		Enabled:   p.Enabled,
		Scopes:    p.Scopes,
		CreatedAt: p.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: p.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func (h *AuthHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := h.auth.DB().ListProviders(r.Context())
	if err != nil {
		writeError(w, "INTERNAL_ERROR", "failed to list providers", 500)
		return
	}

	resp := make([]providerResponse, len(providers))
	for i, p := range providers {
		resp[i] = toProviderResponse(p)
	}
	writeJSON(w, http.StatusOK, resp)
}

type createProviderRequest struct {
	Provider     string `json:"provider"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Scopes       string `json:"scopes"`
}

func (h *AuthHandler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	var req createProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}

	if req.Provider == "" || req.ClientID == "" || req.ClientSecret == "" {
		writeError(w, "VALIDATION_ERROR", "provider, client_id, and client_secret are required", 400)
		return
	}

	encrypted, err := crypto.Encrypt(req.ClientSecret, h.encryptionKey)
	if err != nil {
		writeError(w, "INTERNAL_ERROR", "failed to encrypt secret", 500)
		return
	}

	p, err := h.auth.DB().CreateProvider(r.Context(), req.Provider, req.ClientID, encrypted, req.Scopes)
	if err != nil {
		if errors.Is(err, store.ErrProviderAlreadyExists) {
			writeError(w, "CONFLICT", "provider already exists", 409)
			return
		}
		writeError(w, "INTERNAL_ERROR", "failed to create provider", 500)
		return
	}

	writeJSON(w, http.StatusCreated, toProviderResponse(*p))
}

type updateProviderRequest struct {
	ClientID     *string `json:"client_id,omitempty"`
	ClientSecret *string `json:"client_secret,omitempty"`
	Enabled      *bool   `json:"enabled,omitempty"`
	Scopes       *string `json:"scopes,omitempty"`
}

func (h *AuthHandler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")

	var req updateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}

	// Encrypt client_secret if provided
	var encryptedSecret *string
	if req.ClientSecret != nil {
		enc, err := crypto.Encrypt(*req.ClientSecret, h.encryptionKey)
		if err != nil {
			writeError(w, "INTERNAL_ERROR", "failed to encrypt secret", 500)
			return
		}
		encryptedSecret = &enc
	}

	if err := h.auth.DB().UpdateProvider(r.Context(), provider, req.ClientID, encryptedSecret, req.Enabled, req.Scopes); err != nil {
		if errors.Is(err, store.ErrProviderNotFound) {
			writeError(w, "NOT_FOUND", "provider not found", 404)
			return
		}
		writeError(w, "INTERNAL_ERROR", "failed to update provider", 500)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")

	if err := h.auth.DB().DeleteProvider(r.Context(), provider); err != nil {
		if errors.Is(err, store.ErrProviderNotFound) {
			writeError(w, "NOT_FOUND", "provider not found", 404)
			return
		}
		writeError(w, "INTERNAL_ERROR", "failed to delete provider", 500)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
