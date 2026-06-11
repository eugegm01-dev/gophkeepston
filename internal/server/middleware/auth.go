// Package middleware provides gRPC interceptors for authentication.
package middleware

import (
	"context"

	"github.com/eugegm01-dev/gophkeepston/internal/pkg/jwt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const userIDKey contextKey = "user_id"

var UserIDKey = userIDKey

// UnaryAuthInterceptor returns a gRPC unary interceptor that validates JWT tokens.
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

		ctx = context.WithValue(ctx, userIDKey, userID)
		return handler(ctx, req)
	}
}
