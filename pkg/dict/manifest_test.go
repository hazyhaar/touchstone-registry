package dict

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifest_EntitySpec(t *testing.T) {
	dir := t.TempDir()
	manifest := `id: test-entity-spec
version: "1.0"
jurisdiction: fr
entity_type: company
source: test
type: registry
update_frequency: weekly
entity_spec:
  pattern: "^\\d{9}$"
  display_pattern: "XXX XXX XXX"
  checksum: luhn
  pseudo_strategy: hash
  pseudo_prefix: "COMP-"
  sensitivity: high
`
	path := filepath.Join(dir, "manifest.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	if m.Type != "registry" {
		t.Errorf("Type = %q, want registry", m.Type)
	}
	if m.UpdateFrequency != "weekly" {
		t.Errorf("UpdateFrequency = %q, want weekly", m.UpdateFrequency)
	}
	if m.EntitySpec == nil {
		t.Fatal("EntitySpec is nil")
	}
	if m.EntitySpec.Pattern != `^\d{9}$` {
		t.Errorf("EntitySpec.Pattern = %q", m.EntitySpec.Pattern)
	}
	if m.EntitySpec.Sensitivity != "high" {
		t.Errorf("EntitySpec.Sensitivity = %q, want high", m.EntitySpec.Sensitivity)
	}
	if m.EntitySpec.PseudoPrefix != "COMP-" {
		t.Errorf("EntitySpec.PseudoPrefix = %q, want COMP-", m.EntitySpec.PseudoPrefix)
	}
}

func TestLoadManifest_ResponseFields(t *testing.T) {
	dir := t.TempDir()
	manifest := `id: test-response
version: "1.0"
jurisdiction: fr
entity_type: company
source: test
response_fields:
  - name: siren
    column: siren
  - name: address
    columns: [street, city, zip]
    template: "{{street}}, {{zip}} {{city}}"
  - name: naf_label
    column: naf_code
    mapping: naf_mapping.json
`
	path := filepath.Join(dir, "manifest.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	if len(m.ResponseFields) != 3 {
		t.Fatalf("ResponseFields len = %d, want 3", len(m.ResponseFields))
	}

	rf0 := m.ResponseFields[0]
	if rf0.Name != "siren" || rf0.Column != "siren" {
		t.Errorf("rf[0] = %+v", rf0)
	}

	rf1 := m.ResponseFields[1]
	if rf1.Name != "address" || len(rf1.Columns) != 3 {
		t.Errorf("rf[1] = %+v", rf1)
	}
	if rf1.Template != "{{street}}, {{zip}} {{city}}" {
		t.Errorf("rf[1].Template = %q", rf1.Template)
	}

	rf2 := m.ResponseFields[2]
	if rf2.Name != "naf_label" || rf2.Mapping != "naf_mapping.json" {
		t.Errorf("rf[2] = %+v", rf2)
	}
}

func TestLoadManifest_AliasPool(t *testing.T) {
	dir := t.TempDir()
	manifest := `id: pharma-aliases
version: "1.0"
jurisdiction: fr
entity_type: alias
source: manual
type: alias_pool
domain: pharma
cribled_against:
  - sirene-fr
  - companies-uk
next_criblage: "2026-04-01"
entries:
  - alias: "SANOFI"
    mimics: "SANOFI-AVENTIS"
    suffix: "SA"
  - alias: "PFIZER"
`
	path := filepath.Join(dir, "manifest.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	if m.Type != "alias_pool" {
		t.Errorf("Type = %q, want alias_pool", m.Type)
	}
	if m.Domain != "pharma" {
		t.Errorf("Domain = %q, want pharma", m.Domain)
	}
	if len(m.CribledAgainst) != 2 {
		t.Fatalf("CribledAgainst len = %d, want 2", len(m.CribledAgainst))
	}
	if m.NextCriblage != "2026-04-01" {
		t.Errorf("NextCriblage = %q", m.NextCriblage)
	}
	if len(m.AliasEntries) != 2 {
		t.Fatalf("AliasEntries len = %d, want 2", len(m.AliasEntries))
	}
	if m.AliasEntries[0].Alias != "SANOFI" || m.AliasEntries[0].Mimics != "SANOFI-AVENTIS" {
		t.Errorf("entry[0] = %+v", m.AliasEntries[0])
	}
}

func TestLoadManifest_DefaultType(t *testing.T) {
	dir := t.TempDir()
	manifest := `id: test-default
version: "1.0"
jurisdiction: fr
entity_type: surname
source: test
`
	path := filepath.Join(dir, "manifest.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	// Type should default to "" (empty means legacy registry)
	if m.Type != "" {
		t.Errorf("Type = %q, want empty", m.Type)
	}
	if m.EntitySpec != nil {
		t.Errorf("EntitySpec should be nil for legacy manifest")
	}
}
