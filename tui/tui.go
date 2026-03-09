// Package tui provides a live terminal dashboard for agent simulations
// using bubbletea and lipgloss.
package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dntatme/agents/agent"
)

// Event wraps simulation events for the TUI message channel.
type Event struct {
	Type  string // matches trace event types
	Round int
	Agent string
	Data  map[string]any
}

// SimCallbacks holds all simulation callback functions for wiring into SimulationConfig.
type SimCallbacks struct {
	OnSimulationStart func(prompt string, maxRounds int, agents []string)
	OnSimulationEnd   func(totalRounds int, reason string)
	OnRoundStart      func(round int)
	OnRoundEnd        func(round int, state map[string]any)
	OnAgentActivation func(round int, agentName string)
	OnAgentCompletion func(round int, agentName string, result *agent.RunResult, idle bool)
}

type agentInfo struct {
	Status     string // "pending", "active", "idle", "done"
	Tokens     int32
	Iterations int
	ToolCount  int
	LastTool   string
	AP         int // remaining action points
	MaxAP      int // max AP this round (for bar rendering)
	GotMail    bool // received email this round
}

type toolCallInfo struct {
	Agent    string
	Tool     string
	Duration int64 // ms, 0 if still running
	Done     bool
	APCost   int // action point cost of this tool call
}

type tab int

const (
	tabDashboard tab = iota
	tabDetail
)

type detailEntry struct {
	Time      string
	Kind      string // "thinking", "tool_start", "tool_end", "activated", "completed"
	Content   string // model text, tool args JSON, tool result JSON
	Tool      string
	Iteration int
	Tokens    int32
	Expanded  bool // for collapsible tool results
}

type agentDetail struct {
	Entries []detailEntry // ring buffer, last ~200
}

// Model is the bubbletea model for the simulation dashboard.
type Model struct {
	events      chan Event
	prompt      string
	maxRounds   int
	agents      []string
	round       int
	activeAgent string
	agentStatus map[string]agentInfo
	toolCalls   []toolCallInfo // ring buffer, last ~8
	eventLog    []string       // ring buffer, last ~25
	done        bool
	reason      string
	width       int
	height      int

	activeTab    tab
	detailData   map[string]*agentDetail
	detailScroll int
	detailAgent  int // index into m.agents for selected agent in detail view
	detailFollow bool

	// Detail view filter (press / to activate)
	detailFilter   string
	detailFiltering bool

	// Thinking preview — last thought from active agent
	lastThinking string

	// Round progress — tracks which agents have completed this round
	roundCompleted map[string]bool

	// Stock price
	stockPrice     float64
	stockDelta     float64
	stockSentiment string

	// Pause mode
	paused        bool
	pauseCh       chan struct{}       // send to pause simulation
	resumeCh      chan struct{}       // send to resume simulation
	injectCh      chan InjectEmail    // send composed emails for injection
	pauseMode     pauseScreen
	compose       composeState
	composeCursor int                 // cursor position in To field
	stateScroll   int                 // scroll in state view
	pauseSnapshot *PauseStateSnapshot
}

// InjectEmail carries email data from the TUI compose form to the main goroutine.
type InjectEmail struct {
	From    string
	To      []string
	Subject string
	Body    string
}

// New creates a new TUI model that reads events from the given channel.
func New(events chan Event, pauseCh, resumeCh chan struct{}, injectCh chan InjectEmail) *Model {
	return &Model{
		events:         events,
		agentStatus:    make(map[string]agentInfo),
		detailData:     make(map[string]*agentDetail),
		roundCompleted: make(map[string]bool),
		pauseCh:        pauseCh,
		resumeCh:       resumeCh,
		injectCh:       injectCh,
	}
}

// waitForEvent returns a tea.Cmd that blocks on the event channel.
func waitForEvent(ch chan Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return Event{Type: "channel_closed"}
		}
		return ev
	}
}

// Init starts the event listener.
func (m *Model) Init() tea.Cmd {
	return waitForEvent(m.events)
}

