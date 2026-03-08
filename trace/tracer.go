// Package trace provides JSONL event tracing for agent simulations.
package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"google.golang.org/genai"

	"github.com/dntatme/agents/agent"
)

// Event represents a single trace event written as one JSONL line.
type Event struct {
	Timestamp string         `json:"ts"`
	Round     int            `json:"round"`
	Agent     string         `json:"agent,omitempty"`
	Type      string         `json:"type"`
	Data      map[string]any `json:"data,omitempty"`
}

// Tracer writes JSONL trace events to a file with mutex-protected writes.
type Tracer struct {
	mu      sync.Mutex
	f       *os.File
	enc     *json.Encoder
	round   int
	agent   string
	started map[string]time.Time // keyed by tool name for duration tracking
}

// New creates a new Tracer that writes to the given file path.
// The file is created or truncated if it already exists.
func New(path string) (*Tracer, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("trace: create %s: %w", path, err)
	}
	return &Tracer{
		f:       f,
		enc:     json.NewEncoder(f),
		started: make(map[string]time.Time),
	}, nil
}

// Close flushes and closes the trace file.
func (t *Tracer) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.f.Close()
}

func (t *Tracer) emit(ev Event) {
	ev.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	t.mu.Lock()
	defer t.mu.Unlock()
	_ = t.enc.Encode(ev)
}

// SimulationStart records the beginning of a simulation.
func (t *Tracer) SimulationStart(prompt string, maxRounds int, agents []string) {
	t.emit(Event{
		Type: "simulation_start",
		Data: map[string]any{
			"prompt":     truncateStr(prompt, 200),
			"max_rounds": maxRounds,
			"agents":     agents,
		},
	})
}

// SimulationEnd records the end of a simulation.
func (t *Tracer) SimulationEnd(totalRounds int, reason string) {
	t.emit(Event{
		Type: "simulation_end",
		Data: map[string]any{
			"total_rounds": totalRounds,
			"reason":       reason,
		},
	})
}

// RoundStart records the beginning of a simulation round.
func (t *Tracer) RoundStart(round int) {
	t.round = round
	t.emit(Event{
		Round: round,
		Type:  "round_start",
	})
}

// RoundEnd records the end of a simulation round.
func (t *Tracer) RoundEnd(round int, allIdle bool) {
	t.emit(Event{
		Round: round,
		Type:  "round_end",
		Data: map[string]any{
			"all_idle": allIdle,
		},
	})
}

// AgentActivation records that an agent is about to run.
func (t *Tracer) AgentActivation(round int, agentName string) {
	t.round = round
	t.agent = agentName
	t.emit(Event{
		Round: round,
		Agent: agentName,
		Type:  "agent_activation",
	})
}

// AgentCompletion records the result of an agent's run.
func (t *Tracer) AgentCompletion(round int, agentName string, result *agent.RunResult, idle bool) {
	data := map[string]any{
		"idle": idle,
	}
	if result != nil {
		data["output_preview"] = truncateStr(result.FinalText, 150)
		data["tokens"] = result.TotalTokens
		data["iterations"] = result.Iterations
	}
	t.emit(Event{
		Round: round,
		Agent: agentName,
		Type:  "agent_completion",
		Data:  data,
	})
}

// Hooks returns agent hooks that trace tool calls with timing information.
func (t *Tracer) Hooks() *agent.Hooks {
	return &agent.Hooks{
		BeforeToolCall: func(ctx context.Context, hc *agent.HookContext, fc *genai.FunctionCall) error {
			t.mu.Lock()
			t.started[fc.Name] = time.Now()
			t.mu.Unlock()

			t.emit(Event{
				Round: t.round,
				Agent: t.agent,
				Type:  "tool_call_start",
				Data: map[string]any{
					"tool": fc.Name,
					"args": truncateStr(fmt.Sprintf("%v", fc.Args), 200),
				},
			})
			return nil
		},
		AfterToolCall: func(ctx context.Context, hc *agent.HookContext, fc *genai.FunctionCall, result map[string]any) error {
			var durationMs int64
			t.mu.Lock()
			if start, ok := t.started[fc.Name]; ok {
				durationMs = time.Since(start).Milliseconds()
				delete(t.started, fc.Name)
			}
			t.mu.Unlock()

			t.emit(Event{
				Round: t.round,
				Agent: t.agent,
				Type:  "tool_call_end",
				Data: map[string]any{
					"tool":        fc.Name,
					"result":      truncateStr(fmt.Sprintf("%v", result), 300),
					"duration_ms": durationMs,
				},
			})
			return nil
		},
	}
}

func truncateStr(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
