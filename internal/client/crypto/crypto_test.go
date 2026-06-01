package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key, err := DeriveKey([]byte("password"), []byte("salt"))
	if err != nil {
		t.Fatal(err)
	}
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
	key, err := DeriveKey([]byte("password"), []byte("salt"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Decrypt([]byte("short"), key)
	if err == nil {
		t.Fatal("expected error on short ciphertext")
	}
}
