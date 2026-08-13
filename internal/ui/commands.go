package ui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/yanpgwang/mango-terminal/internal/api"
)

func (m Model) loadInbox() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		sessions, err := m.client.ListSessions(ctx)
		return inboxLoaded{sessions: sessions, err: err}
	}
}

func (m Model) loadAttached(sessionID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		session, err := m.client.GetSession(ctx, sessionID)
		if err != nil {
			return attachedLoaded{err: err}
		}
		threads, err := m.client.ListThreads(ctx, sessionID)
		if err != nil {
			return attachedLoaded{err: err}
		}
		events := make(map[string][]api.Event, len(threads))
		for _, thread := range threads {
			events[thread.ID], err = m.client.ListThreadEvents(ctx, sessionID, thread.ID)
			if err != nil {
				return attachedLoaded{err: err}
			}
		}
		return attachedLoaded{session: session, threads: threads, events: events}
	}
}

func (m Model) loadSummary() tea.Cmd {
	sessionID := m.session.ID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		session, err := m.client.GetSession(ctx, sessionID)
		if err != nil {
			return summaryLoaded{err: err}
		}
		threads, err := m.client.ListThreads(ctx, sessionID)
		return summaryLoaded{session: session, threads: threads, err: err}
	}
}

func (m Model) loadMissingLedgers() tea.Cmd {
	// A full durable reload keeps ordering and reconnect semantics simple. This
	// path runs only when the Thread roster changes.
	return m.loadAttached(m.session.ID)
}

func (m *Model) restartStreams() tea.Cmd {
	m.stopStreams()
	if m.session == nil || len(m.threads) == 0 {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.streamCancel = cancel
	m.stream = m.client.SubscribeSession(ctx, m.session.ID, m.threads)
	return m.waitStream(m.stream)
}

func (m *Model) stopStreams() {
	if m.streamCancel != nil {
		m.streamCancel()
	}
	m.streamCancel, m.stream = nil, nil
}

func (m Model) waitStream(stream <-chan api.StreamUpdate) tea.Cmd {
	return func() tea.Msg {
		update, open := <-stream
		return streamMessage{stream: stream, update: update, open: open}
	}
}

func (m Model) refreshAfter() tea.Cmd {
	return tea.Tick(3*time.Second, func(now time.Time) tea.Msg { return refreshTick(now) })
}

func (m Model) sendMessage(content string) tea.Cmd {
	sessionID := m.session.ID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		err := m.client.SendMessage(ctx, sessionID, content)
		return actionFinished{label: "message sent", err: err}
	}
}

func (m Model) interrupt(threadID string) tea.Cmd {
	sessionID := m.session.ID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		err := m.client.Interrupt(ctx, sessionID, threadID)
		label := "all Threads interrupted"
		if threadID != "" {
			label = "Thread interrupted"
		}
		return actionFinished{label: label, err: err}
	}
}
