// CLAUDE:SUMMARY Synchronization of on-disk dictionaries and legacy import_sources into the admin database.
// CLAUDE:DEPENDS pkg/admin/service.go, pkg/dict/registry.go, pkg/importer/sourcedb.go
// CLAUDE:EXPORTS SyncFromRegistry, MigrateFromSourceDB

package admin

import (
	"encoding/json"
	"time"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
	"github.com/hazyhaar/touchstone-registry/pkg/importer"
)

// SyncFromRegistry ensures all loaded dictionaries from the registry have a matching
// dict_registry record in the admin DB. Existing records are left untouched.
func (s *Service) SyncFromRegistry(reg *dict.Registry) error {
	infos := reg.ListDicts()
	now := time.Now().Unix()

	for _, info := range infos {
		// Check if already exists.
		_, err := s.GetDict(info.ID)
		if err == nil {
			// Update entry_count.
			_ = s.UpdateDict(info.ID, UpdateDictRequest{EntryCount: &info.Entries})
			continue
		}

		manifestJSON, _ := json.Marshal(info)
		typ := info.Type
		if typ == "" {
			typ = "registry"
		}

		_, err = s.db.Exec(`INSERT OR IGNORE INTO dict_registry (id, type, jurisdiction, entity_type, domain, manifest_json, entry_count, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			info.ID, typ, info.Jurisdiction, info.EntityType, info.Domain, string(manifestJSON), info.Entries, now, now)
		if err != nil {
			return err
		}
	}
	return nil
}

// MigrateFromSourceDB imports all rows from the legacy import_sources table into
// the admin sources table. Existing source records (by adapter_id) are skipped.
func (s *Service) MigrateFromSourceDB(sdb *importer.SourceDB) error {
	legacySources, err := sdb.ListSources()
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	for _, src := range legacySources {
		// Check if we already have this adapter.
		var count int
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM sources WHERE adapter_id = ?`, src.AdapterID).Scan(&count)
		if count > 0 {
			continue
		}

		// Ensure the dict exists in dict_registry.
		_, getErr := s.GetDict(src.DictID)
		if getErr != nil {
			// Create a minimal record.
			_, _ = s.db.Exec(`INSERT OR IGNORE INTO dict_registry (id, type, jurisdiction, entity_type, created_at, updated_at)
				VALUES (?, 'registry', '', '', ?, ?)`, src.DictID, now, now)
		}

		_, err := s.CreateSource(CreateSourceRequest{
			DictID:      src.DictID,
			AdapterID:   src.AdapterID,
			Description: src.Description,
			SourceURL:   src.SourceURL,
			License:     src.License,
		})
		if err != nil {
			return err
		}
	}
	return nil
}
