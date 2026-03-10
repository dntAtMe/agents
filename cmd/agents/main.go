package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/dntatme/agents/agent"
	"github.com/dntatme/agents/capabilities/weather"
	"github.com/dntatme/agents/llm"
)

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
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: agents <prompt>")
		os.Exit(1)
	}
	prompt := strings.Join(os.Args[1:], " ")

	ctx := context.Background()

	provider, err := createProvider(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Provider error: %v\n", err)
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

	result, err := agent.Orchestrate(ctx, provider, registry, "triage", prompt, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result.FinalText)
	fmt.Fprintf(os.Stderr, "\n[tokens: %d | iterations: %d | reason: %s]\n",
		result.TotalTokens, result.Iterations, result.TerminateReason)
}
