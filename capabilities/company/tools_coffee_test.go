package company

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
		KeyWorkspaceRoot: root,
		KeyCurrentAgent:  "backend-dev",
		KeyCurrentRound:  1,
		KeyActionPoints:  tracker,
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
	if !strings.Contains(msg, "frontend-dev") {
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
	if !strings.Contains(content, "Coffee Break") {
		t.Error("expected transcript to contain 'Coffee Break'")
	}
	if !strings.Contains(content, "alice") || !strings.Contains(content, "bob") {
		t.Error("expected transcript to contain participant names")
	}

	// Coffee registrations should still be in CoffeeNext (cleared by InitRound, not RunCoffeeBreak)
	// The bonus persists until InitRound applies and clears it
	participants := tracker.CoffeeParticipants()
	if len(participants) != 2 {
		t.Errorf("expected coffee registrations to persist (cleared by InitRound), got %d", len(participants))
	}
}

func TestRunCoffeeBreakSetsToolRestriction(t *testing.T) {
	root := t.TempDir()
	if err := InitWorkspace(root); err != nil {
		t.Fatalf("InitWorkspace: %v", err)
	}

	tracker := NewActionPointTracker(15, 5, 3)
	tracker.RegisterCoffee("alice")
	tracker.RegisterCoffee("bob")

	var toolsRestrictionDuringBreak map[string]bool

	state := map[string]any{
		KeyWorkspaceRoot: root,
		KeyCurrentAgent:  "alice",
		KeyCurrentRound:  1,
		KeyActionPoints:  tracker,
		"sim_run_agent": func(ctx context.Context, targetName, message string, state map[string]any) (string, error) {
			// Capture what tools are allowed during the coffee break
			toolsRestrictionDuringBreak = GetAllowedTools(state)
			return "Just chatting!", nil
		},
	}

	err := RunCoffeeBreak(context.Background(), state)
	if err != nil {
		t.Fatalf("RunCoffeeBreak: %v", err)
	}

	// During break, only relationship tools should be allowed
	if toolsRestrictionDuringBreak == nil {
		t.Fatal("expected tool restriction to be set during coffee break")
	}
	if !toolsRestrictionDuringBreak["view_relationships"] {
		t.Error("expected view_relationships to be allowed during break")
	}
	if !toolsRestrictionDuringBreak["update_relationship"] {
		t.Error("expected update_relationship to be allowed during break")
	}
	if toolsRestrictionDuringBreak["send_email"] {
		t.Error("send_email should be restricted during break")
	}

	// After break, restriction should be cleared
	if allowed := GetAllowedTools(state); allowed != nil {
		t.Error("expected tool restriction to be cleared after coffee break")
	}
}

func TestRunCoffeeBreakSoloSkipsConversation(t *testing.T) {
	root := t.TempDir()
	if err := InitWorkspace(root); err != nil {
		t.Fatalf("InitWorkspace: %v", err)
	}

	tracker := NewActionPointTracker(15, 5, 3)
	tracker.RegisterCoffee("alice") // only one person

	agentCalled := false
	state := map[string]any{
		KeyWorkspaceRoot: root,
		KeyCurrentAgent:  "alice",
		KeyCurrentRound:  1,
		KeyActionPoints:  tracker,
		"sim_run_agent": func(ctx context.Context, targetName, message string, state map[string]any) (string, error) {
			agentCalled = true
			return "Hello", nil
		},
	}

	err := RunCoffeeBreak(context.Background(), state)
	if err != nil {
		t.Fatalf("RunCoffeeBreak: %v", err)
	}

	if agentCalled {
		t.Error("expected no agent to be called for solo coffee break")
	}

	// No transcript file should be created
	transcriptPath := filepath.Join(root, "shared", "coffee", "round-1.md")
	if _, err := os.Stat(transcriptPath); err == nil {
		t.Error("expected no transcript file for solo coffee break")
	}

	// But registration should persist so InitRound grants the bonus
	participants := tracker.CoffeeParticipants()
	if len(participants) != 1 {
		t.Errorf("expected 1 coffee participant to persist, got %d", len(participants))
	}
}

func TestRunCoffeeBreakNoTracker(t *testing.T) {
	state := map[string]any{}
	err := RunCoffeeBreak(context.Background(), state)
	if err != nil {
		t.Fatalf("expected no error with nil tracker, got: %v", err)
	}
}
