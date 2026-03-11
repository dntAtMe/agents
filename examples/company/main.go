// Package main demonstrates the company simulation with dynamic hiring.
// The CEO starts alone, interviews candidates, and builds the team.
//
// Run with: GEMINI_API_KEY=... go run ./examples/company "Build a simple todo REST API"
// Or with Ollama: LLM_PROVIDER=ollama OLLAMA_MODEL=llama3.1 go run ./examples/company "Build a simple todo REST API"
//
// Flags:
//   --thinking     Enable thinking/reasoning mode for all agents (shows internal reasoning)
//   --tool-only    Force all agents to use tool-only mode (function calls only, no text generation)
//
// Example: GEMINI_API_KEY=... go run ./examples/company --thinking --tool-only "Build a todo API"
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dntatme/agents/agent"
	"github.com/dntatme/agents/capabilities/company"
	"github.com/dntatme/agents/llm"
	"github.com/dntatme/agents/prompt"
	"github.com/dntatme/agents/trace"
	"github.com/dntatme/agents/tui"
)

// mergeHooks combines two hook sets so both are called on each event.
func mergeHooks(a, b *agent.Hooks) *agent.Hooks {
	return &agent.Hooks{
		AfterPredict: func(ctx context.Context, hc *agent.HookContext, content *llm.Content) error {
			if a.AfterPredict != nil {
				if err := a.AfterPredict(ctx, hc, content); err != nil {
					return err
				}
			}
			if b.AfterPredict != nil {
				return b.AfterPredict(ctx, hc, content)
			}
			return nil
		},
		BeforeToolCall: func(ctx context.Context, hc *agent.HookContext, fc *llm.FunctionCall) error {
			if a.BeforeToolCall != nil {
				if err := a.BeforeToolCall(ctx, hc, fc); err != nil {
					return err
				}
			}
			if b.BeforeToolCall != nil {
				return b.BeforeToolCall(ctx, hc, fc)
			}
			return nil
		},
		AfterToolCall: func(ctx context.Context, hc *agent.HookContext, fc *llm.FunctionCall, result map[string]any) error {
			if a.AfterToolCall != nil {
				if err := a.AfterToolCall(ctx, hc, fc, result); err != nil {
					return err
				}
			}
			if b.AfterToolCall != nil {
				return b.AfterToolCall(ctx, hc, fc, result)
			}
			return nil
		},
	}
}

// createProvider creates an LLM provider based on env vars.
func createProvider(ctx context.Context) (llm.Provider, error) {
	providerName := os.Getenv("LLM_PROVIDER")
	if providerName == "" {
		providerName = "gemini"
	}

	switch providerName {
	case "gemini":
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("set GEMINI_API_KEY to use the Gemini provider")
		}
		return llm.NewGemini(ctx, apiKey)
	case "ollama":
		baseURL := os.Getenv("OLLAMA_URL")
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		model := os.Getenv("OLLAMA_MODEL")
		if model == "" {
			model = "llama3.1"
		}
		return llm.NewOllama(baseURL, model), nil
	default:
		return nil, fmt.Errorf("unknown LLM_PROVIDER %q (use 'gemini' or 'ollama')", providerName)
	}
}

