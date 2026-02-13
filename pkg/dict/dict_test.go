package dict

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTestDict writes a minimal manifest + CSV in a temp directory and returns the dir.
func writeTestDict(t *testing.T, id, normalize string, csvContent string) string {
	t.Helper()
	dir := t.TempDir()
	dictDir := filepath.Join(dir, id)
	os.MkdirAll(dictDir, 0o755)

	manifest := `id: ` + id + `
version: "1.0"
jurisdiction: test
entity_type: test_type
source: unit test
data_file: data.csv
format:
  delimiter: ";"
  encoding: utf-8
  has_header: true
  key_column: "term"
  normalize: ` + normalize + `
metadata_columns:
  - name: freq
    column: "frequency"
`
	os.WriteFile(filepath.Join(dictDir, "manifest.yaml"), []byte(manifest), 0o644)
	os.WriteFile(filepath.Join(dictDir, "data.csv"), []byte(csvContent), 0o644)
	return dir
}

func TestLoadDictionary(t *testing.T) {
	dir := writeTestDict(t, "test-dict", "lowercase_ascii",
		"term;frequency\nDUPONT;1200\nMartin;3500\nÉlodie;800\n")

	d, err := LoadDictionary(filepath.Join(dir, "test-dict"))
	if err != nil {
		t.Fatalf("LoadDictionary: %v", err)
	}

	if d.Manifest.ID != "test-dict" {
		t.Errorf("ID = %q, want test-dict", d.Manifest.ID)
	}
	if len(d.Entries) != 3 {
		t.Errorf("entries = %d, want 3", len(d.Entries))
	}

	// Normalized keys: DUPONT → dupont, Martin → martin, Élodie → elodie
	for _, key := range []string{"dupont", "martin", "elodie"} {
		if _, ok := d.Entries[key]; !ok {
			t.Errorf("expected key %q after normalization", key)
		}
	}
}

func TestLoadDictionary_Metadata(t *testing.T) {
	dir := writeTestDict(t, "meta-dict", "lowercase_ascii",
		"term;frequency\nDUPONT;1200\n")

	d, err := LoadDictionary(filepath.Join(dir, "meta-dict"))
	if err != nil {
		t.Fatalf("LoadDictionary: %v", err)
	}

	entry, ok := d.Entries["dupont"]
	if !ok {
		t.Fatal("expected key dupont")
	}
	if entry.Metadata["freq"] != "1200" {
		t.Errorf("freq = %q, want 1200", entry.Metadata["freq"])
	}
}

func TestLoadDictionary_EmptyKeys(t *testing.T) {
	dir := writeTestDict(t, "empty-key", "none",
		"term;frequency\n;100\nvalid;200\n;300\n")

	d, err := LoadDictionary(filepath.Join(dir, "empty-key"))
	if err != nil {
		t.Fatalf("LoadDictionary: %v", err)
	}

	// Empty keys should be skipped
	if len(d.Entries) != 1 {
		t.Errorf("entries = %d, want 1 (empty keys skipped)", len(d.Entries))
	}
}

func TestLoadDictionary_MissingKeyColumn(t *testing.T) {
	dir := t.TempDir()
	dictDir := filepath.Join(dir, "bad")
	os.MkdirAll(dictDir, 0o755)

	manifest := `id: bad
version: "1.0"
jurisdiction: test
entity_type: test
source: test
data_file: data.csv
format:
  delimiter: ";"
  has_header: true
  key_column: "nonexistent"
`
	os.WriteFile(filepath.Join(dictDir, "manifest.yaml"), []byte(manifest), 0o644)
	os.WriteFile(filepath.Join(dictDir, "data.csv"), []byte("term;freq\na;1\n"), 0o644)

	_, err := LoadDictionary(dictDir)
	if err == nil {
		t.Error("expected error for missing key column")
	}
}

func TestLookup(t *testing.T) {
	dir := writeTestDict(t, "lookup-dict", "lowercase_ascii",
		"term;frequency\nPARIS;5000\nLyon;3000\n")

	d, err := LoadDictionary(filepath.Join(dir, "lookup-dict"))
	if err != nil {
		t.Fatalf("LoadDictionary: %v", err)
	}

	tests := []struct {
		term  string
		found bool
	}{
		{"PARIS", true},
		{"paris", true},
		{"Paris", true},
		{"lyon", true},
		{"LYON", true},
		{"marseille", false},
	}
	for _, tt := range tests {
		_, ok := d.Lookup(tt.term)
		if ok != tt.found {
			t.Errorf("Lookup(%q) = %v, want %v", tt.term, ok, tt.found)
		}
	}
}

func TestNormalizeTerm(t *testing.T) {
	dir := writeTestDict(t, "norm-dict", "lowercase_ascii",
		"term;frequency\ntest;1\n")

	d, err := LoadDictionary(filepath.Join(dir, "norm-dict"))
	if err != nil {
		t.Fatalf("LoadDictionary: %v", err)
	}

	tests := []struct {
		input, want string
	}{
		{"DUPONT", "dupont"},
		{"Élodie", "elodie"},
		{"café", "cafe"},
		{"naïve", "naive"},
	}
	for _, tt := range tests {
		got := d.NormalizeTerm(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeTerm(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
