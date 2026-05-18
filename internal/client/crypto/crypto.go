package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"

	"golang.org/x/crypto/argon2"
)

func DeriveKey(password, salt []byte) []byte {
	// 1 проход, 64 МБ памяти, 4 потока – достаточно для клиента
	return argon2.IDKey(password, salt, 1, 64*1024, 4, 32)
}

func Encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func Decrypt(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	n := gcm.NonceSize()
	if len(ciphertext) < n {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := ciphertext[:n], ciphertext[n:]
	return gcm.Open(nil, nonce, ct, nil)
}
