// Package main demonstrates nested handoffs with shared state.
//
// A "planner" agent receives a user request and hands off to a "generator"
// agent that builds an app design using tools. When the generator finishes,
// control returns to the planner which summarises the final design from state.
//
// Run with: GEMINI_API_KEY=... go run ./examples/appdesign "Design a todo app"
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kacperpaczos/agents/agent"
	"github.com/kacperpaczos/agents/capabilities/appdesign"
	"github.com/kacperpaczos/agents/llm"
	"github.com/kacperpaczos/agents/prompt"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Set GEMINI_API_KEY to run this example")
		os.Exit(1)
	}

	userPrompt := "Design a todo app with a web frontend, REST API, and database"
	if len(os.Args) > 1 {
		userPrompt = strings.Join(os.Args[1:], " ")
	}

	ctx := context.Background()

	client, err := llm.New(ctx, apiKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Client error: %v\n", err)
		os.Exit(1)
	}

	registry := agent.NewRegistry()

	registry.Register(agent.New("generator").
		PromptBuilder(prompt.NewBuilder().
			Add(prompt.Identity("You are an application architecture generator.")).
			Add(prompt.ToolUsage(
				"Use add_component to create components, add_connection to wire them together. "+
					"Use list_components to review the current design. "+
					"Use remove_component or remove_connection to fix mistakes.")).
			Add(prompt.OutputFormat(
				"After building the design, call list_components once to verify, "+
					"then respond with a brief summary of what you created."))).
		Tools(
			appdesign.AddComponentTool(),
			appdesign.RemoveComponentTool(),
			appdesign.AddConnectionTool(),
			appdesign.RemoveConnectionTool(),
			appdesign.ListComponentsTool(),
		).
		Build())

	registry.Register(agent.New("planner").
		PromptBuilder(prompt.NewBuilder().
			Add(prompt.Identity("You are a software project planner.")).
			Add(prompt.HandoffPolicy(
				"When the user requests an application design, transfer to the 'generator' agent. "+
					"When the generator completes, use list_components to review the final design, "+
					"then present the result to the user.")).
			Add(prompt.OutputFormat(
				"Present the final design as a structured summary with components and connections."))).
		Tool(appdesign.ListComponentsTool()).
		HandoffTo("generator").
		Build())

	if err := registry.Finalize(); err != nil {
		fmt.Fprintf(os.Stderr, "Registry error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("User: %s\n\n", userPrompt)

	config := &agent.OrchestratorConfig{InitialState: make(map[string]any)}
	result, err := agent.Orchestrate(ctx, client, registry, "planner", userPrompt, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Agent: %s\n", result.FinalText)
	fmt.Printf("\n--- Stats ---\n")
	fmt.Printf("Tokens used: %d\n", result.TotalTokens)
	fmt.Printf("Iterations:  %d\n", result.Iterations)
	fmt.Printf("Terminated:  %s\n", result.TerminateReason)

	if result.State != nil {
		if d, ok := result.State["design"].(*appdesign.AppDesign); ok && len(d.Components) > 0 {
			fmt.Printf("\n--- Final Design (from state) ---\n%s\n", d)
		}
	}
}
