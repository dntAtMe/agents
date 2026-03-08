package company

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// agentDirs lists all agent directory names.
var agentDirs = []string{
	"ceo", "cto", "architect", "product-manager", "project-manager",
	"backend-dev", "frontend-dev", "devops",
}

// devAgents lists agents that have plans/ subdirectories.
var devAgents = []string{"backend-dev", "frontend-dev", "devops"}

// InitWorkspace creates the workspace directory structure.
func InitWorkspace(root string) error {
	dirs := []string{
		"shared",
		"shared/meetings",
		"shared/reviews",
		"shared/code-reviews",
		"architect/reviews",
		"src/backend",
		"src/frontend",
		"src/infra",
	}

	// Agent directories with diary.md and notes.md
	for _, a := range agentDirs {
		dirs = append(dirs, a)
	}

	// Dev agents get plans/ subdirectory
	for _, a := range devAgents {
		dirs = append(dirs, filepath.Join(a, "plans"))
	}

	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", d, err)
		}
	}

	// Create initial empty files
	initialFiles := map[string]string{
		"shared/prd.md":          "# Product Requirements Document\n\n*Not yet written.*\n",
		"shared/architecture.md": "# Technical Architecture\n\n*Not yet written.*\n",
		"shared/decisions.md":    "# Architectural Decision Records\n\n*No decisions yet.*\n",
		"shared/task_board.md":   "# Task Board\n\n*No tasks yet.*\n",
		"shared/updates.md":      "# Updates\n\n*No updates yet.*\n",
		"shared/escalations.md":  "# Escalations\n\nNo escalations filed.\n",
		"shared/pips.md":         "# Performance Improvement Plans (PiP)\n\nNo PiP records.\n",
		"shared/firings.md":      "# Firing Requests\n\nNo firing requests.\n",
	}

	for _, a := range agentDirs {
		initialFiles[filepath.Join(a, "diary.md")] = fmt.Sprintf("# %s's Diary\n\n", a)
		initialFiles[filepath.Join(a, "notes.md")] = fmt.Sprintf("# %s's Notes\n\n", a)
		initialFiles[filepath.Join(a, "inbox.md")] = fmt.Sprintf("# %s's Inbox\n\nNo emails.\n", a)
	}

	for relPath, content := range initialFiles {
		fullPath := filepath.Join(root, relPath)
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", relPath, err)
		}
	}

	return nil
}

// ResolvePath resolves a workspace-relative path to an absolute path.
// It validates the path stays within the workspace root.
func ResolvePath(root, relPath string) (string, error) {
	// Clean and join
	full := filepath.Join(root, filepath.Clean(relPath))
	// Ensure it's within root
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(absFull, absRoot) {
		return "", fmt.Errorf("path %q escapes workspace root", relPath)
	}
	return absFull, nil
}

// SyncTaskBoard writes the current task board to shared/task_board.md.
func SyncTaskBoard(root string, tb *TaskBoard) error {
	content := tb.Render()
	return os.WriteFile(filepath.Join(root, "shared", "task_board.md"), []byte(content), 0o644)
}

// SyncDecisions writes the current decisions to shared/decisions.md.
func SyncDecisions(root string, dl *DecisionLog) error {
	content := dl.Render()
	return os.WriteFile(filepath.Join(root, "shared", "decisions.md"), []byte(content), 0o644)
}

// SyncUpdates writes the current updates to shared/updates.md.
func SyncUpdates(root string, ul *UpdateLog) error {
	content := ul.Render("", 0)
	return os.WriteFile(filepath.Join(root, "shared", "updates.md"), []byte(content), 0o644)
}

// SyncMeetingNotes writes a meeting's notes to shared/meetings/MEET-{id}.md.
func SyncMeetingNotes(root string, ml *MeetingLog, m Meeting) error {
	content := ml.RenderMeeting(m)
	return os.WriteFile(filepath.Join(root, "shared", "meetings", m.ID+".md"), []byte(content), 0o644)
}

// SyncInbox writes an agent's inbox to {agent}/inbox.md.
func SyncInbox(root string, el *EmailLog, agentName string) error {
	emails := el.Inbox(agentName, false, "")
	content := el.RenderInbox(emails)
	return os.WriteFile(filepath.Join(root, agentName, "inbox.md"), []byte(content), 0o644)
}

// SyncEscalations writes escalations to shared/escalations.md.
func SyncEscalations(root string, el *EscalationLog) error {
	content := el.Render()
	return os.WriteFile(filepath.Join(root, "shared", "escalations.md"), []byte(content), 0o644)
}

// SyncPiPs writes PiP records to shared/pips.md.
func SyncPiPs(root string, pl *PiPLog) error {
	content := pl.Render()
	return os.WriteFile(filepath.Join(root, "shared", "pips.md"), []byte(content), 0o644)
}

// SyncFirings writes firing requests to shared/firings.md.
func SyncFirings(root string, fl *FiringLog) error {
	content := fl.Render()
	return os.WriteFile(filepath.Join(root, "shared", "firings.md"), []byte(content), 0o644)
}

// SyncRelationships writes an agent's relationship scores to {agent}/relationships.md.
func SyncRelationships(root string, rl *RelationshipLog, agentName string) error {
	content := rl.RenderForAgent(agentName)
	if content == "" {
		content = "# Relationships\n\nNo relationship scores recorded yet.\n"
	}
	return os.WriteFile(filepath.Join(root, agentName, "relationships.md"), []byte(content), 0o644)
}

// SyncCodeReviews writes a code review to shared/code-reviews/{task_id}-review-R{round}.md.
func SyncCodeReviews(root string, log *CodeReviewLog, review CodeReview) error {
	dir := filepath.Join(root, "shared", "code-reviews")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	round := log.ReviewRoundForTask(review.TaskID)
	filename := fmt.Sprintf("%s-review-R%d.md", review.TaskID, round)
	content := log.RenderWithSource(review, root)
	return os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644)
}

// SyncCommandLog writes the command log to shared/command-log.md.
func SyncCommandLog(root string, cl *CommandLog) error {
	content := cl.Render()
	return os.WriteFile(filepath.Join(root, "shared", "command-log.md"), []byte(content), 0o644)
}
