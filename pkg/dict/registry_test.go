package dict

import (
	"os"
	"path/filepath"
	"testing"
)

func setupRegistry(t *testing.T) (*Registry, string) {
	t.Helper()
	dir := t.TempDir()

	// Dict 1: French surnames
	d1 := filepath.Join(dir, "noms-fr")
	os.MkdirAll(d1, 0o755)
	os.WriteFile(filepath.Join(d1, "manifest.yaml"), []byte(`id: noms-fr
version: "1.0"
jurisdiction: fr
entity_type: surname
source: test
data_file: data.csv
format:
  delimiter: ";"
  has_header: true
  key_column: "term"
  normalize: lowercase_ascii
metadata_columns:
  - name: freq
    column: "frequency"
`), 0o644)
	os.WriteFile(filepath.Join(d1, "data.csv"), []byte("term;frequency\nDUPONT;1200\nMartin;3500\nÉlodie;800\n"), 0o644)

	// Dict 2: UK first names
	d2 := filepath.Join(dir, "firstnames-uk")
	os.MkdirAll(d2, 0o755)
	os.WriteFile(filepath.Join(d2, "manifest.yaml"), []byte(`id: firstnames-uk
version: "1.0"
jurisdiction: uk
entity_type: first_name
source: test
data_file: data.csv
format:
  delimiter: ";"
  has_header: true
  key_column: "term"
  normalize: lowercase_ascii
`), 0o644)
	os.WriteFile(filepath.Join(d2, "data.csv"), []byte("term\nJames\nEmma\nMartin\n"), 0o644)

	reg := NewRegistry(dir)
	if err := reg.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return reg, dir
}

func TestRegistryLoad(t *testing.T) {
	reg, _ := setupRegistry(t)

	if reg.DictCount() != 2 {
		t.Errorf("DictCount = %d, want 2", reg.DictCount())
	}
	if reg.TotalEntries() == 0 {
		t.Error("TotalEntries = 0, want > 0")
	}
}

func TestClassify_Match(t *testing.T) {
	reg, _ := setupRegistry(t)

	result := reg.Classify("DUPONT", nil)
	if result.Term != "DUPONT" {
		t.Errorf("Term = %q, want DUPONT", result.Term)
	}
	if result.Normalized != "dupont" {
		t.Errorf("Normalized = %q, want dupont", result.Normalized)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(result.Matches))
	}
	if result.Matches[0].DictID != "noms-fr" {
		t.Errorf("DictID = %q, want noms-fr", result.Matches[0].DictID)
	}
}

func TestClassify_MultiDict(t *testing.T) {
	reg, _ := setupRegistry(t)

	// "Martin" exists in both noms-fr and firstnames-uk
	result := reg.Classify("Martin", nil)
	if len(result.Matches) != 2 {
		t.Errorf("matches = %d, want 2 (noms-fr + firstnames-uk)", len(result.Matches))
	}
}

func TestClassify_NoMatch(t *testing.T) {
	reg, _ := setupRegistry(t)

	result := reg.Classify("Xylocopal", nil)
	if len(result.Matches) != 0 {
		t.Errorf("matches = %d, want 0", len(result.Matches))
	}
	// Normalized should still be set (fallback to lowercase_ascii)
	if result.Normalized != "xylocopal" {
		t.Errorf("Normalized = %q, want xylocopal", result.Normalized)
	}
}

func TestClassify_FilterJurisdiction(t *testing.T) {
	reg, _ := setupRegistry(t)

	result := reg.Classify("Martin", &ClassifyOptions{Jurisdictions: []string{"fr"}})
	if len(result.Matches) != 1 {
		t.Fatalf("matches = %d, want 1 (fr only)", len(result.Matches))
	}
	if result.Matches[0].Jurisdiction != "fr" {
		t.Errorf("Jurisdiction = %q, want fr", result.Matches[0].Jurisdiction)
	}
}

