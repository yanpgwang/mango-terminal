package ui

import (
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/yanpgwang/mango-terminal/internal/feed"
)

// roleColor is the Anthropic-Console-inspired palette that separates User,
// Agent, tool, delegation, report, and failure events at a glance. Keeping
// the mapping in one place means the timeline strip and the role pill on
// every event row always agree.
func roleColor(t theme, kind feed.Kind) color.Color {
	switch kind {
	case feed.User:
		return t.yellow
	case feed.Agent:
		return t.accent
	case feed.Tool:
		return t.blue
	case feed.Delegation:
		return t.yellow
	case feed.Report:
		return t.green
	case feed.Failure:
		return t.red
	case feed.Thinking, feed.Notice:
		return t.muted
	default:
		return t.muted
	}
}

func roleLabel(kind feed.Kind) string {
	switch kind {
	case feed.User:
		return "User"
	case feed.Agent:
		return "Agent"
	case feed.Tool:
		return "Tool"
	case feed.Delegation:
		return "Delegate"
	case feed.Report:
		return "Report"
	case feed.Failure:
		return "Error"
	case feed.Thinking:
		return "Think"
	case feed.Notice:
		return "Notice"
	default:
		return "Event"
	}
}

// rolePill paints a small colored chip with the role name. Selected pills use
// the role color as an opaque background; unselected ones keep the color as
// text on the base panel so a dense list stays readable.
func rolePill(t theme, kind feed.Kind, selected bool) string {
	label := roleLabel(kind)
	color := roleColor(t, kind)
	style := lipgloss.NewStyle().Padding(0, 1).Foreground(color)
	if selected {
		style = style.Background(color).Foreground(t.panel).Bold(true)
	}
	return style.Render(label)
}

// renderTimelineStrip draws one row of colored cells that mirror the event
// ledger. Each cell corresponds to one event (or one bucket of events when
// the ledger outgrows the strip width); its color is the role palette so a
// single glance separates user turns from tool bursts from reports.
func (m Model) renderTimelineStrip(width int, items []feed.Item, cursor int) string {
	if len(items) == 0 {
		return lipgloss.NewStyle().Width(width).Padding(0, 2).Render(
			m.theme.dim.Render("── no events yet ──"))
	}
	available := max(4, width-4)
	// bucket size: number of items per cell. When N events fit in fewer cells
	// than events, several events collapse into one; the cursor still maps
	// consistently.
	cellsWanted := min(available, len(items))
	if cellsWanted < 1 {
		cellsWanted = 1
	}
	cursorCell := 0
	cells := make([]string, 0, cellsWanted)
	for cellIdx := range cellsWanted {
		start := cellIdx * len(items) / cellsWanted
		end := (cellIdx + 1) * len(items) / cellsWanted
		if end > len(items) {
			end = len(items)
		}
		if start >= end {
			end = start + 1
		}
		if cursor >= start && cursor < end {
			cursorCell = cellIdx
		}
		bucketKind := items[start].Kind
		if end-1 > start {
			// Prefer the "most interesting" kind in the bucket so a dominant
			// tool burst does not get masked by a trailing notice.
			for j := start; j < end; j++ {
				if kindPriority(items[j].Kind) < kindPriority(bucketKind) {
					bucketKind = items[j].Kind
				}
			}
		}
		cells = append(cells, string('▆')) // placeholder, styled below
		_ = bucketKind
	}
	// Second pass to actually style; we do it separately so the cursor cell
	// can be recolored without redoing the bucket loop.
	styled := make([]string, len(cells))
	for cellIdx := range cells {
		start := cellIdx * len(items) / cellsWanted
		end := (cellIdx + 1) * len(items) / cellsWanted
		if start >= end {
			end = start + 1
		}
		kind := items[start].Kind
		for j := start; j < end && j < len(items); j++ {
			if kindPriority(items[j].Kind) < kindPriority(kind) {
				kind = items[j].Kind
			}
		}
		c := roleColor(m.theme, kind)
		style := lipgloss.NewStyle().Foreground(c)
		if cellIdx == cursorCell {
			style = lipgloss.NewStyle().Foreground(m.theme.accent).Bold(true)
			styled[cellIdx] = style.Render("▼")
			continue
		}
		styled[cellIdx] = style.Render("▆")
	}
	strip := strings.Join(styled, "")
	return lipgloss.NewStyle().Width(width).Padding(0, 2).Render(strip)
}

// kindPriority ranks event kinds so bucketing can pick the "loudest" one.
// Lower priority wins; user/failure/agent beat plain tool chatter.
func kindPriority(kind feed.Kind) int {
	switch kind {
	case feed.Failure:
		return 0
	case feed.User:
		return 1
	case feed.Delegation:
		return 2
	case feed.Report:
		return 3
	case feed.Agent:
		return 4
	case feed.Tool:
		return 5
	case feed.Notice:
		return 6
	case feed.Thinking:
		return 7
	default:
		return 8
	}
}

