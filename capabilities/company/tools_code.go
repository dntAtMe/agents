package company

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dntatme/agents/tool"
)

// EditFileTool returns a tool that replaces a line range in a file.
func EditFileTool() tool.Tool {
	return tool.Func("edit_file", "Replace a range of lines in a file. Lines are 1-indexed. The range [start_line, end_line] is inclusive and replaced with new_content.").
		StringParam("path", "Workspace-relative path to the file.", true).
		IntParam("start_line", "First line to replace (1-indexed, inclusive).", true).
		IntParam("end_line", "Last line to replace (1-indexed, inclusive).", true).
		StringParam("new_content", "Content to insert in place of the removed lines.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			relPath, _ := args["path"].(string)
			newContent, _ := args["new_content"].(string)
			root := GetWorkspaceRoot(state)

			startLine := toInt(args["start_line"])
			endLine := toInt(args["end_line"])

			fullPath, err := ResolvePath(root, relPath)
			if err != nil {
				return map[string]any{"error": err.Error()}, nil
			}

			data, err := os.ReadFile(fullPath)
			if err != nil {
				return map[string]any{"error": fmt.Sprintf("read: %v", err)}, nil
			}

			lines := strings.Split(string(data), "\n")

			// Validate bounds
			if startLine < 1 || endLine < startLine || startLine > len(lines) {
				return map[string]any{"error": fmt.Sprintf("invalid line range [%d, %d] for file with %d lines", startLine, endLine, len(lines))}, nil
			}
			if endLine > len(lines) {
				endLine = len(lines)
			}

			// Splice: keep lines before start, add new content, keep lines after end
			linesRemoved := endLine - startLine + 1
			var newLines []string
			newLines = append(newLines, lines[:startLine-1]...)
			if newContent != "" {
				insertLines := strings.Split(newContent, "\n")
				newLines = append(newLines, insertLines...)
			}
			newLines = append(newLines, lines[endLine:]...)

			linesAdded := 0
			if newContent != "" {
				linesAdded = len(strings.Split(newContent, "\n"))
			}

			result := strings.Join(newLines, "\n")
			if err := os.WriteFile(fullPath, []byte(result), 0o644); err != nil {
				return map[string]any{"error": fmt.Sprintf("write: %v", err)}, nil
			}

			return map[string]any{
				"status":        "edited",
				"path":          relPath,
				"lines_removed": linesRemoved,
				"lines_added":   linesAdded,
			}, nil
		}).
		Build()
}

// SearchFilesTool returns a tool that searches for a pattern across workspace files.
func SearchFilesTool() tool.Tool {
	return tool.Func("search_files", "Search for a text pattern (string or regex) across workspace files. Returns matching lines with file paths and line numbers.").
		StringParam("pattern", "Search pattern (plain text or regex).", true).
		StringParam("path", "Workspace-relative directory to search in. Defaults to '.'.", false).
		StringParam("file_pattern", "Glob pattern to filter files (e.g. '*.go', '*.js'). Empty means all files.", false).
		IntParam("max_results", "Maximum number of matches to return. Defaults to 20.", false).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			pattern, _ := args["pattern"].(string)
			searchPath, _ := args["path"].(string)
			filePattern, _ := args["file_pattern"].(string)
			maxResults := toInt(args["max_results"])
			if maxResults <= 0 {
				maxResults = 20
			}

			root := GetWorkspaceRoot(state)
			if searchPath == "" {
				searchPath = "."
			}

			fullPath, err := ResolvePath(root, searchPath)
			if err != nil {
				return map[string]any{"error": err.Error()}, nil
			}

			// Try to compile as regex; fall back to literal match
			re, regexErr := regexp.Compile(pattern)
			useRegex := regexErr == nil

			type match struct {
				File    string `json:"file"`
				Line    int    `json:"line"`
				Content string `json:"content"`
			}

			var matches []match
			totalMatches := 0
			truncated := false

			walkErr := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil // skip unreadable entries
				}
				if info.IsDir() {
					// Skip hidden directories
					if strings.HasPrefix(info.Name(), ".") && path != fullPath {
						return filepath.SkipDir
					}
					return nil
				}

				// Apply file pattern filter
				if filePattern != "" {
					matched, _ := filepath.Match(filePattern, info.Name())
					if !matched {
						return nil
					}
				}

				// Skip binary/large files
				if info.Size() > 1024*1024 { // 1MB limit
					return nil
				}

				data, err := os.ReadFile(path)
				if err != nil {
					return nil
				}

				relFile, _ := filepath.Rel(root, path)
				relFile = filepath.ToSlash(relFile)
				lines := strings.Split(string(data), "\n")

				for i, line := range lines {
					var found bool
					if useRegex {
						found = re.MatchString(line)
					} else {
						found = strings.Contains(line, pattern)
					}
					if found {
						totalMatches++
						if len(matches) < maxResults {
							content := line
							if len(content) > 200 {
								content = content[:200] + "..."
							}
							matches = append(matches, match{
								File:    relFile,
								Line:    i + 1,
								Content: strings.TrimSpace(content),
							})
						} else {
							truncated = true
						}
					}
				}
				return nil
			})

			if walkErr != nil {
				return map[string]any{"error": fmt.Sprintf("walk: %v", walkErr)}, nil
			}

			// Convert matches to []any for JSON
			matchList := make([]any, len(matches))
			for i, m := range matches {
				matchList[i] = map[string]any{
					"file":    m.File,
					"line":    m.Line,
					"content": m.Content,
				}
			}

			return map[string]any{
				"matches":       matchList,
				"total_matches": totalMatches,
				"truncated":     truncated,
			}, nil
		}).
		Build()
}

