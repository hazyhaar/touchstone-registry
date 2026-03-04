// CLAUDE:SUMMARY Thread-safe registry that loads all dictionaries from disk and serves cross-dictionary classification queries.
package dict

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// Registry holds all loaded dictionaries and serves classification queries.
type Registry struct {
	mu         sync.RWMutex
	dicts      map[string]*Dictionary
	aliasPools map[string][]AliasEntry // domain → entries
	dictsDir   string
}

// NewRegistry creates a new empty registry for the given directory.
func NewRegistry(dictsDir string) *Registry {
	return &Registry{
		dicts:      make(map[string]*Dictionary),
		aliasPools: make(map[string][]AliasEntry),
		dictsDir:   dictsDir,
	}
}

// Load scans the dicts directory and loads every dictionary.
func (r *Registry) Load() error {
	entries, err := os.ReadDir(r.dictsDir)
	if err != nil {
		return fmt.Errorf("read dicts dir %s: %w", r.dictsDir, err)
	}

	newDicts := make(map[string]*Dictionary)
	newAliases := make(map[string][]AliasEntry)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(r.dictsDir, entry.Name())
		manifestPath := filepath.Join(dir, "manifest.yaml")
		if _, err := os.Stat(manifestPath); err != nil {
			continue
		}

		// Peek at manifest to check if alias_pool.
		m, err := LoadManifest(manifestPath)
		if err != nil {
			return fmt.Errorf("load manifest %s: %w", entry.Name(), err)
		}
		if m.Type == "alias_pool" {
			if m.Domain != "" {
				newAliases[m.Domain] = m.AliasEntries
			}
			// Also load as a dictionary for DictInfo listing.
			d := &Dictionary{
				Manifest:  m,
				Entries:   make(map[string]*Entry),
				normalize: GetNormalizer(m.Format.Normalize),
				dir:       dir,
			}
			newDicts[m.ID] = d
			continue
		}

		d, err := LoadDictionary(dir)
		if err != nil {
			return fmt.Errorf("load dictionary %s: %w", entry.Name(), err)
		}
		newDicts[d.Manifest.ID] = d
	}

	r.mu.Lock()
	// Close previous SQLite connections before replacing.
	for _, old := range r.dicts {
		_ = old.Close()
	}
	r.dicts = newDicts
	r.aliasPools = newAliases
	r.mu.Unlock()
	return nil
}

// Reload reloads all dictionaries from disk (hot reload).
func (r *Registry) Reload() error {
	return r.Load()
}

