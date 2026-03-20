package company

import (
	"context"
	"fmt"
	"strings"

	"github.com/dntatme/agents/agent"
	"github.com/dntatme/agents/llm"
	"github.com/dntatme/agents/tool"
)

// GoogleSearchTool returns a CEO research tool backed by Gemini Google Search grounding.
func GoogleSearchTool() tool.Tool {
	return tool.Func(
		"google_search",
		"Research the web with Google grounding. Use it during founder discovery to investigate markets, competitors, customer pain points, and emerging opportunities.",
	).
		StringParam("query", "The research question or search query to investigate.", true).
		StringParam("research_goal", "Optional context about what decision this research should inform.", false).
		Handler(func(ctx context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			query, _ := args["query"].(string)
			researchGoal, _ := args["research_goal"].(string)

			runtime, ok := GetSimRuntime(state).(*agent.SimRuntime)
			if !ok || runtime == nil || runtime.Provider == nil {
				return map[string]any{"error": "google_search is only available during simulation runtime."}, nil
			}

			searchProvider, ok := runtime.Provider.(llm.GoogleSearchProvider)
			if !ok {
				return map[string]any{"error": "google_search requires the Gemini provider."}, nil
			}

			prompt := strings.TrimSpace(query)
			if researchGoal != "" {
				prompt = fmt.Sprintf(
					"Research goal: %s\n\nQuestion: %s\n\nReturn concise founder-focused research notes grounded in current web information. Highlight customer pain points, market signals, notable competitors, and any obvious risks or constraints.",
					researchGoal,
					query,
				)
			}

			result, err := searchProvider.GoogleSearch(ctx, "", prompt)
			if err != nil {
				return map[string]any{"error": fmt.Sprintf("google_search failed: %v", err)}, nil
			}

			sources := make([]map[string]any, 0, len(result.Sources))
			for _, source := range result.Sources {
				sources = append(sources, map[string]any{
					"title":  source.Title,
					"url":    source.URL,
					"domain": source.Domain,
				})
			}

			return map[string]any{
				"query":          query,
				"research_goal":  researchGoal,
				"summary":        result.Summary,
				"sources":        sources,
				"source_count":   len(sources),
				"search_queries": result.SearchQueries,
			}, nil
		}).
		Build()
}

// ReadCompanyThesisTool returns a tool for inspecting the current company thesis.
func ReadCompanyThesisTool() tool.Tool {
	return tool.Func("read_company_thesis", "Read the current company thesis and founder context tracked by the CEO.").
		NoParams().
		Handler(func(_ context.Context, _ map[string]any, state map[string]any) (map[string]any, error) {
			thesis := GetCompanyThesis(state)
			return map[string]any{
				"company_name":        thesis.CompanyName,
				"purpose":             thesis.Purpose,
				"goal":                thesis.Goal,
				"values":              thesis.Values,
				"assumptions":         thesis.Assumptions,
				"target_user_problem": thesis.TargetUserProblem,
				"strategy_summary":    thesis.StrategySummary,
				"finalized":           thesis.Finalized,
				"phase":               GetCompanyPhase(state),
				"content":             thesis.Render(),
			}, nil
		}).
		Build()
}

// UpdateCompanyThesisTool returns a tool for incrementally updating the company thesis.
func UpdateCompanyThesisTool() tool.Tool {
	return tool.Func("update_company_thesis", "Update the structured company thesis and sync it to shared/company.md.").
		StringParam("company_name", "The startup or company name.", false).
		StringParam("purpose", "Why this company should exist.", false).
		StringParam("goal", "The concrete company goal or ambition.", false).
		StringParam("values", "Core values as a comma-separated or newline-separated list.", false).
		StringParam("assumptions", "Key assumptions as a comma-separated or newline-separated list.", false).
		StringParam("target_user_problem", "The target user and the problem the company solves.", false).
		StringParam("strategy_summary", "The current strategy summary for how the company will win.", false).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			thesis := GetCompanyThesis(state)
			round := GetCurrentRound(state)

			update := CompanyThesisUpdate{
				CompanyName:       readStringArg(args, "company_name"),
				Purpose:           readStringArg(args, "purpose"),
				Goal:              readStringArg(args, "goal"),
				Values:            parseListArg(readStringArg(args, "values")),
				Assumptions:       parseListArg(readStringArg(args, "assumptions")),
				TargetUserProblem: readStringArg(args, "target_user_problem"),
				StrategySummary:   readStringArg(args, "strategy_summary"),
			}
			thesis.Apply(update, round)

			root := GetWorkspaceRoot(state)
			if root != "" {
				_ = SyncCompanyThesis(root, thesis)
			}

			return map[string]any{
				"status":             "updated",
				"phase":              GetCompanyPhase(state),
				"company_name":       thesis.CompanyName,
				"missing_required":   thesis.MissingRequiredFields(),
				"content":            thesis.Render(),
				"finalized":          thesis.Finalized,
				"last_updated_round": thesis.LastUpdatedRound,
			}, nil
		}).
		Build()
}

// FinalizeCompanyThesisTool returns a tool for ending founder discovery early.
func FinalizeCompanyThesisTool() tool.Tool {
	return tool.Func("finalize_company_thesis", "Finalize the company thesis and unlock execution mode early once the startup direction is clear.").
		NoParams().
		Handler(func(_ context.Context, _ map[string]any, state map[string]any) (map[string]any, error) {
			thesis := GetCompanyThesis(state)
			missing := thesis.MissingRequiredFields()
			if len(missing) > 0 {
				return map[string]any{
					"error":            "company thesis is incomplete",
					"missing_required": missing,
					"content":          thesis.Render(),
				}, nil
			}

			round := GetCurrentRound(state)
			thesis.Finalize(round)
			if err := ActivateExecutionMode(state, "CEO finalized the company thesis early."); err != nil {
				return map[string]any{"error": err.Error()}, nil
			}

			root := GetWorkspaceRoot(state)
			if root != "" {
				_ = SyncCompanyThesis(root, thesis)
			}

			return map[string]any{
				"status":       "finalized",
				"phase":        GetCompanyPhase(state),
				"company_name": thesis.CompanyName,
				"content":      thesis.Render(),
			}, nil
		}).
		Build()
}

func readStringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func parseListArg(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == ',' || r == ';'
	})
	var out []string
	for _, field := range fields {
		field = strings.TrimSpace(field)
		field = strings.TrimPrefix(field, "-")
		field = strings.TrimPrefix(field, "*")
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return dedupeNonEmpty(out)
}
