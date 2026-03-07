package tool

import (
	"context"
	"testing"

	"google.golang.org/genai"
)

func TestRegistryRegisterAndLookup(t *testing.T) {
	r := NewRegistry()

	ft := &FuncTool{
		Decl: &genai.FunctionDeclaration{Name: "test_tool", Description: "A test tool."},
		Fn: func(_ context.Context, _ map[string]any, _ map[string]any) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		},
	}
	r.Register(ft)

	if r.Len() != 1 {
		t.Errorf("expected 1 tool, got %d", r.Len())
	}

	got := r.Lookup("test_tool")
	if got == nil {
		t.Fatal("expected to find test_tool")
	}
	if got.Definition().Name != "test_tool" {
		t.Errorf("unexpected name: %s", got.Definition().Name)
	}

	if r.Lookup("missing") != nil {
		t.Error("expected nil for missing tool")
	}
}

func TestRegistryDefinitions(t *testing.T) {
	r := NewRegistry()
	r.Register(&FuncTool{
		Decl: &genai.FunctionDeclaration{Name: "a"},
		Fn:   func(_ context.Context, _ map[string]any, _ map[string]any) (map[string]any, error) { return nil, nil },
	})
	r.Register(&FuncTool{
		Decl: &genai.FunctionDeclaration{Name: "b"},
		Fn:   func(_ context.Context, _ map[string]any, _ map[string]any) (map[string]any, error) { return nil, nil },
	})

	defs := r.Definitions()
	if len(defs) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(defs))
	}
	if defs[0].Name != "a" || defs[1].Name != "b" {
		t.Error("definitions not in insertion order")
	}
}

func TestRegistryDuplicatePanics(t *testing.T) {
	r := NewRegistry()
	ft := &FuncTool{
		Decl: &genai.FunctionDeclaration{Name: "dup"},
		Fn:   func(_ context.Context, _ map[string]any, _ map[string]any) (map[string]any, error) { return nil, nil },
	}
	r.Register(ft)

	defer func() {
		if recover() == nil {
			t.Error("expected panic on duplicate registration")
		}
	}()
	r.Register(ft)
}

func TestFuncToolExecute(t *testing.T) {
	ft := &FuncTool{
		Decl: &genai.FunctionDeclaration{Name: "add"},
		Fn: func(_ context.Context, args map[string]any, _ map[string]any) (map[string]any, error) {
			a, _ := args["a"].(float64)
			b, _ := args["b"].(float64)
			return map[string]any{"result": a + b}, nil
		},
	}

	result, err := ft.Execute(context.Background(), map[string]any{"a": float64(2), "b": float64(3)}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["result"] != float64(5) {
		t.Errorf("expected 5, got %v", result["result"])
	}
}

func TestTransferToolDefinition(t *testing.T) {
	tt := NewTransferTool([]string{"weather", "support"})
	def := tt.Definition()

	if def.Name != TransferToolName {
		t.Errorf("expected %s, got %s", TransferToolName, def.Name)
	}

	agentProp := def.Parameters.Properties["agent_name"]
	if agentProp == nil {
		t.Fatal("expected agent_name property")
	}
	if len(agentProp.Enum) != 2 {
		t.Errorf("expected 2 enum values, got %d", len(agentProp.Enum))
	}
}
