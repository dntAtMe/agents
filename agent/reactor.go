package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"google.golang.org/genai"

	"github.com/kacperpaczos/agents/conversation"
	"github.com/kacperpaczos/agents/llm"
	"github.com/kacperpaczos/agents/termination"
	"github.com/kacperpaczos/agents/tool"
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
}

// Run executes the ReACT loop for one agent.
func Run(ctx context.Context, client *llm.Client, ag *Agent, conv *conversation.Conversation) (*RunResult, error) {
	var totalTokens int32
	iteration := 0

	// Build Gemini config.
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: ag.ResolveSystemPrompt()}},
		},
		Temperature:     ag.Temperature,
		MaxOutputTokens: ag.MaxOutputTokens,
	}

	// Attach tool definitions if any.
	if ag.Tools != nil && ag.Tools.Len() > 0 {
		config.Tools = []*genai.Tool{
			{FunctionDeclarations: ag.Tools.Definitions()},
		}
	}

	for {
		iteration++

		// 1. Call Gemini.
		resp, err := client.GenerateContent(ctx, ag.Model, conv.Messages, config)
		if err != nil {
			return nil, fmt.Errorf("GenerateContent (iter %d): %w", iteration, err)
		}

		// 2. Track token usage.
		if resp.UsageMetadata != nil {
			totalTokens += resp.UsageMetadata.TotalTokenCount
		}

		// 3. Append model response.
		if len(resp.Candidates) == 0 {
			return &RunResult{
				FinalText:       "",
				Conversation:    conv,
				TotalTokens:     totalTokens,
				Iterations:      iteration,
				TerminateReason: "no candidates in response",
			}, nil
		}
		modelContent := resp.Candidates[0].Content
		conv.AppendModelContent(modelContent)

		// 4. Extract function calls.
		var funcCalls []*genai.FunctionCall
		for _, part := range modelContent.Parts {
			if part.FunctionCall != nil {
				funcCalls = append(funcCalls, part.FunctionCall)
			}
		}

		hasToolCalls := len(funcCalls) > 0

		// 5. Check termination policy.
		if ag.TerminationPolicy != nil {
			state := termination.State{
				Iteration:       iteration,
				TotalTokensUsed: totalTokens,
				LastResponse:    resp,
				HasToolCalls:    hasToolCalls,
			}
			if stop, reason := ag.TerminationPolicy.ShouldTerminate(ctx, state); stop {
				return &RunResult{
					FinalText:       extractText(modelContent),
					Conversation:    conv,
					TotalTokens:     totalTokens,
					Iterations:      iteration,
					TerminateReason: reason,
				}, nil
			}
		}

		// 6. No function calls → return (safety net if DoneSignal not configured).
		if !hasToolCalls {
			return &RunResult{
				FinalText:       extractText(modelContent),
				Conversation:    conv,
				TotalTokens:     totalTokens,
				Iterations:      iteration,
				TerminateReason: "no tool calls",
			}, nil
		}

		// 7. Execute function calls.
		var resultParts []*genai.Part
		for _, fc := range funcCalls {
			// Check for handoff.
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
				}, nil
			}

			// Regular tool execution.
			t := ag.Tools.Lookup(fc.Name)
			if t == nil {
				log.Printf("WARNING: unknown tool %q called by model", fc.Name)
				resultParts = append(resultParts, &genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						Name:     fc.Name,
						Response: map[string]any{"error": fmt.Sprintf("unknown tool: %s", fc.Name)},
					},
				})
				continue
			}

			result, err := t.Execute(ctx, fc.Args)
			if err != nil {
				resultParts = append(resultParts, &genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						Name:     fc.Name,
						Response: map[string]any{"error": err.Error()},
					},
				})
				continue
			}

			resultParts = append(resultParts, &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					Name:     fc.Name,
					Response: result,
				},
			})
		}

		// 8. Append tool results to conversation.
		conv.AppendToolResults(resultParts)
	}
}

// extractText joins all text parts from a model response.
func extractText(content *genai.Content) string {
	var parts []string
	for _, p := range content.Parts {
		if p.Text != "" {
			parts = append(parts, p.Text)
		}
	}
	return strings.Join(parts, "")
}
