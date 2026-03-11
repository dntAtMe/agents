package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// OllamaProvider implements Provider using Ollama's OpenAI-compatible API.
type OllamaProvider struct {
	baseURL      string
	defaultModel string
	client       *http.Client
}

// NewOllama creates an Ollama provider.
// baseURL is the Ollama server URL (e.g. "http://localhost:11434").
// defaultModel is used when the model parameter is empty.
func NewOllama(baseURL, defaultModel string) *OllamaProvider {
	return &OllamaProvider{
		baseURL:      strings.TrimRight(baseURL, "/"),
		defaultModel: defaultModel,
		client:       &http.Client{},
	}
}

// GenerateContent converts llm types to OpenAI chat format, calls Ollama, and converts back.
func (o *OllamaProvider) GenerateContent(
	ctx context.Context,
	model string,
	messages []*Content,
	config *GenerateConfig,
) (*GenerateResponse, error) {
	if model == "" {
		model = o.defaultModel
	}

	reqBody := o.buildRequest(model, messages, config)

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal request: %w", err)
	}

	url := o.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("ollama: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: send request: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("ollama: read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: HTTP %d: %s", httpResp.StatusCode, string(body))
	}

	var respBody oaiChatResponse
	if err := json.Unmarshal(body, &respBody); err != nil {
		return nil, fmt.Errorf("ollama: unmarshal response: %w", err)
	}

	return o.parseResponse(respBody), nil
}

// --- OpenAI-compatible request/response types ---

type oaiChatRequest struct {
	Model       string       `json:"model"`
	Messages    []oaiMessage `json:"messages"`
	Tools       []oaiTool    `json:"tools,omitempty"`
	ToolChoice  any          `json:"tool_choice,omitempty"`
	Temperature *float32     `json:"temperature,omitempty"`
	MaxTokens   int32        `json:"max_tokens,omitempty"`
}

type oaiMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type oaiToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function oaiFunctionCall `json:"function"`
}

type oaiFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

type oaiTool struct {
	Type     string          `json:"type"`
	Function oaiToolFunction `json:"function"`
}

type oaiToolFunction struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  *oaiSchema `json:"parameters,omitempty"`
}

type oaiSchema struct {
	Type        string                `json:"type"`
	Description string                `json:"description,omitempty"`
	Properties  map[string]*oaiSchema `json:"properties,omitempty"`
	Required    []string              `json:"required,omitempty"`
	Enum        []string              `json:"enum,omitempty"`
	Items       *oaiSchema            `json:"items,omitempty"`
}

type oaiChatResponse struct {
	Choices []oaiChoice `json:"choices"`
	Usage   *oaiUsage   `json:"usage,omitempty"`
}

type oaiChoice struct {
	Message oaiMessage `json:"message"`
}

type oaiUsage struct {
	PromptTokens int32 `json:"prompt_tokens"`
	TotalTokens  int32 `json:"total_tokens"`
}

// --- Build request ---

func (o *OllamaProvider) buildRequest(model string, messages []*Content, config *GenerateConfig) oaiChatRequest {
	req := oaiChatRequest{
		Model: model,
	}

	// System instruction
	if config != nil && config.SystemInstruction != nil {
		text := extractContentText(config.SystemInstruction)
		if text != "" {
			req.Messages = append(req.Messages, oaiMessage{
				Role:    "system",
				Content: text,
			})
		}
	}

	// Convert messages
	for _, msg := range messages {
		oaiMsgs := o.contentToMessages(msg)
		req.Messages = append(req.Messages, oaiMsgs...)
	}

	// Tools
	if config != nil {
		for _, ts := range config.Tools {
			for _, fd := range ts.FunctionDeclarations {
				req.Tools = append(req.Tools, oaiTool{
					Type: "function",
					Function: oaiToolFunction{
						Name:        fd.Name,
						Description: fd.Description,
						Parameters:  schemaToOAI(fd.Parameters),
					},
				})
			}
		}

		req.Temperature = config.Temperature
		req.MaxTokens = config.MaxOutputTokens

		if config.ToolConfig != nil && len(req.Tools) > 0 {
			req.ToolChoice = toolModeToOAI(config.ToolConfig.Mode)
		}
		if config.ThinkingConfig != nil {
			log.Println("ollama: ThinkingConfig is not supported by Ollama and will be ignored")
		}
	}

	return req
}

