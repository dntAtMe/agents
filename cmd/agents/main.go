package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"

	"github.com/kacperpaczos/agents/agent"
	"github.com/kacperpaczos/agents/llm"
	"github.com/kacperpaczos/agents/tool"
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

	// --- Define tools ---
	weatherTool := &tool.FuncTool{
		Decl: &genai.FunctionDeclaration{
			Name:        "get_weather",
			Description: "Get the current weather for a city.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"city": {
						Type:        genai.TypeString,
						Description: "The city name.",
					},
				},
				Required: []string{"city"},
			},
		},
		Fn: func(_ context.Context, args map[string]any) (map[string]any, error) {
			city, _ := args["city"].(string)
			// Stub response for demo purposes.
			return map[string]any{
				"city":        city,
				"temperature": "15°C",
				"condition":   "Partly cloudy",
				"humidity":    "62%",
			}, nil
		},
	}

	// --- Build agent registry ---
	registry := agent.NewRegistry()

	// Weather specialist agent.
	weatherTools := tool.NewRegistry()
	weatherTools.Register(weatherTool)

	registry.Register(&agent.Agent{
		Name:              "weather",
		Model:             "gemini-2.0-flash",
		SystemPrompt:      "You are a weather specialist. Use the get_weather tool to look up weather information and provide a helpful summary to the user.",
		Tools:             weatherTools,
		TerminationPolicy: agent.DefaultTermination(),
	})

	// Triage agent that can hand off to weather.
	triageTools := tool.NewRegistry()
	triageTools.Register(tool.NewTransferTool([]string{"weather"}))

	registry.Register(&agent.Agent{
		Name:              "triage",
		Model:             "gemini-2.0-flash",
		SystemPrompt:      "You are a triage agent. If the user asks about weather, transfer to the \"weather\" agent using the transfer_to_agent tool. Otherwise, answer directly.",
		Tools:             triageTools,
		TerminationPolicy: agent.DefaultTermination(),
	})

	// --- Orchestrate ---
	result, err := agent.Orchestrate(ctx, client, registry, "triage", prompt, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result.FinalText)
	fmt.Fprintf(os.Stderr, "\n[tokens: %d | iterations: %d | reason: %s]\n",
		result.TotalTokens, result.Iterations, result.TerminateReason)
}
