package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/yanpgwang/mango-terminal/internal/feed"
	"github.com/yanpgwang/mango-terminal/internal/mango"
)

const (
	minTerminalWidth  = 60
	minTerminalHeight = 20
)

type markdownCacheKey struct {
	width   int
	content string
}

type markdownState struct {
	renderers map[int]*glamour.TermRenderer
	entries   map[markdownCacheKey]string
	order     []markdownCacheKey
	next      int
}

const markdownCacheLimit = 2048

func newMarkdownState() markdownState {
	return markdownState{
		renderers: make(map[int]*glamour.TermRenderer),
		entries:   make(map[markdownCacheKey]string),
	}
}

func (m Model) View() tea.View {
	if m.width > 0 && m.height > 0 && (m.width < minTerminalWidth || m.height < minTerminalHeight) {
		view := tea.NewView(m.renderTooSmall())
		view.AltScreen = true
		view.WindowTitle = "Mango"
		view.ReportFocus = true
		return view
	}
	content := m.renderConnect()
	switch m.screen {
	case screenInbox:
		content = m.renderInbox()
	case screenBoard:
		content = m.renderBoard()
	case screenChat:
		content = m.renderWorkspace()
	}
	if m.dialogIsOverlay() && m.width > 0 && m.height > 0 {
		modal := m.renderDialog()
		left := max(0, (m.width-lipgloss.Width(modal))/2)
		top := max(0, (m.height-lipgloss.Height(modal))/2)
		canvas := lipgloss.NewCanvas(max(1, m.width), max(1, m.height))
		// A Layer's X/Y coordinates are interpreted by a Compositor. Drawing a
		// Layer directly onto a Canvas ignores them and paints at (0,0), which
		// left the modal in the corner while the real IME cursor was correctly
		// offset to the intended center position.
		canvas.Compose(lipgloss.NewCompositor(
			lipgloss.NewLayer(content),
			lipgloss.NewLayer(modal).X(left).Y(top).Z(10),
		))
		content = canvas.Render()
	}
	view := tea.NewView(content)
	view.AltScreen = true
	view.WindowTitle = "Mango"
	view.MouseMode = tea.MouseModeCellMotion
	view.ReportFocus = true
	view.Cursor = m.viewCursor()
	return view
}

// viewCursor translates native textarea/textinput cursors into terminal
// coordinates. Besides drawing the caret, this is the anchor terminals expose
// to Chinese, Japanese, and Korean IMEs.
func (m Model) viewCursor() *tea.Cursor {
	if !m.windowFocused || m.width <= 0 || m.height <= 0 ||
		m.width < minTerminalWidth || m.height < minTerminalHeight {
		return nil
	}
	var source *tea.Cursor
	xOffset, yOffset := 0, 0
	if m.dialogIsOverlay() {
		if m.dialogUsesEditor() {
			source = m.editor.Cursor()
			xOffset, yOffset = 5, m.dialogEditorY()
		} else if m.dialogUsesFilter() {
			source = m.filter.Cursor()
			xOffset, yOffset = 3, m.dialogFilterY()
		} else {
			return nil
		}
		modal := m.renderDialog()
		xOffset += max(0, (m.width-lipgloss.Width(modal))/2)
		yOffset += max(0, (m.height-lipgloss.Height(modal))/2)
	} else {
		if m.focus != focusEditor {
			return nil
		}
		if m.screen != screenChat && m.screen != screenBoard {
			return nil
		}
		source = m.editor.Cursor()
		above := m.linesAboveComposer(m.width)
		// Composer sits on the line right after `above`. strings.Join already
		// puts a newline between them, so the composer's first row is exactly
		// `Height(above)` (0-indexed) — no additional +1 offset is needed.
		yOffset = lipgloss.Height(strings.Join(above, "\n"))
	}
	if source == nil {
		return nil
	}
	cur := *source
	cur.X += xOffset
	cur.Y += yOffset

	// A cursor beyond the canvas can make some terminals scroll the alternate
	// screen. Hide it instead when a very short viewport clips its editor.
	if cur.X < 0 || cur.X >= m.width || cur.Y < 0 || cur.Y >= m.height {
		return nil
	}
	return &cur
}

func (m Model) dialogEditorY() int {
	// Dialog border + vertical padding + title/rule/blank + editor box border.
	const dialogChrome = 6
	switch m.dialog {
	case dialogResult:
		// "Result type", blank line, then the editor box.
		return dialogChrome + 2
	case dialogNewSession:
		// Summary, blank, field label, blank, then the editor box.
		return dialogChrome + lipgloss.Height(m.creationEditorSummary(m.dialogWidth())) + 3
	default:
		return dialogChrome
	}
}

func (m Model) dialogFilterY() int {
	// Border + top padding + title + rule + blank line.
	const contentStart = 5
	if m.dialog != dialogNewSession {
		return contentStart
	}
	summaryHeight := lipgloss.Height(m.selectionSummary())
	if m.creation.step == createChooseAgent {
		return contentStart + summaryHeight + 2
	}
	// Environment selection includes a blank line on both sides of its title.
	return contentStart + summaryHeight + 3
}

