# Mango

Mango is a terminal window into durable managed Agents. It is not a local
coding harness: execution, tools, sandboxes, memory, and orchestration remain
in the cloud while this client connects, observes, creates, responds, and
detaches.

This repository is a clean implementation against the public
`managed-agent-go` HTTP and event-stream contract. Models, tools, sandboxes,
memory, and execution remain on the Mango control plane.

## Try it now

No server is required for the interactive demo:

```sh
go run ./cmd/mango --demo
```

Or use the built binary:

```sh
go build -o bin/mango ./cmd/mango
./bin/mango --demo
```

The demo starts at the same central Connect screen as a real control plane and
includes a coordinator, two child Agents, delegation events, a tool call, and
a child-owned permission request. It uses the same state projection and action
code as a real server connection.

## Connect to Mango

The default control plane is `http://127.0.0.1:8080`:

```sh
go run ./cmd/mango
```

Connect elsewhere or attach directly to one durable Session:

```sh
MANGO_URL=https://mango.example.com \
MANGO_API_KEY=your-key \
go run ./cmd/mango attach sesn_...
```

Equivalent flags are `--url` and `--api-key`. Finite API requests time out
after 30 seconds; long-lived event streams are governed by their attachment
context and reconnect independently.

The welcome screen probes the configured, saved, and common local endpoints
without delaying the UI. Press `Enter` on Endpoint to open the picker, use
`Up`/`Down` and `Enter` to select, then press `Enter` on Connect. Press `e` to
add an `http://` or `https://` endpoint manually. A successful connection is
remembered in the user configuration directory as `mango/connection.json`;
API keys are never written to that file.

## Interaction

The normal path is visible on screen and does not require memorizing commands:

1. The framed Mango welcome uses a gently animated cloud scene and an endpoint
   picker. It detects saved and common local candidates, accepts a manual URL,
   validates the connection, and remembers the successful selection.
2. The home screen lists `Create a new Session`, `Find a Session`, `Refresh
   from Cloud`, and every durable Session.
   Press `m` on a Session to rename it, interrupt all active work, archive it,
   or permanently delete it and its event history. Destructive operations open
   with the safe choice selected, and running Sessions must be interrupted
   before archive or deletion.
3. New Session is a sequence of searchable dialogs: Agent, Environment,
   Session details, review. `Create a new Agent` and `Create a cloud
   Environment` are ordinary choices in those lists.
4. An attached Session opens one workspace: the coordinator conversation stays
   on the left while a focusable Subagent workspace on the right shows every
   live child, undelegated roster member, latest activity, usage, unread work,
   and pending action. Opening a child swaps only the left transcript; the rail
   remains visible.
5. `Esc` returns to the Session home without interrupting remote work.

The editor appears only inside an attached Session or a form that genuinely
needs text. User messages always target the coordinator because the Managed
Agents API does not accept a user message targeted directly at a child Thread.
When a child transcript is open, the footer makes that routing explicit and
`Esc` returns to the coordinator before leaving the Session.

Arrow keys choose or scroll, `Enter` selects or sends, and `Esc` closes or goes
back. Those three keys cover the main product. On wide terminals, `Tab` cycles
through the composer, conversation, and Subagent workspace; `Enter` opens a
child transcript, `Space` previews it without leaving the rail, and `x` opens
an explicit child interrupt confirmation. `Ctrl+P`, `Ctrl+G`, `Ctrl+S`, and
`Ctrl+N` remain optional accelerators for commands, Agents, Session search, and
creation. The command palette exposes the same Session management flow while
attached. `Ctrl+C` exits the terminal without stopping remote work.

Every searchable picker and text editor uses a real terminal cursor from the
Bubble Tea v2 components. This gives Chinese/Japanese/Korean IMEs a stable
candidate-window anchor instead of a painted fake cursor.

At 120×30 and above, the Subagent workspace is a dedicated right column. It
absorbs the useful parts of a task footer—status, task, elapsed activity, tokens,
and unread work—then adds Mango-specific Thread transcripts, undelegated roster
members, child-owned approvals, and per-child interruption. Below that size it
collapses into a one-line roster strip and the searchable Agent picker remains
available. Permission and external-tool gates use guarded decision dialogs so
an in-flight keystroke cannot accidentally approve work.

Mango has a small motion language of its own: a privacy-safe rotating fruit
signal while an Agent thinks, in-place streaming with a live caret, and a
short-lived spark on newly durable Agent events.
Use `--no-motion` or `MANGO_NO_MOTION=1` for a static presentation.

Background notification escape sequences are disabled by default because
unsupported terminals may print them literally. Opt in with `--notify bell`
for the portable terminal bell or `--notify osc777` when the terminal is known
to support OSC 777. `MANGO_NOTIFY` provides the same setting. Focused windows
never notify.

## Protocol behavior

Mango treats Thread event ledgers as the source of truth:

- attach opens every known Thread SSE stream before listing history, then
  deduplicates persisted events by ID;
- `event_start` and `event_delta` are projected as ephemeral live previews;
- final `agent.message` events replace previews with the same ID;
- tool results are paired back into their original tool calls;
- child action copies on the primary ledger retain their client-visible event
  ID and `session_thread_id` routing hint;
- `session.thread_created` triggers roster discovery and a clean reattach, so
  there is no continuous API poll;
- if any child Thread stream ends, the aggregate subscription reconnects all
  Threads with bounded exponential backoff rather than silently losing one
  Agent.

Terminals below 60×20 show a bounded resize prompt instead of wrapping dialogs
or corrupting the screen.

See [ARCHITECTURE.md](ARCHITECTURE.md) for the state model.

## Install a release

On macOS or Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/yanpgwang/mango-terminal/main/install.sh | sh
```

## License

Apache-2.0.
