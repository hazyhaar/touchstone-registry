package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
)

// NewRouter returns an http.Handler with all Touchstone API routes.
func NewRouter(reg *dict.Registry) http.Handler {
	mux := http.NewServeMux()
	h := &handler{reg: reg}

	mux.HandleFunc("GET /v1/classify/batch", methodNotAllowed) // prevent GET on batch
	mux.HandleFunc("POST /v1/classify/batch", h.classifyBatch)
	mux.HandleFunc("GET /v1/classify/{term}", h.classifyTerm)
	mux.HandleFunc("GET /v1/dicts", h.listDicts)
	mux.HandleFunc("GET /v1/health", h.health)

	return cors(mux)
}

type handler struct {
	reg *dict.Registry
}

// --- classify single term ---

func (h *handler) classifyTerm(w http.ResponseWriter, r *http.Request) {
	term := r.PathValue("term")
	if term == "" {
		writeError(w, http.StatusBadRequest, "missing term")
		return
	}

	opts := parseOpts(r)
	result := h.reg.Classify(term, opts)
	writeJSON(w, http.StatusOK, result)
}

// --- classify batch ---

type batchRequest struct {
	Terms         []string `json:"terms"`
	Jurisdictions []string `json:"jurisdictions,omitempty"`
	Types         []string `json:"types,omitempty"`
	Dicts         []string `json:"dicts,omitempty"`
}

type batchResponse struct {
	Results []*dict.ClassifyResult `json:"results"`
}

func (h *handler) classifyBatch(w http.ResponseWriter, r *http.Request) {
	var req batchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(req.Terms) == 0 {
		writeError(w, http.StatusBadRequest, "terms array is empty")
		return
	}
	if len(req.Terms) > 100 {
		writeError(w, http.StatusBadRequest, "too many terms (max 100)")
		return
	}

	opts := &dict.ClassifyOptions{
		Jurisdictions: req.Jurisdictions,
		Types:         req.Types,
		Dicts:         req.Dicts,
	}

	resp := batchResponse{Results: make([]*dict.ClassifyResult, len(req.Terms))}
	for i, term := range req.Terms {
		resp.Results[i] = h.reg.Classify(term, opts)
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- list dicts ---

type dictsResponse struct {
	Dictionaries []dict.DictInfo `json:"dictionaries"`
}

func (h *handler) listDicts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, dictsResponse{Dictionaries: h.reg.ListDicts()})
}

// --- health ---

type healthResponse struct {
	Status       string `json:"status"`
	Dictionaries int    `json:"dictionaries"`
	TotalEntries int    `json:"total_entries"`
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:       "ok",
		Dictionaries: h.reg.DictCount(),
		TotalEntries: h.reg.TotalEntries(),
	})
}

// --- helpers ---

func parseOpts(r *http.Request) *dict.ClassifyOptions {
	opts := &dict.ClassifyOptions{}
	if v := r.URL.Query().Get("jurisdictions"); v != "" {
		opts.Jurisdictions = strings.Split(v, ",")
	}
	if v := r.URL.Query().Get("types"); v != "" {
		opts.Types = strings.Split(v, ",")
	}
	if v := r.URL.Query().Get("dicts"); v != "" {
		opts.Dicts = strings.Split(v, ",")
	}
	return opts
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func methodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

// cors is a simple CORS middleware for browser-based clients.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
