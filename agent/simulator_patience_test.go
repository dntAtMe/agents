package agent

import (
	"strings"
	"testing"
)

func TestInitializeAgentPatienceDefaults(t *testing.T) {
	state := map[string]any{}

	initializeAgentPatience(state, []string{"architect", "backend-dev"})

	patience := getAgentPatienceMap(state)
	if patience["architect"] != defaultPatience {
		t.Fatalf("expected architect patience %d, got %d", defaultPatience, patience["architect"])
	}
	if patience["backend-dev"] != defaultPatience {
		t.Fatalf("expected backend-dev patience %d, got %d", defaultPatience, patience["backend-dev"])
	}
	if patience["ceo"] != defaultPatience {
		t.Fatalf("expected ceo patience %d, got %d", defaultPatience, patience["ceo"])
	}
}

func TestUpdateAgentPatienceAfterRun(t *testing.T) {
	state := map[string]any{
		"project_status": "active",
	}
	setAgentPatience(state, "architect", 50)

	updated := updateAgentPatienceAfterRun(state, "architect", false)
	if updated != 47 {
		t.Fatalf("expected patience 47 after active non-idle run, got %d", updated)
	}

	updated = updateAgentPatienceAfterRun(state, "architect", true)
	if updated != 39 {
		t.Fatalf("expected patience 39 after idle run, got %d", updated)
	}

	state["project_status"] = "complete"
	updated = updateAgentPatienceAfterRun(state, "architect", false)
	if updated != 44 {
		t.Fatalf("expected patience recovery to 44 on completion, got %d", updated)
	}
}

func TestBuildActivationPromptIncludesPatience(t *testing.T) {
	prompt := buildActivationPrompt("architect", 2, 1, 42, map[string]any{})

	if !strings.Contains(prompt, "Current patience level: 42/100") {
		t.Fatalf("expected patience line in prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "impatient and terse") {
		t.Fatalf("expected patience tier description in prompt, got: %s", prompt)
	}
}

func TestBuildActivationPromptIncludesCompanyContext(t *testing.T) {
	state := map[string]any{
		"company_context_renderer": func(agentName string) string {
			if agentName == "ceo" {
				return "Company thesis: Signal Forge"
			}
			return ""
		},
		"company_phase":      "founder_discovery",
		"founder_max_rounds": 10,
	}

	prompt := buildActivationPrompt("ceo", 3, 2, 80, state)

	if !strings.Contains(prompt, "Company thesis: Signal Forge") {
		t.Fatalf("expected company context in prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "Founder discovery status:") {
		t.Fatalf("expected founder discovery guidance in prompt, got: %s", prompt)
	}
}