func (m *Model) resize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	m.compact = m.width < 120 || m.height < 30
	innerWidth := max(20, m.width-2)
	m.editor.MaxHeight = 15
	if m.dialogUsesEditor() {
		innerWidth = max(12, dialogInnerWidth(m.dialogWidth())-4)
		m.editor.MaxHeight = max(3, min(10, m.height-15))
	}
	m.editor.SetWidth(innerWidth)
	m.filter.SetWidth(max(12, dialogInnerWidth(m.dialogWidth())-3))
	composerHeight := m.composerHeight()
	// Chat viewport: header (2) + convInfo (1) + help (1) + a bit of slack.
	chatHeight := max(3, m.height-composerHeight-6)
	m.chat.SetWidth(innerWidth)
	m.chat.SetHeight(chatHeight)
}

// boardBodyHeight is the vertical space between the Board header and the
// composer. Both View() and viewCursor() see the same number so the caret
// never drifts off the composer's real position.
func (m Model) boardBodyHeight() int {
	// Board: header (2) + body + composer + help (1) + a bit of slack.
	return max(3, m.height-m.composerHeight()-6)
}

func (m Model) composerHeight() int {
	return m.editor.Height()
}

func (m Model) dialogIsOverlay() bool {
	return m.dialog == dialogCommands || m.dialog == dialogAgents || m.dialog == dialogInterrupt ||
		m.dialog == dialogNewSession || m.dialog == dialogSessions || m.dialog == dialogAction ||
		m.dialog == dialogResult || m.dialog == dialogHelp || m.dialog == dialogQuit
}

func (m Model) dialogUsesEditor() bool {
	if m.dialog == dialogResult {
		return true
	}
	if m.dialog != dialogNewSession {
		return false
	}
	switch m.creation.step {
	case createAgentName, createAgentModel, createAgentSystem, createAgentMCPName, createAgentMCPURL,
		createEnvironmentName, createSessionTitle, createInitialPrompt:
		return true
	default:
		return false
	}
}

func (m Model) dialogUsesFilter() bool {
	switch m.dialog {
	case dialogCommands, dialogAgents, dialogSessions:
		return true
	case dialogNewSession:
		return m.creation.step == createChooseAgent || m.creation.step == createChooseEnvironment
	default:
		return false
	}
}

func (m Model) renderWorkspace() string {
	width, height := max(1, m.width), max(1, m.height)
	parts := append([]string(nil), m.linesAboveComposer(width)...)
	parts = append(parts,
		m.renderComposer(width),
		m.renderHelp(width),
	)
	return lipgloss.NewStyle().Width(width).Height(height).Render(strings.Join(parts, "\n"))
}

// linesAboveComposer returns the ordered slice of rendered rows that sit above
// the composer for the current screen. Both View() and viewCursor() consume it
// so a change in one place cannot desync the caret from the layout below.
func (m Model) linesAboveComposer(width int) []string {
	if m.screen == screenBoard {
		return []string{
			m.renderBoardHeader(width),
			m.renderBoardBody(width, m.boardBodyHeight()),
		}
	}
	return []string{
		m.renderChatHeader(width),
		lipgloss.NewStyle().Width(width).Height(m.chat.Height()).Padding(0, 1).Render(m.chat.View()),
		m.renderConversationInfo(width),
	}
}

// renderChatHeader labels the zoomed Thread and shows the way back to the
// Board. It replaces the older compact-mode-only header so the operator can
// always see which specialist they are looking at, not just which coordinator.
func (m Model) renderChatHeader(width int) string {
	name := "Agent"
	status := ""
	role := ""
	if m.threadCursor >= 0 && m.threadCursor < len(m.threads) {
		thread := m.threads[m.threadCursor]
		name = first(thread.Agent.Name, name)
		status = stateText(m.theme, thread.Status)
		if thread.Primary() {
			role = "coordinator"
		} else {
			role = "sub-agent"
		}
	}
	back := m.theme.dim.Render("‹ Board") + "  " + m.theme.dim.Render("esc")
	prefixWidth := ansi.StringWidth(ansi.Strip(back)) + 4 + ansi.StringWidth(ansi.Strip(status)) + 3 + ansi.StringWidth(role)
	name = truncate(name, max(4, width-4-prefixWidth))
	middle := m.theme.title.Render(name) + "  " + m.theme.dim.Render(role) + "  " + status
	line := lipgloss.NewStyle().Width(width).Padding(0, 2).Render(joinSides(back, middle, width-4))
	return line + "\n" + m.theme.dim.Render(strings.Repeat("─", width))
}

func (m Model) renderComposer(width int) string {
	if m.dialogUsesEditor() {
		return lipgloss.NewStyle().Width(width).Height(m.composerHeight()).Render("")
	}
	return lipgloss.NewStyle().Width(width).Height(m.composerHeight()).Render(m.editor.View())
}

// renderConversationInfo keeps model and metering context beside the work it
// describes. The Agent rail is for navigation; the conversation footer is for
// the currently selected Thread's runtime details.
func (m Model) renderConversationInfo(width int) string {
	if m.session == nil {
		return lipgloss.NewStyle().Width(width).Render("")
	}
	modelName := first(m.session.Agent.Model.ID, "unknown model")
	usage := m.session.Usage
	activeSeconds := m.session.Stats.ActiveSeconds
	if m.threadCursor >= 0 && m.threadCursor < len(m.threads) {
		thread := m.threads[m.threadCursor]
		modelName = first(thread.Agent.Model.ID, modelName)
		if !thread.Primary() || thread.Usage.InputTokens != 0 || thread.Usage.OutputTokens != 0 || thread.Usage.CacheReadInputTokens != 0 {
			usage = thread.Usage
		}
		if !thread.Primary() || thread.Stats.ActiveSeconds != 0 {
			activeSeconds = thread.Stats.ActiveSeconds
		}
	}
	text := "◇ " + modelName + "  ·  " + compactTokens(usage.InputTokens) + " in  /  " +
		compactTokens(usage.OutputTokens) + " out"
	if activeSeconds > 0 {
		text += fmt.Sprintf("  ·  %.1fs", activeSeconds)
	}
	text = truncate(text, max(1, width-4))
	return lipgloss.NewStyle().Width(width).Padding(0, 2).Align(lipgloss.Right).Render(m.theme.dim.Render(text))
}

