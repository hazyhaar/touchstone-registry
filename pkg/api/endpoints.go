package api

import (
	"context"
	"fmt"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
	"github.com/hazyhaar/touchstone-registry/pkg/kit"
)

// Shared request/response types used by both HTTP and MCP transports.

type batchResponse struct {
	Results []*dict.ClassifyResult `json:"results"`
}

type dictsResponse struct {
	Dictionaries []dict.DictInfo `json:"dictionaries"`
}

type classifyTermReq struct {
	Term string
	Opts *dict.ClassifyOptions
}

type classifyBatchReq struct {
	Terms []string
	Opts  *dict.ClassifyOptions
}

// Endpoints returns the three core kit.Endpoints backed by the registry.

func classifyTermEndpoint(reg *dict.Registry) kit.Endpoint {
	return func(_ context.Context, request any) (any, error) {
		req := request.(*classifyTermReq)
		return reg.Classify(req.Term, req.Opts), nil
	}
}

func classifyBatchEndpoint(reg *dict.Registry) kit.Endpoint {
	return func(_ context.Context, request any) (any, error) {
		req := request.(*classifyBatchReq)
		if len(req.Terms) == 0 {
			return nil, fmt.Errorf("terms array is empty")
		}
		if len(req.Terms) > 100 {
			return nil, fmt.Errorf("too many terms (max 100, got %d)", len(req.Terms))
		}
		results := make([]*dict.ClassifyResult, len(req.Terms))
		for i, term := range req.Terms {
			results[i] = reg.Classify(term, req.Opts)
		}
		return batchResponse{Results: results}, nil
	}
}

func listDictsEndpoint(reg *dict.Registry) kit.Endpoint {
	return func(_ context.Context, _ any) (any, error) {
		return dictsResponse{Dictionaries: reg.ListDicts()}, nil
	}
}
