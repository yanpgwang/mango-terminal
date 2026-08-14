package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/yanpgwang/mango-terminal/internal/feed"
	"github.com/yanpgwang/mango-terminal/internal/mango"
)

// renderBoard is the primary Session view. Instead of a scrollable chat with
// the Agent roster hidden in a sidebar, the Board makes the delegation graph
// the main object: coordinator on the left, specialists as cards on the right,
// and pending user-decisions in their own row at the bottom.
func (m Model) renderBoard() string {
	width, height := max(1, m.width), max(1, m.height)
	parts := append([]string(nil), m.linesAboveComposer(width)...)
	parts = append(parts, m.renderComposer(width), m.renderHelp(width))
	return lipgloss.NewStyle().Width(width).Height(height).Render(strings.Join(parts, "\n"))
}

func (m Model) renderBoardHeader(width int) string {
	title := "Managed Session"
	status := ""
	if m.session != nil {
		title = first(m.session.Title, "Untitled Session")
		status = stateText(m.theme, m.session.Status)
	}
	right := m.theme.dim.Render("‹ Sessions  esc")
	metaParts := []string{}
	if m.session != nil {
		if len(m.threads) > 0 {
			label := fmt.Sprintf("%d agents", len(m.threads))
			if roster := subagentCount(*m.session); roster > 0 {
				// Roster reflects what the coordinator can delegate to, so add the
				// coordinator (+1) to make the "of Y" total match total agents in
				// the Session, not just its specialist headcount.
				label = fmt.Sprintf("%d of %d agents", len(m.threads), roster+1)
			}
			metaParts = append(metaParts, label)
		}
		if model := strings.TrimSpace(m.session.Agent.Model.ID); model != "" {
			metaParts = append(metaParts, model)
		}
		if m.session.Usage.InputTokens+m.session.Usage.OutputTokens > 0 {
			metaParts = append(metaParts, compactTokens(m.session.Usage.InputTokens)+" in  /  "+
				compactTokens(m.session.Usage.OutputTokens)+" out")
		}
	}
	meta := m.theme.dim.Render(strings.Join(metaParts, "  ·  "))
	prefix := m.theme.title.Render(title)
	if status != "" {
		prefix += "  " + status
	}
	if meta != "" {
		prefix += "  " + m.theme.dim.Render("·") + "  " + meta
	}
	prefix = truncate(prefix, max(1, width-4-ansi.StringWidth(ansi.Strip(right))))
	line := lipgloss.NewStyle().Width(width).Padding(0, 2).Render(joinSides(prefix, right, width-4))
	tagline := lipgloss.NewStyle().Width(width).Padding(0, 2).Render(
		m.theme.dim.Render("Managed Agents keep running in the cloud after you detach."))
	return line + "\n" + tagline
}

// renderBoardBody lays out the coordinator panel, the sub-agent list, and the
// pending-action strip. On wide terminals coordinator and specialists share a
// horizontal row; below 120 cols the panels stack.
func (m Model) renderBoardBody(width, height int) string {
	coordinator := m.coordinatorThread()
	specialists := m.specialistThreads()

	// Reserve one blank line at top and one for the pending row, leaving the
	// rest for the two Agent panels.
	pendingHeight := 4
	if len(m.pending) == 0 {
		pendingHeight = 3
	}
	panelsHeight := max(6, height-pendingHeight-1)

	var body string
	if m.compact || width < 90 {
		body = m.renderBoardStacked(width, panelsHeight, coordinator, specialists)
	} else {
		body = m.renderBoardTwoColumn(width, panelsHeight, coordinator, specialists)
	}
	pending := m.renderPendingPanel(width, pendingHeight)
	return "\n" + body + "\n" + pending
}