func (m Model) renderHelp(width int) string {
	var text string
	switch m.focus {
	case focusEditor:
		if m.screen == screenBoard {
			text = "enter send  shift+enter newline  tab board  ctrl+n new  ctrl+p commands  esc back"
			if width < 80 {
				text = "enter send  tab board  ctrl+p commands  esc back"
			}
		} else {
			text = "enter send  shift+enter newline  tab chat  ctrl+p commands  esc board"
			if width < 80 {
				text = "enter send  tab chat  ctrl+p commands  esc board"
			}
			if width >= 110 {
				text += "  ctrl+g agents"
			}
		}
	case focusBoard:
		text = "↑↓ pick  enter zoom  z zoom  tab compose  esc Sessions"
		if width < 80 {
			text = "↑↓ pick  enter zoom  tab compose  esc"
		}
	case focusChat:
		text = "↑↓ scroll  enter expand  tab editor  esc board"
		if width < 80 {
			text = "↑↓ scroll  enter inspect  tab editor"
		}
		if width >= 110 {
			text = "↑↓ scroll  shift+↑↓ select  enter inspect  tab editor  esc board"
		}
	}
	if m.err != nil {
		text = m.theme.danger.Render(trimOneLine(m.err.Error(), width-4))
		return lipgloss.NewStyle().Width(width).Padding(0, 2).Render(text)
	}
	if m.loading || m.reconnecting {
		label := first(m.loadingLabel, "Reconnecting")
		return lipgloss.NewStyle().Width(width).Padding(0, 2).Render(m.activity(label))
	}
	return lipgloss.NewStyle().Width(width).Padding(0, 2).Render(m.theme.dim.Render(text))
}

// delegationPreview returns the first task that landed on this Thread, or the
// empty string if the child has not been briefed yet. It is intentionally the
// earliest inbound message because Managed coordinators delegate once at the
// start of a Thread and only chase up with clarifications.
func (m Model) delegationPreview(threadID string) string {
	for _, event := range m.events[threadID] {
		if event.Type() != "agent.thread_message_received" {
			continue
		}
		if text := strings.TrimSpace(feed.ContentText(event["content"])); text != "" {
			if newline := strings.IndexByte(text, '\n'); newline >= 0 {
				text = text[:newline]
			}
			return text
		}
	}
	return ""
}

func (m *Model) renderChat() {
	if m.threadCursor < 0 || m.threadCursor >= len(m.threads) {
		m.chat.SetContent(m.theme.dim.Render("Waiting for the primary Agent…"))
		return
	}
	items := feed.Project(m.threads[m.threadCursor], m.events[m.currentThreadID()])
	if len(items) > 0 && m.itemCursor >= len(items) {
		m.itemCursor = len(items) - 1
	}
	parts := make([]string, 0, len(items)+1)
	contentWidth := min(100, max(30, m.chat.Width()))
	for index, item := range items {
		parts = append(parts, m.renderFeedItem(item, index == m.itemCursor, contentWidth))
	}
	if current := m.previews[m.currentThreadID()]; current != nil {
		live := ""
		if strings.TrimSpace(current.content) != "" {
			streamed := strings.TrimRight(m.renderMarkdown(current.content, contentWidth-4, false), "\n")
			live = lipgloss.NewStyle().PaddingLeft(2).Render(streamed + " " + m.streamCaret())
		} else if current.messageID != "" {
			live = lipgloss.NewStyle().PaddingLeft(2).Render(m.replyActivity())
		} else if current.waiting || current.thinkingID != "" {
			live = lipgloss.NewStyle().PaddingLeft(2).Render(m.fruitThinking())
		}
		if live != "" {
			parts = append(parts, live)
		}
	}
	if len(parts) == 0 {
		parts = append(parts, m.theme.dim.Render("No messages yet. Ask the coordinator to begin."))
	}
	m.chat.SetContent(strings.Join(parts, "\n\n"))
	if m.follow {
		m.chat.GotoBottom()
	}
}

