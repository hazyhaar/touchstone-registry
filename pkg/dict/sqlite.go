// CLAUDE:SUMMARY SQLite serialization and lookup of dictionary entries — replaces in-memory gob for scalable disk-backed dicts.
// CLAUDE:DEPENDS pkg/dict/dict.go
// CLAUDE:EXPORTS SaveSQLite
package dict

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
)

// SaveSQLite writes entries to a SQLite database at path.
// The table schema is: terms(key TEXT PRIMARY KEY, metadata TEXT) WITHOUT ROWID.
func SaveSQLite(entries map[string]*Entry, path string) error {
	_ = os.Remove(path) // start fresh

	db, err := sql.Open("sqlite", path+"?_txlock=immediate&_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE terms (key TEXT PRIMARY KEY, metadata TEXT) WITHOUT ROWID`); err != nil {
		return fmt.Errorf("create table: %w", err)
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO terms (key, metadata) VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	var n int
	for key, entry := range entries {
		var metaJSON []byte
		if entry != nil && len(entry.Metadata) > 0 {
			metaJSON, err = json.Marshal(entry.Metadata)
			if err != nil {
				return fmt.Errorf("marshal metadata for %q: %w", key, err)
			}
		}
		if _, err := stmt.Exec(key, string(metaJSON)); err != nil {
			return fmt.Errorf("insert %q: %w", key, err)
		}
		n++
		if n%50000 == 0 {
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("commit batch: %w", err)
			}
			tx, err = db.BeginTx(context.Background(), nil)
			if err != nil {
				return fmt.Errorf("begin batch: %w", err)
			}
			stmt, err = tx.Prepare(`INSERT OR REPLACE INTO terms (key, metadata) VALUES (?, ?)`)
			if err != nil {
				return fmt.Errorf("prepare batch: %w", err)
			}
		}
	}

	return tx.Commit()
}

// loadSQLite opens a SQLite dict in read-only mode and caches the entry count.
func (d *Dictionary) loadSQLite(path string) error {
	db, err := sql.Open("sqlite", path+"?mode=ro&_txlock=immediate&_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM terms`).Scan(&count); err != nil {
		db.Close()
		return fmt.Errorf("count terms: %w", err)
	}

	d.db = db
	d.entryCount = count
	return nil
}

// lookupSQLite performs a single-key lookup against the SQLite database.
func (d *Dictionary) lookupSQLite(key string) (*Entry, bool) {
	var metadata sql.NullString
	err := d.db.QueryRow(`SELECT metadata FROM terms WHERE key = ?`, key).Scan(&metadata)
	if err != nil {
		return nil, false
	}

	entry := &Entry{}
	if metadata.Valid && metadata.String != "" {
		if err := json.Unmarshal([]byte(metadata.String), &entry.Metadata); err != nil {
			return nil, false
		}
	}
	return entry, true
}
