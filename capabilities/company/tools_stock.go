package company

import (
	"context"
	"fmt"

	"github.com/dntatme/agents/tool"
)

// UpdateStockPriceTool returns a tool for the shareholders agent to set the stock price.
func UpdateStockPriceTool() tool.Tool {
	return tool.Func("update_stock_price",
		"Update the company stock price based on your assessment of company performance. "+
			"Provide the new price, a sentiment label, and your reasoning.").
		NumberParam("new_price", "The new stock price (must be > 0).", true).
		StringParam("sentiment", "Market sentiment label, e.g. 'bullish', 'bearish', 'neutral', 'cautiously optimistic'.", true).
		StringParam("reasoning", "Brief explanation of why the stock price changed.", true).
		Handler(func(ctx context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			newPrice, _ := args["new_price"].(float64)
			sentiment, _ := args["sentiment"].(string)
			reasoning, _ := args["reasoning"].(string)

			if newPrice <= 0 {
				return map[string]any{"error": "new_price must be greater than 0"}, nil
			}
			if sentiment == "" {
				return map[string]any{"error": "sentiment is required"}, nil
			}
			if reasoning == "" {
				return map[string]any{"error": "reasoning is required"}, nil
			}

			round := GetCurrentRound(state)
			tracker := GetStockTracker(state)
			oldPrice := tracker.Current
			tracker.Update(round, newPrice, sentiment)

			delta := newPrice - oldPrice
			sign := ""
			if delta > 0 {
				sign = "+"
			}

			return map[string]any{
				"status":    "updated",
				"old_price": fmt.Sprintf("$%.2f", oldPrice),
				"new_price": fmt.Sprintf("$%.2f", newPrice),
				"delta":     fmt.Sprintf("%s%.2f", sign, delta),
				"sentiment": sentiment,
				"reasoning": reasoning,
			}, nil
		}).
		Build()
}

// CheckStockPriceTool returns a read-only tool for C-suite to check the stock price.
func CheckStockPriceTool() tool.Tool {
	return tool.Func("check_stock_price",
		"Check the current company stock price, recent history, and market sentiment.").
		NoParams().
		Handler(func(ctx context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			tracker := GetStockTracker(state)

			result := map[string]any{
				"current_price": fmt.Sprintf("$%.2f", tracker.Current),
				"sentiment":     tracker.Sentiment,
				"history":       tracker.Render(),
				"brief":         tracker.RenderBrief(),
			}

			if len(tracker.History) > 1 {
				last := tracker.History[len(tracker.History)-1]
				result["last_delta"] = fmt.Sprintf("%.2f", last.Delta)
			}

			return result, nil
		}).
		Build()
}
