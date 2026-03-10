package conversation

import "github.com/dntatme/agents/llm"

// Conversation wraps a message history.
type Conversation struct {
	Messages []*llm.Content
}

// New creates an empty conversation.
func New() *Conversation {
	return &Conversation{}
}

// AppendUserText adds a user text message.
func (c *Conversation) AppendUserText(text string) {
	c.Messages = append(c.Messages, &llm.Content{
		Role:  "user",
		Parts: []*llm.Part{{Text: text}},
	})
}

// AppendModelContent adds a model response to the history.
func (c *Conversation) AppendModelContent(content *llm.Content) {
	c.Messages = append(c.Messages, content)
}

// AppendToolResults adds tool responses as a user-role message.
func (c *Conversation) AppendToolResults(parts []*llm.Part) {
	c.Messages = append(c.Messages, &llm.Content{
		Role:  "user",
		Parts: parts,
	})
}
