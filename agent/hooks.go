package agent

import (
	"context"

	"github.com/dntatme/agents/conversation"
	"github.com/dntatme/agents/llm"
)

// HookContext is passed to all hooks with read/write access to the current run.
// Hooks can mutate State and Conversation in place. Agent is read-only.
type HookContext struct {
	Agent        *Agent
	Conversation *conversation.Conversation
	State        map[string]any
	Iteration    int
	TotalTokens  int32
}

// Hooks provides optional lifecycle callbacks for agent customization.
// Only non-nil hooks are invoked.
type Hooks struct {
	// BeforePredict runs before each Predictor call. Can mutate req.Messages and req.Config in place.
	BeforePredict func(ctx context.Context, hc *HookContext, req *PredictRequest) error
	// AfterPredict runs after prediction, before termination check.
	AfterPredict func(ctx context.Context, hc *HookContext, content *llm.Content) error
	// BeforeToolCall runs before each tool execution.
	BeforeToolCall func(ctx context.Context, hc *HookContext, fc *llm.FunctionCall) error
	// AfterToolCall runs after each tool execution.
	AfterToolCall func(ctx context.Context, hc *HookContext, fc *llm.FunctionCall, result map[string]any) error
	// AfterToolCalls runs after all tools in the batch, before appending to conversation.
	// Can return modified resultParts (e.g., to filter or transform).
	AfterToolCalls func(ctx context.Context, hc *HookContext, resultParts []*llm.Part) ([]*llm.Part, error)
}
