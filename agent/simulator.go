package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/dntatme/agents/conversation"
	"github.com/dntatme/agents/llm"
)

// SimulationConfig controls the simulation loop.
type SimulationConfig struct {
	MaxRounds    int                                  // default 15
	InitialState map[string]any                       // shared state
	AgentOrder   []string                             // activation order per round
	OnRoundEnd   func(round int, state map[string]any) // optional callback
}

// SimulationResult captures the outcome of a full simulation.
type SimulationResult struct {
	FinalState  map[string]any
	TotalRounds int
	AgentRuns   []AgentRunRecord
}

// AgentRunRecord captures a single agent activation within the simulation.
type AgentRunRecord struct {
	Round     int
	Agent     string
	Output    string
	Idle      bool
	Tokens    int32
	Handoffs  []string // agents that were called via handoff during this turn
}

// Simulate runs a round-based multi-agent simulation loop.
//
// Each round, every agent in AgentOrder wakes up with a fresh conversation,
// checks the workspace for updates/tasks, does work, and optionally writes
// diary entries. Agents communicate asynchronously through shared files and
// an updates channel stored in shared state.
//
// Within each agent's turn, the existing Run() ReACT loop handles tool calls.
// If an agent uses transfer_to_agent, a mini-orchestration runs for that subtree.
func Simulate(
	ctx context.Context,
	client *llm.Client,
	registry *Registry,
	userPrompt string,
	config *SimulationConfig,
) (*SimulationResult, error) {
	if config == nil {
		config = &SimulationConfig{}
	}

	maxRounds := config.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 15
	}

	state := config.InitialState
	if state == nil {
		state = make(map[string]any)
	}

	agentOrder := config.AgentOrder
	if len(agentOrder) == 0 {
		agentOrder = []string{
			"ceo", "product-manager", "cto", "architect",
			"project-manager", "backend-dev", "frontend-dev", "devops",
		}
	}

	// Initialize agent_last_round tracking
	if _, ok := state["agent_last_round"]; !ok {
		state["agent_last_round"] = make(map[string]int)
	}

	predictor := NewLLMPredictor(client)
	var allRuns []AgentRunRecord

	// Round 0 (bootstrap): Run CEO with user prompt
	log.Printf("[Simulation] === Round 0 (Bootstrap) ===")
	state["current_round"] = 0
	state["current_agent"] = "ceo"
	state["project_status"] = "active"

	ceoAgent := registry.Lookup("ceo")
	if ceoAgent == nil {
		return nil, fmt.Errorf("ceo agent not found in registry")
	}

	// Merge initial state from agent
	if ceoAgent.InitialState != nil {
		for k, v := range ceoAgent.InitialState {
			if _, exists := state[k]; !exists {
				state[k] = v
			}
		}
	}

	bootstrapConv := conversation.New()
	bootstrapConv.AppendUserText(fmt.Sprintf(
		"[System: Round 0 — Bootstrap. You are the CEO. A new project has been requested.]\n\n"+
			"User Request: %s\n\n"+
			"Set the strategic direction. Delegate to your team: "+
			"transfer to product-manager for requirements, cto for technical direction, "+
			"or project-manager for task breakdown. You can make multiple handoffs.",
		userPrompt,
	))

	bootstrapResult, err := runAgentWithHandoffs(ctx, predictor, client, registry, ceoAgent, bootstrapConv, state)
	if err != nil {
		return nil, fmt.Errorf("bootstrap (ceo): %w", err)
	}

	agentLastRound := state["agent_last_round"].(map[string]int)
	agentLastRound["ceo"] = 0

	allRuns = append(allRuns, AgentRunRecord{
		Round:  0,
		Agent:  "ceo",
		Output: bootstrapResult.FinalText,
		Tokens: bootstrapResult.TotalTokens,
	})

	log.Printf("[Simulation] CEO bootstrap complete: %s", truncate(bootstrapResult.FinalText, 100))

	// Main simulation loop
	for round := 1; round <= maxRounds; round++ {
		log.Printf("[Simulation] === Round %d ===", round)
		state["current_round"] = round

		allIdle := true

		for _, agentName := range agentOrder {
			ag := registry.Lookup(agentName)
			if ag == nil {
				log.Printf("[Simulation] WARNING: agent %q not in registry, skipping", agentName)
				continue
			}

			state["current_agent"] = agentName

			// Merge agent initial state
			if ag.InitialState != nil {
				for k, v := range ag.InitialState {
					if _, exists := state[k]; !exists {
						state[k] = v
					}
				}
			}

			// Build activation prompt
			lastRound := agentLastRound[agentName]
			activationPrompt := buildActivationPrompt(agentName, round, lastRound)

			conv := conversation.New()
			conv.AppendUserText(activationPrompt)

			result, err := runAgentWithHandoffs(ctx, predictor, client, registry, ag, conv, state)
			if err != nil {
				log.Printf("[Simulation] ERROR running %s in round %d: %v", agentName, round, err)
				continue
			}

			agentLastRound[agentName] = round

			idle := isIdle(result.FinalText)
			if !idle {
				allIdle = false
			}

			allRuns = append(allRuns, AgentRunRecord{
				Round:  round,
				Agent:  agentName,
				Output: result.FinalText,
				Idle:   idle,
				Tokens: result.TotalTokens,
			})

			if idle {
				log.Printf("[Simulation]   %s: IDLE", agentName)
			} else {
				log.Printf("[Simulation]   %s: %s", agentName, truncate(result.FinalText, 100))
			}
		}

		// Check termination: all idle or project marked complete
		if projectStatus, ok := state["project_status"].(string); ok && projectStatus == "complete" {
			log.Printf("[Simulation] Project marked complete at round %d", round)
			return &SimulationResult{
				FinalState:  state,
				TotalRounds: round,
				AgentRuns:   allRuns,
			}, nil
		}

		if allIdle && round > 1 {
			log.Printf("[Simulation] All agents idle at round %d, ending simulation", round)
			return &SimulationResult{
				FinalState:  state,
				TotalRounds: round,
				AgentRuns:   allRuns,
			}, nil
		}

		// OnRoundEnd callback
		if config.OnRoundEnd != nil {
			config.OnRoundEnd(round, state)
		}
	}

	log.Printf("[Simulation] Reached maximum rounds (%d)", maxRounds)
	return &SimulationResult{
		FinalState:  state,
		TotalRounds: maxRounds,
		AgentRuns:   allRuns,
	}, nil
}

