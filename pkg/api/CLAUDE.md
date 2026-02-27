# api

Responsabilite: API REST et MCP tools pour le registre touchstone — classify (term/batch), list dicts, health. Expose des kit.Endpoint transport-agnostiques utilises par HTTP et MCP.
Depend de: `pkg/dict`, `github.com/hazyhaar/pkg/kit`, `github.com/modelcontextprotocol/go-sdk/mcp`
Dependants: `cmd/server`
Point d'entree: `handler.go` (NewRouter), `mcp.go` (RegisterMCPTools), `endpoints.go` (kit.Endpoints)
Types cles: `handler`, `classifyTermReq`, `classifyBatchReq`, `batchResponse`, `dictsResponse`
Invariants: Les endpoints sont transport-agnostiques (kit.Endpoint) — le meme code sert HTTP et MCP. CORS est applique via le middleware cors(). Les MCP tools sont: classify_term, classify_batch, list_dicts.
NE PAS: Dupliquer la logique entre HTTP et MCP (utiliser les memes kit.Endpoints). Oublier le CORS sur les nouveaux endpoints.
