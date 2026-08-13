package ui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/yanpgwang/mango-terminal/internal/api"
)

const (
	viewInbox = iota
	viewAttached
)

const (
	focusComposer = iota
	focusTimeline
	focusThreads
)

const (
	overlayNone = iota
	overlayCommands
	overlayPermissions
)

type inboxLoaded struct {
	sessions []api.Session
	err      error
}

type attachedLoaded struct {
	session api.Session
	threads []api.Thread
	events  map[string][]api.Event
	err     error
}

type summaryLoaded struct {
	session api.Session
	threads []api.Thread
	err     error
}

type streamMessage struct {
	stream <-chan api.StreamUpdate
	update api.StreamUpdate
	open   bool
}

type actionFinished struct {
	label string
	err   error
}

type refreshTick time.Time
type reconnectStreams time.Time

type preview struct {
	id      string
	typ     string
	content string
}

type pendingAction struct {
	event    api.Event
	kind     string
	threadID string
}

type Model struct {
	client *api.Client

	width  int
	height int
	view   int

	sessions    []api.Session
	inboxCursor int
	filter      string
	filtering   bool

	session      *api.Session
	threads      []api.Thread
	threadCursor int
	events       map[string][]api.Event
	previews     map[string]*preview
	unread       map[string]int

	viewport      viewport.Model
	composer      textarea.Model
	timeline      timelineView
	spinner       spinner.Model
	theme         theme
	focus         int
	overlay       int
	overlayCursor int
	pending       []pendingAction

	loading bool
	status  string
	err     error

	stream       <-chan api.StreamUpdate
	streamCancel context.CancelFunc

	directAttach string
}

func New(client *api.Client, directAttach string) Model {
	colors := defaultTheme()
	composer := textarea.New()
	composer.Placeholder = "Send a message to the primary Agent"
	composer.Prompt = "> "
	composer.ShowLineNumbers = false
	composer.DynamicHeight = true
	composer.MinHeight = 1
	composer.MaxHeight = 5
	composer.CharLimit = 64 << 10
	composer.SetVirtualCursor(true)
	composer.SetStyles(colors.textareaStyles())
	composer.Focus()

	transcript := viewport.New()
	transcript.SoftWrap = true
	transcript.FillHeight = true
	transcript.MouseWheelEnabled = true
	workSpinner := spinner.New(
		spinner.WithSpinner(spinner.Spinner{
			Frames: []string{"∙  ", "•• ", "•••", " ••", "  ∙", "   "},
			FPS:    140 * time.Millisecond,
		}),
		spinner.WithStyle(colors.dim),
	)

	return Model{
		client: client, view: viewInbox, loading: true,
		events: map[string][]api.Event{}, previews: map[string]*preview{},
		unread: map[string]int{}, viewport: transcript, composer: composer,
		timeline: newTimelineView(), spinner: workSpinner, theme: colors,
		focus:        focusComposer,
		directAttach: strings.TrimSpace(directAttach),
	}
}

