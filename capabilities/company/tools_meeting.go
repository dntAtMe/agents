package company

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dntatme/agents/tool"
)

// CallGroupMeetingTool returns a tool for calling a multi-agent group meeting.
// Only agents in allowedCallers can invoke it.
func CallGroupMeetingTool(allowedCallers []string) tool.Tool {
	allowed := make(map[string]bool, len(allowedCallers))
	for _, name := range allowedCallers {
		allowed[name] = true
	}

	return tool.Func("call_group_meeting",
		"Call a group meeting with multiple agents. Participants discuss the agenda in 2 rounds. "+
			"You are automatically included as a participant.").
		StringParam("attendees", "Comma-separated agent names to invite (e.g. 'cto,architect,backend-dev').", true).
		StringParam("agenda", "The meeting topic or agenda to discuss.", true).
		Handler(func(ctx context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			attendeesRaw, _ := args["attendees"].(string)
			agenda, _ := args["agenda"].(string)

			caller := GetCurrentAgent(state)
			round := GetCurrentRound(state)
			root := GetWorkspaceRoot(state)
			ml := GetMeetingLog(state)

			// Guardrail: caller must be in allowedCallers
			if !allowed[caller] {
				return map[string]any{
					"error": fmt.Sprintf("Agent %q is not authorized to call group meetings.", caller),
				}, nil
			}

			// Parse attendees
			var attendees []string
			for _, a := range strings.Split(attendeesRaw, ",") {
				a = strings.TrimSpace(a)
				if a != "" {
					attendees = append(attendees, a)
				}
			}
			if len(attendees) == 0 {
				return map[string]any{"error": "No attendees specified."}, nil
			}

			// Guardrail: cannot include self in attendees
			for _, a := range attendees {
				if a == caller {
					return map[string]any{
						"error": "You cannot include yourself in attendees — you are automatically included as the meeting caller.",
					}, nil
				}
			}

			// Get sim_run_agent function
			runAgentFn, ok := state["sim_run_agent"].(func(ctx context.Context, targetName, message string, state map[string]any) (string, error))
			if !ok {
				return map[string]any{
					"error": "Group meetings are not available outside of simulation.",
				}, nil
			}

			meetingID := ml.NextID()
			participants := append([]string{caller}, attendees...)

			var transcript []MeetingEntry
			var transcriptText strings.Builder

			// 2 rounds of discussion
			for r := 1; r <= 2; r++ {
				for _, participant := range participants {
					// Build prompt
					prompt := fmt.Sprintf(
						"[Group Meeting %s, Round %d/2]\n"+
							"Agenda: %s\n"+
							"Participants: %s\n"+
							"Transcript so far:\n%s\n\n"+
							"Speak concisely as yourself in this meeting. Address the agenda and respond to what others have said.",
						meetingID, r, agenda, strings.Join(participants, ", "), transcriptText.String(),
					)

					// Swap current_agent → participant
					state[KeyCurrentAgent] = participant
					response, err := runAgentFn(ctx, participant, prompt, state)
					state[KeyCurrentAgent] = caller

					if err != nil {
						// Record error in transcript, continue
						errMsg := fmt.Sprintf("[Error: could not reach %s: %v]", participant, err)
						entry := MeetingEntry{
							Speaker: participant,
							Round:   r,
							Message: errMsg,
						}
						transcript = append(transcript, entry)
						transcriptText.WriteString(fmt.Sprintf("%s: %s\n\n", participant, errMsg))
						continue
					}

					entry := MeetingEntry{
						Speaker: participant,
						Round:   r,
						Message: response,
					}
					transcript = append(transcript, entry)
					transcriptText.WriteString(fmt.Sprintf("%s: %s\n\n", participant, response))
				}
			}

			// Save meeting
			meeting := Meeting{
				ID:         meetingID,
				CalledBy:   caller,
				Agenda:     agenda,
				Attendees:  participants,
				Transcript: transcript,
				SimRound:   round,
				Time:       time.Now(),
			}
			ml.Save(meeting)

			// Write meeting notes to workspace
			if root != "" {
				_ = SyncMeetingNotes(root, ml, meeting)
			}

			// Build transcript string for return
			return map[string]any{
				"meeting_id": meetingID,
				"attendees":  strings.Join(participants, ", "),
				"transcript": transcriptText.String(),
				"status":     "completed",
			}, nil
		}).
		Build()
}
