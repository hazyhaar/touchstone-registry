package dict

import (
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSaveSQLiteRoundTrip(t *testing.T) {
	entries := map[string]*Entry{
		"dupont": {Metadata: map[string]string{"freq": "1200", "rank": "5"}},
		"martin": {Metadata: map[string]string{"freq": "3500"}},
		"empty":  {},
	}

	path := filepath.Join(t.TempDir(), "data.db")
	if err := SaveSQLite(entries, path); err != nil {
		t.Fatalf("SaveSQLite: %v", err)
	}

	d := &Dictionary{normalize: GetNormalizer("none")}
	if err := d.loadSQLite(path); err != nil {
		t.Fatalf("loadSQLite: %v", err)
	}
	defer d.Close()

	if d.entryCount != 3 {
		t.Fatalf("entryCount = %d, want 3", d.entryCount)
	}
	if d.EntryCount() != 3 {
		t.Fatalf("EntryCount() = %d, want 3", d.EntryCount())
	}

	e, ok := d.lookupSQLite("dupont")
	if !ok {
		t.Fatal("expected to find dupont")
	}
	if e.Metadata["freq"] != "1200" {
		t.Errorf("dupont freq = %q, want 1200", e.Metadata["freq"])
	}
	if e.Metadata["rank"] != "5" {
		t.Errorf("dupont rank = %q, want 5", e.Metadata["rank"])
	}

	e, ok = d.lookupSQLite("martin")
	if !ok {
		t.Fatal("expected to find martin")
	}
	if e.Metadata["freq"] != "3500" {
		t.Errorf("martin freq = %q, want 3500", e.Metadata["freq"])
	}

	e, ok = d.lookupSQLite("empty")
	if !ok {
		t.Fatal("expected to find empty")
	}
	if len(e.Metadata) != 0 {
		t.Errorf("empty metadata = %v, want empty", e.Metadata)
	}
}

func TestSaveSQLiteEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	if err := SaveSQLite(map[string]*Entry{}, path); err != nil {
		t.Fatalf("SaveSQLite empty: %v", err)
	}

	d := &Dictionary{normalize: GetNormalizer("none")}
	if err := d.loadSQLite(path); err != nil {
		t.Fatalf("loadSQLite empty: %v", err)
	}
	defer d.Close()

	if d.entryCount != 0 {
		t.Errorf("entryCount = %d, want 0", d.entryCount)
	}
}

