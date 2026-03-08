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

// MeetingEntry represents a single statement in a group meeting.
type MeetingEntry struct {
	Speaker string `json:"speaker"`
	Round   int    `json:"round"` // meeting-internal round (1 or 2)
	Message string `json:"message"`
}

// Meeting represents a group meeting session.
type Meeting struct {
	ID         string         `json:"id"`
	CalledBy   string         `json:"called_by"`
	Agenda     string         `json:"agenda"`
	Attendees  []string       `json:"attendees"`
	Transcript []MeetingEntry `json:"transcript"`
	SimRound   int            `json:"sim_round"`
	Time       time.Time      `json:"time"`
}

// MeetingLog holds all meetings with thread-safe access.
type MeetingLog struct {
	mu       sync.Mutex
	Meetings []Meeting `json:"meetings"`
	counter  int
}

// NewMeetingLog creates an empty meeting log.
func NewMeetingLog() *MeetingLog {
	return &MeetingLog{}
}

// NextID returns the next meeting ID (MEET-001, MEET-002, etc).
func (ml *MeetingLog) NextID() string {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	ml.counter++
	return fmt.Sprintf("MEET-%03d", ml.counter)
}

// Save adds a meeting to the log.
func (ml *MeetingLog) Save(m Meeting) {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	ml.Meetings = append(ml.Meetings, m)
}

// RenderMeeting produces a markdown representation of a single meeting.
func (ml *MeetingLog) RenderMeeting(m Meeting) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Meeting %s\n\n", m.ID))
	sb.WriteString(fmt.Sprintf("**Called by:** %s\n", m.CalledBy))
	sb.WriteString(fmt.Sprintf("**Agenda:** %s\n", m.Agenda))
	sb.WriteString(fmt.Sprintf("**Attendees:** %s\n", strings.Join(m.Attendees, ", ")))
	sb.WriteString(fmt.Sprintf("**Round:** %d\n\n", m.SimRound))
	sb.WriteString("## Transcript\n\n")
	for _, entry := range m.Transcript {
		sb.WriteString(fmt.Sprintf("### Round %d — %s\n\n%s\n\n", entry.Round, entry.Speaker, entry.Message))
	}
	return sb.String()
}

// Email represents a single email message.
type Email struct {
	ID        string   `json:"id"`
	ThreadID  string   `json:"thread_id"`
	From      string   `json:"from"`
	To        []string `json:"to"`
	Subject   string   `json:"subject"`
	Body      string   `json:"body"`
	Round     int      `json:"round"`
	Time      time.Time `json:"time"`
	InReplyTo string   `json:"in_reply_to,omitempty"`
}

// EmailLog holds all emails with thread-safe access.
type EmailLog struct {
	mu      sync.Mutex
	Emails  []Email                    `json:"emails"`
	counter int
	ReadBy  map[string]map[string]bool `json:"-"` // agentName → emailID → read
}

// NewEmailLog creates an empty email log.
func NewEmailLog() *EmailLog {
	return &EmailLog{
		ReadBy: make(map[string]map[string]bool),
	}
}

// Send creates a new email and returns its ID.
func (el *EmailLog) Send(from string, to []string, subject, body string, round int) string {
	el.mu.Lock()
	defer el.mu.Unlock()
	el.counter++
	id := fmt.Sprintf("EMAIL-%03d", el.counter)
	email := Email{
		ID:       id,
		ThreadID: id,
		From:     from,
		To:       to,
		Subject:  subject,
		Body:     body,
		Round:    round,
		Time:     time.Now(),
	}
	el.Emails = append(el.Emails, email)
	return id
}

// Reply creates a reply to an existing email. Returns the new email and an error
// if the parent doesn't exist or the caller wasn't a participant.
func (el *EmailLog) Reply(from, parentID, body string, round int) (Email, error) {
	el.mu.Lock()
	defer el.mu.Unlock()

	// Find parent email
	var parent *Email
	for i := range el.Emails {
		if el.Emails[i].ID == parentID {
			parent = &el.Emails[i]
			break
		}
	}
	if parent == nil {
		return Email{}, fmt.Errorf("email %q not found", parentID)
	}

	// Check caller was a participant (sender or recipient)
	isParticipant := parent.From == from
	if !isParticipant {
		for _, r := range parent.To {
			if r == from {
				isParticipant = true
				break
			}
		}
	}
	if !isParticipant {
		return Email{}, fmt.Errorf("you are not a participant in email %q", parentID)
	}

	// Build recipients: all original participants minus self
	recipientSet := make(map[string]bool)
	recipientSet[parent.From] = true
	for _, r := range parent.To {
		recipientSet[r] = true
	}
	delete(recipientSet, from)
	var to []string
	for r := range recipientSet {
		to = append(to, r)
	}

	// Build subject
	subject := parent.Subject
	if !strings.HasPrefix(subject, "Re: ") {
		subject = "Re: " + subject
	}

	el.counter++
	id := fmt.Sprintf("EMAIL-%03d", el.counter)
	email := Email{
		ID:        id,
		ThreadID:  parent.ThreadID,
		From:      from,
		To:        to,
		Subject:   subject,
		Body:      body,
		Round:     round,
		Time:      time.Now(),
		InReplyTo: parentID,
	}
	el.Emails = append(el.Emails, email)
	return email, nil
}

// Inbox returns emails for a given agent, optionally filtered.
func (el *EmailLog) Inbox(agentName string, unreadOnly bool, fromFilter string) []Email {
	el.mu.Lock()
	defer el.mu.Unlock()

	var result []Email
	for _, e := range el.Emails {
		// Check if agent is a recipient
		isRecipient := false
		for _, r := range e.To {
			if r == agentName {
				isRecipient = true
				break
			}
		}
		if !isRecipient {
			continue
		}

		// Apply from filter
		if fromFilter != "" && e.From != fromFilter {
			continue
		}

		// Apply unread filter
		if unreadOnly {
			if agentReads, ok := el.ReadBy[agentName]; ok {
				if agentReads[e.ID] {
					continue
				}
			}
		}

		result = append(result, e)
	}
	return result
}

// MarkReadBatch marks a batch of emails as read by the given agent.
func (el *EmailLog) MarkReadBatch(agentName string, emails []Email) {
	el.mu.Lock()
	defer el.mu.Unlock()

	if _, ok := el.ReadBy[agentName]; !ok {
		el.ReadBy[agentName] = make(map[string]bool)
	}
	for _, e := range emails {
		el.ReadBy[agentName][e.ID] = true
	}
}

// RenderInbox produces a markdown representation of an email list.
func (el *EmailLog) RenderInbox(emails []Email) string {
	var sb strings.Builder
	sb.WriteString("# Inbox\n\n")
	if len(emails) == 0 {
		sb.WriteString("No emails.\n")
		return sb.String()
	}
	for _, e := range emails {
		sb.WriteString(fmt.Sprintf("## %s: %s\n\n", e.ID, e.Subject))
		sb.WriteString(fmt.Sprintf("**From:** %s\n", e.From))
		sb.WriteString(fmt.Sprintf("**To:** %s\n", strings.Join(e.To, ", ")))
		if e.InReplyTo != "" {
			sb.WriteString(fmt.Sprintf("**In reply to:** %s\n", e.InReplyTo))
		}
		sb.WriteString(fmt.Sprintf("**Thread:** %s\n", e.ThreadID))
		sb.WriteString(fmt.Sprintf("\n%s\n\n---\n\n", e.Body))
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
