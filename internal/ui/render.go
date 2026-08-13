package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/yanpgwang/mango-terminal/internal/timeline"
)

var (
	accent       = lipgloss.Color("6")
	muted        = lipgloss.Color("8")
	warning      = lipgloss.Color("3")
	danger       = lipgloss.Color("1")
	primaryStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)
	mutedStyle   = lipgloss.NewStyle().Foreground(muted)
	warningStyle = lipgloss.NewStyle().Foreground(warning)
	errorStyle   = lipgloss.NewStyle().Foreground(danger)
	headingStyle = lipgloss.NewStyle().Bold(true)
)

func (m Model) View() tea.View {
	content := m.inboxView()
	if m.view == viewAttached {
		content = m.attachedView()
	}
	view := tea.NewView(content)
	view.AltScreen = true
	view.WindowTitle = "Mango Terminal"
	return view
}

func (m Model) inboxView() string {
	width := max(40, m.width)
	lines := []string{
		primaryStyle.Render("Mango Terminal"),
		mutedStyle.Render("Attach to durable agents running in the cloud."),
		"",
	}
	if m.err != nil {
		lines = append(lines, errorStyle.Render(m.err.Error()), "")
	}
	if m.filtering || m.filter != "" {
		cursor := ""
		if m.filtering {
			cursor = "_"
		}
		lines = append(lines, mutedStyle.Render("Filter: ")+m.filter+cursor, "")
	}
	sessions := m.filteredSessions()
	if m.loading && len(sessions) == 0 {
		lines = append(lines, mutedStyle.Render("Connecting to Mango…"))
	}
	if !m.loading && len(sessions) == 0 {
		lines = append(lines, mutedStyle.Render("No durable Sessions."))
	}
	for index, session := range sessions {
		pointer := "  "
		title := first(session.Title, session.Agent.Name, session.ID)
		style := lipgloss.NewStyle()
		if index == m.inboxCursor {
			pointer, style = "› ", primaryStyle
		}
		lines = append(lines,
			style.Render(pointer+truncate(title, width-4)),
			mutedStyle.Render("  "+session.Status+" · "+first(session.Agent.Name, "Agent")+" · "+shortID(session.ID)),
			"",
		)
	}
	help := "↑/↓ select  enter attach  / filter  r refresh  ctrl+q quit"
	lines = append(lines, "", mutedStyle.Render(help))
	return lipgloss.NewStyle().Padding(1, 3).MaxWidth(width).Render(strings.Join(lines, "\n"))
}

func (m Model) attachedView() string {
	if m.session == nil {
		return ""
	}
	sidebarWidth, mainWidth := paneWidths(m.width)
	header := headingStyle.Render(first(m.session.Title, m.session.Agent.Name, m.session.ID)) +
		"  " + statusStyle(m.session.Status)
	if m.loading {
		header += mutedStyle.Render("  syncing")
	}
	if m.err != nil {
		header += "  " + errorStyle.Render(truncate(m.err.Error(), max(20, mainWidth/2)))
	}
	threadLabel := "No Thread"
	if m.threadCursor >= 0 && m.threadCursor < len(m.threads) {
		thread := m.threads[m.threadCursor]
		role := "child"
		if thread.Primary() {
			role = "primary"
		}
		threadLabel = first(thread.Agent.Name, "Agent") + " · " + role + " · " + shortID(thread.ID)
	}
	main := lipgloss.NewStyle().Width(mainWidth).Render(strings.Join([]string{
		header,
		mutedStyle.Render(threadLabel),
		divider(mainWidth),
		m.viewport.View(),
		divider(mainWidth),
		m.composer.View(),
		mutedStyle.Render("esc inbox  tab/⇧tab Thread  ctrl+x interrupt  pgup/dn scroll  ctrl+q quit"),
	}, "\n"))
	if sidebarWidth == 0 {
		return lipgloss.NewStyle().Padding(0, 1).Render(main)
	}
	sidebar := lipgloss.NewStyle().Width(sidebarWidth).Render(m.sidebarView(sidebarWidth))
	return lipgloss.NewStyle().Padding(0, 1).Render(
		lipgloss.JoinHorizontal(lipgloss.Top, sidebar, "  ", main),
	)
}