// runAgentWithHandoffs runs an agent and handles any handoffs via Orchestrate.
func runAgentWithHandoffs(
	ctx context.Context,
	predictor Predictor,
	client *llm.Client,
	registry *Registry,
	ag *Agent,
	conv *conversation.Conversation,
	state map[string]any,
) (*RunResult, error) {
	result, err := Run(ctx, predictor, ag, conv, state)
	if err != nil {
		return nil, err
	}

	// If handoff occurred, run a mini-orchestration
	if result.Handoff != nil {
		log.Printf("[Simulation]   %s → handoff to %s", ag.Name, result.Handoff.TargetAgent)

		orchResult, err := Orchestrate(ctx, client, registry, result.Handoff.TargetAgent,
			fmt.Sprintf("[System: Handoff from %s. Reason: %s]", ag.Name, result.Handoff.Reason),
			&OrchestratorConfig{
				MaxHandoffs:   5,
				MaxStackDepth: 5,
				InitialState:  state,
			},
		)
		if err != nil {
			log.Printf("[Simulation]   Handoff orchestration error: %v", err)
			return result, nil // return original result on error
		}
		return orchResult, nil
	}

	return result, nil
}

// buildActivationPrompt creates the prompt for an agent's turn.
func buildActivationPrompt(agentName string, round, lastRound int) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("[System: Round %d. You are %s.]\n\n", round, agentName))
	sb.WriteString("Check the workspace for updates, tasks, and notes relevant to your role.\n")

	if lastRound > 0 {
		sb.WriteString(fmt.Sprintf("Read updates since round %d to catch up on what happened.\n", lastRound))
	} else {
		sb.WriteString("This may be your first activation — read updates to understand the current state.\n")
	}

	sb.WriteString("\nIf there is work for you to do, do it. If there is nothing for you to do, respond with just 'IDLE'.\n")
	sb.WriteString("\nAt the end of your turn, always write a diary entry with write_diary. Be honest and personal — reflect on your work, the project direction, and your thoughts about the team's work.\n")

	return sb.String()
}

// isIdle checks if the agent's response indicates it had nothing to do.
func isIdle(text string) bool {
	trimmed := strings.TrimSpace(strings.ToUpper(text))
	return trimmed == "IDLE" || strings.HasPrefix(trimmed, "IDLE")
}

// truncate shortens a string for logging.
func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