func TestSaveSQLite_InvalidPath(t *testing.T) {
	err := SaveSQLite(map[string]*Entry{}, "/nonexistent/dir/data.db")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestLoadDictionary_PrefersSQLiteOverGob(t *testing.T) {
	dir := writeTestDict(t, "sqlite-pref", "lowercase_ascii",
		"term;frequency\nCSVONLY;100\n")
	dictDir := filepath.Join(dir, "sqlite-pref")

	// Write gob with gob-only data.
	gobEntries := map[string]*Entry{
		"gobonly": {Metadata: map[string]string{"src": "gob"}},
	}
	if err := SaveGob(gobEntries, filepath.Join(dictDir, "data.gob")); err != nil {
		t.Fatalf("SaveGob: %v", err)
	}

	// Write SQLite with sqlite-only data.
	sqliteEntries := map[string]*Entry{
		"sqliteonly": {Metadata: map[string]string{"src": "sqlite"}},
	}
	if err := SaveSQLite(sqliteEntries, filepath.Join(dictDir, "data.db")); err != nil {
		t.Fatalf("SaveSQLite: %v", err)
	}

	d, err := LoadDictionary(dictDir)
	if err != nil {
		t.Fatalf("LoadDictionary: %v", err)
	}
	defer d.Close()

	// Should have sqlite data, not gob or csv data.
	if _, ok := d.Lookup("sqliteonly"); !ok {
		t.Error("expected key 'sqliteonly' from sqlite file")
	}
	if _, ok := d.Lookup("gobonly"); ok {
		t.Error("key 'gobonly' should not exist — sqlite takes priority")
	}
	if _, ok := d.Lookup("csvonly"); ok {
		t.Error("key 'csvonly' should not exist — sqlite takes priority")
	}
}

func TestLoadDictionary_FallbackGob(t *testing.T) {
	dir := writeTestDict(t, "gob-fallback", "lowercase_ascii",
		"term;frequency\nCSVONLY;100\n")
	dictDir := filepath.Join(dir, "gob-fallback")

	// Write gob, no SQLite — should fall back to gob.
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
	defer d.Close()

	if _, ok := d.Lookup("gobonly"); !ok {
		t.Error("expected key 'gobonly' from gob fallback")
	}
}

func TestLookupSQLite(t *testing.T) {
	dir := t.TempDir()
	dictDir := filepath.Join(dir, "sqlite-lookup")
	if err := os.MkdirAll(dictDir, 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := `id: sqlite-lookup
version: "1.0"
jurisdiction: test
entity_type: test_type
source: unit test
data_file: data.db
format:
  normalize: lowercase_ascii
`
	if err := os.WriteFile(filepath.Join(dictDir, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := map[string]*Entry{
		"paris":  {Metadata: map[string]string{"pop": "2M"}},
		"lyon":   {Metadata: map[string]string{"pop": "500K"}},
		"elodie": {Metadata: map[string]string{"type": "prenom"}},
	}
	if err := SaveSQLite(entries, filepath.Join(dictDir, "data.db")); err != nil {
		t.Fatalf("SaveSQLite: %v", err)
	}

	d, err := LoadDictionary(dictDir)
	if err != nil {
		t.Fatalf("LoadDictionary: %v", err)
	}
	defer d.Close()

	tests := []struct {
		term  string
		found bool
	}{
		{"PARIS", true},
		{"paris", true},
		{"Paris", true},
		{"lyon", true},
		{"LYON", true},
		{"Élodie", true}, // normalized to elodie
		{"marseille", false},
	}
	for _, tt := range tests {
		_, ok := d.Lookup(tt.term)
		if ok != tt.found {
			t.Errorf("Lookup(%q) = %v, want %v", tt.term, ok, tt.found)
		}
	}
}

func TestClassifySQLite(t *testing.T) {
	dir := t.TempDir()
	dictDir := filepath.Join(dir, "sqlite-classify")
	if err := os.MkdirAll(dictDir, 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := `id: sqlite-classify
version: "1.0"
jurisdiction: fr
entity_type: surname
source: unit test
data_file: data.db
format:
  normalize: lowercase_ascii
`
	if err := os.WriteFile(filepath.Join(dictDir, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := map[string]*Entry{
		"dupont": {Metadata: map[string]string{"freq": "1200"}},
	}
	if err := SaveSQLite(entries, filepath.Join(dictDir, "data.db")); err != nil {
		t.Fatalf("SaveSQLite: %v", err)
	}

	d, err := LoadDictionary(dictDir)
	if err != nil {
		t.Fatalf("LoadDictionary: %v", err)
	}
	defer d.Close()

	entry, ok := d.Classify("DUPONT")
	if !ok {
		t.Fatal("expected Classify to match DUPONT")
	}
	if entry.Metadata["freq"] != "1200" {
		t.Errorf("freq = %q, want 1200", entry.Metadata["freq"])
	}
}

func TestEntryCountSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	entries := map[string]*Entry{
		"a": {}, "b": {}, "c": {},
	}
	if err := SaveSQLite(entries, path); err != nil {
		t.Fatalf("SaveSQLite: %v", err)
	}

	d := &Dictionary{normalize: GetNormalizer("none")}
	if err := d.loadSQLite(path); err != nil {
		t.Fatalf("loadSQLite: %v", err)
	}
	defer d.Close()

	if d.EntryCount() != 3 {
		t.Errorf("EntryCount() = %d, want 3", d.EntryCount())
	}
}

func TestRegistryTotalEntries_SQLite(t *testing.T) {
	dir := t.TempDir()

	// Create a SQLite-backed dict.
	dictDir := filepath.Join(dir, "sqlite-dict")
	if err := os.MkdirAll(dictDir, 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := `id: sqlite-dict
version: "1.0"
jurisdiction: test
entity_type: test_type
source: unit test
data_file: data.db
format:
  normalize: none
`
	if err := os.WriteFile(filepath.Join(dictDir, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := map[string]*Entry{
		"alpha": {}, "beta": {}, "gamma": {},
	}
	if err := SaveSQLite(entries, filepath.Join(dictDir, "data.db")); err != nil {
		t.Fatalf("SaveSQLite: %v", err)
	}

	reg := NewRegistry(dir)
	if err := reg.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer reg.Close()

	if reg.DictCount() != 1 {
		t.Fatalf("DictCount = %d, want 1", reg.DictCount())
	}
	if reg.TotalEntries() != 3 {
		t.Errorf("TotalEntries = %d, want 3", reg.TotalEntries())
	}

	// Verify classify works through registry.
	result := reg.Classify("alpha", nil)
	if len(result.Matches) != 1 {
		t.Errorf("matches = %d, want 1", len(result.Matches))
	}
}

func TestDictionaryClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	entries := map[string]*Entry{
		"hello": {Metadata: map[string]string{"v": "1"}},
	}
	if err := SaveSQLite(entries, path); err != nil {
		t.Fatalf("SaveSQLite: %v", err)
	}

	d := &Dictionary{normalize: GetNormalizer("none")}
	if err := d.loadSQLite(path); err != nil {
		t.Fatalf("loadSQLite: %v", err)
	}

	// Before close, lookup works.
	if _, ok := d.lookupSQLite("hello"); !ok {
		t.Error("expected to find hello before close")
	}

	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// After close, lookup fails gracefully.
	if _, ok := d.lookupSQLite("hello"); ok {
		t.Error("expected lookup to fail after close")
	}
}
