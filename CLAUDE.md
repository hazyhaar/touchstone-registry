> **Protocole** — Avant toute tâche, lire [`../CLAUDE.md`](../CLAUDE.md) §Protocole de recherche.
> Commandes obligatoires : `Read <dossier>/CLAUDE.md` → `Grep "CLAUDE:SUMMARY"` → `Grep "CLAUDE:WARN" <fichier>`.
> **Interdit** : Bash(grep/cat/find) au lieu de Grep/Read. Ne jamais lire un fichier entier en première intention.

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
| `pkg/fo/` | FO anonyme: upload→tokenize→classify, rate limiter, contact form |
| `pkg/importer/` | Framework import: adapters, sourcedb, checker |
| `dicts/` | 12 sous-dossiers de dictionnaires embarqués |
| `config.yaml` | Configuration YAML |
| `Makefile` | Build targets (generate, build, test) |

## Dépendances

- `github.com/hazyhaar/pkg` (audit, chassis, dbopen, horosafe, idgen, kit, observability, shield)
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

## FO Anonyme (anon.repvow.fr)

| Route | Description |
|-------|-------------|
| `GET /` | Landing page avec upload drag & drop + formulaire contact |
| `POST /upload` | Upload fichier → tokenize → classify → résultats (HTML ou JSON) |
| `POST /contact` | Demande d'accès validé (honeypot anti-spam) |

Rate limit: 2 req/s + 50/jour par IP. Formats: PDF, DOCX, ODT, MD, TXT, HTML (max 20 Mo).

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

## NER — Détection de noms (CamemBERT ONNX)

Le FO utilise **CamemBERT-NER** (`Jean-Baptiste/camembert-ner`) via ONNX pour la détection contextuelle de noms.
F1 PER : 0.959 (vs ~0.89 spaCy). Fallback heuristique si NER non configuré.
`scripts/ner.py` (spaCy) conservé comme rollback — changer `ner_script` dans config.yaml suffit.

### Architecture

```
Upload → docpipe (text extraction) → CamemBERT NER (ONNX) → dict classify (pattern PII)
                                         ↓
                                   confirmedNames set → filtre patronymes/prénoms
```

- **Noms** : confirmés par CamemBERT NER (entités `I-PER`) — F1 0.959 sur PER français
- **proper_nouns** : dérivés des entités PER + ORG + LOC + MISC (remplace PROPN POS tagging)
- **Patterns structurés** (IBAN, CB, NIR, phone, email) : dicts classiques, pas besoin de NER
- **common_words** : table SQLite `admin.db` (seed via `dicts/common_words.csv`) — filtre prénoms courants

### Scripts NER

| Script | Moteur | Usage |
|--------|--------|-------|
| `scripts/ner_camembert.py` | ONNX CamemBERT (actif) | ~310ms/texte, F1 0.959 PER |
| `scripts/ner.py` | spaCy fr_core_news_md (rollback) | ~1.9s/texte, F1 ~0.89 PER |
| `scripts/export_model.py` | optimum (one-shot) | Export HF → ONNX, exécuté 1 fois |

### Modèle ONNX

| Env | Path modèle |
|-----|-------------|
| Local | `/inference/touchstone-camembert/model/` |
| VPS FO | `/home/ubuntu/touchstone/model/` |

Fichiers : `model.onnx` (420 MB), `tokenizer.json`, `config.json`, `sentencepiece.bpe.model`
Le script résout le path via `CAMEMBERT_MODEL_DIR` ou fallback `../model` relatif au script.

### Python / venv

| Env | Python | venv | Installeur |
|-----|--------|------|------------|
| Local (CamemBERT) | `/inference/touchstone-camembert/` | onnxruntime + tokenizers + numpy | `uv` |
| Local (spaCy) | `/inference/touchstone-ner/` | spaCy + fr_core_news_md | `uv` |
| VPS FO | `/home/ubuntu/touchstone/.venv/` | onnxruntime + tokenizers + numpy + spaCy | `uv` (`~/.local/bin/uv`) |

**Règle** : Python local uniquement sur `/inference/`. Jamais de venv dans le repo ou sur le disque de dev.

### Config YAML (VPS)

```yaml
ner_python: "/home/ubuntu/touchstone/.venv/bin/python"
ner_script: "/home/ubuntu/touchstone/scripts/ner_camembert.py"
```

### Rollback vers spaCy

```yaml
ner_script: "/home/ubuntu/touchstone/scripts/ner.py"
# puis: systemctl restart touchstone
```

### Gestion common_words en prod (sans redéployer)

```bash
# Ajouter un mot
sqlite3 /home/ubuntu/touchstone/admin.db "INSERT OR IGNORE INTO common_words VALUES('nouveau_mot','surname')"
# Lister
sqlite3 /home/ubuntu/touchstone/admin.db "SELECT * FROM common_words ORDER BY word"
# Supprimer
sqlite3 /home/ubuntu/touchstone/admin.db "DELETE FROM common_words WHERE word='mot'"
# Puis restart pour recharger
systemctl restart touchstone
```

## Invariants

- Config via **YAML** (pas env vars)
- Dictionnaires embarqués dans `dicts/`
- Manifest YAML par dict avec `entity_spec`, `response_fields`, `type`
- Admin DB séparée (admin.db) avec audit via `hazyhaar/pkg/audit`
- Au démarrage : sync auto disque→admin DB + migration legacy sources + seed common_words CSV
- Templates templ : `make generate` avant build
- **NER** : spaCy via subprocess, fallback heuristique si non configuré
- **common_words** : données dans `dicts/common_words.csv`, gérées en SQLite après seed

## NE PAS

- Utiliser env vars pour la config (YAML est le standard ici)
- Modifier les fichiers `*_templ.go` — générés par `templ generate`
- Oublier `make generate` avant build si les .templ changent
- Hardcoder des listes de mots dans Go — tout en SQLite ou CSV
- Installer Python / venvs en dehors de `/inference/` (local) ou `.venv/` (VPS)
