package company

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMeetingBasic(t *testing.T) {
	root, state := setupTestWorkspace(t)
	ctx := context.Background()

	// Mock RunAgent
	state["sim_run_agent"] = func(_ context.Context, targetName, message string, _ map[string]any) (string, error) {
		return fmt.Sprintf("Response from %s about the meeting topic", targetName), nil
	}
	state[KeyCurrentAgent] = "ceo"

	tool := CallGroupMeetingTool([]string{"ceo", "cto", "architect"})
	result, err := tool.Execute(ctx, map[string]any{
		"attendees": "cto, architect",
		"agenda":    "Discuss API design",
	}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify meeting ID
	if result["meeting_id"] != "MEET-001" {
		t.Errorf("expected MEET-001, got %v", result["meeting_id"])
	}

	// Verify status
	if result["status"] != "completed" {
		t.Errorf("expected completed, got %v", result["status"])
	}

	// Verify attendees includes caller
	attendees := result["attendees"].(string)
	if !strings.Contains(attendees, "ceo") {
		t.Error("attendees should include caller (ceo)")
	}
	if !strings.Contains(attendees, "cto") {
		t.Error("attendees should include cto")
	}
	if !strings.Contains(attendees, "architect") {
		t.Error("attendees should include architect")
	}

	// Verify transcript has 6 entries (3 participants × 2 rounds)
	transcript := result["transcript"].(string)
	for _, name := range []string{"ceo", "cto", "architect"} {
		count := strings.Count(transcript, name+":")
		if count != 2 {
			t.Errorf("expected %s to speak 2 times, found %d", name, count)
		}
	}

	// Verify meeting notes file written
	notesPath := filepath.Join(root, "shared", "meetings", "MEET-001.md")
	data, err := os.ReadFile(notesPath)
	if err != nil {
		t.Fatalf("meeting notes not written: %v", err)
	}
	if !strings.Contains(string(data), "Discuss API design") {
		t.Error("meeting notes should contain agenda")
	}

	// Verify current_agent restored
	if state[KeyCurrentAgent] != "ceo" {
		t.Errorf("current_agent should be restored to ceo, got %v", state[KeyCurrentAgent])
	}
}

func TestMeetingLeafAgentRejected(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	state[KeyCurrentAgent] = "backend-dev"

	tool := CallGroupMeetingTool([]string{"ceo", "cto"})
	result, err := tool.Execute(ctx, map[string]any{
		"attendees": "cto",
		"agenda":    "Something",
	}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error when leaf agent calls meeting")
	}
}

func TestMeetingSelfInAttendees(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	state[KeyCurrentAgent] = "ceo"

	tool := CallGroupMeetingTool([]string{"ceo"})
	result, err := tool.Execute(ctx, map[string]any{
		"attendees": "ceo, cto",
		"agenda":    "Something",
	}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error when caller includes self in attendees")
	}
}

func TestMeetingAgentNotFound(t *testing.T) {
	root, state := setupTestWorkspace(t)
	ctx := context.Background()

	state["sim_run_agent"] = func(_ context.Context, targetName, message string, _ map[string]any) (string, error) {
		if targetName == "nonexistent" {
			return "", fmt.Errorf("agent %q not found", targetName)
		}
		return "Response from " + targetName, nil
	}
	state[KeyCurrentAgent] = "ceo"

	tool := CallGroupMeetingTool([]string{"ceo"})
	result, err := tool.Execute(ctx, map[string]any{
		"attendees": "cto, nonexistent",
		"agenda":    "Test meeting",
	}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Meeting should complete despite error
	if result["status"] != "completed" {
		t.Errorf("expected completed, got %v", result["status"])
	}

	// Transcript should contain error text
	transcript := result["transcript"].(string)
	if !strings.Contains(transcript, "Error") {
		t.Error("transcript should contain error for nonexistent agent")
	}

	// CTO should still have spoken
	if !strings.Contains(transcript, "cto:") {
		t.Error("cto should still have spoken")
	}

	// Notes file should still be written
	notesPath := filepath.Join(root, "shared", "meetings", "MEET-001.md")
	if _, err := os.ReadFile(notesPath); err != nil {
		t.Fatalf("meeting notes should still be written: %v", err)
	}
}

func TestMeetingNoSimRunAgent(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	state[KeyCurrentAgent] = "ceo"

	tool := CallGroupMeetingTool([]string{"ceo"})
	result, err := tool.Execute(ctx, map[string]any{
		"attendees": "cto",
		"agenda":    "Something",
	}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error when sim_run_agent is not set")
	}
}
