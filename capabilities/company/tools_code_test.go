package company

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditFile(t *testing.T) {
	root, state := setupTestWorkspace(t)
	ctx := context.Background()

	// Write a file first
	content := "line 1\nline 2\nline 3\nline 4\nline 5\n"
	filePath := filepath.Join(root, "src", "backend", "test.go")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	et := EditFileTool()

	// Replace lines 2-3 with new content
	result, err := et.Execute(ctx, map[string]any{
		"path":        "src/backend/test.go",
		"start_line":  float64(2),
		"end_line":    float64(3),
		"new_content": "new line 2\nnew line 3\nextra line",
	}, state)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if result["status"] != "edited" {
		t.Errorf("expected status 'edited', got %v", result["status"])
	}
	if result["lines_removed"] != 2 {
		t.Errorf("expected 2 lines removed, got %v", result["lines_removed"])
	}
	if result["lines_added"] != 3 {
		t.Errorf("expected 3 lines added, got %v", result["lines_added"])
	}

	// Verify file content
	// Original: line1, line2, line3, line4, line5
	// After replacing lines 2-3 with 3 new lines: line1, new2, new3, extra, line4, line5
	data, _ := os.ReadFile(filePath)
	lines := strings.Split(string(data), "\n")
	if lines[0] != "line 1" {
		t.Errorf("expected 'line 1', got %q", lines[0])
	}
	if lines[1] != "new line 2" {
		t.Errorf("expected 'new line 2', got %q", lines[1])
	}
	if lines[3] != "line 4" {
		// lines[3] should be "extra line" since we inserted 3 lines to replace 2
		// line 4 is now at index 4
	}
	if lines[4] != "line 4" {
		t.Errorf("expected 'line 4' at index 4, got %q", lines[4])
	}
}

func TestEditFileOutOfBounds(t *testing.T) {
	root, state := setupTestWorkspace(t)
	ctx := context.Background()

	filePath := filepath.Join(root, "src", "backend", "small.go")
	if err := os.WriteFile(filePath, []byte("only one line"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	et := EditFileTool()
	result, err := et.Execute(ctx, map[string]any{
		"path":        "src/backend/small.go",
		"start_line":  float64(5),
		"end_line":    float64(10),
		"new_content": "replacement",
	}, state)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error for out-of-bounds lines")
	}
}

func TestEditFileSingleLine(t *testing.T) {
	root, state := setupTestWorkspace(t)
	ctx := context.Background()

	filePath := filepath.Join(root, "src", "backend", "single.go")
	if err := os.WriteFile(filePath, []byte("original content"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	et := EditFileTool()
	result, err := et.Execute(ctx, map[string]any{
		"path":        "src/backend/single.go",
		"start_line":  float64(1),
		"end_line":    float64(1),
		"new_content": "replaced content",
	}, state)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if result["status"] != "edited" {
		t.Errorf("expected 'edited', got %v", result["status"])
	}

	data, _ := os.ReadFile(filePath)
	if string(data) != "replaced content" {
		t.Errorf("expected 'replaced content', got %q", string(data))
	}
}

func TestSearchFiles(t *testing.T) {
	root, state := setupTestWorkspace(t)
	ctx := context.Background()

	// Write some files to search
	os.WriteFile(filepath.Join(root, "src", "backend", "main.go"), []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "src", "backend", "handler.go"), []byte("package main\n\nfunc handleRequest() {\n\tfmt.Println(\"handling\")\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "src", "frontend", "app.js"), []byte("function main() {\n  console.log('hello');\n}\n"), 0o644)

	st := SearchFilesTool()

	// Search for "Println"
	result, err := st.Execute(ctx, map[string]any{
		"pattern": "Println",
	}, state)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	totalMatches := result["total_matches"]
	if totalMatches == nil || toInt(totalMatches) < 2 {
		t.Errorf("expected at least 2 matches for Println, got %v", totalMatches)
	}

	// Search with file pattern filter
	result, err = st.Execute(ctx, map[string]any{
		"pattern":      "main",
		"file_pattern": "*.go",
	}, state)
	if err != nil {
		t.Fatalf("search with filter: %v", err)
	}
	matches := result["matches"].([]any)
	for _, m := range matches {
		mm := m.(map[string]any)
		if strings.HasSuffix(mm["file"].(string), ".js") {
			t.Error("should not match .js files when filtering for *.go")
		}
	}
}

func TestSearchFilesRegex(t *testing.T) {
	root, state := setupTestWorkspace(t)
	ctx := context.Background()

	os.WriteFile(filepath.Join(root, "src", "backend", "test.go"), []byte("func TestFoo() {\n}\nfunc TestBar() {\n}\nfunc helper() {\n}\n"), 0o644)

	st := SearchFilesTool()
	result, err := st.Execute(ctx, map[string]any{
		"pattern": "func Test\\w+",
		"path":    "src/backend",
	}, state)
	if err != nil {
		t.Fatalf("regex search: %v", err)
	}
	if toInt(result["total_matches"]) != 2 {
		t.Errorf("expected 2 regex matches, got %v", result["total_matches"])
	}
}

func TestSearchFilesMaxResults(t *testing.T) {
	root, state := setupTestWorkspace(t)
	ctx := context.Background()

	// Write a file with many matches
	var lines []string
	for i := 0; i < 30; i++ {
		lines = append(lines, "match_me here")
	}
	os.WriteFile(filepath.Join(root, "src", "backend", "many.txt"), []byte(strings.Join(lines, "\n")), 0o644)

	st := SearchFilesTool()
	result, err := st.Execute(ctx, map[string]any{
		"pattern":     "match_me",
		"max_results": float64(5),
	}, state)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	matches := result["matches"].([]any)
	if len(matches) != 5 {
		t.Errorf("expected 5 matches, got %d", len(matches))
	}
	if result["truncated"] != true {
		t.Error("expected truncated to be true")
	}
}

func TestDiffFile(t *testing.T) {
	root, state := setupTestWorkspace(t)
	ctx := context.Background()

	// Write original and save snapshot
	filePath := filepath.Join(root, "src", "backend", "diff_test.go")
	os.WriteFile(filePath, []byte("line 1\nline 2\nline 3\n"), 0o644)

	snapLog := GetFileSnapshotLog(state)
	snapLog.Save("src/backend/diff_test.go", "TASK-001", "line 1\nline 2\nline 3\n", 1)

	// Modify file
	os.WriteFile(filePath, []byte("line 1\nmodified line 2\nline 3\nnew line 4\n"), 0o644)

	dt := DiffFileTool()
	result, err := dt.Execute(ctx, map[string]any{
		"path":    "src/backend/diff_test.go",
		"task_id": "TASK-001",
	}, state)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	diff := result["diff"].(string)
	if !strings.Contains(diff, "-") || !strings.Contains(diff, "+") {
		t.Error("diff should contain additions and removals")
	}
	if !strings.Contains(diff, "modified line 2") {
		t.Error("diff should show modified line")
	}
}

func TestDiffFileNoSnapshot(t *testing.T) {
	root, state := setupTestWorkspace(t)
	ctx := context.Background()

	filePath := filepath.Join(root, "src", "backend", "new_file.go")
	os.WriteFile(filePath, []byte("brand new content\n"), 0o644)

	dt := DiffFileTool()
	result, err := dt.Execute(ctx, map[string]any{
		"path": "src/backend/new_file.go",
	}, state)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if result["full"] == nil {
		t.Error("expected full content when no snapshot exists")
	}
}
