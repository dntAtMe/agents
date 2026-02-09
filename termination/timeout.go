package termination

import "context"

// Timeout terminates when the context deadline is exceeded.
type Timeout struct{}

func (Timeout) ShouldTerminate(ctx context.Context, _ State) (bool, string) {
	select {
	case <-ctx.Done():
		return true, "context deadline exceeded"
	default:
		return false, ""
	}
}
