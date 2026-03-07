// Package main demonstrates a multi-agent weather example with hooks and prompt builder.
//
// Run with: GEMINI_API_KEY=... go run ./examples/weather "What's the weather in London?"
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"

	"github.com/kacperpaczos/agents/agent"
	"github.com/kacperpaczos/agents/capabilities/weather"
	"github.com/kacperpaczos/agents/llm"
	"github.com/kacperpaczos/agents/prompt"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Set GEMINI_API_KEY to run this example")
		os.Exit(1)
	}

	userPrompt := "What's the weather in London?"
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

	registry.Register(agent.New("weather").
		PromptBuilder(prompt.NewBuilder().
			Add(prompt.Identity("You are a friendly weather assistant.")).
			Add(prompt.ToolUsage("Use the get_weather tool to look up weather data for the requested city.")).
			Add(prompt.OutputFormat("Give a concise, helpful summary of the weather conditions."))).
		Tool(weather.GetWeatherTool()).
		Hooks(&agent.Hooks{
			AfterToolCall: func(_ context.Context, hc *agent.HookContext, fc *genai.FunctionCall, result map[string]any) error {
				if fc.Name == "get_weather" && result["city"] != nil {
					hc.State["last_weather_city"] = result["city"]
				}
				return nil
			},
		}).
		Build())

	registry.Register(agent.New("triage").
		PromptBuilder(prompt.NewBuilder().
			Add(prompt.Identity("You are a triage agent.")).
			Add(prompt.HandoffPolicy("Route weather questions to the 'weather' agent using transfer_to_agent. For other questions, answer directly."))).
		HandoffTo("weather").
		Build())

	if err := registry.Finalize(); err != nil {
		fmt.Fprintf(os.Stderr, "Registry error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("User: %s\n\n", userPrompt)

	config := &agent.OrchestratorConfig{InitialState: make(map[string]any)}
	result, err := agent.Orchestrate(ctx, client, registry, "triage", userPrompt, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Agent: %s\n", result.FinalText)
	fmt.Printf("\n--- Stats ---\n")
	fmt.Printf("Tokens used: %d\n", result.TotalTokens)
	fmt.Printf("Iterations:  %d\n", result.Iterations)
	fmt.Printf("Terminated:  %s\n", result.TerminateReason)
	if result.State != nil && result.State["last_weather_city"] != nil {
		fmt.Printf("Last weather city (from hook): %v\n", result.State["last_weather_city"])
	}
}