func (m Model) renderBoardTwoColumn(width, height int, coordinator *mango.Thread, specialists []mango.Thread) string {
	leftWidth := width * 55 / 100
	rightWidth := width - leftWidth - 2 // room for the gutter between panels
	if leftWidth < 32 {
		leftWidth = 32
	}
	if rightWidth < 28 {
		rightWidth = max(28, width-leftWidth-2)
	}
	left := m.renderCoordinatorCard(leftWidth, height, coordinator)
	right := m.renderSpecialistList(rightWidth, height, specialists)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
}

func (m Model) renderBoardStacked(width, height int, coordinator *mango.Thread, specialists []mango.Thread) string {
	// Give the coordinator roughly a third; specialists take the rest so the
	// operator can still scan every child on a narrow terminal.
	coordHeight := max(5, height/3)
	specHeight := max(5, height-coordHeight-1)
	panels := []string{
		m.renderCoordinatorCard(width, coordHeight, coordinator),
		m.renderSpecialistList(width, specHeight, specialists),
	}
	return strings.Join(panels, "\n")
}

func (m Model) renderCoordinatorCard(width, height int, thread *mango.Thread) string {
	title := "Coordinator"
	if thread != nil {
		title = "Coordinator · " + first(thread.Agent.Name, "coordinator")
	}
	selected := m.focus == focusBoard && m.boardCursor == 0 && thread != nil
	// Card total height minus rounded border (2) and title/rule rows (2) is the
	// vertical budget for the body.
	bodyHeight := max(1, height-4)
	body := m.coordinatorBody(width-6, bodyHeight)
	return m.renderCard(width, height, title, sessionStatePipFor(m.theme, thread), body, selected, thread != nil)
}

// coordinatorBody paints the coordinator panel body. Long replies wrap to the
// card width instead of being hard-truncated at the first line — the Board is
// meant to be readable at a glance, not just a topology diagram. The body is
// then clipped to the card's vertical budget with an explicit ellipsis so the
// operator knows a zoom would show more.
func (m Model) coordinatorBody(width, height int) string {
	if m.session == nil {
		return m.theme.dim.Render("Waiting for the coordinator to connect…")
	}
	coordinator := m.coordinatorThread()
	if coordinator == nil {
		return m.theme.dim.Render("Waiting for the primary Thread…")
	}
	items := feed.Project(*coordinator, m.events[coordinator.ID])
	latestUser := latestFeedItem(items, feed.User)
	latestAgent := latestFeedItem(items, feed.Agent)
	wrap := func(text string) string {
		return lipgloss.NewStyle().Width(max(4, width)).Render(strings.TrimSpace(text))
	}
	sections := make([]string, 0, 4)
	if latestUser != nil {
		sections = append(sections, m.theme.dim.Render("You:")+"\n"+wrap(latestUser.Body))
	}
	if latestAgent != nil {
		sections = append(sections, m.theme.dim.Render("Latest reply:")+"\n"+wrap(latestAgent.Body))
	}
	if latestUser == nil && latestAgent == nil {
		sections = append(sections, m.theme.dim.Render("No conversation yet. Message the coordinator below."))
	}
	if coordinator.Stats.ActiveSeconds > 0 {
		sections = append(sections, m.theme.dim.Render(fmt.Sprintf("%.1fs active", coordinator.Stats.ActiveSeconds)))
	}
	return clipToHeight(strings.Join(sections, "\n\n"), height, m.theme)
}

// clipToHeight keeps the top `height` lines of content, replacing the last
// kept line with an ellipsis when we had to drop rows. Preserving the very
// last line as a full text row would hide the fact that a zoom reveals more.
func clipToHeight(content string, height int, t theme) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) <= height {
		return content
	}
	kept := append([]string(nil), lines[:height-1]...)
	kept = append(kept, t.dim.Render("… zoom to read the rest"))
	return strings.Join(kept, "\n")
}

