package importer

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Source represents a row from the import_sources table.
type Source struct {
	AdapterID   string
	DictID      string
	Description string
	SourceURL   string
	License     string
	LastCheck   *int64
	LastStatus  *int
	LastError   *string
	UpdatedAt   int64
}

// SourceDB manages the import_sources SQLite table.
type SourceDB struct {
	db *sql.DB
}

// OpenSourceDB opens (or creates) the SQLite database at path and ensures the
// import_sources table exists.
func OpenSourceDB(path string) (*SourceDB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open source db: %w", err)
	}

	const ddl = `CREATE TABLE IF NOT EXISTS import_sources (
		adapter_id   TEXT PRIMARY KEY,
		dict_id      TEXT NOT NULL,
		description  TEXT NOT NULL,
		source_url   TEXT NOT NULL,
		license      TEXT NOT NULL DEFAULT '',
		last_check   INTEGER,
		last_status  INTEGER,
		last_error   TEXT,
		updated_at   INTEGER NOT NULL
	)`
	if _, err := db.Exec(ddl); err != nil {
		db.Close()
		return nil, fmt.Errorf("create import_sources table: %w", err)
	}

	return &SourceDB{db: db}, nil
}

// Close ferme la connexion SQLite.
func (s *SourceDB) Close() error {
	return s.db.Close()
}

// Seed inserts default rows for each adapter (INSERT OR IGNORE — existing rows
// are left untouched so that manual URL overrides survive restarts).
func (s *SourceDB) Seed(adapters []Adapter) error {
	const q = `INSERT OR IGNORE INTO import_sources
		(adapter_id, dict_id, description, source_url, license, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`

	now := time.Now().Unix()
	for _, a := range adapters {
		if _, err := s.db.Exec(q, a.ID(), a.DictID(), a.Description(), a.DefaultURL(), a.License(), now); err != nil {
			return fmt.Errorf("seed %s: %w", a.ID(), err)
		}
	}
	return nil
}

// GetURL returns the current source URL for a given adapter ID.
func (s *SourceDB) GetURL(adapterID string) (string, error) {
	var url string
	err := s.db.QueryRow(`SELECT source_url FROM import_sources WHERE adapter_id = ?`, adapterID).Scan(&url)
	if err != nil {
		return "", fmt.Errorf("get url for %s: %w", adapterID, err)
	}
	return url, nil
}

// SetURL updates the source URL for a given adapter and records the change timestamp.
func (s *SourceDB) SetURL(adapterID, url string) error {
	res, err := s.db.Exec(
		`UPDATE import_sources SET source_url = ?, updated_at = ? WHERE adapter_id = ?`,
		url, time.Now().Unix(), adapterID,
	)
	if err != nil {
		return fmt.Errorf("set url for %s: %w", adapterID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("adapter %s not found in import_sources", adapterID)
	}
	return nil
}

// UpdateCheck persists the result of an availability check.
func (s *SourceDB) UpdateCheck(adapterID string, status int, checkErr string) error {
	now := time.Now().Unix()
	var errPtr *string
	if checkErr != "" {
		errPtr = &checkErr
	}
	_, err := s.db.Exec(
		`UPDATE import_sources SET last_check = ?, last_status = ?, last_error = ? WHERE adapter_id = ?`,
		now, status, errPtr, adapterID,
	)
	if err != nil {
		return fmt.Errorf("update check for %s: %w", adapterID, err)
	}
	return nil
}

// ListSources returns all rows from import_sources ordered by adapter_id.
func (s *SourceDB) ListSources() ([]Source, error) {
	rows, err := s.db.Query(`SELECT adapter_id, dict_id, description, source_url, license,
		last_check, last_status, last_error, updated_at
		FROM import_sources ORDER BY adapter_id`)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()

	var sources []Source
	for rows.Next() {
		var src Source
		if err := rows.Scan(&src.AdapterID, &src.DictID, &src.Description, &src.SourceURL,
			&src.License, &src.LastCheck, &src.LastStatus, &src.LastError, &src.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan source: %w", err)
		}
		sources = append(sources, src)
	}
	return sources, rows.Err()
}
