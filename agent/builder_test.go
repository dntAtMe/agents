package agent

import (
	"context"
	"testing"

	"github.com/kacperpaczos/agents/prompt"
	"github.com/kacperpaczos/agents/termination"
	"github.com/kacperpaczos/agents/tool"

	"google.golang.org/genai"
)

func dummyTool(name string) tool.Tool {
	return &tool.FuncTool{
		Decl: &genai.FunctionDeclaration{Name: name, Description: "test"},
		Fn: func(_ context.Context, _ map[string]any, _ map[string]any) (map[string]any, error) {
			return nil, nil
		},
	}
}

func TestBuilderDefaults(t *testing.T) {
	ag := New("test").Build()

	if ag.Name != "test" {
		t.Errorf("expected name 'test', got %q", ag.Name)
	}
	if ag.Model != "gemini-2.0-flash" {
		t.Errorf("expected default model, got %q", ag.Model)
	}
	if ag.TerminationPolicy == nil {
		t.Error("expected default termination policy")
	}
}

func TestBuilderModel(t *testing.T) {
	ag := New("test").Model("gemini-2.0-pro").Build()
	if ag.Model != "gemini-2.0-pro" {
		t.Errorf("expected 'gemini-2.0-pro', got %q", ag.Model)
	}
}

func TestBuilderSystemPrompt(t *testing.T) {
	ag := New("test").SystemPrompt("You are helpful.").Build()
	if ag.SystemPrompt != "You are helpful." {
		t.Errorf("unexpected system prompt: %q", ag.SystemPrompt)
	}
}

func TestBuilderPromptBuilder(t *testing.T) {
	pb := prompt.NewBuilder().Add(prompt.Identity("Bot"))
	ag := New("test").PromptBuilder(pb).Build()
	if ag.PromptBuilder == nil {
		t.Fatal("expected prompt builder to be set")
	}
	resolved := ag.ResolveSystemPrompt()
	if resolved != "## Identity\nBot" {
		t.Errorf("unexpected resolved prompt: %q", resolved)
	}
}

func TestBuilderToolsRegistered(t *testing.T) {
	ag := New("test").
		Tool(dummyTool("a")).
		Tools(dummyTool("b"), dummyTool("c")).
		Build()

	if ag.Tools.Len() != 3 {
		t.Errorf("expected 3 tools, got %d", ag.Tools.Len())
	}
	if ag.Tools.Lookup("a") == nil {
		t.Error("expected tool 'a'")
	}
	if ag.Tools.Lookup("b") == nil {
		t.Error("expected tool 'b'")
	}
	if ag.Tools.Lookup("c") == nil {
		t.Error("expected tool 'c'")
	}
}

func TestBuilderHandoffTo(t *testing.T) {
	ag := New("triage").
		HandoffTo("weather", "support").
		Build()

	transferTool := ag.Tools.Lookup(tool.TransferToolName)
	if transferTool == nil {
		t.Fatal("expected transfer_to_agent tool")
	}

	def := transferTool.Definition()
	agentProp := def.Parameters.Properties["agent_name"]
	if agentProp == nil {
		t.Fatal("expected agent_name property")
	}
	if len(agentProp.Enum) != 2 {
		t.Errorf("expected 2 enum values, got %d", len(agentProp.Enum))
	}
	if agentProp.Enum[0] != "weather" || agentProp.Enum[1] != "support" {
		t.Errorf("unexpected enum values: %v", agentProp.Enum)
	}
}

func TestBuilderHandoffToWithTools(t *testing.T) {
	ag := New("triage").
		Tool(dummyTool("search")).
		HandoffTo("weather").
		Build()

	if ag.Tools.Len() != 2 {
		t.Errorf("expected 2 tools (search + transfer), got %d", ag.Tools.Len())
	}
	if ag.Tools.Lookup("search") == nil {
		t.Error("expected 'search' tool")
	}
	if ag.Tools.Lookup(tool.TransferToolName) == nil {
		t.Error("expected transfer tool")
	}
}

func TestBuilderTemperature(t *testing.T) {
	ag := New("test").Temperature(0.5).Build()
	if ag.Temperature == nil || *ag.Temperature != 0.5 {
		t.Error("expected temperature 0.5")
	}
}

func TestBuilderMaxOutputTokens(t *testing.T) {
	ag := New("test").MaxOutputTokens(1024).Build()
	if ag.MaxOutputTokens != 1024 {
		t.Errorf("expected 1024, got %d", ag.MaxOutputTokens)
	}
}

func TestBuilderTerminationPolicy(t *testing.T) {
	p := termination.MaxIterations{Max: 5}
	ag := New("test").TerminationPolicy(p).Build()
	if ag.TerminationPolicy == nil {
		t.Error("expected termination policy to be set")
	}
}

func TestBuilderInitialState(t *testing.T) {
	s := map[string]any{"count": 0}
	ag := New("test").InitialState(s).Build()
	if ag.InitialState == nil || ag.InitialState["count"] != 0 {
		t.Error("expected initial state with count=0")
	}
}

func TestBuilderHooks(t *testing.T) {
	h := &Hooks{
		BeforePredict: func(_ context.Context, _ *HookContext, _ *PredictRequest) error {
			return nil
		},
	}
	ag := New("test").Hooks(h).Build()
	if ag.Hooks == nil || ag.Hooks.BeforePredict == nil {
		t.Error("expected hooks to be set")
	}
}

func TestBuilderComplete(t *testing.T) {
	ag := New("weather").
		Model("gemini-2.0-flash").
		PromptBuilder(prompt.NewBuilder().Add(prompt.Identity("Weather bot."))).
		Tool(dummyTool("get_weather")).
		HandoffTo("support").
		Temperature(0.7).
		MaxOutputTokens(2048).
		InitialState(map[string]any{"calls": 0}).
		Build()

	if ag.Name != "weather" {
		t.Errorf("wrong name: %s", ag.Name)
	}
	if ag.Tools.Len() != 2 {
		t.Errorf("expected 2 tools, got %d", ag.Tools.Len())
	}
	if ag.Temperature == nil || *ag.Temperature != 0.7 {
		t.Error("wrong temperature")
	}
	if ag.MaxOutputTokens != 2048 {
		t.Error("wrong max output tokens")
	}
	if ag.InitialState["calls"] != 0 {
		t.Error("wrong initial state")
	}
}
