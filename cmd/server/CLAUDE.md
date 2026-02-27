# cmd/server

Responsabilite: Point d'entree du serveur touchstone — dispatche les sous-commandes serve/import, wire le serveur HTTP+MCP avec le registry de dictionnaires, graceful shutdown.
Depend de: `pkg/api`, `pkg/dict`, `pkg/importer`, `github.com/hazyhaar/pkg/chassis`, `github.com/modelcontextprotocol/go-sdk/mcp`
Dependants: aucun (binary final)
Point d'entree: `main.go` (serve), `import.go` (import)
Types cles: `config`
Invariants: Config via YAML (pas env vars). Le serve supporte HTTP/1.1+2 + HTTP/3 + MCP-over-QUIC. L'import telecharge et build les dictionnaires depuis des sources publiques.
NE PAS: Convertir en env vars (YAML est le standard). Lancer import en production sans verifier la source DB.
