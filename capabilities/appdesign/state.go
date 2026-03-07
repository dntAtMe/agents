package appdesign

// GetDesign retrieves the AppDesign from shared state, creating one if absent.
func GetDesign(state map[string]any) *AppDesign {
	if v, ok := state["design"]; ok {
		if d, ok := v.(*AppDesign); ok {
			return d
		}
	}
	d := &AppDesign{}
	state["design"] = d
	return d
}
