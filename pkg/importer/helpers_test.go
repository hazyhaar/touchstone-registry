package importer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
)

func TestDownloadFile(t *testing.T) {
	content := "hello world"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(content))
	}))
	defer ts.Close()

	dest := filepath.Join(t.TempDir(), "test.txt")
	if err := downloadFile(context.Background(), ts.URL, dest); err != nil {
		t.Fatalf("downloadFile: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != content {
		t.Errorf("content = %q, want %q", string(data), content)
	}
}

func TestDownloadFile_Retry(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	dest := filepath.Join(t.TempDir(), "retry.txt")
	if err := downloadFile(context.Background(), ts.URL, dest); err != nil {
		t.Fatalf("downloadFile with retries: %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestDownloadFile_AllFail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	dest := filepath.Join(t.TempDir(), "fail.txt")
	err := downloadFile(context.Background(), ts.URL, dest)
	if err == nil {
		t.Error("expected error after all retries exhausted")
	}
}

func TestWriteManifest(t *testing.T) {
	dir := t.TempDir()
	m := &dict.Manifest{
		ID:           "test-dict",
		Version:      "2026-02",
		Jurisdiction: "fr",
		EntityType:   "surname",
		Source:       "test",
		License:      "CC0",
		DataFile:     "data.gob",
	}

	if err := writeManifest(dir, m); err != nil {
		t.Fatalf("writeManifest: %v", err)
	}

	// Verify the file was written and can be parsed back.
	loaded, err := dict.LoadManifest(filepath.Join(dir, "manifest.yaml"))
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if loaded.ID != "test-dict" {
		t.Errorf("ID = %q, want test-dict", loaded.ID)
	}
	if loaded.DataFile != "data.gob" {
		t.Errorf("DataFile = %q, want data.gob", loaded.DataFile)
	}
}
