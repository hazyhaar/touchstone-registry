> **Protocole** — Avant toute tâche, lire [`../CLAUDE.md`](../CLAUDE.md) §Protocole de recherche.
> Commandes obligatoires : `cat <dossier>/CLAUDE.md` → `grep -rn "CLAUDE:SUMMARY"` → `grep -n "CLAUDE:WARN" <fichier>`.
> **Interdit** : Glob/Read/Explore/find au lieu de `grep -rn`. Ne jamais lire un fichier entier en première intention.

# touchstone-registry-audit

Responsabilité: Registre linguistique et audit par dictionnaires — résolution riche, classification, alias pools, admin CRUD + panel.
Module: `github.com/hazyhaar/touchstone-registry`
État: Actif, refactoring complet (mars 2026)

## Index

| Fichier/Dir | Rôle |
|-------------|------|
| `cmd/server/` | Entry point serveur (serve + import subcommands) |
| `pkg/dict/` | Core: dictionnaires, registry, classify, resolve, alias pools |
| `pkg/api/` | HTTP routes + MCP tools (classify, resolve, aliases, dicts, health) |
| `pkg/admin/` | Admin API CRUD + audit + panel templ + migration sourcedb |
| `pkg/admin/templates/` | Templates templ pour le panel admin |
| `pkg/importer/` | Framework import: adapters, sourcedb, checker |
| `dicts/` | 12 sous-dossiers de dictionnaires embarqués |
| `config.yaml` | Configuration YAML |
| `Makefile` | Build targets (generate, build, test) |

## Dépendances

- `github.com/hazyhaar/pkg` (audit, chassis, dbopen, idgen, kit)
- `github.com/a-h/templ` (admin panel)
- `github.com/modelcontextprotocol/go-sdk/mcp`
- `modernc.org/sqlite`

## Build / Test

```bash
make build     # templ generate + go build → bin/touchstone
make test      # go test -race -v -count=1 -timeout 120s ./...
make generate  # templ generate seulement
```

## API Publique

| Endpoint | Description |
|----------|-------------|
| `GET /v1/classify/{term}` | Classification binaire cross-dicts |
| `POST /v1/classify/batch` | Batch classification (max 100 termes) |
| `GET /v1/resolve/{term}` | Résolution riche (response_fields, mappings, templates) |
| `GET /v1/aliases/{domain}` | Alias pool par domaine |
| `GET /v1/dicts` | Liste des dictionnaires chargés |
| `GET /v1/health` | Health check |

Query params: `?jurisdictions=fr,uk&types=surname&dicts=patronymes-fr`

## API Admin (Bearer token)

| Endpoint | Description |
|----------|-------------|
| `GET/POST /admin/v1/dicts` | CRUD dictionnaires |
| `GET/PATCH/DELETE /admin/v1/dicts/{id}` | Opérations par dict |
| `GET/POST /admin/v1/sources` | CRUD sources |
| `PATCH/DELETE /admin/v1/sources/{id}` | Opérations par source |
| `GET /admin/v1/imports` | Liste import runs |
| `GET /admin/v1/imports/{id}` | Détail import run |
| `GET /admin/v1/health` | Stats admin (counts) |

Auth: `Authorization: Bearer <admin_token>` (config.yaml)

## Panel Admin (HTML)

| Route | Description |
|-------|-------------|
| `GET /admin/` | Dashboard (stats, derniers imports) |
| `GET /admin/dicts` | Liste dictionnaires |
| `GET /admin/dicts/{id}` | Détail dict (sources, imports) |
| `GET /admin/sources` | Liste sources |
| `GET /admin/imports` | Historique imports |
| `GET /admin/audit` | Audit log viewer |

## MCP Tools

- `classify_term` — Classification single term
- `classify_batch` — Batch classification
- `resolve_term` — Résolution riche
- `get_aliases` — Aliases par domaine
- `list_dicts` — Liste dictionnaires

## Types clés

- `dict.Manifest` — avec EntitySpec, ResponseFields, AliasEntries, Type, Domain
- `dict.EntitySpec` — pattern, pseudo_strategy, sensitivity
- `dict.ResponseField` — column, columns, template, mapping
- `dict.ResolveResult` — match riche avec data map + entity_spec
- `admin.Service` — CRUD dicts, sources, import_runs + audit
- `admin.DictRecord`, `admin.SourceRecord`, `admin.ImportRunRecord`

## Config

```yaml
addr: ":8420"
dicts_dir: "dicts"
admin_token: "changeme"    # bearer token admin API
admin_db: "admin.db"       # path DB admin (défaut: dicts/admin.db)
cert_file: ""              # TLS cert (optionnel)
key_file: ""               # TLS key (optionnel)
```

## Invariants

- Config via **YAML** (pas env vars)
- Dictionnaires embarqués dans `dicts/`
- Manifest YAML par dict avec `entity_spec`, `response_fields`, `type`
- Admin DB séparée (admin.db) avec audit via `hazyhaar/pkg/audit`
- Au démarrage : sync auto disque→admin DB + migration legacy sources
- Templates templ : `make generate` avant build

## NE PAS

- Utiliser env vars pour la config (YAML est le standard ici)
- Modifier les fichiers `*_templ.go` — générés par `templ generate`
- Oublier `make generate` avant build si les .templ changent
