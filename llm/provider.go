package llm

import "context"

// Provider is the abstraction every LLM backend must implement.
type Provider interface {
	GenerateContent(ctx context.Context, model string, messages []*Content, config *GenerateConfig) (*GenerateResponse, error)
}

// GoogleSearchSource is a single grounded web source returned from Google search.
type GoogleSearchSource struct {
	Title  string
	URL    string
	Domain string
}

// GoogleSearchResult is the structured output returned by providers that support
// Google-grounded web research.
type GoogleSearchResult struct {
	Summary       string
	Sources       []GoogleSearchSource
	SearchQueries []string
}

// GoogleSearchProvider is implemented by providers that can run Google-grounded
// web research queries.
type GoogleSearchProvider interface {
	GoogleSearch(ctx context.Context, model string, query string) (*GoogleSearchResult, error)
}
