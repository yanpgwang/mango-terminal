package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/rivo/uniseg"
)

var (
	mangoFruitColor = lipgloss.Color("#FF9D18")
	mangoLinkColor  = lipgloss.Color("#3157D5")
)

// gradientText splits by grapheme cluster, blends a color ramp, then styles
// every printable cluster. It keeps CJK and emoji intact instead of slicing
// UTF-8 bytes.
func gradientText(base lipgloss.Style, value string, bold bool, from, to color.Color) string {
	graphemes := uniseg.NewGraphemes(value)
	clusters := make([]string, 0, len(value))
	for graphemes.Next() {
		clusters = append(clusters, graphemes.Str())
	}
	if len(clusters) == 0 {
		return ""
	}
	ramp := lipgloss.Blend1D(len(clusters), from, to)
	var output strings.Builder
	for index, cluster := range clusters {
		style := base.Foreground(ramp[index]).Bold(bold)
		output.WriteString(style.Render(cluster))
	}
	return output.String()
}

func (m Model) brandWord(value string) string {
	return gradientText(lipgloss.NewStyle(), value, true, mangoFruitColor, mangoLinkColor)
}

// mangoWordmark is the restrained welcome scene used by the centered connect
// screen. It borrows the quiet horizon and sparse accent treatment of a modern
// terminal onboarding flow without turning the product name into a wall of
// block letters.
func (m Model) mangoWordmark() string {
	const width = 58
	title := m.theme.active.Render("Welcome to Mango")
	subtitle := m.theme.dim.Render("managed agents, one window")
	header := joinSides(title, subtitle, width)
	fruit := func(value string) string {
		return gradientText(lipgloss.NewStyle(), value, false, mangoFruitColor, m.theme.accent)
	}
	lines := []string{
		header,
		m.theme.dim.Render(strings.Repeat("…", width)),
		"",
		m.theme.dim.Render("                         ·                    ╭────╮"),
		"             " + fruit("▄▀") + m.theme.dim.Render("                         ╭──╯    ╰─╮"),
		"          " + fruit("▄████▄") + m.theme.dim.Render("                     ╰──────────╯"),
		"         " + fruit("████████") + m.theme.dim.Render("          ·"),
		"          " + fruit("▀████▀") + m.theme.dim.Render("                           ·"),
		"             " + fruit("╲╱") + m.theme.dim.Render("              ╭──────╮"),
		m.theme.dim.Render("……………………………………………………………………………………………………………╯      ╰………………………"),
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

// brandLogo uses a compact text treatment in supporting views and a larger
// block wordmark on the central connection screen.
func (m Model) brandLogo(compact bool) string {
	if compact {
		return m.brandWord("MANGO")
	}
	return m.mangoWordmark()
}
