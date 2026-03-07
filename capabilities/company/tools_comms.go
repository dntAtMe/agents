package company

import (
	"context"
	"fmt"

	"github.com/dntatme/agents/tool"
)

// PostUpdateTool returns a tool for posting updates to the shared channel.
func PostUpdateTool() tool.Tool {
	return tool.Func("post_update", "Post an update message to the shared updates channel for all team members to see.").
		StringParam("message", "The update message to post.", true).
		StringEnumParam("channel", "Channel to post to. Defaults to 'general'.", []string{
			"general", "technical", "product", "reviews",
		}, false).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			message, _ := args["message"].(string)
			channel, _ := args["channel"].(string)

			agent := GetCurrentAgent(state)
			round := GetCurrentRound(state)
			ul := GetUpdateLog(state)

			ul.Post(round, agent, channel, message)

			// Sync to file
			root := GetWorkspaceRoot(state)
			if root != "" {
				_ = SyncUpdates(root, ul)
			}

			return map[string]any{"status": "posted", "channel": channel, "round": round}, nil
		}).
		Build()
}

// ReadUpdatesTool returns a tool for reading updates from the shared channel.
func ReadUpdatesTool() tool.Tool {
	return tool.Func("read_updates", "Read updates from the shared updates channel. Can filter by channel and recency.").
		StringEnumParam("channel", "Filter to a specific channel. Leave empty for all channels.", []string{
			"general", "technical", "product", "reviews",
		}, false).
		IntParam("since_round", "Only show updates from this round onwards. Use 0 or omit for all updates.", false).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			channel, _ := args["channel"].(string)
			sinceRound := 0
			if v, ok := args["since_round"]; ok {
				switch n := v.(type) {
				case float64:
					sinceRound = int(n)
				case int:
					sinceRound = n
				}
			}

			ul := GetUpdateLog(state)
			rendered := ul.Render(channel, sinceRound)

			count := len(ul.Read(channel, sinceRound))
			return map[string]any{
				"content": rendered,
				"count":   fmt.Sprintf("%d updates", count),
			}, nil
		}).
		Build()
}
