package session

import (
	"encoding/json"
	"os"

	"github.com/eugegm01-dev/gophkeepston/internal/client/crypto"
)

type Session struct {
	UserID    string `json:"user_id"`
	MasterKey []byte `json:"master_key"`
}

func Save(encKey []byte, userID string, key []byte) error {
	s := Session{UserID: userID, MasterKey: key}
	plain, err := json.Marshal(s)
	if err != nil {
		return err
	}
	ct, err := crypto.Encrypt(plain, encKey)
	if err != nil {
		return err
	}
	return os.WriteFile("session.enc", ct, 0600)
}

func Load(password []byte) (*Session, error) {
	ct, err := os.ReadFile("session.enc")
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
