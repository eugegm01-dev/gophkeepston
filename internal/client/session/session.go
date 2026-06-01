// Package session manages encrypted session files.
// It saves and loads master key, access/refresh tokens, and provides token refresh.
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/eugegm01-dev/gophkeepston/internal/client/authclient"
	"github.com/eugegm01-dev/gophkeepston/internal/client/crypto"
	"github.com/golang-jwt/jwt/v5"
)

var SessionFile = "session.enc"

const sessionFile = "session.enc"

// Session holds the user ID, master key, and tokens.
type Session struct {
	UserID       string `json:"user_id"`
	MasterKey    []byte `json:"master_key"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Save encrypts the session and writes it to SessionFile.
func Save(encKey []byte, userID string, key []byte, accessToken, refreshToken string) error {
	s := Session{
		UserID:       userID,
		MasterKey:    key,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}
	plain, err := json.Marshal(s)
	if err != nil {
		return err
	}
	ct, err := crypto.Encrypt(plain, encKey)
	if err != nil {
		return err
	}
	return os.WriteFile(SessionFile, ct, 0600)
}

// Load reads and decrypts the session from SessionFile.
func Load(password []byte) (*Session, error) {
	ct, err := os.ReadFile(SessionFile)
	if err != nil {
		return nil, err
	}
	key, err := crypto.DeriveKey(password, []byte("gophkeepston-session-salt"))
	if err != nil {
		return nil, err
	}
	plain, err := crypto.Decrypt(ct, key)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(plain, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// EnsureFreshAccess проверяет, не истёк ли access-токен, и обновляет его через сервер при необходимости.
// internal/client/session/session.go
func (s *Session) EnsureFreshAccess(ctx context.Context, serverAddr string, sessionKey []byte) error {
	if !tokenExpired(s.AccessToken) {
		return nil
	}

	client, err := authclient.NewClient(serverAddr)
	if err != nil {
		return err
	}
	defer client.Close()

	resp, err := client.Refresh(ctx, s.RefreshToken)
	if err != nil {
		return fmt.Errorf("refresh token: %w", err)
	}

	// Атомарное обновление: сначала сохраняем на диск, потом в память
	// (требование безопасности: не терять токены)
	sessionData := struct {
		AccessToken  string `json:"access"`
		RefreshToken string `json:"refresh"`
	}{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	}
	plain, err := json.Marshal(sessionData)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	encrypted, err := crypto.Encrypt(plain, sessionKey) // ← используем существующую Encrypt
	if err != nil {
		return fmt.Errorf("encrypt session: %w", err)
	}

	if err := os.WriteFile(sessionFile, encrypted, 0600); err != nil {
		return fmt.Errorf("write session: %w", err)
	}

	// Только после успешного сохранения обновляем в памяти
	s.AccessToken = resp.AccessToken
	s.RefreshToken = resp.RefreshToken

	return nil
}

func tokenExpired(tokenStr string) bool {
	if tokenStr == "" {
		return true
	}
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	claims := &jwt.MapClaims{}
	_, _, err := parser.ParseUnverified(tokenStr, claims)
	if err != nil {
		return true
	}
	exp, ok := (*claims)["exp"].(float64)
	if !ok {
		return true
	}
	return time.Now().After(time.Unix(int64(exp), 0))
}