func (m Model) renderSpecialistList(width, height int, specialists []mango.Thread) string {
	ghosts := m.unspawnedRoster(specialists)
	title := fmt.Sprintf("Sub-agents · %d", len(specialists))
	if len(ghosts) > 0 {
		title = fmt.Sprintf("Sub-agents · %d live / %d in roster", len(specialists), len(specialists)+len(ghosts))
	}
	if len(specialists) == 0 && len(ghosts) == 0 {
		body := m.theme.dim.Render("This Session runs a single Agent. No specialists are configured.")
		return m.renderCard(width, height, title, "", body, false, false)
	}
	rows := make([]string, 0, len(specialists)+len(ghosts))
	for index, thread := range specialists {
		selected := m.focus == focusBoard && m.boardCursor == index+1
		rows = append(rows, m.renderSpecialistCard(width-4, thread, selected))
	}
	for _, ghost := range ghosts {
		rows = append(rows, m.renderGhostSpecialistCard(width-4, ghost))
	}
	body := strings.Join(rows, "\n\n")
	return m.renderCard(width, height, title, "", body, false, false)
}

// unspawnedRoster returns the roster entries the coordinator has NOT yet
// delegated to. They are still valid targets — the coordinator's own tool
// choice decides when they get a Thread — but until then the operator has no
// way to see they exist without cross-referencing the Agent config elsewhere.
func (m Model) unspawnedRoster(live []mango.Thread) []mango.AgentReference {
	if m.session == nil || m.session.Agent.Multiagent == nil {
		return nil
	}
	liveIDs := map[string]struct{}{}
	liveNames := map[string]struct{}{}
	for _, thread := range live {
		if thread.Agent.ID != "" {
			liveIDs[thread.Agent.ID] = struct{}{}
		}
		if thread.Agent.Name != "" {
			liveNames[thread.Agent.Name] = struct{}{}
		}
	}
	coordinatorID := m.session.Agent.ID
	out := make([]mango.AgentReference, 0, len(m.session.Agent.Multiagent.Agents))
	for _, reference := range m.session.Agent.Multiagent.Agents {
		if reference.ID == coordinatorID {
			continue // roster may include self; the coordinator's own card is already on the Board
		}
		if _, ok := liveIDs[reference.ID]; ok {
			continue
		}
		if reference.Name != "" {
			if _, ok := liveNames[reference.Name]; ok {
				continue
			}
		}
		out = append(out, reference)
	}
	return out
}

// renderGhostSpecialistCard renders a roster member that the coordinator has
// declared but not yet delegated to. It intentionally reads dim so it never
// competes with a live specialist card — the operator should see the shape of
// the team even before the coordinator uses it.
func (m Model) renderGhostSpecialistCard(width int, reference mango.AgentReference) string {
	name := first(reference.Name, shortID(reference.ID), "specialist")
	head := m.theme.dim.Render("◦  "+truncate(name, max(4, width-6)))
	body := m.theme.dim.Render(trimOneLine("not delegated yet · coordinator has the tool but has not used it", max(4, width-4)))
	return lipgloss.NewStyle().Width(width).Padding(0, 1).
		Border(lipgloss.RoundedBorder()).BorderForeground(m.theme.border).Render(head + "\n" + body)
}

func (m Model) renderSpecialistCard(width int, thread mango.Thread, selected bool) string {
	pip := sessionStatePipFor(m.theme, &thread)
	name := m.theme.title.Render(truncate(first(thread.Agent.Name, "specialist"), max(4, width-12)))
	unread := ""
	if count := m.unread[thread.ID]; count > 0 {
		unread = "  " + m.theme.warning.Render(fmt.Sprintf("+%d", count))
	}
	head := pip + "  " + name + unread
	task := m.delegationPreview(thread.ID)
	if strings.TrimSpace(task) == "" {
		task = "no task delegated yet"
	}
	body := m.theme.dim.Render(trimOneLine(task, max(4, width-4)))
	statusLine := m.theme.dim.Render(ansi.Strip(stateText(m.theme, thread.Status)))
	if thread.Stats.ActiveSeconds > 0 {
		statusLine += m.theme.dim.Render(fmt.Sprintf(" · %.1fs active", thread.Stats.ActiveSeconds))
	}
	content := head + "\n" + body + "\n" + statusLine
	if selected {
		return lipgloss.NewStyle().Width(width).Padding(0, 1).
			Border(lipgloss.RoundedBorder()).BorderForeground(m.theme.accent).Render(content)
	}
	return lipgloss.NewStyle().Width(width).Padding(0, 1).
		Border(lipgloss.RoundedBorder()).BorderForeground(m.theme.border).Render(content)
}

