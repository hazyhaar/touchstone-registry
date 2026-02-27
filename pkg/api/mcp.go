// CLAUDE:SUMMARY MCP tool registration exposing classify_term, classify_batch, and list_dicts as MCP-over-QUIC tools.
package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
	"github.com/hazyhaar/pkg/kit"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterMCPTools registers the three Touchstone MCP tools on the server.
func RegisterMCPTools(srv *mcp.Server, reg *dict.Registry) {
	registerMCPClassifyTerm(srv, reg)
	registerMCPClassifyBatch(srv, reg)
	registerMCPListDicts(srv, reg)
}

func registerMCPClassifyTerm(srv *mcp.Server, reg *dict.Registry) {
	tool := mcpTool("classify_term",
		"Classify a single term against public data registries (surnames, first names, companies, cities, street types).",
		map[string]any{
			"term":          map[string]string{"type": "string", "description": "The term to classify"},
			"jurisdictions": map[string]string{"type": "string", "description": "Comma-separated jurisdiction filter (e.g. fr,uk)"},
			"types":         map[string]string{"type": "string", "description": "Comma-separated entity type filter (e.g. surname,first_name)"},
			"dicts":         map[string]string{"type": "string", "description": "Comma-separated dictionary filter (e.g. patronymes-fr)"},
		},
		[]string{"term"},
	)

	endpoint := classifyTermEndpoint(reg)

	kit.RegisterMCPTool(srv, tool, endpoint, func(req *mcp.CallToolRequest) (*kit.MCPDecodeResult, error) {
		args := parseArgs(req)
		term, _ := args["term"].(string)
		return &kit.MCPDecodeResult{Request: &classifyTermReq{
			Term: term,
			Opts: parseMCPOpts(args),
		}}, nil
	})
}

func registerMCPClassifyBatch(srv *mcp.Server, reg *dict.Registry) {
	tool := mcpTool("classify_batch",
		"Classify multiple terms (up to 100) against public data registries.",
		map[string]any{
			"terms":         map[string]string{"type": "string", "description": "Comma-separated list of terms to classify (max 100)"},
			"jurisdictions": map[string]string{"type": "string", "description": "Comma-separated jurisdiction filter"},
			"types":         map[string]string{"type": "string", "description": "Comma-separated entity type filter"},
			"dicts":         map[string]string{"type": "string", "description": "Comma-separated dictionary filter (e.g. patronymes-fr)"},
		},
		[]string{"terms"},
	)

	endpoint := classifyBatchEndpoint(reg)

	kit.RegisterMCPTool(srv, tool, endpoint, func(req *mcp.CallToolRequest) (*kit.MCPDecodeResult, error) {
		args := parseArgs(req)
		termsStr, _ := args["terms"].(string)
		terms := strings.Split(termsStr, ",")
		for i := range terms {
			terms[i] = strings.TrimSpace(terms[i])
		}
		if len(terms) > 100 {
			return nil, fmt.Errorf("too many terms (max 100, got %d)", len(terms))
		}
		return &kit.MCPDecodeResult{Request: &classifyBatchReq{
			Terms: terms,
			Opts:  parseMCPOpts(args),
		}}, nil
	})
}

func registerMCPListDicts(srv *mcp.Server, reg *dict.Registry) {
	tool := mcpTool("list_dicts",
		"List all loaded dictionaries with metadata (jurisdiction, entity type, entry count, source).",
		nil,
		nil,
	)

	endpoint := listDictsEndpoint(reg)

	kit.RegisterMCPTool(srv, tool, endpoint, func(_ *mcp.CallToolRequest) (*kit.MCPDecodeResult, error) {
		return &kit.MCPDecodeResult{Request: nil}, nil
	})
}

// mcpTool builds an *mcp.Tool with a JSON Schema input schema.
func mcpTool(name, description string, properties map[string]any, required []string) *mcp.Tool {
	schema := map[string]any{
		"type": "object",
	}
	if properties != nil {
		schema["properties"] = properties
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	raw, _ := json.Marshal(schema)
	return &mcp.Tool{
		Name:        name,
		Description: description,
		InputSchema: json.RawMessage(raw),
	}
}

// parseArgs extracts the arguments map from a CallToolRequest.
func parseArgs(req *mcp.CallToolRequest) map[string]any {
	var args map[string]any
	if req.Params != nil && req.Params.Arguments != nil {
		json.Unmarshal(req.Params.Arguments, &args)
	}
	if args == nil {
		args = make(map[string]any)
	}
	return args
}

// parseMCPOpts extracts ClassifyOptions from MCP tool arguments.
func parseMCPOpts(args map[string]interface{}) *dict.ClassifyOptions {
	opts := &dict.ClassifyOptions{}
	if v, _ := args["jurisdictions"].(string); v != "" {
		opts.Jurisdictions = strings.Split(v, ",")
	}
	if v, _ := args["types"].(string); v != "" {
		opts.Types = strings.Split(v, ",")
	}
	if v, _ := args["dicts"].(string); v != "" {
		opts.Dicts = strings.Split(v, ",")
	}
	return opts
}
