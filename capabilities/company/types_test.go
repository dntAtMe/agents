package company

import (
	"strings"
	"testing"
)

func TestTaskBoard_AddAndRender(t *testing.T) {
	tb := NewTaskBoard()

	id1 := tb.Add("Implement API", "Build REST endpoints", "backend-dev", "high", "", 0, "")
	id2 := tb.Add("Design UI", "Create mockups", "frontend-dev", "", "", 0, "")

	if id1 != "TASK-001" {
		t.Errorf("expected TASK-001, got %s", id1)
	}
	if id2 != "TASK-002" {
		t.Errorf("expected TASK-002, got %s", id2)
	}

	tasks := tb.SnapshotTasks()
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestTaskBoard_Update(t *testing.T) {
	tb := NewTaskBoard()
	id := tb.Add("Test task", "desc", "backend-dev", "", "", 0, "")

	if err := tb.Update(id, "in_progress", "started work"); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	tasks := tb.SnapshotTasks()
	if len(tasks) != 1 || tasks[0].Status != "in_progress" {
		t.Fatalf("expected in_progress: %+v", tasks)
	}
	if tasks[0].Notes != "started work" {
		t.Error("notes not saved")
	}

	// Update to awaiting_review
	if err := tb.Update(id, "awaiting_review", ""); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	tasks = tb.SnapshotTasks()
	if len(tasks) != 1 || tasks[0].Status != "awaiting_review" {
		t.Fatalf("expected awaiting_review: %+v", tasks)
	}
}

func TestTaskBoard_UpdateNotFound(t *testing.T) {
	tb := NewTaskBoard()
	if err := tb.Update("TASK-999", "done", ""); err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestTaskBoard_JSONRoundTrip(t *testing.T) {
	tb := NewTaskBoard()
	tb.Add("A", "desc", "be", "high", "", 5, "")
	tb.Add("B", "d2", "fe", "", "", 0, "")

	raw, err := tb.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := TaskBoardFromJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	tasks := loaded.SnapshotTasks()
	if len(tasks) != 2 {
		t.Fatalf("tasks: %d", len(tasks))
	}
	if tasks[0].ID != "TASK-001" || tasks[1].ID != "TASK-002" {
		t.Fatalf("ids: %+v", tasks)
	}
	if tasks[0].Deadline != 5 || tasks[1].Deadline != 0 {
		t.Fatalf("deadlines: %+v, %+v", tasks[0], tasks[1])
	}
}

func TestDecisionLog_AddAndRender(t *testing.T) {
	dl := NewDecisionLog()

	id := dl.Add("Use PostgreSQL", "PostgreSQL for data storage", "Mature, ACID-compliant", "MySQL, MongoDB")
	if id != "ADR-001" {
		t.Errorf("expected ADR-001, got %s", id)
	}

	rendered := dl.Render()
	if !strings.Contains(rendered, "ADR-001") {
		t.Error("render should contain ADR-001")
	}
	if !strings.Contains(rendered, "PostgreSQL") {
		t.Error("render should contain decision content")
	}
	if !strings.Contains(rendered, "MySQL, MongoDB") {
		t.Error("render should contain alternatives")
	}
}

func TestUpdateLog_PostAndRead(t *testing.T) {
	ul := NewUpdateLog()

	ul.Post(1, "ceo", "general", "Project started")
	ul.Post(1, "cto", "technical", "Architecture decided")
	ul.Post(2, "backend-dev", "general", "API implemented")

	// Read all
	all := ul.Read("", 0)
	if len(all) != 3 {
		t.Errorf("expected 3 updates, got %d", len(all))
	}

	// Read filtered by channel
	technical := ul.Read("technical", 0)
	if len(technical) != 1 {
		t.Errorf("expected 1 technical update, got %d", len(technical))
	}

	// Read filtered by round
	since2 := ul.Read("", 2)
	if len(since2) != 1 {
		t.Errorf("expected 1 update since round 2, got %d", len(since2))
	}
}

func TestUpdateLog_Render(t *testing.T) {
	ul := NewUpdateLog()
	ul.Post(1, "ceo", "general", "Hello team")

	rendered := ul.Render("", 0)
	if !strings.Contains(rendered, "ceo") {
		t.Error("render should contain agent name")
	}
	if !strings.Contains(rendered, "Hello team") {
		t.Error("render should contain message")
	}
}

func TestUpdateLog_DefaultChannel(t *testing.T) {
	ul := NewUpdateLog()
	ul.Post(1, "ceo", "", "Test message")

	updates := ul.Read("general", 0)
	if len(updates) != 1 {
		t.Errorf("expected default channel to be 'general', got %d updates", len(updates))
	}
}

func TestPiPLog_AddAndRender(t *testing.T) {
	pl := NewPiPLog()
	id := pl.Add(
		"backend-dev",
		"architect",
		"Repeatedly missing quality checks",
		"Follow review checklist and resolve all critical comments",
		6,
		4,
	)
	if id != "PIP-001" {
		t.Fatalf("expected PIP-001, got %s", id)
	}

	rendered := pl.Render()
	if !strings.Contains(rendered, "PIP-001") {
		t.Fatalf("expected rendered PiP ID, got: %s", rendered)
	}
	if !strings.Contains(rendered, "backend-dev") {
		t.Fatalf("expected rendered target agent, got: %s", rendered)
	}
	if !strings.Contains(rendered, "**Review round:** 6") {
		t.Fatalf("expected rendered review round, got: %s", rendered)
	}
}
