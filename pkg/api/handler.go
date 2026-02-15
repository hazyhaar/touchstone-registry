package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
	"github.com/hazyhaar/pkg/kit"
)

// NewRouter returns an http.Handler with all Touchstone API routes.
func NewRouter(reg *dict.Registry) http.Handler {
	mux := http.NewServeMux()
	h := &handler{
		classifyTerm:  classifyTermEndpoint(reg),
		classifyBatch: classifyBatchEndpoint(reg),
		listDicts:     listDictsEndpoint(reg),
		reg:           reg,
	}

	mux.HandleFunc("GET /v1/classify/batch", methodNotAllowed) // prevent GET on batch
	mux.HandleFunc("POST /v1/classify/batch", h.handleClassifyBatch)
	mux.HandleFunc("GET /v1/classify/{term}", h.handleClassifyTerm)
	mux.HandleFunc("GET /v1/dicts", h.handleListDicts)
	mux.HandleFunc("GET /v1/health", h.handleHealth)

	return cors(mux)
}

type handler struct {
	classifyTerm  kit.Endpoint
	classifyBatch kit.Endpoint
	listDicts     kit.Endpoint
	reg           *dict.Registry
}

// --- classify single term ---

func (h *handler) handleClassifyTerm(w http.ResponseWriter, r *http.Request) {
	term := r.PathValue("term")
	if term == "" {
		writeError(w, http.StatusBadRequest, "missing term")
		return
	}

	resp, err := h.classifyTerm(r.Context(), &classifyTermReq{
		Term: term,
		Opts: parseOpts(r),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- classify batch ---

type httpBatchRequest struct {
	Terms         []string `json:"terms"`
	Jurisdictions []string `json:"jurisdictions,omitempty"`
	Types         []string `json:"types,omitempty"`
	Dicts         []string `json:"dicts,omitempty"`
}

func (h *handler) handleClassifyBatch(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024) // 64 KiB max
	var req httpBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	resp, err := h.classifyBatch(r.Context(), &classifyBatchReq{
		Terms: req.Terms,
		Opts: &dict.ClassifyOptions{
			Jurisdictions: req.Jurisdictions,
			Types:         req.Types,
			Dicts:         req.Dicts,
		},
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- list dicts ---

func (h *handler) handleListDicts(w http.ResponseWriter, r *http.Request) {
	resp, err := h.listDicts(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- health ---

type healthResponse struct {
	Status       string `json:"status"`
	Dictionaries int    `json:"dictionaries"`
	TotalEntries int    `json:"total_entries"`
}

func (h *handler) handleHealth(w http.ResponseWriter, r *http.Request) {
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
