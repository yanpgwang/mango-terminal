package ui

import (
	"strings"
	"testing"

	"github.com/yanpgwang/mango-terminal/internal/api"
	"github.com/yanpgwang/mango-terminal/internal/timeline"
)

func TestSelectThreadWrapsAndClearsUnread(t *testing.T) {
	model := New(nil, "")
	parent := "primary"
	model.threads = []api.Thread{
		{ID: "primary"},
		{ID: "child", ParentThreadID: &parent},
	}
	model.threadCursor = 0
	model.unread["child"] = 3
	model.selectThread(1)
	if model.currentThreadID() != "child" || model.unread["child"] != 0 {
		t.Fatalf("selected = %q, unread = %d", model.currentThreadID(), model.unread["child"])
	}
	model.selectThread(1)
	if model.currentThreadID() != "primary" {
		t.Fatalf("wrapped selection = %q", model.currentThreadID())
	}
}

func TestRebuildPendingFindsChildConfirmationOnPrimaryLedger(t *testing.T) {
	model := New(nil, "")
	model.threads = []api.Thread{{ID: "primary"}}
	model.events["primary"] = []api.Event{
		{
			"id": "permission", "type": "agent.tool_use", "name": "write",
			"evaluated_permission": "ask", "session_thread_id": "child",
		},
		{
			"id": "idle", "type": "session.status_idle",
			"stop_reason": map[string]any{
				"type": "requires_action", "event_ids": []any{"permission"},
			},
		},
	}
	model.rebuildPending()
	if len(model.pending) != 1 || model.pending[0].kind != "tool_confirmation" ||
		model.pending[0].threadID != "child" {
		t.Fatalf("pending = %#v", model.pending)
	}
}

func TestTimelineRenderShowsDelegationAndToolState(t *testing.T) {
	theme := defaultTheme()
	delegation := renderTimelineItem(theme, timeline.Item{
		ID: "delegation", Kind: timeline.KindDelegation, Label: "Delegated to researcher",
		Body: "Compare three primary sources.",
	}, 72, false, true)
	if !strings.Contains(delegation, "Delegated to researcher") || !strings.Contains(delegation, "Compare three") {
		t.Fatalf("delegation render = %q", delegation)
	}
	tool := renderTimelineItem(theme, timeline.Item{
		ID: "tool", Kind: timeline.KindTool, Tool: "bash", Status: "complete",
		Body: `{"command":"go test ./..."}`, Result: "ok",
	}, 72, false, false)
	if !strings.Contains(tool, "bash") || !strings.Contains(tool, "complete") || !strings.Contains(tool, "ok") {
		t.Fatalf("tool render = %q", tool)
	}
}

func TestSameThreadRosterIgnoresStatusChanges(t *testing.T) {
	left := []api.Thread{{ID: "primary", Status: "running"}}
	right := []api.Thread{{ID: "primary", Status: "idle"}}
	if !sameThreadRoster(left, right) {
		t.Fatal("status-only refresh must not restart all streams")
	}
}