func (m Model) renderPendingPanel(width, height int) string {
	if len(m.pending) == 0 {
		return m.renderCard(width, height, "Needs your input", "", m.theme.dim.Render("Nothing waiting."), false, false)
	}
	rows := make([]string, 0, len(m.pending))
	baseIndex := len(m.threads)
	for index, action := range m.pending {
		selected := m.focus == focusBoard && m.boardCursor == baseIndex+index
		rows = append(rows, m.renderPendingCard(width-4, action, selected))
	}
	body := strings.Join(rows, "\n")
	title := fmt.Sprintf("Needs your input · %d", len(m.pending))
	return m.renderCard(width, height, title, "", body, false, true)
}

func (m Model) renderPendingCard(width int, action mango.Action, selected bool) string {
	pip := m.theme.warning.Render("!")
	name := m.theme.title.Render(truncate(action.Name, max(4, width/2)))
	target := m.theme.dim.Render(m.threadName(action.ThreadID))
	head := pip + "  " + name + "  " + m.theme.dim.Render("on") + " " + target
	input := strings.TrimSpace(action.Input)
	if input == "" {
		input = "(no input preview)"
	}
	body := m.theme.dim.Render(trimOneLine(input, max(4, width-4)))
	content := head + "\n" + body
	border := m.theme.border
	if selected {
		border = m.theme.accent
	}
	return lipgloss.NewStyle().Width(width).Padding(0, 1).
		Border(lipgloss.RoundedBorder()).BorderForeground(border).Render(content)
}

// renderCard is the shared frame for every Board panel — one line of title,
// one dim rule, then the body. Selected panels borrow the accent color so a
// focused card is visible without moving.
func (m Model) renderCard(width, height int, title, pip, body string, selected, warn bool) string {
	border := m.theme.border
	if selected {
		border = m.theme.accent
	} else if warn {
		border = m.theme.yellow
	}
	inner := max(4, width-4)
	head := m.theme.title.Render(truncate(title, inner))
	if pip != "" {
		head = pip + "  " + head
	}
	rule := m.theme.dim.Render(strings.Repeat("─", inner))
	content := head + "\n" + rule + "\n" + body
	return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 1).
		Border(lipgloss.RoundedBorder()).BorderForeground(border).Render(content)
}

func (m Model) coordinatorThread() *mango.Thread {
	for index := range m.threads {
		if m.threads[index].Primary() {
			return &m.threads[index]
		}
	}
	if len(m.threads) > 0 {
		return &m.threads[0]
	}
	return nil
}

func (m Model) specialistThreads() []mango.Thread {
	out := make([]mango.Thread, 0, len(m.threads))
	for _, thread := range m.threads {
		if !thread.Primary() {
			out = append(out, thread)
		}
	}
	return out
}

// sessionStatePipFor is the safe wrapper around sessionStatePip for a possibly
// nil Thread.
func sessionStatePipFor(t theme, thread *mango.Thread) string {
	if thread == nil {
		return t.dim.Render("○")
	}
	return sessionStatePip(t, thread.Status)
}

func latestFeedItem(items []feed.Item, kind feed.Kind) *feed.Item {
	for index := len(items) - 1; index >= 0; index-- {
		if items[index].Kind == kind && strings.TrimSpace(items[index].Body) != "" {
			return &items[index]
		}
	}
	return nil
}
