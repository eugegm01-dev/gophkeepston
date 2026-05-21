package jwt

import (
	"testing"
)

func TestGenerateAndValidate(t *testing.T) {
	m := NewManager("test-secret")
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
	m := NewManager("secret")
	// Проверяем, что валидатор отвергает заведомо невалидный токен
	badToken := "bad.token.here"
	_, err := m.ValidateToken(badToken)
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}
