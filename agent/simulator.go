package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/dntatme/agents/conversation"
	"github.com/dntatme/agents/llm"
)

const (
	stateKeyAgentPatience = "agent_patience" // map[agentName]patience(0-100)
	defaultPatience       = 100
	minPatience           = 0
	maxPatience           = 100
)

// SimRuntime holds the runtime dependencies needed by tools (like urgent emails)
// that need to invoke agents during a simulation. Stored in state as "sim_runtime".
type SimRuntime struct {
	Client    *llm.Client
	Registry  *Registry
	Predictor Predictor
	// RunAgent runs a target agent with a message and returns its final text.
	// This is set by Simulate() so tools can invoke agents without importing this package.
	RunAgent func(ctx context.Context, targetName, message string, state map[string]any) (string, error)
}

// SimulationConfig controls the simulation loop.
type SimulationConfig struct {
	MaxRounds    int                                   // default 15
	InitialState map[string]any                        // shared state
	AgentOrder   []string                              // activation order per round
	OnRoundEnd   func(round int, state map[string]any) // optional callback

	// Tracing callbacks — all optional.
	OnSimulationStart func(prompt string, maxRounds int, agents []string)
	OnSimulationEnd   func(totalRounds int, reason string)
	OnRoundStart      func(round int)
	OnAgentActivation func(round int, agentName string)
	OnAgentCompletion func(round int, agentName string, result *RunResult, idle bool)
}

// SimulationResult captures the outcome of a full simulation.
type SimulationResult struct {
	FinalState  map[string]any
	TotalRounds int
	AgentRuns   []AgentRunRecord
}

