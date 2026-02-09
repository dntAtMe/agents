package termination

import (
	"context"
	"fmt"
)

// MaxIterations terminates after N iterations.
type MaxIterations struct {
	Max int
}

func (m MaxIterations) ShouldTerminate(_ context.Context, state State) (bool, string) {
	if state.Iteration >= m.Max {
		return true, fmt.Sprintf("reached max iterations (%d)", m.Max)
	}
	return false, ""
}
