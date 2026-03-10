package llm

import "context"

// Provider is the abstraction every LLM backend must implement.
type Provider interface {
	GenerateContent(ctx context.Context, model string, messages []*Content, config *GenerateConfig) (*GenerateResponse, error)
}
