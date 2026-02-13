package importer

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckAll_Mixed(t *testing.T) {
	// Server that returns 200.
	srv200 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv200.Close()

	// Server that returns 404.
	srv404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv404.Close()

	// Server that returns 500.
	srv500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv500.Close()

	dir := t.TempDir()
	sdb, err := OpenSourceDB(filepath.Join(dir, "sources.db"))
	if err != nil {
		t.Fatalf("OpenSourceDB: %v", err)
	}
	defer sdb.Close()

	adapters := []Adapter{
		&fakeAdapter{"ok-source", "d1", "OK source", srv200.URL, "CC0"},
		&fakeAdapter{"notfound-source", "d2", "404 source", srv404.URL, "CC0"},
		&fakeAdapter{"error-source", "d3", "500 source", srv500.URL, "CC0"},
	}
	if err := sdb.Seed(adapters); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	checker := NewChecker(sdb, logger, time.Hour)

	ctx := context.Background()
	checker.CheckAll(ctx)

	sources, err := sdb.ListSources()
	if err != nil {
		t.Fatalf("ListSources: %v", err)
	}

	statusByID := make(map[string]int)
	for _, src := range sources {
		if src.LastStatus != nil {
			statusByID[src.AdapterID] = *src.LastStatus
		}
	}

	if statusByID["ok-source"] != 200 {
		t.Errorf("ok-source: expected 200, got %d", statusByID["ok-source"])
	}
	if statusByID["notfound-source"] != 404 {
		t.Errorf("notfound-source: expected 404, got %d", statusByID["notfound-source"])
	}
	if statusByID["error-source"] != 500 {
		t.Errorf("error-source: expected 500, got %d", statusByID["error-source"])
	}
}

func TestCheckAll_NetworkError(t *testing.T) {
	dir := t.TempDir()
	sdb, err := OpenSourceDB(filepath.Join(dir, "sources.db"))
	if err != nil {
		t.Fatalf("OpenSourceDB: %v", err)
	}
	defer sdb.Close()

	adapters := []Adapter{
		&fakeAdapter{"dead-source", "d1", "dead", "http://127.0.0.1:1", "CC0"},
	}
	if err := sdb.Seed(adapters); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	checker := NewChecker(sdb, logger, time.Hour)

	ctx := context.Background()
	checker.CheckAll(ctx)

	sources, _ := sdb.ListSources()
	src := sources[0]
	if src.LastStatus == nil || *src.LastStatus != 0 {
		t.Errorf("expected status 0 for network error, got %v", src.LastStatus)
	}
	if src.LastError == nil || *src.LastError == "" {
		t.Error("expected non-empty last_error for network error")
	}
}

func TestCheckAll_EmptyDB(t *testing.T) {
	dir := t.TempDir()
	sdb, err := OpenSourceDB(filepath.Join(dir, "sources.db"))
	if err != nil {
		t.Fatalf("OpenSourceDB: %v", err)
	}
	defer sdb.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	checker := NewChecker(sdb, logger, time.Hour)

	// Should not panic on empty DB.
	checker.CheckAll(context.Background())
}

func TestCheckAll_Redirect(t *testing.T) {
	// 301 redirect — should be treated as OK (2xx/3xx).
	srv301 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://example.com/new")
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer srv301.Close()

	dir := t.TempDir()
	sdb, err := OpenSourceDB(filepath.Join(dir, "sources.db"))
	if err != nil {
		t.Fatalf("OpenSourceDB: %v", err)
	}
	defer sdb.Close()

	adapters := []Adapter{
		&fakeAdapter{"redirect-source", "d1", "redirect", srv301.URL, "CC0"},
	}
	if err := sdb.Seed(adapters); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	checker := NewChecker(sdb, logger, time.Hour)
	checker.CheckAll(context.Background())

	sources, _ := sdb.ListSources()
	src := sources[0]
	if src.LastStatus == nil || *src.LastStatus != 301 {
		t.Errorf("expected status 301, got %v", src.LastStatus)
	}
}
