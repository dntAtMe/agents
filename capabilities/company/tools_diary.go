package company

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// LastDiaryEntry reads an agent's diary file and returns the last entry.
// Returns empty string if the diary doesn't exist or has no entries.
func LastDiaryEntry(root, agentName string) string {
	diaryPath := filepath.Join(root, agentName, "diary.md")
	data, err := os.ReadFile(diaryPath)
	if err != nil {
		return ""
	}

	content := string(data)

	// Split on "### Round " headings to find entries
	parts := strings.Split(content, "### Round ")
	if len(parts) < 2 {
		return ""
	}

	// Last non-empty part is the most recent entry
	last := strings.TrimSpace(parts[len(parts)-1])
	if last == "" {
		return ""
	}

	return "### Round " + last
}
