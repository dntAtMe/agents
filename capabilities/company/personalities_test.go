package company

import (
	"testing"
)

func TestPersonalitiesReturnsAll(t *testing.T) {
	ps := Personalities()
	if len(ps) != 12 {
		t.Fatalf("expected 12 personalities, got %d", len(ps))
	}

	names := make(map[string]bool)
	for _, p := range ps {
		if p.Name == "" {
			t.Error("personality name should not be empty")
		}
		if p.Description == "" {
			t.Error("personality description should not be empty")
		}
		names[p.Name] = true
	}

	expected := []string{
		"Lazy Gen Alpha",
		"Edgy Millennial",
		"Overenthusiastic Intern",
		"Grumpy Senior Engineer",
		"Corporate Buzzword Manager",
		"Impatient Deadline Hawk",
		"By-the-Book Compliance Officer",
		"Strategic Bare-Minimumer",
		"Gaslighting Progress Spinner",
		"Machiavellian Credit Hoarder",
		"Chaos Gremlin Prototyper",
		"Perfectionist Scope Creep Artist",
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing personality: %s", name)
		}
	}
}

func TestPersonalitiesReturnsCopy(t *testing.T) {
	ps1 := Personalities()
	ps2 := Personalities()

	ps1[0].Name = "modified"
	if ps2[0].Name == "modified" {
		t.Error("Personalities() should return a copy, not a reference")
	}
}

func TestAssignPersonalities(t *testing.T) {
	agents := []string{"ceo", "cto", "architect", "backend-dev", "frontend-dev"}
	assignments := AssignPersonalities(agents)

	if len(assignments) != len(agents) {
		t.Fatalf("expected %d assignments, got %d", len(agents), len(assignments))
	}

	for _, name := range agents {
		p, ok := assignments[name]
		if !ok {
			t.Errorf("agent %s has no personality assignment", name)
			continue
		}
		if p.Name == "" {
			t.Errorf("agent %s has empty personality name", name)
		}
		if p.Description == "" {
			t.Errorf("agent %s has empty personality description", name)
		}
	}
}

func TestAssignPersonalitiesMoreAgentsThanPersonalities(t *testing.T) {
	agents := []string{
		"a", "b", "c", "d", "e", "f", "g", "h",
		"i", "j", "k", "l", "m", "n", "o", "p",
	}
	assignments := AssignPersonalities(agents)

	if len(assignments) != len(agents) {
		t.Fatalf("expected %d assignments, got %d", len(agents), len(assignments))
	}

	// All agents should have a valid personality
	for _, name := range agents {
		if assignments[name] == nil {
			t.Errorf("agent %s has nil personality", name)
		}
	}
}
