package api

import (
	"fmt"
	"strings"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
	"github.com/hazyhaar/pkg/kit"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterMCPTools registers the three Touchstone MCP tools on the server.
func RegisterMCPTools(srv *server.MCPServer, reg *dict.Registry) {
	registerMCPClassifyTerm(srv, reg)
	registerMCPClassifyBatch(srv, reg)
	registerMCPListDicts(srv, reg)
}

func registerMCPClassifyTerm(srv *server.MCPServer, reg *dict.Registry) {
	tool := mcp.NewTool("classify_term",
		mcp.WithDescription("Classify a single term against public data registries (surnames, first names, companies, cities, street types)."),
		mcp.WithString("term", mcp.Required(), mcp.Description("The term to classify")),
		mcp.WithString("jurisdictions", mcp.Description("Comma-separated jurisdiction filter (e.g. fr,uk)")),
		mcp.WithString("types", mcp.Description("Comma-separated entity type filter (e.g. surname,first_name)")),
		mcp.WithString("dicts", mcp.Description("Comma-separated dictionary filter (e.g. patronymes-fr)")),
	)

	endpoint := classifyTermEndpoint(reg)

	kit.RegisterMCPTool(srv, tool, endpoint, func(req mcp.CallToolRequest) (*kit.MCPDecodeResult, error) {
		args := req.GetArguments()
		term, _ := args["term"].(string)
		return &kit.MCPDecodeResult{Request: &classifyTermReq{
			Term: term,
			Opts: parseMCPOpts(args),
		}}, nil
	})
}

func registerMCPClassifyBatch(srv *server.MCPServer, reg *dict.Registry) {
	tool := mcp.NewTool("classify_batch",
		mcp.WithDescription("Classify multiple terms (up to 100) against public data registries."),
		mcp.WithString("terms", mcp.Required(), mcp.Description("Comma-separated list of terms to classify (max 100)")),
		mcp.WithString("jurisdictions", mcp.Description("Comma-separated jurisdiction filter")),
		mcp.WithString("types", mcp.Description("Comma-separated entity type filter")),
		mcp.WithString("dicts", mcp.Description("Comma-separated dictionary filter (e.g. patronymes-fr)")),
	)

	endpoint := classifyBatchEndpoint(reg)

	kit.RegisterMCPTool(srv, tool, endpoint, func(req mcp.CallToolRequest) (*kit.MCPDecodeResult, error) {
		args := req.GetArguments()
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

func registerMCPListDicts(srv *server.MCPServer, reg *dict.Registry) {
	tool := mcp.NewTool("list_dicts",
		mcp.WithDescription("List all loaded dictionaries with metadata (jurisdiction, entity type, entry count, source)."),
	)

	endpoint := listDictsEndpoint(reg)

	kit.RegisterMCPTool(srv, tool, endpoint, func(_ mcp.CallToolRequest) (*kit.MCPDecodeResult, error) {
		return &kit.MCPDecodeResult{Request: nil}, nil
	})
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
