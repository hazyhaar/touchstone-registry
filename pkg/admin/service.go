// CLAUDE:SUMMARY Admin service layer for CRUD operations on dicts, sources, import runs, and alias pools with audit logging.
// CLAUDE:DEPENDS pkg/admin/schema.go
// CLAUDE:EXPORTS Service, DictRecord, SourceRecord, ImportRunRecord, CreateDictRequest, CreateSourceRequest

package admin

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hazyhaar/pkg/audit"
	"github.com/hazyhaar/pkg/idgen"
)

// Service provides CRUD operations for the admin API.
type Service struct {
	db    *sql.DB
	audit audit.Logger
}

// NewService creates a new admin service.
func NewService(db *sql.DB, auditLogger audit.Logger) *Service {
	return &Service{db: db, audit: auditLogger}
}

// --- Dict records ---

// DictRecord is a row from dict_registry.
type DictRecord struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Jurisdiction string `json:"jurisdiction"`
	EntityType   string `json:"entity_type"`
	Domain       string `json:"domain"`
	Status       string `json:"status"`
	ManifestJSON string `json:"manifest_json"`
	EntryCount   int    `json:"entry_count"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

// CreateDictRequest is the payload for creating a new dictionary.
type CreateDictRequest struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Jurisdiction string `json:"jurisdiction"`
	EntityType   string `json:"entity_type"`
	Domain       string `json:"domain"`
	ManifestJSON string `json:"manifest_json"`
}

// UpdateDictRequest is the payload for updating a dictionary.
type UpdateDictRequest struct {
	Jurisdiction string `json:"jurisdiction,omitempty"`
	EntityType   string `json:"entity_type,omitempty"`
	Domain       string `json:"domain,omitempty"`
	Status       string `json:"status,omitempty"`
	ManifestJSON string `json:"manifest_json,omitempty"`
	EntryCount   *int   `json:"entry_count,omitempty"`
}

// CreateDict inserts a new dictionary record.
func (s *Service) CreateDict(req CreateDictRequest) (*DictRecord, error) {
	now := time.Now().Unix()
	if req.Type == "" {
		req.Type = "registry"
	}
	if req.ManifestJSON == "" {
		req.ManifestJSON = "{}"
	}

	_, err := s.db.Exec(`INSERT INTO dict_registry (id, type, jurisdiction, entity_type, domain, manifest_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ID, req.Type, req.Jurisdiction, req.EntityType, req.Domain, req.ManifestJSON, now, now)
	if err != nil {
		return nil, fmt.Errorf("create dict: %w", err)
	}

	s.logAudit("dict.create", req.ID, req)

	return &DictRecord{
		ID:           req.ID,
		Type:         req.Type,
		Jurisdiction: req.Jurisdiction,
		EntityType:   req.EntityType,
		Domain:       req.Domain,
		Status:       "active",
		ManifestJSON: req.ManifestJSON,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// GetDict returns a single dictionary record by ID.
func (s *Service) GetDict(id string) (*DictRecord, error) {
	var rec DictRecord
	err := s.db.QueryRow(`SELECT id, type, jurisdiction, entity_type, domain, status, manifest_json, entry_count, created_at, updated_at
		FROM dict_registry WHERE id = ?`, id).Scan(
		&rec.ID, &rec.Type, &rec.Jurisdiction, &rec.EntityType, &rec.Domain,
		&rec.Status, &rec.ManifestJSON, &rec.EntryCount, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get dict %s: %w", id, err)
	}
	return &rec, nil
}

// ListDicts returns all dictionary records.
func (s *Service) ListDicts() ([]DictRecord, error) {
	rows, err := s.db.Query(`SELECT id, type, jurisdiction, entity_type, domain, status, manifest_json, entry_count, created_at, updated_at
		FROM dict_registry ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list dicts: %w", err)
	}
	defer rows.Close()

	var recs []DictRecord
	for rows.Next() {
		var rec DictRecord
		if err := rows.Scan(&rec.ID, &rec.Type, &rec.Jurisdiction, &rec.EntityType, &rec.Domain,
			&rec.Status, &rec.ManifestJSON, &rec.EntryCount, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan dict: %w", err)
		}
		recs = append(recs, rec)
	}
	return recs, rows.Err()
}

// UpdateDict updates a dictionary record.
func (s *Service) UpdateDict(id string, req UpdateDictRequest) error {
	now := time.Now().Unix()

	// Build dynamic UPDATE. Simple approach: update all fields if non-empty.
	_, err := s.db.Exec(`UPDATE dict_registry SET
		jurisdiction = CASE WHEN ? != '' THEN ? ELSE jurisdiction END,
		entity_type = CASE WHEN ? != '' THEN ? ELSE entity_type END,
		domain = CASE WHEN ? != '' THEN ? ELSE domain END,
		status = CASE WHEN ? != '' THEN ? ELSE status END,
		manifest_json = CASE WHEN ? != '' THEN ? ELSE manifest_json END,
		entry_count = CASE WHEN ? IS NOT NULL THEN ? ELSE entry_count END,
		updated_at = ?
		WHERE id = ?`,
		req.Jurisdiction, req.Jurisdiction,
		req.EntityType, req.EntityType,
		req.Domain, req.Domain,
		req.Status, req.Status,
		req.ManifestJSON, req.ManifestJSON,
		req.EntryCount, req.EntryCount,
		now, id)
	if err != nil {
		return fmt.Errorf("update dict %s: %w", id, err)
	}

	s.logAudit("dict.update", id, req)
	return nil
}

// DeleteDict soft-deletes a dictionary by setting status to "archived".
func (s *Service) DeleteDict(id string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`UPDATE dict_registry SET status = 'archived', updated_at = ? WHERE id = ?`, now, id)
	if err != nil {
		return fmt.Errorf("delete dict %s: %w", id, err)
	}
	s.logAudit("dict.delete", id, nil)
	return nil
}

// --- Source records ---

// SourceRecord is a row from sources.
type SourceRecord struct {
	ID              string  `json:"id"`
	DictID          string  `json:"dict_id"`
	AdapterID       string  `json:"adapter_id"`
	Description     string  `json:"description"`
	SourceURL       string  `json:"source_url"`
	License         string  `json:"license"`
	Format          string  `json:"format"`
	UpdateFrequency string  `json:"update_frequency"`
	LastCheck       *int64  `json:"last_check"`
	LastStatus      *int    `json:"last_status"`
	LastError       *string `json:"last_error"`
	LastImport      *int64  `json:"last_import"`
	LastImportCount *int    `json:"last_import_count"`
	CreatedAt       int64   `json:"created_at"`
	UpdatedAt       int64   `json:"updated_at"`
}

// CreateSourceRequest is the payload for creating a new source.
type CreateSourceRequest struct {
	DictID          string `json:"dict_id"`
	AdapterID       string `json:"adapter_id"`
	Description     string `json:"description"`
	SourceURL       string `json:"source_url"`
	License         string `json:"license"`
	Format          string `json:"format"`
	UpdateFrequency string `json:"update_frequency"`
}

// UpdateSourceRequest is the payload for updating a source.
type UpdateSourceRequest struct {
	Description     string `json:"description,omitempty"`
	SourceURL       string `json:"source_url,omitempty"`
	License         string `json:"license,omitempty"`
	Format          string `json:"format,omitempty"`
	UpdateFrequency string `json:"update_frequency,omitempty"`
}

// CreateSource inserts a new source record.
func (s *Service) CreateSource(req CreateSourceRequest) (*SourceRecord, error) {
	now := time.Now().Unix()
	id := idgen.New()

	_, err := s.db.Exec(`INSERT INTO sources (id, dict_id, adapter_id, description, source_url, license, format, update_frequency, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, req.DictID, req.AdapterID, req.Description, req.SourceURL, req.License, req.Format, req.UpdateFrequency, now, now)
	if err != nil {
		return nil, fmt.Errorf("create source: %w", err)
	}

	s.logAudit("source.create", id, req)

	return &SourceRecord{
		ID:              id,
		DictID:          req.DictID,
		AdapterID:       req.AdapterID,
		Description:     req.Description,
		SourceURL:       req.SourceURL,
		License:         req.License,
		Format:          req.Format,
		UpdateFrequency: req.UpdateFrequency,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

// ListSources returns all sources, optionally filtered by dict_id.
func (s *Service) ListSources(dictID string) ([]SourceRecord, error) {
	query := `SELECT id, dict_id, adapter_id, description, source_url, license, format, update_frequency,
		last_check, last_status, last_error, last_import, last_import_count, created_at, updated_at
		FROM sources`
	var args []any
	if dictID != "" {
		query += ` WHERE dict_id = ?`
		args = append(args, dictID)
	}
	query += ` ORDER BY dict_id, id`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()

	var recs []SourceRecord
	for rows.Next() {
		var rec SourceRecord
		if err := rows.Scan(&rec.ID, &rec.DictID, &rec.AdapterID, &rec.Description, &rec.SourceURL,
			&rec.License, &rec.Format, &rec.UpdateFrequency,
			&rec.LastCheck, &rec.LastStatus, &rec.LastError,
			&rec.LastImport, &rec.LastImportCount, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan source: %w", err)
		}
		recs = append(recs, rec)
	}
	return recs, rows.Err()
}

// UpdateSource updates a source record.
func (s *Service) UpdateSource(id string, req UpdateSourceRequest) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`UPDATE sources SET
		description = CASE WHEN ? != '' THEN ? ELSE description END,
		source_url = CASE WHEN ? != '' THEN ? ELSE source_url END,
		license = CASE WHEN ? != '' THEN ? ELSE license END,
		format = CASE WHEN ? != '' THEN ? ELSE format END,
		update_frequency = CASE WHEN ? != '' THEN ? ELSE update_frequency END,
		updated_at = ?
		WHERE id = ?`,
		req.Description, req.Description,
		req.SourceURL, req.SourceURL,
		req.License, req.License,
		req.Format, req.Format,
		req.UpdateFrequency, req.UpdateFrequency,
		now, id)
	if err != nil {
		return fmt.Errorf("update source %s: %w", id, err)
	}
	s.logAudit("source.update", id, req)
	return nil
}

// DeleteSource deletes a source record.
func (s *Service) DeleteSource(id string) error {
	_, err := s.db.Exec(`DELETE FROM sources WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete source %s: %w", id, err)
	}
	s.logAudit("source.delete", id, nil)
	return nil
}

// --- Import runs ---

// ImportRunRecord is a row from import_runs.
type ImportRunRecord struct {
	ID         string  `json:"id"`
	SourceID   string  `json:"source_id"`
	DictID     string  `json:"dict_id"`
	StartedAt  int64   `json:"started_at"`
	FinishedAt *int64  `json:"finished_at"`
	Status     string  `json:"status"`
	EntryCount int     `json:"entry_count"`
	Error      *string `json:"error"`
	DurationMs *int64  `json:"duration_ms"`
}

// CreateImportRun inserts a new import run record.
func (s *Service) CreateImportRun(sourceID, dictID string) (*ImportRunRecord, error) {
	now := time.Now().Unix()
	id := idgen.New()

	_, err := s.db.Exec(`INSERT INTO import_runs (id, source_id, dict_id, started_at) VALUES (?, ?, ?, ?)`,
		id, sourceID, dictID, now)
	if err != nil {
		return nil, fmt.Errorf("create import run: %w", err)
	}

	s.logAudit("import.start", id, map[string]string{"source_id": sourceID, "dict_id": dictID})

	return &ImportRunRecord{
		ID:        id,
		SourceID:  sourceID,
		DictID:    dictID,
		StartedAt: now,
		Status:    "running",
	}, nil
}

// FinishImportRun completes an import run.
func (s *Service) FinishImportRun(id string, entryCount int, importErr error) error {
	now := time.Now().Unix()
	status := "success"
	var errStr *string
	if importErr != nil {
		status = "failed"
		s := importErr.Error()
		errStr = &s
	}

	// Calculate duration from started_at.
	var startedAt int64
	_ = s.db.QueryRow(`SELECT started_at FROM import_runs WHERE id = ?`, id).Scan(&startedAt)
	durationMs := (now - startedAt) * 1000

	_, err := s.db.Exec(`UPDATE import_runs SET finished_at = ?, status = ?, entry_count = ?, error = ?, duration_ms = ? WHERE id = ?`,
		now, status, entryCount, errStr, durationMs, id)
	if err != nil {
		return fmt.Errorf("finish import run %s: %w", id, err)
	}

	s.logAudit("import.finish", id, map[string]any{"status": status, "entry_count": entryCount})
	return nil
}

// ListImportRuns returns import runs, optionally filtered.
func (s *Service) ListImportRuns(sourceID, dictID string) ([]ImportRunRecord, error) {
	query := `SELECT id, source_id, dict_id, started_at, finished_at, status, entry_count, error, duration_ms
		FROM import_runs WHERE 1=1`
	var args []any
	if sourceID != "" {
		query += ` AND source_id = ?`
		args = append(args, sourceID)
	}
	if dictID != "" {
		query += ` AND dict_id = ?`
		args = append(args, dictID)
	}
	query += ` ORDER BY started_at DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list import runs: %w", err)
	}
	defer rows.Close()

	var recs []ImportRunRecord
	for rows.Next() {
		var rec ImportRunRecord
		if err := rows.Scan(&rec.ID, &rec.SourceID, &rec.DictID, &rec.StartedAt, &rec.FinishedAt,
			&rec.Status, &rec.EntryCount, &rec.Error, &rec.DurationMs); err != nil {
			return nil, fmt.Errorf("scan import run: %w", err)
		}
		recs = append(recs, rec)
	}
	return recs, rows.Err()
}

// GetImportRun returns a single import run by ID.
func (s *Service) GetImportRun(id string) (*ImportRunRecord, error) {
	var rec ImportRunRecord
	err := s.db.QueryRow(`SELECT id, source_id, dict_id, started_at, finished_at, status, entry_count, error, duration_ms
		FROM import_runs WHERE id = ?`, id).Scan(
		&rec.ID, &rec.SourceID, &rec.DictID, &rec.StartedAt, &rec.FinishedAt,
		&rec.Status, &rec.EntryCount, &rec.Error, &rec.DurationMs)
	if err != nil {
		return nil, fmt.Errorf("get import run %s: %w", id, err)
	}
	return &rec, nil
}

// --- Health ---

// HealthInfo holds admin health metrics.
type HealthInfo struct {
	DictCount   int `json:"dict_count"`
	SourceCount int `json:"source_count"`
	ImportRuns  int `json:"import_runs"`
}

// Health returns basic health stats.
func (s *Service) Health() (*HealthInfo, error) {
	var info HealthInfo
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM dict_registry WHERE status = 'active'`).Scan(&info.DictCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM sources`).Scan(&info.SourceCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM import_runs`).Scan(&info.ImportRuns)
	return &info, nil
}

// --- Audit helper ---

func (s *Service) logAudit(action, targetID string, params any) {
	if s.audit == nil {
		return
	}
	paramsJSON, _ := json.Marshal(params)
	s.audit.LogAsync(&audit.Entry{
		Action:     action,
		Parameters: string(paramsJSON),
		Result:     targetID,
		Status:     "success",
	})
}
