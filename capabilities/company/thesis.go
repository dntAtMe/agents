package company

import (
	"fmt"
	"slices"
	"strings"
)

const (
	CompanyPhaseFounderDiscovery = "founder_discovery"
	CompanyPhaseExecution        = "execution"
)

// CompanyThesis is the persistent company-definition context created by the CEO
// before hiring begins.
type CompanyThesis struct {
	CompanyName       string   `json:"company_name,omitempty"`
	Purpose           string   `json:"purpose,omitempty"`
	Goal              string   `json:"goal,omitempty"`
	Values            []string `json:"values,omitempty"`
	Assumptions       []string `json:"assumptions,omitempty"`
	TargetUserProblem string   `json:"target_user_problem,omitempty"`
	StrategySummary   string   `json:"strategy_summary,omitempty"`
	Finalized         bool     `json:"finalized,omitempty"`
	FinalizedRound    int      `json:"finalized_round,omitempty"`
	LastUpdatedRound  int      `json:"last_updated_round,omitempty"`
}

// CompanyThesisUpdate is a partial update to the company thesis.
type CompanyThesisUpdate struct {
	CompanyName       string
	Purpose           string
	Goal              string
	Values            []string
	Assumptions       []string
	TargetUserProblem string
	StrategySummary   string
}

// NewCompanyThesis creates an empty thesis record.
func NewCompanyThesis() *CompanyThesis {
	return &CompanyThesis{}
}

// Apply merges non-empty fields from the update into the thesis.
func (t *CompanyThesis) Apply(update CompanyThesisUpdate, round int) {
	if v := strings.TrimSpace(update.CompanyName); v != "" {
		t.CompanyName = v
	}
	if v := strings.TrimSpace(update.Purpose); v != "" {
		t.Purpose = v
	}
	if v := strings.TrimSpace(update.Goal); v != "" {
		t.Goal = v
	}
	if len(update.Values) > 0 {
		t.Values = dedupeNonEmpty(update.Values)
	}
	if len(update.Assumptions) > 0 {
		t.Assumptions = dedupeNonEmpty(update.Assumptions)
	}
	if v := strings.TrimSpace(update.TargetUserProblem); v != "" {
		t.TargetUserProblem = v
	}
	if v := strings.TrimSpace(update.StrategySummary); v != "" {
		t.StrategySummary = v
	}
	t.LastUpdatedRound = round
}

// MissingRequiredFields returns the thesis fields required for early finalization.
func (t *CompanyThesis) MissingRequiredFields() []string {
	var missing []string
	if strings.TrimSpace(t.CompanyName) == "" {
		missing = append(missing, "company_name")
	}
	if strings.TrimSpace(t.Purpose) == "" {
		missing = append(missing, "purpose")
	}
	if strings.TrimSpace(t.Goal) == "" {
		missing = append(missing, "goal")
	}
	if len(t.Values) == 0 {
		missing = append(missing, "values")
	}
	if len(t.Assumptions) == 0 {
		missing = append(missing, "assumptions")
	}
	if strings.TrimSpace(t.TargetUserProblem) == "" {
		missing = append(missing, "target_user_problem")
	}
	if strings.TrimSpace(t.StrategySummary) == "" {
		missing = append(missing, "strategy_summary")
	}
	return missing
}

// HasMinimumViableThesis reports whether the thesis is complete enough to finalize early.
func (t *CompanyThesis) HasMinimumViableThesis() bool {
	return len(t.MissingRequiredFields()) == 0
}

// Finalize marks the thesis as finalized.
func (t *CompanyThesis) Finalize(round int) {
	t.Finalized = true
	t.FinalizedRound = round
	t.LastUpdatedRound = round
}

// Render returns the thesis as a markdown artifact.
func (t *CompanyThesis) Render() string {
	var sb strings.Builder

	sb.WriteString("# Company Thesis\n\n")
	if strings.TrimSpace(t.CompanyName) != "" {
		sb.WriteString(fmt.Sprintf("**Company Name:** %s\n\n", t.CompanyName))
	} else {
		sb.WriteString("**Company Name:** *Not decided yet.*\n\n")
	}

	writeSection := func(title, body string) {
		sb.WriteString(fmt.Sprintf("## %s\n\n", title))
		if strings.TrimSpace(body) == "" {
			sb.WriteString("*Not defined yet.*\n\n")
			return
		}
		sb.WriteString(strings.TrimSpace(body))
		sb.WriteString("\n\n")
	}

	writeList := func(title string, items []string) {
		sb.WriteString(fmt.Sprintf("## %s\n\n", title))
		if len(items) == 0 {
			sb.WriteString("*Not defined yet.*\n\n")
			return
		}
		for _, item := range dedupeNonEmpty(items) {
			sb.WriteString("- ")
			sb.WriteString(item)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	writeSection("Purpose", t.Purpose)
	writeSection("Goal", t.Goal)
	writeList("Values", t.Values)
	writeList("Assumptions", t.Assumptions)
	writeSection("Target User / Problem", t.TargetUserProblem)
	writeSection("Current Strategy", t.StrategySummary)

	sb.WriteString("## Status\n\n")
	if t.Finalized {
		sb.WriteString(fmt.Sprintf("- Finalized: yes\n- Finalized round: %d\n", t.FinalizedRound))
	} else {
		sb.WriteString("- Finalized: no\n")
	}
	if t.LastUpdatedRound > 0 {
		sb.WriteString(fmt.Sprintf("- Last updated round: %d\n", t.LastUpdatedRound))
	}
	sb.WriteString("\n")

	return sb.String()
}

// IsToolAllowedInCompanyPhase enforces founder-phase tool restrictions for the CEO.
func IsToolAllowedInCompanyPhase(state map[string]any, agentName, toolName string) bool {
	if agentName != "ceo" || !IsFounderDiscoveryPhase(state) {
		return true
	}
	return founderCEOAllowedTools[toolName]
}

// AdvanceFounderPhase auto-unlocks execution mode once the founder discovery
// round budget has been consumed.
func AdvanceFounderPhase(state map[string]any) bool {
	if !IsFounderDiscoveryPhase(state) {
		return false
	}
	if GetCurrentRound(state) < GetFounderMaxRounds(state) {
		return false
	}
	_ = ActivateExecutionMode(state, "Founder discovery round limit reached.")
	return true
}

// ActivateExecutionMode switches the company out of founder discovery.
func ActivateExecutionMode(state map[string]any, reason string) error {
	SetCompanyPhase(state, CompanyPhaseExecution)
	if fn, ok := state[KeyActivateExecutionMode].(func(string)); ok {
		fn(reason)
	}
	return nil
}

func dedupeNonEmpty(items []string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return slices.Clone(out)
}

var founderCEOAllowedTools = map[string]bool{
	"google_search":           true,
	"read_company_thesis":     true,
	"update_company_thesis":   true,
	"finalize_company_thesis": true,
	"read_file":               true,
	"write_file":              true,
	"list_files":              true,
	"post_update":             true,
	"read_updates":            true,
	"read_decisions":          true,
	"check_inbox":             true,
	"reply_email":             true,
	"write_diary":             true,
	"get_coffee":              true,
}
