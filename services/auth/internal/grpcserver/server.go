package grpcserver

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authv1 "github.com/garancehq/garance/proto/gen/go/auth/v1"
	"github.com/garancehq/garance/services/auth/internal/service"
	"github.com/garancehq/garance/services/auth/internal/store"
)

type AuthGRPCServer struct {
	authv1.UnimplementedAuthServiceServer
	auth *service.AuthService
}

func NewAuthGRPCServer(auth *service.AuthService) *AuthGRPCServer {
	return &AuthGRPCServer{auth: auth}
}

func (s *AuthGRPCServer) Register(srv *grpc.Server) {
	authv1.RegisterAuthServiceServer(srv, s)
}

func (s *AuthGRPCServer) SignUp(ctx context.Context, req *authv1.SignUpRequest) (*authv1.AuthResponse, error) {
	result, err := s.auth.SignUp(ctx, req.Email, req.Password, req.UserAgent, req.IpAddress)
	if err != nil {
		return nil, mapAuthError(err)
	}
	return toAuthResponse(result), nil
}

func (s *AuthGRPCServer) SignIn(ctx context.Context, req *authv1.SignInRequest) (*authv1.AuthResponse, error) {
	result, err := s.auth.SignIn(ctx, req.Email, req.Password, req.UserAgent, req.IpAddress)
	if err != nil {
		return nil, mapAuthError(err)
	}
	return toAuthResponse(result), nil
}

func (s *AuthGRPCServer) RefreshToken(ctx context.Context, req *authv1.RefreshTokenRequest) (*authv1.AuthResponse, error) {
	result, err := s.auth.RefreshToken(ctx, req.RefreshToken, req.UserAgent, req.IpAddress)
	if err != nil {
		return nil, mapAuthError(err)
	}
	return toAuthResponse(result), nil
}

func (s *AuthGRPCServer) SignOut(ctx context.Context, req *authv1.SignOutRequest) (*authv1.SignOutResponse, error) {
	_ = s.auth.SignOut(ctx, req.RefreshToken)
	return &authv1.SignOutResponse{}, nil
}

func (s *AuthGRPCServer) GetUser(ctx context.Context, req *authv1.GetUserRequest) (*authv1.UserResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}
	user, err := s.auth.GetUser(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "internal error")
	}
	return toUserResponse(user), nil
}

func (s *AuthGRPCServer) DeleteUser(ctx context.Context, req *authv1.DeleteUserRequest) (*authv1.DeleteUserResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}
	if err := s.auth.DeleteUser(ctx, userID); err != nil {
		return nil, status.Error(codes.Internal, "failed to delete user")
	}
	return &authv1.DeleteUserResponse{}, nil
}

func toAuthResponse(result *service.AuthResult) *authv1.AuthResponse {
	return &authv1.AuthResponse{
		User: toUserResponse(result.User),
		TokenPair: &authv1.TokenPair{
			AccessToken:  result.TokenPair.AccessToken,
			RefreshToken: result.TokenPair.RefreshToken,
			ExpiresIn:    result.TokenPair.ExpiresIn,
			TokenType:    result.TokenPair.TokenType,
		},
	}
}

func toUserResponse(user *store.User) *authv1.UserResponse {
	return &authv1.UserResponse{
		Id:            user.ID.String(),
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		Role:          user.Role,
		CreatedAt:     user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     user.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func mapAuthError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, service.ErrUserBanned):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, store.ErrEmailAlreadyTaken):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, service.ErrPasswordRequired), errors.Is(err, service.ErrWeakPassword):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
