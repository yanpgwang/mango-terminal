package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/yanpgwang/mango-terminal/internal/timeline"
)

type timelineView struct {
	items    []timeline.Item
	selected int
	expanded map[string]bool
	width    int
	active   bool
	now      time.Time
}

func newTimelineView() timelineView {
	return timelineView{expanded: make(map[string]bool), now: time.Now()}
}

func (v *timelineView) setItems(items []timeline.Item) {
	selectedID := v.selectedID()
	v.items = items
	v.selected = len(v.items) - 1
	if selectedID != "" {
		for index := range v.items {
			if v.items[index].ID == selectedID {
				v.selected = index
				break
			}
		}
	}
	if len(v.items) == 0 {
		v.selected = -1
	}
}

func (v *timelineView) moveSelection(delta int) {
	if len(v.items) == 0 {
		return
	}
	v.selected = clamp(v.selected+delta, 0, len(v.items)-1)
}

func (v *timelineView) toggle() {
	if v.selected < 0 || v.selected >= len(v.items) {
		return
	}
	item := v.items[v.selected]
	if item.ID != "" {
		v.expanded[item.ID] = !v.expanded[item.ID]
	}
}

func (v timelineView) selectedID() string {
	if v.selected < 0 || v.selected >= len(v.items) {
		return ""
	}
	return v.items[v.selected].ID
}

func (v timelineView) render(t theme) string {
	if len(v.items) == 0 {
		return t.dim.Render("No visible activity yet.")
	}
	blocks := make([]string, 0, len(v.items))
	for index, item := range v.items {
		blocks = append(blocks, renderTimelineItem(t, item, v.width, v.expanded[item.ID], v.active && index == v.selected))
	}
	return strings.Join(blocks, "\n\n")
}

func renderTimelineItem(t theme, item timeline.Item, width int, expanded, selected bool) string {
	width = max(24, width)
	marker := "  "
	if selected {
		marker = t.accentText.Render("› ")
	}
	available := max(20, width-lipgloss.Width(marker)-1)

	switch item.Kind {
	case timeline.KindUser:
		body := renderPlainBody(item.Body, available, expanded, 8)
		content := t.strong.Render("You") + timeSuffix(t, item.Time) + "\n" + body
		return marker + indentAfterFirst(content, lipgloss.Width(marker))
	case timeline.KindAgent:
		content := t.accentText.Render(first(item.Agent, item.Label, "Agent")) + timeSuffix(t, item.Time)
		if strings.TrimSpace(item.Body) != "" {
			content += "\n" + markdown(item.Body, available)
		}
		return marker + indentAfterFirst(content, lipgloss.Width(marker))
	case timeline.KindThinking:
		return marker + t.dim.Render("Thinking") + timeSuffix(t, item.Time)
	case timeline.KindDelegation:
		body := renderPlainBody(item.Body, available-3, expanded, 3)
		line := t.warn.Render("◆") + "  " + t.strong.Render(item.Label) + timeSuffix(t, item.Time)
		if body != "" {
			line += "\n" + t.dim.Render("│") + "  " + indentAfterFirst(body, 3)
		}
		return marker + indentAfterFirst(line, lipgloss.Width(marker))
	case timeline.KindReport:
		body := renderPlainBody(item.Body, available-3, expanded, 7)
		line := t.accentText.Render("└─") + " " + t.strong.Render(item.Label) + timeSuffix(t, item.Time)
		if body != "" {
			line += "\n   " + indentAfterFirst(body, 3)
		}
		return marker + indentAfterFirst(line, lipgloss.Width(marker))
	case timeline.KindTool, timeline.KindPermission:
		return marker + indentAfterFirst(renderToolItem(t, item, available, expanded), lipgloss.Width(marker))
	case timeline.KindResult:
		body := renderPlainBody(item.Body, available-3, expanded, 4)
		return marker + t.dim.Render("└─ Result") + "\n   " + indentAfterFirst(body, 3)
	case timeline.KindError:
		body := renderPlainBody(item.Body, available-3, expanded, 6)
		return marker + t.error.Render("× "+item.Label) + "\n  " + indentAfterFirst(body, 2)
	case timeline.KindStatus:
		return marker + t.dim.Render("· "+item.Label) + timeSuffix(t, item.Time)
	default:
		return marker + item.Label
	}
}

func renderToolItem(t theme, item timeline.Item, width int, expanded bool) string {
	glyph, status := "◇", item.Status
	style := t.dim
	switch {
	case item.Kind == timeline.KindPermission || status == "ask":
		glyph, status, style = "!", "approval required", t.warn
	case item.IsError || status == "error":
		glyph, status, style = "×", "failed", t.error
	case status == "complete":
		glyph, status, style = "✓", "complete", t.ok
	case status == "allow" || status == "always_allow":
		glyph, status, style = "◇", "running", t.dim
	}
	header := style.Render(glyph) + "  " + t.strong.Render(first(item.Tool, "tool"))
	if status != "" {
		header += "  " + style.Render(status)
	}
	header += timeSuffix(t, item.Time)

	input := renderPlainBody(item.Body, width-3, expanded, 3)
	if input != "" {
		header += "\n" + t.dim.Render("│") + "  " + indentAfterFirst(input, 3)
	}
	if strings.TrimSpace(item.Result) != "" {
		resultStyle := t.dim
		if item.IsError {
			resultStyle = t.error
		}
		result := renderPlainBody(item.Result, width-3, expanded, 4)
		header += "\n" + resultStyle.Render("└") + "  " + indentAfterFirst(result, 3)
	}
	if expandable(item) && !expanded {
		header += "\n" + t.dim.Render("   space to expand")
	}
	return header
}

func expandable(item timeline.Item) bool {
	return len(strings.Split(strings.TrimSpace(item.Body), "\n")) > 3 ||
		len([]rune(item.Body)) > 240 || len([]rune(item.Result)) > 240
}

func renderPlainBody(body string, width int, expanded bool, collapsedLines int) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	wrapped := lipgloss.NewStyle().Width(max(12, width)).Render(body)
	lines := strings.Split(wrapped, "\n")
	if !expanded && len(lines) > collapsedLines {
		hidden := len(lines) - collapsedLines
		lines = append(lines[:collapsedLines], fmt.Sprintf("… %d more lines", hidden))
	}
	return strings.Join(lines, "\n")
}

func indentAfterFirst(value string, spaces int) string {
	if spaces <= 0 {
		return value
	}
	indent := strings.Repeat(" ", spaces)
	return strings.ReplaceAll(value, "\n", "\n"+indent)
}

func timeSuffix(t theme, value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return t.dim.Render("  " + value.Local().Format("15:04"))
}
