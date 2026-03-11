package llm

// Type constants for Schema (mirrors genai.Type*).
type Type int

const (
	TypeString  Type = 1
	TypeNumber  Type = 2
	TypeInteger Type = 3
	TypeBoolean Type = 4
	TypeObject  Type = 6
	TypeArray   Type = 7
)

// Schema describes a JSON schema for tool parameters.
type Schema struct {
	Type        Type
	Description string
	Properties  map[string]*Schema
	Required    []string
	Enum        []string
	Items       *Schema // for arrays
}

// FunctionDeclaration describes a tool the model can call.
type FunctionDeclaration struct {
	Name        string
	Description string
	Parameters  *Schema
}

// ToolSet groups function declarations for the model.
type ToolSet struct {
	FunctionDeclarations []*FunctionDeclaration
}

// FunctionCall represents a model's request to call a function.
type FunctionCall struct {
	Name string
	Args map[string]any
}

// FunctionResponse carries the result of a function call.
type FunctionResponse struct {
	Name     string
	Response map[string]any
}

// Part is a single piece of content (text, function call, or function response).
type Part struct {
	Text             string
	FunctionCall     *FunctionCall
	FunctionResponse *FunctionResponse
}

// Content is a message in a conversation.
type Content struct {
	Role  string
	Parts []*Part
}

// ToolMode specifies how the model should use tools.
type ToolMode string

const (
	// ToolModeAuto lets the model decide whether to call tools or generate text.
	ToolModeAuto ToolMode = "AUTO"
	// ToolModeAny forces the model to call at least one tool (function call-only mode).
	ToolModeAny ToolMode = "ANY"
	// ToolModeNone prevents the model from calling any tools.
	ToolModeNone ToolMode = "NONE"
)

// ToolConfig configures tool calling behavior.
type ToolConfig struct {
	Mode ToolMode
}

// ThinkingConfig configures thinking/reasoning features.
type ThinkingConfig struct {
	// IncludeThoughts indicates whether to include thoughts in the response.
	IncludeThoughts bool
	// ThinkingBudget is the optional thinking budget in tokens.
	ThinkingBudget *int32
}

// GenerateConfig holds generation parameters.
type GenerateConfig struct {
	SystemInstruction *Content
	Temperature       *float32
	MaxOutputTokens   int32
	Tools             []*ToolSet
	ToolConfig        *ToolConfig
	ThinkingConfig    *ThinkingConfig
}

// Candidate is one generation result.
type Candidate struct {
	Content *Content
}

// UsageMetadata tracks token usage.
type UsageMetadata struct {
	TotalTokenCount int32
}

// GenerateResponse holds the model's response.
type GenerateResponse struct {
	Candidates    []*Candidate
	UsageMetadata *UsageMetadata
}
