package session

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/eugegm01-dev/gophkeepston/internal/client/authclient"
	"github.com/eugegm01-dev/gophkeepston/internal/client/crypto"
	"github.com/golang-jwt/jwt/v5"
)

var SessionFile = "session.enc"

type Session struct {
	UserID       string `json:"user_id"`
	MasterKey    []byte `json:"master_key"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

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

func Load(password []byte) (*Session, error) {
	ct, err := os.ReadFile(SessionFile)
	if err != nil {
		return nil, err
	}
	key := crypto.DeriveKey(password, []byte("gophkeepston-session-salt"))
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
func (s *Session) EnsureFreshAccess(serverAddr string) error {
	if !tokenExpired(s.AccessToken) {
		return nil
	}
	client, err := authclient.NewClient(serverAddr)
	if err != nil {
		return err
	}
	defer client.Close()

	resp, err := client.Refresh(context.Background(), s.RefreshToken)
	if err != nil {
		return err
	}
	s.AccessToken = resp.AccessToken
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
