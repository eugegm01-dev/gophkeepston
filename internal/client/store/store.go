package store

import (
	"fmt"

	"go.etcd.io/bbolt"
)

type Store struct {
	db *bbolt.DB
}

func NewStore(path string) (*Store, error) {
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("open bolt: %w", err)
	}
	// Создаём бакет для записей, если его нет
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("entries"))
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("create bucket: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Put(userID, entryID string, encryptedData []byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("entries"))
		return b.Put([]byte(userID+":"+entryID), encryptedData)
	})
}

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

func (s *Store) Close() error {
	return s.db.Close()
}
