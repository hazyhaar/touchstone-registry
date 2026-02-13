package dict

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveGobLoadGobRoundTrip(t *testing.T) {
	entries := map[string]*Entry{
		"dupont": {Metadata: map[string]string{"freq": "1200", "rank": "5"}},
		"martin": {Metadata: map[string]string{"freq": "3500"}},
		"empty":  {},
	}

	path := filepath.Join(t.TempDir(), "data.gob")
	if err := SaveGob(entries, path); err != nil {
		t.Fatalf("SaveGob: %v", err)
	}

	d := &Dictionary{Entries: make(map[string]*Entry)}
	if err := d.loadGob(path); err != nil {
		t.Fatalf("loadGob: %v", err)
	}

	if len(d.Entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(d.Entries))
	}
	if d.Entries["dupont"].Metadata["freq"] != "1200" {
		t.Errorf("dupont freq = %q, want 1200", d.Entries["dupont"].Metadata["freq"])
	}
	if d.Entries["dupont"].Metadata["rank"] != "5" {
		t.Errorf("dupont rank = %q, want 5", d.Entries["dupont"].Metadata["rank"])
	}
	if d.Entries["martin"].Metadata["freq"] != "3500" {
		t.Errorf("martin freq = %q, want 3500", d.Entries["martin"].Metadata["freq"])
	}
	if d.Entries["empty"].Metadata != nil && len(d.Entries["empty"].Metadata) != 0 {
		t.Errorf("empty metadata should be nil or empty, got %v", d.Entries["empty"].Metadata)
	}
}

func TestSaveGobEmptyMap(t *testing.T) {
	entries := map[string]*Entry{}
	path := filepath.Join(t.TempDir(), "data.gob")

	if err := SaveGob(entries, path); err != nil {
		t.Fatalf("SaveGob empty: %v", err)
	}

	d := &Dictionary{Entries: make(map[string]*Entry)}
	if err := d.loadGob(path); err != nil {
		t.Fatalf("loadGob empty: %v", err)
	}
	if len(d.Entries) != 0 {
		t.Errorf("entries = %d, want 0", len(d.Entries))
	}
}

func TestLoadDictionary_PrefersGob(t *testing.T) {
	dir := writeTestDict(t, "gob-pref", "lowercase_ascii",
		"term;frequency\nDUPONT;1200\n")

	dictDir := filepath.Join(dir, "gob-pref")

	// Write a gob file with different data — LoadDictionary should prefer it.
	gobEntries := map[string]*Entry{
		"gobonly": {Metadata: map[string]string{"src": "gob"}},
	}
	if err := SaveGob(gobEntries, filepath.Join(dictDir, "data.gob")); err != nil {
		t.Fatalf("SaveGob: %v", err)
	}

	d, err := LoadDictionary(dictDir)
	if err != nil {
		t.Fatalf("LoadDictionary: %v", err)
	}

	// Should have gob data, not csv data.
	if _, ok := d.Entries["gobonly"]; !ok {
		t.Error("expected key 'gobonly' from gob file")
	}
	if _, ok := d.Entries["dupont"]; ok {
		t.Error("key 'dupont' should not exist — gob takes priority over csv")
	}
}

func TestLoadDictionary_FallbackCSV(t *testing.T) {
	dir := writeTestDict(t, "csv-fallback", "lowercase_ascii",
		"term;frequency\nMARTIN;3500\n")

	dictDir := filepath.Join(dir, "csv-fallback")

	// No gob file — should fall back to CSV.
	d, err := LoadDictionary(dictDir)
	if err != nil {
		t.Fatalf("LoadDictionary: %v", err)
	}
	if _, ok := d.Entries["martin"]; !ok {
		t.Error("expected key 'martin' from csv fallback")
	}
}

func TestLoadGob_FileNotFound(t *testing.T) {
	d := &Dictionary{Entries: make(map[string]*Entry)}
	err := d.loadGob("/nonexistent/path/data.gob")
	if err == nil {
		t.Error("expected error for nonexistent gob file")
	}
}

func TestSaveGob_InvalidPath(t *testing.T) {
	err := SaveGob(map[string]*Entry{}, "/nonexistent/dir/data.gob")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestLoadDictionary_PatternMethod(t *testing.T) {
	dir := t.TempDir()
	dictDir := filepath.Join(dir, "pattern-dict")
	os.MkdirAll(dictDir, 0o755)

	// Pattern method — no data file required.
	manifest := `id: pattern-dict
version: "1.0"
jurisdiction: intl
entity_type: email
source: regex
method: pattern
patterns:
  - name: email
    regex: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
`
	os.WriteFile(filepath.Join(dictDir, "manifest.yaml"), []byte(manifest), 0o644)

	d, err := LoadDictionary(dictDir)
	if err != nil {
		t.Fatalf("LoadDictionary pattern: %v", err)
	}
	if d.patterns == nil {
		t.Error("expected patterns to be compiled")
	}
	if len(d.Entries) != 0 {
		t.Errorf("entries = %d, want 0 for pattern dict", len(d.Entries))
	}
}
