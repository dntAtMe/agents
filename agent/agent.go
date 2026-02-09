package agent

import (
	"github.com/kacperpaczos/agents/prompt"
	"github.com/kacperpaczos/agents/termination"
	"github.com/kacperpaczos/agents/tool"
)

// Agent defines a single LLM-backed agent.
type Agent struct {
	Name              string
	Model             string // e.g. "gemini-2.0-flash"
	SystemPrompt      string          // plain string prompt (used if PromptBuilder is nil)
	PromptBuilder     *prompt.Builder // mixin-based prompt (takes precedence over SystemPrompt)
	Tools             *tool.Registry
	TerminationPolicy termination.Policy
	Temperature       *float32
	MaxOutputTokens   int32
}

// ResolveSystemPrompt returns the effective system prompt.
// PromptBuilder takes precedence; falls back to SystemPrompt.
func (a *Agent) ResolveSystemPrompt() string {
	if a.PromptBuilder != nil {
		return a.PromptBuilder.Build()
	}
	return a.SystemPrompt
}

// DefaultTermination returns the standard termination policy:
// stop on done signal, max 20 iterations, context timeout, or 100k token budget.
func DefaultTermination() termination.Policy {
	return termination.Any{
		Policies: []termination.Policy{
			termination.DoneSignal{},
			termination.MaxIterations{Max: 20},
			termination.Timeout{},
			termination.TokenBudget{MaxTokens: 100_000},
		},
	}
}
