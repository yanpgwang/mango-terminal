package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// activity is a restrained line spinner. Unlike a status dot it communicates
// motion rather than merely decorating a label, and its width never changes.
func (m Model) activity(label string) string {
	if m.options.ReducedMotion {
		return m.theme.dim.Render("──") + "  " + m.theme.dim.Render(label)
	}
	frames := []string{"╱─", "──", "─╲", "─│", "─╱", "──", "╲─", "│─"}
	frame := m.motion % len(frames)
	style := lipgloss.NewStyle()
	dots := []string{"", ".", "..", "..."}[(m.motion/3)%4]
	return gradientText(style, frames[frame], false, m.theme.accent, m.theme.blue) + "  " + m.theme.dim.Render(label+dots)
}

// fruitThinking is Mango's privacy-safe thinking signal. The server exposes
// only the lifecycle start (never private reasoning text), so the animation
// communicates ongoing work without inventing a chain of thought.
func (m Model) fruitThinking() string {
	if m.options.ReducedMotion {
		return "🥭  " + m.theme.dim.Render("Thinking")
	}
	fruits := []string{"🥭", "🍊", "🍋", "🍐", "🍑", "🍒", "🍇", "🥭"}
	fruit := fruits[(m.motion/2)%len(fruits)]
	verbs := []string{"Thinking", "Connecting ideas", "Exploring", "Shaping a reply"}
	label := verbs[(m.motion/8)%len(verbs)]
	return fruit + "  " + m.theme.dim.Render(label)
}

func (m Model) replyActivity() string {
	if m.options.ReducedMotion {
		return "🥭  " + m.theme.dim.Render("Preparing a reply")
	}
	frames := []string{"🥭", "🥭", "🍊", "🥭"}
	return frames[(m.motion/2)%len(frames)] + "  " + m.theme.dim.Render("Preparing a reply")
}

func (m Model) streamCaret() string {
	if m.options.ReducedMotion || (m.motion/2)%2 == 0 {
		return m.theme.active.Render("▌")
	}
	return m.theme.dim.Render("▎")
}

func (m Model) spark(id string) string {
	if id == "" || m.fresh[id] <= 0 {
		return ""
	}
	if m.options.ReducedMotion {
		return m.theme.active.Render("✦") + " "
	}
	frames := []string{"✦", "✧", "·"}
	index := (10 - m.fresh[id]) / 4
	if index >= len(frames) {
		index = len(frames) - 1
	}
	return m.theme.active.Render(frames[index]) + " "
}

func (m Model) dialogTitle(value string) string {
	return m.brandWord(value)
}

func (m Model) dialogRule(width int) string {
	return gradientText(lipgloss.NewStyle(), strings.Repeat("─", max(0, width)), false, m.theme.accent, m.theme.blue)
}
