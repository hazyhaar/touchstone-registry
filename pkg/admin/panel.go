// CLAUDE:SUMMARY HTTP handlers for admin panel pages (templ + HTMX) rendering dashboard, dicts, sources, imports, audit.
// CLAUDE:DEPENDS pkg/admin/service.go, pkg/admin/templates/
// CLAUDE:EXPORTS NewPanelRouter

package admin

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/hazyhaar/touchstone-registry/pkg/admin/templates"
	"github.com/hazyhaar/touchstone-registry/pkg/dict"
)

// NewPanelRouter returns an http.Handler for the admin panel HTML pages.
// The registry is used to display live dictionary stats.
func NewPanelRouter(svc *Service, reg *dict.Registry) http.Handler {
	mux := http.NewServeMux()
	p := &panelHandler{svc: svc, reg: reg}

	mux.HandleFunc("GET /admin/", p.dashboard)
	mux.HandleFunc("GET /admin/dicts", p.dictsList)
	mux.HandleFunc("GET /admin/dicts/{id}", p.dictDetail)
	mux.HandleFunc("GET /admin/sources", p.sourcesList)
	mux.HandleFunc("GET /admin/imports", p.importsList)
	mux.HandleFunc("GET /admin/audit", p.auditLog)

	return mux
}

type panelHandler struct {
	svc *Service
	reg *dict.Registry
}

func (p *panelHandler) dashboard(w http.ResponseWriter, r *http.Request) {
	health, _ := p.svc.Health()
	runs, _ := p.svc.ListImportRuns("", "")

	totalEntries := 0
	if p.reg != nil {
		totalEntries = p.reg.TotalEntries()
	}

	recentRuns := make([]templates.ImportRunView, 0)
	for i, run := range runs {
		if i >= 10 {
			break
		}
		recentRuns = append(recentRuns, importRunToView(run))
	}

	data := templates.DashboardData{
		DictCount:    health.DictCount,
		SourceCount:  health.SourceCount,
		ImportRuns:   health.ImportRuns,
		TotalEntries: totalEntries,
		RecentRuns:   recentRuns,
	}

	renderTempl(w, r, templates.Dashboard(data))
}

func (p *panelHandler) dictsList(w http.ResponseWriter, r *http.Request) {
	recs, _ := p.svc.ListDicts()

	views := make([]templates.DictView, len(recs))
	for i, rec := range recs {
		views[i] = dictToView(rec)
	}

	renderTempl(w, r, templates.DictsList(views))
}

func (p *panelHandler) dictDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rec, err := p.svc.GetDict(id)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Dict not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sources, _ := p.svc.ListSources(id)
	runs, _ := p.svc.ListImportRuns("", id)

	sourceViews := make([]templates.SourceView, len(sources))
	for i, s := range sources {
		sourceViews[i] = sourceToView(s)
	}

	runViews := make([]templates.ImportRunView, 0)
	for i, run := range runs {
		if i >= 20 {
			break
		}
		runViews = append(runViews, importRunToView(run))
	}

	renderTempl(w, r, templates.DictDetail(dictToView(*rec), sourceViews, runViews))
}

func (p *panelHandler) sourcesList(w http.ResponseWriter, r *http.Request) {
	recs, _ := p.svc.ListSources("")

	views := make([]templates.SourceView, len(recs))
	for i, s := range recs {
		views[i] = sourceToView(s)
	}

	renderTempl(w, r, templates.SourcesList(views))
}

func (p *panelHandler) importsList(w http.ResponseWriter, r *http.Request) {
	runs, _ := p.svc.ListImportRuns("", "")

	views := make([]templates.ImportRunView, len(runs))
	for i, run := range runs {
		views[i] = importRunToView(run)
	}

	renderTempl(w, r, templates.ImportsList(views))
}

func (p *panelHandler) auditLog(w http.ResponseWriter, r *http.Request) {
	// Query audit_log table directly.
	entries := make([]templates.AuditEntryView, 0)
	rows, err := p.svc.db.Query(`SELECT entry_id, timestamp, action, parameters, result, status
		FROM audit_log ORDER BY timestamp DESC LIMIT 100`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var e templates.AuditEntryView
			var ts int64
			if err := rows.Scan(&e.EntryID, &ts, &e.Action, &e.Parameters, &e.Result, &e.Status); err != nil {
				continue
			}
			e.Timestamp = time.Unix(ts, 0).Format("2006-01-02 15:04:05")
			entries = append(entries, e)
		}
	}

	renderTempl(w, r, templates.AuditLog(entries))
}

// --- View conversions ---

func dictToView(rec DictRecord) templates.DictView {
	typ := rec.Type
	if typ == "" {
		typ = "registry"
	}
	return templates.DictView{
		ID:           rec.ID,
		Type:         typ,
		Jurisdiction: rec.Jurisdiction,
		EntityType:   rec.EntityType,
		Domain:       rec.Domain,
		Status:       rec.Status,
		EntryCount:   rec.EntryCount,
		CreatedAt:    time.Unix(rec.CreatedAt, 0).Format("2006-01-02"),
		UpdatedAt:    time.Unix(rec.UpdatedAt, 0).Format("2006-01-02"),
	}
}

func sourceToView(rec SourceRecord) templates.SourceView {
	lastStatus := ""
	if rec.LastStatus != nil {
		if *rec.LastStatus >= 200 && *rec.LastStatus < 400 {
			lastStatus = "ok"
		} else {
			lastStatus = fmt.Sprintf("%d", *rec.LastStatus)
		}
	}
	lastImport := ""
	if rec.LastImport != nil {
		lastImport = time.Unix(*rec.LastImport, 0).Format("2006-01-02 15:04")
	}
	lastImportCount := 0
	if rec.LastImportCount != nil {
		lastImportCount = *rec.LastImportCount
	}
	return templates.SourceView{
		ID:              rec.ID,
		DictID:          rec.DictID,
		AdapterID:       rec.AdapterID,
		Description:     rec.Description,
		SourceURL:       rec.SourceURL,
		License:         rec.License,
		Format:          rec.Format,
		UpdateFrequency: rec.UpdateFrequency,
		LastStatus:      lastStatus,
		LastImport:      lastImport,
		LastImportCount: lastImportCount,
	}
}

func importRunToView(rec ImportRunRecord) templates.ImportRunView {
	errStr := ""
	if rec.Error != nil {
		errStr = *rec.Error
	}
	durationMs := int64(0)
	if rec.DurationMs != nil {
		durationMs = *rec.DurationMs
	}
	return templates.ImportRunView{
		ID:         rec.ID,
		SourceID:   rec.SourceID,
		DictID:     rec.DictID,
		StartedAt:  time.Unix(rec.StartedAt, 0).Format("2006-01-02 15:04"),
		Status:     rec.Status,
		EntryCount: rec.EntryCount,
		DurationMs: durationMs,
		Error:      errStr,
	}
}

func renderTempl(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = c.Render(r.Context(), w)
}
