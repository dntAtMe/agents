package conversation

import "google.golang.org/genai"

// Conversation wraps a Gemini message history.
type Conversation struct {
	Messages []*genai.Content
}

// New creates an empty conversation.
func New() *Conversation {
	return &Conversation{}
}

// AppendUserText adds a user text message.
func (c *Conversation) AppendUserText(text string) {
	c.Messages = append(c.Messages, &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: text}},
	})
}

// AppendModelContent adds a model response to the history.
func (c *Conversation) AppendModelContent(content *genai.Content) {
	c.Messages = append(c.Messages, content)
}

// AppendToolResults adds tool responses as a user-role message.
func (c *Conversation) AppendToolResults(parts []*genai.Part) {
	c.Messages = append(c.Messages, &genai.Content{
		Role:  "user",
		Parts: parts,
	})
}
