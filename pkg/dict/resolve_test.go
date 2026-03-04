package dict

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupResolveRegistry(t *testing.T) (*Registry, string) { //nolint:unparam // dir used in future tests
	t.Helper()
	dir := t.TempDir()

	// Dict with response_fields + entity_spec
	d1 := filepath.Join(dir, "sirene-fr")
	if err := os.MkdirAll(d1, 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := `id: sirene-fr
version: "1.0"
jurisdiction: fr
entity_type: company
source: test
type: registry
update_frequency: monthly
data_file: data.csv
entity_spec:
  pattern: "^\\d{9}$"
  pseudo_strategy: hash
  pseudo_prefix: "SIREN-"
  sensitivity: high
response_fields:
  - name: siren
    column: siren
  - name: name
    column: company_name
  - name: naf_label
    column: naf_code
    mapping: naf_mapping.json
  - name: address
    columns: [street, zip, city]
    template: "{{street}}, {{zip}} {{city}}"
format:
  delimiter: ";"
  encoding: utf-8
  has_header: true
  key_column: "term"
  normalize: lowercase_ascii
metadata_columns:
  - name: siren
    column: "siren"
  - name: company_name
    column: "company_name"
  - name: naf_code
    column: "naf_code"
  - name: street
    column: "street"
  - name: zip
    column: "zip"
  - name: city
    column: "city"
`
	if err := os.WriteFile(filepath.Join(d1, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	csv := "term;siren;company_name;naf_code;street;zip;city\nSANOFI;395009265;Sanofi SA;2120Z;54 rue La Boétie;75008;Paris\nTOTAL;542051180;TotalEnergies;0610Z;2 place Jean Millier;92400;Courbevoie\n"
	if err := os.WriteFile(filepath.Join(d1, "data.csv"), []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}

	// NAF mapping file
	mapping := map[string]string{
		"2120Z": "Fabrication de produits pharmaceutiques",
		"0610Z": "Extraction de pétrole brut",
	}
	mdata, _ := json.Marshal(mapping)
	if err := os.WriteFile(filepath.Join(d1, "naf_mapping.json"), mdata, 0o644); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry(dir)
	if err := reg.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return reg, dir
}

func TestClassify_EntitySpec(t *testing.T) {
	reg, _ := setupResolveRegistry(t)

	result := reg.Classify("SANOFI", nil)
	if len(result.Matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(result.Matches))
	}
	m := result.Matches[0]
	if m.EntitySpec == nil {
		t.Fatal("EntitySpec is nil")
	}
	if m.EntitySpec.Sensitivity != "high" {
		t.Errorf("Sensitivity = %q, want high", m.EntitySpec.Sensitivity)
	}
	if m.EntitySpec.PseudoPrefix != "SIREN-" {
		t.Errorf("PseudoPrefix = %q, want SIREN-", m.EntitySpec.PseudoPrefix)
	}
}

func TestListDicts_EnrichedFields(t *testing.T) {
	reg, _ := setupResolveRegistry(t)

	infos := reg.ListDicts()
	if len(infos) != 1 {
		t.Fatalf("ListDicts = %d, want 1", len(infos))
	}
	info := infos[0]
	if info.Type != "registry" {
		t.Errorf("Type = %q, want registry", info.Type)
	}
	if info.UpdateFrequency != "monthly" {
		t.Errorf("UpdateFrequency = %q, want monthly", info.UpdateFrequency)
	}
	if info.EntitySpec == nil {
		t.Fatal("EntitySpec is nil in DictInfo")
	}
	if info.EntitySpec.Sensitivity != "high" {
		t.Errorf("Sensitivity = %q, want high", info.EntitySpec.Sensitivity)
	}
}

func TestResolve_BasicFields(t *testing.T) {
	reg, _ := setupResolveRegistry(t)

	result := reg.Resolve("SANOFI", nil)
	if result == nil {
		t.Fatal("Resolve returned nil")
	}
	if !result.Match {
		t.Fatal("Match = false, want true")
	}
	if result.Dict != "sirene-fr" {
		t.Errorf("Dict = %q, want sirene-fr", result.Dict)
	}

	// Check response fields
	if result.Data["siren"] != "395009265" {
		t.Errorf("siren = %q, want 395009265", result.Data["siren"])
	}
	if result.Data["name"] != "Sanofi SA" {
		t.Errorf("name = %q, want Sanofi SA", result.Data["name"])
	}
}

func TestResolve_Mapping(t *testing.T) {
	reg, _ := setupResolveRegistry(t)

	result := reg.Resolve("SANOFI", nil)
	if result == nil {
		t.Fatal("Resolve returned nil")
	}
	if result.Data["naf_label"] != "Fabrication de produits pharmaceutiques" {
		t.Errorf("naf_label = %q", result.Data["naf_label"])
	}
}

func TestResolve_Template(t *testing.T) {
	reg, _ := setupResolveRegistry(t)

	result := reg.Resolve("SANOFI", nil)
	if result == nil {
		t.Fatal("Resolve returned nil")
	}
	want := "54 rue La Boétie, 75008 Paris"
	if result.Data["address"] != want {
		t.Errorf("address = %q, want %q", result.Data["address"], want)
	}
}

func TestResolve_NoMatch(t *testing.T) {
	reg, _ := setupResolveRegistry(t)

	result := reg.Resolve("UNKNOWN", nil)
	if result == nil {
		t.Fatal("Resolve returned nil")
	}
	if result.Match {
		t.Error("Match = true, want false")
	}
}

func TestResolve_FallbackMetadata(t *testing.T) {
	// Registry with no response_fields — should return raw metadata
	dir := t.TempDir()
	d := filepath.Join(dir, "noms-fr")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `id: noms-fr
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
`
	if err := os.WriteFile(filepath.Join(d, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "data.csv"), []byte("term;frequency\nDUPONT;1200\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry(dir)
	if err := reg.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	result := reg.Resolve("DUPONT", nil)
	if result == nil {
		t.Fatal("Resolve returned nil")
	}
	if !result.Match {
		t.Fatal("Match = false")
	}
	if result.Data["freq"] != "1200" {
		t.Errorf("freq = %q, want 1200", result.Data["freq"])
	}
}

func TestResolve_WithFilter(t *testing.T) {
	reg, _ := setupResolveRegistry(t)

	// Filter to a non-matching jurisdiction
	result := reg.Resolve("SANOFI", &ClassifyOptions{Jurisdictions: []string{"uk"}})
	if result.Match {
		t.Error("Match = true with wrong jurisdiction filter")
	}
}
