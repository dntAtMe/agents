package company

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
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
	Deadline    int    `json:"deadline,omitempty"` // simulation round by which the task should be done (0 = none)
	Reviewer    string `json:"reviewer,omitempty"`
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
func (tb *TaskBoard) Add(title, description, assignee, priority, dependsOn string, deadline int, reviewer string) string {
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
		Deadline:    deadline,
		Reviewer:    reviewer,
	})
	return id
}

// GetByID returns a pointer to the task with the given ID, or nil if not found.
func (tb *TaskBoard) GetByID(id string) *Task {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	for i := range tb.Tasks {
		if tb.Tasks[i].ID == id {
			return &tb.Tasks[i]
		}
	}
	return nil
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

// taskBoardJSON is the on-disk / wire shape for shared/tasks.json.
type taskBoardJSON struct {
	Tasks   []Task `json:"tasks"`
	Counter int    `json:"counter"`
}

// SnapshotTasks returns a shallow copy of tasks for TUI or tool output.
func (tb *TaskBoard) SnapshotTasks() []Task {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	out := make([]Task, len(tb.Tasks))
	copy(out, tb.Tasks)
	return out
}

// MarshalJSON exports the full board including the ID counter.
func (tb *TaskBoard) MarshalJSON() ([]byte, error) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return json.Marshal(taskBoardJSON{Tasks: tb.Tasks, Counter: tb.counter})
}

// TaskBoardFromJSON loads a task board from JSON bytes (shared/tasks.json).
func TaskBoardFromJSON(data []byte) (*TaskBoard, error) {
	var raw taskBoardJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	tb := NewTaskBoard()
	tb.mu.Lock()
	tb.Tasks = raw.Tasks
	if raw.Counter > 0 {
		tb.counter = raw.Counter
	} else {
		for _, t := range tb.Tasks {
			if n, ok := parseTaskCounter(t.ID); ok && n > tb.counter {
				tb.counter = n
			}
		}
	}
	tb.mu.Unlock()
	return tb, nil
}