// renderTranscriptEventList is the left column of the transcript view. Each
// event is a compact row with a colored role pill, a truncated title, and a
// right-aligned relative time — the same shape the Console uses.
func (m Model) renderTranscriptEventList(width, height int, items []feed.Item, cursor int, focused bool) string {
	inner := max(8, width-4)
	if len(items) == 0 {
		body := m.theme.dim.Render("No events yet. Ask the coordinator to begin.")
		return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 1).
			Border(lipgloss.RoundedBorder()).BorderForeground(m.theme.border).Render(body)
	}
	visible := max(1, height-2)
	visible = min(visible, len(items))
	start, end := visibleRange(len(items), cursor, visible)
	now := time.Now()
	rows := make([]string, 0, end-start)
	for index := start; index < end; index++ {
		item := items[index]
		selected := index == cursor
		pill := rolePill(m.theme, item.Kind, selected)
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = firstLineOf(item.Body)
		}
		if title == "" {
			title = "(empty)"
		}
		right := ""
		if !item.Time.IsZero() {
			right = m.theme.dim.Render(humanizeSince(item.Time, now))
		}
		remain := inner - ansi.StringWidth(ansi.Strip(pill)) - ansi.StringWidth(ansi.Strip(right)) - 2
		title = truncate(title, max(4, remain))
		titleStyle := lipgloss.NewStyle().Foreground(m.theme.text)
		if selected {
			titleStyle = titleStyle.Bold(true).Foreground(m.theme.accent)
		}
		line := pill + " " + titleStyle.Render(title)
		if right != "" {
			pad := max(1, inner-ansi.StringWidth(ansi.Strip(line))-ansi.StringWidth(ansi.Strip(right)))
			line = line + strings.Repeat(" ", pad) + right
		}
		if selected {
			line = lipgloss.NewStyle().PaddingLeft(0).BorderLeft(true).
				BorderStyle(lipgloss.Border{Left: "▌"}).BorderForeground(m.theme.accent).Render(line)
		} else {
			line = "  " + line
		}
		rows = append(rows, line)
	}
	border := m.theme.border
	if focused {
		border = m.theme.accent
	}
	body := strings.Join(rows, "\n")
	return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 1).
		Border(lipgloss.RoundedBorder()).BorderForeground(border).Render(body)
}

// renderTranscriptDetail is the right column. It labels the selected event
// with the same colored role pill, then shows its full body wrapped to the
// panel width. Tool events append their raw input so the operator can inspect
// exactly what the Agent tried to do.
func (m Model) renderTranscriptDetail(width, height int, item *feed.Item) string {
	inner := max(8, width-4)
	if item == nil {
		body := m.theme.dim.Render("Select an event to see its full detail.")
		return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 1).
			Border(lipgloss.RoundedBorder()).BorderForeground(m.theme.border).Render(body)
	}
	head := rolePill(m.theme, item.Kind, true) + "  " +
		m.theme.title.Render(truncate(first(item.Title, roleLabel(item.Kind)), inner-14))
	metaBits := []string{}
	if item.Status != "" {
		metaBits = append(metaBits, item.Status)
	}
	if !item.Time.IsZero() {
		metaBits = append(metaBits, humanizeSince(item.Time, time.Now()))
	}
	if item.ToolName != "" && item.ToolName != item.Title {
		metaBits = append(metaBits, "tool: "+item.ToolName)
	}
	sections := []string{head}
	if len(metaBits) > 0 {
		sections = append(sections, m.theme.dim.Render(strings.Join(metaBits, "  ·  ")))
	}
	sections = append(sections, m.theme.dim.Render(strings.Repeat("─", inner)))
	if strings.TrimSpace(item.Body) != "" {
		wrapped := lipgloss.NewStyle().Width(inner).Render(strings.TrimSpace(item.Body))
		sections = append(sections, wrapped)
	}
	if strings.TrimSpace(item.Detail) != "" {
		sections = append(sections,
			m.theme.dim.Render("Input:"),
			lipgloss.NewStyle().Width(inner).Foreground(m.theme.muted).Render(strings.TrimSpace(item.Detail)),
		)
	}
	body := clipToHeight(strings.Join(sections, "\n"), max(1, height-2), m.theme)
	border := m.theme.border
	return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 1).
		Border(lipgloss.RoundedBorder()).BorderForeground(border).Render(body)
}

func (m Model) transcriptItems() []feed.Item {
	if m.threadCursor < 0 || m.threadCursor >= len(m.threads) {
		return nil
	}
	return feed.Project(m.threads[m.threadCursor], m.events[m.currentThreadID()])
}

func firstLineOf(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if newline := strings.IndexByte(text, '\n'); newline >= 0 {
		return text[:newline]
	}
	return text
}
