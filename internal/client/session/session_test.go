package session

import (
	"os"
	"testing"

	"github.com/eugegm01-dev/gophkeepston/internal/client/crypto"
)

func TestSaveLoad(t *testing.T) {
	tmpFile := "test_session.enc"
	oldFile := SessionFile
	SessionFile = tmpFile
	defer func() {
		SessionFile = oldFile
		os.Remove(tmpFile)
	}()

	password := []byte("testpassword")
	userID := "alice"
	masterKey := []byte("masterkey123")
	accessToken := "access_token"
	refreshToken := "refresh_token"

	// Ключ для шифрования сессии такой же, как в Load
	sessionKey := crypto.DeriveKey(password, []byte("gophkeepston-session-salt"))
	err := Save(sessionKey, userID, masterKey, accessToken, refreshToken)
	if err != nil {
		t.Fatal(err)
	}

	// Загружаем с правильным паролем
	s, err := Load(password)
	if err != nil {
		t.Fatal(err)
	}
	if s.UserID != userID || string(s.MasterKey) != string(masterKey) ||
		s.AccessToken != accessToken || s.RefreshToken != refreshToken {
		t.Errorf("loaded session mismatch: %+v", s)
	}

	// Загружаем с неправильным паролем – должна быть ошибка
	_, err = Load([]byte("wrongpassword"))
	if err == nil {
		t.Error("expected error for wrong password")
	}
}
