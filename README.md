# Mango

**A terminal control surface for durable managed Agents.**

Mango connects to a remote Managed Agents control plane so you can create,
observe, guide, and detach from long-running Agent Sessions without moving
execution into your terminal. Tools, sandboxes, memory, models, and
orchestration remain in the cloud.

![Mango terminal demo](docs/assets/mango-demo.gif)

## What it gives you

- One workspace for the coordinator, child Agents, transcripts, usage, unread
  work, delegation state, and pending actions.
- Durable Session lifecycle management: create, find, attach, rename,
  interrupt, archive, delete, and safely detach while work continues.
- Live HTTP/SSE projection with streaming previews, persisted-event
  deduplication, and whole-roster reconnection when a child stream ends.
- Guarded approval and tool-result dialogs that preserve the owning child
  Thread and its server-visible event ID.
- Responsive wide and compact layouts, real terminal cursors for CJK IMEs,
  optional motion, and opt-in background notifications.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/yanpgwang/mango-terminal/main/install.sh | sh
```

Then launch Mango directly:

```sh
mango
```

## Try the built-in demo

No server is required:

```sh
mango --demo
```

The demo uses the production UI and state projection with an in-memory
backend. It includes a coordinator, two active child Agents, delegation and
tool events, an undelegated roster member, and a child-owned permission gate.

## Connect to Mango

Mango defaults to `http://127.0.0.1:8080`:

```sh
mango
```

Connect to another control plane or attach directly to a durable Session:

```sh
MANGO_URL=https://mango.example.com \
MANGO_API_KEY=your-key \
mango attach sesn_...
```

Equivalent flags are `--url` and `--api-key`. The selected endpoint is
remembered in `mango/connection.json` under the user configuration directory;
API keys are never written there.

## Controls

The main flow uses arrow keys, `Enter`, and `Esc`. In an attached Session:

- `Tab` cycles through the composer, conversation, and Subagent workspace.
- `Enter` opens a selected child transcript; replies still go to the
  coordinator.
- `Space` previews a child without leaving the Subagent workspace.
- `Ctrl+P`, `Ctrl+G`, `Ctrl+S`, and `Ctrl+N` open commands, Agents, Session
  search, and Session creation.
- `Ctrl+C` exits Mango without stopping remote work.

Use `--no-motion` or `MANGO_NO_MOTION=1` for static rendering. Notifications
are disabled by default; opt in with `--notify bell` or `--notify osc777`.

## Built with

- The public `managed-agent-go` HTTP and event-stream contract.
- [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) for the terminal
  application runtime.
- [Bubbles v2](https://github.com/charmbracelet/bubbles) for editors, search,
  scrolling, and activity components.
- [Lip Gloss v2](https://github.com/charmbracelet/lipgloss) for responsive
  layout and styling.
- [Glamour](https://github.com/charmbracelet/glamour) for terminal Markdown.

See [ARCHITECTURE.md](ARCHITECTURE.md) for the protocol, projection, and UI
boundaries.

## Development

```sh
go test ./...
go build -o bin/mango ./cmd/mango
go run ./cmd/mango --demo
vhs demo/welcome.tape
```

The VHS tape records the README GIF to `docs/assets/` and an MP4 copy to the
ignored `dist/` directory. It uses Menlo with explicit terminal metrics so the
recording is reproducible on macOS.

## License

Apache-2.0.
