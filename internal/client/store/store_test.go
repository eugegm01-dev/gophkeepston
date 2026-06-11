package store

import (
	"os"
	"testing"
)

func TestStorePutGetListDelete(t *testing.T) {
	tmpFile := "test_gophkeepston.db"
	defer os.Remove(tmpFile)

	s, err := NewStore(tmpFile)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer s.Close()

	user := "alice"
	// Put
	if err := s.Put(user, "entry1", []byte("secret1")); err != nil {
		t.Fatal(err)
	}
	// Get
	val, err := s.Get(user, "entry1")
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != "secret1" {
		t.Fatalf("got %q, want %q", val, "secret1")
	}
	// List
	ids, err := s.List(user)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "entry1" {
		t.Fatalf("unexpected list: %v", ids)
	}
	// Delete
	if err := s.Delete(user, "entry1"); err != nil {
		t.Fatal(err)
	}
	_, err = s.Get(user, "entry1")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestVersions(t *testing.T) {
	tmpFile := "test_versions.db"
	defer os.Remove(tmpFile)

	s, err := NewStore(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	user := "alice"
	id := "entry1"
	ver := int64(100)
	if err := s.PutVersion(user, id, ver); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetVersion(user, id)
	if err != nil {
		t.Fatal(err)
	}
	if got != ver {
		t.Errorf("expected %d, got %d", ver, got)
	}

	max, err := s.GetMaxVersion(user)
	if err != nil {
		t.Fatal(err)
	}
	if max != ver {
		t.Errorf("max version: expected %d, got %d", ver, max)
	}

	s.DeleteVersion(user, id)
	_, err = s.GetVersion(user, id)
	if err == nil {
		t.Error("expected error after delete")
	}
}
