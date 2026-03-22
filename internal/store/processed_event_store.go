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

	"github.com/bdobrica/RelayShell/internal/sessions"

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

func (s *ProcessedEventStore) UpsertSession(ctx context.Context, session *sessions.Session) error {
	if session == nil {
		return fmt.Errorf("session is required")
	}

	if strings.TrimSpace(session.ID) == "" {
		return fmt.Errorf("session id is required")
	}

	const statement = `
INSERT INTO sessions(
	session_id,
	repo,
	branch,
	agent,
	owner_user_id,
	governor_room_id,
	room_id,
	workspace_dir,
	detected_stack,
	runtime_image,
	state,
	created_at_utc,
	updated_at_utc
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(session_id) DO UPDATE SET
	repo = excluded.repo,
	branch = excluded.branch,
	agent = excluded.agent,
	owner_user_id = excluded.owner_user_id,
	governor_room_id = excluded.governor_room_id,
	room_id = excluded.room_id,
	workspace_dir = excluded.workspace_dir,
	detected_stack = excluded.detected_stack,
	runtime_image = excluded.runtime_image,
	state = excluded.state,
	created_at_utc = excluded.created_at_utc,
	updated_at_utc = excluded.updated_at_utc
`

	now := time.Now().UTC().Format(time.RFC3339Nano)
	createdAt := session.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	if _, err := s.db.ExecContext(
		ctx,
		statement,
		session.ID,
		session.Repo,
		session.Branch,
		session.Agent,
		session.OwnerUserID,
		session.GovernorRoomID,
		session.RoomID,
		session.WorkspaceDir,
		session.DetectedStack,
		session.RuntimeImage,
		string(session.State),
		createdAt.Format(time.RFC3339Nano),
		now,
	); err != nil {
		return fmt.Errorf("upsert session %q: %w", session.ID, err)
	}

	return nil
}

func (s *ProcessedEventStore) DeleteSession(ctx context.Context, sessionID string) error {
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" {
		return nil
	}

	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE session_id = ?`, trimmed); err != nil {
		return fmt.Errorf("delete session %q: %w", trimmed, err)
	}

	return nil
}

func (s *ProcessedEventStore) ListSessions(ctx context.Context) ([]*sessions.Session, error) {
	const query = `
SELECT
	session_id,
	repo,
	branch,
	agent,
	owner_user_id,
	governor_room_id,
	room_id,
	workspace_dir,
	detected_stack,
	runtime_image,
	state,
	created_at_utc
FROM sessions
ORDER BY created_at_utc ASC
`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	items := make([]*sessions.Session, 0)
	for rows.Next() {
		var (
			sessionID      string
			repo           string
			branch         string
			agent          string
			ownerUserID    string
			governorRoomID string
			roomID         string
			workspaceDir   string
			detectedStack  string
			runtimeImage   string
			state          string
			createdAtRaw   string
		)

		if err := rows.Scan(
			&sessionID,
			&repo,
			&branch,
			&agent,
			&ownerUserID,
			&governorRoomID,
			&roomID,
			&workspaceDir,
			&detectedStack,
			&runtimeImage,
			&state,
			&createdAtRaw,
		); err != nil {
			return nil, fmt.Errorf("scan session row: %w", err)
		}

		createdAt, err := time.Parse(time.RFC3339Nano, createdAtRaw)
		if err != nil {
			return nil, fmt.Errorf("parse session created_at for %q: %w", sessionID, err)
		}

		items = append(items, &sessions.Session{
			ID:             sessionID,
			Repo:           repo,
			Branch:         branch,
			Agent:          agent,
			OwnerUserID:    ownerUserID,
			GovernorRoomID: governorRoomID,
			RoomID:         roomID,
			WorkspaceDir:   workspaceDir,
			DetectedStack:  detectedStack,
			RuntimeImage:   runtimeImage,
			State:          sessions.State(state),
			CreatedAt:      createdAt,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions rows: %w", err)
	}

	return items, nil
}

func (s *ProcessedEventStore) Close() error {
	return s.db.Close()
}
