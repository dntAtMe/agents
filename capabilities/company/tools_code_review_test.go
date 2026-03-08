package company

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeReviewLifecycle(t *testing.T) {
	root, state := setupTestWorkspace(t)
	ctx := context.Background()

	state[KeyCurrentAgent] = "architect"
	state[KeyCurrentRound] = 2

	// Create a task
	at := AddTaskTool()
	at.Execute(ctx, map[string]any{
		"title":       "Build API",
		"description": "REST endpoints",
		"assignee":    "backend-dev",
		"reviewer":    "architect",
	}, state)

	// Write some code to review
	codePath := filepath.Join(root, "src", "backend", "handler.go")
	os.WriteFile(codePath, []byte("package main\n\nfunc handler() {\n\t// TODO: implement\n\treturn nil\n}\n"), 0o644)

	// Start review
	srt := StartCodeReviewTool()
	result, err := srt.Execute(ctx, map[string]any{
		"task_id": "TASK-001",
		"summary": "Initial code review of handler implementation",
	}, state)
	if err != nil {
		t.Fatalf("start review: %v", err)
	}
	reviewID := result["review_id"].(string)
	if reviewID != "CR-001" {
		t.Errorf("expected CR-001, got %s", reviewID)
	}

	// Add comments
	act := AddReviewCommentTool()
	result, err = act.Execute(ctx, map[string]any{
		"review_id": reviewID,
		"file":      "src/backend/handler.go",
		"line":      float64(4),
		"severity":  "error",
		"comment":   "TODO should be implemented before review",
	}, state)
	if err != nil {
		t.Fatalf("add comment: %v", err)
	}
	if result["comment_count"] != 1 {
		t.Errorf("expected 1 comment, got %v", result["comment_count"])
	}

	result, err = act.Execute(ctx, map[string]any{
		"review_id": reviewID,
		"file":      "src/backend/handler.go",
		"line":      float64(5),
		"severity":  "warning",
		"comment":   "Return type mismatch — handler has no return type",
	}, state)
	if err != nil {
		t.Fatalf("add comment 2: %v", err)
	}
	if result["comment_count"] != 2 {
		t.Errorf("expected 2 comments, got %v", result["comment_count"])
	}

	// Submit review
	sct := SubmitCodeReviewTool()
	result, err = sct.Execute(ctx, map[string]any{
		"review_id": reviewID,
		"verdict":   "needs_changes",
	}, state)
	if err != nil {
		t.Fatalf("submit review: %v", err)
	}
	if result["verdict"] != "needs_changes" {
		t.Errorf("expected needs_changes, got %v", result["verdict"])
	}

	// Verify task status was updated
	tb := GetTaskBoard(state)
	task := tb.GetByID("TASK-001")
	if task.Status != "needs_changes" {
		t.Errorf("expected task status needs_changes, got %s", task.Status)
	}

	// Verify review file was created
	reviewFiles, _ := filepath.Glob(filepath.Join(root, "shared", "code-reviews", "TASK-001-review-*.md"))
	if len(reviewFiles) == 0 {
		t.Error("expected review file in shared/code-reviews/")
	}

	// Read code reviews
	rrt := ReadCodeReviewsTool()
	result, err = rrt.Execute(ctx, map[string]any{
		"task_id": "TASK-001",
	}, state)
	if err != nil {
		t.Fatalf("read reviews: %v", err)
	}
	content := result["content"].(string)
	if !strings.Contains(content, "needs_changes") {
		t.Error("review content should contain verdict")
	}
	if !strings.Contains(content, "TODO should be implemented") {
		t.Error("review content should contain comments")
	}
}

func TestCodeReviewNotFound(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	act := AddReviewCommentTool()
	result, err := act.Execute(ctx, map[string]any{
		"review_id": "CR-999",
		"file":      "src/backend/handler.go",
		"line":      float64(1),
		"severity":  "error",
		"comment":   "test",
	}, state)
	if err != nil {
		t.Fatalf("add comment: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error for non-existent review")
	}
}

func TestCodeReviewDoubleSubmit(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	state[KeyCurrentAgent] = "architect"
	state[KeyCurrentRound] = 1

	// Create task
	at := AddTaskTool()
	at.Execute(ctx, map[string]any{
		"title":       "Test Task",
		"description": "test",
		"assignee":    "backend-dev",
	}, state)

	// Start and submit review
	srt := StartCodeReviewTool()
	result, _ := srt.Execute(ctx, map[string]any{
		"task_id": "TASK-001",
		"summary": "test",
	}, state)
	reviewID := result["review_id"].(string)

	sct := SubmitCodeReviewTool()
	sct.Execute(ctx, map[string]any{
		"review_id": reviewID,
		"verdict":   "approved",
	}, state)

	// Try to submit again
	result, err := sct.Execute(ctx, map[string]any{
		"review_id": reviewID,
		"verdict":   "needs_changes",
	}, state)
	if err != nil {
		t.Fatalf("double submit: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error for double submission")
	}
}

func TestCodeReviewCommentOnSubmitted(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	state[KeyCurrentAgent] = "architect"
	state[KeyCurrentRound] = 1

	at := AddTaskTool()
	at.Execute(ctx, map[string]any{
		"title":       "Test Task",
		"description": "test",
		"assignee":    "backend-dev",
	}, state)

	srt := StartCodeReviewTool()
	result, _ := srt.Execute(ctx, map[string]any{
		"task_id": "TASK-001",
		"summary": "test",
	}, state)
	reviewID := result["review_id"].(string)

	sct := SubmitCodeReviewTool()
	sct.Execute(ctx, map[string]any{
		"review_id": reviewID,
		"verdict":   "approved",
	}, state)

	// Try to add comment after submission
	act := AddReviewCommentTool()
	result, err := act.Execute(ctx, map[string]any{
		"review_id": reviewID,
		"file":      "src/backend/handler.go",
		"line":      float64(1),
		"severity":  "nit",
		"comment":   "late comment",
	}, state)
	if err != nil {
		t.Fatalf("comment on submitted: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error for commenting on submitted review")
	}
}

func TestReadCodeReviewsEmpty(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	rrt := ReadCodeReviewsTool()
	result, err := rrt.Execute(ctx, map[string]any{
		"task_id": "TASK-999",
	}, state)
	if err != nil {
		t.Fatalf("read reviews: %v", err)
	}
	content := result["content"].(string)
	if !strings.Contains(content, "No code reviews found") {
		t.Error("expected 'No code reviews found' message")
	}
}

func TestCodeReviewFileValidation(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	state[KeyCurrentAgent] = "architect"
	state[KeyCurrentRound] = 1

	at := AddTaskTool()
	at.Execute(ctx, map[string]any{
		"title":       "Test Task",
		"description": "test",
		"assignee":    "backend-dev",
	}, state)

	srt := StartCodeReviewTool()
	result, _ := srt.Execute(ctx, map[string]any{
		"task_id": "TASK-001",
		"summary": "test",
	}, state)
	reviewID := result["review_id"].(string)

	// Comment on non-existent file
	act := AddReviewCommentTool()
	result, err := act.Execute(ctx, map[string]any{
		"review_id": reviewID,
		"file":      "src/backend/nonexistent.go",
		"line":      float64(1),
		"severity":  "error",
		"comment":   "test",
	}, state)
	if err != nil {
		t.Fatalf("comment: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error for non-existent file")
	}
}
