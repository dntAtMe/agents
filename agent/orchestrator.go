package agent

import (
	"context"
	"fmt"

	"github.com/kacperpaczos/agents/conversation"
	"github.com/kacperpaczos/agents/llm"
)

// OrchestratorConfig controls multi-agent orchestration.
type OrchestratorConfig struct {
	MaxHandoffs int // default 10
}

// Orchestrate runs a multi-agent loop with handoffs.
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

	conv := conversation.New()
	conv.AppendUserText(userPrompt)

	currentName := startAgentName

	for handoff := 0; handoff <= maxHandoffs; handoff++ {
		ag := registry.Lookup(currentName)
		if ag == nil {
			return nil, fmt.Errorf("agent %q not found in registry", currentName)
		}

		result, err := Run(ctx, client, ag, conv)
		if err != nil {
			return nil, fmt.Errorf("agent %q: %w", currentName, err)
		}

		// No handoff → done.
		if result.Handoff == nil {
			return result, nil
		}

		// Inject a transfer note so the next agent has context.
		target := result.Handoff.TargetAgent
		reason := result.Handoff.Reason
		note := fmt.Sprintf("[System: conversation transferred from %q to %q. Reason: %s]", currentName, target, reason)
		conv.AppendUserText(note)

		currentName = target
	}

	return nil, fmt.Errorf("exceeded maximum handoffs (%d)", maxHandoffs)
}