// Update handles incoming messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If paused, delegate all keys to pause handler
		if m.paused {
			return m, m.handlePauseKeys(msg)
		}

		// Detail filter mode
		if m.detailFiltering {
			switch msg.String() {
			case "esc":
				m.detailFiltering = false
				m.detailFilter = ""
			case "enter":
				m.detailFiltering = false
			case "backspace":
				if len(m.detailFilter) > 0 {
					m.detailFilter = m.detailFilter[:len(m.detailFilter)-1]
				}
			default:
				if len(msg.String()) == 1 {
					m.detailFilter += msg.String()
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			if m.activeTab == tabDashboard {
				m.activeTab = tabDetail
			} else {
				m.activeTab = tabDashboard
			}
			m.detailScroll = 0
		case "up", "k":
			if m.activeTab == tabDetail && m.detailScroll > 0 {
				m.detailScroll--
				m.detailFollow = false
			}
		case "down", "j":
			if m.activeTab == tabDetail {
				m.detailScroll++
				m.detailFollow = false
			}
		case "f":
			if m.activeTab == tabDetail {
				m.detailFollow = !m.detailFollow
			}
		case "left", "h":
			if m.activeTab == tabDetail && len(m.agents) > 0 {
				m.detailAgent--
				if m.detailAgent < 0 {
					m.detailAgent = len(m.agents) - 1
				}
				m.detailScroll = 0
			}
		case "right", "l":
			if m.activeTab == tabDetail && len(m.agents) > 0 {
				m.detailAgent++
				if m.detailAgent >= len(m.agents) {
					m.detailAgent = 0
				}
				m.detailScroll = 0
			}
		case "e":
			if m.activeTab == tabDetail {
				m.toggleDetailExpand()
			}
		case "/":
			if m.activeTab == tabDetail {
				m.detailFiltering = true
				m.detailFilter = ""
			}
		case "p":
			if !m.done {
				m.requestPause()
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case Event:
		m.handleEvent(msg)
		if m.done {
			return m, tea.Quit
		}
		return m, waitForEvent(m.events)
	}
	return m, nil
}

func (m *Model) requestPause() {
	// Non-blocking send to pause channel
	select {
	case m.pauseCh <- struct{}{}:
		m.paused = true
		m.pauseMode = pauseMain
	default:
		// Already a pause pending
	}
}

func (m *Model) toggleDetailExpand() {
	if len(m.agents) == 0 {
		return
	}
	agentName := m.agents[m.detailAgent]
	d := m.detailData[agentName]
	if d == nil {
		return
	}
	// Toggle expand on the entry nearest to current scroll position
	// Find visible tool_end entries and toggle the first one
	for i := range d.Entries {
		if d.Entries[i].Kind == "tool_end" {
			d.Entries[i].Expanded = !d.Entries[i].Expanded
		}
	}
}

func (m *Model) handleEvent(ev Event) {
	ts := time.Now().Format("15:04:05")

	switch ev.Type {
	case "simulation_start":
		if p, ok := ev.Data["prompt"].(string); ok {
			m.prompt = p
		}
		if mr, ok := ev.Data["max_rounds"].(int); ok {
			m.maxRounds = mr
		}
		if agents, ok := ev.Data["agents"].([]string); ok {
			m.agents = agents
			for _, name := range agents {
				m.agentStatus[name] = agentInfo{Status: "pending"}
			}
		}
		m.appendLog(fmt.Sprintf("[%s] Simulation started", ts))

	case "simulation_end":
		if r, ok := ev.Data["reason"].(string); ok {
			m.reason = r
		}
		if tr, ok := ev.Data["total_rounds"].(int); ok {
			m.round = tr
		}
		m.done = true
		m.appendLog(fmt.Sprintf("[%s] Simulation ended: %s", ts, m.reason))

	case "round_start":
		m.round = ev.Round
		// Reset round tracking
		m.roundCompleted = make(map[string]bool)
		for _, name := range m.agents {
			info := m.agentStatus[name]
			info.GotMail = false
			m.agentStatus[name] = info
		}
		m.appendLog(fmt.Sprintf("[%s] Round %d started", ts, ev.Round))

	case "round_end":
		m.appendLog(fmt.Sprintf("[%s] Round %d ended", ts, ev.Round))

	case "agent_activation":
		m.activeAgent = ev.Agent
		info := m.agentStatus[ev.Agent]
		info.Status = "active"
		m.agentStatus[ev.Agent] = info
		m.appendLog(fmt.Sprintf("[%s] %s activated", ts, ev.Agent))
		m.appendDetail(ev.Agent, detailEntry{
			Time: ts,
			Kind: "activated",
		})

	case "agent_completion":
		info := m.agentStatus[ev.Agent]
		idle := false
		if v, ok := ev.Data["idle"].(bool); ok {
			idle = v
		}
		if idle {
			info.Status = "idle"
		} else {
			info.Status = "done"
		}
		if tok, ok := ev.Data["tokens"].(int32); ok {
			info.Tokens = tok
		}
		if iter, ok := ev.Data["iterations"].(int); ok {
			info.Iterations = iter
		}
		m.agentStatus[ev.Agent] = info
		m.roundCompleted[ev.Agent] = true
		if m.activeAgent == ev.Agent {
			m.activeAgent = ""
			m.lastThinking = ""
		}
		status := "done"
		if idle {
			status = "idle"
		}
		tokStr := ""
		if info.Tokens > 0 {
			tokStr = fmt.Sprintf(", %dk tok", info.Tokens/1000)
		}
		m.appendLog(fmt.Sprintf("[%s] %s completed (%s%s)", ts, ev.Agent, status, tokStr))
		m.appendDetail(ev.Agent, detailEntry{
			Time:    ts,
			Kind:    "completed",
			Content: status + tokStr,
			Tokens:  info.Tokens,
		})

	case "agent_thinking":
		text, _ := ev.Data["text"].(string)
		iteration, _ := ev.Data["iteration"].(int)
		var tokens int32
		if t, ok := ev.Data["tokens"].(int32); ok {
			tokens = t
		}
		if tokens > 0 {
			info := m.agentStatus[ev.Agent]
			info.Tokens = tokens
			m.agentStatus[ev.Agent] = info
		}
		if text != "" {
			// Update thinking preview
			if ev.Agent == m.activeAgent {
				m.lastThinking = truncate(text, 120)
			}
			m.appendDetail(ev.Agent, detailEntry{
				Time:      ts,
				Kind:      "thinking",
				Content:   text,
				Iteration: iteration,
				Tokens:    tokens,
			})
		}

	case "tool_call_start":
		toolName := ""
		if t, ok := ev.Data["tool"].(string); ok {
			toolName = t
		}
		args, _ := ev.Data["args"].(string)
		apCost := 0
		if c, ok := ev.Data["ap_cost"].(int); ok {
			apCost = c
		}
		info := m.agentStatus[ev.Agent]
		info.ToolCount++
		info.LastTool = toolName
		// Detect email receipt
		if toolName == "check_inbox" || toolName == "send_email" {
			info.GotMail = true
		}
		m.agentStatus[ev.Agent] = info
		m.appendToolCall(toolCallInfo{
			Agent:  ev.Agent,
			Tool:   toolName,
			Done:   false,
			APCost: apCost,
		})
		m.appendDetail(ev.Agent, detailEntry{
			Time:    ts,
			Kind:    "tool_start",
			Tool:    toolName,
			Content: args,
		})

	case "tool_call_end":
		toolName := ""
		if t, ok := ev.Data["tool"].(string); ok {
			toolName = t
		}
		var dur int64
		if d, ok := ev.Data["duration_ms"].(int64); ok {
			dur = d
		}
		result, _ := ev.Data["result"].(string)
		// Update matching pending tool call
		for i := len(m.toolCalls) - 1; i >= 0; i-- {
			if m.toolCalls[i].Tool == toolName && !m.toolCalls[i].Done {
				m.toolCalls[i].Done = true
				m.toolCalls[i].Duration = dur
				break
			}
		}
		m.appendDetail(ev.Agent, detailEntry{
			Time:    ts,
			Kind:    "tool_end",
			Tool:    toolName,
			Content: fmt.Sprintf("%dms | %s", dur, result),
		})

	case "ap_update":
		info := m.agentStatus[ev.Agent]
		if ap, ok := ev.Data["remaining"].(int); ok {
			info.AP = ap
		}
		if maxAP, ok := ev.Data["max_ap"].(int); ok {
			info.MaxAP = maxAP
		}
		m.agentStatus[ev.Agent] = info

		// If this update was triggered by a tool call, tag the most recent pending tool call with cost
		if cost, ok := ev.Data["cost"].(int); ok && cost > 0 {
			for i := len(m.toolCalls) - 1; i >= 0; i-- {
				if !m.toolCalls[i].Done && m.toolCalls[i].APCost == 0 {
					m.toolCalls[i].APCost = cost
					break
				}
			}
		}

	case "stock_update":
		if price, ok := ev.Data["price"].(float64); ok {
			m.stockPrice = price
		}
		if delta, ok := ev.Data["delta"].(float64); ok {
			m.stockDelta = delta
		}
		if sentiment, ok := ev.Data["sentiment"].(string); ok {
			m.stockSentiment = sentiment
		}
		m.appendLog(fmt.Sprintf("[%s] Stock: $%.2f (%+.2f) — %s", ts, m.stockPrice, m.stockDelta, m.stockSentiment))

	case "pause_ack":
		// Simulation confirms it's paused, with state snapshot
		m.paused = true
		if snap, ok := ev.Data["snapshot"].(*PauseStateSnapshot); ok {
			m.pauseSnapshot = snap
		}

	case "channel_closed":
		m.done = true
	}
}

func (m *Model) appendLog(entry string) {
	m.eventLog = append(m.eventLog, entry)
	if len(m.eventLog) > 25 {
		m.eventLog = m.eventLog[len(m.eventLog)-25:]
	}
}

func (m *Model) appendToolCall(tc toolCallInfo) {
	m.toolCalls = append(m.toolCalls, tc)
	if len(m.toolCalls) > 8 {
		m.toolCalls = m.toolCalls[len(m.toolCalls)-8:]
	}
}

func (m *Model) appendDetail(agentName string, entry detailEntry) {
	d, ok := m.detailData[agentName]
	if !ok {
		d = &agentDetail{}
		m.detailData[agentName] = d
	}
	d.Entries = append(d.Entries, entry)
	if len(d.Entries) > 200 {
		d.Entries = d.Entries[len(d.Entries)-200:]
	}
}

// --- Styles ---

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	sectionTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12"))

	activeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true)

	idleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	pendingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	doneStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14"))

	logStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))

	toolRunning = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11"))

	toolDone = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	thinkingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("13"))

	detailToolStart = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11"))

	detailToolEnd = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	detailContent = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))

	tabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62"))

	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("8"))

	energyHighStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")) // green

	energyMidStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")) // yellow

	energyLowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")) // red

	energyEmptyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("8")) // dim

	progressDone = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	progressActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("10"))

	progressPending = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))

	mailStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11"))

	thinkingPreview = lipgloss.NewStyle().
			Foreground(lipgloss.Color("13")).
			Italic(true)

	filterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Bold(true)

	toolArgKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true)
)

