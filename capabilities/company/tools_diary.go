package company

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dntatme/agents/tool"
)

// WriteDiaryTool returns a tool for agents to write personal diary entries.
func WriteDiaryTool() tool.Tool {
	return tool.Func("write_diary", "Write a personal diary entry. Be honest and reflective — share your thoughts about work, the project, your teammates, frustrations, and celebrations.").
		StringParam("entry", "Your diary entry. Be personal and reflective.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			entry, _ := args["entry"].(string)
			agent := GetCurrentAgent(state)
			round := GetCurrentRound(state)
			root := GetWorkspaceRoot(state)

			if agent == "" {
				return map[string]any{"error": "no current agent set"}, nil
			}

			diaryPath := filepath.Join(root, agent, "diary.md")

			f, err := os.OpenFile(diaryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				return map[string]any{"error": fmt.Sprintf("open diary: %v", err)}, nil
			}
			defer f.Close()

			formatted := fmt.Sprintf("### Round %d\n%s\n\n", round, entry)
			if _, err := f.WriteString(formatted); err != nil {
				return map[string]any{"error": fmt.Sprintf("write diary: %v", err)}, nil
			}

			return map[string]any{"status": "diary entry written", "round": round}, nil
		}).
		Build()
}
