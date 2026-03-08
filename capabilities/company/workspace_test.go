package company

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitWorkspace(t *testing.T) {
	root := t.TempDir()

	if err := InitWorkspace(root); err != nil {
		t.Fatalf("InitWorkspace failed: %v", err)
	}

	// Check shared files exist
	for _, f := range []string{
		"shared/prd.md",
		"shared/architecture.md",
		"shared/decisions.md",
		"shared/task_board.md",
		"shared/updates.md",
		"shared/pips.md",
	} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Errorf("expected %s to exist: %v", f, err)
		}
	}

	// Check agent directories
	for _, a := range agentDirs {
		diary := filepath.Join(root, a, "diary.md")
		if _, err := os.Stat(diary); err != nil {
			t.Errorf("expected %s/diary.md to exist: %v", a, err)
		}
		notes := filepath.Join(root, a, "notes.md")
		if _, err := os.Stat(notes); err != nil {
			t.Errorf("expected %s/notes.md to exist: %v", a, err)
		}
	}

	// Check dev plan directories
	for _, a := range devAgents {
		plans := filepath.Join(root, a, "plans")
		info, err := os.Stat(plans)
		if err != nil {
			t.Errorf("expected %s/plans/ to exist: %v", a, err)
		} else if !info.IsDir() {
			t.Errorf("expected %s/plans/ to be a directory", a)
		}
	}

	// Check architect reviews dir
	reviews := filepath.Join(root, "architect", "reviews")
	info, err := os.Stat(reviews)
	if err != nil {
		t.Errorf("expected architect/reviews/ to exist: %v", err)
	} else if !info.IsDir() {
		t.Error("expected architect/reviews/ to be a directory")
	}

	// Check src directories
	for _, d := range []string{"src/backend", "src/frontend", "src/infra"} {
		info, err := os.Stat(filepath.Join(root, d))
		if err != nil {
			t.Errorf("expected %s to exist: %v", d, err)
		} else if !info.IsDir() {
			t.Errorf("expected %s to be a directory", d)
		}
	}
}

func TestResolvePath(t *testing.T) {
	root := t.TempDir()

	// Valid path
	p, err := ResolvePath(root, "shared/prd.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(root, "shared", "prd.md")
	if p != expected {
		t.Errorf("expected %s, got %s", expected, p)
	}

	// Path traversal should fail
	_, err = ResolvePath(root, "../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestSyncTaskBoard(t *testing.T) {
	root := t.TempDir()
	if err := InitWorkspace(root); err != nil {
		t.Fatalf("InitWorkspace failed: %v", err)
	}

	tb := NewTaskBoard()
	tb.Add("Test task", "A test", "backend-dev", "", "", 0)

	if err := SyncTaskBoard(root, tb); err != nil {
		t.Fatalf("SyncTaskBoard failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "shared", "task_board.md"))
	if err != nil {
		t.Fatalf("read task board: %v", err)
	}

	content := string(data)
	if content == "" {
		t.Error("task board file should not be empty")
	}
}