func (m *Model) renderFeedItem(item feed.Item, selected bool, width int) string {
	body := ""
	if strings.TrimSpace(item.Body) != "" {
		body = m.renderMarkdown(item.Body, width-3, true)
	}
	selectedStyle := func(content string) string {
		content = m.spark(item.ID) + content
		if !selected {
			return lipgloss.NewStyle().PaddingLeft(2).Render(content)
		}
		return lipgloss.NewStyle().PaddingLeft(1).BorderLeft(true).
			BorderStyle(lipgloss.Border{Left: "▌"}).BorderForeground(m.theme.green).Render(content)
	}

	switch item.Kind {
	case feed.User:
		if body == "" {
			return ""
		}
		return lipgloss.NewStyle().PaddingLeft(1).BorderLeft(true).
			BorderStyle(lipgloss.Border{Left: "▌"}).BorderForeground(m.theme.accent).Render(body)
	case feed.Agent:
		return selectedStyle(body)
	case feed.Tool:
		status := item.Status
		statusStyle := m.theme.dim
		if status == "waiting" {
			statusStyle = m.theme.warning
		}
		if status == "error" {
			statusStyle = m.theme.danger
		}
		if status == "done" {
			statusStyle = m.theme.success
		}
		label := m.theme.dim.Render("✱") + " " + m.theme.title.Render(item.Title) + " " + statusStyle.Render(status)
		if m.expanded[item.ID] {
			details := []string{label}
			if strings.TrimSpace(item.Detail) != "" {
				details = append(details, m.theme.dim.Render(item.Detail))
			}
			if body != "" {
				details = append(details, body)
			}
			return selectedStyle(strings.Join(details, "\n"))
		}
		return selectedStyle(label)
	case feed.Delegation:
		label := m.theme.warning.Render("↗ " + item.Title)
		if body != "" {
			label += "\n" + body
		}
		return selectedStyle(label)
	case feed.Report:
		label := m.theme.success.Render("↙ " + item.Title)
		if body != "" {
			label += "\n" + body
		}
		return selectedStyle(label)
	case feed.Thinking, feed.Notice:
		return selectedStyle(m.theme.dim.Render(item.Title))
	case feed.Failure:
		label := m.theme.danger.Render(item.Title)
		if body != "" {
			label += "\n" + body
		}
		return selectedStyle(label)
	}
	return selectedStyle(body)
}

func (m Model) renderInbox() string {
	width, height := max(1, m.width), max(1, m.height)
	main := m.renderInboxMain(width, height-2)
	helpText := "←→ pill  ↑↓ list  enter select  n new  / find  r refresh  esc disconnect  ? help"
	if width < 90 {
		helpText = "←→ pill  ↑↓ list  enter select  n new  esc"
	}
	help := m.theme.dim.Render(helpText)
	fleet := m.renderInboxFleetSummary()
	footer := lipgloss.NewStyle().Width(width).Padding(0, 2).Render(joinSides(help, fleet, width-4))
	return lipgloss.NewStyle().Width(width).Height(height).Render(main + "\n" + footer)
}

func (m Model) renderInboxMain(width, height int) string {
	// Toolbar sits at the very top of the interactive area — nothing above it
	// except the terminal's own top margin.
	toolbar := lipgloss.NewStyle().Padding(1, 2, 0, 2).Render(m.renderInboxToolbar())

	used := lipgloss.Height(toolbar) + 1 // toolbar + blank line
	gridHeight := max(6, height-used)

	sessionCount := len(m.sessions)
	twoColumn := !m.compact && width >= 120 && sessionCount > 0
	var grid string
	if twoColumn {
		listWidth := width * 55 / 100
		previewWidth := width - listWidth - 2
		list := m.renderInboxList(listWidth, gridHeight)
		preview := m.renderInboxPreview(previewWidth, gridHeight)
		grid = lipgloss.JoinHorizontal(lipgloss.Top, list, "  ", preview)
	} else {
		grid = m.renderInboxList(width, gridHeight)
	}
	return strings.Join([]string{toolbar, "", grid}, "\n")
}

// renderInboxFleetSummary is the compact fleet status the footer shows on the
// right. Loading and error states hijack it because their signal outranks the
// steady-state counts anyway.
func (m Model) renderInboxFleetSummary() string {
	if m.loading {
		return m.activity(first(m.loadingLabel, "Refreshing Sessions"))
	}
	if m.err != nil {
		return m.theme.danger.Render(trimOneLine(m.err.Error(), 60))
	}
	counts := summarizeFleet(m.sessions)
	parts := []string{m.theme.success.Render("CONNECTED")}
	if counts.needsAction > 0 {
		parts = append(parts, m.theme.warning.Render(fmt.Sprintf("%d needs input", counts.needsAction)))
	}
	if counts.running > 0 {
		parts = append(parts, m.theme.active.Render(fmt.Sprintf("%d running", counts.running)))
	}
	if counts.idle > 0 {
		parts = append(parts, m.theme.dim.Render(fmt.Sprintf("%d idle", counts.idle)))
	}
	if counts.other > 0 {
		parts = append(parts, m.theme.dim.Render(fmt.Sprintf("%d other", counts.other)))
	}
	return strings.Join(parts, m.theme.dim.Render("  ·  "))
}

// renderInboxToolbar prints the three top-of-page action buttons. Sitting on
// the cursor still highlights the button the same way it used to highlight a
// list row, so muscle memory (Enter on cursor 0 = New Session) is preserved.
func (m Model) renderInboxToolbar() string {
	labels := []string{"+ New", "/ Find", "↻ Refresh"}
	pills := make([]string, 0, 3)
	for index, label := range labels {
		pills = append(pills, choice(m.theme, label, m.inboxCursor == index, false))
	}
	return strings.Join(pills, "  ")
}

// renderInboxList is the durable-Session list column. Rows are compact but
// carry every fleet signal that matters at a glance: state pip, title, state
// text, activity time, sub-agent count, and the recently-attached badge.
func (m Model) renderInboxList(width, height int) string {
	if len(m.sessions) == 0 {
		body := m.theme.dim.Render("No durable Sessions yet. Press ") +
			m.theme.active.Render("n") + m.theme.dim.Render(" to start one.")
		return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 2).Render(body)
	}
	contentWidth := max(20, width-4)
	rows := make([]string, 0, len(m.sessions))
	rowHeight := 2
	visible := max(2, (height-1)/(rowHeight+1))
	visible = min(visible, len(m.sessions))
	listCursor := m.inboxCursor - 3
	if listCursor < 0 {
		listCursor = 0
	}
	start, end := visibleRange(len(m.sessions), listCursor, visible)
	now := time.Now()
	for index := start; index < end; index++ {
		rows = append(rows, m.renderInboxSessionRow(m.sessions[index], contentWidth, index+3 == m.inboxCursor, now))
	}
	body := strings.Join(rows, "\n\n")
	return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 2).Render(body)
}

