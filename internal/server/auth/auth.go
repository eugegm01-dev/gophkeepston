package auth

import (
	"context"
	"database/sql"

	authpb "github.com/eugegm01-dev/gophkeepston/api/proto/auth"
	"github.com/eugegm01-dev/gophkeepston/internal/pkg/jwt"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthService struct {
	authpb.UnimplementedAuthServer
	db         *sql.DB
	jwtManager *jwt.Manager
}

func NewAuthService(db *sql.DB, jwtSecret string) *AuthService {
	return &AuthService{
		db:         db,
		jwtManager: jwt.NewManager(jwtSecret),
	}
}

func (s *AuthService) Register(ctx context.Context, req *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	id := uuid.New().String()
	_, err := s.db.Exec("INSERT INTO users (id, login, encrypted_secret) VALUES ($1, $2, $3)",
		id, req.Login, req.EncryptedSecret)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "register: %v", err)
	}
	return &authpb.RegisterResponse{UserId: id}, nil
}

func (s *AuthService) Login(ctx context.Context, req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	var userID string
	var encSecret []byte

	err := s.db.QueryRow("SELECT id, encrypted_secret FROM users WHERE login=$1", req.Login).
		Scan(&userID, &encSecret)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid login")
	}
	access, _ := s.jwtManager.GenerateAccessToken(userID)
	refresh, _ := s.jwtManager.GenerateRefreshToken(userID)
	// Сохраняем refresh token в БД (можно добавить таблицу refresh_tokens)
	return &authpb.LoginResponse{
		EncryptedSecret: encSecret,
		AccessToken:     access,
		RefreshToken:    refresh,
	}, nil
}
