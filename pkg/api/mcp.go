package api

import (
	"context"
	"strings"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
	"github.com/hazyhaar/touchstone-registry/pkg/kit"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterMCPTools registers the three Touchstone MCP tools on the server.
func RegisterMCPTools(srv *server.MCPServer, reg *dict.Registry) {
	registerClassifyTerm(srv, reg)
	registerClassifyBatch(srv, reg)
	registerListDicts(srv, reg)
}

func registerClassifyTerm(srv *server.MCPServer, reg *dict.Registry) {
	tool := mcp.NewTool("classify_term",
		mcp.WithDescription("Classify a single term against public data registries (surnames, first names, companies, cities, street types)."),
		mcp.WithString("term", mcp.Required(), mcp.Description("The term to classify")),
		mcp.WithString("jurisdictions", mcp.Description("Comma-separated jurisdiction filter (e.g. fr,uk)")),
		mcp.WithString("types", mcp.Description("Comma-separated entity type filter (e.g. surname,first_name)")),
		mcp.WithString("dicts", mcp.Description("Comma-separated dictionary filter (e.g. patronymes-fr)")),
	)

	kit.RegisterMCPTool(srv, tool, func(_ context.Context, request any) (any, error) {
		req := request.(*classifyTermReq)
		return reg.Classify(req.Term, req.Opts), nil
	}, func(req mcp.CallToolRequest) (*kit.MCPDecodeResult, error) {
		args := req.GetArguments()
		term, _ := args["term"].(string)
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
		return &kit.MCPDecodeResult{Request: &classifyTermReq{Term: term, Opts: opts}}, nil
	})
}

func registerClassifyBatch(srv *server.MCPServer, reg *dict.Registry) {
	tool := mcp.NewTool("classify_batch",
		mcp.WithDescription("Classify multiple terms (up to 100) against public data registries."),
		mcp.WithString("terms", mcp.Required(), mcp.Description("Comma-separated list of terms to classify")),
		mcp.WithString("jurisdictions", mcp.Description("Comma-separated jurisdiction filter")),
		mcp.WithString("types", mcp.Description("Comma-separated entity type filter")),
	)

	kit.RegisterMCPTool(srv, tool, func(_ context.Context, request any) (any, error) {
		req := request.(*classifyBatchReq)
		results := make([]*dict.ClassifyResult, len(req.Terms))
		for i, term := range req.Terms {
			results[i] = reg.Classify(term, req.Opts)
		}
		return batchResponse{Results: results}, nil
	}, func(req mcp.CallToolRequest) (*kit.MCPDecodeResult, error) {
		args := req.GetArguments()
		termsStr, _ := args["terms"].(string)
		terms := strings.Split(termsStr, ",")
		for i := range terms {
			terms[i] = strings.TrimSpace(terms[i])
		}
		opts := &dict.ClassifyOptions{}
		if v, _ := args["jurisdictions"].(string); v != "" {
			opts.Jurisdictions = strings.Split(v, ",")
		}
		if v, _ := args["types"].(string); v != "" {
			opts.Types = strings.Split(v, ",")
		}
		return &kit.MCPDecodeResult{Request: &classifyBatchReq{Terms: terms, Opts: opts}}, nil
	})
}

func registerListDicts(srv *server.MCPServer, reg *dict.Registry) {
	tool := mcp.NewTool("list_dicts",
		mcp.WithDescription("List all loaded dictionaries with metadata (jurisdiction, entity type, entry count, source)."),
	)

	kit.RegisterMCPTool(srv, tool, func(_ context.Context, _ any) (any, error) {
		return dictsResponse{Dictionaries: reg.ListDicts()}, nil
	}, func(_ mcp.CallToolRequest) (*kit.MCPDecodeResult, error) {
		return &kit.MCPDecodeResult{Request: nil}, nil
	})
}

type classifyTermReq struct {
	Term string
	Opts *dict.ClassifyOptions
}

type classifyBatchReq struct {
	Terms []string
	Opts  *dict.ClassifyOptions
}
