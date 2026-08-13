package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/yanpgwang/mango-terminal/internal/timeline"
)

func (m Model) View() tea.View {
	content := m.inboxView()
	if m.view == viewAttached {
		content = m.attachedView()
	}
	if m.overlay != overlayNone && m.width > 0 && m.height > 0 {
		modal := m.overlayView(m.width)
		canvas := lipgloss.NewCanvas(m.width, m.height)
		canvas.Compose(lipgloss.NewLayer(content))
		canvas.Compose(lipgloss.NewLayer(modal).
			X(max(0, (m.width-lipgloss.Width(modal))/2)).
			Y(max(0, (m.height-lipgloss.Height(modal))/2)).
			Z(10))
		content = canvas.Render()
	}
	view := tea.NewView(content)
	view.AltScreen = true
	view.WindowTitle = "Mango Terminal"
	view.MouseMode = tea.MouseModeCellMotion
	return view
}

func (m Model) inboxView() string {
	width, height := max(44, m.width), max(16, m.height)
	contentWidth := min(86, max(40, width-8))
	lines := []string{
		m.brand(contentWidth),
		"",
		m.theme.title.Render("Cloud Sessions"),
		m.theme.dim.Render("Attach to durable Agent work already running in Mango."),
		"",
	}
	if m.err != nil {
		lines = append(lines, m.theme.error.Render("Connection failed  "+m.err.Error()), "")
	}
	if m.filtering || m.filter != "" {
		cursor := ""
		if m.filtering {
			cursor = "▏"
		}
		lines = append(lines, m.theme.accentText.Render("/ ")+m.filter+cursor, divider(m.theme, contentWidth), "")
	}
	sessions := m.filteredSessions()
	if m.loading && len(sessions) == 0 {
		lines = append(lines, m.theme.dim.Render(m.spinner.View()+" Connecting to Mango"))
	}
	if !m.loading && len(sessions) == 0 {
		lines = append(lines, m.theme.dim.Render("No durable Sessions found."))
	}
	for index, session := range sessions {
		selected := index == m.inboxCursor
		marker, titleStyle := "  ", m.theme.copy
		if selected {
			marker, titleStyle = m.theme.accentText.Render("› "), m.theme.selected
		}
		title := truncate(first(session.Title, session.Agent.Name, session.ID), contentWidth-18)
		status := sessionStatus(m.theme, session.Status)
		left := marker + titleStyle.Render(title)
		gap := max(2, contentWidth-lipgloss.Width(left)-lipgloss.Width(status))
		lines = append(lines,
			left+strings.Repeat(" ", gap)+status,
			"  "+m.theme.dim.Render(first(session.Agent.Name, "Agent")+"  "+shortID(session.ID)+"  "+relativeTime(session.CreatedAt)),
			"",
		)
	}
	footer := m.theme.key.Render("enter") + " attach  " +
		m.theme.key.Render("/") + " filter  " +
		m.theme.key.Render("r") + " refresh  " +
		m.theme.key.Render("ctrl+q") + " quit"
	lines = append(lines, "", divider(m.theme, contentWidth), footer)
	body := lipgloss.NewStyle().Width(contentWidth).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, body)
}

func (m Model) brand(width int) string {
	name := lipgloss.NewStyle().Foreground(m.theme.accent).Bold(true).Render("M A N G O")
	tagline := m.theme.dim.Render("MANAGED AGENT TERMINAL")
	gap := max(2, width-lipgloss.Width(name)-lipgloss.Width(tagline))
	return name + strings.Repeat(" ", gap) + tagline
}

func (m Model) attachedView() string {
	if m.session == nil {
		return ""
	}
	width, height := max(44, m.width), max(16, m.height)
	sidebarWidth, mainWidth := paneWidths(width)
	main := m.mainPane(mainWidth, height)
	if sidebarWidth == 0 {
		return lipgloss.NewStyle().Width(width).Height(height).Render(main)
	}
	sidebar := m.sidebarView(sidebarWidth, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main)
}

