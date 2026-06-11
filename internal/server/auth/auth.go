package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"log/slog"
	"time"

	authpb "github.com/eugegm01-dev/gophkeepston/api/proto/auth"
	"github.com/eugegm01-dev/gophkeepston/internal/pkg/jwt"
	"github.com/eugegm01-dev/gophkeepston/internal/server/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthService struct {
	authpb.UnimplementedAuthServer
	db          DBPool
	jwtManager  *jwt.Manager
	rateLimiter *middleware.RateLimiter
}

func NewAuthService(db DBPool, jwtManager *jwt.Manager, rl *middleware.RateLimiter) *AuthService {
	return &AuthService{db: db, jwtManager: jwtManager, rateLimiter: rl}
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

func (s *AuthService) Login(ctx context.Context, req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	// 1. Rate limit в самом начале
	if !s.rateLimiter.Allow(req.Login) {
		return nil, status.Error(codes.ResourceExhausted, "too many login attempts")
	}

	// 2. Один запрос к БД
	var userID string
	var encSecret []byte
	var salt []byte
	err := s.db.QueryRow(ctx,
		"SELECT id, encrypted_secret, salt FROM users WHERE login=$1", req.Login).
		Scan(&userID, &encSecret, &salt)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid login")
	}

	// 3. Генерация токенов
	access, err := s.jwtManager.GenerateAccessToken(userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate access token: %v", err)
	}
	refresh, err := s.jwtManager.GenerateRefreshToken(userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate refresh token: %v", err)
	}

	// 4. Сохранение refresh токена
	_, err = s.db.Exec(ctx, `INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
		uuid.New().String(), userID, hashToken(refresh), time.Now().Add(72*time.Hour))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "save refresh token: %v", err)
	}

	return &authpb.LoginResponse{
		EncryptedSecret: encSecret,
		AccessToken:     access,
		RefreshToken:    refresh,
		Salt:            salt,
	}, nil
}

func (s *AuthService) Register(ctx context.Context, req *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	id := uuid.New().String()
	salt := make([]byte, 16)
	_, err := rand.Read(salt)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate salt: %v", err)
	}
	_, err = s.db.Exec(ctx,
		`INSERT INTO users (id, login, encrypted_secret, salt) VALUES ($1, $2, $3, $4)`,
		id, req.Login, req.EncryptedSecret, salt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique violation
			return nil, status.Error(codes.AlreadyExists, "login already taken")
		}
		return nil, status.Errorf(codes.Internal, "register: %v", err)
	}
	return &authpb.RegisterResponse{UserId: id}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, req *authpb.RefreshTokenRequest) (*authpb.RefreshTokenResponse, error) {
	var userID string
	oldTokenHash := hashToken(req.RefreshToken)
	err := s.db.QueryRow(ctx,
		`SELECT user_id FROM refresh_tokens WHERE token_hash = $1 AND expires_at > now()`,
		oldTokenHash,
	).Scan(&userID)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid or expired refresh token")
	}

	access, err := s.jwtManager.GenerateAccessToken(userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate access token: %v", err)
	}
	newRefresh, err := s.jwtManager.GenerateRefreshToken(userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate refresh token: %v", err)
	}

	// Атомарная ротация (Требование r п.5)
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx, `DELETE FROM refresh_tokens WHERE token_hash = $1`, oldTokenHash); err != nil {
		return nil, status.Errorf(codes.Internal, "delete old refresh token: %v", err)
	}
	if _, err = tx.Exec(ctx,
		`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
		uuid.New().String(), userID, hashToken(newRefresh), time.Now().Add(72*time.Hour),
	); err != nil {
		return nil, status.Errorf(codes.Internal, "insert new refresh token: %v", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "commit tx: %v", err)
	}

	slog.InfoContext(ctx, "token refreshed", "user_id", userID)
	return &authpb.RefreshTokenResponse{
		AccessToken:  access,
		RefreshToken: newRefresh,
	}, nil
}
