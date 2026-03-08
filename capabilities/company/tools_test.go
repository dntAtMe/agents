package company

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestWorkspace(t *testing.T) (string, map[string]any) {
	t.Helper()
	root := t.TempDir()
	if err := InitWorkspace(root); err != nil {
		t.Fatalf("InitWorkspace: %v", err)
	}
	state := map[string]any{
		KeyWorkspaceRoot:  root,
		KeyCurrentAgent:   "backend-dev",
		KeyCurrentRound:   1,
		KeyProjectName:    "test-project",
		KeyProjectStatus:  "active",
		KeyAgentLastRound: map[string]int{},
	}
	return root, state
}

func TestWriteAndReadFile(t *testing.T) {
	root, state := setupTestWorkspace(t)
	ctx := context.Background()

	// Write a file
	wt := WriteFileTool()
	result, err := wt.Execute(ctx, map[string]any{
		"path":    "src/backend/main.go",
		"content": "package main\n\nfunc main() {}\n",
	}, state)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if result["status"] != "written" {
		t.Errorf("expected status 'written', got %v", result["status"])
	}

	// Read it back
	rt := ReadFileTool()
	result, err = rt.Execute(ctx, map[string]any{"path": "src/backend/main.go"}, state)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(result["content"].(string), "package main") {
		t.Error("read content should contain 'package main'")
	}

	// Verify file on disk
	data, _ := os.ReadFile(filepath.Join(root, "src", "backend", "main.go"))
	if !strings.Contains(string(data), "package main") {
		t.Error("file on disk should contain 'package main'")
	}
}

func TestAppendToFile(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	at := AppendToFileTool()
	result, err := at.Execute(ctx, map[string]any{
		"path":    "ceo/notes.md",
		"content": "New note\n",
	}, state)
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if result["status"] != "appended" {
		t.Errorf("expected status 'appended', got %v", result["status"])
	}

	// Read and verify
	rt := ReadFileTool()
	result, err = rt.Execute(ctx, map[string]any{"path": "ceo/notes.md"}, state)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(result["content"].(string), "New note") {
		t.Error("appended content should be present")
	}
}

func TestListFiles(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	lt := ListFilesTool()
	result, err := lt.Execute(ctx, map[string]any{"path": "shared"}, state)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	entries := result["entries"].(string)
	if !strings.Contains(entries, "prd.md") {
		t.Error("should list prd.md")
	}
}

func TestTaskTools(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	// Add task
	at := AddTaskTool()
	result, err := at.Execute(ctx, map[string]any{
		"title":       "Build API",
		"description": "Create REST endpoints",
		"assignee":    "backend-dev",
		"priority":    "high",
	}, state)
	if err != nil {
		t.Fatalf("add task: %v", err)
	}
	taskID := result["task_id"].(string)
	if taskID != "TASK-001" {
		t.Errorf("expected TASK-001, got %s", taskID)
	}

	// Read task board
	rbt := ReadTaskBoardTool()
	result, err = rbt.Execute(ctx, map[string]any{}, state)
	if err != nil {
		t.Fatalf("read board: %v", err)
	}
	if !strings.Contains(result["content"].(string), "Build API") {
		t.Error("board should contain task title")
	}

	// Update task
	ut := UpdateTaskTool()
	result, err = ut.Execute(ctx, map[string]any{
		"task_id": "TASK-001",
		"status":  "in_progress",
		"notes":   "working on it",
	}, state)
	if err != nil {
		t.Fatalf("update task: %v", err)
	}
	if result["new_status"] != "in_progress" {
		t.Errorf("expected in_progress, got %v", result["new_status"])
	}
}

func TestCommsTools(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	// Post update
	pt := PostUpdateTool()
	result, err := pt.Execute(ctx, map[string]any{
		"message": "Architecture review complete",
		"channel": "technical",
	}, state)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if result["status"] != "posted" {
		t.Errorf("expected 'posted', got %v", result["status"])
	}

	// Read updates
	rut := ReadUpdatesTool()
	result, err = rut.Execute(ctx, map[string]any{
		"channel": "technical",
	}, state)
	if err != nil {
		t.Fatalf("read updates: %v", err)
	}
	if !strings.Contains(result["content"].(string), "Architecture review complete") {
		t.Error("should contain posted message")
	}
}