func (o *OllamaProvider) contentToMessages(c *Content) []oaiMessage {
	if c == nil {
		return nil
	}

	role := c.Role
	if role == "" {
		role = "user"
	}

	// Check if this content has function calls (model response with tool calls)
	var toolCalls []oaiToolCall
	var textParts []string
	var funcResponses []*FunctionResponse

	for _, p := range c.Parts {
		if p.Text != "" {
			textParts = append(textParts, p.Text)
		}
		if p.FunctionCall != nil {
			argsJSON, _ := json.Marshal(p.FunctionCall.Args)
			toolCalls = append(toolCalls, oaiToolCall{
				ID:   "call_" + p.FunctionCall.Name,
				Type: "function",
				Function: oaiFunctionCall{
					Name:      p.FunctionCall.Name,
					Arguments: string(argsJSON),
				},
			})
		}
		if p.FunctionResponse != nil {
			funcResponses = append(funcResponses, p.FunctionResponse)
		}
	}

	// If we have function responses, emit them as tool messages
	if len(funcResponses) > 0 {
		var msgs []oaiMessage
		for _, fr := range funcResponses {
			respJSON, _ := json.Marshal(fr.Response)
			msgs = append(msgs, oaiMessage{
				Role:       "tool",
				Content:    string(respJSON),
				ToolCallID: "call_" + fr.Name,
			})
		}
		return msgs
	}

	// If we have tool calls, this is an assistant message with tool calls
	if len(toolCalls) > 0 {
		return []oaiMessage{{
			Role:      "assistant",
			Content:   strings.Join(textParts, "\n"),
			ToolCalls: toolCalls,
		}}
	}

	// Plain text message
	return []oaiMessage{{
		Role:    role,
		Content: strings.Join(textParts, "\n"),
	}}
}

// --- Parse response ---

func (o *OllamaProvider) parseResponse(resp oaiChatResponse) *GenerateResponse {
	result := &GenerateResponse{}

	if resp.Usage != nil {
		result.UsageMetadata = &UsageMetadata{
			PromptTokenCount:   resp.Usage.PromptTokens,
			ResponseTokenCount: resp.Usage.TotalTokens - resp.Usage.PromptTokens,
			TotalTokenCount:    resp.Usage.TotalTokens,
		}
	}

	for _, choice := range resp.Choices {
		var parts []*Part

		if choice.Message.Content != "" {
			parts = append(parts, &Part{Text: choice.Message.Content})
		}

		for _, tc := range choice.Message.ToolCalls {
			var args map[string]any
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
			parts = append(parts, &Part{
				FunctionCall: &FunctionCall{
					Name: tc.Function.Name,
					Args: args,
				},
			})
		}

		role := choice.Message.Role
		if role == "" {
			role = "model"
		}

		result.Candidates = append(result.Candidates, &Candidate{
			Content: &Content{
				Role:  role,
				Parts: parts,
			},
		})
	}

	return result
}

// --- Helpers ---

func extractContentText(c *Content) string {
	if c == nil {
		return ""
	}
	var parts []string
	for _, p := range c.Parts {
		if p.Text != "" {
			parts = append(parts, p.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func schemaToOAI(s *Schema) *oaiSchema {
	if s == nil {
		return nil
	}
	os := &oaiSchema{
		Type:        typeToString(s.Type),
		Description: s.Description,
		Required:    s.Required,
		Enum:        s.Enum,
	}
	if s.Items != nil {
		os.Items = schemaToOAI(s.Items)
	}
	if len(s.Properties) > 0 {
		os.Properties = make(map[string]*oaiSchema, len(s.Properties))
		for k, v := range s.Properties {
			os.Properties[k] = schemaToOAI(v)
		}
	}
	return os
}

func toolModeToOAI(mode ToolMode) string {
	switch mode {
	case ToolModeAny:
		return "required"
	case ToolModeNone:
		return "none"
	case ToolModeAuto:
		return "auto"
	default:
		return "auto"
	}
}

func typeToString(t Type) string {
	switch t {
	case TypeString:
		return "string"
	case TypeNumber:
		return "number"
	case TypeInteger:
		return "integer"
	case TypeBoolean:
		return "boolean"
	case TypeObject:
		return "object"
	case TypeArray:
		return "array"
	default:
		return "string"
	}
}
