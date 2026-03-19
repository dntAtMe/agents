package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// pauseScreen identifies which pause sub-screen is active.
type pauseScreen int

const (
	pauseMain    pauseScreen = iota // main pause menu
	pauseCompose                    // email compose form
	pauseState                      // simulation state overview
)

// composeState holds the email compose form fields.
type composeState struct {
	from    int      // index into agents list for sender
	to      []bool   // toggle for each agent (recipients)
	subject string   // text input
	body    string   // text input
	field   int      // 0=from, 1=to, 2=subject, 3=body
}

// PauseStateSnapshot holds a read-only snapshot of simulation state for the view screen.
type PauseStateSnapshot struct {
	Round    int
	Agents   []PauseAgentInfoEntry
	TaskInfo string
	Emails   int
}

// PauseAgentInfoEntry holds per-agent info for the pause state view.
type PauseAgentInfoEntry struct {
	Name     string
	Status   string
	AP       int
	MaxAP    int
	Patience int
}

var (
	pauseTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("208")).
			Padding(0, 1)

	pauseKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("11"))

	pauseDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))

	composeFieldActive = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("62")).
				Padding(0, 1)

	composeFieldInactive = lipgloss.NewStyle().
				Foreground(lipgloss.Color("7")).
				Padding(0, 1)

	composeLabel = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")).
			Width(10)

	stateHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("12"))
)

// handlePauseKeys handles key events when the TUI is in pause mode.
func (m *Model) handlePauseKeys(msg tea.KeyMsg) tea.Cmd {
	switch m.pauseMode {
	case pauseMain:
		return m.handlePauseMainKeys(msg)
	case pauseCompose:
		return m.handleComposeKeys(msg)
	case pauseState:
		return m.handleStateViewKeys(msg)
	}
	return nil
}

func (m *Model) handlePauseMainKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "i":
		m.pauseMode = pauseCompose
		m.initCompose()
	case "v":
		m.pauseMode = pauseState
		m.stateScroll = 0
	case "r":
		m.paused = false
		m.pauseMode = pauseMain
		// Signal resume to the simulation
		select {
		case m.resumeCh <- struct{}{}:
		default:
		}
	case "q", "ctrl+c":
		return tea.Quit
	}
	return nil
}

func (m *Model) handleComposeKeys(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	switch key {
	case "esc":
		m.pauseMode = pauseMain
		return nil
	case "tab":
		m.compose.field = (m.compose.field + 1) % 4
		return nil
	case "shift+tab":
		m.compose.field = (m.compose.field + 3) % 4
		return nil
	}

	switch m.compose.field {
	case 0: // From — cycle with h/l
		switch key {
		case "h", "left":
			m.compose.from--
			if m.compose.from < 0 {
				m.compose.from = len(m.agents) - 1
			}
		case "l", "right":
			m.compose.from = (m.compose.from + 1) % len(m.agents)
		}
	case 1: // To — toggle agents with number keys or space (cycle through)
		switch key {
		case "space", " ":
			// Cycle cursor through agents and toggle
			if m.composeCursor < len(m.agents) {
				m.compose.to[m.composeCursor] = !m.compose.to[m.composeCursor]
			}
		case "h", "left":
			m.composeCursor--
			if m.composeCursor < 0 {
				m.composeCursor = len(m.agents) - 1
			}
		case "l", "right":
			m.composeCursor = (m.composeCursor + 1) % len(m.agents)
		}
	case 2: // Subject — text input
		switch key {
		case "backspace":
			if len(m.compose.subject) > 0 {
				m.compose.subject = m.compose.subject[:len(m.compose.subject)-1]
			}
		case "enter":
			m.compose.field = 3 // jump to body
		default:
			if len(key) == 1 {
				m.compose.subject += key
			}
		}
	case 3: // Body — text input
		switch key {
		case "backspace":
			if len(m.compose.body) > 0 {
				m.compose.body = m.compose.body[:len(m.compose.body)-1]
			}
		case "ctrl+s":
			// Send the email
			return m.sendComposedEmail()
		default:
			if key == "enter" {
				m.compose.body += "\n"
			} else if len(key) == 1 {
				m.compose.body += key
			}
		}
	}
	return nil
}

func (m *Model) handleStateViewKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.pauseMode = pauseMain
	case "j", "down":
		m.stateScroll++
	case "k", "up":
		if m.stateScroll > 0 {
			m.stateScroll--
		}
	case "q", "ctrl+c":
		return tea.Quit
	}
	return nil
}

