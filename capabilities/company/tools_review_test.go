package company

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteReview_AssignedReviewerCanReview(t *testing.T) {
	root := t.TempDir()
	if err := InitWorkspace(root); err != nil {
		t.Fatalf("InitWorkspace: %v", err)
	}

	tb := NewTaskBoard()
	taskID := tb.Add("Build API", "REST endpoints", "backend-dev", "high", "", 0, "architect")
	tb.Update(taskID, "awaiting_review", "")

	state := map[string]any{
		KeyWorkspaceRoot: root,
		KeyCurrentAgent:  "architect",
		KeyCurrentRound:  1,
		KeyTasks:         tb,
	}

	reviewTool := WriteReviewTool()
	result, err := reviewTool.Execute(context.Background(), map[string]any{
		"task_id":  taskID,
		"verdict":  "approved",
		"feedback": "Looks good!",
	}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "review written" {
		t.Errorf("expected status 'review written', got %v", result["status"])
	}

	// Verify review file written to shared/reviews/
	reviewPath := filepath.Join(root, "shared", "reviews", taskID+"-review.md")
	data, err := os.ReadFile(reviewPath)
	if err != nil {
		t.Fatalf("review file not found: %v", err)
	}
	if !strings.Contains(string(data), "approved") {
		t.Error("review file should contain verdict")
	}
	if !strings.Contains(string(data), "architect") {
		t.Error("review file should contain reviewer name")
	}
}

func TestWriteReview_NonAssignedReviewerRejected(t *testing.T) {
	root := t.TempDir()
	if err := InitWorkspace(root); err != nil {
		t.Fatalf("InitWorkspace: %v", err)
	}

	tb := NewTaskBoard()
	taskID := tb.Add("Build API", "REST endpoints", "backend-dev", "high", "", 0, "architect")
	tb.Update(taskID, "awaiting_review", "")

	state := map[string]any{
		KeyWorkspaceRoot: root,
		KeyCurrentAgent:  "cto",
		KeyCurrentRound:  1,
		KeyTasks:         tb,
	}

	reviewTool := WriteReviewTool()
	result, err := reviewTool.Execute(context.Background(), map[string]any{
		"task_id":  taskID,
		"verdict":  "approved",
		"feedback": "Looks good!",
	}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error for non-assigned reviewer")
	}
	errMsg := result["error"].(string)
	if !strings.Contains(errMsg, "architect") {
		t.Errorf("error should mention assigned reviewer, got: %s", errMsg)
	}
}

func TestWriteReview_NoReviewerAllowsAnyone(t *testing.T) {
	root := t.TempDir()
	if err := InitWorkspace(root); err != nil {
		t.Fatalf("InitWorkspace: %v", err)
	}

	tb := NewTaskBoard()
	taskID := tb.Add("Build API", "REST endpoints", "backend-dev", "high", "", 0, "")
	tb.Update(taskID, "awaiting_review", "")

	state := map[string]any{
		KeyWorkspaceRoot: root,
		KeyCurrentAgent:  "cto",
		KeyCurrentRound:  1,
		KeyTasks:         tb,
	}

	reviewTool := WriteReviewTool()
	result, err := reviewTool.Execute(context.Background(), map[string]any{
		"task_id":  taskID,
		"verdict":  "needs_changes",
		"feedback": "Needs work",
	}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "review written" {
		t.Errorf("expected status 'review written', got %v", result["status"])
	}
}

func TestWriteReview_CorrectPath(t *testing.T) {
	root := t.TempDir()
	if err := InitWorkspace(root); err != nil {
		t.Fatalf("InitWorkspace: %v", err)
	}

	tb := NewTaskBoard()
	taskID := tb.Add("Build API", "REST endpoints", "backend-dev", "high", "", 0, "")

	state := map[string]any{
		KeyWorkspaceRoot: root,
		KeyCurrentAgent:  "architect",
		KeyCurrentRound:  1,
		KeyTasks:         tb,
	}

	reviewTool := WriteReviewTool()
	result, err := reviewTool.Execute(context.Background(), map[string]any{
		"task_id":  taskID,
		"verdict":  "approved",
		"feedback": "Good",
	}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedPath := "shared/reviews/" + taskID + "-review.md"
	if result["review_path"] != expectedPath {
		t.Errorf("expected review_path %q, got %q", expectedPath, result["review_path"])
	}

	// Verify file exists on disk
	if _, err := os.Stat(filepath.Join(root, expectedPath)); err != nil {
		t.Errorf("review file should exist at %s: %v", expectedPath, err)
	}
}
