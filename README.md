# Mango Terminal

Mango Terminal is a local terminal client for attaching to durable agents
running on a remote [Mango](https://github.com/yanpgwang/managed-agent-go)
control plane.

It is not an agent harness. Models, tools, sandboxes, state, and execution stay
on the server. The terminal projects the public Managed Agents event ledger
into an interactive view for observing and controlling work from any machine.

## Product model

```text
local terminal
    |
    | HTTPS + SSE
    v
Mango control plane
    |
    +-- durable Session
    |     +-- primary Thread
    |     +-- child Thread: researcher
    |     `-- child Thread: reviewer
    |
    `-- sandbox, Skills, Memory, Vaults, and Files
```

The current vertical slice supports:

- a remote Session inbox;
- attaching to an existing durable Session;
- primary and child Thread ledgers;
- live streams for every Thread, including background children;
- explicit delegation and report timeline items;
- paired, expandable tool calls and results;
- privacy-safe thinking state and incremental answer previews;
- sending messages to the primary Thread;
- targeted and Session-wide interrupts;
- a command palette and keyboard focus modes;
- durable built-in tool permission review, including child-owned actions.

## Run

```sh
go run ./cmd/mango --url http://127.0.0.1:8080
```

Attach directly:

```sh
go run ./cmd/mango --url http://127.0.0.1:8080 attach sesn_...
```

For authenticated deployments, set `MANGO_API_KEY` or pass `--api-key`.

Inside an attached Session:

| Key | Action |
| --- | --- |
| `Tab` / `Shift+Tab` | Move across the coordinator and specialist Threads |
| `Esc` | Move from the composer into the observable timeline |
| `i` | Return to the composer |
| `Space` | Expand or collapse the selected timeline item |
| `Ctrl+K` | Open the Session command palette |
| `Ctrl+P` | Review durable permission actions |
| `Ctrl+X` | Interrupt the selected Thread |

## Status

This repository is an early product foundation. See
[PRODUCT.md](docs/PRODUCT.md) for the interaction model and delivery sequence.

## Independence

Mango Terminal is a clean implementation built on the permissively licensed
Charm libraries. It studies established terminal interaction patterns but does
not copy or depend on Crush source code. See [ARCHITECTURE.md](ARCHITECTURE.md).

## License

Apache-2.0.
