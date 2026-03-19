package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dntatme/agents/capabilities/company"
)

// KanbanTask is one card derived from structured company.Task data.
type KanbanTask struct {
	ID          string
	Title       string
	Description string
	Assignee    string
	Priority    string
	Reviewer    string
	NotesShort  string
	DependsOn   string
	Deadline    int // target simulation round (0 = none)
}

// KanbanColumn is a workflow column (fixed status).
type KanbanColumn struct {
	StatusKey string
	Title     string
	Tasks     []KanbanTask
}

var kanbanStatusOrder = []string{
	"todo", "in_progress", "awaiting_review", "needs_changes", "approved", "done", "blocked",
}

// taskBoardFileMsg carries initial load of shared/tasks.json.
type taskBoardFileMsg struct {
	jsonRaw string
}

func loadTaskBoardFromDisk(root string) tea.Cmd {
	return func() tea.Msg {
		if root == "" {
			return taskBoardFileMsg{jsonRaw: ""}
		}
		p := filepath.Join(root, company.TasksJSONRelPath)
		b, err := os.ReadFile(p)
		if err != nil {
			return taskBoardFileMsg{jsonRaw: ""}
		}
		return taskBoardFileMsg{jsonRaw: string(b)}
	}
}

func statusColumnTitle(key string) string {
	parts := strings.Split(key, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

func companyTaskToKanban(t company.Task) KanbanTask {
	notes := t.Notes
	if len(notes) > 80 {
		notes = notes[:77] + "…"
	}
	desc := strings.TrimSpace(t.Description)
	if len(desc) > 100 {
		desc = desc[:97] + "…"
	}
	return KanbanTask{
		ID:          t.ID,
		Title:       t.Title,
		Description: desc,
		Assignee:    t.Assignee,
		Priority:    t.Priority,
		Reviewer:    t.Reviewer,
		NotesShort:  notes,
		DependsOn:   t.DependsOn,
		Deadline:    t.Deadline,
	}
}

// buildKanbanColumns maps tasks into a fixed set of status columns (empty columns still shown).
func buildKanbanColumns(tasks []company.Task) []KanbanColumn {
	by := make(map[string][]KanbanTask)
	for _, t := range tasks {
		key := strings.TrimSpace(strings.ToLower(t.Status))
		if key == "" {
			key = "todo"
		}
		by[key] = append(by[key], companyTaskToKanban(t))
	}
	out := make([]KanbanColumn, 0, len(kanbanStatusOrder))
	for _, key := range kanbanStatusOrder {
		colTasks := by[key]
		if colTasks == nil {
			colTasks = []KanbanTask{}
		}
		out = append(out, KanbanColumn{
			StatusKey: key,
			Title:     statusColumnTitle(key),
			Tasks:     colTasks,
		})
	}
	return out
}

func (m *Model) applyKanbanTasksJSON(raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		m.kanbanBoard = buildKanbanColumns(nil)
		return
	}
	var payload struct {
		Tasks []company.Task `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		m.kanbanBoard = buildKanbanColumns(nil)
		return
	}
	m.kanbanBoard = buildKanbanColumns(payload.Tasks)
}

// nonEmptyColumns returns status columns that have at least one task (workflow order preserved).
func nonEmptyColumns(cols []KanbanColumn) []KanbanColumn {
	var out []KanbanColumn
	for _, c := range cols {
		if len(c.Tasks) > 0 {
			out = append(out, c)
		}
	}
	return out
}

func truncateRunes(s string, max int) string {
	if max < 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

// pickHorizontalLayout finds how many columns fit side-by-side without exceeding inner width.
// outerW is the lipgloss block width per column (including border); innerW is usable text width inside the border.
func pickHorizontalLayout(inner, gap, numNonEmpty int) (nShow, outerW, innerW int, ok bool) {
	if numNonEmpty < 1 {
		return 0, 0, 0, false
	}
	fs := kanbanColBorder.GetHorizontalFrameSize()
	const minInner = 12
	for n := numNonEmpty; n >= 1; n-- {
		space := inner - (n-1)*gap
		if space < n {
			continue
		}
		outer := space / n
		in := outer - fs
		if in >= minInner {
			return n, outer, in, true
		}
	}
	return 0, 0, 0, false
}

func (m *Model) kanbanContentInnerWidth() int {
	w := m.width - 2
	if w < 24 {
		w = 24
	}
	return w - 4
}

// kanbanPanelMaxHeight is the maximum lines the whole Board tab panel may occupy
// (header + footer/hint are drawn separately in View).
func (m *Model) kanbanPanelMaxHeight() int {
	hdr := lipgloss.Height(m.renderHeader())
	var tail int
	if m.done {
		tail = lipgloss.Height(m.renderFooter())
	} else {
		tail = 1 // status hint row under the panel
	}
	v := m.height - hdr - tail
	if v < 6 {
		v = 6
	}
	return v
}

// finalizeKanbanPanel wraps inner content in the board border and clips to the terminal viewport.
func (m *Model) finalizeKanbanPanel(inner string) string {
	inner = strings.TrimRight(inner, "\n")
	return borderStyle.Width(m.width - 2).MaxHeight(m.kanbanPanelMaxHeight()).Render(inner)
}

// clampKanbanScroll keeps horizontal column offset in range for non-empty columns only.
func (m *Model) clampKanbanScroll() {
	ne := nonEmptyColumns(m.kanbanBoard)
	if len(ne) == 0 {
		m.kanbanScrollH = 0
		return
	}
	inner := m.kanbanContentInnerWidth()
	nShow, _, _, ok := pickHorizontalLayout(inner, 1, len(ne))
	if !ok {
		m.kanbanScrollH = 0
		return
	}
	maxScroll := len(ne) - nShow
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.kanbanScrollH > maxScroll {
		m.kanbanScrollH = maxScroll
	}
	if m.kanbanScrollH < 0 {
		m.kanbanScrollH = 0
	}
}

var kanbanColBorder = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("62")).
	Padding(0, 1)

func priorityStyle(p string) lipgloss.Style {
	switch strings.ToLower(p) {
	case "high":
		return energyLowStyle
	case "low":
		return idleStyle
	default:
		return energyMidStyle
	}
}

func (m *Model) renderKanbanTab() string {
	m.kanbanStacked = false
	inner := m.kanbanContentInnerWidth()

	var sb strings.Builder
	sb.WriteString(sectionTitle.Render("Task board (kanban)"))
	sb.WriteString("\n")
	sb.WriteString(idleStyle.Render("  Status columns · shared/tasks.json · empty statuses hidden"))
	sb.WriteString("\n\n")

	if m.kanbanBoard == nil {
		return m.finalizeKanbanPanel(sb.String() + idleStyle.Render(
			"  Loading task board…",
		))
	}

	ne := nonEmptyColumns(m.kanbanBoard)
	if len(ne) == 0 {
		return m.finalizeKanbanPanel(sb.String() + idleStyle.Render(
			"  No tasks on the board.",
		))
	}

	const gap = 1
	nShow, outerW, innerW, horizOK := pickHorizontalLayout(inner, gap, len(ne))
	if !horizOK {
		m.kanbanStacked = true
		sb.WriteString(idleStyle.Render("  Stacked layout — terminal too narrow for columns side-by-side."))
		sb.WriteString("\n")
		return m.finalizeKanbanPanel(m.finishKanbanStacked(sb, inner))
	}

	m.clampKanbanScroll()
	ne = nonEmptyColumns(m.kanbanBoard)
	nShow, outerW, innerW, _ = pickHorizontalLayout(inner, gap, len(ne))
	if m.kanbanScrollH+nShow > len(ne) {
		m.kanbanScrollH = max(0, len(ne)-nShow)
	}
	visible := ne[m.kanbanScrollH : m.kanbanScrollH+nShow]

	if len(ne) > nShow {
		ind := fmt.Sprintf("  Columns %d–%d of %d  (h/l)", m.kanbanScrollH+1, m.kanbanScrollH+nShow, len(ne))
		sb.WriteString(idleStyle.Render(ind))
		sb.WriteString("\n")
	} else {
		sb.WriteString(idleStyle.Render("  All columns with tasks are visible side-by-side."))
		sb.WriteString("\n")
	}

	prefixH := lipgloss.Height(sb.String())
	innerBudget := m.kanbanPanelMaxHeight() - borderStyle.GetVerticalFrameSize() - prefixH
	if innerBudget < 8 {
		innerBudget = 8
	}
	maxColOuterH := innerBudget

	parts := make([]string, len(visible))
	for i := range visible {
		parts[i] = m.renderKanbanColumnHoriz(visible[i], innerW, outerW, maxColOuterH)
	}
	board := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	sb.WriteString(board)

	return m.finalizeKanbanPanel(sb.String())
}

// finishKanbanStacked builds inner panel text (no outer border). Caller wraps with finalizeKanbanPanel.
func (m *Model) finishKanbanStacked(sb strings.Builder, contentW int) string {
	lines := m.kanbanStackedLines(contentW)
	innerBudget := m.kanbanPanelMaxHeight() - borderStyle.GetVerticalFrameSize()
	prefixH := lipgloss.Height(sb.String())
	room := innerBudget - prefixH
	if room < 6 {
		room = 6
	}
	maxVis := room
	if len(lines) > maxVis {
		maxVis = room - 1
		if maxVis < 4 {
			maxVis = 4
		}
	}
	maxScroll := len(lines) - maxVis
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.kanbanScrollV > maxScroll {
		m.kanbanScrollV = maxScroll
	}
	if m.kanbanScrollV < 0 {
		m.kanbanScrollV = 0
	}
	end := min(m.kanbanScrollV+maxVis, len(lines))
	if maxScroll > 0 {
		sb.WriteString(idleStyle.Render(fmt.Sprintf("  Rows %d–%d of %d  (j/k scroll)", m.kanbanScrollV+1, end, len(lines))))
		sb.WriteString("\n")
	}
	sb.WriteString(strings.Join(lines[m.kanbanScrollV:end], "\n"))
	return sb.String()
}

func (m *Model) kanbanStackedLines(contentW int) []string {
	ne := nonEmptyColumns(m.kanbanBoard)
	var lines []string
	sepW := min(contentW, 56)
	if sepW < 12 {
		sepW = 12
	}
	sep := idleStyle.Render(strings.Repeat("─", sepW))

	for _, col := range ne {
		hdr := fmt.Sprintf("%s (%d)", col.Title, len(col.Tasks))
		lines = append(lines, sectionTitle.Render(truncateRunes(hdr, contentW)))
		lines = append(lines, sep)
		const maxTasks = 40
		tasks := col.Tasks
		if len(tasks) > maxTasks {
			for _, t := range tasks[:maxTasks] {
				lines = append(lines, stackedCardLines(t, contentW)...)
				lines = append(lines, "")
			}
			lines = append(lines, idleStyle.Render(fmt.Sprintf("  +%d more tasks in this column…", len(tasks)-maxTasks)))
		} else {
			for _, t := range tasks {
				lines = append(lines, stackedCardLines(t, contentW)...)
				lines = append(lines, "")
			}
		}
	}
	return lines
}

// stackedCardLines renders a task as at most maxStackedCardLines rows so one card cannot consume the viewport.
func stackedCardLines(t KanbanTask, w int) []string {
	const maxStackedCardLines = 8
	raw := strings.Split(renderKanbanCardStacked(t, w), "\n")
	if len(raw) <= maxStackedCardLines {
		return raw
	}
	trimmed := make([]string, maxStackedCardLines-1)
	copy(trimmed, raw[:maxStackedCardLines-1])
	return append(trimmed, idleStyle.Render("…"))
}

func renderKanbanCardStacked(t KanbanTask, w int) string {
	var b strings.Builder
	b.WriteString(activeStyle.Render(truncateRunes(t.ID, w)))
	b.WriteString("\n")
	for _, ln := range wrapKanbanText(t.Title, w) {
		b.WriteString(detailContent.Render(truncateRunes(ln, w)))
		b.WriteString("\n")
	}
	if strings.TrimSpace(t.Description) != "" {
		b.WriteString(idleStyle.Render(truncateRunes(t.Description, w)))
		b.WriteString("\n")
	}
	meta := abbreviateAgent(t.Assignee)
	if t.Priority != "" {
		meta += " · " + t.Priority
	}
	b.WriteString(priorityStyle(t.Priority).Render(truncateRunes(meta, w)))
	b.WriteString("\n")
	if t.Reviewer != "" {
		b.WriteString(idleStyle.Render("→ " + truncateRunes(t.Reviewer, w-2)))
		b.WriteString("\n")
	}
	if t.DependsOn != "" {
		b.WriteString(idleStyle.Render("↳ " + truncateRunes(t.DependsOn, w-2)))
		b.WriteString("\n")
	}
	if t.NotesShort != "" {
		b.WriteString(idleStyle.Render(truncateRunes(t.NotesShort, w)))
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderKanbanCardHorizontal(t KanbanTask, innerW int) string {
	var sb strings.Builder
	sb.WriteString(activeStyle.Render(truncateRunes(t.ID, innerW)))
	sb.WriteString("\n")
	sb.WriteString(detailContent.Render(truncateRunes(t.Title, innerW)))
	sb.WriteString("\n")
	meta := abbreviateAgent(t.Assignee)
	if t.Priority != "" {
		meta += " · " + t.Priority
	}
	if t.Reviewer != "" {
		meta += " → " + abbreviateAgent(t.Reviewer)
	}
	if t.Deadline > 0 {
		meta += fmt.Sprintf(" · R%d", t.Deadline)
	}
	sb.WriteString(priorityStyle(t.Priority).Render(truncateRunes(meta, innerW)))
	return sb.String()
}

func (m *Model) renderKanbanColumnHoriz(col KanbanColumn, innerW, outerW, maxColOuterH int) string {
	fsY := kanbanColBorder.GetVerticalFrameSize()
	innerH := maxColOuterH - fsY
	if innerH < 6 {
		innerH = 6
	}
	// Lines inside column: title + count (2) + per card 3 lines + (cards-1) separators (1 line each).
	// innerH >= 2 + 4*c - 1  =>  c <= (innerH - 1) / 4
	maxCards := (innerH - 1) / 4
	if maxCards < 1 {
		maxCards = 1
	}
	if maxCards > 8 {
		maxCards = 8
	}

	header := strings.ReplaceAll(col.Title, "_", " ")
	titleLine := sectionTitle.Render(truncateRunes(header, innerW))
	var b strings.Builder
	b.WriteString(titleLine)
	b.WriteString("\n")
	b.WriteString(idleStyle.Render(fmt.Sprintf("%d tasks", len(col.Tasks))))
	b.WriteString("\n")

	shown := col.Tasks
	extra := 0
	if len(shown) > maxCards {
		extra = len(shown) - maxCards
		shown = shown[:maxCards]
	}

	for i, t := range shown {
		if i > 0 {
			b.WriteString(idleStyle.Render(strings.Repeat("·", min(innerW, 8))))
			b.WriteString("\n")
		}
		b.WriteString(renderKanbanCardHorizontal(t, innerW))
		b.WriteString("\n")
	}
	if extra > 0 {
		b.WriteString(idleStyle.Render(fmt.Sprintf("+%d more…", extra)))
	}

	body := strings.TrimRight(b.String(), "\n")
	return kanbanColBorder.Width(outerW).MaxHeight(maxColOuterH).Render(body)
}

func wrapKanbanText(s string, width int) []string {
	if width < 8 {
		width = 8
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{""}
	}
	r := []rune(s)
	if len(r) <= width {
		return []string{s}
	}
	prefix := string(r[:width])
	breakpoint := width
	if idx := strings.LastIndex(prefix, " "); idx > width/3 {
		breakpoint = idx
	}
	first := strings.TrimSpace(string(r[:breakpoint]))
	rest := strings.TrimSpace(string(r[breakpoint:]))
	rr := []rune(rest)
	if len(rr) > width {
		rest = string(rr[:width-1]) + "…"
	}
	return []string{first, rest}
}
