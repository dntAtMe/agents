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

	"github.com/dntatme/agents/agent"
	"github.com/dntatme/agents/capabilities/company"
	"github.com/dntatme/agents/llm"
	"github.com/dntatme/agents/prompt"
)

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
	fmt.Printf("Workspace: %s\n", workspaceRoot)

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

	fmt.Println("Personality assignments:")
	for _, name := range agentNames {
		fmt.Printf("  %s → %s\n", name, personalities[name].Name)
	}
	fmt.Println()

	// Write personality files to workspace
	for _, name := range agentNames {
		p := personalities[name]
		personalityPath := filepath.Join(workspaceRoot, name, "personality.md")
		_ = os.MkdirAll(filepath.Dir(personalityPath), 0o755)
		_ = os.WriteFile(personalityPath, []byte(fmt.Sprintf("# Personality: %s\n\n%s\n", p.Name, p.Description)), 0o644)
	}

	// Shared ask_agent tool for all agents
	askAgent := company.AskAgentTool()

	// Helper to build personality mixin
	personalityMixin := func(name string) prompt.Mixin {
		return prompt.Mixin{Name: "Personality", Content: personalities[name].Description}
	}

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
				"Use ask_agent to directly message any team member for quick questions or clarifications.")).
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
			askAgent,
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
					"Use ask_agent to directly message team members for clarifications.")).
			Add(prompt.Context(contextInstruction)).
			Add(prompt.Guardrails(diaryInstruction+"\n"+idleInstruction))).
		Tools(
			company.ReadFileTool(),
			company.WriteFileTool(),
			company.ListFilesTool(),
			company.PostUpdateTool(),
			company.ReadUpdatesTool(),
			company.WriteDiaryTool(),
			askAgent,
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
					"Use ask_agent to directly message team members for quick technical questions.")).
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
			askAgent,
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
					"Use ask_agent to directly message developers for clarifications on their plans.")).
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
			askAgent,
		).
		HandoffTo("backend-dev", "frontend-dev", "devops").
		Build())

	// Project Manager
	registry.Register(agent.New("project-manager").
		PromptBuilder(prompt.NewBuilder().
			Add(prompt.Identity(
				"You are the Project Manager. You break work into tasks, track progress, "+
					"and identify blockers. Read the PRD and architecture to create tasks. "+
					"Monitor task statuses each round and post status updates.")).
			Add(personalityMixin("project-manager")).
			Add(prompt.ToolUsage(
				"Use add_task to create new tasks. Use update_task to change statuses. "+
					"Use read_task_board to review current state. "+
					"Use post_update to announce task changes. "+
					"Use ask_agent to directly message team members about blockers or status.")).
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
			askAgent,
		).
		Build())

	// Backend Developer
	registry.Register(agent.New("backend-dev").
		PromptBuilder(prompt.NewBuilder().
			Add(prompt.Identity(
				"You are the Backend Developer. You implement server-side code. "+
					"Your workflow: 1) Read assigned tasks from the task board. "+
					"2) Write an implementation plan to backend-dev/plans/TASK-{id}-plan.md. "+
					"3) Post update requesting architect review. "+
					"4) Once approved, implement code in src/backend/. "+
					"5) Update task status to 'done'.")).
			Add(personalityMixin("backend-dev")).
			Add(prompt.ToolUsage(
				"Use read_task_board to find your assigned tasks. "+
					"Use write_file for plans and source code. "+
					"Use read_file to check architect reviews. "+
					"Use update_task to change task status. "+
					"Use post_update to request reviews. "+
					"Use ask_agent to directly message the architect for quick feedback.")).
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
			askAgent,
		).
		Build())

	// Frontend Developer
	registry.Register(agent.New("frontend-dev").
		PromptBuilder(prompt.NewBuilder().
			Add(prompt.Identity(
				"You are the Frontend Developer. You implement client-side code. "+
					"Your workflow: 1) Read assigned tasks from the task board. "+
					"2) Write an implementation plan to frontend-dev/plans/TASK-{id}-plan.md. "+
					"3) Post update requesting architect review. "+
					"4) Once approved, implement code in src/frontend/. "+
					"5) Update task status to 'done'.")).
			Add(personalityMixin("frontend-dev")).
			Add(prompt.ToolUsage(
				"Use read_task_board to find your assigned tasks. "+
					"Use write_file for plans and source code. "+
					"Use read_file to check architect reviews. "+
					"Use update_task to change task status. "+
					"Use post_update to request reviews. "+
					"Use ask_agent to directly message the architect for quick feedback.")).
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
			askAgent,
		).
		Build())

	// DevOps Engineer
	registry.Register(agent.New("devops").
		PromptBuilder(prompt.NewBuilder().
			Add(prompt.Identity(
				"You are the DevOps Engineer. You handle infrastructure, CI/CD, and deployment. "+
					"Your workflow: 1) Read assigned tasks from the task board. "+
					"2) Write an implementation plan to devops/plans/TASK-{id}-plan.md. "+
					"3) Post update requesting architect review. "+
					"4) Once approved, implement infrastructure in src/infra/. "+
					"5) Update task status to 'done'.")).
			Add(personalityMixin("devops")).
			Add(prompt.ToolUsage(
				"Use read_task_board to find your assigned tasks. "+
					"Use write_file for plans and infra code. "+
					"Use read_file to check architect reviews. "+
					"Use update_task to change task status. "+
					"Use post_update to request reviews. "+
					"Use ask_agent to directly message the architect for quick feedback.")).
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
			askAgent,
		).
		Build())

	// Validate all handoff targets
	if err := registry.Finalize(); err != nil {
		fmt.Fprintf(os.Stderr, "Registry error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Project: %s\n\n", userPrompt)

	// Run simulation
	result, err := agent.Simulate(ctx, client, registry, userPrompt, &agent.SimulationConfig{
		MaxRounds: 10,
		InitialState: map[string]any{
			"workspace_root": workspaceRoot,
			"project_name":   userPrompt,
		},
		AgentOrder: []string{
			"ceo", "product-manager", "cto", "architect",
			"project-manager", "backend-dev", "frontend-dev", "devops",
		},
		OnRoundEnd: func(round int, state map[string]any) {
			log.Printf("=== Round %d complete ===\n", round)
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Simulation error: %v\n", err)
		os.Exit(1)
	}

	// Print summary
	fmt.Printf("\n=== Simulation Complete ===\n")
	fmt.Printf("Total rounds: %d\n", result.TotalRounds)
	fmt.Printf("Total agent runs: %d\n", len(result.AgentRuns))

	var totalTokens int32
	idleCount := 0
	for _, run := range result.AgentRuns {
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
	fmt.Println("  */diary.md             — Agent Diaries")
	fmt.Println("  */personality.md       — Agent Personalities")
	fmt.Println("  architect/reviews/     — Code Reviews")
	fmt.Println("  src/                   — Generated Code")
}