func (m Model) mainPane(width, height int) string {
	header := m.headerView(width)
	composer := m.composerView(width)
	footer := m.statusBar(width)
	reserved := lipgloss.Height(header) + lipgloss.Height(composer) + lipgloss.Height(footer)
	viewportHeight := max(5, height-reserved)
	transcript := lipgloss.NewStyle().
		Width(max(20, width-4)).
		Height(viewportHeight).
		Padding(0, 2).
		Render(m.viewport.View())
	return lipgloss.NewStyle().Width(width).Height(height).Render(
		header + transcript + composer + footer,
	)
}

func (m Model) headerView(width int) string {
	title := truncate(first(m.session.Title, m.session.Agent.Name, m.session.ID), max(20, width-24))
	status := sessionStatus(m.theme, m.session.Status)
	left := m.theme.title.Render(title)
	gap := max(2, width-4-lipgloss.Width(left)-lipgloss.Width(status))
	thread := "Waiting for primary Thread"
	if m.threadCursor >= 0 && m.threadCursor < len(m.threads) {
		selected := m.threads[m.threadCursor]
		role := "specialist"
		if selected.Primary() {
			role = "coordinator"
		}
		thread = first(selected.Agent.Name, "Agent") + "  ·  " + role + "  ·  " + shortID(selected.ID)
	}
	line := left + strings.Repeat(" ", gap) + status
	return lipgloss.NewStyle().
		Width(width).
		Padding(1, 2, 0, 2).
		BorderBottom(true).
		BorderForeground(m.theme.border).
		Render(line + "\n" + m.theme.dim.Render(thread))
}

func (m Model) composerView(width int) string {
	border := m.theme.border
	label := "Message the coordinator"
	if m.focus == focusComposer {
		border = m.theme.accent
		label = m.theme.accentText.Render(label)
	} else {
		label = m.theme.dim.Render(label + "  ·  press i to focus")
	}
	innerWidth := max(20, width-6)
	box := lipgloss.NewStyle().
		Width(innerWidth).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Render(m.composer.View())
	return lipgloss.NewStyle().Width(width).Padding(0, 2).Render(label + "\n" + box)
}

func (m Model) statusBar(width int) string {
	left := m.theme.dim.Render(m.status)
	if m.loading {
		left = m.theme.dim.Render(m.spinner.View() + " syncing")
	}
	if m.err != nil {
		left = m.theme.error.Render(truncate(m.err.Error(), max(18, width/2)))
	}
	if len(m.pending) > 0 {
		left = m.theme.warn.Render(fmt.Sprintf("%d action%s waiting", len(m.pending), plural(len(m.pending))))
	}
	right := m.theme.dim.Render("tab Agent  esc observe  ctrl+k commands  ctrl+q quit")
	gap := max(1, width-4-lipgloss.Width(left)-lipgloss.Width(right))
	if gap == 1 && width < 90 {
		right = m.theme.dim.Render("ctrl+k commands")
		gap = max(1, width-4-lipgloss.Width(left)-lipgloss.Width(right))
	}
	return lipgloss.NewStyle().Width(width).Padding(0, 2, 1, 2).Render(left + strings.Repeat(" ", gap) + right)
}

