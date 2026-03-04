// CLAUDE:SUMMARY HTTP handlers for admin API CRUD operations on dicts, sources, import runs, and audit log.
// CLAUDE:DEPENDS pkg/admin/service.go, pkg/admin/auth.go
// CLAUDE:EXPORTS NewRouter

package admin

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

// NewRouter returns an http.Handler with all admin API routes, protected by bearer auth.
func NewRouter(svc *Service, token string) http.Handler {
	mux := http.NewServeMux()
	h := &adminHandler{svc: svc}

	// Dicts
	mux.HandleFunc("POST /admin/v1/dicts", h.createDict)
	mux.HandleFunc("GET /admin/v1/dicts", h.listDicts)
	mux.HandleFunc("GET /admin/v1/dicts/{id}", h.getDict)
	mux.HandleFunc("PATCH /admin/v1/dicts/{id}", h.updateDict)
	mux.HandleFunc("DELETE /admin/v1/dicts/{id}", h.deleteDict)

	// Sources
	mux.HandleFunc("POST /admin/v1/sources", h.createSource)
	mux.HandleFunc("GET /admin/v1/sources", h.listSources)
	mux.HandleFunc("PATCH /admin/v1/sources/{id}", h.updateSource)
	mux.HandleFunc("DELETE /admin/v1/sources/{id}", h.deleteSource)

	// Imports
	mux.HandleFunc("GET /admin/v1/imports", h.listImportRuns)
	mux.HandleFunc("GET /admin/v1/imports/{id}", h.getImportRun)

	// Health
	mux.HandleFunc("GET /admin/v1/health", h.health)

	return BearerAuth(token)(mux)
}

type adminHandler struct {
	svc *Service
}

// --- Dicts ---

func (h *adminHandler) createDict(w http.ResponseWriter, r *http.Request) {
	var req CreateDictRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.ID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	rec, err := h.svc.CreateDict(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (h *adminHandler) listDicts(w http.ResponseWriter, _ *http.Request) {
	recs, err := h.svc.ListDicts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *adminHandler) getDict(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rec, err := h.svc.GetDict(id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "dict not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (h *adminHandler) updateDict(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req UpdateDictRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := h.svc.UpdateDict(id, req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *adminHandler) deleteDict(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.svc.DeleteDict(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "archived"})
}

// --- Sources ---

func (h *adminHandler) createSource(w http.ResponseWriter, r *http.Request) {
	var req CreateSourceRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.DictID == "" || req.SourceURL == "" {
		writeError(w, http.StatusBadRequest, "dict_id and source_url are required")
		return
	}

	rec, err := h.svc.CreateSource(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (h *adminHandler) listSources(w http.ResponseWriter, r *http.Request) {
	dictID := r.URL.Query().Get("dict_id")
	recs, err := h.svc.ListSources(dictID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *adminHandler) updateSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req UpdateSourceRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := h.svc.UpdateSource(id, req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *adminHandler) deleteSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.svc.DeleteSource(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Imports ---

func (h *adminHandler) listImportRuns(w http.ResponseWriter, r *http.Request) {
	sourceID := r.URL.Query().Get("source_id")
	dictID := r.URL.Query().Get("dict_id")
	recs, err := h.svc.ListImportRuns(sourceID, dictID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *adminHandler) getImportRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rec, err := h.svc.GetImportRun(id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "import run not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

// --- Health ---

func (h *adminHandler) health(w http.ResponseWriter, _ *http.Request) {
	info, err := h.svc.Health()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
