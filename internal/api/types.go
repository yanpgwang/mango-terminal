package api

import "time"

type Session struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	Agent     Agent     `json:"agent"`
	Usage     Usage     `json:"usage"`
}

type Agent struct {
	Name  string `json:"name"`
	Model struct {
		ID string `json:"id"`
	} `json:"model"`
}

type Usage struct {
	InputTokens          int64   `json:"input_tokens"`
	OutputTokens         int64   `json:"output_tokens"`
	CacheReadInputTokens int64   `json:"cache_read_input_tokens"`
	ActiveSeconds        float64 `json:"active_seconds"`
}

type Thread struct {
	ID             string  `json:"id"`
	ParentThreadID *string `json:"parent_thread_id"`
	Status         string  `json:"status"`
	Agent          Agent   `json:"agent"`
}

func (t Thread) Primary() bool { return t.ParentThreadID == nil }

type Event map[string]any

type StreamUpdate struct {
	ThreadID string
	Frame    Event
	Err      error
}
