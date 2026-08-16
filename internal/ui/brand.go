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

// mangoWordmark is the quiet welcome scene used by the centered connect
// screen. The product name stays typographic; the illustration belongs to a
// small family of clouds that drift without changing the layout.
func (m Model) mangoWordmark() string {
	return m.welcomeBlock(58, false)
}

func (m Model) welcomeBlock(width int, compact bool) string {
	title := m.theme.title.Render("Welcome to ") + m.theme.active.Render("Mango")
	header := lipgloss.PlaceHorizontal(width, lipgloss.Center, title)
	subtitle := lipgloss.PlaceHorizontal(width, lipgloss.Center, m.theme.dim.Render("managed agents, one window"))
	spacer := ""
	if compact {
		spacer = m.theme.dim.Render(strings.Repeat("· ", max(0, width/2)))
	}
	return strings.Join([]string{header, subtitle, spacer, m.cloudScene(width, compact)}, "\n")
}

func (m Model) cloudScene(width int, compact bool) string {
	if compact {
		clouds := "╭─˙ᵕ˙─╮       ╭─•ᴗ•─╮       ╭─˘◡˘─╮"
		return lipgloss.PlaceHorizontal(width, lipgloss.Center,
			lipgloss.NewStyle().Foreground(lipgloss.Color("#99A398")).Render(clouds))
	}

	const height = 7
	frame := m.motion / 3
	leftX := 1 + cloudDrift(frame, 8, 2)
	middleX := max(0, (width-16)/2+cloudDrift(frame+3, 10, 2))
	rightX := max(0, width-14-2+cloudDrift(frame+6, 12, 2))
	leftY, rightY := 1, 0
	if !m.options.ReducedMotion {
		leftY += (frame / 5) % 2
		rightY += ((frame + 4) / 6) % 2
	}
	if m.options.ReducedMotion {
		leftX, middleX, rightX = 2, max(0, (width-16)/2), max(0, width-16)
	}

	grid := make([][]rune, height)
	for row := range grid {
		grid[row] = []rune(strings.Repeat(" ", width))
	}
	drawCloudLine(grid, max(0, width/3), 2, "·")
	drawCloudLine(grid, max(0, width*2/3), 3, "·")
	drawCloudLine(grid, leftX+2, leftY, "╭────╮")
	drawCloudLine(grid, leftX, leftY+1, "╭─╯ ˙ᵕ˙ ╰─╮")
	drawCloudLine(grid, leftX, leftY+2, "╰─────────╯")
	drawCloudLine(grid, middleX+4, 4, "╭──────╮")
	drawCloudLine(grid, middleX, 5, "╭──╯  •ᴗ•  ╰──╮")
	drawCloudLine(grid, middleX, 6, "╰─────────────╯")
	drawCloudLine(grid, rightX+2, rightY, "╭─────╮")
	drawCloudLine(grid, rightX, rightY+1, "╭─╯ ˘◡˘ ╰──╮")
	drawCloudLine(grid, rightX, rightY+2, "╰──────────╯")
	lines := make([]string, 0, height)
	for _, row := range grid {
		lines = append(lines, string(row))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#99A398")).Render(strings.Join(lines, "\n"))
}

func drawCloudLine(grid [][]rune, x, y int, value string) {
	if y < 0 || y >= len(grid) {
		return
	}
	for _, character := range []rune(value) {
		if x >= 0 && x < len(grid[y]) {
			grid[y][x] = character
		}
		x++
	}
}

func cloudDrift(frame, period, span int) int {
	if period <= 0 || span <= 0 {
		return 0
	}
	phase := frame % period
	half := max(1, period/2)
	if phase > half {
		phase = period - phase
	}
	return phase*span/half - span/2
}

// brandLogo uses a compact text treatment in supporting views and a larger
// block wordmark on the central connection screen.
func (m Model) brandLogo(compact bool) string {
	if compact {
		return m.brandWord("MANGO")
	}
	return m.mangoWordmark()
}
