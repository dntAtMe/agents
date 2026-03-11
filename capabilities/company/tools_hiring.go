package company

import (
	"context"
	"fmt"
	"strings"

	"github.com/dntatme/agents/tool"
)

// StartInterviewTool returns a tool for the CEO to interview a candidate for a position.
// It generates a random candidate, runs a 3-turn interview loop, and returns the transcript.
func StartInterviewTool() tool.Tool {
	positionEnum := make([]string, len(HirablePositions))
	copy(positionEnum, HirablePositions)

	return tool.Func("start_interview",
		"Start an interview with a randomly generated candidate for a position. "+
			"Runs a 3-turn interview: you ask a question, the candidate responds, repeated 3 times. "+
			"Returns the interview transcript and an interview_id for use with hire_decision.").
		StringEnumParam("position", "The position to interview for.", positionEnum, true).
		StringParam("job_description", "Optional job description or specific requirements for the role.", false).
		Handler(func(ctx context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			position, _ := args["position"].(string)
			jobDesc, _ := args["job_description"].(string)

			caller := GetCurrentAgent(state)
			round := GetCurrentRound(state)
			root := GetWorkspaceRoot(state)
			il := GetInterviewLog(state)

			// Only CEO can interview
			if caller != "ceo" {
				return map[string]any{
					"error": "Only the CEO can conduct interviews.",
				}, nil
			}

			// Check position validity
			if !IsHirablePosition(position) {
				return map[string]any{
					"error": fmt.Sprintf("Invalid position %q. Available: %s", position, strings.Join(HirablePositions, ", ")),
				}, nil
			}

			// Check if position is already filled
			if il.IsPositionFilled(position) {
				return map[string]any{
					"error": fmt.Sprintf("Position %q is already filled. Fire the current holder first.", position),
				}, nil
			}

			// Get sim_run_agent function
			runAgentFn, ok := state["sim_run_agent"].(func(ctx context.Context, targetName, message string, state map[string]any) (string, error))
			if !ok {
				return map[string]any{
					"error": "Interviews are not available outside of simulation.",
				}, nil
			}

			// Generate candidate
			candidateName := GenerateCandidateName()
			personality := RandomCandidatePersonality(position)
			background := GenerateCandidateBackground(candidateName, position)

			candidate := CandidateProfile{
				Name:        candidateName,
				Personality: personality,
				Position:    position,
			}

			interviewID := il.NextID()

			// Create temporary candidate agent via closure
			registerTemp, ok := state["register_temp_agent"].(func(name, systemPrompt string))
			if !ok {
				return map[string]any{
					"error": "Temporary agent registration not available.",
				}, nil
			}

			// Build candidate agent name (temporary)
			candidateAgentName := fmt.Sprintf("candidate-%s", interviewID)

			// Build candidate system prompt — personality shapes behavior but is NOT revealed
			candidatePrompt := fmt.Sprintf(
				"You are %s, a job candidate interviewing for the %s position.\n\n"+
					"Background: %s\n\n"+
					"**Your personality (this shapes how you behave, but do NOT explicitly state these traits):**\n"+
					"- Personality type: %s\n"+
					"- Motivation: %s\n"+
					"- Communication style: %s\n"+
					"- Work culture: %s\n\n"+
					"INTERVIEW RULES:\n"+
					"- Answer the interviewer's questions naturally and conversationally.\n"+
					"- Let your personality come through in HOW you answer, not by describing your traits.\n"+
					"- Be specific and give examples when possible.\n"+
					"- Keep responses concise (2-4 paragraphs max).\n"+
					"- Do NOT reveal your personality type or work ethic label.\n"+
					"- Do NOT use any tools — just respond conversationally.",
				candidateName, position, background,
				personality.Name, personality.Motivation,
				personality.CommunicationStyle, personality.WorkCulture,
			)

			if jobDesc != "" {
				candidatePrompt += fmt.Sprintf("\n\nThe job description states: %s", jobDesc)
			}

			// Register temporary candidate agent
			registerTemp(candidateAgentName, candidatePrompt)

			// Emit interview_start event to TUI
			if emitEvent, ok := state["emit_event"].(func(eventType, agent string, data map[string]any)); ok {
				emitEvent("interview_start", caller, map[string]any{
					"interview_id": interviewID,
					"candidate":    candidateName,
					"position":     position,
				})
			}

			// Set environment to interview
			SetAgentEnvironment(state, caller, EnvInterview)
			defer SetAgentEnvironment(state, caller, EnvOffice)

			// Run 3-turn interview
			var transcript []InterviewEntry
			var transcriptText strings.Builder
			transcriptText.WriteString(fmt.Sprintf("Interview for %s position with candidate %s\n\n", position, candidateName))

			for turn := 1; turn <= 3; turn++ {
				// CEO asks a question
				ceoPrompt := fmt.Sprintf(
					"[Interview %s — Turn %d/3]\n"+
						"You are interviewing %s for the %s position.\n"+
						"Background: %s\n\n"+
						"Transcript so far:\n%s\n\n"+
						"Ask your next interview question. Be thorough — probe for technical skills, "+
						"work ethic, collaboration style, and problem-solving ability. "+
						"Keep your question concise (1-2 paragraphs).",
					interviewID, turn, candidateName, position, background, transcriptText.String(),
				)

				state[KeyCurrentAgent] = caller
				ceoResponse, err := runAgentFn(ctx, caller, ceoPrompt, state)
				if err != nil {
					return map[string]any{
						"error": fmt.Sprintf("Interview error (CEO turn %d): %v", turn, err),
					}, nil
				}

				ceoEntry := InterviewEntry{Speaker: caller, Message: ceoResponse, Turn: turn}
				transcript = append(transcript, ceoEntry)
				transcriptText.WriteString(fmt.Sprintf("**%s (Turn %d):** %s\n\n", caller, turn, ceoResponse))

				// Candidate responds
				candidatePrompt := fmt.Sprintf(
					"[Interview %s — Turn %d/3]\n"+
						"The interviewer asked:\n%s\n\n"+
						"Full transcript so far:\n%s\n\n"+
						"Respond to the question as yourself. Stay in character.",
					interviewID, turn, ceoResponse, transcriptText.String(),
				)

				state[KeyCurrentAgent] = candidateAgentName
				candidateResponse, err := runAgentFn(ctx, candidateAgentName, candidatePrompt, state)
				state[KeyCurrentAgent] = caller
				if err != nil {
					return map[string]any{
						"error": fmt.Sprintf("Interview error (candidate turn %d): %v", turn, err),
					}, nil
				}

				candEntry := InterviewEntry{Speaker: candidateName, Message: candidateResponse, Turn: turn}
				transcript = append(transcript, candEntry)
				transcriptText.WriteString(fmt.Sprintf("**%s (Turn %d):** %s\n\n", candidateName, turn, candidateResponse))
			}

			// Save interview
			interview := Interview{
				ID:          interviewID,
				Position:    position,
				Interviewer: caller,
				Candidate:   candidate,
				Transcript:  transcript,
				Status:      InterviewComplete,
				Round:       round,
			}
			il.Save(interview)

			// Sync transcript to workspace
			if root != "" {
				_ = SyncInterviewTranscript(root, &interview)
			}

			// Emit interview_end event to TUI
			if emitEvent, ok := state["emit_event"].(func(eventType, agent string, data map[string]any)); ok {
				emitEvent("interview_end", caller, map[string]any{
					"interview_id": interviewID,
					"candidate":    candidateName,
					"position":     position,
				})
			}

			return map[string]any{
				"interview_id": interviewID,
				"candidate":    candidateName,
				"position":     position,
				"background":   background,
				"transcript":   transcriptText.String(),
				"status":       "complete",
				"next_step":    fmt.Sprintf("Use hire_decision with interview_id=%q to hire or pass on this candidate.", interviewID),
			}, nil
		}).
		Build()
}