func (m *Model) initCompose() {
	to := make([]bool, len(m.agents))
	m.compose = composeState{
		from:    0,
		to:      to,
		subject: "",
		body:    "",
		field:   0,
	}
	m.composeCursor = 0
}

func (m *Model) sendComposedEmail() tea.Cmd {
	// Build recipient list
	var recipients []string
	for i, selected := range m.compose.to {
		if selected && i < len(m.agents) {
			recipients = append(recipients, m.agents[i])
		}
	}
	if len(recipients) == 0 || m.compose.subject == "" {
		return nil
	}

	from := ""
	if m.compose.from >= 0 && m.compose.from < len(m.agents) {
		from = m.agents[m.compose.from]
	}

	// Send composed email through the injection channel
	select {
	case m.injectCh <- InjectEmail{
		From:    from,
		To:      recipients,
		Subject: m.compose.subject,
		Body:    m.compose.body,
	}:
	default:
	}

	m.appendLog(fmt.Sprintf("[INJECT] Email from %s to %s: %s", from, strings.Join(recipients, ","), m.compose.subject))
	m.pauseMode = pauseMain
	return nil
}

// --- Pause view rendering ---

func (m *Model) renderPauseOverlay() string {
	w := m.width
	if w < 1 {
		w = 1
	}

	var sections []string

	title := pauseTitleStyle.Width(w).Render("  ⏸  SIMULATION PAUSED")
	sections = append(sections, title)

	switch m.pauseMode {
	case pauseMain:
		sections = append(sections, m.renderPauseMenu())
	case pauseCompose:
		sections = append(sections, m.renderCompose())
	case pauseState:
		sections = append(sections, m.renderStateView())
	}

	return strings.Join(sections, "\n")
}

func (m *Model) renderPauseMenu() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  Round %d", m.round))
	if m.maxRounds > 0 {
		sb.WriteString(fmt.Sprintf("/%d", m.maxRounds))
	}
	sb.WriteString("\n\n")

	options := []struct{ key, desc string }{
		{"i", "Inject/compose email"},
		{"v", "View simulation state"},
		{"r", "Resume simulation"},
		{"q", "Quit"},
	}

	for _, opt := range options {
		sb.WriteString(fmt.Sprintf("  %s  %s\n",
			pauseKeyStyle.Render("["+opt.key+"]"),
			pauseDescStyle.Render(opt.desc),
		))
	}

	return borderStyle.Width(m.width - 2).Render(sb.String())
}

func (m *Model) renderCompose() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(sectionTitle.Render("  Compose Email"))
	sb.WriteString("\n\n")

	// From field
	fromName := "(none)"
	if m.compose.from >= 0 && m.compose.from < len(m.agents) {
		fromName = m.agents[m.compose.from]
	}
	fromStyle := composeFieldInactive
	if m.compose.field == 0 {
		fromStyle = composeFieldActive
	}
	sb.WriteString(fmt.Sprintf("  %s %s\n", composeLabel.Render("From:"), fromStyle.Render("< "+fromName+" >")))
	if m.compose.field == 0 {
		sb.WriteString(idleStyle.Render("         h/l to cycle agents"))
		sb.WriteString("\n")
	}

	// To field
	sb.WriteString(fmt.Sprintf("  %s ", composeLabel.Render("To:")))
	for i, name := range m.agents {
		selected := i < len(m.compose.to) && m.compose.to[i]
		cursor := m.compose.field == 1 && m.composeCursor == i

		label := name
		if selected {
			label = "[" + name + "]"
		}

		if cursor {
			sb.WriteString(composeFieldActive.Render(label))
		} else if selected {
			sb.WriteString(activeStyle.Render(label))
		} else {
			sb.WriteString(idleStyle.Render(label))
		}
		sb.WriteString(" ")
	}
	sb.WriteString("\n")
	if m.compose.field == 1 {
		sb.WriteString(idleStyle.Render("         h/l to move, space to toggle"))
		sb.WriteString("\n")
	}

	// Subject field
	subjectStyle := composeFieldInactive
	if m.compose.field == 2 {
		subjectStyle = composeFieldActive
	}
	subjectDisplay := m.compose.subject
	if m.compose.field == 2 {
		subjectDisplay += "█"
	}
	if subjectDisplay == "" && m.compose.field != 2 {
		subjectDisplay = "(empty)"
	}
	sb.WriteString(fmt.Sprintf("  %s %s\n", composeLabel.Render("Subject:"), subjectStyle.Render(subjectDisplay)))

	// Body field (with wrapping for multi-line)
	bodyStyle := composeFieldInactive
	if m.compose.field == 3 {
		bodyStyle = composeFieldActive
	}
	bodyDisplay := m.compose.body
	if m.compose.field == 3 {
		bodyDisplay += "█"
	}
	if bodyDisplay == "" && m.compose.field != 3 {
		bodyDisplay = "(empty)"
		sb.WriteString(fmt.Sprintf("  %s %s\n", composeLabel.Render("Body:"), bodyStyle.Render(bodyDisplay)))
	} else {
		// Wrap body text to fit window width
		bodyWidth := m.width - 16 // Account for label and padding
		if bodyWidth < 20 {
			bodyWidth = 20
		}
		bodyLines := wrapText(bodyDisplay, bodyWidth)
		// Show first line with label, rest indented
		if len(bodyLines) > 0 {
			sb.WriteString(fmt.Sprintf("  %s %s\n", composeLabel.Render("Body:"), bodyStyle.Render(bodyLines[0])))
			for _, line := range bodyLines[1:] {
				sb.WriteString(fmt.Sprintf("  %s %s\n", strings.Repeat(" ", 10), bodyStyle.Render(line)))
			}
		}
	}

	sb.WriteString("\n")
	sb.WriteString(idleStyle.Render("  Tab: next field  Ctrl+S: send  Esc: back"))
	sb.WriteString("\n")

	return borderStyle.Width(m.width - 2).Render(sb.String())
}

