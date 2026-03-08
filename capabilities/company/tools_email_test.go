package company

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmailSendBasic(t *testing.T) {
	root, state := setupTestWorkspace(t)
	ctx := context.Background()

	state[KeyCurrentAgent] = "ceo"

	st := SendEmailTool()
	result, err := st.Execute(ctx, map[string]any{
		"to":      "cto",
		"subject": "Architecture review needed",
		"body":    "Please review the proposed architecture.",
	}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["email_id"] != "EMAIL-001" {
		t.Errorf("expected EMAIL-001, got %v", result["email_id"])
	}
	if result["status"] != "sent" {
		t.Errorf("expected sent, got %v", result["status"])
	}

	// Check inbox.md updated for recipient
	data, err := os.ReadFile(filepath.Join(root, "cto", "inbox.md"))
	if err != nil {
		t.Fatalf("read inbox: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Architecture review needed") {
		t.Error("inbox should contain email subject")
	}
	if !strings.Contains(content, "ceo") {
		t.Error("inbox should show sender")
	}
}

func TestEmailCheckInboxFilters(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	// Send multiple emails
	el := GetEmailLog(state)
	el.Send("ceo", []string{"cto"}, nil, "First email", "body1", 1, false)
	el.Send("architect", []string{"cto"}, nil, "Second email", "body2", 1, false)
	el.Send("ceo", []string{"backend-dev"}, nil, "Not for cto", "body3", 1, false)

	state[KeyCurrentAgent] = "cto"

	ci := CheckInboxTool()

	// Check all inbox
	result, err := ci.Execute(ctx, map[string]any{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["count"] != "2" {
		t.Errorf("expected 2 emails for cto, got %v", result["count"])
	}

	// Check with from filter — need to re-send since all were marked read
	el.Send("ceo", []string{"cto"}, nil, "Third from ceo", "body4", 2, false)
	el.Send("architect", []string{"cto"}, nil, "Third from architect", "body5", 2, false)

	result, err = ci.Execute(ctx, map[string]any{
		"from":        "ceo",
		"unread_only": true,
	}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["count"] != "1" {
		t.Errorf("expected 1 email from ceo (unread), got %v", result["count"])
	}
}

func TestEmailCheckInboxMarksRead(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	el := GetEmailLog(state)
	el.Send("ceo", []string{"cto"}, nil, "Test email", "body", 1, false)

	state[KeyCurrentAgent] = "cto"

	ci := CheckInboxTool()

	// First check — should return 1
	result, err := ci.Execute(ctx, map[string]any{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["count"] != "1" {
		t.Errorf("expected 1, got %v", result["count"])
	}

	// Second check with unread_only — should return 0
	result, err = ci.Execute(ctx, map[string]any{"unread_only": true}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["count"] != "0" {
		t.Errorf("expected 0 unread, got %v", result["count"])
	}
}

func TestEmailReplyBasic(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	// CEO sends to CTO
	state[KeyCurrentAgent] = "ceo"
	st := SendEmailTool()
	st.Execute(ctx, map[string]any{
		"to":      "cto",
		"subject": "Architecture question",
		"body":    "What stack should we use?",
	}, state)

	// CTO replies
	state[KeyCurrentAgent] = "cto"
	rt := ReplyEmailTool()
	result, err := rt.Execute(ctx, map[string]any{
		"email_id": "EMAIL-001",
		"body":     "I suggest Go with PostgreSQL.",
	}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "sent" {
		t.Errorf("expected sent, got %v", result["status"])
	}
	if result["thread_id"] != "EMAIL-001" {
		t.Errorf("expected thread_id EMAIL-001, got %v", result["thread_id"])
	}

	// Check that reply has Re: subject
	el := GetEmailLog(state)
	el.mu.Lock()
	reply := el.Emails[1]
	el.mu.Unlock()
	if reply.Subject != "Re: Architecture question" {
		t.Errorf("expected 'Re: Architecture question', got %q", reply.Subject)
	}

	// Check recipients: reply from cto should go to ceo
	if len(reply.To) != 1 || reply.To[0] != "ceo" {
		t.Errorf("expected reply to go to [ceo], got %v", reply.To)
	}
}

func TestEmailReplyNotParticipant(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	// CEO sends to CTO
	el := GetEmailLog(state)
	el.Send("ceo", []string{"cto"}, nil, "Private", "secret stuff", 1, false)

	// Architect tries to reply
	state[KeyCurrentAgent] = "architect"
	rt := ReplyEmailTool()
	result, err := rt.Execute(ctx, map[string]any{
		"email_id": "EMAIL-001",
		"body":     "I want in!",
	}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error when non-participant replies")
	}
}

func TestEmailSendToSelf(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	state[KeyCurrentAgent] = "ceo"

	st := SendEmailTool()
	result, err := st.Execute(ctx, map[string]any{
		"to":      "ceo",
		"subject": "Note to self",
		"body":    "Remember to...",
	}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error when sending to self")
	}
}

func TestEmailThreadIntegrity(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	el := GetEmailLog(state)

	// Email 1: CEO → CTO
	state[KeyCurrentAgent] = "ceo"
	st := SendEmailTool()
	st.Execute(ctx, map[string]any{
		"to":      "cto",
		"subject": "Thread test",
		"body":    "Starting a thread",
	}, state)

	// Email 2: CTO replies
	state[KeyCurrentAgent] = "cto"
	rt := ReplyEmailTool()
	rt.Execute(ctx, map[string]any{
		"email_id": "EMAIL-001",
		"body":     "Reply 1",
	}, state)

	// Email 3: CEO replies to the reply
	state[KeyCurrentAgent] = "ceo"
	result, err := rt.Execute(ctx, map[string]any{
		"email_id": "EMAIL-002",
		"body":     "Reply 2",
	}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All three should share the same thread_id
	el.mu.Lock()
	threadIDs := make(map[string]bool)
	for _, e := range el.Emails {
		threadIDs[e.ThreadID] = true
	}
	el.mu.Unlock()

	if len(threadIDs) != 1 {
		t.Errorf("expected 1 thread, got %d threads", len(threadIDs))
	}

	// The third email should have thread_id = EMAIL-001
	if result["thread_id"] != "EMAIL-001" {
		t.Errorf("expected thread_id EMAIL-001, got %v", result["thread_id"])
	}
}

func TestEmailSendWithCC(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	state[KeyCurrentAgent] = "ceo"

	st := SendEmailTool()
	result, err := st.Execute(ctx, map[string]any{
		"to":      "cto",
		"subject": "Architecture review",
		"body":    "Please review.",
		"cc":      "architect, project-manager",
	}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "sent" {
		t.Errorf("expected sent, got %v", result["status"])
	}

	el := GetEmailLog(state)

	// CC recipient should see the email in their inbox
	state[KeyCurrentAgent] = "architect"
	ci := CheckInboxTool()
	inboxResult, err := ci.Execute(ctx, map[string]any{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inboxResult["count"] != "1" {
		t.Errorf("CC recipient architect should see 1 email, got %v", inboxResult["count"])
	}

	// project-manager should also see it
	state[KeyCurrentAgent] = "project-manager"
	inboxResult, err = ci.Execute(ctx, map[string]any{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inboxResult["count"] != "1" {
		t.Errorf("CC recipient project-manager should see 1 email, got %v", inboxResult["count"])
	}

	// Verify CC field stored on email
	el.mu.Lock()
	email := el.Emails[0]
	el.mu.Unlock()
	if len(email.CC) != 2 {
		t.Errorf("expected 2 CC recipients, got %d", len(email.CC))
	}
}

func TestEmailUrgentFlag(t *testing.T) {
	_, state := setupTestWorkspace(t)

	el := GetEmailLog(state)
	el.Send("ceo", []string{"cto"}, nil, "Urgent matter", "Fix now!", 1, true)

	el.mu.Lock()
	email := el.Emails[0]
	el.mu.Unlock()

	if !email.Urgent {
		t.Error("expected email to be marked urgent")
	}

	// Check rendered inbox shows [URGENT]
	emails := el.Inbox("cto", false, "")
	rendered := el.RenderInbox(emails)
	if !strings.Contains(rendered, "[URGENT]") {
		t.Error("expected rendered inbox to contain [URGENT] prefix")
	}
}

func TestEmailCCRecipientCanReply(t *testing.T) {
	_, state := setupTestWorkspace(t)
	ctx := context.Background()

	el := GetEmailLog(state)
	// Send email with architect on CC
	el.Send("ceo", []string{"cto"}, []string{"architect"}, "Discussion", "Let's discuss.", 1, false)

	// Architect (CC'd) should be able to reply
	state[KeyCurrentAgent] = "architect"
	rt := ReplyEmailTool()
	result, err := rt.Execute(ctx, map[string]any{
		"email_id": "EMAIL-001",
		"body":     "I have thoughts on this.",
	}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result["error"]; ok {
		t.Errorf("CC recipient should be able to reply, got error: %v", result["error"])
	}
	if result["status"] != "sent" {
		t.Errorf("expected sent, got %v", result["status"])
	}
}