// renderInboxSessionRow renders one durable Session as a two-line entry. The
// first line carries the state pip, title, and a small marker if this is the
// Session the user just detached from. The second line packs Agent identity,
// subagent-count, and relative activity so a glance separates a live turn from
// an idle Session.
func (m Model) renderInboxSessionRow(session mango.Session, contentWidth int, selected bool, now time.Time) string {
	marker := "  "
	titleStyle := lipgloss.NewStyle().Foreground(m.theme.text)
	if selected {
		marker, titleStyle = "› ", m.theme.active
	}
	pip := sessionStatePip(m.theme, session.Status)
	returnMark := ""
	if session.ID != "" && session.ID == m.lastAttachedID {
		returnMark = "  " + m.theme.active.Render("⤴")
	}
	nameWidth := max(12, contentWidth-ansi.StringWidth(marker)-ansi.StringWidth(pip)-1-
		ansi.StringWidth(ansi.Strip(returnMark))-1-ansi.StringWidth(stateText(m.theme, session.Status))-1)
	name := truncate(first(session.Title, session.Agent.Name, session.ID), nameWidth)
	head := marker + pip + " " + titleStyle.Render(name) + "  " + stateText(m.theme, session.Status) + returnMark

	metaParts := []string{first(session.Agent.Name, "Agent")}
	if model := strings.TrimSpace(session.Agent.Model.ID); model != "" {
		metaParts = append(metaParts, model)
	}
	if children := subagentCount(session); children > 0 {
		metaParts = append(metaParts, fmt.Sprintf("%d sub-agents", children))
	}
	if since := humanizeSince(recency(session), now); since != "" {
		metaParts = append(metaParts, since)
	}
	metaParts = append(metaParts, shortID(session.ID))
	meta := strings.Join(metaParts, " · ")
	return head + "\n    " + m.theme.dim.Render(trimOneLine(meta, contentWidth-4))
}

// renderInboxPreview is the right-side detail card shown on wide terminals.
// It reveals everything the operator would need to decide "attach or not"
// without reading the compact left list row twice.
func (m Model) renderInboxPreview(width, height int) string {
	if m.inboxCursor < 3 {
		return m.renderToolbarHelpCard(width, height)
	}
	index := m.inboxCursor - 3
	if index < 0 || index >= len(m.sessions) {
		return lipgloss.NewStyle().Width(width).Height(height).Render("")
	}
	session := m.sessions[index]
	inner := max(4, width-6)
	title := m.theme.title.Render(truncate(first(session.Title, "Untitled Session"), inner))
	rule := m.theme.dim.Render(strings.Repeat("─", inner))
	rows := []string{title, stateText(m.theme, session.Status), rule}
	rows = append(rows, m.previewField("Agent", first(session.Agent.Name, "unknown"), inner))
	if model := strings.TrimSpace(session.Agent.Model.ID); model != "" {
		rows = append(rows, m.previewField("Model", model, inner))
	}
	if env := strings.TrimSpace(session.EnvironmentID); env != "" {
		rows = append(rows, m.previewField("Environment", shortID(env), inner))
	}
	rows = append(rows, m.previewField("Sub-agents", m.previewSubagentsValue(session), inner))
	if session.Usage.InputTokens+session.Usage.OutputTokens > 0 {
		rows = append(rows, m.previewField("Tokens",
			compactTokens(session.Usage.InputTokens)+" in  /  "+compactTokens(session.Usage.OutputTokens)+" out", inner))
	}
	if session.Stats.ActiveSeconds > 0 {
		rows = append(rows, m.previewField("Active", fmt.Sprintf("%.1fs", session.Stats.ActiveSeconds), inner))
	}
	if since := humanizeSince(recency(session), time.Now()); since != "" {
		rows = append(rows, m.previewField("Activity", since, inner))
	}
	rows = append(rows, m.previewField("ID", shortID(session.ID), inner))
	if session.ID != "" && session.ID == m.lastAttachedID {
		rows = append(rows, "", m.theme.active.Render("⤴ recently attached"))
	}
	rows = append(rows, "", m.theme.dim.Render("enter to attach"))
	body := strings.Join(rows, "\n")
	return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 1).
		Border(lipgloss.RoundedBorder()).BorderForeground(m.theme.border).Render(body)
}

// renderToolbarHelpCard shows a short explanation for the pill the operator's
// cursor is currently sitting on. It fills the right column when no Session
// is highlighted so the pane never renders as dead space.
func (m Model) renderToolbarHelpCard(width, height int) string {
	inner := max(4, width-6)
	title := "Getting started"
	body := "Move the cursor down to inspect a Session, or press Enter on a pill."
	switch m.inboxCursor {
	case 0:
		title, body = "Create a new Session", "Choose an Agent, an Environment, a title, and an optional first message. The Session is durable — you can detach and come back to it any time."
	case 1:
		title, body = "Find a Session", "Search every durable Session by title, Agent name, or ID. Useful when the fleet grows beyond the visible list."
	case 2:
		title, body = "Refresh from Cloud", "Re-fetch the Session list from Mango. Sessions keep running in the cloud after you detach, so their state may have moved since you last looked."
	}
	head := m.theme.title.Render(truncate(title, inner))
	rule := m.theme.dim.Render(strings.Repeat("─", inner))
	content := head + "\n" + rule + "\n" + m.theme.dim.Render(lipgloss.NewStyle().Width(inner).Render(body))
	return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 1).
		Border(lipgloss.RoundedBorder()).BorderForeground(m.theme.border).Render(content)
}

