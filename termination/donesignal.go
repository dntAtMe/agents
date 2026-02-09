package termination

import "context"

// DoneSignal terminates when the model response contains no tool calls.
type DoneSignal struct{}

func (DoneSignal) ShouldTerminate(_ context.Context, state State) (bool, string) {
	if !state.HasToolCalls {
		return true, "model produced no tool calls"
	}
	return false, ""
}
