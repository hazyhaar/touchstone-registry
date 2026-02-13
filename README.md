# Touchstone

A blind, stateless lookup service that classifies terms against public data registries. No NLP, no inference, no logs, no sessions. You send a word, you get a classification.

Touchstone is the missing public infrastructure for entity classification. It tells you whether `DUPONT` is a surname, whether `SCI LES LILAS` is a registered company, whether `Rue des Acacias` is a known street type — by looking it up in public registries, not by guessing.

## Why

Every organization that anonymizes documents maintains its own incomplete, unversioned, unshared lists of names, companies, and addresses. Every NER model guesses whether a token is a person or a place. Public registries already have the answers — they're just fragmented across jurisdictions, formats, and APIs.

Touchstone unifies them behind a single protocol.

## How it works

```
Client                          Touchstone                    Dictionaries
  │                                │                              │
  │  GET /v1/classify/DUPONT       │                              │
  │ ─────────────────────────────> │                              │
  │                                │  lookup("dupont")            │
  │                                │ ──────────────────────────>  │
  │                                │  { surname, rank: 22 }       │
  │                                │ <──────────────────────────  │
  │  { match: true,                │                              │
  │    type: "surname",            │                              │
  │    frequency: "common" }       │                              │
  │ <───────────────────────────── │                              │
```

The document never leaves the client. Only isolated terms travel. Touchstone doesn't know they came from the same document, the same user, or the same continent.

## Quick start

```bash
# Build
go build -o touchstone ./cmd/server

# Run
./touchstone serve --config config.yaml

# Classify a term
curl -s http://localhost:8420/v1/classify/DUPONT | jq .

# Classify in batch
curl -s -X POST http://localhost:8420/v1/classify/batch \
  -H "Content-Type: application/json" \
  -d '{"terms": ["DUPONT", "JEAN-PIERRE", "SCI LES LILAS"]}' | jq .

# List loaded dictionaries
curl -s http://localhost:8420/v1/dicts | jq .
```

## Design principles

**No log.** No request is ever logged — not to a file, not to memory, not to a metric. Only aggregate counters (requests/sec, errors/sec) are permitted.

**No correlation.** No cookies, no sessions, no tokens, no stored IPs. Each request is atomic and amnesic. The server cannot link two requests together.

**No intelligence.** No NLP, no ML model, no inference. Pure hashmap lookup. CPU-only. The server is a function: `term → classification`. Nothing more.

**No state.** The server can restart at any moment without loss. Everything is rebuilt from dictionary files on boot.

**Public data only.** Dictionaries contain only data sourced from public registries (INSEE, SIRENE, BAN, Companies House, ONS, SSA, etc.).

## API

### `GET /v1/classify/{term}`

Classify a single term against all loaded dictionaries.

Optional query parameters:
- `jurisdictions` — filter by jurisdiction: `?jurisdictions=fr,uk`
- `types` — filter by entity type: `?types=first_name,surname`
- `dicts` — filter by dictionary: `?dicts=sirene-fr`

### `POST /v1/classify/batch`

Classify up to 100 terms in one call.

```json
{
  "terms": ["DUPONT", "JEAN-PIERRE", "SCI LES LILAS"],
  "jurisdictions": ["fr"]
}
```

### `GET /v1/dicts`

List all loaded dictionaries with metadata (jurisdiction, entity type, entry count, source, version).

### `GET /v1/health`

Returns server status and loaded dictionary summary.

## Dictionaries

Each dictionary is a folder in `dicts/` containing a `manifest.yaml` and a data file (CSV).

```
dicts/
├── prenoms-fr/
│   ├── manifest.yaml
│   └── data.csv
├── patronymes-fr/
│   ├── manifest.yaml
│   └── data.csv
├── sirene-fr/
│   ├── manifest.yaml
│   └── data.csv
└── ...
```

### Manifest format

```yaml
id: patronymes-fr
version: "2025-01"
jurisdiction: fr
entity_type: surname
source: "INSEE fichier des noms de famille"
source_url: "https://www.insee.fr/fr/statistiques/2540004"
license: CC0
data_file: data.csv
format:
  delimiter: ";"
  encoding: utf-8
  has_header: true
  key_column: "term"
  normalize: lowercase_ascii
metadata_columns:
  - name: frequency
    column: "frequency"
  - name: rank
    column: "rank"
```

### Normalization modes

| Mode | Behavior | Use case |
|---|---|---|
| `lowercase_ascii` | Lowercase + strip accents (é→e, ö→o) | Default. Names, companies. |
| `lowercase_utf8` | Lowercase, preserve accents | When accents are distinctive. |
| `none` | Exact match, case-sensitive | Identifiers (SIREN numbers, etc.). |

### Adding a dictionary

Write a `manifest.yaml`, drop a CSV next to it, restart the server (or send `SIGHUP` for hot reload). That's it.

Dictionary data is licensed CC0. The manifest format is part of the Touchstone protocol specification.

## MCP support

Touchstone exposes three MCP tools for native LLM integration:

- `classify_term` — classify a single term
- `classify_batch` — classify multiple terms
- `list_dicts` — list available dictionaries

Any LLM with MCP support can query Touchstone directly.

## Built-in demo dictionaries

The repository ships with 7 demo dictionaries for immediate testing:

| Dictionary | Jurisdiction | Type | Entries | Source |
|---|---|---|---|---|
| `prenoms-fr` | FR | first_name | ~60 | INSEE prénoms |
| `patronymes-fr` | FR | surname | ~60 | INSEE patronymes |
| `sirene-fr` | FR | company | ~30 | SIRENE (sample) |
| `communes-fr` | FR | city | ~30 | COG INSEE |
| `voies-fr` | FR | street | ~20 | BAN types de voies |
| `firstnames-uk` | UK | first_name | ~30 | ONS |
| `companies-uk` | UK | company | ~20 | Companies House (sample) |

These are truncated samples for development. Production dictionaries are maintained separately and can contain millions of entries.

## Tech stack

- **Go** — single binary, no runtime dependency
- **net/http** — no framework
- **In-memory hashmaps** — no database
- **`golang.org/x/text`** — Unicode normalization
- **`gopkg.in/yaml.v3`** — manifest parsing

No Docker required. No Redis. No Postgres. `./touchstone serve` and it's up.

## Roadmap

1. Core server with classify/dicts/health endpoints
2. Batch classification
3. CORS for browser-based clients
4. Hot reload on SIGHUP
5. MCP server (stdio)
6. Full-size dictionary packages (separate repos)
7. Dictionary sync tooling (pull from INSEE, Companies House, etc.)
8. Protocol specification (for independent implementations)

## License

Code: [Apache License 2.0](LICENSE)

Dictionaries and data: [CC0 1.0 Universal](LICENSE-DATA)

## Contributing

Touchstone is designed to be absorbed by larger open-source projects. The Apache 2.0 license is chosen specifically so that projects like Microsoft Presidio, spaCy, or any anonymization framework can integrate Touchstone without legal friction.

The most valuable contributions are new dictionaries. If your country has public registries that aren't covered yet, open a PR with a manifest and a data file.
