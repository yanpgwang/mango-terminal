package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/yanpgwang/mango-terminal/internal/demo"
	"github.com/yanpgwang/mango-terminal/internal/feed"
	"github.com/yanpgwang/mango-terminal/internal/mango"
)

type failingSendBackend struct{ mango.Backend }

func (f failingSendBackend) SendMessage(context.Context, string, string) ([]mango.Event, error) {
	return nil, errors.New("control plane unavailable")
}

func TestThreadSelectionIsExplicit(t *testing.T) {
	model := New(demo.New(), "")
	model.loading = false
	parent := "sthr_primary"
	model.threads = []mango.Thread{{ID: parent}, {ID: "sthr_child", ParentThreadID: &parent}}
	model.threadCursor = 0
	model.selectThread(1)
	if model.currentThreadID() != "sthr_child" {
		t.Fatalf("thread = %q", model.currentThreadID())
	}
	model.setFocus(focusChat)
	if model.focus != focusChat || model.editor.Focused() {
		t.Fatal("chat focus did not blur editor")
	}
}

func TestTabNeverChangesSelectedAgent(t *testing.T) {
	model := New(demo.New(), "")
	parent := "sthr_primary"
	model.threads = []mango.Thread{{ID: parent}, {ID: "sthr_child", ParentThreadID: &parent}}
	model.threadCursor, model.screen, model.width, model.height = 0, screenChat, 140, 42
	model.resize()

	updated, _ := model.updateChat(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	model = updated.(Model)
	if model.focus != focusChat || model.currentThreadID() != parent {
		t.Fatalf("after tab focus=%v thread=%q", model.focus, model.currentThreadID())
	}
	updated, _ = model.updateChat(tea.KeyPressMsg(tea.Key{Code: 'l', Text: "l"}))
	model = updated.(Model)
	if model.focus != focusChat || model.currentThreadID() != parent {
		t.Fatalf("after l focus=%v thread=%q", model.focus, model.currentThreadID())
	}
	updated, _ = model.updateChat(tea.KeyPressMsg(tea.Key{Code: 'g', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if model.dialog != dialogAgents {
		t.Fatalf("ctrl+g dialog=%v", model.dialog)
	}
	updated, _ = model.updateDialog(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	model = updated.(Model)
	updated, _ = model.updateDialog(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.currentThreadID() != "sthr_child" {
		t.Fatalf("agent picker thread=%q", model.currentThreadID())
	}
}

func TestAttachedSessionShowsBoardWithAgents(t *testing.T) {
	model := New(demo.New(), "")
	model.loading = false
	model.width, model.height, model.screen = 140, 42, screenBoard
	model.session = &mango.Session{ID: "sesn_1", Title: "Launch"}
	parent := "sthr_primary"
	model.threads = []mango.Thread{
		{ID: parent, Status: "idle", Agent: mango.Agent{Name: "coordinator"}},
		{ID: "sthr_child", ParentThreadID: &parent, Status: "idle", Agent: mango.Agent{Name: "researcher"}},
	}
	model.threadCursor = 0
	model.resize()
	view := ansi.Strip(model.renderBoard())
	for _, want := range []string{"Launch", "Coordinator", "Sub-agents", "researcher"} {
		if !strings.Contains(view, want) {
			t.Fatalf("board missing %q: %q", want, view)
		}
	}
}

func TestConnectionLogoUsesGradientWordmarkOnly(t *testing.T) {
	model := New(demo.New(), "")
	logo := model.brandLogo(false)
	plain := ansi.Strip(logo)
	if logo == plain {
		t.Fatal("connection logo has no ANSI color treatment")
	}
	if !strings.Contains(plain, "███  ███") || !strings.Contains(plain, " ▀████▀") ||
		!strings.Contains(plain, "powered by Bubble Tea") {
		t.Fatalf("connection logo lost the Mango wordmark: %q", plain)
	}
	if strings.ContainsAny(plain, "◆●╱╲") {
		t.Fatalf("connection logo still contains a graphical mark: %q", plain)
	}
	if width := lipgloss.Width(logo); width > 52 {
		t.Fatalf("connection logo width=%d, want <=52", width)
	}
	if height := lipgloss.Height(logo); height != 8 {
		t.Fatalf("connection logo height=%d, want 8", height)
	}
}

func TestConnectionLogoFitsMinimumSupportedTerminal(t *testing.T) {
	model := New(demo.New(), "")
	model.width, model.height, model.screen = 60, 20, screenConnect
	view := model.renderConnect()
	if width := lipgloss.Width(view); width > model.width {
		t.Fatalf("connection view width=%d terminal=%d", width, model.width)
	}
	if height := lipgloss.Height(view); height > model.height {
		t.Fatalf("connection view height=%d terminal=%d", height, model.height)
	}
	if plain := ansi.Strip(view); !strings.Contains(plain, "Connect") || !strings.Contains(plain, "powered by Bubble Tea") {
		t.Fatalf("connection view lost controls or wordmark: %q", plain)
	}
}

func TestTextareaUsesRealCursorForIME(t *testing.T) {
	model := New(demo.New(), "")
	if model.editor.VirtualCursor() {
		t.Fatal("textarea virtual cursor leaves terminal IME without an anchor")
	}
	model.loading = false
	model.width, model.height, model.screen = 100, 32, screenChat
	model.session = &mango.Session{ID: "sesn_ime"}
	model.threads = []mango.Thread{{ID: "sthr_primary", Status: "idle"}}
	model.threadCursor = 0
	model.editor.SetValue("凡")
	model.resize()
	model.renderChat()

	view := model.View()
	if view.Cursor == nil {
		t.Fatal("focused chat textarea did not expose a terminal cursor")
	}
	if view.Cursor.X >= model.width/2 || view.Cursor.Y < model.height/2 || view.Cursor.Y >= model.height {
		t.Fatalf("chat cursor = (%d,%d), want composer near lower-left", view.Cursor.X, view.Cursor.Y)
	}
	model.screen = screenConnect
	if cursor := model.View().Cursor; cursor != nil {
		t.Fatalf("connection screen exposed editor cursor at (%d,%d)", cursor.X, cursor.Y)
	}
}

func TestChatAndDialogCursorOffsetsStayInsideTheirEditors(t *testing.T) {
	model := New(demo.New(), "")
	model.loading = false
	model.width, model.height, model.screen = 140, 42, screenChat
	model.session = &mango.Session{ID: "sesn_1", Title: "Cursor test"}
	model.threads = []mango.Thread{{ID: "sthr_primary", Status: "idle"}}
	model.threadCursor = 0
	model.editor.SetValue("中文")
	model.resize()
	model.renderChat()

	chat := model.View()
	if chat.Cursor == nil || chat.Cursor.X >= 24 || chat.Cursor.Y < model.height/2 || chat.Cursor.Y >= model.height {
		if chat.Cursor == nil {
			t.Fatal("chat textarea did not expose a terminal cursor")
		}
		t.Fatalf("chat cursor = (%d,%d), want lower-left composer", chat.Cursor.X, chat.Cursor.Y)
	}

	model.dialog = dialogResult
	model.editor.Focus()
	model.resize()
	dialog := model.View()
	if dialog.Cursor == nil {
		t.Fatal("dialog textarea did not expose a terminal cursor")
	}
	modal := model.renderDialog()
	left := max(0, (model.width-lipgloss.Width(modal))/2)
	top := max(0, (model.height-lipgloss.Height(modal))/2)
	if dialog.Cursor.X <= left || dialog.Cursor.X >= left+lipgloss.Width(modal) ||
		dialog.Cursor.Y <= top || dialog.Cursor.Y >= top+lipgloss.Height(modal) {
		t.Fatalf("dialog cursor = (%d,%d), modal=(%d,%d)-(%d,%d)",
			dialog.Cursor.X, dialog.Cursor.Y, left, top, left+lipgloss.Width(modal), top+lipgloss.Height(modal))
	}
}

func TestSelectionDialogsUseRealTextInputCursorForIME(t *testing.T) {
	model := New(demo.New(), "")
	model.loading = false
	model.width, model.height, model.screen = 120, 36, screenInbox
	model.openSessions()
	model.filter.SetValue("中文")
	model.inboxFilter = model.filter.Value()
	model.resize()

	view := model.View()
	if view.Cursor == nil {
		t.Fatal("Session picker did not expose a real textinput cursor")
	}
	modal := model.renderDialog()
	left := max(0, (model.width-lipgloss.Width(modal))/2)
	top := max(0, (model.height-lipgloss.Height(modal))/2)
	if view.Cursor.X <= left || view.Cursor.X >= left+lipgloss.Width(modal) ||
		view.Cursor.Y <= top || view.Cursor.Y >= top+lipgloss.Height(modal) {
		t.Fatalf("filter cursor = (%d,%d), modal=(%d,%d)-(%d,%d)",
			view.Cursor.X, view.Cursor.Y, left, top, left+lipgloss.Width(modal), top+lipgloss.Height(modal))
	}
}

func TestPendingChildActionKeepsVisibleCrosspostID(t *testing.T) {
	model := New(demo.New(), "")
	model.threads = []mango.Thread{{ID: "sthr_primary"}}
	model.events["sthr_primary"] = []mango.Event{
		{"id": "sevt_crosspost", "type": "agent.tool_use", "name": "bash", "evaluated_permission": "ask", "session_thread_id": "sthr_child"},
		{"id": "sevt_idle", "type": "session.status_idle", "stop_reason": map[string]any{"type": "requires_action", "event_ids": []any{"sevt_crosspost"}}},
	}
	model.pending = feed.PendingActions(model.primaryThreadID(), model.events)
	if len(model.pending) != 1 || model.pending[0].ID != "sevt_crosspost" || model.pending[0].ThreadID != "sthr_child" {
		t.Fatalf("pending = %#v", model.pending)
	}
}

func TestPendingActionOpensDecisionDialog(t *testing.T) {
	model := New(demo.New(), "")
	model.width, model.height, model.screen = 120, 36, screenChat
	model.threads = []mango.Thread{{ID: "sthr_primary"}}
	model.threadCursor = 0
	model.resize()
	model.applyStream(mango.StreamUpdate{ThreadID: "sthr_primary", Frame: mango.Event{
		"id": "sevt_tool", "type": "agent.tool_use", "name": "bash",
		"input": map[string]any{"command": "go test ./..."}, "evaluated_permission": "ask",
	}})
	model.applyStream(mango.StreamUpdate{ThreadID: "sthr_primary", Frame: mango.Event{
		"id": "sevt_idle", "type": "session.status_idle",
		"stop_reason": map[string]any{"type": "requires_action", "event_ids": []any{"sevt_tool"}},
	}})
	if model.dialog != dialogAction || len(model.pending) != 1 {
		t.Fatalf("dialog=%v pending=%#v", model.dialog, model.pending)
	}
	if !model.dialogIsOverlay() {
		t.Fatal("permission decision should be an overlay")
	}
	dialog := model.renderDialog()
	if !strings.Contains(dialog, "bash") || !strings.Contains(dialog, "Allow once") {
		t.Fatalf("dialog = %q", dialog)
	}
	model.applyStream(mango.StreamUpdate{ThreadID: "sthr_primary", Frame: mango.Event{
		"id": "sevt_done", "type": "session.status_idle",
		"stop_reason": map[string]any{"type": "end_turn"},
	}})
	if model.dialog != dialogNone || len(model.pending) != 0 || !model.editor.Focused() {
		t.Fatalf("resolved dialog=%v pending=%#v focused=%v", model.dialog, model.pending, model.editor.Focused())
	}
}

func TestConversationUsesRailsWithoutRoleHeaders(t *testing.T) {
	model := New(demo.New(), "")
	user := model.renderFeedItem(feed.Item{Kind: feed.User, Title: "You", Body: "hello"}, false, 80)
	agent := model.renderFeedItem(feed.Item{Kind: feed.Agent, Title: "coordinator", Body: "hi"}, false, 80)
	if strings.Contains(user, "You") || strings.Contains(agent, "coordinator") || !strings.Contains(user, "▌") {
		t.Fatalf("user=%q agent=%q", user, agent)
	}
}

func TestMarkdownCacheEvictsIncrementallyAtCapacity(t *testing.T) {
	model := New(demo.New(), "")
	for index := range markdownCacheLimit + 2 {
		model.renderMarkdown(fmt.Sprintf("message %d", index), 80, true)
	}
	if len(model.markdown.entries) != markdownCacheLimit || len(model.markdown.order) != markdownCacheLimit {
		t.Fatalf("entries=%d order=%d", len(model.markdown.entries), len(model.markdown.order))
	}
	if _, ok := model.markdown.entries[markdownCacheKey{width: 80, content: "message 0"}]; ok {
		t.Fatal("oldest cache entry was not evicted")
	}
	if _, ok := model.markdown.entries[markdownCacheKey{width: 80, content: fmt.Sprintf("message %d", markdownCacheLimit+1)}]; !ok {
		t.Fatal("newest cache entry is missing")
	}
}

func TestStreamingPreviewGrowsInPlaceThenBecomesDurable(t *testing.T) {
	model := New(demo.New(), "")
	model.width, model.height, model.screen = 100, 30, screenChat
	model.threads = []mango.Thread{{ID: "sthr_primary"}}
	model.threadCursor = 0
	model.resize()
	model.applyStream(mango.StreamUpdate{ThreadID: "sthr_primary", Frame: mango.Event{
		"type": "event_start", "event": map[string]any{"id": "sevt_reply", "type": "agent.message"},
	}})
	for _, text := range []string{"hello ", "world"} {
		model.applyStream(mango.StreamUpdate{ThreadID: "sthr_primary", Frame: mango.Event{
			"type": "event_delta", "event_id": "sevt_reply",
			"delta": map[string]any{"content": map[string]any{"text": text}},
		}})
	}
	if model.previews["sthr_primary"] == nil || model.previews["sthr_primary"].content != "hello world" {
		t.Fatalf("preview = %#v", model.previews["sthr_primary"])
	}
	model.applyStream(mango.StreamUpdate{ThreadID: "sthr_primary", Frame: mango.Event{
		"id": "sevt_reply", "type": "agent.message",
		"content": []any{map[string]any{"type": "text", "text": "hello world"}},
	}})
	if model.previews["sthr_primary"] != nil || len(model.events["sthr_primary"]) != 1 {
		t.Fatalf("preview=%#v events=%#v", model.previews["sthr_primary"], model.events["sthr_primary"])
	}
}

func TestThinkingFruitTransitionsIntoStreamingReply(t *testing.T) {
	model := New(demo.New(), "")
	model.width, model.height, model.screen = 100, 30, screenChat
	model.session = &mango.Session{ID: "sesn_live"}
	model.threads = []mango.Thread{{ID: "sthr_primary"}}
	model.threadCursor = 0
	model.resize()

	model.beginLiveTurn("sthr_primary")
	if plain := ansi.Strip(model.chat.View()); !strings.Contains(plain, "🥭") || !strings.Contains(plain, "Thinking") {
		t.Fatalf("optimistic thinking state = %q", plain)
	}
	model.applyStream(mango.StreamUpdate{ThreadID: "sthr_primary", Frame: mango.Event{
		"type": "event_start", "event": map[string]any{"id": "sevt_think", "type": "agent.thinking"},
	}})
	if model.previews["sthr_primary"].thinkingID != "sevt_think" {
		t.Fatalf("thinking preview = %#v", model.previews["sthr_primary"])
	}
	model.applyStream(mango.StreamUpdate{ThreadID: "sthr_primary", Frame: mango.Event{
		"type": "event_start", "event": map[string]any{"id": "sevt_reply", "type": "agent.message"},
	}})
	model.applyStream(mango.StreamUpdate{ThreadID: "sthr_primary", Frame: mango.Event{
		"type": "event_delta", "event_id": "sevt_reply",
		"delta": map[string]any{"content": map[string]any{"text": "streaming now"}},
	}})
	current := model.previews["sthr_primary"]
	if current == nil || current.thinkingID != "" || current.messageID != "sevt_reply" || current.content != "streaming now" {
		t.Fatalf("reply preview = %#v", current)
	}
	if plain := ansi.Strip(model.chat.View()); !strings.Contains(plain, "streaming now") || !strings.Contains(plain, "▌") {
		t.Fatalf("streaming view = %q", plain)
	}
}

func TestStreamingDeltaRecoversWhenStartWasMissed(t *testing.T) {
	model := New(demo.New(), "")
	model.width, model.height, model.screen = 100, 30, screenChat
	model.threads = []mango.Thread{{ID: "sthr_primary"}}
	model.threadCursor = 0
	model.resize()
	model.applyStream(mango.StreamUpdate{ThreadID: "sthr_primary", Frame: mango.Event{
		"type": "event_delta", "event_id": "sevt_reply",
		"delta": map[string]any{"content": map[string]any{"text": "recovered"}},
	}})
	current := model.previews["sthr_primary"]
	if current == nil || current.messageID != "sevt_reply" || current.content != "recovered" {
		t.Fatalf("recovered preview = %#v", current)
	}
}

func TestConversationFooterOwnsModelAndLiveUsage(t *testing.T) {
	model := New(demo.New(), "")
	model.width, model.height, model.screen = 140, 42, screenChat
	model.session = &mango.Session{ID: "sesn_usage"}
	model.threads = []mango.Thread{{ID: "sthr_primary"}}
	model.threads[0].Agent.Model.ID = "deepseek-v4-flash"
	model.threadCursor = 0
	model.resize()
	model.applyStream(mango.StreamUpdate{ThreadID: "sthr_primary", Frame: mango.Event{
		"id": "sevt_usage", "type": "span.model_request_end",
		"model_usage": map[string]any{"input_tokens": float64(1200), "output_tokens": float64(345), "cache_read_input_tokens": float64(90)},
	}})

	footer := ansi.Strip(model.renderConversationInfo(108))
	if !strings.Contains(footer, "deepseek-v4-flash") || !strings.Contains(footer, "1.2K in") || !strings.Contains(footer, "345 out") {
		t.Fatalf("conversation footer = %q", footer)
	}
	if model.session.Usage.InputTokens != 1200 || model.threads[0].Usage.CacheReadInputTokens != 90 {
		t.Fatalf("usage session=%#v thread=%#v", model.session.Usage, model.threads[0].Usage)
	}
}

func TestNewSessionWizardIsFocusedCenterCard(t *testing.T) {
	model := New(demo.New(), "")
	model.width, model.height, model.screen = 140, 42, screenInbox
	updated, command := model.startNewSession()
	model = updated.(Model)
	if command == nil || !model.dialogIsOverlay() || model.dialog != dialogNewSession {
		t.Fatalf("dialog=%v overlay=%v command=%v", model.dialog, model.dialogIsOverlay(), command)
	}
	if width := model.dialogWidth(); width != 66 {
		t.Fatalf("creation card width=%d, want 66", width)
	}
	modal := model.renderDialog()
	if lipgloss.Width(modal) >= model.width-20 {
		t.Fatalf("creation card width=%d terminal=%d", lipgloss.Width(modal), model.width)
	}

	model.loading = false
	model.creation.step = createSessionTitle
	model.creation.agent.Name = "researcher"
	model.creation.agent.Model.ID = "deepseek-v4-flash"
	model.creation.environment.Name = "Mango cloud"
	model.editor.SetValue("研究历")
	model.editor.Focus()
	model.resize()
	modal = model.renderDialog()
	left := max(0, (model.width-lipgloss.Width(modal))/2)
	top := max(0, (model.height-lipgloss.Height(modal))/2)
	view := model.View()
	if view.Cursor == nil {
		t.Fatal("creation card did not expose its textarea cursor")
	}
	source := model.editor.Cursor()
	wantX, wantY := left+5+source.X, top+model.dialogEditorY()+source.Y
	if view.Cursor.X != wantX || view.Cursor.Y != wantY {
		t.Fatalf("creation cursor=(%d,%d), want text position (%d,%d)", view.Cursor.X, view.Cursor.Y, wantX, wantY)
	}

	plainLines := strings.Split(ansi.Strip(view.Content), "\n")
	titleX := -1
	for _, line := range plainLines {
		if index := strings.Index(line, "New Session"); index >= 0 {
			titleX = ansi.StringWidth(line[:index])
			break
		}
	}
	if titleX != left+3 {
		t.Fatalf("creation card title x=%d, want centered card x=%d", titleX, left+3)
	}
}

func TestNewSessionWizardCreatesAndAttaches(t *testing.T) {
	model := New(demo.New(), "")
	model.loading = false
	model.width, model.height = 100, 32
	model.resize()

	updated, command := model.updateInbox(tea.KeyPressMsg(tea.Key{Code: 'n', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if model.dialog != dialogNewSession || command == nil {
		t.Fatalf("dialog=%v command=%v", model.dialog, command)
	}
	updated, _ = model.Update(command())
	model = updated.(Model)
	if model.creation.step != createChooseAgent || len(model.creation.agents) == 0 {
		t.Fatalf("creation = %#v", model.creation)
	}

	updated, _ = model.updateNewSession(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	updated, _ = model.updateNewSession(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.creation.step != createSessionTitle {
		t.Fatalf("step = %v", model.creation.step)
	}
	model.editor.SetValue("Created in TUI")
	updated, _ = model.updateNewSession(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	model.editor.SetValue("hello from the wizard")
	updated, _ = model.updateNewSession(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.creation.step != createConfirm {
		t.Fatalf("step = %v", model.creation.step)
	}
	updated, command = model.updateNewSession(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	updated, attach := model.Update(command())
	model = updated.(Model)
	if attach == nil {
		t.Fatal("created Session was not attached")
	}
	updated, _ = model.Update(attach())
	model = updated.(Model)
	if model.screen != screenBoard || model.session == nil || model.session.Title != "Created in TUI" {
		t.Fatalf("screen=%v session=%#v err=%v", model.screen, model.session, model.err)
	}
	if got := model.events[model.primaryThreadID()]; len(got) != 1 || got[0].Type() != "user.message" {
		t.Fatalf("initial events = %#v", got)
	}
}

func TestCreationPickersUseFuzzyFiltering(t *testing.T) {
	model := New(demo.New(), "")
	model.dialog = dialogNewSession
	updated, _ := model.resourcesLoaded(creationResourcesLoaded{agents: []mango.Agent{
		{ID: "agent_reviewer", Name: "reviewer"},
		{ID: "agent_researcher", Name: "researcher"},
	}})
	model = updated.(Model)
	for _, char := range "rvr" {
		updated, _ = model.updateNewSession(tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
		model = updated.(Model)
	}
	filtered := model.filteredCreationAgents()
	if len(filtered) != 1 || filtered[0].Name != "reviewer" {
		t.Fatalf("filtered Agents = %#v", filtered)
	}
}

func TestConnectThenHomeStartsWithVisibleCreateChoice(t *testing.T) {
	model := New(demo.New(), "")
	if model.screen != screenConnect || model.loading {
		t.Fatalf("initial screen=%v loading=%v", model.screen, model.loading)
	}
	updated, command := model.updateConnect(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if command == nil || !model.loading {
		t.Fatalf("connect command=%v loading=%v", command, model.loading)
	}
	updated, _ = model.Update(command())
	model = updated.(Model)
	if model.screen != screenInbox || len(model.sessions) == 0 || model.inboxCursor != 0 {
		t.Fatalf("screen=%v sessions=%d cursor=%d err=%v", model.screen, len(model.sessions), model.inboxCursor, model.err)
	}
	updated, command = model.updateInbox(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.dialog != dialogNewSession || command == nil {
		t.Fatalf("create choice dialog=%v command=%v", model.dialog, command)
	}
}

func TestCommandsUseSearchablePicker(t *testing.T) {
	model := New(demo.New(), "")
	model.session = &mango.Session{ID: "sesn_1"}
	model.openCommands()
	for _, char := range "swtchagt" {
		updated, _ := model.updateDialog(tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
		model = updated.(Model)
	}
	commands := model.filteredCommands()
	if len(commands) != 1 || commands[0].id != "agents" {
		t.Fatalf("commands = %#v", commands)
	}
}

func TestBackgroundTurnCompletionUsesTerminalNotification(t *testing.T) {
	model := NewWithOptions(demo.New(), "", Options{Notifications: NotificationOSC777})
	model.session = &mango.Session{Title: "Launch review"}
	model.windowFocused = false
	command := model.notificationFor(mango.StreamUpdate{Frame: mango.Event{
		"id": "sevt_idle", "type": "session.status_idle",
		"stop_reason": map[string]any{"type": "end_turn"},
	}})
	if command == nil {
		t.Fatal("missing notification command")
	}
	raw, ok := command().(tea.RawMsg)
	if !ok || !strings.Contains(raw.Msg.(string), "Mango finished a turn") {
		t.Fatalf("notification = %#v", raw)
	}
	model.windowFocused = true
	if command := model.notificationFor(mango.StreamUpdate{Frame: mango.Event{"id": "sevt_idle", "type": "session.status_idle"}}); command != nil {
		t.Fatal("focused window should not notify")
	}
}

func TestNotificationsAreDisabledUnlessExplicitlyEnabled(t *testing.T) {
	model := New(demo.New(), "")
	model.session = &mango.Session{Title: "Launch review"}
	model.windowFocused = false
	if command := model.notificationFor(mango.StreamUpdate{Frame: mango.Event{
		"id": "sevt_idle", "type": "session.status_idle",
	}}); command != nil {
		t.Fatal("default notification mode must not emit terminal escape sequences")
	}
}

func TestQuitUsesSafeCentralConfirmation(t *testing.T) {
	model := New(demo.New(), "")
	model.width, model.height, model.screen = 100, 30, screenChat
	model.resize()

	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if command != nil || model.dialog != dialogQuit || model.dialogCursor != 1 || !model.dialogIsOverlay() {
		t.Fatalf("quit prompt command=%v dialog=%v cursor=%d", command, model.dialog, model.dialogCursor)
	}
	dialog := ansi.Strip(model.renderDialog())
	if !strings.Contains(dialog, "Quit Mango?") || !strings.Contains(dialog, "keep running in the") ||
		!strings.Contains(dialog, "cloud.") ||
		!strings.Contains(dialog, "Keep working") {
		t.Fatalf("quit dialog = %q", dialog)
	}

	updated, command = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if command != nil || model.dialog != dialogNone {
		t.Fatalf("safe default command=%v dialog=%v", command, model.dialog)
	}
}

func TestQuitConfirmationSupportsChoiceAndDoubleControlC(t *testing.T) {
	model := New(demo.New(), "")
	model.width, model.height, model.screen = 100, 30, screenChat
	model.resize()
	model.openQuitDialog()

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	model = updated.(Model)
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if command == nil {
		t.Fatal("selected Quit did not return a quit command")
	}
	if _, ok := command().(tea.QuitMsg); !ok {
		t.Fatal("selected Quit did not emit tea.QuitMsg")
	}

	model = New(demo.New(), "")
	model.openQuitDialog()
	updated, command = model.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	if command == nil {
		t.Fatal("second ctrl+c did not return a quit command")
	}
	if _, ok := command().(tea.QuitMsg); !ok {
		t.Fatal("second ctrl+c did not emit tea.QuitMsg")
	}
}

func TestQuitCancelRestoresUnderlyingDialog(t *testing.T) {
	model := New(demo.New(), "")
	model.dialog = dialogHelp
	model.openQuitDialog()
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = updated.(Model)
	if command != nil || model.dialog != dialogHelp {
		t.Fatalf("cancel command=%v dialog=%v", command, model.dialog)
	}
}

func TestHomeRefreshIsAVisibleChoice(t *testing.T) {
	model := New(demo.New(), "")
	model.screen, model.loading, model.inboxCursor = screenInbox, false, 2
	updated, command := model.updateInbox(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if command == nil || !model.loading || model.loadingLabel != "Refreshing Sessions" {
		t.Fatalf("command=%v loading=%v label=%q", command, model.loading, model.loadingLabel)
	}
}

func TestTextDialogRendersEditorOnlyOnce(t *testing.T) {
	model := New(demo.New(), "")
	model.width, model.height, model.screen = 100, 30, screenChat
	model.session = &mango.Session{ID: "sesn_1"}
	model.threads = []mango.Thread{{ID: "sthr_primary"}}
	model.pending = []mango.Action{{ID: "sevt_tool", Kind: mango.ActionToolResult}}
	model.dialog = dialogResult
	model.editor.SetValue("unique-modal-draft")
	model.resize()
	if got := strings.Count(model.View().Content, "unique-modal-draft"); got != 1 {
		t.Fatalf("editor rendered %d times", got)
	}
}

func TestFailedSendPreservesDraftAndReadyPlaceholder(t *testing.T) {
	backend := failingSendBackend{Backend: demo.New()}
	model := New(backend, "")
	model.screen, model.loading = screenChat, false
	model.session = &mango.Session{ID: "sesn_1"}
	model.threads = []mango.Thread{{ID: "sthr_primary"}}
	model.editor.SetValue("do not lose this")
	updated, command := model.updateChat(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if command == nil || model.editor.Value() != "" {
		t.Fatalf("command=%v draft=%q", command, model.editor.Value())
	}
	updated, _ = model.Update(command())
	model = updated.(Model)
	if model.editor.Value() != "do not lose this" || model.editor.Placeholder != "Message coordinator…" || model.err == nil {
		t.Fatalf("draft=%q placeholder=%q err=%v", model.editor.Value(), model.editor.Placeholder, model.err)
	}
}

func TestOldOperationCannotMutateNewSession(t *testing.T) {
	model := New(demo.New(), "")
	model.session = &mango.Session{ID: "sesn_new"}
	updated, _ := model.Update(operationDone{
		sessionID: "sesn_old", label: "message sent",
		events: []mango.Event{{"id": "sevt_old", "type": "agent.message"}},
	})
	model = updated.(Model)
	if len(model.events) != 0 || model.status != "" {
		t.Fatalf("stale operation mutated model: events=%#v status=%q", model.events, model.status)
	}
}

func TestWideCharacterTruncationFitsTerminalCells(t *testing.T) {
	got := truncate("研究者🍊协调员", 8)
	if width := ansi.StringWidth(got); width > 8 {
		t.Fatalf("truncate width=%d value=%q", width, got)
	}
}

func TestSmallTerminalUsesBoundedFallback(t *testing.T) {
	model := New(demo.New(), "")
	model.width, model.height = 30, 10
	view := model.View().Content
	for _, line := range strings.Split(view, "\n") {
		if width := ansi.StringWidth(line); width > model.width {
			t.Fatalf("line width=%d exceeds %d: %q", width, model.width, line)
		}
	}
}

func TestLargeActionInputCannotGrowDialogPastTerminal(t *testing.T) {
	model := New(demo.New(), "")
	model.width, model.height, model.screen = 60, 20, screenChat
	model.threads = []mango.Thread{{ID: "sthr_primary"}}
	model.pending = []mango.Action{{
		ID: "sevt_tool", Kind: mango.ActionConfirmation, Name: "write_file",
		Input: strings.Repeat("a very long tool argument that must wrap safely ", 2000),
	}}
	model.dialog = dialogAction
	model.resize()
	dialog := model.renderDialog()
	if height := lipgloss.Height(dialog); height > model.height {
		t.Fatalf("dialog height=%d terminal=%d", height, model.height)
	}
	if width := lipgloss.Width(dialog); width > model.width {
		t.Fatalf("dialog width=%d terminal=%d", width, model.width)
	}
	if !strings.Contains(dialog, "more line") {
		t.Fatalf("large input was not summarized: %q", dialog)
	}
}

func TestTextDialogsStayInsideMinimumTerminal(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Model)
	}{
		{
			name: "tool result",
			setup: func(model *Model) {
				model.pending = []mango.Action{{ID: "sevt_tool", Kind: mango.ActionToolResult}}
				model.dialog = dialogResult
			},
		},
		{
			name: "creation field",
			setup: func(model *Model) {
				model.dialog = dialogNewSession
				model.creation.step = createInitialPrompt
				model.creation.agent = mango.Agent{ID: "agent_1", Name: "coordinator"}
				model.creation.environment = mango.Environment{ID: "env_1", Name: "Mango cloud"}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := New(demo.New(), "")
			model.width, model.height, model.screen = 60, 20, screenChat
			test.setup(&model)
			model.editor.SetValue(strings.Repeat("line of text\n", 50))
			model.resize()
			dialog := model.renderDialog()
			if height := lipgloss.Height(dialog); height > model.height {
				t.Fatalf("dialog height=%d terminal=%d", height, model.height)
			}
			if width := lipgloss.Width(dialog); width > model.width {
				t.Fatalf("dialog width=%d terminal=%d", width, model.width)
			}
		})
	}
}

func TestLongNamesStayInsideMinimumTerminalDialogs(t *testing.T) {
	longName := strings.Repeat("研究型协调 Agent 🍊 ", 30)
	tests := []struct {
		name  string
		setup func(*Model)
	}{
		{
			name: "Agent picker",
			setup: func(model *Model) {
				model.dialog = dialogAgents
				model.threads = []mango.Thread{{ID: "sthr_1", Status: "idle", Agent: mango.Agent{Name: longName}}}
			},
		},
		{
			name: "creation picker",
			setup: func(model *Model) {
				model.dialog = dialogNewSession
				model.creation.step = createChooseAgent
				model.creation.agents = []mango.Agent{{ID: "agent_1", Name: longName}}
			},
		},
		{
			name: "interrupt",
			setup: func(model *Model) {
				model.dialog = dialogInterrupt
				model.threads = []mango.Thread{{ID: "sthr_1", Agent: mango.Agent{Name: longName}}}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := New(demo.New(), "")
			model.width, model.height, model.screen = 60, 20, screenChat
			test.setup(&model)
			model.resize()
			dialog := model.renderDialog()
			if width, height := lipgloss.Width(dialog), lipgloss.Height(dialog); width > model.width || height > model.height {
				t.Fatalf("dialog=%dx%d terminal=%dx%d", width, height, model.width, model.height)
			}
		})
	}
}

func TestThreadCreatedRequestsAnEventDrivenRosterRefresh(t *testing.T) {
	model := New(demo.New(), "")
	model.threads = []mango.Thread{{ID: "sthr_primary"}}
	changed := model.applyStream(mango.StreamUpdate{ThreadID: "sthr_primary", Frame: mango.Event{
		"id": "sevt_created", "type": "session.thread_created",
		"session_thread_id": "sthr_child", "agent_name": "reviewer",
	}})
	if !changed {
		t.Fatal("thread creation did not request a roster refresh")
	}
}

func TestAsyncActionDialogHasBoundedGraceAndSkipsImmediateReopen(t *testing.T) {
	model := New(demo.New(), "")
	model.pending = []mango.Action{{ID: "sevt_tool", Kind: mango.ActionConfirmation}}
	model.openActionDialog(true)
	updated, command := model.updateDialog(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if command != nil || model.dialog != dialogAction {
		t.Fatal("in-flight enter should be absorbed")
	}
	model.dialogGraceOpenedAt = time.Now().Add(-actionGraceMaxDelay - time.Millisecond)
	model.dialogGraceLastInputAt = time.Now()
	if model.actionDialogInGrace() {
		t.Fatal("absolute grace ceiling did not arm dialog")
	}
	model.closeActionDialog()
	model.dialog = dialogNone
	model.openActionDialog(true)
	if model.actionDialogInGrace() {
		t.Fatal("immediate same-dialog reopen should skip grace")
	}
}

func TestFreshAgentEventGetsOneShotSpark(t *testing.T) {
	model := New(demo.New(), "")
	model.threads = []mango.Thread{{ID: "sthr_primary"}}
	model.applyStream(mango.StreamUpdate{ThreadID: "sthr_primary", Frame: mango.Event{
		"id": "sevt_reply", "type": "agent.message",
		"content": []any{map[string]any{"type": "text", "text": "done"}},
	}})
	if model.fresh["sevt_reply"] == 0 || model.spark("sevt_reply") == "" {
		t.Fatalf("fresh = %#v", model.fresh)
	}
}

func TestNewSessionWizardCanCreateMissingResources(t *testing.T) {
	model := New(demo.New(), "")
	model.dialog = dialogNewSession
	updated, _ := model.resourcesLoaded(creationResourcesLoaded{})
	model = updated.(Model)
	if model.creation.step != createChooseAgent || model.creation.agentCursor != 0 {
		t.Fatalf("step = %v", model.creation.step)
	}
	updated, _ = model.updateNewSession(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.creation.step != createAgentName {
		t.Fatalf("create Agent choice step = %v", model.creation.step)
	}

	model.editor.SetValue("fresh coder")
	updated, _ = model.updateNewSession(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	model.editor.SetValue("deepseek-v4-flash")
	updated, _ = model.updateNewSession(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	model.editor.SetValue("Work carefully.")
	updated, _ = model.updateNewSession(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.creation.step != createAgentTools {
		t.Fatalf("step after system = %v", model.creation.step)
	}
	updated, _ = model.updateNewSession(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	updated, _ = model.updateNewSession(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	updated, command := model.updateNewSession(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if command == nil {
		t.Fatal("Agent review did not produce a create command")
	}
	updated, _ = model.Update(command())
	model = updated.(Model)
	if model.creation.agent.ID == "" || model.creation.step != createChooseEnvironment || model.creation.environmentCursor != 0 {
		t.Fatalf("Agent creation = %#v", model.creation)
	}
	updated, _ = model.updateNewSession(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.creation.step != createEnvironmentName {
		t.Fatalf("create Environment choice step = %v", model.creation.step)
	}

	model.editor.SetValue("fresh cloud")
	updated, command = model.updateNewSession(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)
	if model.creation.environment.ID == "" || model.creation.step != createSessionTitle {
		t.Fatalf("Environment creation = %#v", model.creation)
	}
}
