// Package crypto provides encryption and key derivation functions.
// It uses AES-256-GCM for symmetric encryption and Argon2id for key derivation.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"

	"golang.org/x/crypto/argon2"
)

// DeriveKey derives a 32-byte encryption key from a password and salt using Argon2id.
func DeriveKey(password, salt []byte) []byte {
	// 1 проход, 64 МБ памяти, 4 потока – достаточно для клиента
	return argon2.IDKey(password, salt, 1, 64*1024, 4, 32)
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
