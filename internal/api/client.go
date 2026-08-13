package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	managedAgentsBeta = "managed-agents-2026-04-01"
	anthropicVersion  = "2023-06-01"
)

type Config struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

type Error struct {
	StatusCode int
	Status     string
	Detail     string
}

func (e *Error) Error() string {
	return fmt.Sprintf("Mango API %s: %s", e.Status, e.Detail)
}

func New(config Config) (*Client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid Mango URL %q", baseURL)
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		// Streaming requests are bounded by their context and must not inherit a
		// whole-response timeout from the client.
		httpClient = &http.Client{}
	}
	return &Client{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(config.APIKey),
		http:    httpClient,
	}, nil
}

func (c *Client) ListSessions(ctx context.Context) ([]Session, error) {
	path := "/v1/sessions?limit=100&order=desc"
	var sessions []Session
	for path != "" {
		var page struct {
			Data     []Session `json:"data"`
			NextPage *string   `json:"next_page"`
		}
		if err := c.get(ctx, path, &page); err != nil {
			return nil, err
		}
		sessions = append(sessions, page.Data...)
		path = nextPath("/v1/sessions?limit=100&order=desc", page.NextPage)
	}
	return sessions, nil
}

func (c *Client) GetSession(ctx context.Context, sessionID string) (Session, error) {
	var session Session
	err := c.get(ctx, "/v1/sessions/"+url.PathEscape(sessionID), &session)
	return session, err
}

func (c *Client) ListThreads(ctx context.Context, sessionID string) ([]Thread, error) {
	base := "/v1/sessions/" + url.PathEscape(sessionID) + "/threads?limit=100"
	path := base
	var threads []Thread
	for path != "" {
		var page struct {
			Data     []Thread `json:"data"`
			NextPage *string  `json:"next_page"`
		}
		if err := c.get(ctx, path, &page); err != nil {
			return nil, err
		}
		threads = append(threads, page.Data...)
		path = nextPath(base, page.NextPage)
	}
	return threads, nil
}

func (c *Client) ListThreadEvents(
	ctx context.Context,
	sessionID string,
	threadID string,
) ([]Event, error) {
	base := "/v1/sessions/" + url.PathEscape(sessionID) + "/threads/" +
		url.PathEscape(threadID) + "/events?limit=100"
	path := base
	var events []Event
	for path != "" {
		var page struct {
			Data     []Event `json:"data"`
			NextPage *string `json:"next_page"`
		}
		if err := c.get(ctx, path, &page); err != nil {
			return nil, err
		}
		events = append(events, page.Data...)
		path = nextPath(base, page.NextPage)
	}
	return events, nil
}

func (c *Client) SendMessage(ctx context.Context, sessionID, content string) error {
	return c.sendEvents(ctx, sessionID, []map[string]any{{
		"type":    "user.message",
		"content": []map[string]any{{"type": "text", "text": content}},
	}})
}

func (c *Client) Interrupt(ctx context.Context, sessionID, threadID string) error {
	event := map[string]any{"type": "user.interrupt"}
	if threadID != "" {
		event["session_thread_id"] = threadID
	}
	return c.sendEvents(ctx, sessionID, []map[string]any{event})
}

func (c *Client) sendEvents(
	ctx context.Context,
	sessionID string,
	events []map[string]any,
) error {
	return c.do(ctx, http.MethodPost,
		"/v1/sessions/"+url.PathEscape(sessionID)+"/events",
		map[string]any{"events": events}, nil,
	)
}

func (c *Client) get(ctx context.Context, path string, destination any) error {
	request, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return responseError(response)
	}
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		return fmt.Errorf("decode Mango response: %w", err)
	}
	return nil
}

func (c *Client) do(
	ctx context.Context,
	method string,
	path string,
	body any,
	destination any,
) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	request, err := c.newRequest(ctx, method, path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return responseError(response)
	}
	if destination == nil {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		return fmt.Errorf("decode Mango response: %w", err)
	}
	return nil
}

func (c *Client) newRequest(
	ctx context.Context,
	method string,
	path string,
	body io.Reader,
) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("anthropic-beta", managedAgentsBeta)
	request.Header.Set("anthropic-version", anthropicVersion)
	if c.apiKey != "" {
		request.Header.Set("x-api-key", c.apiKey)
	}
	if body != nil {
		request.Header.Set("content-type", "application/json")
	}
	return request, nil
}

func nextPath(base string, cursor *string) string {
	if cursor == nil || strings.TrimSpace(*cursor) == "" {
		return ""
	}
	separator := "?"
	if strings.Contains(base, "?") {
		separator = "&"
	}
	return base + separator + "page=" + url.QueryEscape(*cursor)
}

func responseError(response *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	detail := strings.TrimSpace(string(data))
	if json.Unmarshal(data, &envelope) == nil && envelope.Error.Message != "" {
		detail = envelope.Error.Message
	}
	if detail == "" {
		detail = http.StatusText(response.StatusCode)
	}
	return &Error{
		StatusCode: response.StatusCode,
		Status:     response.Status,
		Detail:     detail,
	}
}

func IsNotFound(err error) bool {
	var apiError *Error
	return errors.As(err, &apiError) && apiError.StatusCode == http.StatusNotFound
}
