package ui

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type commandItem struct {
	label       string
	description string
	key         string
}

func (m Model) commandItems() []commandItem {
	items := []commandItem{
		{label: "Interrupt selected Thread", description: "Stop only the Agent currently in focus", key: "interrupt_one"},
		{label: "Interrupt all Threads", description: "Stop the coordinator and every active specialist", key: "interrupt_all"},
		{label: "Refresh durable state", description: "Reload the Session, Thread roster, and event ledgers", key: "refresh"},
		{label: "Focus Agent Threads", description: "Move keyboard focus to the coordinator roster", key: "threads"},
		{label: "Detach from Session", description: "Return to the cloud Session inbox", key: "detach"},
	}
	if len(m.pending) > 0 {
		items = append([]commandItem{{
			label:       fmt.Sprintf("Review pending actions (%d)", len(m.pending)),
			description: "Allow or deny a tool waiting at a durable permission gate",
			key:         "permissions",
		}}, items...)
	}
	return items
}

func (m Model) updateOverlay(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "ctrl+k":
		m.overlay = overlayNone
		if m.focus == focusComposer {
			return m, m.composer.Focus()
		}
		return m, nil
	case "up", "k":
		m.overlayCursor = clamp(m.overlayCursor-1, 0, m.overlayLength()-1)
		return m, nil
	case "down", "j":
		m.overlayCursor = clamp(m.overlayCursor+1, 0, m.overlayLength()-1)
		return m, nil
	}

	if m.overlay == overlayPermissions {
		if m.overlayCursor < 0 || m.overlayCursor >= len(m.pending) {
			return m, nil
		}
		action := m.pending[m.overlayCursor]
		switch key.String() {
		case "a":
			if action.kind == "tool_confirmation" {
				m.overlay, m.loading = overlayNone, true
				return m, m.confirmTool(action, "allow", "")
			}
		case "d":
			if action.kind == "tool_confirmation" {
				m.overlay, m.loading = overlayNone, true
				return m, m.confirmTool(action, "deny", "Denied from Mango Terminal")
			}
		}
		return m, nil
	}

	if key.String() != "enter" || m.overlay != overlayCommands {
		return m, nil
	}
	items := m.commandItems()
	if m.overlayCursor < 0 || m.overlayCursor >= len(items) {
		return m, nil
	}
	switch items[m.overlayCursor].key {
	case "permissions":
		m.overlay, m.overlayCursor = overlayPermissions, 0
	case "interrupt_one":
		m.overlay, m.loading = overlayNone, true
		return m, m.interrupt(m.currentThreadID())
	case "interrupt_all":
		m.overlay, m.loading = overlayNone, true
		return m, m.interrupt("")
	case "refresh":
		m.overlay, m.loading = overlayNone, true
		return m, m.loadAttached(m.session.ID)
	case "threads":
		m.overlay, m.focus = overlayNone, focusThreads
		m.timeline.active = false
	case "detach":
		m.stopStreams()
		m.overlay, m.view, m.session = overlayNone, viewInbox, nil
		m.loading, m.err = true, nil
		return m, m.loadInbox()
	}
	return m, nil
}

func (m Model) overlayLength() int {
	if m.overlay == overlayPermissions {
		return len(m.pending)
	}
	return len(m.commandItems())
}

func (m Model) overlayView(width int) string {
	if m.overlay == overlayPermissions {
		return m.permissionsOverlay(width)
	}
	return m.commandsOverlay(width)
}

func (m Model) commandsOverlay(width int) string {
	boxWidth := min(68, max(42, width-8))
	lines := []string{
		m.theme.title.Render("Commands"),
		m.theme.dim.Render("Control the attached cloud Session"),
		"",
	}
	for index, item := range m.commandItems() {
		pointer, title := "  ", m.theme.copy.Render(item.label)
		if index == m.overlayCursor {
			pointer, title = m.theme.accentText.Render("› "), m.theme.selected.Render(item.label)
		}
		lines = append(lines,
			pointer+title,
			"  "+m.theme.dim.Render(truncate(item.description, boxWidth-5)),
			"",
		)
	}
	lines = append(lines, m.theme.dim.Render("↑/↓ select   enter run   esc close"))
	return overlayBox(m.theme, boxWidth, strings.Join(lines, "\n"))
}

func (m Model) permissionsOverlay(width int) string {
	boxWidth := min(72, max(44, width-8))
	lines := []string{
		m.theme.warn.Render(fmt.Sprintf("Pending actions  %d", len(m.pending))),
		m.theme.dim.Render("The Session is durably paused until these actions are resolved."),
		"",
	}
	for index, action := range m.pending {
		name := first(stringValue(action.event["name"]), "tool")
		pointer, title := "  ", m.theme.copy.Render(name)
		if index == m.overlayCursor {
			pointer, title = m.theme.accentText.Render("› "), m.theme.selected.Render(name)
		}
		kind := strings.ReplaceAll(action.kind, "_", " ")
		lines = append(lines, pointer+title+"  "+m.theme.dim.Render(kind))
		if index == m.overlayCursor {
			input := renderPlainBody(jsonValue(action.event["input"]), boxWidth-5, false, 4)
			if input != "" {
				lines = append(lines, "  "+m.theme.dim.Render(input))
			}
		}
		lines = append(lines, "")
	}
	if len(m.pending) > 0 && m.pending[m.overlayCursor].kind == "tool_confirmation" {
		lines = append(lines,
			m.theme.key.Render("a")+" allow   "+m.theme.key.Render("d")+" deny   "+m.theme.dim.Render("↑/↓ choose  esc close"),
		)
	} else {
		lines = append(lines, m.theme.dim.Render("This action requires an external result client.  esc close"))
	}
	return overlayBox(m.theme, boxWidth, strings.Join(lines, "\n"))
}

func overlayBox(t theme, width int, content string) string {
	return t.copy.Copy().
		Width(width).
		Padding(1, 2).
		Background(t.surface).
		BorderForeground(t.accent).
		BorderStyle(lipgloss.RoundedBorder()).
		Render(content)
}

func jsonValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}
