package company

import (
	"context"
	"fmt"

	"github.com/dntatme/agents/tool"
)

// AddTaskTool returns a tool for creating tasks on the task board.
func AddTaskTool() tool.Tool {
	return tool.Func("add_task", "Create a new task on the task board.").
		StringParam("title", "Short title for the task.", true).
		StringParam("description", "Detailed description of what needs to be done.", true).
		StringParam("assignee", "Agent to assign (e.g. 'backend-dev', 'frontend-dev', 'devops').", true).
		StringParam("priority", "Priority: low, medium, high. Defaults to medium.", false).
		StringParam("depends_on", "Task ID this depends on (e.g. 'TASK-001').", false).
		IntParam("deadline", "Simulation round by which this task should be completed (e.g. 5). Use 0 or omit if no target round.", false).
		StringParam("reviewer", "Agent who should review this task (e.g. 'architect', 'cto'). Optional.", false).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			title, _ := args["title"].(string)
			desc, _ := args["description"].(string)
			assignee, _ := args["assignee"].(string)
			priority, _ := args["priority"].(string)
			dependsOn, _ := args["depends_on"].(string)
			reviewer, _ := args["reviewer"].(string)

			deadline := 0
			if v, ok := args["deadline"]; ok {
				switch n := v.(type) {
				case int:
					deadline = n
				case float64:
					deadline = int(n)
				}
			}

			tb := GetTaskBoard(state)
			id := tb.Add(title, desc, assignee, priority, dependsOn, deadline, reviewer)

			// Sync to file
			root := GetWorkspaceRoot(state)
			if root != "" {
				_ = SyncTaskBoard(root, tb)
			}

			return map[string]any{"status": "created", "task_id": id}, nil
		}).
		Build()
}

// UpdateTaskTool returns a tool for updating task status.
func UpdateTaskTool() tool.Tool {
	return tool.Func("update_task", "Update the status of an existing task.").
		StringParam("task_id", "The task ID (e.g. 'TASK-001').", true).
		StringEnumParam("status", "New status for the task.", []string{
			"todo", "in_progress", "awaiting_review", "needs_changes", "approved", "done", "blocked",
		}, true).
		StringParam("notes", "Optional notes about the status change.", false).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			taskID, _ := args["task_id"].(string)
			status, _ := args["status"].(string)
			notes, _ := args["notes"].(string)

			tb := GetTaskBoard(state)
			if err := tb.Update(taskID, status, notes); err != nil {
				return map[string]any{"error": err.Error()}, nil
			}

			// Auto-notify reviewer when moving to awaiting_review
			if status == "awaiting_review" {
				task := tb.GetByID(taskID)
				if task != nil && task.Reviewer != "" {
					caller := GetCurrentAgent(state)
					if caller == "" {
						caller = "system"
					}
					el := GetEmailLog(state)
					round := GetCurrentRound(state)
					el.Send(
						caller,
						[]string{task.Reviewer},
						nil,
						fmt.Sprintf("Task %s is ready for your review", taskID),
						fmt.Sprintf("Task %s is ready for your review: %s", taskID, task.Title),
						round,
						false,
					)
					root := GetWorkspaceRoot(state)
					if root != "" {
						_ = SyncInbox(root, el, task.Reviewer)
					}
				}
			}

			// Sync to file
			root := GetWorkspaceRoot(state)
			if root != "" {
				_ = SyncTaskBoard(root, tb)
			}

			return map[string]any{"status": "updated", "task_id": taskID, "new_status": status}, nil
		}).
		Build()
}

// ReadTaskBoardTool returns a tool that reads the current task board.
func ReadTaskBoardTool() tool.Tool {
	statusOrder := []string{"todo", "in_progress", "awaiting_review", "needs_changes", "approved", "done", "blocked"}
	return tool.Func("read_task_board", "Read all tasks as structured data (JSON fields: id, title, description, assignee, status, priority, depends_on, notes, reviewer, deadline). deadline is the target simulation round (0 if unset). Compare with the current round to spot overdue work. Tasks are grouped by status column for readability.").
		NoParams().
		Handler(func(_ context.Context, _ map[string]any, state map[string]any) (map[string]any, error) {
			tb := GetTaskBoard(state)
			tasks := tb.SnapshotTasks()
			byStatus := make(map[string][]Task)
			for _, t := range tasks {
				st := t.Status
				if st == "" {
					st = "todo"
				}
				byStatus[st] = append(byStatus[st], t)
			}
			columns := make([]map[string]any, 0, len(statusOrder))
			for _, st := range statusOrder {
				columns = append(columns, map[string]any{
					"status": st,
					"tasks":  byStatus[st],
				})
			}
			return map[string]any{
				"status_column_order": statusOrder,
				"columns":             columns,
				"tasks":               tasks,
			}, nil
		}).
		Build()
}

// taskTools returns all task-related tools.
func taskTools() []tool.Tool {
	return []tool.Tool{
		AddTaskTool(),
		UpdateTaskTool(),
		ReadTaskBoardTool(),
	}
}

// readOnlyTaskTools returns task tools that only read.
func readOnlyTaskTools() []tool.Tool {
	return []tool.Tool{
		ReadTaskBoardTool(),
	}
}

// readUpdateTaskTools returns tools for reading and updating tasks.
func readUpdateTaskTools() []tool.Tool {
	return []tool.Tool{
		ReadTaskBoardTool(),
		UpdateTaskTool(),
	}
}

// addUpdateReadTaskTools returns tools for adding, updating, and reading tasks.
func addUpdateReadTaskTools() []tool.Tool {
	return []tool.Tool{
		AddTaskTool(),
		UpdateTaskTool(),
		ReadTaskBoardTool(),
	}
}

// FormatTaskID formats a task number to a standard ID.
func FormatTaskID(n int) string {
	return fmt.Sprintf("TASK-%03d", n)
}
