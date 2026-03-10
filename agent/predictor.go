package agent

import (
	"context"

	"github.com/dntatme/agents/llm"
)

// PredictRequest holds the inputs for a prediction call.
type PredictRequest struct {
	Model    string
	Messages []*llm.Content
	Config   *llm.GenerateConfig
}

// Predictor generates model content from messages and config.
// Implement this interface to provide custom prediction logic (e.g., cache, mock, different provider).
type Predictor interface {
	Predict(ctx context.Context, req PredictRequest) (*llm.GenerateResponse, error)
}

// LLMPredictor wraps an LLM provider to implement Predictor.
type LLMPredictor struct {
	provider llm.Provider
}

// NewLLMPredictor creates a Predictor from an LLM provider.
func NewLLMPredictor(provider llm.Provider) Predictor {
	return &LLMPredictor{provider: provider}
}

// Predict delegates to the underlying LLM provider.
func (p *LLMPredictor) Predict(ctx context.Context, req PredictRequest) (*llm.GenerateResponse, error) {
	return p.provider.GenerateContent(ctx, req.Model, req.Messages, req.Config)
}
