package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientListsSessionsAndSetsProtocolHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/sessions" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if request.Header.Get("x-api-key") != "test-key" {
			t.Errorf("x-api-key = %q", request.Header.Get("x-api-key"))
		}
		if request.Header.Get("anthropic-beta") != managedAgentsBeta {
			t.Errorf("anthropic-beta = %q", request.Header.Get("anthropic-beta"))
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"data": []map[string]any{{
				"id": "sesn_one", "title": "One", "status": "idle",
				"agent": map[string]any{"name": "Agent"},
			}},
			"next_page": nil,
		})
	}))
	defer server.Close()

	client, err := New(Config{BaseURL: server.URL, APIKey: "test-key", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := client.ListSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].ID != "sesn_one" {
		t.Fatalf("sessions = %#v", sessions)
	}
}

func TestClientSendsTargetedInterrupt(t *testing.T) {
	var event map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Events []map[string]any `json:"events"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		event = body.Events[0]
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Interrupt(context.Background(), "sesn_one", "sthr_child"); err != nil {
		t.Fatal(err)
	}
	if event["type"] != "user.interrupt" || event["session_thread_id"] != "sthr_child" {
		t.Fatalf("event = %#v", event)
	}
}