// AgentRunRecord captures a single agent activation within the simulation.
type AgentRunRecord struct {
	Round    int
	Agent    string
	Output   string
	Idle     bool
	Tokens   int32
	Handoffs []string // agents that were called via handoff during this turn
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
	initializeAgentPatience(state, agentOrder)

	// Initialize agent_last_round tracking
	if _, ok := state["agent_last_round"]; !ok {
		state["agent_last_round"] = make(map[string]int)
	}

	if config.OnSimulationStart != nil {
		config.OnSimulationStart(userPrompt, maxRounds, agentOrder)
	}

	predictor := NewLLMPredictor(client)
	var allRuns []AgentRunRecord

	// Store SimRuntime in state so tools like urgent emails can invoke agents
	state["sim_runtime"] = &SimRuntime{
		Client:    client,
		Registry:  registry,
		Predictor: predictor,
	}

	// Store RunAgent function directly in state for tools to use without circular imports
	state["sim_run_agent"] = func(ctx context.Context, targetName, message string, callerState map[string]any) (string, error) {
		ag := registry.Lookup(targetName)
		if ag == nil {
			return "", fmt.Errorf("agent %q not found", targetName)
		}

		round := 0
		if r, ok := callerState["current_round"].(int); ok {
			round = r
		}

		if config.OnAgentActivation != nil {
			config.OnAgentActivation(round, targetName)
		}

		conv := conversation.New()
		conv.AppendUserText(message)
		result, err := Run(ctx, predictor, ag, conv, callerState)
		if err != nil {
			if config.OnAgentCompletion != nil {
				config.OnAgentCompletion(round, targetName, nil, false)
			}
			return "", err
		}

		if config.OnAgentCompletion != nil {
			idle := isIdle(result.FinalText)
			config.OnAgentCompletion(round, targetName, result, idle)
		}

		return result.FinalText, nil
	}

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
	bootstrapPatience := getAgentPatience(state, "ceo")
	bootstrapConv.AppendUserText(fmt.Sprintf(
		"[System: Round 0 — Bootstrap. You are the CEO. A new project has been requested.]\n\n"+
			"User Request: %s\n\n"+
			"Set the strategic direction. Delegate to your team: "+
			"transfer to product-manager for requirements, cto for technical direction, "+
			"or project-manager for task breakdown. You can make multiple handoffs.",
		userPrompt,
	))
	bootstrapConv.AppendUserText(fmt.Sprintf(
		"Current patience level: %d/100 (%s). Let this shape your tone and urgency: "+
			"as patience drops, be more direct, push harder on blockers, and escalate sooner.",
		bootstrapPatience, patienceTier(bootstrapPatience),
	))

	bootstrapResult, err := runAgentWithHandoffs(ctx, predictor, client, registry, ceoAgent, bootstrapConv, state)
	if err != nil {
		return nil, fmt.Errorf("bootstrap (ceo): %w", err)
	}
	updateAgentPatienceAfterRun(state, "ceo", isIdle(bootstrapResult.FinalText))

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

		if config.OnRoundStart != nil {
			config.OnRoundStart(round)
		}

		allIdle := true

		for _, agentName := range agentOrder {
			// Skip fired agents
			if fired, ok := state["fired_agents"].(map[string]bool); ok && fired[agentName] {
				log.Printf("[Simulation] %s is fired, skipping", agentName)
				continue
			}

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

			if config.OnAgentActivation != nil {
				config.OnAgentActivation(round, agentName)
			}

			// Build activation prompt
			lastRound := agentLastRound[agentName]
			patience := getAgentPatience(state, agentName)
			activationPrompt := buildActivationPrompt(agentName, round, lastRound, patience, state)

			conv := conversation.New()
			conv.AppendUserText(activationPrompt)

			result, err := runAgentWithHandoffs(ctx, predictor, client, registry, ag, conv, state)
			if err != nil {
				log.Printf("[Simulation] ERROR running %s in round %d: %v", agentName, round, err)
				continue
			}

			agentLastRound[agentName] = round

			idle := isIdle(result.FinalText)
			updateAgentPatienceAfterRun(state, agentName, idle)
			if !idle {
				allIdle = false
			}

			if config.OnAgentCompletion != nil {
				config.OnAgentCompletion(round, agentName, result, idle)
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
			if config.OnSimulationEnd != nil {
				config.OnSimulationEnd(round, "project_complete")
			}
			return &SimulationResult{
				FinalState:  state,
				TotalRounds: round,
				AgentRuns:   allRuns,
			}, nil
		}

		if allIdle && round > 1 {
			log.Printf("[Simulation] All agents idle at round %d, ending simulation", round)
			if config.OnSimulationEnd != nil {
				config.OnSimulationEnd(round, "all_idle")
			}
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
	if config.OnSimulationEnd != nil {
		config.OnSimulationEnd(maxRounds, "max_rounds")
	}
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
func buildActivationPrompt(agentName string, round, lastRound, patience int, state map[string]any) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("[System: Round %d. You are %s.]\n\n", round, agentName))

	sb.WriteString("IMPORTANT — Start your turn by checking your inbox with check_inbox.\n")
	sb.WriteString("Emails from colleagues may contain requests, questions, or information you need to act on.\n")
	sb.WriteString("Reply to any emails that need a response before moving on to other work.\n\n")

	sb.WriteString("Check the workspace for updates, tasks, and notes relevant to your role.\n")

	if lastRound > 0 {
		sb.WriteString(fmt.Sprintf("Read updates since round %d to catch up on what happened.\n", lastRound))
	} else {
		sb.WriteString("This may be your first activation — read updates to understand the current state.\n")
	}

	sb.WriteString("\nIf there is work for you to do, do it. If there is nothing for you to do, respond with just 'IDLE'.\n")
	sb.WriteString(fmt.Sprintf(
		"\nCurrent patience level: %d/100 (%s).\n",
		patience, patienceTier(patience),
	))
	sb.WriteString("Let this affect your behavior: as patience drops, be more direct, less accommodating, and quicker to escalate blockers.\n")
	sb.WriteString("\nAt the end of your turn, always write a diary entry with write_diary. Be honest and personal — reflect on your work, the project direction, and your thoughts about the team's work.\n")

	// Inject relationship context if a renderer is available
	if renderer, ok := state["relationship_renderer"].(func(string) string); ok {
		if relContext := renderer(agentName); relContext != "" {
			sb.WriteString("\n")
			sb.WriteString(relContext)
		}
	}

	return sb.String()
}

func initializeAgentPatience(state map[string]any, agentNames []string) {
	patience := getAgentPatienceMap(state)
	for _, name := range agentNames {
		if _, ok := patience[name]; !ok {
			patience[name] = defaultPatience
		}
	}
	if _, ok := patience["ceo"]; !ok {
		patience["ceo"] = defaultPatience
	}
}

func getAgentPatienceMap(state map[string]any) map[string]int {
	if v, ok := state[stateKeyAgentPatience]; ok {
		switch m := v.(type) {
		case map[string]int:
			return m
		case map[string]any:
			converted := make(map[string]int, len(m))
			for k, val := range m {
				switch n := val.(type) {
				case int:
					converted[k] = n
				case float64:
					converted[k] = int(n)
				}
			}
			state[stateKeyAgentPatience] = converted
			return converted
		}
	}
	m := make(map[string]int)
	state[stateKeyAgentPatience] = m
	return m
}

func getAgentPatience(state map[string]any, agentName string) int {
	patience := getAgentPatienceMap(state)
	if p, ok := patience[agentName]; ok {
		return clampPatience(p)
	}
	patience[agentName] = defaultPatience
	return defaultPatience
}

func setAgentPatience(state map[string]any, agentName string, value int) {
	patience := getAgentPatienceMap(state)
	patience[agentName] = clampPatience(value)
}

// updateAgentPatienceAfterRun lowers patience while goals remain unmet (project active).
func updateAgentPatienceAfterRun(state map[string]any, agentName string, idle bool) int {
	current := getAgentPatience(state, agentName)
	status, _ := state["project_status"].(string)
	if status == "complete" {
		setAgentPatience(state, agentName, current+5)
		return getAgentPatience(state, agentName)
	}

	decay := 3
	if idle {
		decay = 8
	}
	setAgentPatience(state, agentName, current-decay)
	return getAgentPatience(state, agentName)
}

func clampPatience(v int) int {
	if v < minPatience {
		return minPatience
	}
	if v > maxPatience {
		return maxPatience
	}
	return v
}

func patienceTier(patience int) string {
	switch {
	case patience >= 70:
		return "patient and collaborative"
	case patience >= 40:
		return "impatient and terse"
	default:
		return "highly impatient and escalation-prone"
	}
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