// DiffFileTool returns a tool that compares current file content against a stored snapshot.
func DiffFileTool() tool.Tool {
	return tool.Func("diff_file", "Compare current file content against a previous snapshot. Shows line-by-line diff. If no snapshot exists, returns the full file content.").
		StringParam("path", "Workspace-relative path to the file.", true).
		StringParam("task_id", "Optional task ID to scope the snapshot lookup.", false).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			relPath, _ := args["path"].(string)
			taskID, _ := args["task_id"].(string)
			root := GetWorkspaceRoot(state)

			fullPath, err := ResolvePath(root, relPath)
			if err != nil {
				return map[string]any{"error": err.Error()}, nil
			}

			currentData, err := os.ReadFile(fullPath)
			if err != nil {
				return map[string]any{"error": fmt.Sprintf("read: %v", err)}, nil
			}
			currentContent := string(currentData)

			snapLog := GetFileSnapshotLog(state)
			snapshot := snapLog.GetLatest(relPath, taskID)

			if snapshot == nil {
				return map[string]any{
					"diff": "(no previous snapshot — showing full content)",
					"path": relPath,
					"full": currentContent,
				}, nil
			}

			// Compute simple line-by-line diff
			diff := simpleDiff(snapshot.Content, currentContent)
			return map[string]any{
				"diff": diff,
				"path": relPath,
			}, nil
		}).
		Build()
}

// simpleDiff produces a unified-style diff between old and new content.
func simpleDiff(old, new string) string {
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	var sb strings.Builder
	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}

	i, j := 0, 0
	for i < len(oldLines) || j < len(newLines) {
		if i < len(oldLines) && j < len(newLines) {
			if oldLines[i] == newLines[j] {
				sb.WriteString(fmt.Sprintf(" %d: %s\n", j+1, newLines[j]))
				i++
				j++
			} else {
				sb.WriteString(fmt.Sprintf("-%d: %s\n", i+1, oldLines[i]))
				sb.WriteString(fmt.Sprintf("+%d: %s\n", j+1, newLines[j]))
				i++
				j++
			}
		} else if i < len(oldLines) {
			sb.WriteString(fmt.Sprintf("-%d: %s\n", i+1, oldLines[i]))
			i++
		} else {
			sb.WriteString(fmt.Sprintf("+%d: %s\n", j+1, newLines[j]))
			j++
		}
	}
	return sb.String()
}

// toInt converts an interface value to int, handling float64 from JSON.
func toInt(v any) int {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	case int64:
		return int(n)
	}
	return 0
}
