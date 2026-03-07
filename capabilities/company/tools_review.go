package company

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dntatme/agents/tool"
)

// WriteReviewTool returns a tool for writing implementation plan reviews.
func WriteReviewTool() tool.Tool {
	return tool.Func("write_review", "Write a review for a developer's implementation plan. Updates the task status and creates a review file.").
		StringParam("task_id", "The task ID to review (e.g. 'TASK-003').", true).
		StringEnumParam("verdict", "Review verdict.", []string{"approved", "needs_changes"}, true).
		StringParam("feedback", "Detailed feedback on the implementation plan.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			taskID, _ := args["task_id"].(string)
			verdict, _ := args["verdict"].(string)
			feedback, _ := args["feedback"].(string)

			root := GetWorkspaceRoot(state)
			round := GetCurrentRound(state)

			// Write review file
			reviewPath := filepath.Join(root, "architect", "reviews", fmt.Sprintf("%s-review.md", taskID))
			reviewContent := fmt.Sprintf("# Review: %s\n\n**Round:** %d\n**Verdict:** %s\n\n## Feedback\n\n%s\n",
				taskID, round, verdict, feedback)

			if err := os.MkdirAll(filepath.Dir(reviewPath), 0o755); err != nil {
				return map[string]any{"error": fmt.Sprintf("create review dir: %v", err)}, nil
			}

			if err := os.WriteFile(reviewPath, []byte(reviewContent), 0o644); err != nil {
				return map[string]any{"error": fmt.Sprintf("write review: %v", err)}, nil
			}

			// Update task status
			tb := GetTaskBoard(state)
			newStatus := verdict // "approved" or "needs_changes"
			if err := tb.Update(taskID, newStatus, fmt.Sprintf("Review: %s", verdict)); err != nil {
				return map[string]any{"error": err.Error()}, nil
			}

			// Sync task board
			if root != "" {
				_ = SyncTaskBoard(root, tb)
			}

			return map[string]any{
				"status":      "review written",
				"task_id":     taskID,
				"verdict":     verdict,
				"review_path": fmt.Sprintf("architect/reviews/%s-review.md", taskID),
			}, nil
		}).
		Build()
}
