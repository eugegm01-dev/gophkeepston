package session

import (
	"context"
	"testing"

	authpb "github.com/eugegm01-dev/gophkeepston/api/proto/auth"
)

type mockClient struct {
	refreshFunc func(ctx context.Context, token string) (*authpb.RefreshTokenResponse, error)
}

func (m *mockClient) Refresh(ctx context.Context, token string) (*authpb.RefreshTokenResponse, error) {
	if m.refreshFunc != nil {
		return m.refreshFunc(ctx, token)
	}
	return nil, nil
}

func (m *mockClient) Close() error { return nil }

func TestEnsureFreshAccess_Success(t *testing.T) {
	s := &Session{
		AccessToken:  "expired_jwt_string",
		RefreshToken: "valid_refresh",
	}

	mock := &mockClient{
		refreshFunc: func(ctx context.Context, token string) (*authpb.RefreshTokenResponse, error) {
			return &authpb.RefreshTokenResponse{
				AccessToken:  "new_access_token",
				RefreshToken: "new_refresh_token",
			}, nil
		},
	}

	// Ключ должен быть 32 байта для AES-256
	key := []byte("12345678901234567890123456789012") // 32 байта
	err := s.EnsureFreshAccessWithClient(context.Background(), mock, key)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s.AccessToken != "new_access_token" {
		t.Errorf("expected new_access_token, got %s", s.AccessToken)
	}
}
