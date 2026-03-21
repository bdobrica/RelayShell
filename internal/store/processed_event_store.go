package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type ProcessedEventStore struct {
	db *sql.DB
}

func NewProcessedEventStore(ctx context.Context, dbPath string) (*ProcessedEventStore, error) {
	dsn := fmt.Sprintf("file:%s", filepath.Clean(dbPath))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite db: %w", err)
	}

	store := &ProcessedEventStore{db: db}
	if err := store.initSchema(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *ProcessedEventStore) initSchema(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS processed_events (
	event_id TEXT PRIMARY KEY,
	room_id TEXT NOT NULL,
	processed_at_utc TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_processed_events_room_id ON processed_events(room_id);
`

	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("initialize processed events schema: %w", err)
	}

	return nil
}

func (s *ProcessedEventStore) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	const query = `SELECT 1 FROM processed_events WHERE event_id = ? LIMIT 1`

	var one int
	err := s.db.QueryRowContext(ctx, query, eventID).Scan(&one)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}

	return false, fmt.Errorf("check processed event %q: %w", eventID, err)
}

func (s *ProcessedEventStore) MarkProcessed(ctx context.Context, eventID, roomID string) error {
	const statement = `
INSERT INTO processed_events(event_id, room_id, processed_at_utc)
VALUES (?, ?, ?)
ON CONFLICT(event_id) DO NOTHING
`

	processedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.ExecContext(ctx, statement, eventID, roomID, processedAt); err != nil {
		return fmt.Errorf("mark processed event %q: %w", eventID, err)
	}

	return nil
}

func (s *ProcessedEventStore) DeleteProcessedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	const statement = `DELETE FROM processed_events WHERE processed_at_utc < ?`

	result, err := s.db.ExecContext(ctx, statement, cutoff.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, fmt.Errorf("delete processed events before %s: %w", cutoff.UTC().Format(time.RFC3339Nano), err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read deleted rows count: %w", err)
	}

	return rows, nil
}

func (s *ProcessedEventStore) Close() error {
	return s.db.Close()
}
