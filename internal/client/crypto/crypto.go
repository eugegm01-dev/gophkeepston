// Package crypto provides encryption and key derivation functions.
// It uses AES-256-GCM for symmetric encryption and Argon2id for key derivation.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"golang.org/x/crypto/scrypt"
)

// DeriveKey derives a 32-byte encryption key from a password and salt using Argon2id.
// ВАЖНО: в production соль должна быть случайной для каждого пользователя и храниться на сервере.
func DeriveKey(password, salt []byte) ([]byte, error) {
	if len(password) < 8 {
		return nil, fmt.Errorf("password too short: min 8 bytes")
	}
	key, err := scrypt.Key(password, salt, 32768, 8, 1, 32)
	if err != nil {
		return nil, fmt.Errorf("scrypt: %w", err)
	}
	return key, nil
}

// Encrypt encrypts plaintext with the given key using AES-256-GCM.
// It returns the nonce prepended to the ciphertext
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

// Decrypt decrypts ciphertext with the given key using AES-256-GCM.
// The ciphertext must start with the nonce.
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
