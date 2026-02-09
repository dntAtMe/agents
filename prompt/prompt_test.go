package prompt

import (
	"strings"
	"testing"
)

func TestBuilderEmpty(t *testing.T) {
	b := NewBuilder()
	if got := b.Build(); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestBuilderSingleMixin(t *testing.T) {
	got := NewBuilder().
		Add(Identity("You are a helpful assistant.")).
		Build()

	expected := "## Identity\nYou are a helpful assistant."
	if got != expected {
		t.Errorf("got:\n%s\nwant:\n%s", got, expected)
	}
}

func TestBuilderMultipleMixins(t *testing.T) {
	got := NewBuilder().
		Add(Identity("You are a weather bot.")).
		Add(ToolUsage("Use get_weather to look up data.")).
		Add(Guardrails("Never reveal internal tool names.")).
		Build()

	// Verify section ordering and headers.
	sections := []string{"## Identity", "## Tool Usage", "## Guardrails"}
	for _, s := range sections {
		if !strings.Contains(got, s) {
			t.Errorf("missing section %q in:\n%s", s, got)
		}
	}

	// Verify sections are separated by double newlines.
	parts := strings.Split(got, "\n\n")
	if len(parts) != 3 {
		t.Errorf("expected 3 sections, got %d", len(parts))
	}
}

func TestBuilderAddSection(t *testing.T) {
	got := NewBuilder().
		AddSection("Custom", "Custom content here.").
		Build()

	if !strings.Contains(got, "## Custom\nCustom content here.") {
		t.Errorf("unexpected output: %s", got)
	}
}

func TestBuilderChaining(t *testing.T) {
	b := NewBuilder().
		Add(Identity("Bot")).
		AddSection("Mode", "Verbose mode enabled.")

	if len(b.Mixins()) != 2 {
		t.Errorf("expected 2 mixins, got %d", len(b.Mixins()))
	}
}

func TestMixinsReturnsCopy(t *testing.T) {
	b := NewBuilder().Add(Identity("A"))
	mixins := b.Mixins()
	mixins[0].Name = "Changed"

	if b.Mixins()[0].Name != "Identity" {
		t.Error("Mixins() should return a copy, not a reference")
	}
}

func TestLibraryHelpers(t *testing.T) {
	tests := []struct {
		mixin Mixin
		name  string
	}{
		{Identity("x"), "Identity"},
		{ToolUsage("x"), "Tool Usage"},
		{OutputFormat("x"), "Output Format"},
		{Guardrails("x"), "Guardrails"},
		{HandoffPolicy("x"), "Handoff Policy"},
		{Context("x"), "Context"},
	}
	for _, tt := range tests {
		if tt.mixin.Name != tt.name {
			t.Errorf("expected name %q, got %q", tt.name, tt.mixin.Name)
		}
		if tt.mixin.Content != "x" {
			t.Errorf("expected content 'x', got %q", tt.mixin.Content)
		}
	}
}
