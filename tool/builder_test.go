package tool

import (
	"context"
	"testing"

	"github.com/dntatme/agents/llm"
)

func TestBuilderBasic(t *testing.T) {
	ft := Func("greet", "Say hello.").
		StringParam("name", "Who to greet.", true).
		Handler(func(_ context.Context, args map[string]any, _ map[string]any) (map[string]any, error) {
			return map[string]any{"greeting": "hello " + args["name"].(string)}, nil
		}).
		Build()

	def := ft.Definition()
	if def.Name != "greet" {
		t.Errorf("expected name 'greet', got %q", def.Name)
	}
	if def.Description != "Say hello." {
		t.Errorf("expected description 'Say hello.', got %q", def.Description)
	}
	if def.Parameters == nil {
		t.Fatal("expected parameters to be set")
	}
	if def.Parameters.Type != llm.TypeObject {
		t.Errorf("expected TypeObject, got %v", def.Parameters.Type)
	}
	nameProp := def.Parameters.Properties["name"]
	if nameProp == nil {
		t.Fatal("expected 'name' property")
	}
	if nameProp.Type != llm.TypeString {
		t.Errorf("expected TypeString, got %v", nameProp.Type)
	}
	if len(def.Parameters.Required) != 1 || def.Parameters.Required[0] != "name" {
		t.Errorf("expected required=[name], got %v", def.Parameters.Required)
	}

	result, err := ft.Execute(context.Background(), map[string]any{"name": "world"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["greeting"] != "hello world" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestBuilderRequiredVsOptional(t *testing.T) {
	ft := Func("test", "Test tool.").
		StringParam("required_param", "Required.", true).
		StringParam("optional_param", "Optional.", false).
		Handler(func(_ context.Context, _ map[string]any, _ map[string]any) (map[string]any, error) {
			return nil, nil
		}).
		Build()

	def := ft.Definition()
	if len(def.Parameters.Properties) != 2 {
		t.Errorf("expected 2 properties, got %d", len(def.Parameters.Properties))
	}
	if len(def.Parameters.Required) != 1 {
		t.Errorf("expected 1 required, got %d", len(def.Parameters.Required))
	}
	if def.Parameters.Required[0] != "required_param" {
		t.Errorf("expected required_param, got %s", def.Parameters.Required[0])
	}
}

func TestBuilderEnumParam(t *testing.T) {
	ft := Func("pick", "Pick a color.").
		StringEnumParam("color", "The color.", []string{"red", "green", "blue"}, true).
		Handler(func(_ context.Context, _ map[string]any, _ map[string]any) (map[string]any, error) {
			return nil, nil
		}).
		Build()

	def := ft.Definition()
	colorProp := def.Parameters.Properties["color"]
	if colorProp == nil {
		t.Fatal("expected 'color' property")
	}
	if len(colorProp.Enum) != 3 {
		t.Errorf("expected 3 enum values, got %d", len(colorProp.Enum))
	}
	if colorProp.Enum[0] != "red" || colorProp.Enum[1] != "green" || colorProp.Enum[2] != "blue" {
		t.Errorf("unexpected enum values: %v", colorProp.Enum)
	}
}

func TestBuilderNoParams(t *testing.T) {
	ft := Func("list", "List items.").
		NoParams().
		Handler(func(_ context.Context, _ map[string]any, _ map[string]any) (map[string]any, error) {
			return map[string]any{"items": []string{}}, nil
		}).
		Build()

	def := ft.Definition()
	if def.Parameters == nil {
		t.Fatal("expected parameters to be set for NoParams")
	}
	if def.Parameters.Type != llm.TypeObject {
		t.Errorf("expected TypeObject, got %v", def.Parameters.Type)
	}
	if len(def.Parameters.Properties) != 0 {
		t.Errorf("expected empty properties, got %d", len(def.Parameters.Properties))
	}
}

func TestBuilderAllParamTypes(t *testing.T) {
	ft := Func("multi", "Multi param tool.").
		StringParam("s", "string", true).
		NumberParam("n", "number", true).
		IntParam("i", "integer", false).
		BoolParam("b", "boolean", false).
		Handler(func(_ context.Context, _ map[string]any, _ map[string]any) (map[string]any, error) {
			return nil, nil
		}).
		Build()

	def := ft.Definition()
	if def.Parameters.Properties["s"].Type != llm.TypeString {
		t.Error("expected TypeString for 's'")
	}
	if def.Parameters.Properties["n"].Type != llm.TypeNumber {
		t.Error("expected TypeNumber for 'n'")
	}
	if def.Parameters.Properties["i"].Type != llm.TypeInteger {
		t.Error("expected TypeInteger for 'i'")
	}
	if def.Parameters.Properties["b"].Type != llm.TypeBoolean {
		t.Error("expected TypeBoolean for 'b'")
	}
	if len(def.Parameters.Required) != 2 {
		t.Errorf("expected 2 required params, got %d", len(def.Parameters.Required))
	}
}

func TestBuilderPanicsOnNilHandler(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on nil handler")
		}
	}()

	Func("bad", "No handler.").
		StringParam("x", "param", true).
		Build()
}
