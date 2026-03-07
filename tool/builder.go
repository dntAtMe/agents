package tool

import (
	"context"

	"google.golang.org/genai"
)

// Builder provides a fluent API for constructing FuncTool instances.
type Builder struct {
	name        string
	description string
	properties  map[string]*genai.Schema
	required    []string
	noParams    bool
	handler     func(ctx context.Context, args map[string]any, state map[string]any) (map[string]any, error)
}

// Func starts building a new tool with the given name and description.
func Func(name, description string) *Builder {
	return &Builder{
		name:        name,
		description: description,
		properties:  make(map[string]*genai.Schema),
	}
}

// StringParam adds a string parameter.
func (b *Builder) StringParam(name, desc string, required bool) *Builder {
	b.properties[name] = &genai.Schema{
		Type:        genai.TypeString,
		Description: desc,
	}
	if required {
		b.required = append(b.required, name)
	}
	return b
}

// StringEnumParam adds a string parameter constrained to specific values.
func (b *Builder) StringEnumParam(name, desc string, values []string, required bool) *Builder {
	b.properties[name] = &genai.Schema{
		Type:        genai.TypeString,
		Description: desc,
		Enum:        values,
	}
	if required {
		b.required = append(b.required, name)
	}
	return b
}

// NumberParam adds a number (float64) parameter.
func (b *Builder) NumberParam(name, desc string, required bool) *Builder {
	b.properties[name] = &genai.Schema{
		Type:        genai.TypeNumber,
		Description: desc,
	}
	if required {
		b.required = append(b.required, name)
	}
	return b
}

// IntParam adds an integer parameter.
func (b *Builder) IntParam(name, desc string, required bool) *Builder {
	b.properties[name] = &genai.Schema{
		Type:        genai.TypeInteger,
		Description: desc,
	}
	if required {
		b.required = append(b.required, name)
	}
	return b
}

// BoolParam adds a boolean parameter.
func (b *Builder) BoolParam(name, desc string, required bool) *Builder {
	b.properties[name] = &genai.Schema{
		Type:        genai.TypeBoolean,
		Description: desc,
	}
	if required {
		b.required = append(b.required, name)
	}
	return b
}

// NoParams marks the tool as having no parameters (empty object schema).
func (b *Builder) NoParams() *Builder {
	b.noParams = true
	return b
}

// Handler sets the function that executes the tool.
func (b *Builder) Handler(fn func(ctx context.Context, args map[string]any, state map[string]any) (map[string]any, error)) *Builder {
	b.handler = fn
	return b
}

// Build constructs the FuncTool. Panics if no handler is set.
func (b *Builder) Build() *FuncTool {
	if b.handler == nil {
		panic("tool.Builder: handler is required")
	}

	decl := &genai.FunctionDeclaration{
		Name:        b.name,
		Description: b.description,
	}

	if b.noParams {
		decl.Parameters = &genai.Schema{
			Type:       genai.TypeObject,
			Properties: map[string]*genai.Schema{},
		}
	} else if len(b.properties) > 0 {
		decl.Parameters = &genai.Schema{
			Type:       genai.TypeObject,
			Properties: b.properties,
			Required:   b.required,
		}
	}

	return &FuncTool{
		Decl: decl,
		Fn:   b.handler,
	}
}
