package company

import (
	"context"
	"fmt"
	"os"

	"github.com/dntatme/agents/tool"
)

// StartCodeReviewTool returns a tool that begins a code review session.
func StartCodeReviewTool() tool.Tool {
	return tool.Func("start_code_review", "Begin a new code review session for a task. Returns a review_id to use with add_review_comment and submit_code_review.").
		StringParam("task_id", "The task ID to review (e.g. 'TASK-003').", true).
		StringParam("summary", "High-level summary of the review.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			taskID, _ := args["task_id"].(string)
			summary, _ := args["summary"].(string)
			reviewer := GetCurrentAgent(state)
			round := GetCurrentRound(state)

			// Verify task exists
			tb := GetTaskBoard(state)
			task := tb.GetByID(taskID)
			if task == nil {
				return map[string]any{"error": fmt.Sprintf("task %q not found", taskID)}, nil
			}

			reviewLog := GetCodeReviewLog(state)
			id := reviewLog.Add(taskID, reviewer, summary, round)

			return map[string]any{
				"review_id": id,
				"task_id":   taskID,
				"status":    "review started",
			}, nil
		}).
		Build()
}

// AddReviewCommentTool returns a tool that adds an inline comment to a code review.
func AddReviewCommentTool() tool.Tool {
	return tool.Func("add_review_comment", "Add an inline comment to an active code review. Call this once per comment.").
		StringParam("review_id", "The review ID from start_code_review.", true).
		StringParam("file", "Workspace-relative path to the file being commented on.", true).
		IntParam("line", "Line number the comment refers to.", true).
		StringEnumParam("severity", "Severity of the issue.", []string{"error", "warning", "suggestion", "nit"}, true).
		StringParam("comment", "The review comment.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			reviewID, _ := args["review_id"].(string)
			file, _ := args["file"].(string)
			line := toInt(args["line"])
			severity, _ := args["severity"].(string)
			comment, _ := args["comment"].(string)

			root := GetWorkspaceRoot(state)

			// Validate file exists
			fullPath, err := ResolvePath(root, file)
			if err != nil {
				return map[string]any{"error": err.Error()}, nil
			}
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				return map[string]any{"error": fmt.Sprintf("file %q does not exist", file)}, nil
			}

			reviewLog := GetCodeReviewLog(state)
			review := reviewLog.GetByID(reviewID)
			if review == nil {
				return map[string]any{"error": fmt.Sprintf("review %q not found", reviewID)}, nil
			}

			if review.Verdict != "" {
				return map[string]any{"error": fmt.Sprintf("review %q is already submitted", reviewID)}, nil
			}

			// Add comment (thread-safe via the log's mutex is already released,
			// but since GetByID returns a pointer to the slice element, we need to lock)
			reviewLog.mu.Lock()
			review.Comments = append(review.Comments, CodeComment{
				File:     file,
				Line:     line,
				Severity: severity,
				Comment:  comment,
			})
			count := len(review.Comments)
			reviewLog.mu.Unlock()

			return map[string]any{
				"status":        "comment added",
				"comment_count": count,
			}, nil
		}).
		Build()
}

// SubmitCodeReviewTool returns a tool that finalizes and submits a code review.
func SubmitCodeReviewTool() tool.Tool {
	return tool.Func("submit_code_review", "Finalize and submit a code review with a verdict. Updates the task status and writes the review to shared/code-reviews/.").
		StringParam("review_id", "The review ID from start_code_review.", true).
		StringEnumParam("verdict", "Review verdict.", []string{"approved", "needs_changes"}, true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			reviewID, _ := args["review_id"].(string)
			verdict, _ := args["verdict"].(string)

			root := GetWorkspaceRoot(state)

			reviewLog := GetCodeReviewLog(state)
			review := reviewLog.GetByID(reviewID)
			if review == nil {
				return map[string]any{"error": fmt.Sprintf("review %q not found", reviewID)}, nil
			}
			if review.Verdict != "" {
				return map[string]any{"error": fmt.Sprintf("review %q is already submitted with verdict %q", reviewID, review.Verdict)}, nil
			}

			// Set verdict
			reviewLog.mu.Lock()
			review.Verdict = verdict
			taskID := review.TaskID
			reviewLog.mu.Unlock()

			// Update task status
			tb := GetTaskBoard(state)
			newStatus := verdict // "approved" or "needs_changes"
			caller := GetCurrentAgent(state)
			_ = tb.Update(taskID, newStatus, fmt.Sprintf("Code review by %s: %s", caller, verdict))

			// Snapshot reviewed files for future diff_file
			snapLog := GetFileSnapshotLog(state)
			round := GetCurrentRound(state)
			for _, c := range review.Comments {
				fullPath, err := ResolvePath(root, c.File)
				if err != nil {
					continue
				}
				data, err := os.ReadFile(fullPath)
				if err != nil {
					continue
				}
				snapLog.Save(c.File, taskID, string(data), round)
			}

			// Sync review to file
			if root != "" {
				_ = SyncTaskBoard(root, tb)
				_ = SyncCodeReviews(root, reviewLog, *review)
			}

			return map[string]any{
				"status":  "review submitted",
				"task_id": taskID,
				"verdict": verdict,
			}, nil
		}).
		Build()
}

// ReadCodeReviewsTool returns a tool that reads code reviews for a task.
func ReadCodeReviewsTool() tool.Tool {
	return tool.Func("read_code_reviews", "Read all code reviews for a given task, including inline comments with source context.").
		StringParam("task_id", "The task ID to read reviews for.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			taskID, _ := args["task_id"].(string)
			root := GetWorkspaceRoot(state)

			reviewLog := GetCodeReviewLog(state)
			reviews := reviewLog.GetByTaskID(taskID)

			if len(reviews) == 0 {
				return map[string]any{
					"content": fmt.Sprintf("No code reviews found for %s.", taskID),
				}, nil
			}

			var content string
			for _, r := range reviews {
				content += reviewLog.RenderWithSource(r, root)
				content += "\n---\n\n"
			}

			return map[string]any{
				"content":      content,
				"review_count": len(reviews),
			}, nil
		}).
		Build()
}
