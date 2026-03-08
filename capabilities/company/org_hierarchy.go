package company

import "sync"

// OrgHierarchy represents the organizational structure of the company.
type OrgHierarchy struct {
	mu        sync.Mutex
	ManagerOf map[string][]string // manager -> direct reports
	ReportsTo map[string]string   // agent -> their manager
}

// NewOrgHierarchy creates an empty org hierarchy.
func NewOrgHierarchy() *OrgHierarchy {
	return &OrgHierarchy{
		ManagerOf: make(map[string][]string),
		ReportsTo: make(map[string]string),
	}
}

// SetManager establishes a manager-subordinate relationship.
func (oh *OrgHierarchy) SetManager(subordinate, manager string) {
	oh.mu.Lock()
	defer oh.mu.Unlock()
	oh.ReportsTo[subordinate] = manager
	oh.ManagerOf[manager] = append(oh.ManagerOf[manager], subordinate)
}

// GetManager returns the manager of the given agent, or "" if none.
func (oh *OrgHierarchy) GetManager(agent string) string {
	oh.mu.Lock()
	defer oh.mu.Unlock()
	return oh.ReportsTo[agent]
}

// GetDirectReports returns the direct reports of a manager.
func (oh *OrgHierarchy) GetDirectReports(manager string) []string {
	oh.mu.Lock()
	defer oh.mu.Unlock()
	return oh.ManagerOf[manager]
}

// IsManager returns true if the agent has direct reports.
func (oh *OrgHierarchy) IsManager(agent string) bool {
	oh.mu.Lock()
	defer oh.mu.Unlock()
	return len(oh.ManagerOf[agent]) > 0
}

// IsInManagementChain returns true if target is a direct or indirect report of manager.
func (oh *OrgHierarchy) IsInManagementChain(manager, target string) bool {
	if manager == "" || target == "" || manager == target {
		return false
	}

	oh.mu.Lock()
	defer oh.mu.Unlock()

	queue := append([]string{}, oh.ManagerOf[manager]...)
	seen := make(map[string]bool)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if seen[current] {
			continue
		}
		seen[current] = true

		if current == target {
			return true
		}

		queue = append(queue, oh.ManagerOf[current]...)
	}
	return false
}