func (m Model) previewField(label, value string, width int) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	labelWidth := 12
	label = m.theme.dim.Render(fmt.Sprintf("%-*s", labelWidth, label))
	value = truncate(value, max(4, width-labelWidth-2))
	return label + "  " + value
}

func (m Model) previewSubagentsValue(session mango.Session) string {
	roster := subagentCount(session)
	if roster == 0 {
		return "single Agent"
	}
	return fmt.Sprintf("%d in roster", roster)
}

func sessionStatePip(t theme, status string) string {
	switch status {
	case "running", "rescheduling":
		return t.active.Render("●")
	case "requires_action":
		return t.warning.Render("!")
	case "terminated", "failed", "error":
		return t.danger.Render("○")
	default:
		return t.dim.Render("○")
	}
}

func (m Model) renderConnect() string {
	width, height := max(1, m.width), max(1, m.height)
	contentWidth := min(72, max(34, width-8))
	endpoint := first(strings.TrimSpace(m.options.Endpoint), "Mango Cloud")
	lines := []string{
		m.brandLogo(false),
		"",
		gradientText(lipgloss.NewStyle(), "Your window into managed Agents", true, m.theme.accent, m.theme.blue),
		m.theme.dim.Render("Sessions keep running in the cloud after you detach."),
		"",
		m.theme.title.Render("Cloud"),
		m.theme.dim.Render(truncate(endpoint, contentWidth)),
		"",
	}
	if m.loading {
		lines = append(lines, m.activity(first(m.loadingLabel, "Connecting to Mango Cloud")))
	} else {
		lines = append(lines, choice(m.theme, "Connect", true, false), "", m.theme.dim.Render("enter connect  ctrl+c quit"))
	}
	if m.err != nil {
		lines = append(lines, "", m.theme.danger.Render(trimOneLine(m.err.Error(), contentWidth)))
	}
	card := lipgloss.NewStyle().Width(contentWidth).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, card)
}

