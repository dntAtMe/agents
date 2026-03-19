package tui

import (
	"testing"

	"github.com/dntatme/agents/capabilities/company"
)

func TestBuildKanbanColumns(t *testing.T) {
	tasks := []company.Task{
		{ID: "TASK-001", Title: "First", Status: "todo", Assignee: "pm", Priority: "high", Reviewer: "cto", Deadline: 3},
		{ID: "TASK-002", Title: "Done thing", Status: "done", Assignee: "be", Priority: "low"},
	}
	cols := buildKanbanColumns(tasks)
	if len(cols) != len(kanbanStatusOrder) {
		t.Fatalf("expected %d columns, got %d", len(kanbanStatusOrder), len(cols))
	}
	var todoCol *KanbanColumn
	for i := range cols {
		if cols[i].StatusKey == "todo" {
			todoCol = &cols[i]
			break
		}
	}
	if todoCol == nil {
		t.Fatal("missing todo column")
	}
	if len(todoCol.Tasks) != 1 {
		t.Fatalf("todo tasks: %d", len(todoCol.Tasks))
	}
	task := todoCol.Tasks[0]
	if task.ID != "TASK-001" || task.Title != "First" {
		t.Fatalf("task: %+v", task)
	}
	if task.Assignee != "pm" || task.Reviewer != "cto" || task.Deadline != 3 {
		t.Fatalf("assignee/reviewer/deadline: %+v", task)
	}
	var doneCol *KanbanColumn
	for i := range cols {
		if cols[i].StatusKey == "done" {
			doneCol = &cols[i]
			break
		}
	}
	if doneCol == nil || len(doneCol.Tasks) != 1 {
		t.Fatalf("done column: %+v", doneCol)
	}
}

func TestBuildKanbanColumnsEmptyStatuses(t *testing.T) {
	cols := buildKanbanColumns([]company.Task{{ID: "TASK-001", Title: "X", Assignee: "a"}})
	var todoCol *KanbanColumn
	for i := range cols {
		if cols[i].StatusKey == "todo" {
			todoCol = &cols[i]
			break
		}
	}
	if todoCol == nil || len(todoCol.Tasks) != 1 {
		t.Fatal("empty status should default to todo")
	}
}