func (m Model) sidebarView(width, height int) string {
	inner := max(16, width-4)
	lines := []string{
		m.theme.accentText.Render("MANGO"),
		m.theme.dim.Render("managed agent terminal"),
		"",
		m.theme.title.Render("Agent Threads"),
	}
	for index, thread := range m.threads {
		selected := index == m.threadCursor
		marker, titleStyle := "  ", m.theme.copy
		if selected {
			marker, titleStyle = m.theme.accentText.Render("› "), m.theme.selected
		}
		name := truncate(first(thread.Agent.Name, "Agent"), inner-8)
		activity := ""
		if count := m.unread[thread.ID]; count > 0 {
			activity = m.theme.accentText.Render(fmt.Sprintf(" +%d", count))
		}
		lines = append(lines,
			marker+titleStyle.Render(name)+activity,
			"  "+threadStatus(m.theme, thread.Status)+m.theme.dim.Render("  "+threadRole(thread)),
		)
	}
	lines = append(lines,
		"",
		m.theme.title.Render("Cloud Session"),
		m.theme.dim.Render(shortID(m.session.ID)),
		m.theme.dim.Render(first(m.session.Agent.Model.ID, "model unavailable")),
		"",
		m.theme.title.Render("Usage"),
		m.theme.dim.Render(fmt.Sprintf("%s in  ·  %s out", commas(m.session.Usage.InputTokens), commas(m.session.Usage.OutputTokens))),
		m.theme.dim.Render(fmt.Sprintf("%.1fs active", m.session.Usage.ActiveSeconds)),
	)
	if len(m.pending) > 0 {
		lines = append(lines,
			"",
			m.theme.warn.Render(fmt.Sprintf("Pending actions  %d", len(m.pending))),
			m.theme.dim.Render("ctrl+p to review"),
		)
	}
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(1, 2).
		BorderRight(true).
		BorderForeground(m.theme.border).
		Render(strings.Join(lines, "\n"))
}

func (m *Model) renderTimeline() {
	threadID := m.currentThreadID()
	if threadID == "" || m.threadCursor < 0 || m.threadCursor >= len(m.threads) {
		m.viewport.SetContent(m.theme.dim.Render("Waiting for the primary Thread…"))
		return
	}
	items := timeline.Project(m.threads[m.threadCursor], m.events[threadID])
	m.timeline.setItems(items)
	m.timeline.width = max(20, m.viewport.Width()-2)
	m.timeline.now = time.Now()
	content := m.timeline.render(m.theme)
	if current := m.previews[threadID]; current != nil {
		label := "Working"
		if current.typ == "agent.thinking" {
			label = "Thinking"
		}
		live := m.theme.dim.Render(m.spinner.View() + " " + label)
		if strings.TrimSpace(current.content) != "" {
			live += "\n" + markdown(current.content, m.timeline.width)
		}
		if content != "" {
			content += "\n\n"
		}
		content += live
	}
	wasBottom := m.viewport.AtBottom()
	m.viewport.SetContent(content)
	if wasBottom || m.viewport.YOffset() == 0 {
		m.viewport.GotoBottom()
	}
}

func markdown(content string, width int) string {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(max(20, width-2)),
	)
	if err != nil {
		return content
	}
	rendered, err := renderer.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSpace(rendered)
}

func paneWidths(width int) (int, int) {
	if width < 92 {
		return 0, width
	}
	sidebar := min(34, max(26, width/4))
	return sidebar, max(30, width-sidebar)
}

func divider(t theme, width int) string {
	return t.dim.Render(strings.Repeat("─", max(1, width)))
}

func sessionStatus(t theme, status string) string {
	switch status {
	case "running":
		return t.warn.Render("RUNNING")
	case "failed", "error":
		return t.error.Render(strings.ToUpper(status))
	case "idle":
		return t.ok.Render("IDLE")
	default:
		return t.dim.Render(strings.ToUpper(first(status, "UNKNOWN")))
	}
}

func threadStatus(t theme, status string) string {
	switch status {
	case "running":
		return t.warn.Render("working")
	case "failed", "error", "terminated":
		return t.error.Render(status)
	case "idle":
		return t.dim.Render("idle")
	default:
		return t.dim.Render(first(status, "unknown"))
	}
}

func threadRole(thread interface{ Primary() bool }) string {
	if thread.Primary() {
		return "coordinator"
	}
	return "specialist"
}

func first(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func truncate(value string, width int) string {
	if width <= 0 || len([]rune(value)) <= width {
		return value
	}
	runes := []rune(value)
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

func commas(value int64) string {
	text := fmt.Sprintf("%d", value)
	for index := len(text) - 3; index > 0; index -= 3 {
		text = text[:index] + "," + text[index:]
	}
	return text
}

func relativeTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	delta := time.Since(value)
	if delta < time.Minute {
		return "just now"
	}
	if delta < time.Hour {
		return fmt.Sprintf("%dm ago", int(delta.Minutes()))
	}
	if delta < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(delta.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(delta.Hours()/24))
}

func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
