package company

import (
	"testing"
)

func TestPersonalitiesReturnsAll(t *testing.T) {
	ps := Personalities()
	expectedCount := len(hardWorkingPersonalities) + len(slackerPersonalities) + len(maliciousPersonalities)
	if len(ps) != expectedCount {
		t.Fatalf("expected %d personalities, got %d", expectedCount, len(ps))
	}

	names := make(map[string]bool)
	for _, p := range ps {
		if p.Name == "" {
			t.Error("personality name should not be empty")
		}
		if p.Motivation == "" {
			t.Errorf("personality %q has empty Motivation", p.Name)
		}
		if p.CommunicationStyle == "" {
			t.Errorf("personality %q has empty CommunicationStyle", p.Name)
		}
		if p.WorkCulture == "" {
			t.Errorf("personality %q has empty WorkCulture", p.Name)
		}
		if p.WorkEthic != HardWorking && p.WorkEthic != Slacker && p.WorkEthic != Malicious {
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
		if p.Motivation == "" {
			t.Errorf("agent %s has empty Motivation", name)
		}
		if p.CommunicationStyle == "" {
			t.Errorf("agent %s has empty CommunicationStyle", name)
		}
		if p.WorkCulture == "" {
			t.Errorf("agent %s has empty WorkCulture", name)
		}
	}
}

func TestAssignPersonalities_CEOAndCTOAlwaysHardWorking(t *testing.T) {
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

func TestAssignPersonalities_CEOAndCTONeverMalicious(t *testing.T) {
	for i := 0; i < 100; i++ {
		agents := []string{"ceo", "cto", "architect", "project-manager", "backend-dev", "frontend-dev", "devops", "product-manager"}
		assignments := AssignPersonalities(agents)

		if assignments["ceo"].WorkEthic == Malicious {
			t.Errorf("iteration %d: CEO should never be malicious", i)
		}
		if assignments["cto"].WorkEthic == Malicious {
			t.Errorf("iteration %d: CTO should never be malicious", i)
		}
	}
}

func TestAssignPersonalities_MixOfWorkEthics(t *testing.T) {
	agents := []string{"ceo", "cto", "architect", "project-manager", "backend-dev", "frontend-dev", "devops", "product-manager"}

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

	for _, name := range agents {
		if assignments[name] == nil {
			t.Errorf("agent %s has nil personality", name)
		}
	}
}

func TestWorkEthicCounts(t *testing.T) {
	hw := 0
	sl := 0
	mal := 0
	for _, p := range Personalities() {
		switch p.WorkEthic {
		case HardWorking:
			hw++
		case Slacker:
			sl++
		case Malicious:
			mal++
		}
	}
	if hw != len(hardWorkingPersonalities) {
		t.Errorf("expected %d hard-working, got %d", len(hardWorkingPersonalities), hw)
	}
	if sl != len(slackerPersonalities) {
		t.Errorf("expected %d slacker, got %d", len(slackerPersonalities), sl)
	}
	if mal != len(maliciousPersonalities) {
		t.Errorf("expected %d malicious, got %d", len(maliciousPersonalities), mal)
	}
}

func TestMaliciousPoolHasThreeEntries(t *testing.T) {
	if len(maliciousPersonalities) != 3 {
		t.Errorf("expected 3 malicious personalities, got %d", len(maliciousPersonalities))
	}
}

func TestAtMostOneMaliciousPerSimulation(t *testing.T) {
	agents := []string{"ceo", "cto", "architect", "project-manager", "backend-dev", "frontend-dev", "devops", "product-manager"}

	for i := 0; i < 100; i++ {
		assignments := AssignPersonalities(agents)
		malCount := 0
		for _, p := range assignments {
			if p.WorkEthic == Malicious {
				malCount++
			}
		}
		if malCount > 1 {
			t.Errorf("iteration %d: expected at most 1 malicious, got %d", i, malCount)
		}
	}
}

func TestAssignPersonalities_RolePopulated(t *testing.T) {
	agents := []string{"ceo", "cto", "architect", "backend-dev"}
	assignments := AssignPersonalities(agents)

	for _, name := range agents {
		p := assignments[name]
		if p.Role == "" {
			t.Errorf("agent %s should have Role populated", name)
		}
	}
}

func TestDescriptionReturnsNonEmpty(t *testing.T) {
	for _, p := range Personalities() {
		desc := p.Description()
		if desc == "" {
			t.Errorf("personality %q returned empty Description()", p.Name)
		}
	}
}

func TestRoleFor(t *testing.T) {
	role := RoleFor("ceo")
	if role == "" {
		t.Error("RoleFor(\"ceo\") should return non-empty string")
	}

	unknown := RoleFor("nonexistent")
	if unknown != "" {
		t.Errorf("RoleFor(\"nonexistent\") should return empty, got %q", unknown)
	}
}
