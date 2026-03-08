package company

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RunCoffeeBreak runs a casual coffee break chat between agents who called get_coffee.
// It is called from OnBetweenRounds after a round ends.
// If fewer than 2 participants signed up, it's a no-op (bonus still applies next round).
func RunCoffeeBreak(ctx context.Context, state map[string]any) error {
	tracker := GetActionPointTracker(state)
	if tracker == nil {
		return nil
	}

	participants := tracker.CoffeeParticipants()
	if len(participants) < 2 {
		// Solo coffee: skip conversation, bonus still applies via InitRound next round
		return nil
	}

	round := GetCurrentRound(state)
	root := GetWorkspaceRoot(state)
	caller := GetCurrentAgent(state)

	// Get sim_run_agent function
	runAgentFn, ok := state["sim_run_agent"].(func(ctx context.Context, targetName, message string, state map[string]any) (string, error))
	if !ok {
		return nil
	}

	// Restrict tools during coffee break to relationship tools only
	SetAllowedTools(state, AllowedToolsCoffeeBreak)
	defer SetAllowedTools(state, nil)

	var transcriptText strings.Builder
	transcriptText.WriteString(fmt.Sprintf("# Coffee Break — After Round %d\n\n", round))
	transcriptText.WriteString(fmt.Sprintf("**Participants:** %s\n\n", strings.Join(participants, ", ")))

	// 2 rounds of casual chat
	for r := 1; r <= 2; r++ {
		transcriptText.WriteString(fmt.Sprintf("## Round %d\n\n", r))

		for _, participant := range participants {
			prompt := fmt.Sprintf(
				"[Coffee Break after Round %d, Chat Round %d/2]\n"+
					"You're at the coffee machine with: %s.\n"+
					"Chat casually — gossip about work, vent, joke around. Be authentic to your personality.\n"+
					"Keep it short (2-3 sentences max).\n"+
					"You can use view_relationships and update_relationship if you want, but mostly just chat.\n\n"+
					"Conversation so far:\n%s",
				round, r, strings.Join(participants, ", "), transcriptText.String(),
			)

			// Swap current_agent
			state[KeyCurrentAgent] = participant
			response, err := runAgentFn(ctx, participant, prompt, state)
			state[KeyCurrentAgent] = caller

			if err != nil {
				transcriptText.WriteString(fmt.Sprintf("**%s:** [couldn't make it to the coffee machine]\n\n", participant))
				continue
			}

			transcriptText.WriteString(fmt.Sprintf("**%s:** %s\n\n", participant, response))
		}
	}

	// Save transcript to shared/coffee/round-{N}.md
	if root != "" {
		coffeeDir := filepath.Join(root, "shared", "coffee")
		_ = os.MkdirAll(coffeeDir, 0o755)
		filename := fmt.Sprintf("round-%d.md", round)
		_ = os.WriteFile(filepath.Join(coffeeDir, filename), []byte(transcriptText.String()), 0o644)
	}

	return nil
}
