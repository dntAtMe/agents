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
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			title, _ := args["title"].(string)
			desc, _ := args["description"].(string)
			assignee, _ := args["assignee"].(string)
			priority, _ := args["priority"].(string)
			dependsOn, _ := args["depends_on"].(string)

			tb := GetTaskBoard(state)
			id := tb.Add(title, desc, assignee, priority, dependsOn)

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
	return tool.Func("read_task_board", "Read the current task board showing all tasks grouped by status.").
		NoParams().
		Handler(func(_ context.Context, _ map[string]any, state map[string]any) (map[string]any, error) {
			tb := GetTaskBoard(state)
			rendered := tb.Render()
			if rendered == "# Task Board\n\n" {
				return map[string]any{"content": "No tasks on the board yet."}, nil
			}
			return map[string]any{"content": rendered}, nil
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
