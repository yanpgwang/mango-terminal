package timeline

import (
	"testing"

	"github.com/yanpgwang/mango-terminal/internal/api"
)

func TestProjectDelegationAndReportByThreadRole(t *testing.T) {
	primary := api.Thread{ID: "primary"}
	primary.Agent.Name = "coordinator"
	parent := "primary"
	child := api.Thread{ID: "child", ParentThreadID: &parent}
	child.Agent.Name = "researcher"

	delegation := api.Event{
		"id": "one", "type": "agent.thread_message_sent",
		"to_agent_name": "researcher",
		"content":       []any{map[string]any{"type": "text", "text": "Find evidence"}},
	}
	received := api.Event{
		"id": "two", "type": "agent.thread_message_received",
		"from_agent_name": "coordinator",
		"content":         []any{map[string]any{"type": "text", "text": "Find evidence"}},
	}
	report := api.Event{
		"id": "three", "type": "agent.thread_message_sent",
		"to_agent_name": "coordinator",
		"content":       []any{map[string]any{"type": "text", "text": "Evidence found"}},
	}
	receivedReport := api.Event{
		"id": "four", "type": "agent.thread_message_received",
		"from_agent_name": "researcher",
		"content":         []any{map[string]any{"type": "text", "text": "Evidence found"}},
	}

	primaryItems := Project(primary, []api.Event{delegation, receivedReport})
	if primaryItems[0].Kind != KindDelegation || primaryItems[1].Kind != KindReport {
		t.Fatalf("primary projection = %#v", primaryItems)
	}
	childItems := Project(child, []api.Event{received, report})
	if childItems[0].Kind != KindDelegation || childItems[1].Kind != KindReport {
		t.Fatalf("child projection = %#v", childItems)
	}
}

func TestProjectThinkingDoesNotExposeProviderReasoning(t *testing.T) {
	thread := api.Thread{ID: "thread"}
	thread.Agent.Name = "reviewer"
	items := Project(thread, []api.Event{{
		"id": "thinking", "type": "agent.thinking",
		"thinking": "private chain of thought",
	}})
	if len(items) != 1 || items[0].Kind != KindThinking || items[0].Body != "" {
		t.Fatalf("thinking projection = %#v", items)
	}
}

func TestProjectSuppressesBookkeepingSpans(t *testing.T) {
	items := Project(api.Thread{ID: "thread"}, []api.Event{{
		"id": "span", "type": "span.model_request_start",
	}})
	if len(items) != 0 {
		t.Fatalf("bookkeeping projection = %#v, want none", items)
	}
}
