package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
)

func setupTestRegistry(t *testing.T) *dict.Registry {
	t.Helper()
	dir := t.TempDir()

	// Dict: noms-fr
	d1 := filepath.Join(dir, "noms-fr")
	if err := os.MkdirAll(d1, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `id: noms-fr
version: "1.0"
jurisdiction: fr
entity_type: surname
source: test
type: registry
data_file: data.csv
entity_spec:
  sensitivity: medium
  pseudo_strategy: hash
response_fields:
  - name: freq
    column: frequency
format:
  delimiter: ";"
  has_header: true
  key_column: "term"
  normalize: lowercase_ascii
metadata_columns:
  - name: frequency
    column: "frequency"
`
	if err := os.WriteFile(filepath.Join(d1, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d1, "data.csv"), []byte("term;frequency\nDUPONT;1200\nMartin;3500\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Alias pool: pharma
	d2 := filepath.Join(dir, "pharma-aliases")
	if err := os.MkdirAll(d2, 0o755); err != nil {
		t.Fatal(err)
	}
	aliasManifest := `id: pharma-aliases
version: "1.0"
jurisdiction: fr
entity_type: alias
source: manual
type: alias_pool
domain: pharma
entries:
  - alias: "SANOFI"
    mimics: "SANOFI-AVENTIS"
`
	if err := os.WriteFile(filepath.Join(d2, "manifest.yaml"), []byte(aliasManifest), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := dict.NewRegistry(dir)
	if err := reg.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return reg
}

func TestHandler_ClassifyTerm(t *testing.T) {
	reg := setupTestRegistry(t)
	router := NewRouter(reg)

	req := httptest.NewRequest("GET", "/v1/classify/DUPONT", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var result dict.ClassifyResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("matches = %d", len(result.Matches))
	}
	if result.Matches[0].EntitySpec == nil {
		t.Error("EntitySpec is nil")
	}
}

func TestHandler_ResolveTerm(t *testing.T) {
	reg := setupTestRegistry(t)
	router := NewRouter(reg)

	req := httptest.NewRequest("GET", "/v1/resolve/DUPONT", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var result dict.ResolveResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if !result.Match {
		t.Error("Match = false")
	}
	if result.Data["freq"] != "1200" {
		t.Errorf("freq = %q, want 1200", result.Data["freq"])
	}
}

func TestHandler_GetAliases(t *testing.T) {
	reg := setupTestRegistry(t)
	router := NewRouter(reg)

	req := httptest.NewRequest("GET", "/v1/aliases/pharma", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var resp aliasesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Domain != "pharma" {
		t.Errorf("Domain = %q", resp.Domain)
	}
	if len(resp.Aliases) != 1 {
		t.Fatalf("aliases = %d", len(resp.Aliases))
	}
	if resp.Aliases[0].Alias != "SANOFI" {
		t.Errorf("alias = %q", resp.Aliases[0].Alias)
	}
}

func TestHandler_GetAliases_Unknown(t *testing.T) {
	reg := setupTestRegistry(t)
	router := NewRouter(reg)

	req := httptest.NewRequest("GET", "/v1/aliases/unknown", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var resp aliasesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Aliases) != 0 {
		t.Errorf("aliases should be empty for unknown domain")
	}
}

func TestHandler_Health(t *testing.T) {
	reg := setupTestRegistry(t)
	router := NewRouter(reg)

	req := httptest.NewRequest("GET", "/v1/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var resp healthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Errorf("Status = %q", resp.Status)
	}
	if resp.Dictionaries < 1 {
		t.Errorf("Dictionaries = %d", resp.Dictionaries)
	}
}

func TestHandler_ListDicts(t *testing.T) {
	reg := setupTestRegistry(t)
	router := NewRouter(reg)

	req := httptest.NewRequest("GET", "/v1/dicts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var resp dictsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Dictionaries) < 1 {
		t.Errorf("dictionaries = %d", len(resp.Dictionaries))
	}
}

func TestHandler_CORS(t *testing.T) {
	reg := setupTestRegistry(t)
	router := NewRouter(reg)

	req := httptest.NewRequest("OPTIONS", "/v1/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("CORS header missing")
	}
}
