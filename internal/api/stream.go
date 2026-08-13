package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// SubscribeSession multiplexes the independent streams for every known Thread.
// A caller can therefore keep background child activity visible while reading
// the primary ledger or another child. A refreshed Thread roster should replace
// this subscription so newly-created children receive their own stream.
func (c *Client) SubscribeSession(
	ctx context.Context,
	sessionID string,
	threads []Thread,
) <-chan StreamUpdate {
	updates := make(chan StreamUpdate, max(32, len(threads)*8))
	var workers sync.WaitGroup
	workers.Add(len(threads))
	for _, thread := range threads {
		threadID := thread.ID
		go func() {
			defer workers.Done()
			c.subscribeThread(ctx, sessionID, threadID, updates)
		}()
	}
	go func() {
		workers.Wait()
		close(updates)
	}()
	return updates
}

func (c *Client) subscribeThread(
	ctx context.Context,
	sessionID string,
	threadID string,
	updates chan<- StreamUpdate,
) {
	path := "/v1/sessions/" + url.PathEscape(sessionID) + "/threads/" +
		url.PathEscape(threadID) +
		"/stream?event_deltas%5B%5D=agent.message&event_deltas%5B%5D=agent.thinking"
	request, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		sendUpdate(ctx, updates, StreamUpdate{ThreadID: threadID, Err: err})
		return
	}
	request.Header.Set("accept", "text/event-stream")
	response, err := c.http.Do(request)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			sendUpdate(ctx, updates, StreamUpdate{ThreadID: threadID, Err: err})
		}
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		sendUpdate(ctx, updates, StreamUpdate{ThreadID: threadID, Err: responseError(response)})
		return
	}

	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), 2<<20)
	var data strings.Builder
	flush := func() bool {
		if data.Len() == 0 {
			return true
		}
		var frame Event
		err := json.Unmarshal([]byte(data.String()), &frame)
		data.Reset()
		return sendUpdate(ctx, updates, StreamUpdate{ThreadID: threadID, Frame: frame, Err: err})
	}
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			continue
		}
		if line == "" && !flush() {
			return
		}
	}
	if !flush() {
		return
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) {
		sendUpdate(ctx, updates, StreamUpdate{ThreadID: threadID, Err: err})
	}
}

func sendUpdate(
	ctx context.Context,
	destination chan<- StreamUpdate,
	update StreamUpdate,
) bool {
	select {
	case destination <- update:
		return true
	case <-ctx.Done():
		return false
	}
}
