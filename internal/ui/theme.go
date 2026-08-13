package ui

import (
	"image/color"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// theme contains semantic styles, not component geometry. Keeping color here
// makes the terminal UI replaceable without leaking presentation into the
// Managed Agents event projection.
type theme struct {
	accent    color.Color
	secondary color.Color
	text      color.Color
	muted     color.Color
	surface   color.Color
	border    color.Color
	warning   color.Color
	danger    color.Color
	success   color.Color

	title      lipgloss.Style
	strong     lipgloss.Style
	copy       lipgloss.Style
	dim        lipgloss.Style
	accentText lipgloss.Style
	warn       lipgloss.Style
	error      lipgloss.Style
	ok         lipgloss.Style
	selected   lipgloss.Style
	key        lipgloss.Style
}

func defaultTheme() theme {
	t := theme{
		accent:    lipgloss.Color("#6DE4E4"),
		secondary: lipgloss.Color("#B69CFF"),
		text:      lipgloss.Color("#E8E8E8"),
		muted:     lipgloss.Color("#777B83"),
		surface:   lipgloss.Color("#1C1E22"),
		border:    lipgloss.Color("#383B42"),
		warning:   lipgloss.Color("#E6C176"),
		danger:    lipgloss.Color("#F07178"),
		success:   lipgloss.Color("#A7D97A"),
	}
	t.title = lipgloss.NewStyle().Foreground(t.text).Bold(true)
	t.strong = lipgloss.NewStyle().Foreground(t.text).Bold(true)
	t.copy = lipgloss.NewStyle().Foreground(t.text)
	t.dim = lipgloss.NewStyle().Foreground(t.muted)
	t.accentText = lipgloss.NewStyle().Foreground(t.accent).Bold(true)
	t.warn = lipgloss.NewStyle().Foreground(t.warning)
	t.error = lipgloss.NewStyle().Foreground(t.danger)
	t.ok = lipgloss.NewStyle().Foreground(t.success)
	t.selected = lipgloss.NewStyle().Foreground(t.text).Background(t.surface).Bold(true)
	t.key = lipgloss.NewStyle().Foreground(t.text).Background(t.surface).Padding(0, 1)
	return t
}

func (t theme) textareaStyles() textarea.Styles {
	styles := textarea.DefaultDarkStyles()
	styles.Focused.Base = lipgloss.NewStyle().Foreground(t.text)
	styles.Focused.Text = lipgloss.NewStyle().Foreground(t.text)
	styles.Focused.Placeholder = lipgloss.NewStyle().Foreground(t.muted)
	styles.Focused.Prompt = lipgloss.NewStyle().Foreground(t.accent).Bold(true)
	styles.Focused.EndOfBuffer = lipgloss.NewStyle().Foreground(t.muted)
	styles.Blurred = styles.Focused
	styles.Cursor.Color = t.accent
	styles.Cursor.Shape = tea.CursorBar
	styles.Cursor.Blink = true
	return styles
}