func TestClassify_FilterType(t *testing.T) {
	reg, _ := setupRegistry(t)

	result := reg.Classify("Martin", &ClassifyOptions{Types: []string{"first_name"}})
	if len(result.Matches) != 1 {
		t.Fatalf("matches = %d, want 1 (first_name only)", len(result.Matches))
	}
	if result.Matches[0].EntityType != "first_name" {
		t.Errorf("EntityType = %q, want first_name", result.Matches[0].EntityType)
	}
}

func TestClassify_FilterDict(t *testing.T) {
	reg, _ := setupRegistry(t)

	result := reg.Classify("Martin", &ClassifyOptions{Dicts: []string{"firstnames-uk"}})
	if len(result.Matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(result.Matches))
	}
	if result.Matches[0].DictID != "firstnames-uk" {
		t.Errorf("DictID = %q, want firstnames-uk", result.Matches[0].DictID)
	}
}

func TestClassify_FilterNoResult(t *testing.T) {
	reg, _ := setupRegistry(t)

	result := reg.Classify("Martin", &ClassifyOptions{Jurisdictions: []string{"de"}})
	if len(result.Matches) != 0 {
		t.Errorf("matches = %d, want 0 (no de jurisdiction)", len(result.Matches))
	}
}

func TestClassify_Deterministic(t *testing.T) {
	reg, _ := setupRegistry(t)

	// Run multiple times to verify deterministic ordering
	for i := 0; i < 20; i++ {
		result := reg.Classify("Martin", nil)
		if len(result.Matches) != 2 {
			t.Fatalf("iteration %d: matches = %d, want 2", i, len(result.Matches))
		}
		// Sorted by dict ID: firstnames-uk < noms-fr
		if result.Matches[0].DictID != "firstnames-uk" {
			t.Errorf("iteration %d: first match = %q, want firstnames-uk (sorted order)", i, result.Matches[0].DictID)
		}
		if result.Matches[1].DictID != "noms-fr" {
			t.Errorf("iteration %d: second match = %q, want noms-fr (sorted order)", i, result.Matches[1].DictID)
		}
	}
}

func TestListDicts(t *testing.T) {
	reg, _ := setupRegistry(t)

	infos := reg.ListDicts()
	if len(infos) != 2 {
		t.Fatalf("ListDicts = %d, want 2", len(infos))
	}
	// Sorted by ID
	if infos[0].ID != "firstnames-uk" {
		t.Errorf("first = %q, want firstnames-uk", infos[0].ID)
	}
	if infos[1].ID != "noms-fr" {
		t.Errorf("second = %q, want noms-fr", infos[1].ID)
	}
}

func TestReload(t *testing.T) {
	reg, dir := setupRegistry(t)

	if reg.DictCount() != 2 {
		t.Fatalf("before reload: %d dicts", reg.DictCount())
	}

	// Add a third dict
	d3 := filepath.Join(dir, "villes-fr")
	os.MkdirAll(d3, 0o755)
	os.WriteFile(filepath.Join(d3, "manifest.yaml"), []byte(`id: villes-fr
version: "1.0"
jurisdiction: fr
entity_type: city
source: test
data_file: data.csv
format:
  delimiter: ";"
  has_header: true
  key_column: "term"
  normalize: lowercase_ascii
`), 0o644)
	os.WriteFile(filepath.Join(d3, "data.csv"), []byte("term\nParis\nLyon\n"), 0o644)

	if err := reg.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if reg.DictCount() != 3 {
		t.Errorf("after reload: %d dicts, want 3", reg.DictCount())
	}
}

func TestEmptyRegistry(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	if err := reg.Load(); err != nil {
		t.Fatalf("Load empty: %v", err)
	}
	if reg.DictCount() != 0 {
		t.Errorf("DictCount = %d, want 0", reg.DictCount())
	}
	if reg.TotalEntries() != 0 {
		t.Errorf("TotalEntries = %d, want 0", reg.TotalEntries())
	}

	result := reg.Classify("anything", nil)
	if len(result.Matches) != 0 {
		t.Errorf("matches = %d, want 0", len(result.Matches))
	}
}
