package company

import "sync"

// ActionPointTracker manages per-agent per-round action point budgets.
type ActionPointTracker struct {
	mu         sync.Mutex
	Budgets    map[string]int  // agent → remaining AP this round
	CoffeeNext map[string]bool // agent → gets bonus AP next round
	DefaultAP  int
	BonusAP    int
	HardCap    int // negative AP threshold for force-stop (stored as positive)
}

// NewActionPointTracker creates a tracker with the given budget parameters.
func NewActionPointTracker(defaultAP, bonusAP, hardCap int) *ActionPointTracker {
	return &ActionPointTracker{
		Budgets:    make(map[string]int),
		CoffeeNext: make(map[string]bool),
		DefaultAP:  defaultAP,
		BonusAP:    bonusAP,
		HardCap:    hardCap,
	}
}

// InitRound resets budgets for all agents, applying coffee bonuses from last round,
// then clears the coffee bonus registrations so they don't carry over.
func (t *ActionPointTracker) InitRound(agents []string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, agent := range agents {
		budget := t.DefaultAP
		if t.CoffeeNext[agent] {
			budget += t.BonusAP
		}
		t.Budgets[agent] = budget
	}
	// Clear after applying so bonuses are one-shot
	t.CoffeeNext = make(map[string]bool)
}

// SetBudget explicitly sets an agent's AP budget (used for urgent email activations).
func (t *ActionPointTracker) SetBudget(agent string, amount int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Budgets[agent] = amount
}

// Remaining returns the remaining AP for an agent.
func (t *ActionPointTracker) Remaining(agent string) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	if v, ok := t.Budgets[agent]; ok {
		return v
	}
	return t.DefaultAP
}

// Deduct subtracts cost from the agent's AP and returns the new remaining value.
func (t *ActionPointTracker) Deduct(agent string, cost int) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.Budgets[agent]; !ok {
		t.Budgets[agent] = t.DefaultAP
	}
	t.Budgets[agent] -= cost
	return t.Budgets[agent]
}

// RegisterCoffee marks an agent for a coffee break bonus next round.
func (t *ActionPointTracker) RegisterCoffee(agent string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.CoffeeNext[agent] = true
}

// CoffeeParticipants returns agents who called get_coffee this round.
func (t *ActionPointTracker) CoffeeParticipants() []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	var participants []string
	for agent := range t.CoffeeNext {
		participants = append(participants, agent)
	}
	return participants
}

// ClearCoffee clears coffee registrations after a break runs.
func (t *ActionPointTracker) ClearCoffee() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.CoffeeNext = make(map[string]bool)
}
