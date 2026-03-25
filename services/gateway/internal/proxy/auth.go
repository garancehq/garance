package proxy

import (
	"encoding/json"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	authv1 "github.com/garancehq/garance/proto/gen/go/auth/v1"
	"github.com/garancehq/garance/services/gateway/internal/handler"
	"github.com/garancehq/garance/services/gateway/internal/middleware"
)

type AuthProxy struct {
	client authv1.AuthServiceClient
}

func NewAuthProxy(addr string) (*AuthProxy, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &AuthProxy{client: authv1.NewAuthServiceClient(conn)}, nil
}

func (p *AuthProxy) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/v1/signup", p.SignUp)
	mux.HandleFunc("POST /auth/v1/signin", p.SignIn)
	mux.HandleFunc("POST /auth/v1/token/refresh", p.RefreshToken)
	mux.HandleFunc("POST /auth/v1/signout", p.SignOut)
	mux.HandleFunc("GET /auth/v1/user", p.GetUser)
	mux.HandleFunc("DELETE /auth/v1/user", p.DeleteUser)
}

func (p *AuthProxy) SignUp(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		handler.WriteError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}

	resp, err := p.client.SignUp(r.Context(), &authv1.SignUpRequest{
		Email:     body.Email,
		Password:  body.Password,
		UserAgent: r.UserAgent(),
		IpAddress: r.RemoteAddr,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	handler.WriteJSON(w, 201, resp)
}

func (p *AuthProxy) SignIn(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		handler.WriteError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}

	resp, err := p.client.SignIn(r.Context(), &authv1.SignInRequest{
		Email:     body.Email,
		Password:  body.Password,
		UserAgent: r.UserAgent(),
		IpAddress: r.RemoteAddr,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	handler.WriteJSON(w, 200, resp)
}

func (p *AuthProxy) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		handler.WriteError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}

	resp, err := p.client.RefreshToken(r.Context(), &authv1.RefreshTokenRequest{
		RefreshToken: body.RefreshToken,
		UserAgent:    r.UserAgent(),
		IpAddress:    r.RemoteAddr,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	handler.WriteJSON(w, 200, resp)
}

func (p *AuthProxy) SignOut(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		handler.WriteError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}

	_, err := p.client.SignOut(r.Context(), &authv1.SignOutRequest{
		RefreshToken: body.RefreshToken,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	w.WriteHeader(204)
}

func (p *AuthProxy) GetUser(w http.ResponseWriter, r *http.Request) {
	uid, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || uid == "" {
		handler.WriteError(w, "UNAUTHORIZED", "authentication required", 401)
		return
	}

	resp, err := p.client.GetUser(r.Context(), &authv1.GetUserRequest{UserId: uid})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	handler.WriteJSON(w, 200, resp)
}

func (p *AuthProxy) DeleteUser(w http.ResponseWriter, r *http.Request) {
	uid, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || uid == "" {
		handler.WriteError(w, "UNAUTHORIZED", "authentication required", 401)
		return
	}

	_, err := p.client.DeleteUser(r.Context(), &authv1.DeleteUserRequest{UserId: uid})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	w.WriteHeader(204)
}
