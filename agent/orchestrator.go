package agent

import (
	"context"
	"fmt"

	"github.com/dntatme/agents/conversation"
	"github.com/dntatme/agents/llm"
)

// OrchestratorConfig controls multi-agent orchestration.
type OrchestratorConfig struct {
	MaxHandoffs   int            // default 10
	MaxStackDepth int            // default 10
	InitialState  map[string]any // shared state passed to all agents
}

type stackFrame struct {
	agentName string
}

// Orchestrate runs a multi-agent loop with stack-based nested handoffs.
// When agent A hands off to agent B, B runs as a subroutine. When B finishes,
// control returns to A with the child's result injected as a system note.
// State is shared by reference — all agents see the same map.
func Orchestrate(
	ctx context.Context,
	client *llm.Client,
	registry *Registry,
	startAgentName string,
	userPrompt string,
	config *OrchestratorConfig,
) (*RunResult, error) {
	if config == nil {
		config = &OrchestratorConfig{}
	}
	maxHandoffs := config.MaxHandoffs
	if maxHandoffs <= 0 {
		maxHandoffs = 10
	}
	maxDepth := config.MaxStackDepth
	if maxDepth <= 0 {
		maxDepth = 10
	}

	state := config.InitialState
	if state == nil {
		state = make(map[string]any)
	}

	stack := []stackFrame{{agentName: startAgentName}}
	convs := map[string]*conversation.Conversation{}

	// Create initial conversation with user prompt.
	conv := conversation.New()
	conv.AppendUserText(userPrompt)
	convs[startAgentName] = conv

	handoffCount := 0
	predictor := NewLLMPredictor(client)

	for len(stack) > 0 {
		current := stack[len(stack)-1]

		ag := registry.Lookup(current.agentName)
		if ag == nil {
			return nil, fmt.Errorf("agent %q not found in registry", current.agentName)
		}

		// Merge agent's InitialState defaults (skip existing keys).
		if ag.InitialState != nil {
			for k, v := range ag.InitialState {
				if _, exists := state[k]; !exists {
					state[k] = v
				}
			}
		}

		result, err := Run(ctx, predictor, ag, convs[current.agentName], state)
		if err != nil {
			return nil, fmt.Errorf("agent %q: %w", current.agentName, err)
		}

		if result.Handoff == nil {
			// Agent finished without handoff — pop stack.
			stack = stack[:len(stack)-1]

			if len(stack) == 0 {
				// Top-level agent done.
				result.State = state
				return result, nil
			}

			// Inject child result into parent's conversation.
			parent := stack[len(stack)-1]
			note := fmt.Sprintf("[System: agent %q completed. Result: %s]", current.agentName, result.FinalText)
			convs[parent.agentName].AppendUserText(note)
			continue
		}

		// Handoff requested.
		handoffCount++
		if handoffCount > maxHandoffs {
			return nil, fmt.Errorf("exceeded maximum handoffs (%d)", maxHandoffs)
		}

		target := result.Handoff.TargetAgent
		if len(stack)+1 > maxDepth {
			return nil, fmt.Errorf("exceeded maximum stack depth (%d)", maxDepth)
		}

		stack = append(stack, stackFrame{agentName: target})

		// Create a fresh conversation for the child agent with context.
		childConv := conversation.New()
		note := fmt.Sprintf("[System: conversation transferred from %q to %q. Reason: %s]", current.agentName, target, result.Handoff.Reason)
		childConv.AppendUserText(note)
		convs[target] = childConv
	}

	return nil, fmt.Errorf("orchestration ended with empty stack")
}