func (m *Model) renderStateView() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(stateHeaderStyle.Render("  Simulation State"))
	sb.WriteString("\n\n")

	// Round info
	sb.WriteString(fmt.Sprintf("  Round: %d", m.round))
	if m.maxRounds > 0 {
		sb.WriteString(fmt.Sprintf("/%d", m.maxRounds))
	}
	sb.WriteString("\n\n")

	// Agent summary
	sb.WriteString(stateHeaderStyle.Render("  Agents"))
	sb.WriteString("\n")

	var lines []string
	maxWidth := m.width - 8 // Account for border and padding
	if maxWidth < 1 {
		maxWidth = 1
	}

	for _, name := range m.agents {
		info := m.agentStatus[name]
		apStr := ""
		if info.MaxAP > 0 {
			apStr = fmt.Sprintf("  AP: %d/%d", info.AP, info.MaxAP)
		}
		tokStr := ""
		if info.Tokens > 0 {
			tokStr = fmt.Sprintf("  %.1fk tok", float64(info.Tokens)/1000)
		}
		line := fmt.Sprintf("  %-18s %-7s%s%s", name, info.Status, apStr, tokStr)
		// Truncate if too long
		if len(line) > maxWidth {
			line = line[:maxWidth-3] + "..."
		}
		lines = append(lines, line)
	}

	// Snapshot data from pause_ack
	if m.pauseSnapshot != nil {
		for _, ai := range m.pauseSnapshot.Agents {
			pStr := fmt.Sprintf("  patience: %d/100", ai.Patience)
			for i, line := range lines {
				if strings.Contains(line, ai.Name) {
					lines[i] = line + pStr
					break
				}
			}
		}
	}

	// Event log summary
	lines = append(lines, "")
	lines = append(lines, stateHeaderStyle.Render("  Recent Events"))
	start := 0
	if len(m.eventLog) > 10 {
		start = len(m.eventLog) - 10
	}
	for _, entry := range m.eventLog[start:] {
		display := "  " + entry
		// Truncate long entries
		if len(display) > maxWidth {
			display = display[:maxWidth-3] + "..."
		}
		lines = append(lines, display)
	}

	// Apply scroll
	maxVisible := m.height - 8
	if maxVisible < 5 {
		maxVisible = 5
	}
	maxScroll := len(lines) - maxVisible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.stateScroll < 0 {
		m.stateScroll = 0
	}
	if m.stateScroll > maxScroll {
		m.stateScroll = maxScroll
	}
	end := m.stateScroll + maxVisible
	if end > len(lines) {
		end = len(lines)
	}

	for _, line := range lines[m.stateScroll:end] {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(idleStyle.Render("  j/k: scroll  Esc: back"))
	sb.WriteString("\n")

	return borderStyle.Width(m.width - 2).Render(sb.String())
}
