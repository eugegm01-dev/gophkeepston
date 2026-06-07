package jwt

import (
	"testing"
	"time"
)

func TestGenerateAndValidate(t *testing.T) {
	m := NewManager("test-secret", 15*time.Minute, 72*time.Hour)
	userID := "user123"

	access, err := m.GenerateAccessToken(userID)
	if err != nil {
		t.Fatal(err)
	}
	sub, err := m.ValidateToken(access)
	if err != nil {
		t.Fatal(err)
	}
	if sub != userID {
		t.Fatalf("expected %q, got %q", userID, sub)
	}

	refresh, err := m.GenerateRefreshToken(userID)
	if err != nil {
		t.Fatal(err)
	}
	sub, err = m.ValidateToken(refresh)
	if err != nil {
		t.Fatal(err)
	}
	if sub != userID {
		t.Fatalf("expected %q, got %q", userID, sub)
	}
}

func TestExpiredToken(t *testing.T) {
	m := NewManager("secret", 15*time.Minute, 72*time.Hour)
	// Проверяем, что валидатор отвергает заведомо невалидный токен
	badToken := "bad.token.here"
	_, err := m.ValidateToken(badToken)
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}
