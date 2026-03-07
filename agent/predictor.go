package agent

import (
	"context"

	"google.golang.org/genai"

	"github.com/kacperpaczos/agents/llm"
)

// PredictRequest holds the inputs for a prediction call.
type PredictRequest struct {
	Model   string
	Messages []*genai.Content
	Config  *genai.GenerateContentConfig
}

// Predictor generates model content from messages and config.
// Implement this interface to provide custom prediction logic (e.g., cache, mock, different provider).
type Predictor interface {
	Predict(ctx context.Context, req PredictRequest) (*genai.GenerateContentResponse, error)
}

// LLMPredictor wraps an LLM client to implement Predictor.
type LLMPredictor struct {
	client *llm.Client
}

// NewLLMPredictor creates a Predictor from an LLM client.
func NewLLMPredictor(client *llm.Client) Predictor {
	return &LLMPredictor{client: client}
}

// Predict delegates to the underlying LLM client.
func (p *LLMPredictor) Predict(ctx context.Context, req PredictRequest) (*genai.GenerateContentResponse, error) {
	return p.client.GenerateContent(ctx, req.Model, req.Messages, req.Config)
}
