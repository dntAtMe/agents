// Package main demonstrates a multi-agent weather example.
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
	"github.com/kacperpaczos/agents/llm"
	"github.com/kacperpaczos/agents/prompt"
	"github.com/kacperpaczos/agents/tool"
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

	// --- Tools ---
	getWeather := &tool.FuncTool{
		Decl: &genai.FunctionDeclaration{
			Name:        "get_weather",
			Description: "Get the current weather for a given city.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"city": {
						Type:        genai.TypeString,
						Description: "City name, e.g. 'London'.",
					},
				},
				Required: []string{"city"},
			},
		},
		Fn: func(_ context.Context, args map[string]any) (map[string]any, error) {
			city, _ := args["city"].(string)
			// Simulated weather data.
			data := map[string]map[string]any{
				"london":    {"temperature": "15°C", "condition": "Rainy", "humidity": "80%"},
				"new york":  {"temperature": "22°C", "condition": "Sunny", "humidity": "45%"},
				"tokyo":     {"temperature": "18°C", "condition": "Cloudy", "humidity": "70%"},
			}
			if w, ok := data[strings.ToLower(city)]; ok {
				w["city"] = city
				return w, nil
			}
			return map[string]any{
				"city":        city,
				"temperature": "20°C",
				"condition":   "Unknown",
				"humidity":    "50%",
			}, nil
		},
	}

	// --- Agents ---
	registry := agent.NewRegistry()

	weatherTools := tool.NewRegistry()
	weatherTools.Register(getWeather)

	registry.Register(&agent.Agent{
		Name:  "weather",
		Model: "gemini-2.0-flash",
		PromptBuilder: prompt.NewBuilder().
			Add(prompt.Identity("You are a friendly weather assistant.")).
			Add(prompt.ToolUsage("Use the get_weather tool to look up weather data for the requested city.")).
			Add(prompt.OutputFormat("Give a concise, helpful summary of the weather conditions.")),
		Tools:             weatherTools,
		TerminationPolicy: agent.DefaultTermination(),
	})

	triageTools := tool.NewRegistry()
	triageTools.Register(tool.NewTransferTool([]string{"weather"}))

	registry.Register(&agent.Agent{
		Name:  "triage",
		Model: "gemini-2.0-flash",
		PromptBuilder: prompt.NewBuilder().
			Add(prompt.Identity("You are a triage agent.")).
			Add(prompt.HandoffPolicy("Route weather questions to the 'weather' agent using transfer_to_agent. For other questions, answer directly.")),
		Tools:             triageTools,
		TerminationPolicy: agent.DefaultTermination(),
	})

	// --- Run ---
	fmt.Printf("User: %s\n\n", userPrompt)

	result, err := agent.Orchestrate(ctx, client, registry, "triage", userPrompt, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Agent: %s\n", result.FinalText)
	fmt.Printf("\n--- Stats ---\n")
	fmt.Printf("Tokens used: %d\n", result.TotalTokens)
	fmt.Printf("Iterations:  %d\n", result.Iterations)
	fmt.Printf("Terminated:  %s\n", result.TerminateReason)
}
