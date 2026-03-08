package company

import (
	"context"
	"strings"
	"testing"
)

func setupEscalationState(t *testing.T) (string, map[string]any) {
	t.Helper()
	root := t.TempDir()
	if err := InitWorkspace(root); err != nil {
		t.Fatalf("InitWorkspace: %v", err)
	}
	oh := NewOrgHierarchy()
	oh.SetManager("backend-dev", "architect")
	oh.SetManager("architect", "cto")
	oh.SetManager("cto", "ceo")

	state := map[string]any{
		KeyWorkspaceRoot: root,
		KeyCurrentRound:  3,
		KeyOrgHierarchy:  oh,
		KeyFiredAgents:   map[string]bool{},
	}
	return root, state
}

func fileAndRespondEscalation(t *testing.T, state map[string]any, filer, about, responder string) map[string]any {
	t.Helper()
	ctx := context.Background()

	// File escalation
	state[KeyCurrentAgent] = filer
	fileTool := FileEscalationTool()
	result, err := fileTool.Execute(ctx, map[string]any{
		"about_agent": about,
		"reason":      "Poor performance",
		"evidence":    "Missed deadline",
	}, state)
	if err != nil {
		t.Fatalf("file escalation: %v", err)
	}
	escID := result["escalation_id"].(string)

	// Respond with action_taken
	state[KeyCurrentAgent] = responder
	respondTool := RespondToEscalationTool()
	result, err = respondTool.Execute(ctx, map[string]any{
		"escalation_id": escID,
		"status":        "action_taken",
		"resolution":    "Formal warning issued.",
	}, state)
	if err != nil {
		t.Fatalf("respond escalation: %v", err)
	}
	return result
}

func TestEscalation_AutoPiPOnSecondActionTaken(t *testing.T) {
	_, state := setupEscalationState(t)

	// First escalation — no auto-action
	result := fileAndRespondEscalation(t, state, "architect", "backend-dev", "cto")
	if _, ok := result["auto_action"]; ok {
		t.Error("first escalation should not trigger auto-action")
	}

	// Second escalation — should auto-generate PiP
	result = fileAndRespondEscalation(t, state, "architect", "backend-dev", "cto")
	autoAction, ok := result["auto_action"].(string)
	if !ok {
		t.Fatal("expected auto_action on second action_taken escalation")
	}
	if !strings.Contains(autoAction, "PiP") {
		t.Errorf("expected PiP auto-action, got: %s", autoAction)
	}

	// Verify PiP was actually created
	pl := GetPiPLog(state)
	if !pl.HasActivePiP("backend-dev") {
		t.Error("backend-dev should have an active PiP")
	}
}

func TestEscalation_AutoFireOnThirdWhileOnPiP(t *testing.T) {
	_, state := setupEscalationState(t)

	// First two escalations → PiP
	fileAndRespondEscalation(t, state, "architect", "backend-dev", "cto")
	fileAndRespondEscalation(t, state, "architect", "backend-dev", "cto")

	// Verify PiP exists
	pl := GetPiPLog(state)
	if !pl.HasActivePiP("backend-dev") {
		t.Fatal("backend-dev should have PiP after 2 escalations")
	}

	// Third escalation while on PiP → should auto-fire
	result := fileAndRespondEscalation(t, state, "architect", "backend-dev", "cto")
	autoAction, ok := result["auto_action"].(string)
	if !ok {
		t.Fatal("expected auto_action on third escalation while on PiP")
	}
	if !strings.Contains(autoAction, "Firing request") {
		t.Errorf("expected firing request auto-action, got: %s", autoAction)
	}

	// Verify firing request was created (sent to CEO since cto is not CEO)
	fl := GetFiringLog(state)
	pending := fl.GetPendingApprovals()
	if len(pending) == 0 {
		t.Error("expected pending firing request for backend-dev")
	}
}

func TestEscalation_NoPiPIfDismissed(t *testing.T) {
	_, state := setupEscalationState(t)
	ctx := context.Background()

	// File two escalations but dismiss them (not action_taken)
	for i := 0; i < 2; i++ {
		state[KeyCurrentAgent] = "architect"
		fileTool := FileEscalationTool()
		result, _ := fileTool.Execute(ctx, map[string]any{
			"about_agent": "backend-dev",
			"reason":      "Minor issue",
			"evidence":    "Not serious",
		}, state)
		escID := result["escalation_id"].(string)

		state[KeyCurrentAgent] = "cto"
		respondTool := RespondToEscalationTool()
		result, _ = respondTool.Execute(ctx, map[string]any{
			"escalation_id": escID,
			"status":        "dismissed",
			"resolution":    "Not actionable.",
		}, state)
		if _, ok := result["auto_action"]; ok {
			t.Error("dismissed escalations should not trigger auto-action")
		}
	}

	// Verify no PiP
	pl := GetPiPLog(state)
	if pl.HasActivePiP("backend-dev") {
		t.Error("dismissed escalations should not generate PiP")
	}
}

func TestEscalation_CountMethods(t *testing.T) {
	el := NewEscalationLog()
	el.Add("architect", "backend-dev", "cto", "Reason 1", "Evidence 1", 1)
	el.Add("architect", "backend-dev", "cto", "Reason 2", "Evidence 2", 2)
	el.Add("architect", "frontend-dev", "cto", "Reason 3", "Evidence 3", 2)

	if got := el.CountAbout("backend-dev"); got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
	if got := el.CountAbout("frontend-dev"); got != 1 {
		t.Errorf("expected 1, got %d", got)
	}

	// Mark one as action_taken
	el.UpdateStatus("ESC-001", "action_taken", "Warning issued")
	if got := el.CountActionTakenAbout("backend-dev"); got != 1 {
		t.Errorf("expected 1 action_taken, got %d", got)
	}
}
