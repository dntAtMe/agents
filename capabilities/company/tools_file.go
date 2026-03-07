package company

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dntatme/agents/tool"
)

// WriteFileTool returns a tool that writes/overwrites a file in the workspace.
func WriteFileTool() tool.Tool {
	return tool.Func("write_file", "Write or overwrite a file in the workspace. Auto-creates parent directories.").
		StringParam("path", "Workspace-relative path (e.g. 'shared/prd.md' or 'src/backend/main.go').", true).
		StringParam("content", "The full content to write.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			relPath, _ := args["path"].(string)
			content, _ := args["content"].(string)
			root := GetWorkspaceRoot(state)

			fullPath, err := ResolvePath(root, relPath)
			if err != nil {
				return map[string]any{"error": err.Error()}, nil
			}

			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				return map[string]any{"error": fmt.Sprintf("create dirs: %v", err)}, nil
			}

			if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
				return map[string]any{"error": fmt.Sprintf("write: %v", err)}, nil
			}

			return map[string]any{"status": "written", "path": relPath}, nil
		}).
		Build()
}

// ReadFileTool returns a tool that reads a file from the workspace.
func ReadFileTool() tool.Tool {
	return tool.Func("read_file", "Read the contents of a file in the workspace.").
		StringParam("path", "Workspace-relative path to read.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			relPath, _ := args["path"].(string)
			root := GetWorkspaceRoot(state)

			fullPath, err := ResolvePath(root, relPath)
			if err != nil {
				return map[string]any{"error": err.Error()}, nil
			}

			data, err := os.ReadFile(fullPath)
			if err != nil {
				return map[string]any{"error": fmt.Sprintf("read: %v", err)}, nil
			}

			return map[string]any{"content": string(data), "path": relPath}, nil
		}).
		Build()
}

// AppendToFileTool returns a tool that appends content to a file.
func AppendToFileTool() tool.Tool {
	return tool.Func("append_to_file", "Append content to the end of a file in the workspace.").
		StringParam("path", "Workspace-relative path.", true).
		StringParam("content", "Content to append.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			relPath, _ := args["path"].(string)
			content, _ := args["content"].(string)
			root := GetWorkspaceRoot(state)

			fullPath, err := ResolvePath(root, relPath)
			if err != nil {
				return map[string]any{"error": err.Error()}, nil
			}

			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				return map[string]any{"error": fmt.Sprintf("create dirs: %v", err)}, nil
			}

			f, err := os.OpenFile(fullPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				return map[string]any{"error": fmt.Sprintf("open: %v", err)}, nil
			}
			defer f.Close()

			if _, err := f.WriteString(content); err != nil {
				return map[string]any{"error": fmt.Sprintf("append: %v", err)}, nil
			}

			return map[string]any{"status": "appended", "path": relPath}, nil
		}).
		Build()
}

// ListFilesTool returns a tool that lists directory entries.
func ListFilesTool() tool.Tool {
	return tool.Func("list_files", "List files and directories at the given workspace path.").
		StringParam("path", "Workspace-relative directory path. Use '.' for workspace root.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			relPath, _ := args["path"].(string)
			root := GetWorkspaceRoot(state)

			fullPath, err := ResolvePath(root, relPath)
			if err != nil {
				return map[string]any{"error": err.Error()}, nil
			}

			entries, err := os.ReadDir(fullPath)
			if err != nil {
				return map[string]any{"error": fmt.Sprintf("list: %v", err)}, nil
			}

			var items []string
			for _, e := range entries {
				name := e.Name()
				if e.IsDir() {
					name += "/"
				}
				items = append(items, name)
			}

			return map[string]any{"entries": strings.Join(items, "\n"), "path": relPath}, nil
		}).
		Build()
}