func parseTaskCounter(id string) (int, bool) {
	const p = "TASK-"
	if !strings.HasPrefix(id, p) {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(id, p))
	if err != nil {
		return 0, false
	}
	return n, true
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
	ID        string    `json:"id"`
	ThreadID  string    `json:"thread_id"`
	From      string    `json:"from"`
	To        []string  `json:"to"`
	CC        []string  `json:"cc,omitempty"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	Round     int       `json:"round"`
	Time      time.Time `json:"time"`
	InReplyTo string    `json:"in_reply_to,omitempty"`
	Urgent    bool      `json:"urgent,omitempty"`
}

// EmailLog holds all emails with thread-safe access.
type EmailLog struct {
	mu      sync.Mutex
	Emails  []Email `json:"emails"`
	counter int
	ReadBy  map[string]map[string]bool `json:"-"` // agentName → emailID → read
	// UrgentSent tracks urgent emails: "sender:recipient:round" → true.
	// Used to enforce one urgent email per sender→recipient per round.
	UrgentSent map[string]bool `json:"-"`
}

// NewEmailLog creates an empty email log.
func NewEmailLog() *EmailLog {
	return &EmailLog{
		ReadBy:     make(map[string]map[string]bool),
		UrgentSent: make(map[string]bool),
	}
}

// CanSendUrgent checks whether sender can send an urgent email to recipient in the given round.
func (el *EmailLog) CanSendUrgent(sender, recipient string, round int) bool {
	el.mu.Lock()
	defer el.mu.Unlock()
	key := fmt.Sprintf("%s:%s:%d", sender, recipient, round)
	return !el.UrgentSent[key]
}

// RecordUrgent marks that sender sent an urgent email to recipient in the given round.
func (el *EmailLog) RecordUrgent(sender, recipient string, round int) {
	el.mu.Lock()
	defer el.mu.Unlock()
	if el.UrgentSent == nil {
		el.UrgentSent = make(map[string]bool)
	}
	key := fmt.Sprintf("%s:%s:%d", sender, recipient, round)
	el.UrgentSent[key] = true
}

// Send creates a new email and returns its ID.
func (el *EmailLog) Send(from string, to, cc []string, subject, body string, round int, urgent bool) string {
	el.mu.Lock()
	defer el.mu.Unlock()
	el.counter++
	id := fmt.Sprintf("EMAIL-%03d", el.counter)
	email := Email{
		ID:       id,
		ThreadID: id,
		From:     from,
		To:       to,
		CC:       cc,
		Subject:  subject,
		Body:     body,
		Round:    round,
		Time:     time.Now(),
		Urgent:   urgent,
	}
	el.Emails = append(el.Emails, email)
	return id
}

// Reply creates a reply to an existing email. Returns the new email and an error
// if the parent doesn't exist or the caller wasn't a participant.
func (el *EmailLog) Reply(from, parentID, body string, round int, cc []string, urgent bool) (Email, error) {
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

	// Check caller was a participant (sender, recipient, or CC)
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
		for _, r := range parent.CC {
			if r == from {
				isParticipant = true
				break
			}
		}
	}
	if !isParticipant {
		return Email{}, fmt.Errorf("you are not a participant in email %q", parentID)
	}

	// Build recipients: all original participants minus self (reply-all)
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

	// Merge CC: parent CC + new CC, minus self and To recipients
	ccSet := make(map[string]bool)
	for _, r := range parent.CC {
		ccSet[r] = true
	}
	for _, r := range cc {
		ccSet[r] = true
	}
	delete(ccSet, from)
	for _, r := range to {
		delete(ccSet, r)
	}
	var mergedCC []string
	for r := range ccSet {
		mergedCC = append(mergedCC, r)
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
		CC:        mergedCC,
		Subject:   subject,
		Body:      body,
		Round:     round,
		Time:      time.Now(),
		InReplyTo: parentID,
		Urgent:    urgent,
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
		// Check if agent is a recipient (To or CC)
		isRecipient := false
		for _, r := range e.To {
			if r == agentName {
				isRecipient = true
				break
			}
		}
		if !isRecipient {
			for _, r := range e.CC {
				if r == agentName {
					isRecipient = true
					break
				}
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
		subjectLine := e.Subject
		if e.Urgent {
			subjectLine = "[URGENT] " + subjectLine
		}
		sb.WriteString(fmt.Sprintf("## %s: %s\n\n", e.ID, subjectLine))
		sb.WriteString(fmt.Sprintf("**From:** %s\n", e.From))
		sb.WriteString(fmt.Sprintf("**To:** %s\n", strings.Join(e.To, ", ")))
		if len(e.CC) > 0 {
			sb.WriteString(fmt.Sprintf("**CC:** %s\n", strings.Join(e.CC, ", ")))
		}
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

// --- Relationship types ---

// ScoreChange records a single change to a relationship score.
type ScoreChange struct {
	OldScore int       `json:"old_score"`
	NewScore int       `json:"new_score"`
	Reason   string    `json:"reason"`
	Round    int       `json:"round"`
	Time     time.Time `json:"time"`
}

// RelationshipScore tracks the relationship between two agents.
type RelationshipScore struct {
	FromAgent   string        `json:"from_agent"`
	ToAgent     string        `json:"to_agent"`
	Score       int           `json:"score"`
	LastUpdated time.Time     `json:"last_updated"`
	History     []ScoreChange `json:"history"`
}

// RelationshipLog holds all relationship scores with thread-safe access.
type RelationshipLog struct {
	mu     sync.Mutex
	Scores map[string]map[string]*RelationshipScore // from -> to -> score
}

// NewRelationshipLog creates an empty relationship log.
func NewRelationshipLog() *RelationshipLog {
	return &RelationshipLog{
		Scores: make(map[string]map[string]*RelationshipScore),
	}
}

// GetScore returns the score from one agent to another. Default is 50.
func (rl *RelationshipLog) GetScore(from, to string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if m, ok := rl.Scores[from]; ok {
		if rs, ok := m[to]; ok {
			return rs.Score
		}
	}
	return 50
}

// AdjustScore changes the score from one agent to another by delta, clamped to [-100, +100].
func (rl *RelationshipLog) AdjustScore(from, to string, delta int, reason string, round int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if _, ok := rl.Scores[from]; !ok {
		rl.Scores[from] = make(map[string]*RelationshipScore)
	}
	rs, ok := rl.Scores[from][to]
	if !ok {
		rs = &RelationshipScore{
			FromAgent: from,
			ToAgent:   to,
			Score:     50,
		}
		rl.Scores[from][to] = rs
	}

	oldScore := rs.Score
	newScore := oldScore + delta
	if newScore > 100 {
		newScore = 100
	}
	if newScore < -100 {
		newScore = -100
	}
	rs.Score = newScore
	rs.LastUpdated = time.Now()
	rs.History = append(rs.History, ScoreChange{
		OldScore: oldScore,
		NewScore: newScore,
		Reason:   reason,
		Round:    round,
		Time:     time.Now(),
	})
}

// GetAllScores returns all scores for a given agent.
func (rl *RelationshipLog) GetAllScores(from string) map[string]int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	result := make(map[string]int)
	if m, ok := rl.Scores[from]; ok {
		for to, rs := range m {
			result[to] = rs.Score
		}
	}
	return result
}

// RenderForAgent produces a markdown representation of an agent's relationships.
func (rl *RelationshipLog) RenderForAgent(agent string) string {
	scores := rl.GetAllScores(agent)
	if len(scores) == 0 {
		return ""
	}

	// Sort by agent name for deterministic output
	names := make([]string, 0, len(scores))
	for name := range scores {
		names = append(names, name)
	}
	sort.Strings(names)

	var sb strings.Builder
	sb.WriteString("## Your Relationship Scores\n\n")
	for _, name := range names {
		score := scores[name]
		tier := relationshipTier(score)
		sb.WriteString(fmt.Sprintf("- **%s**: %d/100 (%s)\n", name, score, tier))
	}
	sb.WriteString("\nYour relationship scores influence your tone and cooperation level. " +
		"Be authentic — lower scores mean less patience and willingness to help.\n")
	return sb.String()
}

// relationshipTier returns a human-readable description for a score.
func relationshipTier(score int) string {
	switch {
	case score >= 70:
		return "very positive — collaborative, extra mile"
	case score >= 40:
		return "neutral — professional"
	case score >= 10:
		return "strained — curt, minimal help"
	default:
		return "hostile — dismissive, uncooperative"
	}
}

// --- Escalation types ---

// Escalation represents a complaint filed about another agent.
type Escalation struct {
	ID         string    `json:"id"`
	FromAgent  string    `json:"from_agent"`
	AboutAgent string    `json:"about_agent"`
	ToManager  string    `json:"to_manager"`
	Reason     string    `json:"reason"`
	Evidence   string    `json:"evidence"`
	Round      int       `json:"round"`
	Time       time.Time `json:"time"`
	Status     string    `json:"status"`     // pending, acknowledged, dismissed, action_taken
	Resolution string    `json:"resolution"` // manager's response
}

// --- PiP types ---

// PiPRecord represents a Performance Improvement Plan entry.
type PiPRecord struct {
	ID           string    `json:"id"`
	TargetAgent  string    `json:"target_agent"`
	RecordedBy   string    `json:"recorded_by"`
	Reason       string    `json:"reason"`
	Expectations string    `json:"expectations"`
	ReviewRound  int       `json:"review_round"`
	Status       string    `json:"status"` // active, completed, failed, canceled
	Round        int       `json:"round"`
	Time         time.Time `json:"time"`
}

// PiPLog holds all PiP records with thread-safe access.
type PiPLog struct {
	mu      sync.Mutex
	Records []PiPRecord `json:"records"`
	counter int
}

// NewPiPLog creates an empty PiP log.
func NewPiPLog() *PiPLog {
	return &PiPLog{}
}

// Add creates a new PiP record and returns its ID.
func (pl *PiPLog) Add(targetAgent, recordedBy, reason, expectations string, reviewRound, round int) string {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	pl.counter++
	id := fmt.Sprintf("PIP-%03d", pl.counter)
	pl.Records = append(pl.Records, PiPRecord{
		ID:           id,
		TargetAgent:  targetAgent,
		RecordedBy:   recordedBy,
		Reason:       reason,
		Expectations: expectations,
		ReviewRound:  reviewRound,
		Status:       "active",
		Round:        round,
		Time:         time.Now(),
	})
	return id
}

// Render produces a markdown representation of all PiP records.
func (pl *PiPLog) Render() string {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("# Performance Improvement Plans (PiP)\n\n")
	if len(pl.Records) == 0 {
		sb.WriteString("No PiP records.\n")
		return sb.String()
	}
	for _, p := range pl.Records {
		sb.WriteString(fmt.Sprintf("## %s\n\n", p.ID))
		sb.WriteString(fmt.Sprintf("**Target:** %s\n", p.TargetAgent))
		sb.WriteString(fmt.Sprintf("**Recorded by:** %s\n", p.RecordedBy))
		sb.WriteString(fmt.Sprintf("**Reason:** %s\n", p.Reason))
		sb.WriteString(fmt.Sprintf("**Expectations:** %s\n", p.Expectations))
		if p.ReviewRound > 0 {
			sb.WriteString(fmt.Sprintf("**Review round:** %d\n", p.ReviewRound))
		}
		sb.WriteString(fmt.Sprintf("**Status:** %s\n", p.Status))
		sb.WriteString(fmt.Sprintf("**Created in round:** %d\n\n---\n\n", p.Round))
	}
	return sb.String()
}

// HasActivePiP returns true if the given agent has an active PiP.
func (pl *PiPLog) HasActivePiP(agent string) bool {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	for _, p := range pl.Records {
		if p.TargetAgent == agent && p.Status == "active" {
			return true
		}
	}
	return false
}

// EscalationLog holds all escalations with thread-safe access.
type EscalationLog struct {
	mu          sync.Mutex
	Escalations []Escalation `json:"escalations"`
	counter     int
}

// NewEscalationLog creates an empty escalation log.
func NewEscalationLog() *EscalationLog {
	return &EscalationLog{}
}

// Add creates a new escalation and returns its ID.
func (el *EscalationLog) Add(fromAgent, aboutAgent, toManager, reason, evidence string, round int) string {
	el.mu.Lock()
	defer el.mu.Unlock()
	el.counter++
	id := fmt.Sprintf("ESC-%03d", el.counter)
	el.Escalations = append(el.Escalations, Escalation{
		ID:         id,
		FromAgent:  fromAgent,
		AboutAgent: aboutAgent,
		ToManager:  toManager,
		Reason:     reason,
		Evidence:   evidence,
		Round:      round,
		Time:       time.Now(),
		Status:     "pending",
	})
	return id
}

// UpdateStatus updates the status and resolution of an escalation.
func (el *EscalationLog) UpdateStatus(id, status, resolution string) error {
	el.mu.Lock()
	defer el.mu.Unlock()
	for i := range el.Escalations {
		if el.Escalations[i].ID == id {
			el.Escalations[i].Status = status
			el.Escalations[i].Resolution = resolution
			return nil
		}
	}
	return fmt.Errorf("escalation %q not found", id)
}

// GetPendingFor returns pending escalations assigned to a given manager.
func (el *EscalationLog) GetPendingFor(manager string) []Escalation {
	el.mu.Lock()
	defer el.mu.Unlock()
	var result []Escalation
	for _, e := range el.Escalations {
		if e.ToManager == manager && e.Status == "pending" {
			result = append(result, e)
		}
	}
	return result
}

// GetAllFor returns all escalations assigned to a given manager.
func (el *EscalationLog) GetAllFor(manager string) []Escalation {
	el.mu.Lock()
	defer el.mu.Unlock()
	var result []Escalation
	for _, e := range el.Escalations {
		if e.ToManager == manager {
			result = append(result, e)
		}
	}
	return result
}

// CountAbout returns the number of escalations filed about a specific agent.
func (el *EscalationLog) CountAbout(agent string) int {
	el.mu.Lock()
	defer el.mu.Unlock()
	count := 0
	for _, e := range el.Escalations {
		if e.AboutAgent == agent {
			count++
		}
	}
	return count
}

// CountActionTakenAbout returns the number of escalations about an agent with status "action_taken".
func (el *EscalationLog) CountActionTakenAbout(agent string) int {
	el.mu.Lock()
	defer el.mu.Unlock()
	count := 0
	for _, e := range el.Escalations {
		if e.AboutAgent == agent && e.Status == "action_taken" {
			count++
		}
	}
	return count
}

// GetByID returns an escalation by its ID.
func (el *EscalationLog) GetByID(id string) (Escalation, bool) {
	el.mu.Lock()
	defer el.mu.Unlock()
	for _, e := range el.Escalations {
		if e.ID == id {
			return e, true
		}
	}
	return Escalation{}, false
}

// Render produces a markdown representation of all escalations.
func (el *EscalationLog) Render() string {
	el.mu.Lock()
	defer el.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("# Escalations\n\n")
	if len(el.Escalations) == 0 {
		sb.WriteString("No escalations filed.\n")
		return sb.String()
	}
	for _, e := range el.Escalations {
		sb.WriteString(fmt.Sprintf("## %s\n\n", e.ID))
		sb.WriteString(fmt.Sprintf("**Filed by:** %s\n", e.FromAgent))
		sb.WriteString(fmt.Sprintf("**About:** %s\n", e.AboutAgent))
		sb.WriteString(fmt.Sprintf("**To manager:** %s\n", e.ToManager))
		sb.WriteString(fmt.Sprintf("**Reason:** %s\n", e.Reason))
		if e.Evidence != "" {
			sb.WriteString(fmt.Sprintf("**Evidence:** %s\n", e.Evidence))
		}
		sb.WriteString(fmt.Sprintf("**Status:** %s\n", e.Status))
		if e.Resolution != "" {
			sb.WriteString(fmt.Sprintf("**Resolution:** %s\n", e.Resolution))
		}
		sb.WriteString(fmt.Sprintf("**Round:** %d\n\n---\n\n", e.Round))
	}
	return sb.String()
}

// --- Firing types ---

// FiringRecord represents a request to fire an agent.
type FiringRecord struct {
	ID          string    `json:"id"`
	TargetAgent string    `json:"target_agent"`
	RequestedBy string    `json:"requested_by"`
	Reason      string    `json:"reason"`
	CEOApproval string    `json:"ceo_approval"` // pending, approved, denied
	CEOComments string    `json:"ceo_comments"`
	Round       int       `json:"round"`
	Time        time.Time `json:"time"`
}

// FiringLog holds all firing requests with thread-safe access.
type FiringLog struct {
	mu       sync.Mutex
	Requests []FiringRecord `json:"requests"`
	counter  int
	Fired    map[string]bool `json:"fired"`
}

// NewFiringLog creates an empty firing log.
func NewFiringLog() *FiringLog {
	return &FiringLog{
		Fired: make(map[string]bool),
	}
}

// RequestFire creates a new firing request and returns its ID.
func (fl *FiringLog) RequestFire(targetAgent, requestedBy, reason string, round int) string {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.counter++
	id := fmt.Sprintf("FIRE-%03d", fl.counter)
	fl.Requests = append(fl.Requests, FiringRecord{
		ID:          id,
		TargetAgent: targetAgent,
		RequestedBy: requestedBy,
		Reason:      reason,
		CEOApproval: "pending",
		Round:       round,
		Time:        time.Now(),
	})
	return id
}

// CEODecision records the CEO's decision on a firing request.
func (fl *FiringLog) CEODecision(id, decision, comments string) error {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	for i := range fl.Requests {
		if fl.Requests[i].ID == id {
			fl.Requests[i].CEOApproval = decision
			fl.Requests[i].CEOComments = comments
			if decision == "approved" {
				fl.Fired[fl.Requests[i].TargetAgent] = true
			}
			return nil
		}
	}
	return fmt.Errorf("firing request %q not found", id)
}

// IsFired returns true if an agent has been fired.
func (fl *FiringLog) IsFired(agent string) bool {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	return fl.Fired[agent]
}

// GetPendingApprovals returns all pending firing requests.
func (fl *FiringLog) GetPendingApprovals() []FiringRecord {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	var result []FiringRecord
	for _, r := range fl.Requests {
		if r.CEOApproval == "pending" {
			result = append(result, r)
		}
	}
	return result
}

// Render produces a markdown representation of all firing requests.
func (fl *FiringLog) Render() string {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("# Firing Requests\n\n")
	if len(fl.Requests) == 0 {
		sb.WriteString("No firing requests.\n")
		return sb.String()
	}
	for _, r := range fl.Requests {
		sb.WriteString(fmt.Sprintf("## %s\n\n", r.ID))
		sb.WriteString(fmt.Sprintf("**Target:** %s\n", r.TargetAgent))
		sb.WriteString(fmt.Sprintf("**Requested by:** %s\n", r.RequestedBy))
		sb.WriteString(fmt.Sprintf("**Reason:** %s\n", r.Reason))
		sb.WriteString(fmt.Sprintf("**CEO Decision:** %s\n", r.CEOApproval))
		if r.CEOComments != "" {
			sb.WriteString(fmt.Sprintf("**CEO Comments:** %s\n", r.CEOComments))
		}
		sb.WriteString(fmt.Sprintf("**Round:** %d\n\n---\n\n", r.Round))
	}
	return sb.String()
}

// --- Code review types ---

// CodeComment represents an inline comment on a specific file and line.
type CodeComment struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Severity string `json:"severity"` // error, warning, suggestion, nit
	Comment  string `json:"comment"`
}

// CodeReview represents a structured code review session.
type CodeReview struct {
	ID       string        `json:"id"`
	TaskID   string        `json:"task_id"`
	Reviewer string        `json:"reviewer"`
	Round    int           `json:"round"`
	Verdict  string        `json:"verdict"` // "", approved, needs_changes (empty = pending)
	Summary  string        `json:"summary"`
	Comments []CodeComment `json:"comments"`
}

// CodeReviewLog holds all code reviews with thread-safe access.
type CodeReviewLog struct {
	mu      sync.Mutex
	Reviews []CodeReview `json:"reviews"`
	counter int
}

// NewCodeReviewLog creates an empty code review log.
func NewCodeReviewLog() *CodeReviewLog {
	return &CodeReviewLog{}
}

// Add creates a new code review and returns its ID.
func (cl *CodeReviewLog) Add(taskID, reviewer, summary string, round int) string {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.counter++
	id := fmt.Sprintf("CR-%03d", cl.counter)
	cl.Reviews = append(cl.Reviews, CodeReview{
		ID:       id,
		TaskID:   taskID,
		Reviewer: reviewer,
		Round:    round,
		Summary:  summary,
	})
	return id
}

// GetByID returns a pointer to the code review with the given ID, or nil.
func (cl *CodeReviewLog) GetByID(id string) *CodeReview {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	for i := range cl.Reviews {
		if cl.Reviews[i].ID == id {
			return &cl.Reviews[i]
		}
	}
	return nil
}

// GetByTaskID returns all reviews for a given task ID.
func (cl *CodeReviewLog) GetByTaskID(taskID string) []CodeReview {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	var result []CodeReview
	for _, r := range cl.Reviews {
		if r.TaskID == taskID {
			result = append(result, r)
		}
	}
	return result
}

// ReviewRoundForTask returns the next review round number for a given task.
func (cl *CodeReviewLog) ReviewRoundForTask(taskID string) int {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	maxRound := 0
	for _, r := range cl.Reviews {
		if r.TaskID == taskID {
			maxRound++
		}
	}
	return maxRound
}

// Render produces a markdown representation of reviews for a given task.
func (cl *CodeReviewLog) Render(taskID string) string {
	reviews := cl.GetByTaskID(taskID)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Code Reviews for %s\n\n", taskID))
	if len(reviews) == 0 {
		sb.WriteString("No reviews yet.\n")
		return sb.String()
	}
	for _, r := range reviews {
		sb.WriteString(fmt.Sprintf("## %s (Reviewer: %s, Round: %d)\n\n", r.ID, r.Reviewer, r.Round))
		verdict := r.Verdict
		if verdict == "" {
			verdict = "pending"
		}
		sb.WriteString(fmt.Sprintf("**Verdict:** %s\n\n", verdict))
		sb.WriteString(fmt.Sprintf("**Summary:** %s\n\n", r.Summary))
		if len(r.Comments) > 0 {
			sb.WriteString("### Comments\n\n")
			for _, c := range r.Comments {
				sb.WriteString(fmt.Sprintf("- **%s:%d** [%s]: %s\n", c.File, c.Line, c.Severity, c.Comment))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("---\n\n")
	}
	return sb.String()
}

// RenderWithSource renders a review with inline source context from the workspace.
func (cl *CodeReviewLog) RenderWithSource(review CodeReview, workspaceRoot string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Code Review %s for %s\n\n", review.ID, review.TaskID))
	sb.WriteString(fmt.Sprintf("**Reviewer:** %s\n", review.Reviewer))
	sb.WriteString(fmt.Sprintf("**Round:** %d\n", review.Round))
	verdict := review.Verdict
	if verdict == "" {
		verdict = "pending"
	}
	sb.WriteString(fmt.Sprintf("**Verdict:** %s\n\n", verdict))
	sb.WriteString(fmt.Sprintf("**Summary:** %s\n\n", review.Summary))

	if len(review.Comments) > 0 {
		sb.WriteString("## Inline Comments\n\n")
		for _, c := range review.Comments {
			sb.WriteString(fmt.Sprintf("### %s:%d [%s]\n\n", c.File, c.Line, strings.ToUpper(c.Severity)))
			// Try to show the referenced source line
			if workspaceRoot != "" {
				fullPath := workspaceRoot + string(os.PathSeparator) + c.File
				data, err := os.ReadFile(fullPath)
				if err == nil {
					lines := strings.Split(string(data), "\n")
					if c.Line > 0 && c.Line <= len(lines) {
						start := c.Line - 2
						if start < 0 {
							start = 0
						}
						end := c.Line + 1
						if end > len(lines) {
							end = len(lines)
						}
						sb.WriteString("```\n")
						for i := start; i < end; i++ {
							marker := "  "
							if i == c.Line-1 {
								marker = "> "
							}
							sb.WriteString(fmt.Sprintf("%s%d: %s\n", marker, i+1, lines[i]))
						}
						sb.WriteString("```\n\n")
					}
				}
			}
			sb.WriteString(fmt.Sprintf("**Comment:** %s\n\n", c.Comment))
		}
	}
	return sb.String()
}