func (m Model) sidebarView(width int) string {
	lines := []string{headingStyle.Render("Agent Threads")}
	for index, thread := range m.threads {
		marker := "  "
		if index == m.threadCursor {
			marker = "› "
		}
		activity := ""
		if count := m.unread[thread.ID]; count > 0 {
			activity = fmt.Sprintf(" +%d", count)
		}
		role := first(thread.Agent.Name, "Agent")
		line := marker + statusGlyph(thread.Status) + " " + role + activity
		if index == m.threadCursor {
			line = primaryStyle.Render(truncate(line, width))
		} else {
			line = truncate(line, width)
		}
		lines = append(lines, line)
	}
	lines = append(lines,
		"",
		headingStyle.Render("Cloud Session"),
		mutedStyle.Render("  "+shortID(m.session.ID)),
		mutedStyle.Render("  "+first(m.session.Agent.Model.ID, "model unavailable")),
		"",
		headingStyle.Render("Usage"),
		mutedStyle.Render(fmt.Sprintf("  %s in · %s out",
			commas(m.session.Usage.InputTokens), commas(m.session.Usage.OutputTokens))),
		mutedStyle.Render(fmt.Sprintf("  %.1fs active", m.session.Usage.ActiveSeconds)),
		"",
		mutedStyle.Render(truncate(m.status, width)),
	)
	return strings.Join(lines, "\n")
}

func (m *Model) renderTimeline() {
	threadID := m.currentThreadID()
	if threadID == "" || m.threadCursor < 0 || m.threadCursor >= len(m.threads) {
		m.viewport.SetContent(mutedStyle.Render("Waiting for the primary Thread…"))
		return
	}
	items := timeline.Project(m.threads[m.threadCursor], m.events[threadID])
	lines := make([]string, 0, len(items)*2)
	for _, item := range items {
		lines = append(lines, renderItem(item, m.viewport.Width()), "")
	}
	if current := m.previews[threadID]; current != nil {
		label := "Streaming"
		if current.typ == "agent.thinking" {
			label = "Thinking"
		}
		body := current.content
		if body == "" {
			body = "working…"
		}
		lines = append(lines, warningStyle.Render(label)+"\n"+body+mutedStyle.Render(" ▍"), "")
	}
	if len(lines) == 0 {
		lines = append(lines, mutedStyle.Render("No visible timeline activity."))
	}
	wasBottom := m.viewport.AtBottom()
	m.viewport.SetContent(strings.Join(lines, "\n"))
	if wasBottom || m.viewport.YOffset() == 0 {
		m.viewport.GotoBottom()
	}
}

func renderItem(item timeline.Item, width int) string {
	label := headingStyle.Render(item.Label)
	switch item.Kind {
	case timeline.KindUser:
		label = primaryStyle.Render(item.Label)
	case timeline.KindThinking, timeline.KindStatus:
		label = mutedStyle.Render(item.Label)
	case timeline.KindDelegation, timeline.KindTool, timeline.KindPermission:
		label = warningStyle.Render(item.Label)
	case timeline.KindReport:
		label = primaryStyle.Render(item.Label)
	case timeline.KindError:
		label = errorStyle.Render(item.Label)
	}
	if strings.TrimSpace(item.Body) == "" {
		return label
	}
	body := item.Body
	if item.Kind == timeline.KindAgent || item.Kind == timeline.KindReport ||
		item.Kind == timeline.KindResult {
		body = markdown(body, width)
	}
	return label + "\n" + body
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
	if width < 90 {
		return 0, max(20, width-2)
	}
	sidebar := min(32, max(24, width/4))
	return sidebar, max(20, width-sidebar-3)
}

func divider(width int) string {
	return mutedStyle.Render(strings.Repeat("─", max(1, width)))
}

func statusStyle(status string) string {
	switch status {
	case "running":
		return warningStyle.Render(status)
	case "failed", "error":
		return errorStyle.Render(status)
	case "idle":
		return primaryStyle.Render(status)
	default:
		return mutedStyle.Render(status)
	}
}

func statusGlyph(status string) string {
	switch status {
	case "running":
		return warningStyle.Render("◆")
	case "failed", "error":
		return errorStyle.Render("×")
	case "idle":
		return primaryStyle.Render("●")
	default:
		return mutedStyle.Render("○")
	}
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