// HireDecisionTool returns a tool for the CEO to hire or pass on an interviewed candidate.
func HireDecisionTool() tool.Tool {
	return tool.Func("hire_decision",
		"Make a hiring decision on a completed interview. "+
			"If hiring, the candidate becomes a permanent team member with full tools.").
		StringParam("interview_id", "The interview ID (e.g. 'INT-001').", true).
		StringEnumParam("decision", "Your decision.", []string{"hire", "pass"}, true).
		StringParam("reporting_to", "Who the new hire reports to (required if hiring). E.g. 'ceo', 'cto', 'architect'.", false).
		Handler(func(ctx context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			interviewID, _ := args["interview_id"].(string)
			decision, _ := args["decision"].(string)
			reportingTo, _ := args["reporting_to"].(string)

			caller := GetCurrentAgent(state)
			round := GetCurrentRound(state)
			root := GetWorkspaceRoot(state)
			il := GetInterviewLog(state)

			if caller != "ceo" {
				return map[string]any{"error": "Only the CEO can make hiring decisions."}, nil
			}

			interview := il.GetByID(interviewID)
			if interview == nil {
				return map[string]any{"error": fmt.Sprintf("Interview %q not found.", interviewID)}, nil
			}

			if interview.Status != InterviewComplete {
				return map[string]any{
					"error": fmt.Sprintf("Interview %q is not in 'complete' status (current: %s).", interviewID, interview.Status),
				}, nil
			}

			if decision == "pass" {
				il.SetStatus(interviewID, InterviewPassed)

				// Unregister temporary candidate agent
				candidateAgentName := fmt.Sprintf("candidate-%s", interviewID)
				if unregister, ok := state["unregister_agent"].(func(string)); ok {
					unregister(candidateAgentName)
				}

				// Emit candidate_passed event to TUI
				if emitEvent, ok := state["emit_event"].(func(eventType, agent string, data map[string]any)); ok {
					emitEvent("candidate_passed", caller, map[string]any{
						"interview_id": interviewID,
						"candidate":    interview.Candidate.Name,
						"position":     interview.Position,
					})
				}

				return map[string]any{
					"status":    "passed",
					"candidate": interview.Candidate.Name,
					"message":   fmt.Sprintf("Passed on %s for %s position.", interview.Candidate.Name, interview.Position),
				}, nil
			}

			// Hiring
			if reportingTo == "" {
				return map[string]any{"error": "reporting_to is required when hiring."}, nil
			}

			// Register hired agent via closure
			registerHired, ok := state["register_hired_agent"].(func(position string, personality *Personality, candidateName string, reportingTo string) error)
			if !ok {
				return map[string]any{"error": "Hired agent registration not available."}, nil
			}

			if err := registerHired(interview.Position, interview.Candidate.Personality, interview.Candidate.Name, reportingTo); err != nil {
				return map[string]any{"error": fmt.Sprintf("Failed to register hired agent: %v", err)}, nil
			}

			il.SetStatus(interviewID, InterviewHired)
			AddHiredAgent(state, interview.Position)

			// Unregister temporary candidate agent
			candidateAgentName := fmt.Sprintf("candidate-%s", interviewID)
			if unregister, ok := state["unregister_agent"].(func(string)); ok {
				unregister(candidateAgentName)
			}

			// Send welcome email
			el := GetEmailLog(state)
			el.Send("ceo", []string{interview.Position}, nil,
				fmt.Sprintf("Welcome to the team, %s!", interview.Candidate.Name),
				fmt.Sprintf("Congratulations! You have been hired as %s.\n\n"+
					"You report to %s. Check your inbox and the workspace to get started.\n\n"+
					"Welcome aboard!",
					interview.Position, reportingTo),
				round, false)

			if root != "" {
				_ = SyncInbox(root, el, interview.Position)
				_ = SyncInterviewTranscript(root, interview)
			}

			// Emit candidate_hired event to TUI
			if emitEvent, ok := state["emit_event"].(func(eventType, agent string, data map[string]any)); ok {
				emitEvent("candidate_hired", caller, map[string]any{
					"interview_id": interviewID,
					"candidate":    interview.Candidate.Name,
					"position":     interview.Position,
					"reporting_to": reportingTo,
				})
			}

			return map[string]any{
				"status":       "hired",
				"candidate":    interview.Candidate.Name,
				"position":     interview.Position,
				"reporting_to": reportingTo,
				"message":      fmt.Sprintf("%s has been hired as %s, reporting to %s.", interview.Candidate.Name, interview.Position, reportingTo),
			}, nil
		}).
		Build()
}
