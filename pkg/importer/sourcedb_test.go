package importer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// fakeAdapter implements Adapter for test seeding.
type fakeAdapter struct {
	id, dictID, desc, url, license string
}

func (f *fakeAdapter) ID() string          { return f.id }
func (f *fakeAdapter) DictID() string      { return f.dictID }
func (f *fakeAdapter) Description() string { return f.desc }
func (f *fakeAdapter) DefaultURL() string  { return f.url }
func (f *fakeAdapter) License() string     { return f.license }
func (f *fakeAdapter) Import(context.Context, string, string) error {
	return nil
}

func tempSourceDB(t *testing.T) *SourceDB {
	t.Helper()
	dir := t.TempDir()
	sdb, err := OpenSourceDB(filepath.Join(dir, "sources.db"))
	if err != nil {
		t.Fatalf("OpenSourceDB: %v", err)
	}
	t.Cleanup(func() { sdb.Close() })
	return sdb
}

func TestOpenSourceDB_CreatesTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	sdb, err := OpenSourceDB(path)
	if err != nil {
		t.Fatalf("OpenSourceDB: %v", err)
	}
	defer sdb.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("db file not created: %v", err)
	}

	// Verify the table exists by listing (should return empty).
	sources, err := sdb.ListSources()
	if err != nil {
		t.Fatalf("ListSources on empty db: %v", err)
	}
	if len(sources) != 0 {
		t.Fatalf("expected 0 sources, got %d", len(sources))
	}
}

func TestSeedAndGetURL(t *testing.T) {
	sdb := tempSourceDB(t)

	adapters := []Adapter{
		&fakeAdapter{"a1", "d1", "desc1", "https://example.com/a1", "CC0"},
		&fakeAdapter{"a2", "d2", "desc2", "https://example.com/a2", "MIT"},
	}

	if err := sdb.Seed(adapters); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	url, err := sdb.GetURL("a1")
	if err != nil {
		t.Fatalf("GetURL: %v", err)
	}
	if url != "https://example.com/a1" {
		t.Fatalf("expected https://example.com/a1, got %s", url)
	}

	// Seed again should not overwrite.
	modified := []Adapter{
		&fakeAdapter{"a1", "d1", "desc1", "https://changed.com/a1", "CC0"},
	}
	if err := sdb.Seed(modified); err != nil {
		t.Fatalf("Seed again: %v", err)
	}

	url, err = sdb.GetURL("a1")
	if err != nil {
		t.Fatalf("GetURL after re-seed: %v", err)
	}
	if url != "https://example.com/a1" {
		t.Fatalf("re-seed should not overwrite, got %s", url)
	}
}

func TestSetURL(t *testing.T) {
	sdb := tempSourceDB(t)

	adapters := []Adapter{
		&fakeAdapter{"a1", "d1", "desc1", "https://example.com/original", "CC0"},
	}
	if err := sdb.Seed(adapters); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	if err := sdb.SetURL("a1", "https://example.com/updated"); err != nil {
		t.Fatalf("SetURL: %v", err)
	}

	url, err := sdb.GetURL("a1")
	if err != nil {
		t.Fatalf("GetURL: %v", err)
	}
	if url != "https://example.com/updated" {
		t.Fatalf("expected updated URL, got %s", url)
	}
}

func TestSetURL_NotFound(t *testing.T) {
	sdb := tempSourceDB(t)

	err := sdb.SetURL("nonexistent", "https://example.com")
	if err == nil {
		t.Fatal("expected error for nonexistent adapter")
	}
}

func TestUpdateCheck(t *testing.T) {
	sdb := tempSourceDB(t)

	adapters := []Adapter{
		&fakeAdapter{"a1", "d1", "desc1", "https://example.com/a1", "CC0"},
	}
	if err := sdb.Seed(adapters); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	if err := sdb.UpdateCheck("a1", 200, ""); err != nil {
		t.Fatalf("UpdateCheck: %v", err)
	}

	sources, err := sdb.ListSources()
	if err != nil {
		t.Fatalf("ListSources: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	src := sources[0]
	if src.LastStatus == nil || *src.LastStatus != 200 {
		t.Fatalf("expected last_status=200, got %v", src.LastStatus)
	}
	if src.LastCheck == nil || *src.LastCheck == 0 {
		t.Fatal("expected last_check to be set")
	}
	if src.LastError != nil {
		t.Fatalf("expected nil last_error, got %v", *src.LastError)
	}

	// Now with an error.
	if err := sdb.UpdateCheck("a1", 404, "not found"); err != nil {
		t.Fatalf("UpdateCheck with error: %v", err)
	}

	sources, _ = sdb.ListSources()
	src = sources[0]
	if src.LastStatus == nil || *src.LastStatus != 404 {
		t.Fatalf("expected last_status=404, got %v", src.LastStatus)
	}
	if src.LastError == nil || *src.LastError != "not found" {
		t.Fatalf("expected last_error='not found', got %v", src.LastError)
	}
}

func TestListSources_Order(t *testing.T) {
	sdb := tempSourceDB(t)

	adapters := []Adapter{
		&fakeAdapter{"z-last", "d1", "desc1", "https://example.com/z", "CC0"},
		&fakeAdapter{"a-first", "d2", "desc2", "https://example.com/a", "MIT"},
	}
	if err := sdb.Seed(adapters); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	sources, err := sdb.ListSources()
	if err != nil {
		t.Fatalf("ListSources: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
	if sources[0].AdapterID != "a-first" {
		t.Fatalf("expected first source to be 'a-first', got %s", sources[0].AdapterID)
	}
}