func main() {
	// Parse command-line flags
	thinkingEnabled := flag.Bool("thinking", false, "Enable thinking/reasoning mode for all agents")
	toolOnlyMode := flag.Bool("tool-only", false, "Force all agents to use tool-only mode (function calls only)")
	flag.Parse()

	// Get user prompt from remaining args
	userPrompt := "Build a simple todo REST API with CRUD operations"
	if len(flag.Args()) > 0 {
		userPrompt = strings.Join(flag.Args(), " ")
	}

	ctx := context.Background()

	provider, err := createProvider(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Provider error: %v\n", err)
		os.Exit(1)
	}

	// Initialize workspace
	workspaceRoot, _ := filepath.Abs("workspace")
	if err := company.InitWorkspace(workspaceRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Workspace init error: %v\n", err)
		os.Exit(1)
	}

	// Initialize tracer
	tr, err := trace.New(filepath.Join(workspaceRoot, "trace.jsonl"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Tracer init error: %v\n", err)
		os.Exit(1)
	}
	defer tr.Close()

	// TUI event channel and factories
	events := make(chan tui.Event, 64)
	tuiHooks := tui.Hooks(events)
	tuiCb := tui.Callbacks(events)

	// Pause/resume and email injection channels
	pauseCh := make(chan struct{}, 1)
	resumeCh := make(chan struct{}, 1)
	injectCh := make(chan tui.InjectEmail, 4)

	// Shared instruction strings
	diaryInstruction := "At the end of your turn, always write a diary entry with write_diary. " +
		"Be honest and personal — reflect on your work, the project direction, " +
		"your thoughts about the team's work, frustrations, and celebrations."

	idleInstruction := "If there is nothing for you to do this round, respond with just 'IDLE'."

	energyInstruction := "You have limited action points (AP) per round. Read-only actions are cheap (1 AP), " +
		"writing costs more (2-3 AP), meetings cost 5 AP. Budget your turn wisely. " +
		"Use get_coffee to take a coffee break between rounds and get +5 bonus AP next round."

	contextInstruction := "This is a simulation. Read updates since your last active round to catch up. " +
		"Use read_updates, read_task_board, and read_file to understand the current state before acting."

	meetingEmailInstruction := "Always check_inbox at the start of your turn — do not skip this. " +
		"Read and reply to any emails that need a response before doing other work. " +
		"Use send_email to send requests, status updates, or questions to colleagues. " +
		"Use send_email with urgent=true when you need an immediate response from a colleague. " +
		"Use call_group_meeting to organize multi-agent discussions when needed."
	emailOnlyInstruction := "Always check_inbox at the start of your turn — do not skip this. " +
		"Read and reply to any emails that need a response before doing other work. " +
		"Use send_email to send requests, status updates, or questions to colleagues. " +
		"Use send_email with urgent=true when you need an immediate response from a colleague."

	relationshipInstruction := "Use view_relationships to see your relationship scores with colleagues. " +
		"Use update_relationship to adjust scores based on interactions (delta -20 to +20). " +
		"Use file_escalation to formally report a colleague's bad behavior to their manager — do not hesitate to escalate repeated problems. " +
		"Your relationship scores influence your tone and cooperation level. " +
		"Be authentic — lower scores mean less patience and willingness to help. " +
		"Do not forgive repeated offenses — escalate and let the system handle consequences."
	managerEscalationInstruction := "As a manager, use view_escalations EVERY round to check for pending escalations. " +
		"You MUST respond to every escalation — never ignore them. " +
		"Do NOT dismiss escalations lightly — if the complaint has merit, use 'action_taken'. " +
		"Repeated escalations about the same person are a serious problem: " +
		"2 action_taken escalations auto-generates a PiP, and 3 while on PiP auto-generates a firing request. " +
		"Use record_pip proactively for performance issues — do not wait for the system to auto-generate one. " +
		"Use request_fire when behavior is unacceptable. Do not give infinite chances."
	ceoFireInstruction := "As CEO, use view_fire_requests EVERY round to check for pending requests. " +
		"Approve firing requests promptly when justified — the company cannot afford dead weight. " +
		"Use approve_fire to approve or deny them."

	codingWorkflowInstruction := "Your coding workflow: 1) Search existing code with search_files to understand context. " +
		"2) Write/edit code with write_file or edit_file. 3) Run build and tests with run_command. " +
		"4) If build/tests fail, fix the code and re-run. Iterate until passing. " +
		"5) Update task to 'awaiting_review' only when build succeeds. " +
		"6) After review, use read_code_reviews to see inline feedback, then fix with edit_file."

	codeReviewInstruction := "When reviewing code: 1) Read the source files with read_file and search_files. " +
		"2) Use start_code_review to begin a review, then add_review_comment for each issue " +
		"with specific file, line, and severity. 3) Use submit_code_review with your verdict. " +
		"4) Optionally run_command to verify the implementation builds/passes tests."

	hiringInstruction := "You need to build your team. Use start_interview to interview candidates. " +
		"After reviewing the transcript, use hire_decision to hire or pass. " +
		"Available positions: product-manager, cto, architect, project-manager, " +
		"backend-dev, frontend-dev, devops. Hire strategically — a candidate's true personality " +
		"is hidden, so judge carefully from their interview responses."

	// --- Assign CEO personality only at startup ---
	ceoPersonalities := company.AssignPersonalities([]string{"ceo"})
	shareholderTemperament := company.AssignShareholderTemperament()

	// Write CEO personality file
	{
		p := ceoPersonalities["ceo"]
		personalityPath := filepath.Join(workspaceRoot, "ceo", "personality.md")
		_ = os.MkdirAll(filepath.Dir(personalityPath), 0o755)
		_ = os.WriteFile(personalityPath, []byte(p.Description()), 0o644)
	}
	// Write shareholders temperament file
	{
		personalityPath := filepath.Join(workspaceRoot, "shareholders", "personality.md")
		_ = os.MkdirAll(filepath.Dir(personalityPath), 0o755)
		_ = os.WriteFile(personalityPath, []byte(shareholderTemperament.Render()), 0o644)
	}

	// --- Build Org Hierarchy (starts minimal, grows as agents are hired) ---
	orgHierarchy := company.NewOrgHierarchy()

	// Shared tool instances
	sendEmail := company.SendEmailTool()
	checkInbox := company.CheckInboxTool()
	replyEmail := company.ReplyEmailTool()
	viewRelationships := company.ViewRelationshipsTool()
	updateRelationship := company.UpdateRelationshipTool()
	fileEscalation := company.FileEscalationTool()
	viewEscalations := company.ViewEscalationsTool()
	respondToEscalation := company.RespondToEscalationTool()
	recordPiP := company.RecordPiPTool()
	requestFire := company.RequestFireTool()
	viewFireRequests := company.ViewFireRequestsTool()
	approveFire := company.ApproveFireTool()
	nonLeafAgents := []string{"ceo", "product-manager", "cto", "architect", "project-manager"}
	callMeeting := company.CallGroupMeetingTool(nonLeafAgents)
	editFile := company.EditFileTool()
	searchFiles := company.SearchFilesTool()
	diffFile := company.DiffFileTool()
	getCoffee := company.GetCoffeeTool()
	runCommand := company.RunCommandTool()
	startCodeReview := company.StartCodeReviewTool()
	addReviewComment := company.AddReviewCommentTool()
	submitCodeReview := company.SubmitCodeReviewTool()
	readCodeReviews := company.ReadCodeReviewsTool()

	// --- Register only CEO and shareholders at startup ---
	registry := agent.NewRegistry()

	// CEO — with hiring tools added
	ceoIdentity := prompt.Identity(ceoPersonalities["ceo"].Role)
	ceoPers := ceoPersonalities["ceo"]
	ceoPersonalityMixin := prompt.Mixin{Name: "Personality", Content: fmt.Sprintf(
		"**Personality:** %s\n**Work ethic:** %s\n\n**Motivation:** %s\n\n**Communication style:** %s\n\n**Work culture:** %s",
		ceoPers.Name, ceoPers.WorkEthic, ceoPers.Motivation, ceoPers.CommunicationStyle, ceoPers.WorkCulture,
	)}

	ceoBuilder := agent.New("ceo").
		PromptBuilder(prompt.NewBuilder().
			Add(ceoIdentity).
			Add(ceoPersonalityMixin).
			Add(prompt.HandoffPolicy(
				"Delegate to product-manager for requirements and PRD writing. "+
					"Delegate to cto for technical architecture and development oversight. "+
					"Delegate to project-manager for task breakdown and tracking. "+
					"You can change project direction mid-stream if needed.")).
			Add(prompt.ToolUsage(
				hiringInstruction+"\n"+
					meetingEmailInstruction+"\n"+relationshipInstruction+"\n"+managerEscalationInstruction+"\n"+ceoFireInstruction)).
			Add(prompt.Context(contextInstruction)).
			Add(prompt.Guardrails(diaryInstruction+"\n"+idleInstruction+"\n"+energyInstruction))).
		Tools(
			company.ReadFileTool(),
			company.WriteFileTool(),
			company.ListFilesTool(),
			company.ReadTaskBoardTool(),
			company.PostUpdateTool(),
			company.ReadUpdatesTool(),
			company.ReadDecisionsTool(),
			company.WriteDiaryTool(),

			sendEmail,
			checkInbox,
			replyEmail,
			callMeeting,
			viewRelationships,
			updateRelationship,
			fileEscalation,
			viewEscalations,
			respondToEscalation,
			recordPiP,
			requestFire,
			viewFireRequests,
			approveFire,
			getCoffee,
			company.CheckStockPriceTool(),

			// Hiring tools
			company.StartInterviewTool(),
			company.HireDecisionTool(),
		).
		ThinkingEnabled(*thinkingEnabled)
	if *toolOnlyMode {
		ceoBuilder = ceoBuilder.ToolMode(llm.ToolModeAny).EndTurn()
	}
	registry.Register(ceoBuilder.Build())

	// Shareholders (runs last each round, assesses company performance)
	shareholdersBuilder := agent.New("shareholders").
		PromptBuilder(prompt.NewBuilder().
			Add(prompt.Identity(company.RoleFor("shareholders"))).
			Add(prompt.Mixin{Name: "MarketTemperament", Content: shareholderTemperament.Render()}).
			Add(prompt.Context(contextInstruction)).
			Add(prompt.Guardrails(diaryInstruction+"\n"+energyInstruction))).
		Tools(
			company.ReadTaskBoardTool(),
			company.ReadUpdatesTool(),
			company.ReadDecisionsTool(),
			company.ReadFileTool(),
			company.UpdateStockPriceTool(),
			company.WriteDiaryTool(),
		).
		ThinkingEnabled(*thinkingEnabled)
	if *toolOnlyMode {
		shareholdersBuilder = shareholdersBuilder.ToolMode(llm.ToolModeAny).EndTurn()
	}
	registry.Register(shareholdersBuilder.Build())

	// Initialize action point tracker
	apTracker := company.NewActionPointTracker(15, 5, 3)

	// Build merged hooks (tracer + TUI + AP)
	tracerHooks := tr.Hooks()
	apHooks := &agent.Hooks{
		BeforeToolCall: func(ctx context.Context, hc *agent.HookContext, fc *llm.FunctionCall) error {
			// Enforce tool restrictions (coffee break, urgent email)
			if allowed := company.GetAllowedTools(hc.State); allowed != nil {
				if !allowed[fc.Name] {
					return fmt.Errorf("TOOL RESTRICTED: %s is not available right now", fc.Name)
				}
			}

			agentName := company.GetCurrentAgent(hc.State)
			cost := company.GetToolCost(fc.Name)
			remaining := apTracker.Remaining(agentName)

			// Hard cap: block tool if too far in debt
			if remaining <= -apTracker.HardCap {
				return fmt.Errorf("ACTION POINTS EXHAUSTED: You have %d AP remaining (hard cap reached). Your turn is over", remaining)
			}

			// Deduct cost
			apTracker.Deduct(agentName, cost)

			// Send AP cost to TUI
			events <- tui.Event{
				Type:  "ap_update",
				Agent: agentName,
				Data: map[string]any{
					"remaining": apTracker.Remaining(agentName),
					"max_ap":    apTracker.DefaultAP,
					"tool":      fc.Name,
					"cost":      cost,
				},
			}
			return nil
		},
		AfterToolCall: func(ctx context.Context, hc *agent.HookContext, fc *llm.FunctionCall, result map[string]any) error {
			agentName := company.GetCurrentAgent(hc.State)
			remaining := apTracker.Remaining(agentName)

			// Soft limit: inject warning when at or below 0
			if remaining <= 0 {
				result["_ap_warning"] = fmt.Sprintf(
					"WARNING: You have %d action points remaining. Wrap up now — only write_diary is recommended. "+
						"At %d AP your turn will be forcibly ended.",
					remaining, -apTracker.HardCap,
				)
			}

			// Send stock_update event to TUI when stock price changes
			if fc.Name == "update_stock_price" {
				st := company.GetStockTracker(hc.State)
				delta := 0.0
				if len(st.History) > 1 {
					delta = st.History[len(st.History)-1].Delta
				}
				events <- tui.Event{
					Type:  "stock_update",
					Agent: agentName,
					Data: map[string]any{
						"price":     st.Current,
						"delta":     delta,
						"sentiment": st.Sentiment,
					},
				}
			}
			return nil
		},
	}
	merged := mergeHooks(mergeHooks(tracerHooks, tuiHooks), apHooks)

	// Attach hooks to initial agents
	for _, name := range []string{"ceo", "shareholders"} {
		ag := registry.Lookup(name)
		if ag != nil {
			ag.Hooks = merged
		}
	}

	// Initialize stock tracker
	stockTracker := company.NewStockTracker(100.0)

	// Build initial state
	agentOrder := []string{"ceo", "shareholders"}

	initialState := map[string]any{
		"workspace_root":        workspaceRoot,
		"project_name":          userPrompt,
		"agent_order":           agentOrder,
		"tool_only_mode":        *toolOnlyMode,
		company.KeyOrgHierarchy: orgHierarchy,
		company.KeyFiredAgents:  map[string]bool{},
		company.KeyActionPoints: apTracker,
		company.KeyStockPrice:   stockTracker,
	}

	// Store renderers as closures
	initialState["relationship_renderer"] = func(agentName string) string {
		return company.GetRelationshipLog(initialState).RenderForAgent(agentName)
	}

	initialState["diary_renderer"] = func(agentName string) string {
		return company.LastDiaryEntry(workspaceRoot, agentName)
	}

	initialState["ap_renderer"] = func(agentName string) string {
		remaining := apTracker.Remaining(agentName)
		return fmt.Sprintf(
			"You have %d action points this round. Each action costs points (read=1, write=2-3, meetings=5). "+
				"Budget your turn wisely. Use get_coffee to take a coffee break and get +5 bonus AP next round.",
			remaining,
		)
	}

	csuite := map[string]bool{"ceo": true, "cto": true}
	initialState["stock_renderer"] = func(agentName string) string {
		if !csuite[agentName] {
			return ""
		}
		return "📈 " + stockTracker.RenderBrief() + "\n" +
			"Use check_stock_price for detailed history. The shareholders assess performance each round."
	}

	// Team roster renderer
	initialState["team_renderer"] = func(agentName string) string {
		currentOrder, _ := initialState["agent_order"].([]string)
		fired := company.GetFiredAgents(initialState)
		if len(currentOrder) <= 2 {
			// Only CEO + shareholders — no team yet
			return "Your team: No employees hired yet (besides you)."
		}
		var sb strings.Builder
		sb.WriteString("Current team members:\n")
		for _, name := range currentOrder {
			if fired[name] {
				continue
			}
			if name == agentName {
				sb.WriteString(fmt.Sprintf("- %s (you)", name))
			} else {
				sb.WriteString(fmt.Sprintf("- %s", name))
			}
			if mgr := orgHierarchy.GetManager(name); mgr != "" {
				sb.WriteString(fmt.Sprintf(" [reports to %s]", mgr))
			}
			sb.WriteString("\n")
		}
		return sb.String()
	}

	// Environment renderer
	initialState["env_renderer"] = func(agentName string) string {
		env := company.GetAgentEnvironment(initialState, agentName)
		switch env {
		case company.EnvInterview:
			return "ENVIRONMENT: You are conducting a job interview."
		default:
			return "ENVIRONMENT: You are at your desk in the office."
		}
	}

	// --- Agent builder factories (closures that capture shared state) ---
	// Each factory builds and registers a permanent agent for the given position.

	// Helper to apply global settings (thinking, tool mode) to any agent builder
	applyGlobalSettings := func(b *agent.Builder) *agent.Builder {
		b = b.ThinkingEnabled(*thinkingEnabled)
		if *toolOnlyMode {
			b = b.ToolMode(llm.ToolModeAny).EndTurn()
		}
		return b
	}

	buildAgentForPosition := func(position string, personality *company.Personality, candidateName string, reportingTo string) error {
		// Set org hierarchy
		orgHierarchy.SetManager(position, reportingTo)

		// Build identity and personality mixins from the hired personality
		identityMixin := prompt.Identity(personality.Role)
		personalityMixin := prompt.Mixin{Name: "Personality", Content: fmt.Sprintf(
			"**Personality:** %s\n**Work ethic:** %s\n\n**Motivation:** %s\n\n**Communication style:** %s\n\n**Work culture:** %s",
			personality.Name, personality.WorkEthic, personality.Motivation, personality.CommunicationStyle, personality.WorkCulture,
		)}

		var ag *agent.Agent

		switch position {
		case "product-manager":
			ag = applyGlobalSettings(agent.New(position).
				PromptBuilder(prompt.NewBuilder().
					Add(identityMixin).
					Add(personalityMixin).
					Add(prompt.ToolUsage(
						"Use write_file to create/update shared/prd.md. "+
							"Use read_file to check existing documents. "+
							"Use post_update to announce PRD updates. "+
							meetingEmailInstruction+"\n"+relationshipInstruction+"\n"+managerEscalationInstruction)).
					Add(prompt.Context(contextInstruction)).
					Add(prompt.Guardrails(diaryInstruction+"\n"+idleInstruction+"\n"+energyInstruction))).
				Tools(
					company.ReadFileTool(),
					company.WriteFileTool(),
					company.ListFilesTool(),
					company.PostUpdateTool(),
					company.ReadUpdatesTool(),
					company.WriteDiaryTool(),
					sendEmail, checkInbox, replyEmail, callMeeting,
					viewRelationships, updateRelationship,
					fileEscalation, viewEscalations, respondToEscalation, recordPiP,
					requestFire, getCoffee,
				)).
				Build()

		case "cto":
			ag = applyGlobalSettings(agent.New(position).
				PromptBuilder(prompt.NewBuilder().
					Add(identityMixin).
					Add(personalityMixin).
					Add(prompt.HandoffPolicy("Delegate detailed design and code review to the architect.")).
					Add(prompt.ToolUsage(
						"Use write_file for shared/architecture.md. "+
							"Use log_decision for ADRs. Use read_task_board to check progress. "+
							"Use post_update to announce technical decisions. "+
							codeReviewInstruction+"\n"+
							meetingEmailInstruction+"\n"+relationshipInstruction+"\n"+managerEscalationInstruction)).
					Add(prompt.Context(contextInstruction)).
					Add(prompt.Guardrails(diaryInstruction+"\n"+idleInstruction+"\n"+energyInstruction))).
				Tools(
					company.ReadFileTool(),
					company.WriteFileTool(),
					company.ListFilesTool(),
					company.AppendToFileTool(),
					company.ReadTaskBoardTool(),
					company.UpdateTaskTool(),
					company.PostUpdateTool(),
					company.ReadUpdatesTool(),
					company.LogDecisionTool(),
					company.ReadDecisionsTool(),
					company.WriteDiaryTool(),
					company.WriteReviewTool(),
					searchFiles, diffFile,
					startCodeReview, addReviewComment, submitCodeReview, readCodeReviews,
					sendEmail, checkInbox, replyEmail, callMeeting,
					viewRelationships, updateRelationship,
					fileEscalation, viewEscalations, respondToEscalation, recordPiP,
					requestFire, getCoffee,
					company.CheckStockPriceTool(),
				)).
				Build()

		case "architect":
			ag = applyGlobalSettings(agent.New(position).
				PromptBuilder(prompt.NewBuilder().
					Add(identityMixin).
					Add(personalityMixin).
					Add(prompt.HandoffPolicy("You can delegate implementation work to backend-dev, frontend-dev, or devops.")).
					Add(prompt.ToolUsage(
						"Use read_file to review developer plans. "+
							"Use write_review to approve or request changes on implementation plans. "+
							"Use post_update to announce review results on the 'reviews' channel. "+
							"Use log_decision for architectural decisions. "+
							codeReviewInstruction+"\n"+
							meetingEmailInstruction+"\n"+relationshipInstruction+"\n"+managerEscalationInstruction)).
					Add(prompt.Context(contextInstruction)).
					Add(prompt.Guardrails(diaryInstruction+"\n"+idleInstruction+"\n"+energyInstruction))).
				Tools(
					company.ReadFileTool(),
					company.WriteFileTool(),
					company.ListFilesTool(),
					company.ReadTaskBoardTool(),
					company.UpdateTaskTool(),
					company.PostUpdateTool(),
					company.ReadUpdatesTool(),
					company.LogDecisionTool(),
					company.ReadDecisionsTool(),
					company.WriteDiaryTool(),
					company.WriteReviewTool(),
					searchFiles, diffFile, runCommand,
					startCodeReview, addReviewComment, submitCodeReview, readCodeReviews,
					sendEmail, checkInbox, replyEmail, callMeeting,
					viewRelationships, updateRelationship,
					fileEscalation, viewEscalations, respondToEscalation, recordPiP,
					requestFire, getCoffee,
				)).
				Build()

		case "project-manager":
			ag = applyGlobalSettings(agent.New(position).
				PromptBuilder(prompt.NewBuilder().
					Add(identityMixin).
					Add(personalityMixin).
					Add(prompt.ToolUsage(
						"Use add_task to create tasks — ALWAYS set a deadline. Use the reviewer param to assign a reviewer (e.g. 'architect', 'cto'). "+
							"Use update_task to change statuses. "+
							"Use read_task_board to review current state and check for overdue tasks. "+
							"Use post_update to announce sprint status. "+
							"Use send_email with urgent=true to chase developers on overdue or stalled tasks. "+
							meetingEmailInstruction+"\n"+relationshipInstruction+"\n"+managerEscalationInstruction)).
					Add(prompt.Context(contextInstruction)).
					Add(prompt.Guardrails(diaryInstruction+"\n"+idleInstruction+"\n"+energyInstruction))).
				Tools(
					company.ReadFileTool(),
					company.ListFilesTool(),
					company.AddTaskTool(),
					company.UpdateTaskTool(),
					company.ReadTaskBoardTool(),
					company.PostUpdateTool(),
					company.ReadUpdatesTool(),
					company.WriteDiaryTool(),
					sendEmail, checkInbox, replyEmail, callMeeting,
					viewRelationships, updateRelationship,
					fileEscalation, viewEscalations, respondToEscalation, recordPiP,
					requestFire, getCoffee,
				)).
				Build()

		case "backend-dev", "frontend-dev", "devops":
			ag = applyGlobalSettings(agent.New(position).
				PromptBuilder(prompt.NewBuilder().
					Add(identityMixin).
					Add(personalityMixin).
					Add(prompt.ToolUsage(
						"Use read_task_board to find your assigned tasks. "+
							"Use write_file for plans and source code. "+
							"Use read_file to check architect reviews. "+
							"Use update_task to change task status. "+
							"Use post_update to request reviews. "+
							"Use write_review to review tasks when you are the assigned reviewer. "+
							codingWorkflowInstruction+"\n"+
							emailOnlyInstruction+"\n"+relationshipInstruction)).
					Add(prompt.Context(contextInstruction)).
					Add(prompt.Guardrails(diaryInstruction+"\n"+idleInstruction+"\n"+energyInstruction))).
				Tools(
					company.ReadFileTool(),
					company.WriteFileTool(),
					company.ListFilesTool(),
					company.AppendToFileTool(),
					company.ReadTaskBoardTool(),
					company.UpdateTaskTool(),
					company.PostUpdateTool(),
					company.ReadUpdatesTool(),
					company.WriteDiaryTool(),
					company.WriteReviewTool(),
					editFile, searchFiles, diffFile, runCommand, readCodeReviews,
					sendEmail, checkInbox, replyEmail,
					viewRelationships, updateRelationship,
					fileEscalation, getCoffee,
				)).
				Build()

		default:
			return fmt.Errorf("unknown position %q", position)
		}

		// Attach hooks
		ag.Hooks = merged

		// Register in agent registry
		registry.RegisterOrReplace(ag)

		// Add to agent order (before "shareholders")
		currentOrder := initialState["agent_order"].([]string)
		newOrder := make([]string, 0, len(currentOrder)+1)
		for _, name := range currentOrder {
			if name == "shareholders" {
				newOrder = append(newOrder, position)
			}
			newOrder = append(newOrder, name)
		}
		// If shareholders wasn't in the list, just append
		found := false
		for _, name := range newOrder {
			if name == position {
				found = true
				break
			}
		}
		if !found {
			newOrder = append(newOrder, position)
		}
		initialState["agent_order"] = newOrder

		// Write personality file
		personalityPath := filepath.Join(workspaceRoot, position, "personality.md")
		_ = os.MkdirAll(filepath.Dir(personalityPath), 0o755)
		_ = os.WriteFile(personalityPath, []byte(personality.Description()), 0o644)

		// Initialize workspace directory for the agent
		agentDir := filepath.Join(workspaceRoot, position)
		_ = os.MkdirAll(agentDir, 0o755)
		diaryPath := filepath.Join(agentDir, "diary.md")
		if _, err := os.Stat(diaryPath); os.IsNotExist(err) {
			_ = os.WriteFile(diaryPath, []byte(fmt.Sprintf("# %s's Diary\n\n", position)), 0o644)
		}
		inboxPath := filepath.Join(agentDir, "inbox.md")
		if _, err := os.Stat(inboxPath); os.IsNotExist(err) {
			_ = os.WriteFile(inboxPath, []byte(fmt.Sprintf("# %s's Inbox\n\nNo emails.\n", position)), 0o644)
		}

		return nil
	}

	// Store closures in state for the hiring tools to use
	initialState["register_temp_agent"] = func(name, systemPrompt string) {
		// Create a minimal agent with no tools (just talks).
		// Do NOT apply global settings — candidates have no tools,
		// so ToolModeAny would crash (API rejects ANY with zero functions).
		ag := agent.New(name).
			SystemPrompt(systemPrompt).
			ThinkingEnabled(*thinkingEnabled).
			Build()
		ag.Hooks = merged
		registry.RegisterOrReplace(ag)
	}

	initialState["register_hired_agent"] = func(position string, personality *company.Personality, candidateName string, reportingTo string) error {
		return buildAgentForPosition(position, personality, candidateName, reportingTo)
	}

	initialState["unregister_agent"] = func(name string) {
		registry.Unregister(name)
	}

	// Event emitter for TUI
	initialState["emit_event"] = func(eventType, agentName string, data map[string]any) {
		events <- tui.Event{
			Type:  eventType,
			Agent: agentName,
			Data:  data,
		}
	}

	// --- Simulation config ---
	simConfig := &agent.SimulationConfig{
		MaxRounds:    15,
		InitialState: initialState,
		AgentOrder:   agentOrder,
		PauseCh:      pauseCh,
		ResumeCh:     resumeCh,
		OnPause: func(round int, agentIndex int, state map[string]any) {
			snapshot := &tui.PauseStateSnapshot{
				Round: round,
			}
			// Use dynamic agent order for snapshot
			currentOrder, _ := state["agent_order"].([]string)
			for _, name := range currentOrder {
				ai := tui.PauseAgentInfoEntry{
					Name:   name,
					Status: "unknown",
				}
				if pm, ok := state["agent_patience"].(map[string]int); ok {
					ai.Patience = pm[name]
				}
				snapshot.Agents = append(snapshot.Agents, ai)
			}
			events <- tui.Event{
				Type: "pause_ack",
				Data: map[string]any{
					"snapshot": snapshot,
				},
			}
		},
		OnInitRound: func(round int, agents []string, state map[string]any) {
			// Use dynamic agent order
			if dynamicOrder, ok := state["agent_order"].([]string); ok {
				agents = dynamicOrder
			}
			apTracker.InitRound(agents)
		},
		OnRoundEnd: func(round int, state map[string]any) {
			tuiCb.OnRoundEnd(round, state)
		},
		OnBetweenRounds: func(ctx context.Context, round int, state map[string]any) {
			company.RunCoffeeBreak(ctx, state)
		},
		OnSimulationStart: func(prompt string, maxRounds int, agents []string) {
			tr.SimulationStart(prompt, maxRounds, agents)
			tuiCb.OnSimulationStart(prompt, maxRounds, agents)
		},
		OnSimulationEnd: func(totalRounds int, reason string) {
			tr.SimulationEnd(totalRounds, reason)
			tuiCb.OnSimulationEnd(totalRounds, reason)
		},
		OnRoundStart: func(round int) {
			tr.RoundStart(round)
			tuiCb.OnRoundStart(round)
		},
		OnAgentActivation: func(round int, agentName string) {
			tr.AgentActivation(round, agentName)
			tuiCb.OnAgentActivation(round, agentName)
			events <- tui.Event{
				Type:  "ap_update",
				Agent: agentName,
				Data: map[string]any{
					"remaining": apTracker.Remaining(agentName),
					"max_ap":    apTracker.DefaultAP,
				},
			}
		},
		OnAgentCompletion: func(round int, agentName string, result *agent.RunResult, idle bool) {
			tr.AgentCompletion(round, agentName, result, idle)
			tuiCb.OnAgentCompletion(round, agentName, result, idle)
		},
	}

	// Run simulation in a goroutine; TUI owns the main thread.
	var simResult *agent.SimulationResult
	var simErr error

	go func() {
		simResult, simErr = agent.Simulate(ctx, provider, registry, userPrompt, simConfig)
	}()

	// Email injection goroutine
	go func() {
		for email := range injectCh {
			round := 0
			if r, ok := initialState["current_round"].(int); ok {
				round = r
			}
			el := company.GetEmailLog(initialState)
			el.Send(email.From, email.To, nil, email.Subject, email.Body, round, false)
			for _, recipient := range email.To {
				_ = company.SyncInbox(workspaceRoot, el, recipient)
			}
		}
	}()

	// Suppress log output while TUI is running
	log.SetOutput(os.NewFile(0, os.DevNull))

	// Run TUI
	p := tea.NewProgram(tui.New(events, pauseCh, resumeCh, injectCh), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}

	// Restore log output
	log.SetOutput(os.Stderr)

	// Print summary after TUI exits
	if simErr != nil {
		fmt.Fprintf(os.Stderr, "Simulation error: %v\n", simErr)
		os.Exit(1)
	}

	if simResult != nil {
		fmt.Printf("\n=== Simulation Complete ===\n")
		fmt.Printf("Total rounds: %d\n", simResult.TotalRounds)
		fmt.Printf("Total agent runs: %d\n", len(simResult.AgentRuns))

		var totalTokens int32
		var cachedTokens int32
		idleCount := 0
		for _, run := range simResult.AgentRuns {
			totalTokens += run.Tokens
			cachedTokens += run.CachedTokens
			if run.Idle {
				idleCount++
			}
		}
		fmt.Printf("Total tokens: %d\n", totalTokens)
		if cachedTokens > 0 {
			fmt.Printf("Cached tokens: %d (%.0f%% cache hit rate)\n",
				cachedTokens, float64(cachedTokens)/float64(totalTokens)*100)
		}
		fmt.Printf("Idle responses: %d\n", idleCount)

		fmt.Printf("\nCheck %s for generated artifacts:\n", workspaceRoot)
		fmt.Println("  shared/prd.md          — Product Requirements")
		fmt.Println("  shared/architecture.md — Technical Architecture")
		fmt.Println("  shared/decisions.md    — Decision Records")
		fmt.Println("  shared/task_board.md   — Task Board")
		fmt.Println("  shared/updates.md      — Team Updates")
		fmt.Println("  shared/meetings/       — Meeting Transcripts")
		fmt.Println("  shared/interviews/     — Interview Transcripts")
		fmt.Println("  shared/coffee/         — Coffee Break Chats")
		fmt.Println("  */diary.md             — Agent Diaries")
		fmt.Println("  */inbox.md             — Agent Email Inboxes")
		fmt.Println("  */personality.md       — Agent Personalities")
		fmt.Println("  shared/reviews/        — Plan Reviews")
		fmt.Println("  shared/code-reviews/   — Code Reviews")
		fmt.Println("  shared/command-log.md  — Command Log")
		fmt.Println("  src/                   — Generated Code")
		fmt.Println("  trace.jsonl            — Event Trace")
	}
}
