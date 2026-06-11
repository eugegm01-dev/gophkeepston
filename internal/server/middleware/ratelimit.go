package middleware

import (
	"context"
	"sync"
	"time"

	"github.com/eugegm01-dev/gophkeepston/internal/pkg/jwt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// RateLimiter защищает от брутфорса (требование безопасности)
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string]*attempt
	max      int
	window   time.Duration
}
type contextKey string

const UserIDKey contextKey = "userID"

type attempt struct {
	count int
	first time.Time
}

func NewRateLimiter(max int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		attempts: make(map[string]*attempt),
		max:      max,
		window:   window,
	}
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	a, ok := rl.attempts[key]
	if !ok || now.Sub(a.first) > rl.window {
		rl.attempts[key] = &attempt{count: 1, first: now}
		return true
	}
	a.count++
	return a.count <= rl.max
}

// UnaryAuthInterceptor добавляет rate-limit к auth-методам
func UnaryAuthInterceptor(jwtManager *jwt.Manager) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if info.FullMethod == "/gophkeeper.auth.Auth/Register" || info.FullMethod == "/gophkeeper.auth.Auth/Login" || info.FullMethod == "/gophkeeper.auth.Auth/RefreshToken" {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		tokens := md.Get("authorization")
		if len(tokens) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing token")
		}

		token := tokens[0]
		// Убираем префикс "Bearer ", если есть
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		userID, err := jwtManager.ValidateToken(token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		ctx = context.WithValue(ctx, UserIDKey, userID)
		return handler(ctx, req)
	}
}