func (m Model) renderDialog() string {
	width := m.dialogWidth()
	innerWidth := dialogInnerWidth(width)
	content := ""
	title := ""
	switch m.dialog {
	case dialogCommands:
		title = "Commands"
		rows := []string{}
		commands := m.filteredCommands()
		start, end := visibleRange(len(commands), m.dialogCursor, m.dialogListSize(9))
		for index := start; index < end; index++ {
			command := commands[index]
			marker := "  "
			style := lipgloss.NewStyle().Foreground(m.theme.text)
			if index == m.dialogCursor {
				marker, style = "› ", m.theme.active
			}
			rows = append(rows, marker+style.Render(command.title)+"\n    "+m.theme.dim.Render(command.help))
		}
		if len(rows) == 0 {
			rows = append(rows, m.theme.dim.Render("No matching commands."))
		}
		content = m.renderCreationSearch(m.dialogQuery, "Type to filter commands") + "\n\n" +
			strings.Join(rows, "\n") + "\n\n" + m.theme.dim.Render(m.dialogHint(
			"type filter  ↑↓ choose  enter run  esc", "type  ↑↓ choose  enter run  esc"))
	case dialogAgents:
		title = "Agents"
		threads := m.filteredThreads()
		start, end := visibleRange(len(threads), m.dialogCursor, m.dialogListSize(9))
		rows := make([]string, 0, end-start)
		for index := start; index < end; index++ {
			thread := threads[index]
			marker := "  "
			style := lipgloss.NewStyle().Foreground(m.theme.text)
			if index == m.dialogCursor {
				marker, style = "› ", m.theme.active
			}
			role := "specialist"
			if thread.Primary() {
				role = "coordinator"
			}
			unread := ""
			if count := m.unread[thread.ID]; count > 0 {
				unread = m.theme.warning.Render(fmt.Sprintf(" +%d", count))
			}
			nameWidth := max(8, innerWidth-ansi.StringWidth(marker)-4-ansi.StringWidth(unread))
			name := truncate(first(thread.Agent.Name, "Agent"), nameWidth)
			rows = append(rows, marker+style.Render(name)+unread+
				"\n    "+m.theme.dim.Render(role+" · ")+stateText(m.theme, thread.Status))
		}
		if len(rows) == 0 {
			rows = append(rows, m.theme.dim.Render("No matching Agents."))
		}
		content = m.renderCreationSearch(m.dialogQuery, "Type to filter Agents") + "\n\n" +
			strings.Join(rows, "\n") + "\n\n" + m.theme.dim.Render(m.dialogHint(
			"type filter  ↑↓ choose  enter view  esc", "type  ↑↓ choose  enter view  esc"))
	case dialogAction:
		title = fmt.Sprintf("Pending actions · %d", len(m.pending))
		if len(m.pending) == 0 {
			content = "Nothing is waiting."
			break
		}
		action := m.pending[clamp(m.actionCursor, 0, len(m.pending)-1)]
		thread := m.threadName(action.ThreadID)
		detail, omitted := boundedDialogText(action.Input, innerWidth, max(1, min(10, m.height-17)))
		header := trimOneLine(action.Name+" · "+thread, innerWidth)
		content = m.theme.warning.Render(header) + "\n\n" +
			m.theme.dim.Render(detail)
		if omitted > 0 {
			content += "\n" + m.theme.warning.Render(fmt.Sprintf("… %d more line(s) · expand the tool event in chat", omitted))
		}
		content += "\n\n"
		if action.Kind == mango.ActionConfirmation {
			content += choice(m.theme, "Allow once", m.actionChoice == 0, false) + "  " + choice(m.theme, "Deny", m.actionChoice == 1, true) +
				"\n\n" + m.theme.dim.Render(m.dialogHint(
				"←→ choose  enter confirm  a allow  d deny  esc close", "←→ choose  enter confirm  a allow  d deny"))
		} else {
			content += choice(m.theme, "Provide result", true, false) + "\n\n" + m.theme.dim.Render("enter continue  esc close")
		}
		if m.loading {
			content += "\n\n" + m.activity("Submitting decision")
		}
	case dialogResult:
		title = "Return tool result"
		state := m.theme.success.Render("success")
		if m.resultError {
			state = m.theme.danger.Render("error")
		}
		box := lipgloss.NewStyle().Width(innerWidth).Border(lipgloss.RoundedBorder()).BorderForeground(m.theme.accent).Padding(0, 1).Render(m.editor.View())
		content = "Result type: " + state + "\n\n" + box + "\n\n" + m.theme.dim.Render(m.dialogHint(
			"enter submit  ctrl+e success/error  esc back", "enter submit  ctrl+e type  esc back"))
		if m.loading {
			content += "\n\n" + m.activity("Submitting result")
		}
	case dialogInterrupt:
		title = "Interrupt Agent?"
		name := "current Agent"
		if m.threadCursor >= 0 && m.threadCursor < len(m.threads) {
			name = first(m.threads[m.threadCursor].Agent.Name, name)
		}
		name = truncate(name, max(8, innerWidth-24))
		content = "Cancel active work on " + m.theme.warning.Render(name) + "?\nThe durable Session and history stay intact.\n\n" +
			choice(m.theme, "Interrupt", m.dialogCursor == 0, true) + "  " + choice(m.theme, "Keep running", m.dialogCursor == 1, false) +
			"\n\n" + m.theme.dim.Render("←→ choose  enter confirm  esc close")
	case dialogNewSession:
		title, content = m.renderCreationDialog(width)
	case dialogSessions:
		title = "Sessions"
		filtered := m.filteredSessions()
		rows := make([]string, 0, min(len(filtered), 9))
		start, end := visibleRange(len(filtered), m.inboxCursor, m.dialogListSize(9))
		for index := start; index < end; index++ {
			session := filtered[index]
			marker, style := "  ", lipgloss.NewStyle().Foreground(m.theme.text)
			if index == m.inboxCursor {
				marker, style = "› ", m.theme.active
			}
			name := first(session.Title, session.Agent.Name, session.ID)
			rows = append(rows, marker+style.Render(truncate(name, width-18))+"  "+stateText(m.theme, session.Status)+
				"\n    "+m.theme.dim.Render(trimOneLine(first(session.Agent.Name, "Agent")+" · "+shortID(session.ID), innerWidth-4)))
		}
		if len(rows) == 0 {
			rows = append(rows, m.theme.dim.Render("No matching Sessions."))
		}
		content = m.renderCreationSearch(m.inboxFilter, "Type to filter Sessions") + "\n\n" +
			strings.Join(rows, "\n") + "\n\n" +
			m.theme.dim.Render(m.dialogHint(
				"type filter  ↑↓ choose  enter open  ctrl+n new  esc", "type  ↑↓ choose  enter open  ctrl+n new"))
		if m.loading {
			content += "\n\n" + m.activity("Opening Session")
		} else if m.err != nil {
			content += "\n\n" + m.theme.danger.Render(trimOneLine(m.err.Error(), width-8))
		}
	case dialogHelp:
		title = "Keyboard help"
		helpLines := []string{
			m.theme.title.Render("Create and navigate"),
			"  enter        send or confirm",
			"  shift+enter  insert a newline",
			"  ctrl+n       create a Session",
			"  ctrl+s       search Sessions",
			"  ctrl+p       command palette",
			"  ctrl+g       switch Agent view",
			"",
			m.theme.title.Render("Inspect a conversation"),
			"  tab          move between editor and chat",
			"  ↑↓ / j k     scroll chat",
			"  shift+↑↓     select an event",
			"  enter/space  toggle details",
			"  ←→ / h l     collapse or expand",
			"",
			m.theme.dim.Render("f1 help  esc close  ctrl+c quit"),
		}
		if m.height < 24 {
			helpLines = []string{
				m.theme.title.Render("Mango shortcuts"),
				"  enter / shift+enter   send / newline",
				"  ctrl+n / ctrl+s       new / Sessions",
				"  ctrl+p / ctrl+g       commands / Agents",
				"  tab                   editor / chat",
				"  ↑↓ / shift+↑↓         scroll / select",
				"  enter / ←→            inspect event",
				m.theme.dim.Render("f1 help  esc close  ctrl+c quit"),
			}
		}
		content = strings.Join(helpLines, "\n")
	case dialogQuit:
		title = "Quit Mango?"
		content = lipgloss.JoinVertical(lipgloss.Center,
			"Managed Sessions and Agents keep running in the cloud.",
			"Leaving only detaches this terminal.",
			"",
			choice(m.theme, "Quit", m.dialogCursor == 0, true)+"  "+
				choice(m.theme, "Keep working", m.dialogCursor == 1, false),
			"",
			m.theme.dim.Render(m.dialogHint(
				"←→ choose  enter confirm  y/n decide  ctrl+c again quits  esc close",
				"←→ choose  enter confirm  ctrl+c again quits")),
		)
	}
	box := m.dialogTitle(title) + "\n" + m.dialogRule(innerWidth) + "\n\n" + content
	return lipgloss.NewStyle().Width(width).Padding(1, 2).Background(m.theme.panel).
		Border(lipgloss.RoundedBorder()).BorderForeground(m.theme.border).Render(box)
}

