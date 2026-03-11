package main

import (
	"context"
	"fmt"

	"github.com/dntatme/agents/agent"
	"github.com/dntatme/agents/llm"
	"github.com/dntatme/agents/tool"
)

// Example demonstrating how to run agents in function call-only mode,
// where they can ONLY call tools and cannot generate freeform text.

func main() {
	ctx := context.Background()

	// Create a Gemini provider
	provider, err := llm.NewGemini(ctx, "YOUR_API_KEY")
	if err != nil {
		panic(err)
	}
	predictor := agent.NewLLMPredictor(provider)

	// Create a tool registry
	registry := tool.NewRegistry()
	// Add some tools...
	// registry.Register(...)

	// Create an agent with ToolMode set to ANY (function call-only mode)
	toolModeAny := llm.ToolModeAny
	ag := &agent.Agent{
		Name:              "ToolOnlyAgent",
		Model:             "gemini-2.0-flash",
		SystemPrompt:      "You are a helpful assistant. You can only respond by calling tools.",
		Tools:             registry,
		TerminationPolicy: agent.DefaultTermination(),
		ToolMode:          &toolModeAny, // This forces the agent to ONLY call tools
	}

	// Run the agent
	result, err := ag.Run(ctx, predictor, nil)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Agent completed after %d iterations\n", result.Iterations)
}
