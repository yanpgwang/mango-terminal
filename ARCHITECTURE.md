# Architecture

Mango Terminal is intentionally a remote client, not another Agent runtime.
It has three layers.

```text
managed-agent-go HTTP + SSE
            │
            ▼
 internal/mango     protocol, attach, history, streams, actions
            │
            ▼
 internal/feed      deterministic ledger → conversation projection
            │
            ▼
 internal/ui        cloud-window state, dialogs, and responsive rendering
```

## Protocol boundary

`internal/mango` owns the wire shapes and header contract. Attaching follows
the server's gap-free recommendation:

1. fetch the Session and Thread roster;
2. open an SSE subscription for every Thread;
3. after all response headers are open, list each durable ledger;
4. merge by persisted event ID.

The stream is not assumed to replay. Reconnection performs a new attach with
bounded exponential backoff. A `session.thread_created` event triggers roster
discovery and reattachment; Mango does not poll every Session continuously.
All per-Thread subscriptions form one aggregate: if any child stream ends,
the siblings are canceled so the complete roster is reattached rather than
silently leaving one Agent stale.

Session creation follows the control plane's resource boundary: choose or
create a versioned Agent and Environment, then create a Session that freezes
both snapshots. An optional initial `user.message` is sent as
`initial_events`, so the first turn can start as part of the creation request.
Resource selection uses one consistent dialog interaction: a live fuzzy
filter, arrow-key selection, Enter confirmation, and a final review before the
API mutation. Creating an Agent or Environment is a visible list choice rather
than a hidden shortcut.

## Projection boundary

`internal/feed` is pure. Given a Thread and its events, it creates conversation
items for messages, tools, delegations, reports, notices, and failures. It also
derives the latest aggregate `requires_action` barrier from the primary
ledger. This keeps rendering and network behavior independent.

Child actions require special care. The primary ledger contains a cross-posted
copy with a different, client-visible event ID plus `session_thread_id`. The
client responds with that visible ID. The server resolves it back to the owning
child Thread.

## Interaction boundary

Mango is organized around cloud lifecycle rather than a permanent composer:

```text
Connect  ─Enter→  Session home  ─Enter→  attached Session
  ▲                    ▲                         │
  └──── disconnect ────┘──────── Esc/detach ────┘
```

The Connect screen is currently a control-plane connection gate, not an
account login. The server contract has no production identity/device-login
endpoint yet. Likewise, the Session home refreshes the Session list explicitly
because the current API exposes SSE per Session and per Thread, not one
workspace-wide activity stream.

Inside an attached Session, the composer and conversation are the only two
focus states. The wide sidebar is passive Session context; it never steals
keyboard focus and never changes the observed Thread. Agent inspection is
explicit through a searchable picker. Sending always targets the coordinator,
matching the server contract. `Esc` detaches back to the Session home;
interrupting work is a separate confirmed action.

The conversation is a single visual stream. User messages use an accent rail,
assistant text grows in place from SSE deltas, tools remain compact until
expanded, and action gates open a focused, input-guarded decision dialog. The
current Thread's model and token usage sit directly below the conversation;
the passive sidebar is reserved for Session and Agent navigation.

With no attached Session, there is no chat editor. The home screen is a durable
Session list with visible create, find, and refresh choices. Session creation
opens a dialog flow and performs the control-plane mutation only after final
review.

The UI composes Bubble Tea v2 primitives rather than maintaining terminal
input mechanics itself: `textarea` owns multiline conversation/form input,
`textinput` owns searchable pickers, `viewport` owns conversation scrolling,
and `spinner` owns bounded activity feedback. Both text components expose a
real terminal cursor so IME candidate windows follow the actual insertion
point.

Ephemeral motion is deliberately kept outside the ledger projection. Streaming
previews, the fruit-based thinking signal, and new-event sparks can redraw
freely; only final events enter `internal/feed`. Motion can be disabled. Terminal focus reporting
suppresses notifications while the user is already looking at Mango, and raw
terminal notification protocols require explicit opt-in.

At 120×30 and above, the sidebar occupies 32 columns on the right. Smaller
terminals use a compact header and keep the conversation, activity strip, and
editor in one column. Below 60×20 the renderer replaces the workspace with a
bounded resize prompt; it never invents a larger canvas than the terminal.

## Demo backend

`internal/demo` is a Mango Backend implementation backed by in-memory event
ledgers. It exists for product review and UI regression, not as a parallel
protocol. Sending, interrupts, and action responses travel through the same UI
commands used by the HTTP client.
