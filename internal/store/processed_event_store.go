package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type ProcessedEventStore struct {
	db *sql.DB
}

type sqlMigration struct {
	Version int
	Name    string
	SQL     string
}

//go:embed migrations/*.sql
var migrationFiles embed.FS

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
	if err := store.applyMigrations(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *ProcessedEventStore) applyMigrations(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	applied_at_utc TEXT NOT NULL
);
`); err != nil {
		return fmt.Errorf("initialize schema_migrations: %w", err)
	}

	applied := map[int]struct{}{}
	rows, err := s.db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("scan schema_migrations row: %w", err)
		}
		applied[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate schema_migrations rows: %w", err)
	}

	migrations, err := loadSQLMigrations()
	if err != nil {
		return err
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })

	for _, migration := range migrations {
		if _, ok := applied[migration.Version]; ok {
			continue
		}

		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration transaction %d: %w", migration.Version, err)
		}

		if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d (%s): %w", migration.Version, migration.Name, err)
		}

		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO schema_migrations(version, name, applied_at_utc) VALUES (?, ?, ?)`,
			migration.Version,
			migration.Name,
			time.Now().UTC().Format(time.RFC3339Nano),
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d (%s): %w", migration.Version, migration.Name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d (%s): %w", migration.Version, migration.Name, err)
		}
	}

	return nil
}

func loadSQLMigrations() ([]sqlMigration, error) {
	entries, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return nil, fmt.Errorf("list migration files: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no SQL migration files found in migrations/*.sql")
	}

	migrations := make([]sqlMigration, 0, len(entries))
	seenVersions := map[int]string{}

	for _, entry := range entries {
		baseName := filepath.Base(entry)
		if !strings.HasSuffix(baseName, ".sql") {
			continue
		}

		trimmed := strings.TrimSuffix(baseName, ".sql")
		parts := strings.SplitN(trimmed, "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid migration filename %q: expected <version>_<name>.sql", baseName)
		}

		version, err := strconv.Atoi(parts[0])
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("invalid migration version in %q", baseName)
		}

		name := strings.TrimSpace(parts[1])
		if name == "" {
			return nil, fmt.Errorf("missing migration name in %q", baseName)
		}

		if prev, exists := seenVersions[version]; exists {
			return nil, fmt.Errorf("duplicate migration version %d in %q and %q", version, prev, baseName)
		}
		seenVersions[version] = baseName

		raw, err := migrationFiles.ReadFile(entry)
		if err != nil {
			return nil, fmt.Errorf("read migration file %q: %w", baseName, err)
		}

		migrations = append(migrations, sqlMigration{
			Version: version,
			Name:    name,
			SQL:     string(raw),
		})
	}

	return migrations, nil
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