func (m Model) threadName(id string) string {
	for _, thread := range m.threads {
		if thread.ID == id {
			return first(thread.Agent.Name, shortID(id))
		}
	}
	return shortID(id)
}

func (m *Model) renderMarkdown(content string, width int, cache bool) string {
	width = max(20, width)
	key := markdownCacheKey{width: width, content: content}
	if cache {
		if rendered, ok := m.markdown.entries[key]; ok {
			return rendered
		}
	}
	renderer := m.markdown.renderers[width]
	if renderer == nil {
		var err error
		renderer, err = glamour.NewTermRenderer(glamour.WithStandardStyle("dark"), glamour.WithWordWrap(width))
		if err != nil {
			return content
		}
		m.markdown.renderers[width] = renderer
	}
	rendered, err := renderer.Render(content)
	if err != nil {
		return content
	}
	rendered = strings.TrimSpace(rendered)
	if cache {
		if len(m.markdown.order) < markdownCacheLimit {
			m.markdown.order = append(m.markdown.order, key)
		} else {
			oldest := m.markdown.order[m.markdown.next]
			delete(m.markdown.entries, oldest)
			m.markdown.order[m.markdown.next] = key
			m.markdown.next = (m.markdown.next + 1) % markdownCacheLimit
		}
		m.markdown.entries[key] = rendered
	}
	return rendered
}

func stateText(t theme, status string) string {
	switch status {
	case "running", "rescheduling":
		return t.warning.Render(status)
	case "idle":
		return t.dim.Render("idle")
	case "requires_action":
		return t.warning.Render("needs input")
	case "terminated", "error", "failed":
		return t.danger.Render(status)
	default:
		return t.dim.Render(first(status, "unknown"))
	}
}

func choice(t theme, text string, selected, danger bool) string {
	style := lipgloss.NewStyle().Foreground(t.text).Background(t.soft).Padding(0, 1)
	if selected {
		style = style.Background(t.accent).Bold(true)
	}
	if danger && selected {
		style = style.Background(t.red)
	}
	return style.Render(text)
}

func joinSides(left, right string, width int) string {
	space := max(1, width-ansi.StringWidth(left)-ansi.StringWidth(right))
	return left + strings.Repeat(" ", space) + right
}

func first(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
func shortID(value string) string {
	width := ansi.StringWidth(value)
	if width <= 15 {
		return value
	}
	return ansi.Cut(value, 0, 9) + "…" + ansi.Cut(value, width-4, width)
}
func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(value) <= width {
		return value
	}
	return ansi.Truncate(value, width, "…")
}

func (m Model) dialogWidth() int {
	if m.dialog == dialogQuit {
		return min(56, max(40, m.width-8))
	}
	if m.dialog == dialogNewSession {
		// Keep the creation wizard as a focused center card with visible
		// breathing room around it, even on wide terminals.
		return min(66, max(52, m.width-12))
	}
	return min(70, max(40, m.width-8))
}

func dialogInnerWidth(width int) int {
	return max(1, width-6)
}

func (m Model) dialogListSize(limit int) int {
	return max(1, min(limit, (m.height-12)/2))
}

func (m Model) dialogHint(full, compact string) string {
	if dialogInnerWidth(m.dialogWidth()) < 60 {
		return compact
	}
	return full
}

func (m Model) creationListSize(limit int) int {
	return max(1, min(limit, (m.height-18)/2))
}

func (m Model) renderTooSmall() string {
	width, height := max(1, m.width), max(1, m.height)
	message := truncate(fmt.Sprintf("Mango needs at least %d×%d · resize this terminal", minTerminalWidth, minTerminalHeight), width)
	return lipgloss.NewStyle().Width(width).Height(height).Align(lipgloss.Center, lipgloss.Center).
		Render(m.theme.active.Render(message))
}

func boundedDialogText(value string, width, maxLines int) (string, int) {
	const maxSourceBytes = 64 << 10
	value = capUTF8(value, maxSourceBytes)
	wrapped := ansi.Hardwrap(value, max(1, width), true)
	lines := strings.Split(wrapped, "\n")
	if len(lines) <= maxLines {
		return wrapped, 0
	}
	return strings.Join(lines[:maxLines], "\n"), len(lines) - maxLines
}
func trimOneLine(value string, width int) string {
	return truncate(strings.Join(strings.Fields(value), " "), width)
}
func commas(value int64) string {
	text := fmt.Sprintf("%d", value)
	for index := len(text) - 3; index > 0; index -= 3 {
		text = text[:index] + "," + text[index:]
	}
	return text
}

func compactTokens(value int64) string {
	switch {
	case value >= 1_000_000:
		formatted := fmt.Sprintf("%.1fM", float64(value)/1_000_000)
		return strings.Replace(formatted, ".0M", "M", 1)
	case value >= 1_000:
		formatted := fmt.Sprintf("%.1fK", float64(value)/1_000)
		return strings.Replace(formatted, ".0K", "K", 1)
	default:
		return fmt.Sprintf("%d", value)
	}
}
