package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kacperpaczos/agents/agent"
	"github.com/kacperpaczos/agents/capabilities/weather"
	"github.com/kacperpaczos/agents/llm"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "GEMINI_API_KEY environment variable is required")
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: agents <prompt>")
		os.Exit(1)
	}
	prompt := strings.Join(os.Args[1:], " ")

	ctx := context.Background()

	client, err := llm.New(ctx, apiKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create client: %v\n", err)
		os.Exit(1)
	}

	registry := agent.NewRegistry()

	registry.Register(agent.New("weather").
		SystemPrompt("You are a weather specialist. Use the get_weather tool to look up weather information and provide a helpful summary to the user.").
		Tool(weather.GetWeatherTool()).
		Build())

	registry.Register(agent.New("triage").
		SystemPrompt("You are a triage agent. If the user asks about weather, transfer to the \"weather\" agent using the transfer_to_agent tool. Otherwise, answer directly.").
		HandoffTo("weather").
		Build())

	if err := registry.Finalize(); err != nil {
		fmt.Fprintf(os.Stderr, "Registry error: %v\n", err)
		os.Exit(1)
	}

	result, err := agent.Orchestrate(ctx, client, registry, "triage", prompt, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result.FinalText)
	fmt.Fprintf(os.Stderr, "\n[tokens: %d | iterations: %d | reason: %s]\n",
		result.TotalTokens, result.Iterations, result.TerminateReason)
}
