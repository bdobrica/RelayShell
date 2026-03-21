package store

// Package store contains in-memory and SQLite-backed persistence primitives.
// SQLite stores in this package use versioned schema migrations applied at
// startup. Migration files are stored as SQL under internal/store/migrations.