func (m Model) Init() tea.Cmd {
	var load tea.Cmd
	if m.directAttach != "" {
		load = m.loadAttached(m.directAttach)
	} else {
		load = m.loadInbox()
	}
	return tea.Batch(load, m.spinner.Tick)
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		m.renderTimeline()
		return m, nil
	case spinner.TickMsg:
		var command tea.Cmd
		m.spinner, command = m.spinner.Update(msg)
		if m.view == viewAttached && m.previews[m.currentThreadID()] != nil {
			m.renderTimeline()
		}
		return m, command
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" || msg.String() == "ctrl+q" {
			m.stopStreams()
			return m, tea.Quit
		}
	case inboxLoaded:
		m.loading, m.err = false, msg.err
		if msg.err == nil {
			m.sessions = msg.sessions
			m.inboxCursor = clamp(m.inboxCursor, 0, len(m.filteredSessions())-1)
			m.status = "connected"
		}
		return m, nil
	case attachedLoaded:
		m.loading, m.err = false, msg.err
		if msg.err != nil {
			return m, nil
		}
		selectedThread := ""
		if m.session != nil && m.session.ID == msg.session.ID {
			selectedThread = m.currentThreadID()
		}
		m.session = &msg.session
		m.threads, m.events = msg.threads, msg.events
		m.rebuildPending()
		m.threadCursor = threadIndex(m.threads, selectedThread)
		if m.threadCursor < 0 {
			m.threadCursor = primaryIndex(m.threads)
		}
		m.view, m.status = viewAttached, "attached"
		m.focus, m.overlay = focusComposer, overlayNone
		m.composer.Focus()
		m.renderTimeline()
		return m, tea.Batch(m.restartStreams(), m.refreshAfter())
	case summaryLoaded:
		m.loading, m.err = false, msg.err
		if msg.err == nil && m.session != nil && msg.session.ID == m.session.ID {
			*m.session = msg.session
			if !sameThreadRoster(m.threads, msg.threads) {
				selected := m.currentThreadID()
				m.threads = msg.threads
				m.threadCursor = threadIndex(m.threads, selected)
				if m.threadCursor < 0 {
					m.threadCursor = primaryIndex(m.threads)
				}
				return m, tea.Batch(m.restartStreams(), m.loadMissingLedgers(), m.refreshAfter())
			}
			selected := m.currentThreadID()
			m.threads = msg.threads
			m.threadCursor = threadIndex(m.threads, selected)
		}
		return m, m.refreshAfter()
	case streamMessage:
		if msg.stream != m.stream {
			return m, nil
		}
		if !msg.open {
			m.stream = nil
			m.status = "disconnected · retrying"
			return m, tea.Tick(2*time.Second, func(now time.Time) tea.Msg { return reconnectStreams(now) })
		}
		if msg.update.Err != nil {
			m.err = fmt.Errorf("Thread %s stream: %w", shortID(msg.update.ThreadID), msg.update.Err)
			return m, m.waitStream(msg.stream)
		}
		m.applyStream(msg.update)
		return m, m.waitStream(msg.stream)
	case actionFinished:
		m.loading, m.err = false, msg.err
		if msg.err == nil {
			m.status = msg.label
			if m.view == viewAttached && m.session != nil {
				return m, m.loadAttached(m.session.ID)
			}
		}
		return m, nil
	case refreshTick:
		if m.view == viewAttached && m.session != nil {
			return m, m.loadSummary()
		}
		return m, nil
	case reconnectStreams:
		if m.view == viewAttached && m.session != nil && m.stream == nil {
			m.status = "reconnecting"
			return m, m.restartStreams()
		}
		return m, nil
	}

	if m.view == viewInbox {
		return m.updateInbox(message)
	}
	return m.updateAttached(message)
}

func (m Model) updateInbox(message tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := message.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if m.filtering {
		switch key.String() {
		case "esc":
			m.filtering = false
		case "enter":
			m.filtering = false
		case "backspace":
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
			}
		default:
			if key.Text != "" && !key.Mod.Contains(tea.ModCtrl) && !key.Mod.Contains(tea.ModAlt) {
				m.filter += key.Text
			}
		}
		m.inboxCursor = clamp(m.inboxCursor, 0, len(m.filteredSessions())-1)
		return m, nil
	}
	switch key.String() {
	case "up", "k":
		m.inboxCursor = clamp(m.inboxCursor-1, 0, len(m.filteredSessions())-1)
	case "down", "j":
		m.inboxCursor = clamp(m.inboxCursor+1, 0, len(m.filteredSessions())-1)
	case "/":
		m.filtering = true
	case "r":
		m.loading, m.err = true, nil
		return m, m.loadInbox()
	case "enter":
		sessions := m.filteredSessions()
		if m.inboxCursor >= 0 && m.inboxCursor < len(sessions) {
			m.loading, m.err = true, nil
			return m, m.loadAttached(sessions[m.inboxCursor].ID)
		}
	}
	return m, nil
}

