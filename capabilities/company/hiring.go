package company

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// InterviewStatus tracks the state of an interview.
type InterviewStatus string

const (
	InterviewPending  InterviewStatus = "pending"
	InterviewComplete InterviewStatus = "complete"
	InterviewHired    InterviewStatus = "hired"
	InterviewPassed   InterviewStatus = "passed"
)

// CandidateProfile describes a generated candidate.
type CandidateProfile struct {
	Name        string
	Personality *Personality
	Background  string
	Position    string
}

// Interview tracks a single interview session.
type Interview struct {
	ID          string
	Position    string
	Interviewer string
	Candidate   CandidateProfile
	Transcript  []InterviewEntry
	Status      InterviewStatus
	Round       int
}

// InterviewEntry is a single turn in an interview conversation.
type InterviewEntry struct {
	Speaker string
	Message string
	Turn    int
}

// InterviewLog stores all interviews with thread-safe access.
type InterviewLog struct {
	mu         sync.Mutex
	Interviews []Interview
	counter    int
}

// NewInterviewLog creates an empty interview log.
func NewInterviewLog() *InterviewLog {
	return &InterviewLog{}
}

// NextID returns the next interview ID.
func (il *InterviewLog) NextID() string {
	il.mu.Lock()
	defer il.mu.Unlock()
	il.counter++
	return fmt.Sprintf("INT-%03d", il.counter)
}

// Save stores a completed interview.
func (il *InterviewLog) Save(interview Interview) {
	il.mu.Lock()
	defer il.mu.Unlock()
	// Update existing or append
	for i, existing := range il.Interviews {
		if existing.ID == interview.ID {
			il.Interviews[i] = interview
			return
		}
	}
	il.Interviews = append(il.Interviews, interview)
}

// GetByID returns an interview by ID, or nil if not found.
func (il *InterviewLog) GetByID(id string) *Interview {
	il.mu.Lock()
	defer il.mu.Unlock()
	for i := range il.Interviews {
		if il.Interviews[i].ID == id {
			return &il.Interviews[i]
		}
	}
	return nil
}

// SetStatus updates the status of an interview.
func (il *InterviewLog) SetStatus(id string, status InterviewStatus) {
	il.mu.Lock()
	defer il.mu.Unlock()
	for i := range il.Interviews {
		if il.Interviews[i].ID == id {
			il.Interviews[i].Status = status
			return
		}
	}
}

// IsPositionFilled checks if any interview for the given position has been hired.
func (il *InterviewLog) IsPositionFilled(position string) bool {
	il.mu.Lock()
	defer il.mu.Unlock()
	for _, interview := range il.Interviews {
		if interview.Position == position && interview.Status == InterviewHired {
			return true
		}
	}
	return false
}

// --- Candidate name generation ---

var firstNames = []string{
	"Alex", "Jordan", "Sam", "Morgan", "Casey", "Riley", "Quinn", "Avery",
	"Taylor", "Jamie", "Drew", "Skyler", "Blake", "Rowan", "Finley", "Sage",
	"Kai", "River", "Phoenix", "Emery", "Dana", "Pat", "Kim", "Lee",
	"Robin", "Ash", "Kit", "Remy", "Darcy", "Jules", "Noel", "Reese",
}

var lastNames = []string{
	"Chen", "Patel", "Kim", "Mueller", "Santos", "Okafor", "Petrov", "Tanaka",
	"Johansson", "Singh", "Martinez", "Dubois", "Kowalski", "Yamamoto", "Ali",
	"Andersen", "Nakamura", "Fernandez", "Novak", "Park", "Larsson", "Costa",
	"Weber", "Suzuki", "Reyes", "Fischer", "Gupta", "Moreau", "Takahashi", "Berg",
}

// GenerateCandidateName returns a random first+last name.
func GenerateCandidateName() string {
	first := firstNames[rand.Intn(len(firstNames))]
	last := lastNames[rand.Intn(len(lastNames))]
	return first + " " + last
}

// RandomCandidatePersonality picks a random personality from the full pool
// (including slackers and malicious) and sets the Role for the given position.
func RandomCandidatePersonality(position string) *Personality {
	all := Personalities() // includes hard-working, slacker, malicious
	p := all[rand.Intn(len(all))]
	p.Role = RoleFor(position)
	p.Skillset, p.Specializations = RollSkills(position)
	p.SkillBehavior = SkillBehaviorFor(p.WorkEthic)
	p.Background = GenerateBackground(position)
	return &p
}

// GenerateCandidateBackground generates a plausible background string.
func GenerateCandidateBackground(name, position string) string {
	experiences := []string{
		"5 years at a fast-growing startup",
		"previously worked at a Fortune 500 company",
		"self-taught with open-source contributions",
		"fresh out of a top CS program",
		"10 years of industry experience across multiple domains",
		"former freelancer turned full-time",
		"bootcamp graduate with 3 years experience",
		"PhD in Computer Science with industry internships",
		"career changer from finance/consulting",
		"remote-first worker with distributed team experience",
	}
	exp := experiences[rand.Intn(len(experiences))]
	return fmt.Sprintf("%s — %s, applying for %s", name, exp, position)
}

// SyncInterviewTranscript writes an interview transcript to workspace.
func SyncInterviewTranscript(root string, interview *Interview) error {
	dir := filepath.Join(root, "shared", "interviews")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Interview %s\n\n", interview.ID))
	sb.WriteString(fmt.Sprintf("**Position:** %s\n", interview.Position))
	sb.WriteString(fmt.Sprintf("**Candidate:** %s\n", interview.Candidate.Name))
	sb.WriteString(fmt.Sprintf("**Interviewer:** %s\n", interview.Interviewer))
	sb.WriteString(fmt.Sprintf("**Status:** %s\n", interview.Status))
	sb.WriteString(fmt.Sprintf("**Round:** %d\n\n", interview.Round))
	sb.WriteString("## Transcript\n\n")

	for _, entry := range interview.Transcript {
		sb.WriteString(fmt.Sprintf("**%s (Turn %d):** %s\n\n", entry.Speaker, entry.Turn, entry.Message))
	}

	filename := fmt.Sprintf("%s.md", interview.ID)
	return os.WriteFile(filepath.Join(dir, filename), []byte(sb.String()), 0o644)
}

// HirablePositions lists the positions that can be hired.
var HirablePositions = []string{
	"product-manager", "cto", "architect",
	"project-manager", "backend-dev", "frontend-dev", "devops",
}

// IsHirablePosition checks if the given position is valid for hiring.
func IsHirablePosition(position string) bool {
	for _, p := range HirablePositions {
		if p == position {
			return true
		}
	}
	return false
}
