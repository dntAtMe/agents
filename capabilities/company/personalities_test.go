package company

import (
	"testing"
)

func TestPersonalitiesReturnsAll(t *testing.T) {
	ps := Personalities()
	expectedCount := len(hardWorkingPersonalities) + len(slackerPersonalities)
	if len(ps) != expectedCount {
		t.Fatalf("expected %d personalities, got %d", expectedCount, len(ps))
	}

	names := make(map[string]bool)
	for _, p := range ps {
		if p.Name == "" {
			t.Error("personality name should not be empty")
		}
		if p.Description == "" {
			t.Error("personality description should not be empty")
		}
		if p.WorkEthic != HardWorking && p.WorkEthic != Slacker {
			t.Errorf("personality %q has invalid work ethic: %q", p.Name, p.WorkEthic)
		}
		names[p.Name] = true
	}

	if len(names) != expectedCount {
		t.Errorf("expected %d unique names, got %d", expectedCount, len(names))
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

func TestAssignPersonalities_CEOAndCTOAlwaysHardWorking(t *testing.T) {
	// Run multiple times to test randomness
	for i := 0; i < 20; i++ {
		agents := []string{"ceo", "cto", "architect", "project-manager", "backend-dev", "frontend-dev", "devops", "product-manager"}
		assignments := AssignPersonalities(agents)

		if assignments["ceo"].WorkEthic != HardWorking {
			t.Errorf("iteration %d: CEO should always be hard-working, got %q (%s)", i, assignments["ceo"].WorkEthic, assignments["ceo"].Name)
		}
		if assignments["cto"].WorkEthic != HardWorking {
			t.Errorf("iteration %d: CTO should always be hard-working, got %q (%s)", i, assignments["cto"].WorkEthic, assignments["cto"].Name)
		}
	}
}

func TestAssignPersonalities_MixOfWorkEthics(t *testing.T) {
	// With 8 agents (2 forced hard-working + 6 others alternating), we should get a mix
	agents := []string{"ceo", "cto", "architect", "project-manager", "backend-dev", "frontend-dev", "devops", "product-manager"}

	// Run multiple times and check we get both types among non-protected agents
	sawSlacker := false
	sawHardWorking := false
	for i := 0; i < 20; i++ {
		assignments := AssignPersonalities(agents)
		for _, name := range agents {
			if alwaysHardWorking[name] {
				continue
			}
			if assignments[name].WorkEthic == Slacker {
				sawSlacker = true
			}
			if assignments[name].WorkEthic == HardWorking {
				sawHardWorking = true
			}
		}
	}

	if !sawSlacker {
		t.Error("expected at least some slacker assignments among non-protected agents")
	}
	if !sawHardWorking {
		t.Error("expected at least some hard-working assignments among non-protected agents")
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

func TestWorkEthicCounts(t *testing.T) {
	hw := 0
	sl := 0
	for _, p := range Personalities() {
		switch p.WorkEthic {
		case HardWorking:
			hw++
		case Slacker:
			sl++
		}
	}
	if hw != len(hardWorkingPersonalities) {
		t.Errorf("expected %d hard-working, got %d", len(hardWorkingPersonalities), hw)
	}
	if sl != len(slackerPersonalities) {
		t.Errorf("expected %d slacker, got %d", len(slackerPersonalities), sl)
	}
}
