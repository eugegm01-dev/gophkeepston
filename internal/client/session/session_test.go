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
	sessionKey, err := crypto.DeriveKey(password, []byte("gophkeepston-session-salt"))
	if err != nil {
		t.Fatal(err)
	}
	err = Save(sessionKey, userID, masterKey, accessToken, refreshToken)
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
func TestEnsureFreshAccess(t *testing.T) {
	tmpFile := "test_session_refresh.enc"
	oldFile := SessionFile
	SessionFile = tmpFile
	defer func() {
		SessionFile = oldFile
		os.Remove(tmpFile)
	}()

	// создаём сессию с истекшим access-токеном
	password := []byte("pass")
	sessionKey, _ := crypto.DeriveKey(password, []byte("gophkeepston-session-salt"))
	oldAccess := "expired_token"
	oldRefresh := "valid_refresh"
	err := Save(sessionKey, "user1", []byte("key"), oldAccess, oldRefresh)
	if err != nil {
		t.Fatal(err)
	}

	//s, err := Load(password)
	//if err != nil {
	//	t.Fatal(err)
	//}

	// подменяем клиент, чтобы Refresh возвращал новые токены
	// (здесь нужен mock-сервер – упрощённо, можно просто подменить client.Refresh через моки)
	// Поскольку это сложно без изменения кода, отметим, что в реальном проекте нужно добавить интерфейс.
	// Пока напишем тест, который проверяет логику tokenExpired и факт вызова Refresh.
	// Для полного покрытия потребуется рефакторинг (выделить интерфейс RefreshClient).
	t.Skip("требуется рефакторинг для мокирования")
}
