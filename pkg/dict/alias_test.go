package dict

import (
	"os"
	"path/filepath"
	"testing"
)

func setupAliasRegistry(t *testing.T) *Registry {
	t.Helper()
	dir := t.TempDir()

	// Alias pool: pharma
	d1 := filepath.Join(dir, "pharma-aliases")
	if err := os.MkdirAll(d1, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `id: pharma-aliases
version: "1.0"
jurisdiction: fr
entity_type: alias
source: manual
type: alias_pool
domain: pharma
cribled_against:
  - sirene-fr
next_criblage: "2026-04-01"
entries:
  - alias: "SANOFI"
    mimics: "SANOFI-AVENTIS"
    suffix: "SA"
  - alias: "PFIZER"
    mimics: "PFIZER INC"
`
	if err := os.WriteFile(filepath.Join(d1, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	// Regular dict
	d2 := filepath.Join(dir, "noms-fr")
	if err := os.MkdirAll(d2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d2, "manifest.yaml"), []byte(`id: noms-fr
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
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d2, "data.csv"), []byte("term\nDUPONT\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry(dir)
	if err := reg.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return reg
}

func TestGetAliases(t *testing.T) {
	reg := setupAliasRegistry(t)

	aliases := reg.GetAliases("pharma")
	if len(aliases) != 2 {
		t.Fatalf("aliases = %d, want 2", len(aliases))
	}
	if aliases[0].Alias != "SANOFI" {
		t.Errorf("alias[0] = %q, want SANOFI", aliases[0].Alias)
	}
	if aliases[0].Mimics != "SANOFI-AVENTIS" {
		t.Errorf("mimics[0] = %q, want SANOFI-AVENTIS", aliases[0].Mimics)
	}
}

func TestGetAliases_UnknownDomain(t *testing.T) {
	reg := setupAliasRegistry(t)

	aliases := reg.GetAliases("finance")
	if aliases != nil {
		t.Errorf("aliases = %v, want nil", aliases)
	}
}

func TestListAliasDomains(t *testing.T) {
	reg := setupAliasRegistry(t)

	domains := reg.ListAliasDomains()
	if len(domains) != 1 {
		t.Fatalf("domains = %d, want 1", len(domains))
	}
	if domains[0] != "pharma" {
		t.Errorf("domain = %q, want pharma", domains[0])
	}
}

func TestAliasPool_InDictList(t *testing.T) {
	reg := setupAliasRegistry(t)

	infos := reg.ListDicts()
	// Should include both regular dict and alias pool
	if len(infos) != 2 {
		t.Fatalf("ListDicts = %d, want 2", len(infos))
	}

	// Find the alias pool
	var found bool
	for _, info := range infos {
		if info.ID == "pharma-aliases" {
			found = true
			if info.Type != "alias_pool" {
				t.Errorf("Type = %q, want alias_pool", info.Type)
			}
			if info.Domain != "pharma" {
				t.Errorf("Domain = %q, want pharma", info.Domain)
			}
			if info.Entries != 2 {
				t.Errorf("Entries = %d, want 2", info.Entries)
			}
		}
	}
	if !found {
		t.Error("pharma-aliases not found in ListDicts")
	}
}
