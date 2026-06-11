package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/eugegm01-dev/gophkeepston/internal/pkg/jwt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestUnaryAuthInterceptor(t *testing.T) {
	mngr := jwt.NewManager("test-secret", 15*time.Minute, 72*time.Hour)
	// Инициализируем RateLimiter для теста (с большими лимитами, чтобы не блокировал тест)
	rl := NewRateLimiter(100, time.Minute)
	interceptor := UnaryAuthInterceptor(mngr, rl)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		id, _ := UserIDFromContext(ctx)
		return id, nil
	}

	// 1. Без метаданных
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/some.service/Method"}, handler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}

	// 2. С невалидным токеном
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer bad.token"))
	_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/gophkeeper.sync.Sync/Push"}, handler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}

	// 3. С валидным токеном
	token, _ := mngr.GenerateAccessToken("user42")
	ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))

	// ВАЖНО: grpc.UnaryServerInterceptor работает с контекстом,
	// но в тестах мы вызываем его как обычную функцию.
	resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/gophkeeper.sync.Sync/Push"}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID, ok := resp.(string); !ok || userID != "user42" {
		t.Fatalf("expected user_id 'user42', got %v", resp)
	}

	// 4. Логин/Регистрация (метод без авторизации)
	// Важно: RateLimiter всё равно сработает (это правильно), поэтому передаем пустой контекст
	ctx = context.Background()
	resp, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/gophkeeper.auth.Auth/Login"}, handler)
	if err != nil {
		t.Fatalf("unexpected error for auth method: %v", err)
	}
	// Здесь handler вернет nil, так как UserID в контексте нет
}