// View renders the TUI dashboard.
func (m *Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	// Pause overlay takes over the whole screen
	if m.paused {
		return m.renderPauseOverlay()
	}

	var sections []string

	sections = append(sections, m.renderHeader())

	switch m.activeTab {
	case tabDashboard:
		sections = append(sections, m.renderMainArea())
		sections = append(sections, m.renderEventLog())
	case tabDetail:
		sections = append(sections, m.renderDetail())
	}

	if m.done {
		sections = append(sections, m.renderFooter())
	} else {
		hint := "  Tab: views  p: pause  q: quit"
		if m.activeTab == tabDetail {
			hint = "  Tab: views  h/l: agent  j/k: scroll  f: follow  /: filter  e: expand  p: pause  q: quit"
		}
		sections = append(sections, idleStyle.Render(hint))
	}

	return strings.Join(sections, "\n")
}

func (m *Model) renderHeader() string {
	roundInfo := fmt.Sprintf("Round %d", m.round)
	if m.maxRounds > 0 {
		roundInfo = fmt.Sprintf("Round %d/%d", m.round, m.maxRounds)
	}

	activeInfo := ""
	if m.activeAgent != "" {
		activeInfo = fmt.Sprintf("  > %s", m.activeAgent)
	}

	status := "running"
	if m.done {
		status = "done"
	}

	// Tab indicator
	var dashTab, detailTab string
	if m.activeTab == tabDashboard {
		dashTab = tabActiveStyle.Render(" Dashboard ")
		detailTab = tabInactiveStyle.Render(" Detail ")
	} else {
		dashTab = tabInactiveStyle.Render(" Dashboard ")
		detailTab = tabActiveStyle.Render(" Detail ")
	}
	tabs := fmt.Sprintf("[%s|%s]", dashTab, detailTab)

	stockInfo := ""
	if m.stockPrice > 0 {
		arrow := "→"
		var stockStyle lipgloss.Style
		if m.stockDelta > 0 {
			arrow = "↑"
			stockStyle = energyHighStyle
		} else if m.stockDelta < 0 {
			arrow = "↓"
			stockStyle = energyLowStyle
		} else {
			stockStyle = energyMidStyle
		}
		stockInfo = "  " + stockStyle.Render(fmt.Sprintf("$%.2f %s%+.2f", m.stockPrice, arrow, m.stockDelta))
	}

	header := fmt.Sprintf("  Company Sim [%s]   %s%s%s  %s", status, roundInfo, activeInfo, stockInfo, tabs)

	w := m.width
	if w < 40 {
		w = 40
	}

	var parts []string
	parts = append(parts, headerStyle.Width(w).Render(header))

	// Thinking preview line
	if m.lastThinking != "" && m.activeAgent != "" {
		preview := fmt.Sprintf("  %s: %s", m.activeAgent, m.lastThinking)
		if len(preview) > w-4 {
			preview = preview[:w-7] + "..."
		}
		parts = append(parts, thinkingPreview.Render(preview))
	}

	// Round progress bar
	if m.round > 0 && len(m.agents) > 0 {
		parts = append(parts, m.renderRoundProgress())
	}

	return strings.Join(parts, "\n")
}

