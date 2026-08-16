package ui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/yanpgwang/mango-terminal/internal/endpoint"
)

type connectFocus int

const (
	connectFocusEndpoint connectFocus = iota
	connectFocusButton
)

type endpointAvailability int

const (
	endpointUnknown endpointAvailability = iota
	endpointChecking
	endpointReachable
	endpointUnreachable
)

type endpointChoice struct {
	url          string
	label        string
	source       string
	availability endpointAvailability
	skipProbe    bool
}

type connectState struct {
	focus           connectFocus
	selected        int
	pickerCursor    int
	pickerOpen      bool
	editing         bool
	endpoints       []endpointChoice
	endpointInput   textinput.Model
	validationError string
}

type endpointProbeDone struct {
	url       string
	reachable bool
}

type endpointSaved struct{ err error }

func newConnectState(options Options, t theme) connectState {
	input := textinput.New()
	input.Prompt = "> "
	input.Placeholder = "https://mango.example.com"
	input.CharLimit = 2048
	input.SetVirtualCursor(false)
	styles := textinput.DefaultDarkStyles()
	styles.Focused.Prompt = t.active
	styles.Focused.Text = lipgloss.NewStyle().Foreground(t.text)
	styles.Focused.Placeholder = t.dim
	styles.Blurred = styles.Focused
	input.SetStyles(styles)
	input.Blur()

	state := connectState{focus: connectFocusEndpoint, endpointInput: input}
	for _, option := range options.Endpoints {
		state.addEndpoint(option)
	}
	initial := strings.TrimSpace(options.Endpoint)
	if initial != "" {
		index := state.indexOf(initial)
		if index < 0 {
			state.endpoints = append([]endpointChoice{{url: initial, label: initial, source: "configured"}}, state.endpoints...)
			index = 0
		}
		state.selected, state.pickerCursor = index, index
	}
	if len(state.endpoints) == 0 {
		state.endpoints = append(state.endpoints, endpointChoice{url: "Mango Cloud", label: "Mango Cloud", source: "configured", skipProbe: true})
	}
	state.selected = clamp(state.selected, 0, len(state.endpoints)-1)
	state.pickerCursor = state.selected
	return state
}

func (c *connectState) addEndpoint(option EndpointOption) int {
	target := strings.TrimSpace(option.URL)
	if target == "" {
		target = strings.TrimSpace(option.Label)
	}
	if target == "" {
		return -1
	}
	if index := c.indexOf(target); index >= 0 {
		if c.endpoints[index].source == "" {
			c.endpoints[index].source = strings.TrimSpace(option.Source)
		}
		if option.Available {
			c.endpoints[index].availability = endpointReachable
		}
		return index
	}
	availability := endpointUnknown
	if option.Available {
		availability = endpointReachable
	} else if !option.SkipProbe {
		availability = endpointChecking
	}
	label := strings.TrimSpace(option.Label)
	if label == "" {
		label = target
	}
	c.endpoints = append(c.endpoints, endpointChoice{
		url: target, label: label, source: strings.TrimSpace(option.Source),
		availability: availability, skipProbe: option.SkipProbe,
	})
	return len(c.endpoints) - 1
}

func (c connectState) indexOf(target string) int {
	for index, option := range c.endpoints {
		if option.url == target {
			return index
		}
	}
	return -1
}

func (c connectState) current() endpointChoice {
	if len(c.endpoints) == 0 {
		return endpointChoice{}
	}
	return c.endpoints[clamp(c.selected, 0, len(c.endpoints)-1)]
}

func (m Model) endpointProbeCommands() []tea.Cmd {
	if m.options.ProbeEndpoint == nil {
		return nil
	}
	commands := make([]tea.Cmd, 0, len(m.connect.endpoints))
	for _, option := range m.connect.endpoints {
		if option.skipProbe || option.availability == endpointReachable || option.url == "" {
			continue
		}
		target := option.url
		probe := m.options.ProbeEndpoint
		ctx := m.ctx
		commands = append(commands, func() tea.Msg {
			return endpointProbeDone{url: target, reachable: probe(ctx, target)}
		})
	}
	return commands
}

func (m Model) probeEndpoint(target string) tea.Cmd {
	if m.options.ProbeEndpoint == nil || target == "" {
		return nil
	}
	probe := m.options.ProbeEndpoint
	ctx := m.ctx
	return func() tea.Msg {
		return endpointProbeDone{url: target, reachable: probe(ctx, target)}
	}
}

func (m Model) applyEndpointProbe(message endpointProbeDone) Model {
	for index := range m.connect.endpoints {
		if m.connect.endpoints[index].url != message.url {
			continue
		}
		if message.reachable {
			m.connect.endpoints[index].availability = endpointReachable
		} else {
			m.connect.endpoints[index].availability = endpointUnreachable
		}
	}
	return m
}

