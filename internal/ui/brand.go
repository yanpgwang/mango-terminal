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

// mangoWordmark composes a six-row block letterform, then paints each row with
// a horizontal color ramp. Every glyph shares one cap height and baseline;
// half blocks are reserved for rounded edges instead of changing its height.
func (m Model) mangoWordmark() string {
	lines := []string{
		"██    ██  " + "  ▄██▄    " + "██    ██  " + " ▄████▄   " + " ▄████▄ ",
		"███  ███  " + " ▄█  █▄   " + "███   ██  " + "██    ██  " + "██    ██",
		"████████  " + "██    ██  " + "████  ██  " + "██        " + "██    ██",
		"██ ██ ██  " + "████████  " + "██ ██ ██  " + "██  ████  " + "██    ██",
		"██    ██  " + "██    ██  " + "██  ████  " + "██    ██  " + "██    ██",
		"██    ██  " + "██    ██  " + "██   ███  " + " ▀████▀   " + " ▀████▀ ",
	}
	for index, line := range lines {
		lines[index] = gradientText(lipgloss.NewStyle(), line, false, mangoFruitColor, mangoLinkColor)
	}
	wordmark := strings.Join(lines, "\n")
	attribution := m.theme.dim.Render("powered by Bubble Tea")
	return wordmark + "\n\n" + lipgloss.PlaceHorizontal(lipgloss.Width(wordmark), lipgloss.Center, attribution)
}

// brandLogo uses a compact text treatment in supporting views and a larger
// block wordmark on the central connection screen.
func (m Model) brandLogo(compact bool) string {
	if compact {
		return m.brandWord("MANGO")
	}
	return m.mangoWordmark()
}
