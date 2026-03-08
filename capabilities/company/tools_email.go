package company

import (
	"context"
	"fmt"
	"strings"

	"github.com/dntatme/agents/tool"
)

// SendEmailTool returns a tool for sending emails to other agents.
func SendEmailTool() tool.Tool {
	return tool.Func("send_email",
		"Send an email to one or more agents. Use for async communication that doesn't need an immediate response.").
		StringParam("to", "Comma-separated list of recipient agent names (e.g. 'cto,architect').", true).
		StringParam("subject", "The email subject line.", true).
		StringParam("body", "The email body content.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			toRaw, _ := args["to"].(string)
			subject, _ := args["subject"].(string)
			body, _ := args["body"].(string)

			caller := GetCurrentAgent(state)
			round := GetCurrentRound(state)
			root := GetWorkspaceRoot(state)
			el := GetEmailLog(state)

			// Parse recipients
			var recipients []string
			for _, r := range strings.Split(toRaw, ",") {
				r = strings.TrimSpace(r)
				if r != "" {
					recipients = append(recipients, r)
				}
			}
			if len(recipients) == 0 {
				return map[string]any{"error": "No valid recipients specified."}, nil
			}

			// Guardrail: cannot send to yourself
			for _, r := range recipients {
				if r == caller {
					return map[string]any{"error": "You cannot send an email to yourself."}, nil
				}
			}

			emailID := el.Send(caller, recipients, subject, body, round)

			// Sync inbox.md for each recipient
			if root != "" {
				for _, r := range recipients {
					_ = SyncInbox(root, el, r)
				}
			}

			return map[string]any{
				"email_id": emailID,
				"status":   "sent",
			}, nil
		}).
		Build()
}

// CheckInboxTool returns a tool for checking an agent's email inbox.
func CheckInboxTool() tool.Tool {
	return tool.Func("check_inbox",
		"Check your email inbox. Returns emails sent to you, optionally filtered by read status or sender.").
		BoolParam("unread_only", "If true, only show unread emails.", false).
		StringParam("from", "Filter emails from a specific agent.", false).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			caller := GetCurrentAgent(state)
			el := GetEmailLog(state)

			unreadOnly := false
			if v, ok := args["unread_only"]; ok {
				if b, ok := v.(bool); ok {
					unreadOnly = b
				}
			}
			fromFilter, _ := args["from"].(string)

			emails := el.Inbox(caller, unreadOnly, fromFilter)
			rendered := el.RenderInbox(emails)

			// Mark returned emails as read
			el.MarkReadBatch(caller, emails)

			return map[string]any{
				"emails": rendered,
				"count":  fmt.Sprintf("%d", len(emails)),
			}, nil
		}).
		Build()
}

// ReplyEmailTool returns a tool for replying to an email.
func ReplyEmailTool() tool.Tool {
	return tool.Func("reply_email",
		"Reply to an email. The reply goes to all participants in the original email thread.").
		StringParam("email_id", "The ID of the email to reply to (e.g. 'EMAIL-001').", true).
		StringParam("body", "The reply body content.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			emailID, _ := args["email_id"].(string)
			body, _ := args["body"].(string)

			caller := GetCurrentAgent(state)
			round := GetCurrentRound(state)
			root := GetWorkspaceRoot(state)
			el := GetEmailLog(state)

			reply, err := el.Reply(caller, emailID, body, round)
			if err != nil {
				return map[string]any{"error": err.Error()}, nil
			}

			// Sync inbox.md for all recipients
			if root != "" {
				for _, r := range reply.To {
					_ = SyncInbox(root, el, r)
				}
			}

			return map[string]any{
				"email_id":  reply.ID,
				"thread_id": reply.ThreadID,
				"status":    "sent",
			}, nil
		}).
		Build()
}
