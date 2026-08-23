# Runtime architecture

The production image contains the Go service and the compiled React client.
Caddy is the public edge and forwards both HTTP and WebSocket requests to the
service. PostgreSQL is the durable identity and event/snapshot store.

```text
browser ── HTTPS / WebSocket ──> Caddy ──> Go service ──> PostgreSQL
                                      └── in-memory active room loops
```

The game package has no HTTP, WebSocket, or SQL dependencies. It owns the board
topology, deployment validation, legal movement, combat, clocks, elimination,
draws, and visibility projections. Rooms serialize commands and timer ticks
under one lock and broadcast a projection for each participant. A spectator
projection always omits ranks, including in fully visible rooms.

The current v1 runtime keeps active room state in memory for low-latency command
processing. Sessions, room metadata, match events, and periodic snapshots are
persisted when PostgreSQL is available. On startup, active rooms are rebuilt
from their latest canonical event payload and participants can reconnect using
their persisted username session. A no-`DATABASE_URL` local run deliberately
falls back to ephemeral sessions and rooms so the frontend and rules can be
exercised without a database.

Room lifecycle commands are authoritative server commands and are available to
all room members, including spectators. A full lobby can be moved into setup;
an active match can be paused and resumed without losing its remaining turn
clock; stopping records a stopped result; and resetting returns the room to the
lobby while preserving seated participants and restoring default deployments.
