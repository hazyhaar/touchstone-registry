// CLAUDE:SUMMARY Gob serialization and deserialization of dictionary entry hashmaps for fast loading.
package dict

import (
	"encoding/gob"
	"fmt"
	"os"
)

// loadGob deserializes entries from a gob-encoded file into d.Entries.
func (d *Dictionary) loadGob(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open gob file: %w", err)
	}
	defer f.Close()

	if err := gob.NewDecoder(f).Decode(&d.Entries); err != nil {
		return fmt.Errorf("decode gob: %w", err)
	}
	return nil
}

// SaveGob serializes entries to a gob-encoded file at path.
func SaveGob(entries map[string]*Entry, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create gob file: %w", err)
	}
	defer f.Close()

	if err := gob.NewEncoder(f).Encode(entries); err != nil {
		return fmt.Errorf("encode gob: %w", err)
	}
	return nil
}
