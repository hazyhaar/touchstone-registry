package admin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
	"github.com/hazyhaar/touchstone-registry/pkg/importer"
)

func TestSyncFromRegistry(t *testing.T) {
	svc := setupTestService(t)

	// Create a test registry with 2 dicts.
	dir := t.TempDir()
	d1 := filepath.Join(dir, "noms-fr")
	if err := os.MkdirAll(d1, 0o755); err != nil {
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
`
	if err := os.WriteFile(filepath.Join(d1, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d1, "data.csv"), []byte("term\nDUPONT\nMartin\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := dict.NewRegistry(dir)
	if err := reg.Load(); err != nil {
		t.Fatal(err)
	}

	// Sync
	if err := svc.SyncFromRegistry(reg); err != nil {
		t.Fatal(err)
	}

	// Verify
	recs, _ := svc.ListDicts()
	if len(recs) != 1 {
		t.Fatalf("dicts = %d, want 1", len(recs))
	}
	if recs[0].ID != "noms-fr" {
		t.Errorf("ID = %q", recs[0].ID)
	}
	if recs[0].EntryCount != 2 {
		t.Errorf("EntryCount = %d, want 2", recs[0].EntryCount)
	}

	// Run sync again — should be idempotent.
	if err := svc.SyncFromRegistry(reg); err != nil {
		t.Fatal(err)
	}
	recs2, _ := svc.ListDicts()
	if len(recs2) != 1 {
		t.Errorf("after re-sync: dicts = %d, want 1", len(recs2))
	}
}

func TestMigrateFromSourceDB(t *testing.T) {
	svc := setupTestService(t)

	// Create legacy source DB.
	sdbPath := filepath.Join(t.TempDir(), "sources.db")
	sdb, err := importer.OpenSourceDB(sdbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer sdb.Close()

	// Seed it with all adapters.
	if err := sdb.Seed(importer.All()); err != nil {
		t.Fatal(err)
	}

	// Migrate
	if err := svc.MigrateFromSourceDB(sdb); err != nil {
		t.Fatal(err)
	}

	// Verify sources migrated
	sources, _ := svc.ListSources("")
	if len(sources) == 0 {
		t.Error("no sources migrated")
	}

	// Run again — should be idempotent.
	if err := svc.MigrateFromSourceDB(sdb); err != nil {
		t.Fatal(err)
	}
	sources2, _ := svc.ListSources("")
	if len(sources2) != len(sources) {
		t.Errorf("after re-migrate: sources = %d (was %d)", len(sources2), len(sources))
	}
}
