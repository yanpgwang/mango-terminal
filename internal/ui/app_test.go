package ui

import (
	"testing"

	"github.com/yanpgwang/mango-terminal/internal/api"
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

func TestSameThreadRosterIgnoresStatusChanges(t *testing.T) {
	left := []api.Thread{{ID: "primary", Status: "running"}}
	right := []api.Thread{{ID: "primary", Status: "idle"}}
	if !sameThreadRoster(left, right) {
		t.Fatal("status-only refresh must not restart all streams")
	}
}
