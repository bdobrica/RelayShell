CREATE TABLE IF NOT EXISTS sessions (
    session_id TEXT PRIMARY KEY,
    repo TEXT NOT NULL,
    branch TEXT NOT NULL,
    agent TEXT NOT NULL,
    owner_user_id TEXT NOT NULL,
    governor_room_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    workspace_dir TEXT NOT NULL,
    detected_stack TEXT NOT NULL,
    runtime_image TEXT NOT NULL,
    state TEXT NOT NULL,
    created_at_utc TEXT NOT NULL,
    updated_at_utc TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_room_id ON sessions(room_id);
CREATE INDEX IF NOT EXISTS idx_sessions_state ON sessions(state);
