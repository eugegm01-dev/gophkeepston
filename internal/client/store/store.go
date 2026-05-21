// Package store implements a local encrypted storage using BoltDB.
// It stores user entries (passwords, texts, cards, binaries) in encrypted form.
package store

import (
	"encoding/binary"
	"fmt"
	"strings"
	"sync"

	"go.etcd.io/bbolt"
)

// Store is a local BoltDB-backed store for encrypted entries.
type Store struct {
	mu sync.Mutex
	db *bbolt.DB
}

// NewStore opens or creates a BoltDB file at the given path and returns a Store.
func NewStore(path string) (*Store, error) {
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("open bolt: %w", err)
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte("entries")); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("versions")); err != nil {
			return err
		}
		return nil
	})
	return &Store{db: db}, nil
}

// Put saves an encrypted entry under the user's namespace.
func (s *Store) Put(userID, entryID string, encryptedData []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("entries"))
		return b.Put([]byte(userID+":"+entryID), encryptedData)
	})
}

// Get retrieves an encrypted entry by user and entry ID.
func (s *Store) Get(userID, entryID string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var data []byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("entries"))
		v := b.Get([]byte(userID + ":" + entryID))
		if v == nil {
			return fmt.Errorf("entry not found")
		}
		data = make([]byte, len(v))
		copy(data, v)
		return nil
	})
	return data, err
}

// List returns all entry IDs for a given user.
func (s *Store) List(userID string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	prefix := userID + ":"
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("entries"))
		c := b.Cursor()
		for k, _ := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, _ = c.Next() {
			id := strings.TrimPrefix(string(k), prefix)
			ids = append(ids, id)
		}
		return nil
	})
	return ids, err
}

// Delete removes an entry from the store.
func (s *Store) Delete(userID, entryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("entries"))
		return b.Delete([]byte(userID + ":" + entryID))
	})
}

// Close closes the underlying BoltDB database.
func (s *Store) Close() error {
	return s.db.Close()
}

// Версионность
func (s *Store) PutVersion(userID, entryID string, version int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("versions"))
		return b.Put([]byte(userID+":"+entryID), int64ToBytes(version))
	})
}

func (s *Store) GetVersion(userID, entryID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ver int64
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("versions"))
		if b == nil {
			return fmt.Errorf("versions bucket not found")
		}
		data := b.Get([]byte(userID + ":" + entryID))
		if data == nil {
			return fmt.Errorf("version not found")
		}
		ver = bytesToInt64(data)
		return nil
	})
	return ver, err
}

func (s *Store) DeleteVersion(userID, entryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("versions"))
		if b == nil {
			return nil
		}
		return b.Delete([]byte(userID + ":" + entryID))
	})
}

func (s *Store) GetMaxVersion(userID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var max int64
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("versions"))
		if b == nil {
			return nil // нет бакета – максимум 0
		}
		c := b.Cursor()
		prefix := []byte(userID + ":")
		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = c.Next() {
			ver := bytesToInt64(v)
			if ver > max {
				max = ver
			}
		}
		return nil
	})
	return max, err
}

// Вспомогательные функции для преобразования int64 ↔ []byte
func int64ToBytes(i int64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(i))
	return buf
}

func bytesToInt64(b []byte) int64 {
	return int64(binary.BigEndian.Uint64(b))
}