func (m Model) updateAttached(message tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := message.(tea.KeyPressMsg)
	if ok {
		if m.overlay != overlayNone {
			return m.updateOverlay(key)
		}
		switch key.String() {
		case "esc":
			if m.focus == focusComposer {
				m.composer.Blur()
				m.focus = focusTimeline
				m.timeline.active = true
				m.renderTimeline()
			} else {
				m.focus = focusComposer
				m.timeline.active = false
				return m, m.composer.Focus()
			}
			return m, nil
		case "tab":
			m.selectThread(1)
			return m, nil
		case "shift+tab":
			m.selectThread(-1)
			return m, nil
		case "pgup":
			m.viewport.HalfPageUp()
			return m, nil
		case "pgdown":
			m.viewport.HalfPageDown()
			return m, nil
		case "ctrl+r":
			m.loading = true
			return m, m.loadAttached(m.session.ID)
		case "ctrl+x":
			m.loading = true
			return m, m.interrupt(m.currentThreadID())
		case "ctrl+k":
			m.overlay, m.overlayCursor = overlayCommands, 0
			m.composer.Blur()
			return m, nil
		case "ctrl+p":
			if len(m.pending) > 0 {
				m.overlay, m.overlayCursor = overlayPermissions, 0
				m.composer.Blur()
			}
			return m, nil
		case "ctrl+l":
			m.focus = focusThreads
			m.timeline.active = false
			m.composer.Blur()
			return m, nil
		case "ctrl+t":
			m.focus = focusTimeline
			m.timeline.active = true
			m.composer.Blur()
			m.renderTimeline()
			return m, nil
		case "i":
			if m.focus != focusComposer {
				m.focus = focusComposer
				m.timeline.active = false
				m.renderTimeline()
				return m, m.composer.Focus()
			}
		case "enter":
			if m.focus == focusComposer {
				content := strings.TrimSpace(m.composer.Value())
				if content != "" {
					m.composer.Reset()
					m.loading = true
					return m, m.sendMessage(content)
				}
			} else if m.focus == focusTimeline {
				m.timeline.toggle()
				m.renderTimeline()
				return m, nil
			}
		case " ":
			if m.focus == focusTimeline {
				m.timeline.toggle()
				m.renderTimeline()
				return m, nil
			}
		case "up", "k":
			if m.focus == focusTimeline {
				m.timeline.moveSelection(-1)
				m.viewport.ScrollUp(3)
				m.renderTimeline()
				return m, nil
			}
			if m.focus == focusThreads {
				m.selectThread(-1)
				return m, nil
			}
		case "down", "j":
			if m.focus == focusTimeline {
				m.timeline.moveSelection(1)
				m.viewport.ScrollDown(3)
				m.renderTimeline()
				return m, nil
			}
			if m.focus == focusThreads {
				m.selectThread(1)
				return m, nil
			}
		}
	}
	var composerCommand, viewportCommand tea.Cmd
	if m.focus == focusComposer {
		previousHeight := m.composer.Height()
		m.composer, composerCommand = m.composer.Update(message)
		if m.composer.Height() != previousHeight {
			m.resize()
			m.renderTimeline()
		}
	}
	m.viewport, viewportCommand = m.viewport.Update(message)
	return m, tea.Batch(composerCommand, viewportCommand)
}

func (m *Model) selectThread(delta int) {
	if len(m.threads) == 0 {
		return
	}
	m.threadCursor = (m.threadCursor + delta + len(m.threads)) % len(m.threads)
	delete(m.unread, m.currentThreadID())
	m.renderTimeline()
}

