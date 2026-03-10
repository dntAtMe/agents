package llm

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// GeminiProvider implements Provider using the Gemini API.
type GeminiProvider struct {
	inner *genai.Client
}

// NewGemini creates a Gemini provider using the provided API key.
func NewGemini(ctx context.Context, apiKey string) (*GeminiProvider, error) {
	c, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("genai.NewClient: %w", err)
	}
	return &GeminiProvider{inner: c}, nil
}

// GenerateContent converts llm types to genai types, calls the API, and converts back.
func (g *GeminiProvider) GenerateContent(
	ctx context.Context,
	model string,
	messages []*Content,
	config *GenerateConfig,
) (*GenerateResponse, error) {
	genaiMessages := toGenaiContents(messages)
	genaiConfig := toGenaiConfig(config)

	resp, err := g.inner.Models.GenerateContent(ctx, model, genaiMessages, genaiConfig)
	if err != nil {
		return nil, err
	}

	return fromGenaiResponse(resp), nil
}

// --- Conversion: llm -> genai ---

func toGenaiContents(contents []*Content) []*genai.Content {
	if contents == nil {
		return nil
	}
	out := make([]*genai.Content, len(contents))
	for i, c := range contents {
		out[i] = toGenaiContent(c)
	}
	return out
}

func toGenaiContent(c *Content) *genai.Content {
	if c == nil {
		return nil
	}
	gc := &genai.Content{
		Role:  c.Role,
		Parts: make([]*genai.Part, len(c.Parts)),
	}
	for i, p := range c.Parts {
		gc.Parts[i] = toGenaiPart(p)
	}
	return gc
}

func toGenaiPart(p *Part) *genai.Part {
	gp := &genai.Part{}
	if p.Text != "" {
		gp.Text = p.Text
	}
	if p.FunctionCall != nil {
		gp.FunctionCall = &genai.FunctionCall{
			Name: p.FunctionCall.Name,
			Args: p.FunctionCall.Args,
		}
	}
	if p.FunctionResponse != nil {
		gp.FunctionResponse = &genai.FunctionResponse{
			Name:     p.FunctionResponse.Name,
			Response: p.FunctionResponse.Response,
		}
	}
	return gp
}

func toGenaiConfig(config *GenerateConfig) *genai.GenerateContentConfig {
	if config == nil {
		return nil
	}
	gc := &genai.GenerateContentConfig{
		Temperature:     config.Temperature,
		MaxOutputTokens: config.MaxOutputTokens,
	}
	if config.SystemInstruction != nil {
		gc.SystemInstruction = toGenaiContent(config.SystemInstruction)
	}
	if len(config.Tools) > 0 {
		gc.Tools = make([]*genai.Tool, len(config.Tools))
		for i, ts := range config.Tools {
			gc.Tools[i] = toGenaiToolSet(ts)
		}
	}
	return gc
}

func toGenaiToolSet(ts *ToolSet) *genai.Tool {
	gt := &genai.Tool{
		FunctionDeclarations: make([]*genai.FunctionDeclaration, len(ts.FunctionDeclarations)),
	}
	for i, fd := range ts.FunctionDeclarations {
		gt.FunctionDeclarations[i] = toGenaiFuncDecl(fd)
	}
	return gt
}

func toGenaiFuncDecl(fd *FunctionDeclaration) *genai.FunctionDeclaration {
	gfd := &genai.FunctionDeclaration{
		Name:        fd.Name,
		Description: fd.Description,
	}
	if fd.Parameters != nil {
		gfd.Parameters = toGenaiSchema(fd.Parameters)
	}
	return gfd
}

func toGenaiSchema(s *Schema) *genai.Schema {
	if s == nil {
		return nil
	}
	gs := &genai.Schema{
		Type:        toGenaiType(s.Type),
		Description: s.Description,
		Required:    s.Required,
		Enum:        s.Enum,
	}
	if s.Items != nil {
		gs.Items = toGenaiSchema(s.Items)
	}
	if len(s.Properties) > 0 {
		gs.Properties = make(map[string]*genai.Schema, len(s.Properties))
		for k, v := range s.Properties {
			gs.Properties[k] = toGenaiSchema(v)
		}
	}
	return gs
}

func toGenaiType(t Type) genai.Type {
	switch t {
	case TypeString:
		return genai.TypeString
	case TypeNumber:
		return genai.TypeNumber
	case TypeInteger:
		return genai.TypeInteger
	case TypeBoolean:
		return genai.TypeBoolean
	case TypeObject:
		return genai.TypeObject
	case TypeArray:
		return genai.TypeArray
	default:
		return genai.TypeString
	}
}

// --- Conversion: genai -> llm ---

func fromGenaiResponse(resp *genai.GenerateContentResponse) *GenerateResponse {
	if resp == nil {
		return &GenerateResponse{}
	}
	r := &GenerateResponse{}
	if resp.UsageMetadata != nil {
		r.UsageMetadata = &UsageMetadata{
			TotalTokenCount: resp.UsageMetadata.TotalTokenCount,
		}
	}
	if len(resp.Candidates) > 0 {
		r.Candidates = make([]*Candidate, len(resp.Candidates))
		for i, c := range resp.Candidates {
			r.Candidates[i] = &Candidate{
				Content: fromGenaiContent(c.Content),
			}
		}
	}
	return r
}

func fromGenaiContent(c *genai.Content) *Content {
	if c == nil {
		return nil
	}
	content := &Content{
		Role:  c.Role,
		Parts: make([]*Part, len(c.Parts)),
	}
	for i, p := range c.Parts {
		content.Parts[i] = fromGenaiPart(p)
	}
	return content
}

func fromGenaiPart(p *genai.Part) *Part {
	part := &Part{}
	if p.Text != "" {
		part.Text = p.Text
	}
	if p.FunctionCall != nil {
		part.FunctionCall = &FunctionCall{
			Name: p.FunctionCall.Name,
			Args: p.FunctionCall.Args,
		}
	}
	if p.FunctionResponse != nil {
		part.FunctionResponse = &FunctionResponse{
			Name:     p.FunctionResponse.Name,
			Response: p.FunctionResponse.Response,
		}
	}
	return part
}