// Match is a single dictionary hit for a classified term.
type Match struct {
	DictID       string            `json:"dict_id"`
	Jurisdiction string            `json:"jurisdiction"`
	EntityType   string            `json:"entity_type"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	EntitySpec   *EntitySpec       `json:"entity_spec,omitempty"`
}

// ClassifyResult is the response for a single term classification.
type ClassifyResult struct {
	Term       string  `json:"term"`
	Normalized string  `json:"normalized"`
	Matches    []Match `json:"matches"`
}

// ClassifyOptions are optional filters for classification.
type ClassifyOptions struct {
	Jurisdictions []string
	Types         []string
	Dicts         []string
}

// Classify looks up a term across all (or filtered) dictionaries.
// Dictionaries are iterated in sorted ID order for deterministic results.
func (r *Registry) Classify(term string, opts *ClassifyOptions) *ClassifyResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := &ClassifyResult{
		Term:    term,
		Matches: []Match{},
	}

	// Sorted iteration for deterministic Normalized field.
	ids := make([]string, 0, len(r.dicts))
	for id := range r.dicts {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		d := r.dicts[id]
		if opts != nil {
			if len(opts.Jurisdictions) > 0 && !contains(opts.Jurisdictions, d.Manifest.Jurisdiction) {
				continue
			}
			if len(opts.Types) > 0 && !contains(opts.Types, d.Manifest.EntityType) {
				continue
			}
			if len(opts.Dicts) > 0 && !contains(opts.Dicts, d.Manifest.ID) {
				continue
			}
		}

		entry, ok := d.Classify(term)
		if !ok {
			continue
		}

		if result.Normalized == "" {
			result.Normalized = d.NormalizeTerm(term)
		}

		m := Match{
			DictID:       d.Manifest.ID,
			Jurisdiction: d.Manifest.Jurisdiction,
			EntityType:   d.Manifest.EntityType,
			EntitySpec:   d.Manifest.EntitySpec,
		}
		if entry.Metadata != nil {
			m.Metadata = entry.Metadata
		}
		result.Matches = append(result.Matches, m)
	}

	if result.Normalized == "" {
		result.Normalized = NormalizeLowercaseASCII(term)
	}
	return result
}

// DictInfo is the public metadata for a loaded dictionary.
type DictInfo struct {
	ID              string      `json:"id"`
	Version         string      `json:"version"`
	Jurisdiction    string      `json:"jurisdiction"`
	EntityType      string      `json:"entity_type"`
	Source          string      `json:"source"`
	SourceURL       string      `json:"source_url,omitempty"`
	License         string      `json:"license"`
	Entries         int         `json:"entries"`
	Type            string      `json:"type,omitempty"`
	UpdateFrequency string      `json:"update_frequency,omitempty"`
	EntitySpec      *EntitySpec `json:"entity_spec,omitempty"`
	Domain          string      `json:"domain,omitempty"`
}

// ListDicts returns metadata for all loaded dictionaries, sorted by ID.
func (r *Registry) ListDicts() []DictInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]DictInfo, 0, len(r.dicts))
	for _, d := range r.dicts {
		entries := d.EntryCount()
		if d.Manifest.Type == "alias_pool" {
			entries = len(d.Manifest.AliasEntries)
		}
		infos = append(infos, DictInfo{
			ID:              d.Manifest.ID,
			Version:         d.Manifest.Version,
			Jurisdiction:    d.Manifest.Jurisdiction,
			EntityType:      d.Manifest.EntityType,
			Source:          d.Manifest.Source,
			SourceURL:       d.Manifest.SourceURL,
			License:         d.Manifest.License,
			Entries:         entries,
			Type:            d.Manifest.Type,
			UpdateFrequency: d.Manifest.UpdateFrequency,
			EntitySpec:      d.Manifest.EntitySpec,
			Domain:          d.Manifest.Domain,
		})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].ID < infos[j].ID })
	return infos
}

// Resolve looks up a term across filtered dictionaries and returns the first rich match.
func (r *Registry) Resolve(term string, opts *ClassifyOptions) *ResolveResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.dicts))
	for id := range r.dicts {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		d := r.dicts[id]
		if opts != nil {
			if len(opts.Jurisdictions) > 0 && !contains(opts.Jurisdictions, d.Manifest.Jurisdiction) {
				continue
			}
			if len(opts.Types) > 0 && !contains(opts.Types, d.Manifest.EntityType) {
				continue
			}
			if len(opts.Dicts) > 0 && !contains(opts.Dicts, d.Manifest.ID) {
				continue
			}
		}

		result, ok := d.Resolve(term)
		if ok {
			return result
		}
	}

	return &ResolveResult{Match: false}
}

// GetAliases returns alias entries for a given domain.
func (r *Registry) GetAliases(domain string) []AliasEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	aliases, ok := r.aliasPools[domain]
	if !ok {
		return nil
	}
	// Return a copy to avoid data races.
	out := make([]AliasEntry, len(aliases))
	copy(out, aliases)
	return out
}

// ListAliasDomains returns all loaded alias pool domains.
func (r *Registry) ListAliasDomains() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	domains := make([]string, 0, len(r.aliasPools))
	for d := range r.aliasPools {
		domains = append(domains, d)
	}
	sort.Strings(domains)
	return domains
}

// DictCount returns the number of loaded dictionaries.
func (r *Registry) DictCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.dicts)
}

// TotalEntries returns the total number of entries across all dictionaries.
func (r *Registry) TotalEntries() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	total := 0
	for _, d := range r.dicts {
		total += d.EntryCount()
	}
	return total
}

// Close releases all resources (SQLite connections) held by dictionaries.
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, d := range r.dicts {
		_ = d.Close()
	}
	return nil
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
