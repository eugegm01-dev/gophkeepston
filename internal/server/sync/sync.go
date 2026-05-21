// Package sync implements the gRPC Sync service (Push/Pull).
// It stores encrypted entries in PostgreSQL and uses Lamport versions for conflict resolution.
package sync

import (
	"context"
	"database/sql"
	"time"

	syncpb "github.com/eugegm01-dev/gophkeepston/api/proto/sync"
	"github.com/eugegm01-dev/gophkeepston/internal/server/middleware"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SyncService implements the Sync gRPC server.
type SyncService struct {
	syncpb.UnimplementedSyncServer
	db *sql.DB
}

// NewSyncService creates a SyncService with a database connection.
func NewSyncService(db *sql.DB) *SyncService {
	return &SyncService{db: db}
}

// Push inserts or updates encrypted entries.
func (s *SyncService) Push(ctx context.Context, req *syncpb.PushRequest) (*syncpb.PushResponse, error) {
	userID, ok := ctx.Value(middleware.UserIDKey).(string)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user_id not found")
	}

	for _, entry := range req.Entries {
		// Обновляем или вставляем запись
		_, err := s.db.Exec(`
            INSERT INTO entries (id, user_id, entry_id, encrypted_data, encrypted_meta, version, updated_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7)
            ON CONFLICT (user_id, entry_id) DO UPDATE SET
                encrypted_data = EXCLUDED.encrypted_data,
                encrypted_meta = EXCLUDED.encrypted_meta,
                version = EXCLUDED.version,
                updated_at = EXCLUDED.updated_at
            WHERE entries.version < EXCLUDED.version
        `, uuid.New().String(), userID, entry.Id, entry.EncryptedData, entry.EncryptedMeta, entry.Version, time.Now())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "push entry: %v", err)
		}
	}
	return &syncpb.PushResponse{}, nil
}

// Pull returns entries newer than the given version.
func (s *SyncService) Pull(ctx context.Context, req *syncpb.PullRequest) (*syncpb.PullResponse, error) {
	userID, ok := ctx.Value(middleware.UserIDKey).(string)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user_id not found")
	}

	rows, err := s.db.Query(`
        SELECT entry_id, encrypted_data, encrypted_meta, version, updated_at
        FROM entries
        WHERE user_id = $1 AND version > $2
        ORDER BY version ASC
    `, userID, req.SinceVersion)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "pull entries: %v", err)
	}
	defer rows.Close()

	var entries []*syncpb.Entry
	for rows.Next() {
		var e syncpb.Entry
		var updatedAt time.Time
		if err := rows.Scan(&e.Id, &e.EncryptedData, &e.EncryptedMeta, &e.Version, &updatedAt); err != nil {
			return nil, status.Errorf(codes.Internal, "scan entry: %v", err)
		}
		e.UpdatedAt = updatedAt.Unix()
		entries = append(entries, &e)
	}

	return &syncpb.PullResponse{Entries: entries}, nil
}
