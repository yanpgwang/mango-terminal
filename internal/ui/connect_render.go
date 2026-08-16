package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

type connectLayout struct {
	view           string
	inputX, inputY int
}

func (m Model) buildConnectLayout() connectLayout {
	width, height := max(1, m.width), max(1, m.height)
	frameWidth, frameHeight, contentWidth, compact := connectDimensions(width, height)

	welcome := m.welcomeBlock(contentWidth, compact)
	card, inputCardX, inputCardY := m.renderConnectCard(contentWidth, compact)
	footer := m.theme.dim.Render(truncate(m.connectFooter(), frameWidth))
	inner := strings.Join([]string{welcome, "", card, "", footer}, "\n")
	if lipgloss.Height(inner) > frameHeight {
		welcome = m.welcomeBlock(contentWidth, true)
		inner = strings.Join([]string{welcome, card, footer}, "\n")
	}

	innerWidth, innerHeight := lipgloss.Width(inner), lipgloss.Height(inner)
	innerLeft := max(0, (frameWidth-innerWidth)/2)
	innerTop := max(0, (frameHeight-innerHeight)/2)
	frameBody := lipgloss.Place(frameWidth, frameHeight, lipgloss.Center, lipgloss.Center, inner)
	frame := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#354138")).
		Render(frameBody)
	frameTotalWidth, frameTotalHeight := lipgloss.Width(frame), lipgloss.Height(frame)
	frameLeft := max(0, (width-frameTotalWidth)/2)
	frameTop := max(0, (height-frameTotalHeight)/2)
	view := lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, frame)

	cardTop := lipgloss.Height(welcome) + 1
	if lipgloss.Height(strings.Join([]string{welcome, "", card, "", footer}, "\n")) > frameHeight {
		cardTop = lipgloss.Height(welcome)
	}
	cardLeft := max(0, (innerWidth-lipgloss.Width(card))/2)
	return connectLayout{
		view:   view,
		inputX: frameLeft + 1 + innerLeft + cardLeft + inputCardX,
		inputY: frameTop + 1 + innerTop + cardTop + inputCardY,
	}
}

func (m Model) renderConnectCard(width int, compact bool) (string, int, int) {
	innerWidth, horizontalPadding, verticalPadding := connectCardMetrics(width, compact)
	lines := []string{m.theme.title.Render("Connect to Mango Cloud")}
	if !compact {
		lines = append(lines, "")
	}

	inputLine := -1
	if m.connect.editing {
		lines = append(lines, m.theme.active.Render("Endpoint · enter an address"))
		inputLine = len(lines)
		input := m.connect.endpointInput
		if input.Width() != innerWidth {
			input.SetWidth(innerWidth)
		}
		lines = append(lines, input.View())
		if m.connect.validationError != "" {
			lines = append(lines, m.theme.danger.Render(trimOneLine(m.connect.validationError, innerWidth)))
		} else {
			lines = append(lines, m.theme.dim.Render("http:// and https:// are supported"))
		}
	} else if m.connect.pickerOpen {
		lines = append(lines, m.theme.active.Render("Endpoint · choose one"))
		lines = append(lines, m.renderEndpointPicker(innerWidth)...)
	} else {
		label := "  Endpoint"
		if m.connect.focus == connectFocusEndpoint {
			label = "› Endpoint"
		}
		selected := m.connect.current()
		value := first(selected.label, selected.url, "Mango Cloud")
		arrows := "  "
		if len(m.connect.endpoints) > 1 {
			arrows = "‹ ›"
		}
		valueWidth := max(8, innerWidth-lipgloss.Width(label)-lipgloss.Width(arrows)-4)
		valueStyle := m.theme.title
		if m.connect.focus == connectFocusEndpoint {
			valueStyle = m.theme.active
		}
		row := label + "  " + valueStyle.Render(truncate(value, valueWidth)) + "  " + m.theme.dim.Render(arrows)
		lines = append(lines, row)
		lines = append(lines, m.theme.dim.Render(truncate(m.endpointSummary(selected), innerWidth)))
	}

	if !compact || (!m.connect.pickerOpen && !m.connect.editing) {
		lines = append(lines, "")
	}
	if m.loading {
		lines = append(lines, lipgloss.PlaceHorizontal(innerWidth, lipgloss.Center,
			m.activity(first(m.loadingLabel, "Connecting to Mango Cloud"))))
	} else {
		button := "Connect"
		buttonStyle := lipgloss.NewStyle().
			Width(innerWidth).
			Align(lipgloss.Center).
			Foreground(lipgloss.Color("#11150F")).
			Background(m.theme.accent)
		if m.connect.focus == connectFocusButton && !m.connect.pickerOpen && !m.connect.editing {
			buttonStyle = buttonStyle.Bold(true)
		}
		lines = append(lines, buttonStyle.Render(button))
	}
	if m.err != nil {
		lines = append(lines, m.theme.danger.Render(trimOneLine(m.err.Error(), innerWidth)))
	} else if !compact {
		note := "The selected endpoint is saved after a successful connection."
		if strings.HasPrefix(m.connect.current().url, "demo://") {
			note = "The local demo runs entirely on this device."
		}
		lines = append(lines, m.theme.dim.Render(note))
	}

	card := lipgloss.NewStyle().
		Width(innerWidth).
		Padding(verticalPadding, horizontalPadding).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#3B463D")).
		Render(strings.Join(lines, "\n"))
	inputX := 1 + horizontalPadding
	inputY := 1 + verticalPadding + inputLine
	return card, inputX, inputY
}