// --- File snapshot types ---

// FileSnapshot stores a snapshot of a file's content at a point in time.
type FileSnapshot struct {
	Path    string `json:"path"`
	TaskID  string `json:"task_id"`
	Content string `json:"content"`
	Round   int    `json:"round"`
}

// FileSnapshotLog holds file snapshots with thread-safe access.
type FileSnapshotLog struct {
	mu        sync.Mutex
	Snapshots []FileSnapshot `json:"snapshots"`
}

// NewFileSnapshotLog creates an empty file snapshot log.
func NewFileSnapshotLog() *FileSnapshotLog {
	return &FileSnapshotLog{}
}

// Save stores a file snapshot.
func (fl *FileSnapshotLog) Save(path, taskID, content string, round int) {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.Snapshots = append(fl.Snapshots, FileSnapshot{
		Path:    path,
		TaskID:  taskID,
		Content: content,
		Round:   round,
	})
}

// GetLatest returns the most recent snapshot for a given path and optional task ID.
func (fl *FileSnapshotLog) GetLatest(path, taskID string) *FileSnapshot {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	var latest *FileSnapshot
	for i := range fl.Snapshots {
		s := &fl.Snapshots[i]
		if s.Path != path {
			continue
		}
		if taskID != "" && s.TaskID != taskID {
			continue
		}
		latest = s
	}
	return latest
}