func TestDecisionTools(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	// Log decision
	ld := LogDecisionTool()
	result, err := ld.Execute(ctx, map[string]any{
		"title":        "Database Choice",
		"decision":     "Use PostgreSQL",
		"rationale":    "ACID compliance",
		"alternatives": "MySQL, SQLite",
	}, state)
	if err != nil {
		t.Fatalf("log decision: %v", err)
	}
	if result["decision_id"] != "ADR-001" {
		t.Errorf("expected ADR-001, got %v", result["decision_id"])
	}

	// Read decisions
	rd := ReadDecisionsTool()
	result, err = rd.Execute(ctx, map[string]any{}, state)
	if err != nil {
		t.Fatalf("read decisions: %v", err)
	}
	if !strings.Contains(result["content"].(string), "PostgreSQL") {
		t.Error("should contain decision content")
	}
}

func TestDiaryTool(t *testing.T) {
	root, state := setupTestWorkspace(t)
	ctx := context.Background()

	dt := WriteDiaryTool()
	result, err := dt.Execute(ctx, map[string]any{
		"entry": "Today was productive. The architecture looks solid.",
	}, state)
	if err != nil {
		t.Fatalf("diary: %v", err)
	}
	if result["status"] != "diary entry written" {
		t.Errorf("expected 'diary entry written', got %v", result["status"])
	}

	// Check file
	data, err := os.ReadFile(filepath.Join(root, "backend-dev", "diary.md"))
	if err != nil {
		t.Fatalf("read diary: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Round 1") {
		t.Error("diary should contain round number")
	}
	if !strings.Contains(content, "productive") {
		t.Error("diary should contain entry text")
	}
}

func TestReviewTool(t *testing.T) {
	root, state := setupTestWorkspace(t)
	ctx := context.Background()

	// Create a task first
	at := AddTaskTool()
	at.Execute(ctx, map[string]any{
		"title":       "Build API",
		"description": "REST endpoints",
		"assignee":    "backend-dev",
	}, state)

	// Write review
	wrt := WriteReviewTool()
	result, err := wrt.Execute(ctx, map[string]any{
		"task_id":  "TASK-001",
		"verdict":  "approved",
		"feedback": "Good plan, proceed with implementation.",
	}, state)
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if result["verdict"] != "approved" {
		t.Errorf("expected approved, got %v", result["verdict"])
	}

	// Check review file
	data, err := os.ReadFile(filepath.Join(root, "shared", "reviews", "TASK-001-review.md"))
	if err != nil {
		t.Fatalf("read review: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "approved") {
		t.Error("review should contain verdict")
	}
	if !strings.Contains(content, "Good plan") {
		t.Error("review should contain feedback")
	}

	// Verify task status was updated
	tb := GetTaskBoard(state)
	for _, task := range tb.Tasks {
		if task.ID == "TASK-001" && task.Status != "approved" {
			t.Errorf("task should be approved, got %s", task.Status)
		}
	}
}

func TestStateAccessors(t *testing.T) {
	state := make(map[string]any)

	// GetTaskBoard creates if missing
	tb := GetTaskBoard(state)
	if tb == nil {
		t.Fatal("expected non-nil task board")
	}

	// Same instance returned
	tb2 := GetTaskBoard(state)
	if tb != tb2 {
		t.Error("expected same task board instance")
	}

	// DecisionLog
	dl := GetDecisionLog(state)
	if dl == nil {
		t.Fatal("expected non-nil decision log")
	}

	// UpdateLog
	ul := GetUpdateLog(state)
	if ul == nil {
		t.Fatal("expected non-nil update log")
	}

	// Current agent
	state[KeyCurrentAgent] = "ceo"
	if GetCurrentAgent(state) != "ceo" {
		t.Error("expected ceo")
	}

	// Current round
	state[KeyCurrentRound] = 5
	if GetCurrentRound(state) != 5 {
		t.Error("expected round 5")
	}

	// Agent last round
	alr := GetAgentLastRound(state)
	if alr == nil {
		t.Fatal("expected non-nil agent last round")
	}
}

func TestPathTraversal(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	wt := WriteFileTool()
	result, err := wt.Execute(ctx, map[string]any{
		"path":    "../../etc/passwd",
		"content": "evil",
	}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error for path traversal")
	}
}

func TestFileEscalationRoutesToSuperiorWhenCallerWouldReceiveIt(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	oh := NewOrgHierarchy()
	oh.SetManager("cto", "ceo")
	oh.SetManager("architect", "cto")
	oh.SetManager("backend-dev", "architect")

	state[KeyOrgHierarchy] = oh
	state[KeyCurrentAgent] = "architect"
	state[KeyCurrentRound] = 2

	ft := FileEscalationTool()
	result, err := ft.Execute(ctx, map[string]any{
		"about_agent": "backend-dev",
		"reason":      "Repeatedly ignored required architecture constraints.",
		"evidence":    "Ignored review comments in TASK-003 plan.",
	}, state)
	if err != nil {
		t.Fatalf("file escalation: %v", err)
	}

	if result["to_manager"] != "cto" {
		t.Fatalf("expected escalation to be routed to cto, got %v", result["to_manager"])
	}

	esc := GetEscalationLog(state)
	if len(esc.Escalations) != 1 {
		t.Fatalf("expected 1 escalation, got %d", len(esc.Escalations))
	}
	if esc.Escalations[0].ToManager != "cto" {
		t.Fatalf("expected escalation manager cto, got %s", esc.Escalations[0].ToManager)
	}
}

func TestFileEscalationRejectsSelfRoutingWithoutSuperior(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	oh := NewOrgHierarchy()
	oh.SetManager("cto", "ceo")

	state[KeyOrgHierarchy] = oh
	state[KeyCurrentAgent] = "ceo"
	state[KeyCurrentRound] = 2

	ft := FileEscalationTool()
	result, err := ft.Execute(ctx, map[string]any{
		"about_agent": "cto",
		"reason":      "Escalation loop check",
		"evidence":    "Test evidence",
	}, state)
	if err != nil {
		t.Fatalf("file escalation: %v", err)
	}

	errMsg, ok := result["error"].(string)
	if !ok {
		t.Fatalf("expected error for self-routing escalation, got %v", result)
	}
	if !strings.Contains(errMsg, "no superior") {
		t.Fatalf("expected 'no superior' error, got %q", errMsg)
	}
}

func TestRecordPiPForAgentInManagementChain(t *testing.T) {
	root, state := setupTestWorkspace(t)
	ctx := context.Background()

	oh := NewOrgHierarchy()
	oh.SetManager("cto", "ceo")
	oh.SetManager("architect", "cto")
	oh.SetManager("backend-dev", "architect")
	state[KeyOrgHierarchy] = oh
	state[KeyCurrentAgent] = "cto"
	state[KeyCurrentRound] = 3

	rt := RecordPiPTool()
	result, err := rt.Execute(ctx, map[string]any{
		"agent_name":   "backend-dev",
		"reason":       "Missed repeated delivery commitments.",
		"expectations": "Provide weekly status and close assigned blockers.",
		"review_round": 5,
	}, state)
	if err != nil {
		t.Fatalf("record pip: %v", err)
	}
	if result["status"] != "recorded" {
		t.Fatalf("expected status recorded, got %v", result["status"])
	}

	pl := GetPiPLog(state)
	if len(pl.Records) != 1 {
		t.Fatalf("expected 1 pip record, got %d", len(pl.Records))
	}
	if pl.Records[0].TargetAgent != "backend-dev" {
		t.Fatalf("expected target backend-dev, got %s", pl.Records[0].TargetAgent)
	}
	if pl.Records[0].RecordedBy != "cto" {
		t.Fatalf("expected recorded_by cto, got %s", pl.Records[0].RecordedBy)
	}

	data, err := os.ReadFile(filepath.Join(root, "shared", "pips.md"))
	if err != nil {
		t.Fatalf("read shared/pips.md: %v", err)
	}
	if !strings.Contains(string(data), "PIP-001") {
		t.Fatalf("expected pips.md to include PIP-001, got: %s", string(data))
	}
}

func TestRecordPiPRejectedOutsideManagementChain(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	oh := NewOrgHierarchy()
	oh.SetManager("cto", "ceo")
	oh.SetManager("architect", "cto")
	oh.SetManager("backend-dev", "architect")
	state[KeyOrgHierarchy] = oh
	state[KeyCurrentAgent] = "cto"
	state[KeyCurrentRound] = 3

	rt := RecordPiPTool()
	result, err := rt.Execute(ctx, map[string]any{
		"agent_name":   "product-manager",
		"reason":       "test",
		"expectations": "test",
	}, state)
	if err != nil {
		t.Fatalf("record pip: %v", err)
	}
	errMsg, ok := result["error"].(string)
	if !ok {
		t.Fatalf("expected error, got %v", result)
	}
	if !strings.Contains(errMsg, "management chain") {
		t.Fatalf("expected management chain error, got %q", errMsg)
	}
}
