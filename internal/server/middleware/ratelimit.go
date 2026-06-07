package middleware

import (
	"context"
	"sync"
	"time"

	"github.com/eugegm01-dev/gophkeepston/internal/pkg/jwt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// RateLimiter защищает от брутфорса
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string]*attempt
	max      int
	window   time.Duration
	stopCh   chan struct{}
}

func NewRateLimiter(max int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		attempts: make(map[string]*attempt),
		max:      max,
		window:   window,
		stopCh:   make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for key, a := range rl.attempts {
				if now.Sub(a.first) > rl.window {
					delete(rl.attempts, key)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

type attempt struct {
	count int
	first time.Time
}

type contextKey string

const userIDKey contextKey = "userID"

func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	a, ok := rl.attempts[key]

	// Удаляем старые записи
	if ok && now.Sub(a.first) > rl.window {
		delete(rl.attempts, key)
		ok = false
	}

	if !ok {
		rl.attempts[key] = &attempt{count: 1, first: now}
		return true
	}

	a.count++
	return a.count <= rl.max
}

// UnaryAuthInterceptor объединяет RateLimit и JWT-валидацию
func UnaryAuthInterceptor(jwtManager *jwt.Manager, rl *RateLimiter) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

		// 1. Сначала Rate Limit (защита от нагрузки до тяжелых операций)
		p, ok := peer.FromContext(ctx)
		if ok {
			if !rl.Allow(p.Addr.String()) {
				return nil, status.Error(codes.ResourceExhausted, "too many requests")
			}
		}

		// 2. Исключаем методы, не требующие авторизации
		if info.FullMethod == "/gophkeeper.auth.Auth/Register" ||
			info.FullMethod == "/gophkeeper.auth.Auth/Login" {
			return handler(ctx, req)
		}

		// 3. JWT-валидация
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		tokens := md.Get("authorization")
		if len(tokens) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing token")
		}

		token := tokens[0]
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		userID, err := jwtManager.ValidateToken(token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		// 4. Пробрасываем userID в контекст
		newCtx := context.WithValue(ctx, userIDKey, userID)
		return handler(newCtx, req)
	}
}
