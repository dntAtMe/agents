package company

import (
	"context"
	"testing"
)

func TestRunCommandAllowList(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	rt := RunCommandTool()

	// Test rejected command
	result, err := rt.Execute(ctx, map[string]any{
		"command": "rm -rf /",
	}, state)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error for disallowed command 'rm'")
	}
}

func TestRunCommandShellOperators(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	rt := RunCommandTool()

	operators := []string{
		"ls | grep foo",
		"ls ; rm -rf /",
		"ls && echo done",
		"ls || echo fail",
		"echo $(whoami)",
		"echo `whoami`",
		"ls > output.txt",
		"ls >> output.txt",
	}

	for _, cmd := range operators {
		result, err := rt.Execute(ctx, map[string]any{
			"command": cmd,
		}, state)
		if err != nil {
			t.Fatalf("run %q: %v", cmd, err)
		}
		if _, ok := result["error"]; !ok {
			t.Errorf("expected error for shell operator in %q", cmd)
		}
	}
}

func TestRunCommandEmptyCommand(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	rt := RunCommandTool()
	result, err := rt.Execute(ctx, map[string]any{
		"command": "",
	}, state)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error for empty command")
	}
}

func TestRunCommandSuccess(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	rt := RunCommandTool()

	// Portable: Go is always on PATH when running these tests.
	cmd := "go version"

	result, err := rt.Execute(ctx, map[string]any{
		"command": cmd,
	}, state)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, ok := result["error"]; ok {
		t.Errorf("unexpected error: %v", result["error"])
	}
	if result["exit_code"] != 0 {
		t.Errorf("expected exit code 0, got %v", result["exit_code"])
	}
	if result["timed_out"] != false {
		t.Error("should not have timed out")
	}

	// Verify command was logged
	cmdLog := GetCommandLog(state)
	cmdLog.mu.Lock()
	count := len(cmdLog.Commands)
	cmdLog.mu.Unlock()
	if count != 1 {
		t.Errorf("expected 1 command logged, got %d", count)
	}
}

func TestRunCommandTimeoutClamped(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	rt := RunCommandTool()

	// Timeout > 60 should be clamped
	cmd := "go version"
	result, err := rt.Execute(ctx, map[string]any{
		"command":         cmd,
		"timeout_seconds": float64(120),
	}, state)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Should still succeed (clamped to 60, not rejected)
	if _, ok := result["error"]; ok {
		t.Errorf("unexpected error: %v", result["error"])
	}
}
