package tool

import (
	"context"

	"google.golang.org/genai"
)

// Tool is the interface every agent tool must satisfy.
type Tool interface {
	Definition() *genai.FunctionDeclaration
	Execute(ctx context.Context, args map[string]any, state map[string]any) (map[string]any, error)
}

// FuncTool adapts a closure into a Tool.
type FuncTool struct {
	Decl *genai.FunctionDeclaration
	Fn   func(ctx context.Context, args map[string]any, state map[string]any) (map[string]any, error)
}

func (f *FuncTool) Definition() *genai.FunctionDeclaration { return f.Decl }

func (f *FuncTool) Execute(ctx context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
	return f.Fn(ctx, args, state)
}
