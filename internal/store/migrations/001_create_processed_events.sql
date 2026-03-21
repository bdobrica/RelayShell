CREATE TABLE IF NOT EXISTS processed_events (
    event_id TEXT PRIMARY KEY,
    room_id TEXT NOT NULL,
    processed_at_utc TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_processed_events_room_id ON processed_events(room_id);
