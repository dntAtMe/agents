package prompt

// Pre-built mixins for common agent behaviors.

// Identity returns a mixin that sets the agent's persona.
func Identity(description string) Mixin {
	return Mixin{Name: "Identity", Content: description}
}

// ToolUsage returns a mixin with instructions for how to use available tools.
func ToolUsage(instructions string) Mixin {
	return Mixin{Name: "Tool Usage", Content: instructions}
}

// OutputFormat returns a mixin constraining the response format.
func OutputFormat(instructions string) Mixin {
	return Mixin{Name: "Output Format", Content: instructions}
}

// Guardrails returns a mixin with safety or policy constraints.
func Guardrails(rules string) Mixin {
	return Mixin{Name: "Guardrails", Content: rules}
}

// HandoffPolicy returns a mixin explaining when and how to transfer to other agents.
func HandoffPolicy(policy string) Mixin {
	return Mixin{Name: "Handoff Policy", Content: policy}
}

// Context returns a mixin injecting dynamic runtime context (e.g. current date, user info).
func Context(info string) Mixin {
	return Mixin{Name: "Context", Content: info}
}
