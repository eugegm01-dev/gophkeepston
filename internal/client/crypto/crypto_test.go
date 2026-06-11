package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key := DeriveKey([]byte("password"), []byte("salt"))
	plain := []byte("Hello, GophKeeper!")
	ct, err := Encrypt(plain, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	got, err := Decrypt(ct, key)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(plain, got) {
		t.Fatalf("mismatch: got %q want %q", got, plain)
	}
}

func TestDecryptCorrupted(t *testing.T) {
	key := DeriveKey([]byte("pass"), []byte("salt"))
	_, err := Decrypt([]byte("short"), key)
	if err == nil {
		t.Fatal("expected error on short ciphertext")
	}
}
