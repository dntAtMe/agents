package company

import (
	"fmt"
	"sort"
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