func (m *Model) renderRoundProgress() string {
	var parts []string
	parts = append(parts, "  [")
	for i, name := range m.agents {
		if i > 0 {
			parts = append(parts, " > ")
		}
		// Abbreviate agent name
		short := abbreviateAgent(name)
		if name == m.activeAgent {
			parts = append(parts, progressActive.Render("▶"+short))
		} else if m.roundCompleted[name] {
			parts = append(parts, progressDone.Render(short))
		} else {
			parts = append(parts, progressPending.Render(short))
		}
	}
	parts = append(parts, "]")
	return strings.Join(parts, "")
}

func abbreviateAgent(name string) string {
	switch name {
	case "ceo":
		return "CEO"
	case "product-manager":
		return "PM"
	case "cto":
		return "CTO"
	case "architect":
		return "ARCH"
	case "project-manager":
		return "PJM"
	case "backend-dev":
		return "BE"
	case "frontend-dev":
		return "FE"
	case "devops":
		return "DO"
	case "shareholders":
		return "SH"
	default:
		if len(name) > 4 {
			return strings.ToUpper(name[:4])
		}
		return strings.ToUpper(name)
	}
}

func (m *Model) renderMainArea() string {
	agentCol := m.renderAgents()
	toolCol := m.renderToolCalls()

	halfWidth := m.width/2 - 3
	if halfWidth < 20 {
		halfWidth = 20
	}

	left := borderStyle.Width(halfWidth).Render(agentCol)
	right := borderStyle.Width(halfWidth).Render(toolCol)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m *Model) renderAgents() string {
	var sb strings.Builder
	sb.WriteString(sectionTitle.Render("Agents"))
	sb.WriteString("\n")

	for _, name := range m.agents {
		info := m.agentStatus[name]
		var icon string
		var style lipgloss.Style

		switch info.Status {
		case "active":
			icon = "◉"
			style = activeStyle
		case "idle":
			icon = "○"
			style = idleStyle
		case "done":
			icon = "●"
			style = doneStyle
		default: // pending
			icon = "○"
			style = pendingStyle
		}

		label := fmt.Sprintf("%s %-16s %-7s", icon, name, info.Status)

		// Email indicator
		if info.GotMail {
			label += mailStyle.Render(" ✉")
		}

		// Energy bar
		if info.MaxAP > 0 {
			label += "  " + renderEnergyBar(info.AP, info.MaxAP)
		}

		if info.Tokens > 0 {
			if info.Tokens >= 1000 {
				label += fmt.Sprintf("  %.1fk tok", float64(info.Tokens)/1000)
			} else {
				label += fmt.Sprintf("  %d tok", info.Tokens)
			}
		}
		if info.ToolCount > 0 {
			label += fmt.Sprintf("  [%d tools]", info.ToolCount)
		}
		sb.WriteString(style.Render(label))
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m *Model) renderToolCalls() string {
	var sb strings.Builder
	sb.WriteString(sectionTitle.Render("Tool Calls (by agent)"))
	sb.WriteString("\n")

	if len(m.toolCalls) == 0 {
		sb.WriteString(idleStyle.Render("  (none yet)"))
		sb.WriteString("\n")
		return sb.String()
	}

	// Group tool calls by agent, show last 3 per agent
	agentTools := make(map[string][]toolCallInfo)
	var agentOrder []string
	seen := make(map[string]bool)
	for _, tc := range m.toolCalls {
		if !seen[tc.Agent] {
			seen[tc.Agent] = true
			agentOrder = append(agentOrder, tc.Agent)
		}
		agentTools[tc.Agent] = append(agentTools[tc.Agent], tc)
	}

	for _, agentName := range agentOrder {
		tools := agentTools[agentName]
		// Show last 3
		start := 0
		if len(tools) > 3 {
			start = len(tools) - 3
		}

		short := abbreviateAgent(agentName)
		sb.WriteString(idleStyle.Render(fmt.Sprintf("  %s:", short)))
		sb.WriteString("\n")

		for _, tc := range tools[start:] {
			toolDisplay := tc.Tool
			if len(toolDisplay) > 16 {
				toolDisplay = toolDisplay[:14] + ".."
			}

			apTag := ""
			if tc.APCost > 0 {
				apTag = fmt.Sprintf("  -%dAP", tc.APCost)
			}

			if tc.Done {
				line := fmt.Sprintf("    > %-16s %4dms%s", toolDisplay, tc.Duration, apTag)
				sb.WriteString(toolDone.Render(line))
			} else {
				line := fmt.Sprintf("    * %-16s ...%s", toolDisplay, apTag)
				sb.WriteString(toolRunning.Render(line))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (m *Model) renderEventLog() string {
	var sb strings.Builder
	title := sectionTitle.Render("Event Log")

	w := m.width - 4
	if w < 30 {
		w = 30
	}

	sb.WriteString(title)
	sb.WriteString("\n")

	for _, entry := range m.eventLog {
		display := entry
		if len(display) > w {
			display = display[:w-3] + "..."
		}
		sb.WriteString(logStyle.Render("  " + display))
		sb.WriteString("\n")
	}

	return borderStyle.Width(m.width - 2).Render(sb.String())
}

func (m *Model) renderDetail() string {
	if len(m.agents) == 0 {
		return borderStyle.Width(m.width - 2).Render(
			idleStyle.Render("  No agents registered yet"),
		)
	}

	// Clamp index
	if m.detailAgent < 0 {
		m.detailAgent = 0
	}
	if m.detailAgent >= len(m.agents) {
		m.detailAgent = len(m.agents) - 1
	}
	agentName := m.agents[m.detailAgent]

	var sb strings.Builder

	// Agent selector: < agent1 | [agent2] | agent3 >
	var selectorParts []string
	for i, name := range m.agents {
		if i == m.detailAgent {
			selectorParts = append(selectorParts, tabActiveStyle.Render(" "+name+" "))
		} else {
			selectorParts = append(selectorParts, tabInactiveStyle.Render(" "+name+" "))
		}
	}
	selector := "<< " + strings.Join(selectorParts, idleStyle.Render("|")) + " >>"
	sb.WriteString(selector)
	sb.WriteString("\n")

	// Agent info line
	info := m.agentStatus[agentName]
	tokStr := ""
	if info.Tokens > 0 {
		if info.Tokens >= 1000 {
			tokStr = fmt.Sprintf("  tokens %.1fk", float64(info.Tokens)/1000)
		} else {
			tokStr = fmt.Sprintf("  tokens %d", info.Tokens)
		}
	}
	iterStr := ""
	if info.Iterations > 0 {
		iterStr = fmt.Sprintf("  iter %d", info.Iterations)
	}
	apStr := ""
	if info.MaxAP > 0 {
		apStr = "  " + renderEnergyBar(info.AP, info.MaxAP)
	}
	agentHeader := fmt.Sprintf("◉ %s  %s%s%s%s", agentName, info.Status, apStr, iterStr, tokStr)
	sb.WriteString(activeStyle.Render(agentHeader))
	sb.WriteString("\n")

	// Filter indicator
	if m.detailFilter != "" {
		sb.WriteString(filterStyle.Render(fmt.Sprintf("  filter: %s", m.detailFilter)))
		sb.WriteString("\n")
	} else if m.detailFiltering {
		sb.WriteString(filterStyle.Render("  filter: █"))
		sb.WriteString("\n")
	}

	sb.WriteString(strings.Repeat("─", m.width-6))
	sb.WriteString("\n")

	d := m.detailData[agentName]
	if d == nil || len(d.Entries) == 0 {
		sb.WriteString(idleStyle.Render("  (no entries yet)"))
		sb.WriteString("\n")
		return borderStyle.Width(m.width - 2).Render(sb.String())
	}

	// Build display lines from entries
	var lines []string
	contentWidth := m.width - 10
	if contentWidth < 20 {
		contentWidth = 20
	}

	for _, e := range d.Entries {
		// Apply filter
		if m.detailFilter != "" {
			filterLower := strings.ToLower(m.detailFilter)
			entryText := strings.ToLower(e.Kind + " " + e.Tool + " " + e.Content)
			if !strings.Contains(entryText, filterLower) {
				continue
			}
		}

		switch e.Kind {
		case "thinking":
			header := thinkingStyle.Render(fmt.Sprintf("[%s] Thinking (iter %d)", e.Time, e.Iteration))
			lines = append(lines, header)
			for _, line := range wrapText(e.Content, contentWidth) {
				lines = append(lines, detailContent.Render("  "+line))
			}
		case "tool_start":
			header := detailToolStart.Render(fmt.Sprintf("[%s] >> %s", e.Time, e.Tool))
			lines = append(lines, header)
			if e.Content != "" {
				// Color-code key names in tool args
				colored := colorizeToolArgs(e.Content, contentWidth-8)
				lines = append(lines, "  args: "+colored)
			}
		case "tool_end":
			header := detailToolEnd.Render(fmt.Sprintf("[%s] << %s", e.Time, e.Tool))
			lines = append(lines, header)
			if e.Content != "" {
				if e.Expanded {
					// Show full result wrapped
					for _, line := range wrapText(e.Content, contentWidth-10) {
						lines = append(lines, detailContent.Render("  result: "+line))
					}
				} else {
					// One-line summary
					lines = append(lines, detailContent.Render("  result: "+truncate(e.Content, contentWidth-10)))
				}
			}
		case "activated":
			lines = append(lines, activeStyle.Render(fmt.Sprintf("[%s] Agent activated", e.Time)))
		case "completed":
			lines = append(lines, doneStyle.Render(fmt.Sprintf("[%s] Agent completed (%s)", e.Time, e.Content)))
		}
	}

	// Apply scroll
	maxVisible := m.height - 10
	if maxVisible < 5 {
		maxVisible = 5
	}

	// Clamp scroll
	maxScroll := len(lines) - maxVisible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.detailFollow {
		m.detailScroll = maxScroll
	}
	if m.detailScroll > maxScroll {
		m.detailScroll = maxScroll
	}

	start := m.detailScroll
	end := start + maxVisible
	if end > len(lines) {
		end = len(lines)
	}

	for _, line := range lines[start:end] {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	if maxScroll > 0 {
		scrollInfo := fmt.Sprintf("  [%d/%d]", m.detailScroll+1, maxScroll+1)
		if m.detailFollow {
			scrollInfo += " follow:on"
		}
		sb.WriteString(idleStyle.Render(scrollInfo))
		sb.WriteString("\n")
	}

	return borderStyle.Width(m.width - 2).Render(sb.String())
}

func (m *Model) renderFooter() string {
	msg := fmt.Sprintf("  Simulation complete: %s (press q to exit)", m.reason)
	return headerStyle.Width(m.width).Render(msg)
}

// --- Helper functions ---

func truncate(s string, maxLen int) string {
	if maxLen < 4 {
		maxLen = 4
	}
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

func wrapText(s string, width int) []string {
	if width < 10 {
		width = 10
	}
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		for len(line) > width {
			lines = append(lines, line[:width])
			line = line[width:]
		}
		lines = append(lines, line)
	}
	return lines
}

// renderEnergyBar draws a compact energy gauge like "⚡██░░ 8/15".
func renderEnergyBar(current, max int) string {
	if max <= 0 {
		return ""
	}

	const barLen = 5
	filled := 0
	if current > 0 {
		filled = (current * barLen) / max
		if filled > barLen {
			filled = barLen
		}
		if current > 0 && filled == 0 {
			filled = 1 // show at least one block if any AP left
		}
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)
	label := fmt.Sprintf("%d/%d", current, max)

	// Pick color based on ratio
	var style lipgloss.Style
	ratio := float64(current) / float64(max)
	switch {
	case ratio > 0.5:
		style = energyHighStyle
	case ratio > 0.2:
		style = energyMidStyle
	case current > 0:
		style = energyLowStyle
	default:
		style = energyEmptyStyle
	}

	return style.Render("⚡" + bar + " " + label)
}

// colorizeToolArgs highlights key names (e.g. "to:", "subject:") in tool arguments.
func colorizeToolArgs(args string, maxLen int) string {
	truncated := truncate(args, maxLen)
	// Highlight common JSON key patterns
	result := truncated
	for _, key := range []string{"to:", "from:", "subject:", "body:", "file:", "path:", "content:", "name:", "status:", "agent:"} {
		result = strings.ReplaceAll(result, `"`+key[:len(key)-1]+`"`, toolArgKey.Render(`"`+key[:len(key)-1]+`"`))
	}
	return result
}

func formatArgs(args map[string]any) string {
	if len(args) == 0 {
		return "{}"
	}
	b, err := json.Marshal(args)
	if err != nil {
		return fmt.Sprintf("%v", args)
	}
	return truncate(string(b), 300)
}

func formatResult(result map[string]any) string {
	if len(result) == 0 {
		return "{}"
	}
	b, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf("%v", result)
	}
	return truncate(string(b), 300)
}

func extractTextFromContent(content *genai.Content) string {
	if content == nil {
		return ""
	}
	var parts []string
	for _, p := range content.Parts {
		if p.Text != "" {
			parts = append(parts, p.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// --- Hooks factory ---

// Hooks returns agent.Hooks that send tool_call_start, tool_call_end, and agent_thinking events to the channel.
func Hooks(ch chan Event) *agent.Hooks {
	started := make(map[string]time.Time)
	return &agent.Hooks{
		AfterPredict: func(ctx context.Context, hc *agent.HookContext, content *genai.Content) error {
			text := extractTextFromContent(content)
			if text != "" {
				ch <- Event{
					Type:  "agent_thinking",
					Agent: hc.Agent.Name,
					Data: map[string]any{
						"text":      truncate(text, 500),
						"iteration": hc.Iteration,
						"tokens":    hc.TotalTokens,
					},
				}
			}
			return nil
		},
		BeforeToolCall: func(ctx context.Context, hc *agent.HookContext, fc *genai.FunctionCall) error {
			started[fc.Name] = time.Now()
			ch <- Event{
				Type:  "tool_call_start",
				Agent: hc.Agent.Name,
				Data: map[string]any{
					"tool": fc.Name,
					"args": formatArgs(fc.Args),
				},
			}
			return nil
		},
		AfterToolCall: func(ctx context.Context, hc *agent.HookContext, fc *genai.FunctionCall, result map[string]any) error {
			var dur int64
			if start, ok := started[fc.Name]; ok {
				dur = time.Since(start).Milliseconds()
				delete(started, fc.Name)
			}
			ch <- Event{
				Type:  "tool_call_end",
				Agent: hc.Agent.Name,
				Data: map[string]any{
					"tool":        fc.Name,
					"duration_ms": dur,
					"result":      formatResult(result),
				},
			}
			return nil
		},
	}
}

// --- Callbacks factory ---

// Callbacks returns SimCallbacks that send events to the channel.
// Use these to wire into SimulationConfig.
func Callbacks(ch chan Event) SimCallbacks {
	return SimCallbacks{
		OnSimulationStart: func(prompt string, maxRounds int, agents []string) {
			ch <- Event{
				Type: "simulation_start",
				Data: map[string]any{
					"prompt":     prompt,
					"max_rounds": maxRounds,
					"agents":     agents,
				},
			}
		},
		OnSimulationEnd: func(totalRounds int, reason string) {
			ch <- Event{
				Type: "simulation_end",
				Data: map[string]any{
					"total_rounds": totalRounds,
					"reason":       reason,
				},
			}
			close(ch)
		},
		OnRoundStart: func(round int) {
			ch <- Event{
				Type:  "round_start",
				Round: round,
			}
		},
		OnRoundEnd: func(round int, state map[string]any) {
			ch <- Event{
				Type:  "round_end",
				Round: round,
			}
		},
		OnAgentActivation: func(round int, agentName string) {
			ch <- Event{
				Type:  "agent_activation",
				Round: round,
				Agent: agentName,
			}
		},
		OnAgentCompletion: func(round int, agentName string, result *agent.RunResult, idle bool) {
			data := map[string]any{
				"idle": idle,
			}
			if result != nil {
				data["tokens"] = result.TotalTokens
				data["iterations"] = result.Iterations
			}
			ch <- Event{
				Type:  "agent_completion",
				Round: round,
				Agent: agentName,
				Data:  data,
			}
		},
	}
}
