package storage

import "database/sql"

// Storage is the PostgreSQL-backed store used by the service.
type Storage struct {
	db *sql.DB
}

// New returns a Storage that uses db.
func New(db *sql.DB) *Storage {
	return &Storage{db: db}
}
