package sync

import (
	"context"
	"time"

	syncpb "github.com/eugegm01-dev/gophkeepston/api/proto/sync"
	"github.com/eugegm01-dev/gophkeepston/internal/server/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SyncService struct {
	syncpb.UnimplementedSyncServer
	db *pgxpool.Pool
}

func NewSyncService(db *pgxpool.Pool) *SyncService {
	return &SyncService{db: db}
}

func (s *SyncService) Push(ctx context.Context, req *syncpb.PushRequest) (*syncpb.PushResponse, error) {
	userID, ok := ctx.Value(middleware.UserIDKey).(string)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user_id not found")
	}

	for _, entry := range req.Entries {
		// Используем upsert: обновление только если версия выше
		_, err := s.db.Exec(ctx, `
            INSERT INTO entries (user_id, entry_id, encrypted_data, encrypted_meta, version, updated_at)
            VALUES ($1, $2, $3, $4, $5, $6)
            ON CONFLICT (user_id, entry_id) DO UPDATE SET
                encrypted_data = EXCLUDED.encrypted_data,
                encrypted_meta = EXCLUDED.encrypted_meta,
                version = EXCLUDED.version,
                updated_at = EXCLUDED.updated_at
            WHERE entries.version < EXCLUDED.version
        `, userID, entry.Id, entry.EncryptedData, entry.EncryptedMeta, entry.Version, time.Now())

		if err != nil {
			return nil, status.Errorf(codes.Internal, "push entry %s: %v", entry.Id, err)
		}
	}
	return &syncpb.PushResponse{}, nil
}

func (s *SyncService) Pull(ctx context.Context, req *syncpb.PullRequest) (*syncpb.PullResponse, error) {
	userID, ok := ctx.Value(middleware.UserIDKey).(string)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user_id not found")
	}

	rows, err := s.db.Query(ctx, `
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

	if err := rows.Err(); err != nil {
		return nil, status.Errorf(codes.Internal, "iteration error: %v", err)
	}

	return &syncpb.PullResponse{Entries: entries}, nil
}

func (s *SyncService) Delete(ctx context.Context, req *syncpb.DeleteRequest) (*syncpb.DeleteResponse, error) {
	userID, ok := ctx.Value(middleware.UserIDKey).(string)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user_id not found")
	}

	result, err := s.db.Exec(ctx,
		`DELETE FROM entries WHERE user_id = $1 AND entry_id = $2`,
		userID, req.EntryId,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "delete entry: %v", err)
	}

	if result.RowsAffected() == 0 {
		return &syncpb.DeleteResponse{}, nil
	}

	return &syncpb.DeleteResponse{}, nil
}