func (m *Model) applyStream(update api.StreamUpdate) {
	typ := stringValue(update.Frame["type"])
	switch typ {
	case "event_start":
		event, _ := update.Frame["event"].(map[string]any)
		m.previews[update.ThreadID] = &preview{
			id: stringValue(event["id"]), typ: stringValue(event["type"]),
		}
	case "event_delta":
		current := m.previews[update.ThreadID]
		if current != nil && current.id == stringValue(update.Frame["event_id"]) {
			delta, _ := update.Frame["delta"].(map[string]any)
			content, _ := delta["content"].(map[string]any)
			current.content += stringValue(content["text"])
		}
	default:
		id := stringValue(update.Frame["id"])
		if id != "" && !hasEvent(m.events[update.ThreadID], id) {
			m.events[update.ThreadID] = append(m.events[update.ThreadID], update.Frame)
			if update.ThreadID != m.currentThreadID() {
				m.unread[update.ThreadID]++
			}
		}
		if current := m.previews[update.ThreadID]; current != nil && current.id == id {
			delete(m.previews, update.ThreadID)
		}
	}
	m.rebuildPending()
	if update.ThreadID == m.currentThreadID() {
		m.renderTimeline()
	}
	m.status = "live"
}

func (m Model) filteredSessions() []api.Session {
	needle := strings.ToLower(strings.TrimSpace(m.filter))
	if needle == "" {
		return m.sessions
	}
	return slices.DeleteFunc(slices.Clone(m.sessions), func(session api.Session) bool {
		haystack := strings.ToLower(strings.Join([]string{
			session.ID, session.Title, session.Status, session.Agent.Name,
		}, " "))
		return !strings.Contains(haystack, needle)
	})
}

func (m Model) currentThreadID() string {
	if m.threadCursor < 0 || m.threadCursor >= len(m.threads) {
		return ""
	}
	return m.threads[m.threadCursor].ID
}

func (m *Model) resize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	_, mainWidth := paneWidths(m.width)
	m.composer.SetWidth(max(20, mainWidth-2))
	m.viewport.SetWidth(max(20, mainWidth-2))
	m.viewport.SetHeight(max(5, m.height-m.composer.Height()-9))
	m.timeline.width = max(20, mainWidth-4)
}

func (m *Model) rebuildPending() {
	var primaryEvents []api.Event
	for _, thread := range m.threads {
		if thread.Primary() {
			primaryEvents = m.events[thread.ID]
			break
		}
	}
	byID := make(map[string]api.Event, len(primaryEvents))
	var required []string
	for _, event := range primaryEvents {
		byID[stringValue(event["id"])] = event
		if stringValue(event["type"]) != "session.status_idle" {
			continue
		}
		stopReason, _ := event["stop_reason"].(map[string]any)
		if stringValue(stopReason["type"]) != "requires_action" {
			required = nil
			continue
		}
		values, _ := stopReason["event_ids"].([]any)
		required = required[:0]
		for _, value := range values {
			if id := stringValue(value); id != "" {
				required = append(required, id)
			}
		}
	}
	m.pending = m.pending[:0]
	for _, id := range required {
		event, ok := byID[id]
		if !ok {
			continue
		}
		kind := "tool_result"
		switch stringValue(event["type"]) {
		case "agent.custom_tool_use":
			kind = "custom_tool_result"
		case "agent.tool_use", "agent.mcp_tool_use":
			if stringValue(event["evaluated_permission"]) == "ask" {
				kind = "tool_confirmation"
			}
		}
		m.pending = append(m.pending, pendingAction{
			event: event, kind: kind, threadID: stringValue(event["session_thread_id"]),
		})
	}
	m.overlayCursor = clamp(m.overlayCursor, 0, len(m.pending)-1)
}

func primaryIndex(threads []api.Thread) int {
	for index, thread := range threads {
		if thread.Primary() {
			return index
		}
	}
	if len(threads) > 0 {
		return 0
	}
	return -1
}

func threadIndex(threads []api.Thread, id string) int {
	for index, thread := range threads {
		if thread.ID == id {
			return index
		}
	}
	return -1
}

func sameThreadRoster(left, right []api.Thread) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].ID != right[index].ID {
			return false
		}
	}
	return true
}

func hasEvent(events []api.Event, id string) bool {
	return slices.ContainsFunc(events, func(event api.Event) bool {
		return stringValue(event["id"]) == id
	})
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12] + "…"
}

func clamp(value, low, high int) int {
	if high < low {
		return low
	}
	return min(max(value, low), high)
}
