package termination

import (
	"context"
	"testing"
	"time"
)

func TestDoneSignal(t *testing.T) {
	p := DoneSignal{}

	ok, _ := p.ShouldTerminate(context.Background(), State{HasToolCalls: true})
	if ok {
		t.Error("should not terminate when tool calls present")
	}

	ok, reason := p.ShouldTerminate(context.Background(), State{HasToolCalls: false})
	if !ok {
		t.Error("should terminate when no tool calls")
	}
	if reason == "" {
		t.Error("reason should not be empty")
	}
}

func TestMaxIterations(t *testing.T) {
	p := MaxIterations{Max: 5}

	ok, _ := p.ShouldTerminate(context.Background(), State{Iteration: 3})
	if ok {
		t.Error("should not terminate at iteration 3/5")
	}

	ok, _ = p.ShouldTerminate(context.Background(), State{Iteration: 5})
	if !ok {
		t.Error("should terminate at iteration 5/5")
	}

	ok, _ = p.ShouldTerminate(context.Background(), State{Iteration: 10})
	if !ok {
		t.Error("should terminate at iteration 10/5")
	}
}

func TestTimeout(t *testing.T) {
	p := Timeout{}

	ok, _ := p.ShouldTerminate(context.Background(), State{})
	if ok {
		t.Error("should not terminate with no deadline")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond)

	ok, _ = p.ShouldTerminate(ctx, State{})
	if !ok {
		t.Error("should terminate when context is done")
	}
}

func TestTokenBudget(t *testing.T) {
	p := TokenBudget{MaxTokens: 1000}

	ok, _ := p.ShouldTerminate(context.Background(), State{TotalTokensUsed: 500})
	if ok {
		t.Error("should not terminate under budget")
	}

	ok, _ = p.ShouldTerminate(context.Background(), State{TotalTokensUsed: 1000})
	if !ok {
		t.Error("should terminate at budget")
	}
}

func TestAny(t *testing.T) {
	p := Any{Policies: []Policy{
		MaxIterations{Max: 10},
		TokenBudget{MaxTokens: 500},
	}}

	// Neither fires.
	ok, _ := p.ShouldTerminate(context.Background(), State{Iteration: 1, TotalTokensUsed: 100})
	if ok {
		t.Error("should not terminate when no policy fires")
	}

	// Token budget fires.
	ok, _ = p.ShouldTerminate(context.Background(), State{Iteration: 1, TotalTokensUsed: 600})
	if !ok {
		t.Error("should terminate when token budget fires")
	}
}

func TestAll(t *testing.T) {
	p := All{Policies: []Policy{
		MaxIterations{Max: 5},
		TokenBudget{MaxTokens: 500},
	}}

	// Only one fires.
	ok, _ := p.ShouldTerminate(context.Background(), State{Iteration: 10, TotalTokensUsed: 100})
	if ok {
		t.Error("should not terminate when only one policy fires")
	}

	// Both fire.
	ok, _ = p.ShouldTerminate(context.Background(), State{Iteration: 10, TotalTokensUsed: 600})
	if !ok {
		t.Error("should terminate when all policies fire")
	}
}
