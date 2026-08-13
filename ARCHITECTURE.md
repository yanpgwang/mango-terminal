# Architecture

Mango Terminal is deliberately separated from the Mango server repository and
from local coding-agent harnesses.

## Boundaries

- `internal/api` owns the public HTTP and SSE protocol.
- `internal/timeline` projects durable wire events into product-level timeline
  items. It contains no terminal rendering code.
- `internal/ui` owns Bubble Tea state, layout, input, and rendering. It never
  calls model providers or executes agent tools.
- `cmd/mango` only parses process configuration and starts the application.

The UI must remain replaceable without changing the API client, and wire-event
growth must be absorbed by the projection layer rather than scattered across
render functions.

## Remote-first invariants

1. The server event ledger is the source of truth. Local state is a cache.
2. Reattaching reconstructs the same view from persisted events before opening
   live streams.
3. Every primary and child Thread has an independent ledger and stream.
4. A disconnected terminal never stops cloud execution.
5. Thinking content is private. The terminal may render lifecycle state but
   not hidden provider reasoning.
6. Permission and interrupt actions always carry their authoritative Session
   and Thread identity.

## UI direction

The product uses Bubble Tea, Bubbles, Lip Gloss, and Glamour directly. It takes
inspiration from mature terminal products while keeping an original component
and state model designed for Managed Agents:

- Session inbox rather than a local workspace landing page;
- attach and reconnect rather than starting a local model loop;
- delegation/report items rather than an opaque generic subagent tool;
- concurrent Thread activity rather than one linear transcript;
- server permissions and durable interrupts rather than local process control.

