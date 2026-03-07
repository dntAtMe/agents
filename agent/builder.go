package agent

import (
	"github.com/dntatme/agents/prompt"
	"github.com/dntatme/agents/termination"
	"github.com/dntatme/agents/tool"
)

// Builder provides a fluent API for constructing Agent instances.
type Builder struct {
	name              string
	model             string
	systemPrompt      string
	promptBuilder     *prompt.Builder
	tools             []tool.Tool
	handoffTargets    []string
	terminationPolicy termination.Policy
	temperature       *float32
	maxOutputTokens   int32
	initialState      map[string]any
	predictor         Predictor
	hooks             *Hooks
}

// New starts building an agent with the given name and sensible defaults.
func New(name string) *Builder {
	return &Builder{
		name:              name,
		model:             "gemini-2.0-flash",
		terminationPolicy: DefaultTermination(),
	}
}

// Model sets the LLM model name.
func (b *Builder) Model(model string) *Builder {
	b.model = model
	return b
}

// SystemPrompt sets a plain-text system prompt.
func (b *Builder) SystemPrompt(s string) *Builder {
	b.systemPrompt = s
	return b
}

// PromptBuilder sets a mixin-based prompt builder (takes precedence over SystemPrompt).
func (b *Builder) PromptBuilder(pb *prompt.Builder) *Builder {
	b.promptBuilder = pb
	return b
}

// Tool adds a single tool to the agent.
func (b *Builder) Tool(t tool.Tool) *Builder {
	b.tools = append(b.tools, t)
	return b
}

// Tools adds multiple tools to the agent.
func (b *Builder) Tools(tools ...tool.Tool) *Builder {
	b.tools = append(b.tools, tools...)
	return b
}

// HandoffTo specifies agent names this agent can transfer to.
// Build() will auto-create and register a transfer_to_agent tool.
func (b *Builder) HandoffTo(targets ...string) *Builder {
	b.handoffTargets = append(b.handoffTargets, targets...)
	return b
}

// TerminationPolicy overrides the default termination policy.
func (b *Builder) TerminationPolicy(p termination.Policy) *Builder {
	b.terminationPolicy = p
	return b
}

// Temperature sets the sampling temperature.
func (b *Builder) Temperature(t float32) *Builder {
	b.temperature = &t
	return b
}

// MaxOutputTokens sets the maximum output token count.
func (b *Builder) MaxOutputTokens(n int32) *Builder {
	b.maxOutputTokens = n
	return b
}

// InitialState sets default state values merged on first run.
func (b *Builder) InitialState(s map[string]any) *Builder {
	b.initialState = s
	return b
}

// Predictor sets a custom predictor for this agent.
func (b *Builder) Predictor(p Predictor) *Builder {
	b.predictor = p
	return b
}

// Hooks sets lifecycle callbacks.
func (b *Builder) Hooks(h *Hooks) *Builder {
	b.hooks = h
	return b
}

// Build constructs the Agent with a populated tool registry.
func (b *Builder) Build() *Agent {
	reg := tool.NewRegistry()

	for _, t := range b.tools {
		reg.Register(t)
	}

	if len(b.handoffTargets) > 0 {
		reg.Register(tool.NewTransferTool(b.handoffTargets))
	}

	ag := &Agent{
		Name:              b.name,
		Model:             b.model,
		SystemPrompt:      b.systemPrompt,
		PromptBuilder:     b.promptBuilder,
		Tools:             reg,
		TerminationPolicy: b.terminationPolicy,
		Temperature:       b.temperature,
		MaxOutputTokens:   b.maxOutputTokens,
		InitialState:      b.initialState,
		Predictor:         b.predictor,
		Hooks:             b.hooks,
	}

	return ag
}
