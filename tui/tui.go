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
}

type toolCallInfo struct {
	Agent    string
	Tool     string
	Duration int64 // ms, 0 if still running
	Done     bool
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
}

type agentDetail struct {
	Entries []detailEntry // ring buffer, last ~50
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
	eventLog    []string       // ring buffer, last ~15
	done        bool
	reason      string
	width       int
	height      int

	activeTab    tab
	detailData   map[string]*agentDetail
	detailScroll int
	detailAgent  int // index into m.agents for selected agent in detail view
	detailFollow bool
}

// New creates a new TUI model that reads events from the given channel.
func New(events chan Event) *Model {
	return &Model{
		events:      events,
		agentStatus: make(map[string]agentInfo),
		detailData:  make(map[string]*agentDetail),
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
		if m.activeAgent == ev.Agent {
			m.activeAgent = ""
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
		if text != "" {
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
		info := m.agentStatus[ev.Agent]
		info.ToolCount++
		info.LastTool = toolName
		m.agentStatus[ev.Agent] = info
		m.appendToolCall(toolCallInfo{
			Agent: ev.Agent,
			Tool:  toolName,
			Done:  false,
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

	case "channel_closed":
		m.done = true
	}
}

func (m *Model) appendLog(entry string) {
	m.eventLog = append(m.eventLog, entry)
	if len(m.eventLog) > 15 {
		m.eventLog = m.eventLog[len(m.eventLog)-15:]
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
	if len(d.Entries) > 50 {
		d.Entries = d.Entries[len(d.Entries)-50:]
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
)

// View renders the TUI dashboard.
func (m *Model) View() string {
	if m.width == 0 {
		return "Initializing..."
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
		hint := "  Press Tab to switch views, q to quit"
		if m.activeTab == tabDetail {
			hint = "  Tab: views  h/l: agent  j/k: scroll  f: follow  q: quit"
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

	header := fmt.Sprintf("  Company Sim [%s]   %s%s  %s", status, roundInfo, activeInfo, tabs)

	w := m.width
	if w < 40 {
		w = 40
	}
	return headerStyle.Width(w).Render(header)
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

		label := fmt.Sprintf("%s %-16s %s", icon, name, info.Status)
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
	sb.WriteString(sectionTitle.Render("Tool Calls"))
	sb.WriteString("\n")

	if len(m.toolCalls) == 0 {
		sb.WriteString(idleStyle.Render("  (none yet)"))
		sb.WriteString("\n")
		return sb.String()
	}

	for _, tc := range m.toolCalls {
		toolDisplay := tc.Tool
		if len(toolDisplay) > 16 {
			toolDisplay = toolDisplay[:14] + ".."
		}

		if tc.Done {
			line := fmt.Sprintf("  > %-16s %dms", toolDisplay, tc.Duration)
			sb.WriteString(toolDone.Render(line))
		} else {
			line := fmt.Sprintf("  * %-16s ...", toolDisplay)
			sb.WriteString(toolRunning.Render(line))
		}
		sb.WriteString("\n")
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
	agentHeader := fmt.Sprintf("◉ %s  %s%s%s", agentName, info.Status, iterStr, tokStr)
	sb.WriteString(activeStyle.Render(agentHeader))
	sb.WriteString("\n")
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
				lines = append(lines, detailContent.Render("  args: "+truncate(e.Content, contentWidth-8)))
			}
		case "tool_end":
			header := detailToolEnd.Render(fmt.Sprintf("[%s] << %s", e.Time, e.Tool))
			lines = append(lines, header)
			if e.Content != "" {
				lines = append(lines, detailContent.Render("  result: "+truncate(e.Content, contentWidth-10)))
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
