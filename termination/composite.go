package termination

import "context"

// Any terminates when any one of its policies fires (OR combinator).
type Any struct {
	Policies []Policy
}

func (a Any) ShouldTerminate(ctx context.Context, state State) (bool, string) {
	for _, p := range a.Policies {
		if ok, reason := p.ShouldTerminate(ctx, state); ok {
			return true, reason
		}
	}
	return false, ""
}

// All terminates only when every policy agrees (AND combinator).
type All struct {
	Policies []Policy
}

func (a All) ShouldTerminate(ctx context.Context, state State) (bool, string) {
	for _, p := range a.Policies {
		if ok, _ := p.ShouldTerminate(ctx, state); !ok {
			return false, ""
		}
	}
	return true, "all termination policies agreed"
}
