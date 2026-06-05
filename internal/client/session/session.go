package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	authpb "github.com/eugegm01-dev/gophkeepston/api/proto/auth"
	"github.com/eugegm01-dev/gophkeepston/internal/client/authclient"
	"github.com/eugegm01-dev/gophkeepston/internal/client/crypto"
	"github.com/golang-jwt/jwt/v5"
)

const SessionFile = "session.enc"

type sessionData struct {
	UserID       string `json:"user_id"`
	MasterKey    []byte `json:"master_key"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type AuthClient interface {
	Refresh(ctx context.Context, refreshToken string) (*authpb.RefreshTokenResponse, error)
	Close() error
}

type Session struct {
	mu           sync.RWMutex
	UserID       string
	MasterKey    []byte
	AccessToken  string
	RefreshToken string
}

func Save(encKey []byte, userID string, masterKey []byte, accessToken, refreshToken string) error {
	data := sessionData{
		UserID:       userID,
		MasterKey:    masterKey,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}
	plain, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	encrypted, err := crypto.Encrypt(plain, encKey)
	if err != nil {
		return fmt.Errorf("encrypt session: %w", err)
	}
	return os.WriteFile(SessionFile, encrypted, 0600)
}

func Load(password []byte) (*Session, error) {
	key, err := crypto.DeriveKey(password, []byte("gophkeepston-session-salt"))
	if err != nil {
		return nil, fmt.Errorf("derive session key: %w", err)
	}
	encrypted, err := os.ReadFile(SessionFile)
	if err != nil {
		return nil, err
	}
	plain, err := crypto.Decrypt(encrypted, key)
	if err != nil {
		return nil, fmt.Errorf("decrypt session: %w", err)
	}
	var data sessionData
	if err := json.Unmarshal(plain, &data); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &Session{
		UserID:       data.UserID,
		MasterKey:    data.MasterKey,
		AccessToken:  data.AccessToken,
		RefreshToken: data.RefreshToken,
	}, nil
}

func (s *Session) EnsureFreshAccess(ctx context.Context, serverAddr string, sessionKey []byte) error {
	client, err := authclient.NewClient(serverAddr)
	if err != nil {
		return fmt.Errorf("create auth client: %w", err)
	}
	defer client.Close()
	return s.EnsureFreshAccessWithClient(ctx, client, sessionKey)
}

func (s *Session) EnsureFreshAccessWithClient(ctx context.Context, client AuthClient, sessionKey []byte) error {
	if !s.isTokenExpired() {
		return nil
	}
	resp, err := client.Refresh(ctx, s.RefreshToken)
	if err != nil {
		return fmt.Errorf("refresh tokens from server: %w", err)
	}
	if resp == nil {
		return fmt.Errorf("received nil response from refresh")
	}
	return s.updateAndPersist(resp.AccessToken, resp.RefreshToken, sessionKey)
}

func (s *Session) updateAndPersist(newAccess, newRefresh string, encKey []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AccessToken = newAccess
	s.RefreshToken = newRefresh

	data := sessionData{
		UserID:       s.UserID,
		MasterKey:    s.MasterKey,
		AccessToken:  s.AccessToken,
		RefreshToken: s.RefreshToken,
	}
	plain, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	encrypted, err := crypto.Encrypt(plain, encKey)
	if err != nil {
		return fmt.Errorf("encrypt session: %w", err)
	}
	tmpFile := SessionFile + ".tmp"
	if err := os.WriteFile(tmpFile, encrypted, 0600); err != nil {
		return fmt.Errorf("write temp session: %w", err)
	}
	if err := os.Rename(tmpFile, SessionFile); err != nil {
		return fmt.Errorf("commit session: %w", err)
	}
	return nil
}

func (s *Session) isTokenExpired() bool {
	s.mu.RLock()
	tokenStr := s.AccessToken
	s.mu.RUnlock()
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
