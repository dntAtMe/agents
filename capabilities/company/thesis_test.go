package company

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateCompanyThesisToolPersistsMarkdown(t *testing.T) {
	root, state := setupTestWorkspace(t)
	state[KeyCurrentAgent] = "ceo"

	tool := UpdateCompanyThesisTool()
	_, err := tool.Execute(context.Background(), map[string]any{
		"company_name":        "Signal Forge",
		"purpose":             "Help founders discover strong B2B workflow niches.",
		"goal":                "Launch a profitable SaaS business.",
		"values":              "speed, clarity, evidence",
		"assumptions":         "buyers pay for workflow automation, compliance teams need faster intake",
		"target_user_problem": "Compliance teams lose time triaging repetitive requests.",
		"strategy_summary":    "Start with one painful workflow and sell into regulated SMBs.",
	}, state)
	if err != nil {
		t.Fatalf("update_company_thesis: %v", err)
	}

	thesis := GetCompanyThesis(state)
	if thesis.CompanyName != "Signal Forge" {
		t.Fatalf("expected company name to be updated, got %q", thesis.CompanyName)
	}
	if len(thesis.Values) != 3 {
		t.Fatalf("expected 3 values, got %v", thesis.Values)
	}

	data, err := os.ReadFile(filepath.Join(root, "shared", "company.md"))
	if err != nil {
		t.Fatalf("read shared/company.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Signal Forge") || !strings.Contains(content, "Compliance teams lose time triaging repetitive requests.") {
		t.Fatalf("unexpected company.md contents:\n%s", content)
	}
}

func TestFinalizeCompanyThesisToolRequiresCompleteThesis(t *testing.T) {
	_, state := setupTestWorkspace(t)
	state[KeyCurrentAgent] = "ceo"
	state[KeyCompanyPhase] = CompanyPhaseFounderDiscovery

	tool := FinalizeCompanyThesisTool()
	result, err := tool.Execute(context.Background(), map[string]any{}, state)
	if err != nil {
		t.Fatalf("finalize_company_thesis: %v", err)
	}
	if result["error"] == nil {
		t.Fatalf("expected validation error, got %#v", result)
	}
	missing, _ := result["missing_required"].([]string)
	if len(missing) == 0 {
		t.Fatalf("expected missing fields, got %#v", result)
	}
	if GetCompanyPhase(state) != CompanyPhaseFounderDiscovery {
		t.Fatalf("phase should remain founder discovery, got %q", GetCompanyPhase(state))
	}
}

func TestAdvanceFounderPhaseActivatesExecutionMode(t *testing.T) {
	_, state := setupTestWorkspace(t)
	state[KeyCompanyPhase] = CompanyPhaseFounderDiscovery
	state[KeyFounderMaxRounds] = 10
	state[KeyCurrentRound] = 10

	called := false
	reason := ""
	state[KeyActivateExecutionMode] = func(r string) {
		called = true
		reason = r
	}

	advanced := AdvanceFounderPhase(state)
	if !advanced {
		t.Fatal("expected founder phase to auto-advance")
	}
	if !called {
		t.Fatal("expected execution activation callback to run")
	}
	if GetCompanyPhase(state) != CompanyPhaseExecution {
		t.Fatalf("expected execution phase, got %q", GetCompanyPhase(state))
	}
	if !strings.Contains(reason, "round limit") {
		t.Fatalf("unexpected activation reason: %q", reason)
	}
}

func TestIsToolAllowedInCompanyPhase(t *testing.T) {
	state := map[string]any{KeyCompanyPhase: CompanyPhaseFounderDiscovery}
	if !IsToolAllowedInCompanyPhase(state, "ceo", "google_search") {
		t.Fatal("google_search should be allowed in founder discovery")
	}
	if IsToolAllowedInCompanyPhase(state, "ceo", "start_interview") {
		t.Fatal("start_interview should be blocked in founder discovery")
	}
	if !IsToolAllowedInCompanyPhase(state, "cto", "write_file") {
		t.Fatal("non-CEO tools should not be restricted by founder phase helper")
	}
}
