package termination

import (
	"context"
	"fmt"
)

// TokenBudget terminates when cumulative token usage exceeds the limit.
type TokenBudget struct {
	MaxTokens int32
}

func (t TokenBudget) ShouldTerminate(_ context.Context, state State) (bool, string) {
	if state.TotalTokensUsed >= t.MaxTokens {
		return true, fmt.Sprintf("token budget exhausted (%d/%d)", state.TotalTokensUsed, t.MaxTokens)
	}
	return false, ""
}
