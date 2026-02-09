package llm

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// Client is a thin wrapper around the Gemini genai.Client.
type Client struct {
	inner *genai.Client
}

// New creates a Gemini client using the provided API key.
func New(ctx context.Context, apiKey string) (*Client, error) {
	c, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("genai.NewClient: %w", err)
	}
	return &Client{inner: c}, nil
}

// GenerateContent calls the Gemini model.
func (c *Client) GenerateContent(
	ctx context.Context,
	model string,
	messages []*genai.Content,
	config *genai.GenerateContentConfig,
) (*genai.GenerateContentResponse, error) {
	return c.inner.Models.GenerateContent(ctx, model, messages, config)
}
