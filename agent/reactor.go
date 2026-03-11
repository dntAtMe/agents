package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/dntatme/agents/conversation"
	"github.com/dntatme/agents/llm"
	"github.com/dntatme/agents/termination"
	"github.com/dntatme/agents/tool"
)

// HandoffResult is returned when the model calls transfer_to_agent.
type HandoffResult struct {
	TargetAgent string
	Reason      string
}

// RunResult is the outcome of a single agent run.
type RunResult struct {
	FinalText       string
	Handoff         *HandoffResult
	Conversation    *conversation.Conversation
	TotalTokens     int32
	Iterations      int
	TerminateReason string
	State           map[string]any
}

// Run executes the ReACT loop for one agent.
// predictor is the default predictor; if ag.Predictor is set, it overrides.
func Run(ctx context.Context, predictor Predictor, ag *Agent, conv *conversation.Conversation, state map[string]any) (*RunResult, error) {
	var totalTokens int32
	iteration := 0

	// Select predictor: agent's override or default.
	pred := predictor
	if ag.Predictor != nil {
		pred = ag.Predictor
	}

	// Build base config.
	config := &llm.GenerateConfig{
		SystemInstruction: &llm.Content{
			Parts: []*llm.Part{{Text: ag.ResolveSystemPrompt()}},
		},
		Temperature:     ag.Temperature,
		MaxOutputTokens: ag.MaxOutputTokens,
	}

	// Attach tool definitions if any.
	if ag.Tools != nil && ag.Tools.Len() > 0 {
		config.Tools = []*llm.ToolSet{
			{FunctionDeclarations: ag.Tools.Definitions()},
		}
	}

	// Attach tool config if specified.
	if ag.ToolMode != nil {
		config.ToolConfig = &llm.ToolConfig{
			Mode: *ag.ToolMode,
		}
	}

	// Attach thinking config if enabled.
	if ag.ThinkingEnabled {
		config.ThinkingConfig = &llm.ThinkingConfig{
			IncludeThoughts: true,
		}
	}

	for {
		iteration++

		// 1. Build predict request.
		req := PredictRequest{
			Model:    ag.Model,
			Messages: conv.Messages,
			Config:   config,
		}

		// 2. BeforePredict hook (can mutate req).
		if ag.Hooks != nil && ag.Hooks.BeforePredict != nil {
			hc := &HookContext{
				Agent:        ag,
				Conversation: conv,
				State:        state,
				Iteration:    iteration,
				TotalTokens:  totalTokens,
			}
			if err := ag.Hooks.BeforePredict(ctx, hc, &req); err != nil {
				return nil, fmt.Errorf("BeforePredict (iter %d): %w", iteration, err)
			}
		}

		// 3. Call predictor.
		resp, err := pred.Predict(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("Predict (iter %d): %w", iteration, err)
		}

		// 4. Track token usage.
		if resp.UsageMetadata != nil {
			totalTokens += resp.UsageMetadata.TotalTokenCount
		}

		// 5. Append model response.
		if len(resp.Candidates) == 0 {
			return &RunResult{
				FinalText:       "",
				Conversation:    conv,
				TotalTokens:     totalTokens,
				Iterations:      iteration,
				TerminateReason: "no candidates in response",
				State:           state,
			}, nil
		}
		modelContent := resp.Candidates[0].Content

		// 6. AfterPredict hook.
		if ag.Hooks != nil && ag.Hooks.AfterPredict != nil {
			hc := &HookContext{
				Agent:        ag,
				Conversation: conv,
				State:        state,
				Iteration:    iteration,
				TotalTokens:  totalTokens,
			}
			if err := ag.Hooks.AfterPredict(ctx, hc, modelContent); err != nil {
				return nil, fmt.Errorf("AfterPredict (iter %d): %w", iteration, err)
			}
		}

		conv.AppendModelContent(modelContent)

		// 7. Extract function calls.
		var funcCalls []*llm.FunctionCall
		for _, part := range modelContent.Parts {
			if part.FunctionCall != nil {
				funcCalls = append(funcCalls, part.FunctionCall)
			}
		}

		hasToolCalls := len(funcCalls) > 0

		// Check termination policy.
		if ag.TerminationPolicy != nil {
			tstate := termination.State{
				Iteration:       iteration,
				TotalTokensUsed: totalTokens,
				LastResponse:    resp,
				HasToolCalls:    hasToolCalls,
			}
			if stop, reason := ag.TerminationPolicy.ShouldTerminate(ctx, tstate); stop {
				return &RunResult{
					FinalText:       extractText(modelContent),
					Conversation:    conv,
					TotalTokens:     totalTokens,
					Iterations:      iteration,
					TerminateReason: reason,
					State:           state,
				}, nil
			}
		}

		// No function calls → return (safety net if DoneSignal not configured).
		if !hasToolCalls {
			return &RunResult{
				FinalText:       extractText(modelContent),
				Conversation:    conv,
				TotalTokens:     totalTokens,
				Iterations:      iteration,
				TerminateReason: "no tool calls",
				State:           state,
			}, nil
		}

		// Execute function calls.
		var resultParts []*llm.Part
		for _, fc := range funcCalls {
			// Check for handoff (hooks not invoked for transfer).
			if fc.Name == tool.TransferToolName {
				targetAgent, _ := fc.Args["agent_name"].(string)
				reason, _ := fc.Args["reason"].(string)
				return &RunResult{
					Handoff: &HandoffResult{
						TargetAgent: targetAgent,
						Reason:      reason,
					},
					Conversation:    conv,
					TotalTokens:     totalTokens,
					Iterations:      iteration,
					TerminateReason: "handoff",
					State:           state,
				}, nil
			}

			// BeforeToolCall hook.
			if ag.Hooks != nil && ag.Hooks.BeforeToolCall != nil {
				hc := &HookContext{
					Agent:        ag,
					Conversation: conv,
					State:        state,
					Iteration:    iteration,
					TotalTokens:  totalTokens,
				}
				if err := ag.Hooks.BeforeToolCall(ctx, hc, fc); err != nil {
					return nil, fmt.Errorf("BeforeToolCall %q (iter %d): %w", fc.Name, iteration, err)
				}
			}

			// Regular tool execution.
			t := ag.Tools.Lookup(fc.Name)
			if t == nil {
				log.Printf("WARNING: unknown tool %q called by model", fc.Name)
				resultParts = append(resultParts, &llm.Part{
					FunctionResponse: &llm.FunctionResponse{
						Name:     fc.Name,
						Response: map[string]any{"error": fmt.Sprintf("unknown tool: %s", fc.Name)},
					},
				})
				continue
			}

			result, err := t.Execute(ctx, fc.Args, state)
			if err != nil {
				resultParts = append(resultParts, &llm.Part{
					FunctionResponse: &llm.FunctionResponse{
						Name:     fc.Name,
						Response: map[string]any{"error": err.Error()},
					},
				})
				continue
			}

			// AfterToolCall hook.
			if ag.Hooks != nil && ag.Hooks.AfterToolCall != nil {
				hc := &HookContext{
					Agent:        ag,
					Conversation: conv,
					State:        state,
					Iteration:    iteration,
					TotalTokens:  totalTokens,
				}
				if err := ag.Hooks.AfterToolCall(ctx, hc, fc, result); err != nil {
					return nil, fmt.Errorf("AfterToolCall %q (iter %d): %w", fc.Name, iteration, err)
				}
			}

			resultParts = append(resultParts, &llm.Part{
				FunctionResponse: &llm.FunctionResponse{
					Name:     fc.Name,
					Response: result,
				},
			})
		}

		// AfterToolCalls hook.
		if ag.Hooks != nil && ag.Hooks.AfterToolCalls != nil {
			hc := &HookContext{
				Agent:        ag,
				Conversation: conv,
				State:        state,
				Iteration:    iteration,
				TotalTokens:  totalTokens,
			}
			var err error
			resultParts, err = ag.Hooks.AfterToolCalls(ctx, hc, resultParts)
			if err != nil {
				return nil, fmt.Errorf("AfterToolCalls (iter %d): %w", iteration, err)
			}
		}

		// Append tool results to conversation.
		conv.AppendToolResults(resultParts)
	}
}

// extractText joins all text parts from a model response.
func extractText(content *llm.Content) string {
	var parts []string
	for _, p := range content.Parts {
		if p.Text != "" {
			parts = append(parts, p.Text)
		}
	}
	return strings.Join(parts, "")
}
