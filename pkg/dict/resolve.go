// CLAUDE:SUMMARY Rich resolution of terms with response_fields, JSON mappings, and template expansion.
// CLAUDE:DEPENDS pkg/dict/dict.go, pkg/dict/manifest.go
// CLAUDE:EXPORTS ResolveResult

package dict

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ResolveResult is the response for a term resolution with rich data.
type ResolveResult struct {
	Match      bool              `json:"match"`
	Type       string            `json:"type,omitempty"`
	Dict       string            `json:"dict,omitempty"`
	Data       map[string]string `json:"data,omitempty"`
	EntitySpec *EntitySpec       `json:"entity_spec,omitempty"`
}

// mappingCache caches JSON mapping files loaded from disk.
type mappingCache struct {
	mu    sync.RWMutex
	cache map[string]map[string]string // path → code→label
}

var globalMappingCache = &mappingCache{
	cache: make(map[string]map[string]string),
}

func (mc *mappingCache) get(path string) (map[string]string, error) {
	mc.mu.RLock()
	m, ok := mc.cache[path]
	mc.mu.RUnlock()
	if ok {
		return m, nil
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Double-check after acquiring write lock.
	if cached, ok := mc.cache[path]; ok {
		return cached, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read mapping %s: %w", path, err)
	}
	m = make(map[string]string)
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse mapping %s: %w", path, err)
	}
	mc.cache[path] = m
	return m, nil
}

// Resolve looks up a term in this dictionary and applies response_fields.
func (d *Dictionary) Resolve(term string) (*ResolveResult, bool) {
	entry, ok := d.Classify(term)
	if !ok {
		return nil, false
	}

	result := &ResolveResult{
		Match:      true,
		Type:       d.Manifest.Type,
		Dict:       d.Manifest.ID,
		EntitySpec: d.Manifest.EntitySpec,
		Data:       make(map[string]string),
	}

	if len(d.Manifest.ResponseFields) == 0 {
		// Fallback: return raw metadata
		if entry.Metadata != nil {
			for k, v := range entry.Metadata {
				result.Data[k] = v
			}
		}
		return result, true
	}

	// Apply response_fields
	for _, rf := range d.Manifest.ResponseFields {
		val := d.resolveField(rf, entry)
		result.Data[rf.Name] = val
	}

	return result, true
}

// resolveField extracts a single response field value from an entry.
func (d *Dictionary) resolveField(rf ResponseField, entry *Entry) string {
	if entry.Metadata == nil {
		return ""
	}

	// Template with multiple columns
	if rf.Template != "" && len(rf.Columns) > 0 {
		result := rf.Template
		for _, col := range rf.Columns {
			result = strings.ReplaceAll(result, "{{"+col+"}}", entry.Metadata[col])
		}
		return result
	}

	// Single column with optional mapping
	col := rf.Column
	if col == "" && len(rf.Columns) > 0 {
		col = rf.Columns[0]
	}
	val := entry.Metadata[col]

	// Apply mapping if specified
	if rf.Mapping != "" && val != "" {
		mappingPath := filepath.Join(d.dir, rf.Mapping)
		m, err := globalMappingCache.get(mappingPath)
		if err == nil {
			if label, ok := m[val]; ok {
				return label
			}
		}
	}

	return val
}
