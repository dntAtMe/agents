package company

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Task represents a work item on the task board.
type Task struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Assignee    string `json:"assignee"`
	Status      string `json:"status"` // todo, in_progress, awaiting_review, needs_changes, approved, done, blocked
	Priority    string `json:"priority"`
	DependsOn   string `json:"depends_on,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

// TaskBoard holds all tasks with thread-safe access.
type TaskBoard struct {
	mu      sync.Mutex
	Tasks   []Task `json:"tasks"`
	counter int
}

// NewTaskBoard creates an empty task board.
func NewTaskBoard() *TaskBoard {
	return &TaskBoard{}
}

// Add creates a new task and returns its ID.
func (tb *TaskBoard) Add(title, description, assignee, priority, dependsOn string) string {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.counter++
	id := fmt.Sprintf("TASK-%03d", tb.counter)
	if priority == "" {
		priority = "medium"
	}
	tb.Tasks = append(tb.Tasks, Task{
		ID:          id,
		Title:       title,
		Description: description,
		Assignee:    assignee,
		Status:      "todo",
		Priority:    priority,
		DependsOn:   dependsOn,
	})
	return id
}

// Update modifies an existing task's status and optional notes.
func (tb *TaskBoard) Update(id, status, notes string) error {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	for i := range tb.Tasks {
		if tb.Tasks[i].ID == id {
			tb.Tasks[i].Status = status
			if notes != "" {
				tb.Tasks[i].Notes = notes
			}
			return nil
		}
	}
	return fmt.Errorf("task %q not found", id)
}

// Render produces a markdown representation of the task board grouped by status.
func (tb *TaskBoard) Render() string {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	statusOrder := []string{"todo", "in_progress", "awaiting_review", "needs_changes", "approved", "done", "blocked"}
	grouped := make(map[string][]Task)
	for _, t := range tb.Tasks {
		grouped[t.Status] = append(grouped[t.Status], t)
	}

	var sb strings.Builder
	sb.WriteString("# Task Board\n\n")
	for _, s := range statusOrder {
		tasks := grouped[s]
		if len(tasks) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("## %s\n\n", strings.ToUpper(s)))
		for _, t := range tasks {
			sb.WriteString(fmt.Sprintf("- **%s**: %s (assignee: %s, priority: %s)\n", t.ID, t.Title, t.Assignee, t.Priority))
			if t.Notes != "" {
				sb.WriteString(fmt.Sprintf("  Notes: %s\n", t.Notes))
			}
			if t.DependsOn != "" {
				sb.WriteString(fmt.Sprintf("  Depends on: %s\n", t.DependsOn))
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// Decision represents an architectural decision record.
type Decision struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Decision     string `json:"decision"`
	Rationale    string `json:"rationale"`
	Alternatives string `json:"alternatives,omitempty"`
}

// DecisionLog holds all architectural decisions.
type DecisionLog struct {
	mu        sync.Mutex
	Decisions []Decision `json:"decisions"`
	counter   int
}

// NewDecisionLog creates an empty decision log.
func NewDecisionLog() *DecisionLog {
	return &DecisionLog{}
}

// Add creates a new decision and returns its ID.
func (dl *DecisionLog) Add(title, decision, rationale, alternatives string) string {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.counter++
	id := fmt.Sprintf("ADR-%03d", dl.counter)
	dl.Decisions = append(dl.Decisions, Decision{
		ID:           id,
		Title:        title,
		Decision:     decision,
		Rationale:    rationale,
		Alternatives: alternatives,
	})
	return id
}

// Render produces a markdown representation of all decisions.
func (dl *DecisionLog) Render() string {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("# Architectural Decision Records\n\n")
	for _, d := range dl.Decisions {
		sb.WriteString(fmt.Sprintf("## %s: %s\n\n", d.ID, d.Title))
		sb.WriteString(fmt.Sprintf("**Decision:** %s\n\n", d.Decision))
		sb.WriteString(fmt.Sprintf("**Rationale:** %s\n\n", d.Rationale))
		if d.Alternatives != "" {
			sb.WriteString(fmt.Sprintf("**Alternatives considered:** %s\n\n", d.Alternatives))
		}
		sb.WriteString("---\n\n")
	}
	return sb.String()
}

// Update represents a message in the updates channel.
type Update struct {
	Round   int       `json:"round"`
	Agent   string    `json:"agent"`
	Channel string    `json:"channel"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

// UpdateLog holds all posted updates.
type UpdateLog struct {
	mu      sync.Mutex
	Updates []Update `json:"updates"`
}

// NewUpdateLog creates an empty update log.
func NewUpdateLog() *UpdateLog {
	return &UpdateLog{}
}

// Post adds an update message.
func (ul *UpdateLog) Post(round int, agent, channel, message string) {
	ul.mu.Lock()
	defer ul.mu.Unlock()
	if channel == "" {
		channel = "general"
	}
	ul.Updates = append(ul.Updates, Update{
		Round:   round,
		Agent:   agent,
		Channel: channel,
		Message: message,
		Time:    time.Now(),
	})
}

// Read returns updates, optionally filtered by channel and since_round.
func (ul *UpdateLog) Read(channel string, sinceRound int) []Update {
	ul.mu.Lock()
	defer ul.mu.Unlock()

	var result []Update
	for _, u := range ul.Updates {
		if sinceRound > 0 && u.Round < sinceRound {
			continue
		}
		if channel != "" && u.Channel != channel {
			continue
		}
		result = append(result, u)
	}
	return result
}

// Render produces a markdown representation of updates.
func (ul *UpdateLog) Render(channel string, sinceRound int) string {
	updates := ul.Read(channel, sinceRound)
	var sb strings.Builder
	sb.WriteString("# Updates\n\n")
	for _, u := range updates {
		sb.WriteString(fmt.Sprintf("**[Round %d] %s** (%s): %s\n\n", u.Round, u.Agent, u.Channel, u.Message))
	}
	if len(updates) == 0 {
		sb.WriteString("No updates.\n")
	}
	return sb.String()
}
