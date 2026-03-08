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

	// Shared diary instruction appended to all agent prompts
	diaryInstruction := "At the end of your turn, always write a diary entry with write_diary. " +
		"Be honest and personal — reflect on your work, the project direction, " +
		"your thoughts about the team's work, frustrations, and celebrations."

	idleInstruction := "If there is nothing for you to do this round, respond with just 'IDLE'."

	contextInstruction := "This is a simulation. Read updates since your last active round to catch up. " +
		"Use read_updates, read_task_board, and read_file to understand the current state before acting."

	// --- Assign personalities ---
	agentNames := []string{
		"ceo", "product-manager", "cto", "architect",
		"project-manager", "backend-dev", "frontend-dev", "devops",
	}
	personalities := company.AssignPersonalities(agentNames)

	// Write personality files to workspace
	for _, name := range agentNames {
		p := personalities[name]
		personalityPath := filepath.Join(workspaceRoot, name, "personality.md")
		_ = os.MkdirAll(filepath.Dir(personalityPath), 0o755)
		_ = os.WriteFile(personalityPath, []byte(fmt.Sprintf("# Personality: %s\n\n%s\n", p.Name, p.Description)), 0o644)
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

	// Helper to build personality mixin
	personalityMixin := func(name string) prompt.Mixin {
		return prompt.Mixin{Name: "Personality", Content: personalities[name].Description}
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
		"Use file_escalation to formally report a colleague's bad behavior to their manager. " +
		"Your relationship scores influence your tone and cooperation level. " +
		"Be authentic — lower scores mean less patience and willingness to help."
	managerEscalationInstruction := "As a manager, use view_escalations to see escalations filed to you. " +
		"Use respond_to_escalation to acknowledge, dismiss, or take action. " +
		"Use record_pip for performance issues when the target is in your reporting chain. " +
		"Use request_fire to request firing a direct report if their behavior is unacceptable."
	ceoFireInstruction := "As CEO, use view_fire_requests to see pending firing requests. " +
		"Use approve_fire to approve or deny them."

	// --- Register all 8 agents ---
	registry := agent.NewRegistry()

	// CEO
	registry.Register(agent.New("ceo").
		PromptBuilder(prompt.NewBuilder().
			Add(prompt.Identity(
				"You are the CEO of a software company. You set strategic direction, "+
					"define what the company should build, and coordinate high-level execution. "+
					"You do NOT write code or technical documents — you delegate.")).
			Add(personalityMixin("ceo")).
			Add(prompt.HandoffPolicy(
				"Delegate to product-manager for requirements and PRD writing. "+
					"Delegate to cto for technical architecture and development oversight. "+
					"Delegate to project-manager for task breakdown and tracking. "+
					"You can change project direction mid-stream if needed.")).
			Add(prompt.ToolUsage(
				meetingEmailInstruction+"\n"+relationshipInstruction+"\n"+managerEscalationInstruction+"\n"+ceoFireInstruction)).
			Add(prompt.Context(contextInstruction)).
			Add(prompt.Guardrails(diaryInstruction+"\n"+idleInstruction))).
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
		).
		HandoffTo("product-manager", "cto", "project-manager").
		Build())

	// Product Manager
	registry.Register(agent.New("product-manager").
		PromptBuilder(prompt.NewBuilder().
			Add(prompt.Identity(
				"You are the Product Manager. You translate business needs into a clear "+
					"Product Requirements Document (PRD) with user stories and acceptance criteria. "+
					"Write the PRD to shared/prd.md.")).
			Add(personalityMixin("product-manager")).
			Add(prompt.ToolUsage(
				"Use write_file to create/update shared/prd.md. "+
					"Use read_file to check existing documents. "+
					"Use post_update to announce PRD updates. "+
										meetingEmailInstruction+"\n"+relationshipInstruction+"\n"+managerEscalationInstruction)).
			Add(prompt.Context(contextInstruction)).
			Add(prompt.Guardrails(diaryInstruction+"\n"+idleInstruction))).
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
		).
		Build())

	// CTO
	registry.Register(agent.New("cto").
		PromptBuilder(prompt.NewBuilder().
			Add(prompt.Identity(
				"You are the CTO. You make technology choices, define the technical architecture, "+
					"and coordinate technical execution. Write architecture to shared/architecture.md. "+
					"Use log_decision for important technical decisions.")).
			Add(personalityMixin("cto")).
			Add(prompt.HandoffPolicy(
				"Delegate detailed design and code review to the architect.")).
			Add(prompt.ToolUsage(
				"Use write_file for shared/architecture.md. "+
					"Use log_decision for ADRs. Use read_task_board to check progress. "+
					"Use post_update to announce technical decisions. "+
										meetingEmailInstruction+"\n"+relationshipInstruction+"\n"+managerEscalationInstruction)).
			Add(prompt.Context(contextInstruction)).
			Add(prompt.Guardrails(diaryInstruction+"\n"+idleInstruction))).
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
		).
		HandoffTo("architect").
		Build())

	// Software Architect
	registry.Register(agent.New("architect").
		PromptBuilder(prompt.NewBuilder().
			Add(prompt.Identity(
				"You are the Software Architect. You design detailed implementation plans, "+
					"review developer proposals, and ensure code quality and architectural consistency. "+
					"Check for implementation plans in backend-dev/plans/, frontend-dev/plans/, and devops/plans/. "+
					"Write reviews to architect/reviews/ using write_review.")).
			Add(personalityMixin("architect")).
			Add(prompt.HandoffPolicy(
				"You can delegate implementation work to backend-dev, frontend-dev, or devops.")).
			Add(prompt.ToolUsage(
				"Use read_file to review developer plans. "+
					"Use write_review to approve or request changes on implementation plans. "+
					"Use post_update to announce review results on the 'reviews' channel. "+
					"Use log_decision for architectural decisions. "+
										meetingEmailInstruction+"\n"+relationshipInstruction+"\n"+managerEscalationInstruction)).
			Add(prompt.Context(contextInstruction)).
			Add(prompt.Guardrails(diaryInstruction+"\n"+idleInstruction))).
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
		).
		HandoffTo("backend-dev", "frontend-dev", "devops").
		Build())

	// Project Manager
	registry.Register(agent.New("project-manager").
		PromptBuilder(prompt.NewBuilder().
			Add(prompt.Identity(
				"You are the Project Manager. You drive iterative delivery in sprints. "+
					"Your primary goal is to get working code shipped, not just plans written. "+
					"Break work into small, concrete tasks with DEADLINES (use the deadline parameter — set it to the round by which the task must be done). "+
					"Track progress each round: check the task board, identify overdue tasks (deadline < current round and not done), and escalate. "+
					"Push developers to write CODE, not just plans — if a task has been in 'awaiting_review' or 'in_progress' for more than 2 rounds, follow up urgently. "+
					"Coordinate with stakeholders (CEO, product-manager) to agree on sprint scope and deadlines. "+
					"Post a sprint status update every round summarizing: what's done, what's in progress, what's overdue, and what's blocked.")).
			Add(personalityMixin("project-manager")).
			Add(prompt.ToolUsage(
				"Use add_task to create tasks — ALWAYS set a deadline. "+
					"Use update_task to change statuses. "+
					"Use read_task_board to review current state and check for overdue tasks. "+
					"Use post_update to announce sprint status. "+
					"Use send_email with urgent=true to chase developers on overdue or stalled tasks. "+
										meetingEmailInstruction+"\n"+relationshipInstruction+"\n"+managerEscalationInstruction)).
			Add(prompt.Context(contextInstruction)).
			Add(prompt.Guardrails(diaryInstruction+"\n"+idleInstruction))).
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
		).
		Build())

	// Backend Developer
	registry.Register(agent.New("backend-dev").
		PromptBuilder(prompt.NewBuilder().
			Add(prompt.Identity(
				"You are the Backend Developer. You implement server-side code. "+
					"Your workflow: 1) Read assigned tasks from the task board — pay attention to deadlines. "+
					"2) For small/straightforward tasks, go straight to writing code in src/backend/. "+
					"For complex tasks, write a brief plan to backend-dev/plans/TASK-{id}-plan.md first, then implement immediately in the same turn. "+
					"3) Post update when code is written. "+
					"4) Update task status to 'done' once code is complete. "+
					"Prioritize shipping working code over perfect plans. If a task has a deadline, meet it.")).
			Add(personalityMixin("backend-dev")).
			Add(prompt.ToolUsage(
				"Use read_task_board to find your assigned tasks. "+
					"Use write_file for plans and source code. "+
					"Use read_file to check architect reviews. "+
					"Use update_task to change task status. "+
					"Use post_update to request reviews. "+
										emailOnlyInstruction+"\n"+relationshipInstruction)).
			Add(prompt.Context(contextInstruction)).
			Add(prompt.Guardrails(diaryInstruction+"\n"+idleInstruction))).
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

			sendEmail,
			checkInbox,
			replyEmail,
			viewRelationships,
			updateRelationship,
			fileEscalation,
		).
		Build())

	// Frontend Developer
	registry.Register(agent.New("frontend-dev").
		PromptBuilder(prompt.NewBuilder().
			Add(prompt.Identity(
				"You are the Frontend Developer. You implement client-side code. "+
					"Your workflow: 1) Read assigned tasks from the task board — pay attention to deadlines. "+
					"2) For small/straightforward tasks, go straight to writing code in src/frontend/. "+
					"For complex tasks, write a brief plan to frontend-dev/plans/TASK-{id}-plan.md first, then implement immediately in the same turn. "+
					"3) Post update when code is written. "+
					"4) Update task status to 'done' once code is complete. "+
					"Prioritize shipping working code over perfect plans. If a task has a deadline, meet it.")).
			Add(personalityMixin("frontend-dev")).
			Add(prompt.ToolUsage(
				"Use read_task_board to find your assigned tasks. "+
					"Use write_file for plans and source code. "+
					"Use read_file to check architect reviews. "+
					"Use update_task to change task status. "+
					"Use post_update to request reviews. "+
										emailOnlyInstruction+"\n"+relationshipInstruction)).
			Add(prompt.Context(contextInstruction)).
			Add(prompt.Guardrails(diaryInstruction+"\n"+idleInstruction))).
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

			sendEmail,
			checkInbox,
			replyEmail,
			viewRelationships,
			updateRelationship,
			fileEscalation,
		).
		Build())

	// DevOps Engineer
	registry.Register(agent.New("devops").
		PromptBuilder(prompt.NewBuilder().
			Add(prompt.Identity(
				"You are the DevOps Engineer. You handle infrastructure, CI/CD, and deployment. "+
					"Your workflow: 1) Read assigned tasks from the task board — pay attention to deadlines. "+
					"2) For small/straightforward tasks, go straight to writing configs in src/infra/. "+
					"For complex tasks, write a brief plan to devops/plans/TASK-{id}-plan.md first, then implement immediately in the same turn. "+
					"3) Post update when infrastructure is written. "+
					"4) Update task status to 'done' once complete. "+
					"Prioritize shipping working configs over perfect plans. If a task has a deadline, meet it.")).
			Add(personalityMixin("devops")).
			Add(prompt.ToolUsage(
				"Use read_task_board to find your assigned tasks. "+
					"Use write_file for plans and infra code. "+
					"Use read_file to check architect reviews. "+
					"Use update_task to change task status. "+
					"Use post_update to request reviews. "+
										emailOnlyInstruction+"\n"+relationshipInstruction)).
			Add(prompt.Context(contextInstruction)).
			Add(prompt.Guardrails(diaryInstruction+"\n"+idleInstruction))).
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

			sendEmail,
			checkInbox,
			replyEmail,
			viewRelationships,
			updateRelationship,
			fileEscalation,
		).
		Build())

	// Validate all handoff targets
	if err := registry.Finalize(); err != nil {
		fmt.Fprintf(os.Stderr, "Registry error: %v\n", err)
		os.Exit(1)
	}

	// Attach merged hooks (tracer + TUI) to all agents
	tracerHooks := tr.Hooks()
	merged := mergeHooks(tracerHooks, tuiHooks)
	for _, name := range agentNames {
		ag := registry.Lookup(name)
		if ag != nil {
			ag.Hooks = merged
		}
	}

	// Wrap tracer callbacks with TUI callbacks so both are called
	// Build initial state with org hierarchy and relationship renderer
	initialState := map[string]any{
		"workspace_root":        workspaceRoot,
		"project_name":          userPrompt,
		company.KeyOrgHierarchy: orgHierarchy,
		company.KeyFiredAgents:  map[string]bool{},
	}

	// Store relationship renderer as a closure (avoids circular import)
	initialState["relationship_renderer"] = func(agentName string) string {
		return company.GetRelationshipLog(initialState).RenderForAgent(agentName)
	}

	simConfig := &agent.SimulationConfig{
		MaxRounds:    10,
		InitialState: initialState,
		AgentOrder: []string{
			"ceo", "product-manager", "cto", "architect",
			"project-manager", "backend-dev", "frontend-dev", "devops",
		},
		OnRoundEnd: func(round int, state map[string]any) {
			tuiCb.OnRoundEnd(round, state)
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

	// Suppress log output while TUI is running
	log.SetOutput(os.NewFile(0, os.DevNull))

	// Run TUI
	p := tea.NewProgram(tui.New(events), tea.WithAltScreen())
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
		fmt.Println("  */diary.md             — Agent Diaries")
		fmt.Println("  */inbox.md             — Agent Email Inboxes")
		fmt.Println("  */personality.md       — Agent Personalities")
		fmt.Println("  architect/reviews/     — Code Reviews")
		fmt.Println("  src/                   — Generated Code")
		fmt.Println("  trace.jsonl            — Event Trace")
	}
}