func (m Model) renderEndpointPicker(width int) []string {
	count := len(m.connect.endpoints) + 1
	visible := min(4, count)
	start, end := visibleRange(count, m.connect.pickerCursor, visible)
	rows := make([]string, 0, end-start)
	for index := start; index < end; index++ {
		marker := "  "
		style := lipgloss.NewStyle().Foreground(m.theme.text)
		if index == m.connect.pickerCursor {
			marker, style = "› ", m.theme.active
		}
		if index == len(m.connect.endpoints) {
			rows = append(rows, marker+style.Render("Enter another endpoint…"))
			continue
		}
		option := m.connect.endpoints[index]
		status := m.endpointStatus(option)
		label := truncate(first(option.label, option.url), max(8, width-2-lipgloss.Width(status)-5))
		rows = append(rows, marker+style.Render(label)+m.theme.dim.Render("  ·  ")+status)
	}
	return rows
}

func (m Model) endpointSummary(option endpointChoice) string {
	parts := make([]string, 0, 2)
	if option.source != "" {
		parts = append(parts, option.source)
	}
	switch option.availability {
	case endpointReachable:
		if option.skipProbe {
			parts = append(parts, "ready")
		} else {
			parts = append(parts, "reachable")
		}
	case endpointChecking:
		parts = append(parts, "checking…")
	case endpointUnreachable:
		parts = append(parts, "not reachable · connection is still allowed")
	}
	if len(parts) == 0 {
		return "configured endpoint"
	}
	return strings.Join(parts, " · ")
}

func (m Model) endpointStatus(option endpointChoice) string {
	switch option.availability {
	case endpointReachable:
		if option.skipProbe {
			return m.theme.success.Render("● ready")
		}
		return m.theme.success.Render("● reachable")
	case endpointChecking:
		return m.theme.dim.Render("◌ checking")
	case endpointUnreachable:
		return m.theme.dim.Render("○ not reachable")
	default:
		return m.theme.dim.Render(first(option.source, "configured"))
	}
}

func connectDimensions(width, height int) (frameWidth, frameHeight, contentWidth int, compact bool) {
	frameWidth = min(112, max(50, width-8))
	frameHeight = min(38, max(16, height-4))
	compact = frameWidth < 76 || frameHeight < 27
	contentWidth = min(70, max(46, frameWidth-8))
	return
}

func connectCardMetrics(width int, compact bool) (innerWidth, horizontalPadding, verticalPadding int) {
	horizontalPadding, verticalPadding = 2, 1
	if compact {
		horizontalPadding, verticalPadding = 1, 0
	}
	innerWidth = max(20, width-2-2*horizontalPadding)
	return
}

func (m Model) connectFooter() string {
	switch {
	case m.connect.editing:
		return "type endpoint · enter add · esc cancel"
	case m.connect.pickerOpen:
		return "↑↓ choose · enter select · e edit · esc close"
	case m.connect.focus == connectFocusEndpoint:
		return fmt.Sprintf("enter choose · ←→ switch · e edit · ↓ connect · ctrl+c quit")
	default:
		return "↑ endpoint · enter connect · ctrl+c quit"
	}
}
