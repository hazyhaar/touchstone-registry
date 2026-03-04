# admin

Responsabilite: API admin CRUD pour dictionnaires, sources, import runs, audit log. Panel templ+HTMX. Migration sourcedb legacy.
Depend de: `hazyhaar/pkg/audit`, `hazyhaar/pkg/idgen`, `pkg/dict`, `pkg/importer`, `a-h/templ`, `modernc.org/sqlite`
Dependants: `cmd/server`
Point d'entree: `handler.go` (NewRouter — API JSON), `panel.go` (NewPanelRouter — HTML pages), `service.go` (CRUD)
Types cles: `Service`, `DictRecord`, `SourceRecord`, `ImportRunRecord`, `CreateDictRequest`, `CreateSourceRequest`
Invariants: API JSON protegee par bearer token (auth.go). Panel HTML sans auth (proteger via reseau). Audit via hazyhaar/pkg/audit.SQLiteLogger. Admin DB separee (admin.db). SyncFromRegistry et MigrateFromSourceDB au boot (idempotents).
NE PAS: Modifier les `*_templ.go` (generes). Oublier `templ generate` apres modification des `.templ`.
