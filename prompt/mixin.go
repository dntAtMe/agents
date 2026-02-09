// Package prompt provides a mixin system for composing system prompts
// from reusable, named sections.
package prompt

import "strings"

// Mixin is a named, reusable section of a system prompt.
type Mixin struct {
	// Name identifies this mixin (used as the section header).
	Name string
	// Content is the prompt text for this section.
	Content string
}

// Builder composes a system prompt from an ordered list of mixins.
type Builder struct {
	mixins []Mixin
}

// NewBuilder creates an empty prompt builder.
func NewBuilder() *Builder {
	return &Builder{}
}

// Add appends a mixin to the builder. Returns the builder for chaining.
func (b *Builder) Add(m Mixin) *Builder {
	b.mixins = append(b.mixins, m)
	return b
}

// AddSection is shorthand for Add(Mixin{Name: name, Content: content}).
func (b *Builder) AddSection(name, content string) *Builder {
	return b.Add(Mixin{Name: name, Content: content})
}

// Build renders the final system prompt. Each mixin becomes a labeled section:
//
//	## Identity
//	You are a weather specialist...
//
//	## Tool Usage
//	Use get_weather to look up...
func (b *Builder) Build() string {
	if len(b.mixins) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, m := range b.mixins {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString("## ")
		sb.WriteString(m.Name)
		sb.WriteString("\n")
		sb.WriteString(m.Content)
	}
	return sb.String()
}

// Mixins returns a copy of the current mixin list.
func (b *Builder) Mixins() []Mixin {
	out := make([]Mixin, len(b.mixins))
	copy(out, b.mixins)
	return out
}
