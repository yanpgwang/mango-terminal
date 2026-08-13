// Package timeline turns low-level Managed Agents wire events into semantic
// items that terminal renderers can present without understanding the API
// union. The durable event ledger remains the source of truth.
package timeline

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yanpgwang/mango-terminal/internal/api"
)

type Kind string

const (
	KindUser       Kind = "user"
	KindAgent      Kind = "agent"
	KindThinking   Kind = "thinking"
	KindDelegation Kind = "delegation"
	KindReport     Kind = "report"
	KindTool       Kind = "tool"
	KindResult     Kind = "result"
	KindPermission Kind = "permission"
	KindStatus     Kind = "status"
	KindError      Kind = "error"
)

type Item struct {
	ID       string
	Kind     Kind
	Label    string
	Body     string
	Agent    string
	Peer     string
	Tool     string
	Status   string
	ThreadID string
	RawType  string
}

type Preview struct {
	Kind Kind
	Body string
}

func Project(thread api.Thread, events []api.Event) []Item {
	items := make([]Item, 0, len(events))
	for _, event := range events {
		item, ok := projectEvent(thread, event)
		if ok {
			items = append(items, item)
		}
	}
	return items
}

func projectEvent(thread api.Thread, event api.Event) (Item, bool) {
	typ := stringValue(event["type"])
	item := Item{
		ID:       stringValue(event["id"]),
		ThreadID: thread.ID,
		RawType:  typ,
		Body:     contentText(event["content"]),
		Agent:    thread.Agent.Name,
	}
	switch typ {
	case "user.message":
		item.Kind, item.Label = KindUser, "You"
	case "agent.message":
		item.Kind, item.Label = KindAgent, first(thread.Agent.Name, "Agent")
	case "agent.thinking":
		item.Kind, item.Label = KindThinking, first(thread.Agent.Name, "Agent")+" is thinking"
	case "agent.thread_message_sent":
		item.Peer = stringValue(event["to_agent_name"])
		if thread.Primary() {
			item.Kind, item.Label = KindDelegation, "Delegated to "+first(item.Peer, "child Agent")
		} else {
			item.Kind, item.Label = KindReport, "Report to "+first(item.Peer, "coordinator")
		}
	case "agent.thread_message_received":
		item.Peer = stringValue(event["from_agent_name"])
		if thread.Primary() {
			item.Kind, item.Label = KindReport, "Report from "+first(item.Peer, "child Agent")
		} else {
			item.Kind, item.Label = KindDelegation, "Task from "+first(item.Peer, "coordinator")
		}
	case "agent.tool_use", "agent.mcp_tool_use", "agent.custom_tool_use":
		item.Tool = first(stringValue(event["name"]), "tool")
		item.Status = first(stringValue(event["evaluated_permission"]), "running")
		if item.Status == "ask" {
			item.Kind, item.Label = KindPermission, "Permission required · "+item.Tool
		} else {
			item.Kind, item.Label = KindTool, "Tool · "+item.Tool
		}
		item.Body = jsonText(event["input"])
	case "agent.tool_result", "agent.mcp_tool_result", "user.tool_result", "user.custom_tool_result":
		item.Kind, item.Label = KindResult, "Tool result"
		if boolValue(event["is_error"]) {
			item.Kind, item.Label = KindError, "Tool error"
		}
	case "session.error":
		item.Kind, item.Label = KindError, "Session error"
		item.Body = jsonText(event["error"])
	case "user.interrupt":
		item.Kind, item.Label = KindStatus, "Interrupted"
	case "session.thread_status_running", "session.thread_status_idle",
		"session.thread_status_rescheduled", "session.thread_status_terminated":
		item.Kind = KindStatus
		item.Status = strings.TrimPrefix(typ, "session.thread_status_")
		item.Label = first(stringValue(event["agent_name"]), thread.Agent.Name, "Thread") + " · " + item.Status
	default:
		// Spans and control-plane bookkeeping are available in raw mode later,
		// but are not product-level timeline entries.
		return Item{}, false
	}
	return item, true
}

func contentText(value any) string {
	items, ok := value.([]any)
	if !ok {
		return jsonText(value)
	}
	parts := make([]string, 0, len(items))
	for _, value := range items {
		block, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if text := stringValue(block["text"]); text != "" {
			parts = append(parts, text)
			continue
		}
		parts = append(parts, jsonText(block))
	}
	return strings.Join(parts, "\n")
}

func jsonText(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func boolValue(value any) bool {
	result, _ := value.(bool)
	return result
}

func first(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
