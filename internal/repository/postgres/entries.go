package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Entry представляет модель записи в БД
type Entry struct {
	EntryID       string
	EncryptedData []byte
	Version       int64
}

type EntryRepository struct {
	db *pgxpool.Pool
}

func NewEntryRepository(db *pgxpool.Pool) *EntryRepository {
	return &EntryRepository{db: db}
}

// Pull получает все записи пользователя, которые новее указанной версии
func (r *EntryRepository) Pull(ctx context.Context, userID string, sinceVersion int64) ([]Entry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT entry_id, encrypted_data, version 
		FROM entries 
		WHERE user_id = $1 AND version > $2`, userID, sinceVersion)
	if err != nil {
		return nil, fmt.Errorf("query pull: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.EntryID, &e.EncryptedData, &e.Version); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
			if err := rows.Err(); err != nil {
				return nil, status.Errorf(codes.Internal, "rows iteration error: %v", err)
			}
		}
		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return entries, nil
}

// Push выполняет UPSERT (создание или обновление записи)
func (r *EntryRepository) Push(ctx context.Context, userID string, e Entry) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO entries (user_id, entry_id, encrypted_data, version)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, entry_id) 
		DO UPDATE SET 
			encrypted_data = EXCLUDED.encrypted_data,
			version = EXCLUDED.version,
			updated_at = now()`,
		userID, e.EntryID, e.EncryptedData, e.Version)

	if err != nil {
		return fmt.Errorf("upsert entry: %w", err)
	}
	return nil
}

func (r *EntryRepository) Delete(ctx context.Context, userID string, entryID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE entries 
		SET version = -1, updated_at = now() 
		WHERE user_id = $1 AND entry_id = $2`,
		userID, entryID)

	if err != nil {
		return fmt.Errorf("mark deleted: %w", err)
	}
	return nil
}
