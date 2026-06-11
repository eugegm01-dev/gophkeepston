package middleware

import (
	"context"
	"testing"

	"github.com/eugegm01-dev/gophkeepston/internal/pkg/jwt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestUnaryAuthInterceptor(t *testing.T) {
	mngr := jwt.NewManager("test-secret")
	interceptor := UnaryAuthInterceptor(mngr)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return ctx.Value(UserIDKey), nil
	}

	// Без метаданных – ошибка
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/some.service/Method"}, handler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}

	// С невалидным токеном
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer bad.token"))
	_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/gophkeeper.sync.Sync/Push"}, handler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}

	// С валидным токеном
	token, _ := mngr.GenerateAccessToken("user42")
	ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))
	resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/gophkeeper.sync.Sync/Push"}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID, ok := resp.(string); !ok || userID != "user42" {
		t.Fatalf("expected user_id 'user42', got %v", resp)
	}

	// Метод регистрации/логина пропускается без проверки
	ctx = context.Background()
	resp, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/gophkeeper.auth.Auth/Login"}, handler)
	if err != nil {
		t.Fatalf("unexpected error for auth method: %v", err)
	}
	if resp != nil {
		t.Fatalf("expected nil response, got %v", resp)
	}
}
