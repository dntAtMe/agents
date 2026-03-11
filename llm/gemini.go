package llm

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"
)

// GeminiProvider implements Provider using the Gemini API.
type GeminiProvider struct {
	inner        *genai.Client
	defaultModel string
}

// NewGemini creates a Gemini provider using the provided API key.
// The default model is read from GEMINI_MODEL env var, falling back to "gemini-2.0-flash".
func NewGemini(ctx context.Context, apiKey string) (*GeminiProvider, error) {
	c, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("genai.NewClient: %w", err)
	}
	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &GeminiProvider{
		inner:        c,
		defaultModel: model,
	}, nil
}

// GenerateContent converts llm types to genai types, calls the API, and converts back.
func (g *GeminiProvider) GenerateContent(
	ctx context.Context,
	model string,
	messages []*Content,
	config *GenerateConfig,
) (*GenerateResponse, error) {
	if model == "" {
		model = g.defaultModel
	}

	genaiMessages := toGenaiContents(messages)
	genaiConfig := toGenaiConfig(config)

	// Gemini 2.5+ models enable thinking by default. When the caller did
	// not request thinking, explicitly disable it so we don't burn tokens.
	// We can't send ThinkingConfig to 2.0 models (API rejects it).
	if genaiConfig.ThinkingConfig == nil && modelSupportsThinking(model) {
		budget := int32(0)
		genaiConfig.ThinkingConfig = &genai.ThinkingConfig{
			ThinkingBudget: &budget,
		}
	}

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
	if config.ToolConfig != nil {
		gc.ToolConfig = toGenaiToolConfig(config.ToolConfig)
	}
	if config.ThinkingConfig != nil {
		gc.ThinkingConfig = toGenaiThinkingConfig(config.ThinkingConfig)
	}
	return gc
}

func toGenaiThinkingConfig(tc *ThinkingConfig) *genai.ThinkingConfig {
	if tc == nil {
		return nil
	}
	return &genai.ThinkingConfig{
		IncludeThoughts: tc.IncludeThoughts,
		ThinkingBudget:  tc.ThinkingBudget,
	}
}

func toGenaiToolConfig(tc *ToolConfig) *genai.ToolConfig {
	if tc == nil {
		return nil
	}
	return &genai.ToolConfig{
		FunctionCallingConfig: &genai.FunctionCallingConfig{
			Mode: toGenaiFunctionCallingMode(tc.Mode),
		},
	}
}

func toGenaiFunctionCallingMode(mode ToolMode) genai.FunctionCallingConfigMode {
	switch mode {
	case ToolModeAny:
		return genai.FunctionCallingConfigModeAny
	case ToolModeNone:
		return genai.FunctionCallingConfigModeNone
	case ToolModeAuto:
		return genai.FunctionCallingConfigModeAuto
	default:
		return genai.FunctionCallingConfigModeAuto
	}
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

// modelSupportsThinking returns true for Gemini models that enable thinking
// by default (2.5+). These models need an explicit ThinkingBudget=0 to disable.
func modelSupportsThinking(model string) bool {
	return strings.Contains(model, "-2.5-") || strings.Contains(model, "-2.5")
}
