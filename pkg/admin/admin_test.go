package admin

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_pragma=journal_mode(wal)&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(Schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func setupTestService(t *testing.T) *Service {
	t.Helper()
	db := setupTestDB(t)
	return NewService(db, nil) // no audit for unit tests
}

// --- Service tests ---

func TestCreateDict(t *testing.T) {
	svc := setupTestService(t)

	rec, err := svc.CreateDict(CreateDictRequest{
		ID:           "test-dict",
		Type:         "registry",
		Jurisdiction: "fr",
		EntityType:   "company",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.ID != "test-dict" {
		t.Errorf("ID = %q", rec.ID)
	}
	if rec.Status != "active" {
		t.Errorf("Status = %q", rec.Status)
	}

	// Verify in DB
	got, err := svc.GetDict("test-dict")
	if err != nil {
		t.Fatal(err)
	}
	if got.Jurisdiction != "fr" {
		t.Errorf("Jurisdiction = %q", got.Jurisdiction)
	}
}

func TestListDicts(t *testing.T) {
	svc := setupTestService(t)

	_, _ = svc.CreateDict(CreateDictRequest{ID: "a"})
	_, _ = svc.CreateDict(CreateDictRequest{ID: "b"})

	recs, err := svc.ListDicts()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("len = %d, want 2", len(recs))
	}
	if recs[0].ID != "a" {
		t.Errorf("first = %q, want a", recs[0].ID)
	}
}

func TestDeleteDict(t *testing.T) {
	svc := setupTestService(t)
	_, _ = svc.CreateDict(CreateDictRequest{ID: "del-me"})

	if err := svc.DeleteDict("del-me"); err != nil {
		t.Fatal(err)
	}

	got, _ := svc.GetDict("del-me")
	if got.Status != "archived" {
		t.Errorf("Status = %q, want archived", got.Status)
	}
}

func TestCreateSource(t *testing.T) {
	svc := setupTestService(t)
	_, _ = svc.CreateDict(CreateDictRequest{ID: "dict-1"})

	rec, err := svc.CreateSource(CreateSourceRequest{
		DictID:    "dict-1",
		SourceURL: "https://example.com/data.csv",
		License:   "CC0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.DictID != "dict-1" {
		t.Errorf("DictID = %q", rec.DictID)
	}
	if rec.ID == "" {
		t.Error("ID is empty")
	}

	// List sources
	sources, _ := svc.ListSources("dict-1")
	if len(sources) != 1 {
		t.Fatalf("sources = %d, want 1", len(sources))
	}
}

func TestImportRun(t *testing.T) {
	svc := setupTestService(t)
	_, _ = svc.CreateDict(CreateDictRequest{ID: "dict-1"})
	src, _ := svc.CreateSource(CreateSourceRequest{DictID: "dict-1", SourceURL: "https://example.com"})

	run, err := svc.CreateImportRun(src.ID, "dict-1")
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "running" {
		t.Errorf("Status = %q, want running", run.Status)
	}

	if err := svc.FinishImportRun(run.ID, 100, nil); err != nil {
		t.Fatal(err)
	}

	got, _ := svc.GetImportRun(run.ID)
	if got.Status != "success" {
		t.Errorf("Status = %q, want success", got.Status)
	}
	if got.EntryCount != 100 {
		t.Errorf("EntryCount = %d, want 100", got.EntryCount)
	}
}

func TestHealth(t *testing.T) {
	svc := setupTestService(t)
	_, _ = svc.CreateDict(CreateDictRequest{ID: "d1"})
	_, _ = svc.CreateDict(CreateDictRequest{ID: "d2"})

	info, err := svc.Health()
	if err != nil {
		t.Fatal(err)
	}
	if info.DictCount != 2 {
		t.Errorf("DictCount = %d, want 2", info.DictCount)
	}
}

// --- HTTP handler tests ---

func TestBearerAuth_Valid(t *testing.T) {
	handler := BearerAuth("secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestBearerAuth_Invalid(t *testing.T) {
	handler := BearerAuth("secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestBearerAuth_Missing(t *testing.T) {
	handler := BearerAuth("secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAdminRouter_CreateAndList(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	router := NewRouter(svc, "test-token")

	// Create dict
	body := `{"id":"test-dict","type":"registry","jurisdiction":"fr","entity_type":"company"}`
	req := httptest.NewRequest("POST", "/admin/v1/dicts", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}

	// List dicts
	req = httptest.NewRequest("GET", "/admin/v1/dicts", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d", w.Code)
	}

	var dicts []DictRecord
	if err := json.NewDecoder(w.Body).Decode(&dicts); err != nil {
		t.Fatal(err)
	}
	if len(dicts) != 1 {
		t.Errorf("dicts = %d, want 1", len(dicts))
	}
}

func TestAdminRouter_Unauthorized(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	router := NewRouter(svc, "test-token")

	req := httptest.NewRequest("GET", "/admin/v1/dicts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAdminRouter_Health(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	router := NewRouter(svc, "test-token")

	req := httptest.NewRequest("GET", "/admin/v1/health", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("health status = %d", w.Code)
	}
}
