package company

import (
	"context"
	"fmt"
	"strings"

	"github.com/dntatme/agents/tool"
)

// GetCoffeeTool returns a tool that lets an agent opt into a between-rounds coffee break.
// The coffee break grants bonus AP next round.
func GetCoffeeTool() tool.Tool {
	return tool.Func("get_coffee",
		"Take a coffee break after this round. You'll chat casually with other agents who also "+
			"got coffee, and you'll get bonus action points next round (+5 AP). Costs 3 AP.").
		Handler(func(ctx context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			caller := GetCurrentAgent(state)
			tracker := GetActionPointTracker(state)

			if tracker == nil {
				return map[string]any{
					"error": "Action point system is not available.",
				}, nil
			}

			// Register for coffee break
			tracker.RegisterCoffee(caller)

			// List who else is getting coffee
			participants := tracker.CoffeeParticipants()
			var others []string
			for _, p := range participants {
				if p != caller {
					others = append(others, p)
				}
			}

			msg := fmt.Sprintf("You've signed up for the coffee break after this round. You'll get +5 bonus AP next round.")
			if len(others) > 0 {
				msg += fmt.Sprintf(" Others getting coffee so far: %s.", strings.Join(others, ", "))
			} else {
				msg += " No one else has signed up yet — hopefully someone joins you!"
			}

			return map[string]any{
				"status":  "registered",
				"message": msg,
			}, nil
		}).
		Build()
}
