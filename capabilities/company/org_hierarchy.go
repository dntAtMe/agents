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
