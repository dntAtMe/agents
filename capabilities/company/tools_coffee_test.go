package company

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGetCoffeeRegistersAgent(t *testing.T) {
	root := t.TempDir()
	if err := InitWorkspace(root); err != nil {
		t.Fatalf("InitWorkspace: %v", err)
	}

	tracker := NewActionPointTracker(15, 5, 3)
	tracker.InitRound([]string{"backend-dev", "frontend-dev"})

	state := map[string]any{
		KeyWorkspaceRoot:  root,
		KeyCurrentAgent:   "backend-dev",
		KeyCurrentRound:   1,
		KeyActionPoints:   tracker,
	}

	tool := GetCoffeeTool()
	result, err := tool.Execute(context.Background(), map[string]any{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["status"] != "registered" {
		t.Errorf("expected status=registered, got %v", result["status"])
	}

	participants := tracker.CoffeeParticipants()
	found := false
	for _, p := range participants {
		if p == "backend-dev" {
			found = true
		}
	}
	if !found {
		t.Error("expected backend-dev to be registered for coffee")
	}
}

func TestGetCoffeeShowsOthers(t *testing.T) {
	tracker := NewActionPointTracker(15, 5, 3)
	tracker.InitRound([]string{"backend-dev", "frontend-dev"})
	tracker.RegisterCoffee("frontend-dev") // someone already signed up

	state := map[string]any{
		KeyCurrentAgent: "backend-dev",
		KeyCurrentRound: 1,
		KeyActionPoints: tracker,
	}

	tool := GetCoffeeTool()
	result, err := tool.Execute(context.Background(), map[string]any{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg, _ := result["message"].(string)
	if msg == "" {
		t.Fatal("expected non-empty message")
	}

	// Should mention frontend-dev
	if !containsString(msg, "frontend-dev") {
		t.Errorf("expected message to mention frontend-dev, got: %s", msg)
	}
}

func TestGetCoffeeNoTracker(t *testing.T) {
	state := map[string]any{
		KeyCurrentAgent: "backend-dev",
		KeyCurrentRound: 1,
	}

	tool := GetCoffeeTool()
	result, err := tool.Execute(context.Background(), map[string]any{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := result["error"]; !ok {
		t.Error("expected error when no tracker in state")
	}
}

func TestRunCoffeeBreakWithParticipants(t *testing.T) {
	root := t.TempDir()
	if err := InitWorkspace(root); err != nil {
		t.Fatalf("InitWorkspace: %v", err)
	}

	tracker := NewActionPointTracker(15, 5, 3)
	tracker.RegisterCoffee("alice")
	tracker.RegisterCoffee("bob")

	state := map[string]any{
		KeyWorkspaceRoot: root,
		KeyCurrentAgent:  "alice",
		KeyCurrentRound:  3,
		KeyActionPoints:  tracker,
		"sim_run_agent": func(ctx context.Context, targetName, message string, state map[string]any) (string, error) {
			return "Hey, how's it going? This project is wild.", nil
		},
	}

	err := RunCoffeeBreak(context.Background(), state)
	if err != nil {
		t.Fatalf("RunCoffeeBreak: %v", err)
	}

	// Check transcript file was created
	transcriptPath := filepath.Join(root, "shared", "coffee", "round-3.md")
	data, err := os.ReadFile(transcriptPath)
	if err != nil {
		t.Fatalf("expected coffee transcript to exist: %v", err)
	}

	content := string(data)
	if !containsString(content, "Coffee Break") {
		t.Error("expected transcript to contain 'Coffee Break'")
	}
	if !containsString(content, "alice") || !containsString(content, "bob") {
		t.Error("expected transcript to contain participant names")
	}

	// Coffee registrations should be cleared
	participants := tracker.CoffeeParticipants()
	if len(participants) != 0 {
		t.Errorf("expected coffee registrations to be cleared, got %d", len(participants))
	}
}

func TestRunCoffeeBreakTooFewParticipants(t *testing.T) {
	tracker := NewActionPointTracker(15, 5, 3)
	tracker.RegisterCoffee("alice") // only one person

	state := map[string]any{
		KeyCurrentAgent: "alice",
		KeyCurrentRound: 1,
		KeyActionPoints: tracker,
	}

	err := RunCoffeeBreak(context.Background(), state)
	if err != nil {
		t.Fatalf("RunCoffeeBreak: %v", err)
	}

	// Should be a no-op — no crash, no file
}

func TestRunCoffeeBreakNoTracker(t *testing.T) {
	state := map[string]any{}
	err := RunCoffeeBreak(context.Background(), state)
	if err != nil {
		t.Fatalf("expected no error with nil tracker, got: %v", err)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
