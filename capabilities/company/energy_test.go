package company

import (
	"testing"
)

func TestNewActionPointTracker(t *testing.T) {
	tracker := NewActionPointTracker(15, 5, 3)

	if tracker.DefaultAP != 15 {
		t.Errorf("expected DefaultAP=15, got %d", tracker.DefaultAP)
	}
	if tracker.BonusAP != 5 {
		t.Errorf("expected BonusAP=5, got %d", tracker.BonusAP)
	}
	if tracker.HardCap != 3 {
		t.Errorf("expected HardCap=3, got %d", tracker.HardCap)
	}
}

func TestInitRound(t *testing.T) {
	tracker := NewActionPointTracker(15, 5, 3)
	agents := []string{"alice", "bob"}

	tracker.InitRound(agents)

	if r := tracker.Remaining("alice"); r != 15 {
		t.Errorf("expected alice=15, got %d", r)
	}
	if r := tracker.Remaining("bob"); r != 15 {
		t.Errorf("expected bob=15, got %d", r)
	}
}

func TestInitRoundWithCoffeeBonus(t *testing.T) {
	tracker := NewActionPointTracker(15, 5, 3)
	agents := []string{"alice", "bob"}

	// Register alice for coffee
	tracker.RegisterCoffee("alice")

	tracker.InitRound(agents)

	if r := tracker.Remaining("alice"); r != 20 {
		t.Errorf("expected alice=20 (15+5 coffee bonus), got %d", r)
	}
	if r := tracker.Remaining("bob"); r != 15 {
		t.Errorf("expected bob=15, got %d", r)
	}
}

func TestDeduct(t *testing.T) {
	tracker := NewActionPointTracker(15, 5, 3)
	tracker.InitRound([]string{"alice"})

	remaining := tracker.Deduct("alice", 3)
	if remaining != 12 {
		t.Errorf("expected 12 after deducting 3, got %d", remaining)
	}

	remaining = tracker.Deduct("alice", 10)
	if remaining != 2 {
		t.Errorf("expected 2 after deducting 10, got %d", remaining)
	}

	// Go negative
	remaining = tracker.Deduct("alice", 5)
	if remaining != -3 {
		t.Errorf("expected -3 after deducting 5, got %d", remaining)
	}
}

func TestDeductUnknownAgent(t *testing.T) {
	tracker := NewActionPointTracker(15, 5, 3)

	// Deducting from unknown agent should start from default
	remaining := tracker.Deduct("unknown", 5)
	if remaining != 10 {
		t.Errorf("expected 10 (15-5), got %d", remaining)
	}
}

func TestCoffeeParticipants(t *testing.T) {
	tracker := NewActionPointTracker(15, 5, 3)

	tracker.RegisterCoffee("alice")
	tracker.RegisterCoffee("bob")

	participants := tracker.CoffeeParticipants()
	if len(participants) != 2 {
		t.Errorf("expected 2 participants, got %d", len(participants))
	}

	// Check both are present
	found := map[string]bool{}
	for _, p := range participants {
		found[p] = true
	}
	if !found["alice"] || !found["bob"] {
		t.Errorf("expected alice and bob, got %v", participants)
	}
}

func TestClearCoffee(t *testing.T) {
	tracker := NewActionPointTracker(15, 5, 3)

	tracker.RegisterCoffee("alice")
	tracker.ClearCoffee()

	participants := tracker.CoffeeParticipants()
	if len(participants) != 0 {
		t.Errorf("expected 0 participants after clear, got %d", len(participants))
	}
}

func TestCoffeeBonusAppliedOnceAndCleared(t *testing.T) {
	tracker := NewActionPointTracker(15, 5, 3)

	tracker.RegisterCoffee("alice")

	// First round: alice gets bonus
	tracker.InitRound([]string{"alice"})
	if r := tracker.Remaining("alice"); r != 20 {
		t.Errorf("expected 20, got %d", r)
	}

	// Clear coffee and init again: no bonus
	tracker.ClearCoffee()
	tracker.InitRound([]string{"alice"})
	if r := tracker.Remaining("alice"); r != 15 {
		t.Errorf("expected 15 after clear, got %d", r)
	}
}

func TestGetToolCostKnown(t *testing.T) {
	tests := map[string]int{
		"read_file":          1,
		"write_diary":        2,
		"send_email":         3,
		"call_group_meeting": 5,
		"get_coffee":         3,
	}
	for tool, expected := range tests {
		if got := GetToolCost(tool); got != expected {
			t.Errorf("GetToolCost(%q) = %d, want %d", tool, got, expected)
		}
	}
}

func TestGetToolCostUnknown(t *testing.T) {
	if got := GetToolCost("unknown_tool"); got != 2 {
		t.Errorf("GetToolCost(unknown) = %d, want 2", got)
	}
}

func TestGetActionPointTrackerFromState(t *testing.T) {
	tracker := NewActionPointTracker(15, 5, 3)
	state := map[string]any{
		KeyActionPoints: tracker,
	}

	got := GetActionPointTracker(state)
	if got != tracker {
		t.Error("expected to get the same tracker from state")
	}
}

func TestGetActionPointTrackerNil(t *testing.T) {
	state := map[string]any{}
	got := GetActionPointTracker(state)
	if got != nil {
		t.Error("expected nil when tracker not in state")
	}
}
