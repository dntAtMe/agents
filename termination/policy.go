package termination

import (
	"context"

	"google.golang.org/genai"
)

// State holds the current iteration state for termination evaluation.
type State struct {
	Iteration       int
	TotalTokensUsed int32
	LastResponse    *genai.GenerateContentResponse
	HasToolCalls    bool
}

// Policy decides whether the ReACT loop should stop.
type Policy interface {
	ShouldTerminate(ctx context.Context, state State) (bool, string)
}
