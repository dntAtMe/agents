// Package main demonstrates the company simulation with 8 agents collaborating
// in a round-based loop. Each agent wakes up, checks for work, acts, and writes
// a personal diary entry.
//
// Run with: GEMINI_API_KEY=... go run ./examples/company "Build a simple todo REST API"
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"google.golang.org/genai"

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
		AfterPredict: func(ctx context.Context, hc *agent.HookContext, content *genai.Content) error {
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
		BeforeToolCall: func(ctx context.Context, hc *agent.HookContext, fc *genai.FunctionCall) error {
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
		AfterToolCall: func(ctx context.Context, hc *agent.HookContext, fc *genai.FunctionCall, result map[string]any) error {
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

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Set GEMINI_API_KEY to run this example")
		os.Exit(1)
	}

	userPrompt := "Build a simple todo REST API with CRUD operations"
	if len(os.Args) > 1 {
		userPrompt = strings.Join(os.Args[1:], " ")
	}

	ctx := context.Background()

	client, err := llm.New(ctx, apiKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Client error: %v\n", err)
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

	// Shared diary instruction appended to all agent prompts
	diaryInstruction := "At the end of your turn, always write a diary entry with write_diary. " +
		"Be honest and personal — reflect on your work, the project direction, " +
		"your thoughts about the team's work, frustrations, and celebrations."

	idleInstruction := "If there is nothing for you to do this round, respond with just 'IDLE'."

	energyInstruction := "You have limited action points (AP) per round. Read-only actions are cheap (1 AP), " +
		"writing costs more (2-3 AP), meetings cost 5 AP. Budget your turn wisely. " +
		"Use get_coffee to take a coffee break between rounds and get +5 bonus AP next round."

	contextInstruction := "This is a simulation. Read updates since your last active round to catch up. " +
		"Use read_updates, read_task_board, and read_file to understand the current state before acting."

	// --- Assign personalities ---
	// Agents that get personalities (shareholders is excluded)
	personalityAgents := []string{
		"ceo", "product-manager", "cto", "architect",
		"project-manager", "backend-dev", "frontend-dev", "devops",
	}
	// All agents including shareholders
	agentNames := []string{
		"ceo", "product-manager", "cto", "architect",
		"project-manager", "backend-dev", "frontend-dev", "devops",
		"shareholders",
	}
	personalities := company.AssignPersonalities(personalityAgents)
	shareholderTemperament := company.AssignShareholderTemperament()

	// Write personality files to workspace (skip shareholders — handled separately)
	for _, name := range personalityAgents {
		p := personalities[name]
		personalityPath := filepath.Join(workspaceRoot, name, "personality.md")
		_ = os.MkdirAll(filepath.Dir(personalityPath), 0o755)
		_ = os.WriteFile(personalityPath, []byte(p.Description()), 0o644)
	}
	// Write shareholders temperament file
	{
		personalityPath := filepath.Join(workspaceRoot, "shareholders", "personality.md")
		_ = os.MkdirAll(filepath.Dir(personalityPath), 0o755)
		_ = os.WriteFile(personalityPath, []byte(shareholderTemperament.Render()), 0o644)
	}

	// --- Build Org Hierarchy ---
	orgHierarchy := company.NewOrgHierarchy()
	orgHierarchy.SetManager("product-manager", "ceo")
	orgHierarchy.SetManager("cto", "ceo")
	orgHierarchy.SetManager("project-manager", "ceo")
	orgHierarchy.SetManager("architect", "cto")
	orgHierarchy.SetManager("backend-dev", "architect")
	orgHierarchy.SetManager("frontend-dev", "architect")
	orgHierarchy.SetManager("devops", "architect")

	// Shared tools for all agents
	sendEmail := company.SendEmailTool()
	checkInbox := company.CheckInboxTool()
	replyEmail := company.ReplyEmailTool()

	// Relationship tools (all agents)
	viewRelationships := company.ViewRelationshipsTool()
	updateRelationship := company.UpdateRelationshipTool()

	// Escalation tools
	fileEscalation := company.FileEscalationTool()
	viewEscalations := company.ViewEscalationsTool()
	respondToEscalation := company.RespondToEscalationTool()
	recordPiP := company.RecordPiPTool()

	// Firing tools
	requestFire := company.RequestFireTool()
	viewFireRequests := company.ViewFireRequestsTool()
	approveFire := company.ApproveFireTool()

	// Meeting tool — only non-leaf agents can call meetings
	nonLeafAgents := []string{"ceo", "product-manager", "cto", "architect", "project-manager"}
	callMeeting := company.CallGroupMeetingTool(nonLeafAgents)

	// Code editing & search tools
	editFile := company.EditFileTool()
	searchFiles := company.SearchFilesTool()
	diffFile := company.DiffFileTool()

	// Coffee break tool (all agents)
	getCoffee := company.GetCoffeeTool()

	// Command execution tool
	runCommand := company.RunCommandTool()

	// Structured code review tools
	startCodeReview := company.StartCodeReviewTool()
	addReviewComment := company.AddReviewCommentTool()
	submitCodeReview := company.SubmitCodeReviewTool()
	readCodeReviews := company.ReadCodeReviewsTool()

	// Helper to build identity mixin from personality role
	identityMixin := func(name string) prompt.Mixin {
		return prompt.Identity(personalities[name].Role)
	}

	// Helper to build personality mixin
	personalityMixin := func(name string) prompt.Mixin {
		p := personalities[name]
		return prompt.Mixin{Name: "Personality", Content: fmt.Sprintf(
			"**Personality:** %s\n**Work ethic:** %s\n\n**Motivation:** %s\n\n**Communication style:** %s\n\n**Work culture:** %s",
			p.Name, p.WorkEthic, p.Motivation, p.CommunicationStyle, p.WorkCulture,
		)}
	}

	// Communication instructions
	meetingEmailInstruction := "Always check_inbox at the start of your turn — do not skip this. " +
		"Read and reply to any emails that need a response before doing other work. " +
		"Use send_email to send requests, status updates, or questions to colleagues. " +
		"Use send_email with urgent=true when you need an immediate response from a colleague. " +
		"Use call_group_meeting to organize multi-agent discussions when needed."
	emailOnlyInstruction := "Always check_inbox at the start of your turn — do not skip this. " +
		"Read and reply to any emails that need a response before doing other work. " +
		"Use send_email to send requests, status updates, or questions to colleagues. " +
		"Use send_email with urgent=true when you need an immediate response from a colleague."

	// Relationship & escalation instructions
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

	// Coding workflow instructions for developers
	codingWorkflowInstruction := "Your coding workflow: 1) Search existing code with search_files to understand context. " +
		"2) Write/edit code with write_file or edit_file. 3) Run build and tests with run_command. " +
		"4) If build/tests fail, fix the code and re-run. Iterate until passing. " +
		"5) Update task to 'awaiting_review' only when build succeeds. " +
		"6) After review, use read_code_reviews to see inline feedback, then fix with edit_file."

	// Code review workflow instructions for reviewers
	codeReviewInstruction := "When reviewing code: 1) Read the source files with read_file and search_files. " +
		"2) Use start_code_review to begin a review, then add_review_comment for each issue " +
		"with specific file, line, and severity. 3) Use submit_code_review with your verdict. " +
		"4) Optionally run_command to verify the implementation builds/passes tests."

	// --- Register all 8 agents ---
	registry := agent.NewRegistry()

	// CEO
	registry.Register(agent.New("ceo").
		PromptBuilder(prompt.NewBuilder().
			Add(identityMixin("ceo")).
			Add(personalityMixin("ceo")).
			Add(prompt.HandoffPolicy(
				"Delegate to product-manager for requirements and PRD writing. "+
					"Delegate to cto for technical architecture and development oversight. "+
					"Delegate to project-manager for task breakdown and tracking. "+
					"You can change project direction mid-stream if needed.")).
			Add(prompt.ToolUsage(
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
		).
		HandoffTo("product-manager", "cto", "project-manager").
		Build())

	// Product Manager
	registry.Register(agent.New("product-manager").
		PromptBuilder(prompt.NewBuilder().
			Add(identityMixin("product-manager")).
			Add(personalityMixin("product-manager")).
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
			getCoffee,
		).
		Build())

	// CTO
	registry.Register(agent.New("cto").
		PromptBuilder(prompt.NewBuilder().
			Add(identityMixin("cto")).
			Add(personalityMixin("cto")).
			Add(prompt.HandoffPolicy(
				"Delegate detailed design and code review to the architect.")).
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

			searchFiles,
			diffFile,
			startCodeReview,
			addReviewComment,
			submitCodeReview,
			readCodeReviews,

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
			getCoffee,
			company.CheckStockPriceTool(),
		).
		HandoffTo("architect").
		Build())

	// Software Architect
	registry.Register(agent.New("architect").
		PromptBuilder(prompt.NewBuilder().
			Add(identityMixin("architect")).
			Add(personalityMixin("architect")).
			Add(prompt.HandoffPolicy(
				"You can delegate implementation work to backend-dev, frontend-dev, or devops.")).
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

			searchFiles,
			diffFile,
			runCommand,
			startCodeReview,
			addReviewComment,
			submitCodeReview,
			readCodeReviews,

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
			getCoffee,
		).
		HandoffTo("backend-dev", "frontend-dev", "devops").
		Build())

	// Project Manager
	registry.Register(agent.New("project-manager").
		PromptBuilder(prompt.NewBuilder().
			Add(identityMixin("project-manager")).
			Add(personalityMixin("project-manager")).
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
			getCoffee,
		).
		Build())

	// Backend Developer
	registry.Register(agent.New("backend-dev").
		PromptBuilder(prompt.NewBuilder().
			Add(identityMixin("backend-dev")).
			Add(personalityMixin("backend-dev")).
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

			editFile,
			searchFiles,
			diffFile,
			runCommand,
			readCodeReviews,

			sendEmail,
			checkInbox,
			replyEmail,
			viewRelationships,
			updateRelationship,
			fileEscalation,
			getCoffee,
		).
		Build())

	// Frontend Developer
	registry.Register(agent.New("frontend-dev").
		PromptBuilder(prompt.NewBuilder().
			Add(identityMixin("frontend-dev")).
			Add(personalityMixin("frontend-dev")).
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

			editFile,
			searchFiles,
			diffFile,
			runCommand,
			readCodeReviews,

			sendEmail,
			checkInbox,
			replyEmail,
			viewRelationships,
			updateRelationship,
			fileEscalation,
			getCoffee,
		).
		Build())

	// DevOps Engineer
	registry.Register(agent.New("devops").
		PromptBuilder(prompt.NewBuilder().
			Add(identityMixin("devops")).
			Add(personalityMixin("devops")).
			Add(prompt.ToolUsage(
				"Use read_task_board to find your assigned tasks. "+
					"Use write_file for plans and infra code. "+
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

			editFile,
			searchFiles,
			diffFile,
			runCommand,
			readCodeReviews,

			sendEmail,
			checkInbox,
			replyEmail,
			viewRelationships,
			updateRelationship,
			fileEscalation,
			getCoffee,
		).
		Build())

	// Shareholders (runs last each round, assesses company performance)
	registry.Register(agent.New("shareholders").
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
		Build())

	// Validate all handoff targets
	if err := registry.Finalize(); err != nil {
		fmt.Fprintf(os.Stderr, "Registry error: %v\n", err)
		os.Exit(1)
	}

	// Initialize action point tracker
	apTracker := company.NewActionPointTracker(15, 5, 3)

	// Attach merged hooks (tracer + TUI + AP) to all agents
	tracerHooks := tr.Hooks()
	apHooks := &agent.Hooks{
		BeforeToolCall: func(ctx context.Context, hc *agent.HookContext, fc *genai.FunctionCall) error {
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

			// Send AP cost to TUI (enriches the tool_call_start event data)
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
		AfterToolCall: func(ctx context.Context, hc *agent.HookContext, fc *genai.FunctionCall, result map[string]any) error {
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
	for _, name := range agentNames {
		ag := registry.Lookup(name)
		if ag != nil {
			ag.Hooks = merged
		}
	}

	// Initialize stock tracker
	stockTracker := company.NewStockTracker(100.0)

	// Build initial state with org hierarchy and relationship renderer
	initialState := map[string]any{
		"workspace_root":          workspaceRoot,
		"project_name":            userPrompt,
		company.KeyOrgHierarchy:   orgHierarchy,
		company.KeyFiredAgents:    map[string]bool{},
		company.KeyActionPoints:   apTracker,
		company.KeyStockPrice:     stockTracker,
	}

	// Store relationship renderer as a closure (avoids circular import)
	initialState["relationship_renderer"] = func(agentName string) string {
		return company.GetRelationshipLog(initialState).RenderForAgent(agentName)
	}

	// Store diary renderer for memory continuity between rounds
	initialState["diary_renderer"] = func(agentName string) string {
		return company.LastDiaryEntry(workspaceRoot, agentName)
	}

	// Store AP renderer for activation prompts
	initialState["ap_renderer"] = func(agentName string) string {
		remaining := apTracker.Remaining(agentName)
		return fmt.Sprintf(
			"You have %d action points this round. Each action costs points (read=1, write=2-3, meetings=5). "+
				"Budget your turn wisely. Use get_coffee to take a coffee break and get +5 bonus AP next round.",
			remaining,
		)
	}

	// Store stock renderer — only CEO and CTO see stock info in activation prompt
	csuite := map[string]bool{"ceo": true, "cto": true}
	initialState["stock_renderer"] = func(agentName string) string {
		if !csuite[agentName] {
			return ""
		}
		return "📈 " + stockTracker.RenderBrief() + "\n" +
			"Use check_stock_price for detailed history. The shareholders assess performance each round."
	}

	simConfig := &agent.SimulationConfig{
		MaxRounds:    15,
		InitialState: initialState,
		AgentOrder: []string{
			"ceo", "product-manager", "cto", "architect",
			"project-manager", "backend-dev", "frontend-dev", "devops",
			"shareholders",
		},
		PauseCh:  pauseCh,
		ResumeCh: resumeCh,
		OnPause: func(round int, agentIndex int, state map[string]any) {
			// Build state snapshot for the TUI
			snapshot := &tui.PauseStateSnapshot{
				Round: round,
			}
			for _, name := range agentNames {
				ai := tui.PauseAgentInfoEntry{
					Name:   name,
					Status: "unknown",
				}
				// Extract patience
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
			// Send initial AP to TUI
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
		simResult, simErr = agent.Simulate(ctx, client, registry, userPrompt, simConfig)
	}()

	// Email injection goroutine — watches injectCh for composed emails.
	// Since the simulation is paused when injection happens, state access is safe.
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
		idleCount := 0
		for _, run := range simResult.AgentRuns {
			totalTokens += run.Tokens
			if run.Idle {
				idleCount++
			}
		}
		fmt.Printf("Total tokens: %d\n", totalTokens)
		fmt.Printf("Idle responses: %d\n", idleCount)

		fmt.Printf("\nCheck %s for generated artifacts:\n", workspaceRoot)
		fmt.Println("  shared/prd.md          — Product Requirements")
		fmt.Println("  shared/architecture.md — Technical Architecture")
		fmt.Println("  shared/decisions.md    — Decision Records")
		fmt.Println("  shared/task_board.md   — Task Board")
		fmt.Println("  shared/updates.md      — Team Updates")
		fmt.Println("  shared/meetings/       — Meeting Transcripts")
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
