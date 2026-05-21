// Package store implements a local encrypted storage using BoltDB.
// It stores user entries (passwords, texts, cards, binaries) in encrypted form.
package store

import (
	"fmt"
	"strings"

	"go.etcd.io/bbolt"
)

// Store is a local BoltDB-backed store for encrypted entries.
type Store struct {
	db *bbolt.DB
}

// NewStore opens or creates a BoltDB file at the given path and returns a Store.
func NewStore(path string) (*Store, error) {
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("open bolt: %w", err)
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("entries"))
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("create bucket: %w", err)
	}
	return &Store{db: db}, nil
}

// Put saves an encrypted entry under the user's namespace.
func (s *Store) Put(userID, entryID string, encryptedData []byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("entries"))
		return b.Put([]byte(userID+":"+entryID), encryptedData)
	})
}

// Get retrieves an encrypted entry by user and entry ID.
func (s *Store) Get(userID, entryID string) ([]byte, error) {
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
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("entries"))
		return b.Delete([]byte(userID + ":" + entryID))
	})
}

// Close closes the underlying BoltDB database.
func (s *Store) Close() error {
	return s.db.Close()
}