// --- Command execution types ---

// CommandResult records a command execution.
type CommandResult struct {
	Command  string `json:"command"`
	Agent    string `json:"agent"`
	Round    int    `json:"round"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output"`
	TimedOut bool   `json:"timed_out"`
}

// CommandLog holds command execution results with thread-safe access.
type CommandLog struct {
	mu       sync.Mutex
	Commands []CommandResult `json:"commands"`
}

// NewCommandLog creates an empty command log.
func NewCommandLog() *CommandLog {
	return &CommandLog{}
}

// Add records a command result.
func (cl *CommandLog) Add(result CommandResult) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.Commands = append(cl.Commands, result)
}

// Render produces a markdown representation of command execution history.
func (cl *CommandLog) Render() string {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("# Command Log\n\n")
	if len(cl.Commands) == 0 {
		sb.WriteString("No commands executed.\n")
		return sb.String()
	}
	for i, c := range cl.Commands {
		sb.WriteString(fmt.Sprintf("## Command #%d (Round %d, Agent: %s)\n\n", i+1, c.Round, c.Agent))
		sb.WriteString(fmt.Sprintf("```\n$ %s\n", c.Command))
		if c.Output != "" {
			sb.WriteString(c.Output)
			if !strings.HasSuffix(c.Output, "\n") {
				sb.WriteString("\n")
			}
		}
		sb.WriteString("```\n\n")
		sb.WriteString(fmt.Sprintf("Exit code: %d", c.ExitCode))
		if c.TimedOut {
			sb.WriteString(" (timed out)")
		}
		sb.WriteString("\n\n---\n\n")
	}
	return sb.String()
}