func (m Model) updateConnectKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	if m.connect.editing {
		switch key.String() {
		case "esc":
			m.connect.editing = false
			m.connect.validationError = ""
			m.connect.endpointInput.Blur()
			return m, nil
		case "enter":
			target, err := normalizeEndpoint(m.connect.endpointInput.Value())
			if err != nil {
				m.connect.validationError = err.Error()
				return m, nil
			}
			index := m.connect.indexOf(target)
			if index < 0 {
				index = m.connect.addEndpoint(EndpointOption{URL: target, Source: "manual"})
			}
			m.connect.selected, m.connect.pickerCursor = index, index
			m.connect.editing, m.connect.pickerOpen = false, false
			m.connect.focus = connectFocusButton
			m.connect.validationError = ""
			m.connect.endpointInput.Blur()
			return m, m.probeEndpoint(target)
		}
		var command tea.Cmd
		m.connect.endpointInput, command = m.connect.endpointInput.Update(key)
		m.connect.validationError = ""
		return m, command
	}

	if m.connect.pickerOpen {
		count := len(m.connect.endpoints) + 1 // final row is manual entry
		switch key.String() {
		case "esc":
			m.connect.pickerOpen = false
			m.connect.pickerCursor = m.connect.selected
			return m, nil
		case "up", "k":
			m.connect.pickerCursor = wrap(m.connect.pickerCursor-1, count)
			return m, nil
		case "down", "j":
			m.connect.pickerCursor = wrap(m.connect.pickerCursor+1, count)
			return m, nil
		case "e":
			return m.beginEndpointEdit()
		case "enter":
			if m.connect.pickerCursor == len(m.connect.endpoints) {
				return m.beginEndpointEdit()
			}
			m.connect.selected = m.connect.pickerCursor
			m.connect.pickerOpen = false
			m.connect.focus = connectFocusButton
			return m, nil
		}
		return m, nil
	}

	switch key.String() {
	case "up", "down", "tab", "shift+tab":
		if m.connect.focus == connectFocusEndpoint {
			m.connect.focus = connectFocusButton
		} else {
			m.connect.focus = connectFocusEndpoint
		}
		return m, nil
	case "left", "h":
		if m.connect.focus == connectFocusEndpoint && len(m.connect.endpoints) > 1 {
			m.connect.selected = wrap(m.connect.selected-1, len(m.connect.endpoints))
			m.connect.pickerCursor = m.connect.selected
		}
		return m, nil
	case "right", "l":
		if m.connect.focus == connectFocusEndpoint && len(m.connect.endpoints) > 1 {
			m.connect.selected = wrap(m.connect.selected+1, len(m.connect.endpoints))
			m.connect.pickerCursor = m.connect.selected
		}
		return m, nil
	case "e":
		return m.beginEndpointEdit()
	case "enter":
		if m.connect.focus == connectFocusEndpoint {
			m.connect.pickerOpen = true
			m.connect.pickerCursor = m.connect.selected
			return m, nil
		}
		return m.connectSelectedEndpoint()
	case "ctrl+p":
		m.openCommands()
		return m, nil
	case "?":
		m.dialog = dialogHelp
		return m, nil
	}
	return m, nil
}

func (m Model) updateConnectPaste(message tea.PasteMsg) (tea.Model, tea.Cmd) {
	if !m.connect.editing || m.loading {
		return m, nil
	}
	var command tea.Cmd
	m.connect.endpointInput, command = m.connect.endpointInput.Update(message)
	m.connect.validationError = ""
	return m, command
}

func (m Model) beginEndpointEdit() (tea.Model, tea.Cmd) {
	m.connect.editing = true
	m.connect.validationError = ""
	current := m.connect.current().url
	if strings.HasPrefix(current, "http://") || strings.HasPrefix(current, "https://") {
		m.connect.endpointInput.SetValue(current)
	} else {
		m.connect.endpointInput.Reset()
	}
	m.connect.endpointInput.CursorEnd()
	return m, m.connect.endpointInput.Focus()
}

func (m Model) connectSelectedEndpoint() (tea.Model, tea.Cmd) {
	selected := m.connect.current()
	if m.options.BackendForEndpoint != nil {
		backend, err := m.options.BackendForEndpoint(selected.url)
		if err != nil {
			m.err = err
			m.status = "invalid endpoint"
			return m, nil
		}
		m.backend = backend
	}
	m.options.Endpoint = selected.url
	m.loading, m.loadingLabel, m.err = true, "Connecting to "+first(selected.label, "Mango Cloud"), nil
	return m, m.loadInbox()
}

func (m Model) saveSelectedEndpoint() tea.Cmd {
	if m.options.SaveEndpoint == nil {
		return nil
	}
	target := m.connect.current().url
	if target == "" || strings.HasPrefix(target, "demo://") {
		return nil
	}
	save := m.options.SaveEndpoint
	return func() tea.Msg { return endpointSaved{err: save(target)} }
}

func normalizeEndpoint(value string) (string, error) {
	return endpoint.Normalize(value)
}
