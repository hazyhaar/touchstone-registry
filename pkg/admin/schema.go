// CLAUDE:SUMMARY DDL schema for the admin database (dict_registry, sources, import_runs).
// CLAUDE:DEPENDS
// CLAUDE:EXPORTS Schema

package admin

// Schema is the DDL for the admin database tables.
const Schema = `
CREATE TABLE IF NOT EXISTS dict_registry (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL DEFAULT 'registry' CHECK(type IN ('registry','alias_pool')),
    jurisdiction TEXT NOT NULL DEFAULT '',
    entity_type TEXT NOT NULL DEFAULT '',
    domain TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    manifest_json TEXT NOT NULL DEFAULT '{}',
    entry_count INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sources (
    id TEXT PRIMARY KEY,
    dict_id TEXT NOT NULL REFERENCES dict_registry(id),
    adapter_id TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL,
    license TEXT NOT NULL DEFAULT '',
    format TEXT NOT NULL DEFAULT '',
    update_frequency TEXT NOT NULL DEFAULT '',
    last_check INTEGER,
    last_status INTEGER,
    last_error TEXT,
    last_import INTEGER,
    last_import_count INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS import_runs (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL REFERENCES sources(id),
    dict_id TEXT NOT NULL,
    started_at INTEGER NOT NULL,
    finished_at INTEGER,
    status TEXT NOT NULL DEFAULT 'running' CHECK(status IN ('running','success','failed')),
    entry_count INTEGER DEFAULT 0,
    error TEXT,
    duration_ms INTEGER
);
`
