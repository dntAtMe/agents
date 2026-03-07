package appdesign

import "encoding/json"

// Component represents a single element in the app design.
type Component struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Connection represents a directed link between two components.
type Connection struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Description string `json:"description"`
}

// AppDesign holds the full application design.
type AppDesign struct {
	Components  []Component  `json:"components"`
	Connections []Connection `json:"connections"`
}

// String returns a pretty-printed JSON representation.
func (d *AppDesign) String() string {
	b, _ := json.MarshalIndent(d, "", "  ")
	return string(b)
}
