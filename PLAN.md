# Army Chess — Project Plan

## 1. Product and rules specification

### Product goal

Build a self-hosted browser application for four-player, two-versus-two 四国军棋 with:

- Private invite-code rooms.
- Four player seats plus up to 50 spectators by default.
- Authoritative server-side rules and hidden-information filtering.
- Reconnection, persistent matches, full replays, saved deployments, match history, and basic statistics.
- Docker Compose deployment on one personal server.

The rules baseline is the common JJ-style ruleset, which documents the conventional 25-piece army, protected camps, railway movement, setup restrictions, timeouts, elimination, and draws. Because published sources acknowledge that 四国军棋 has no single official ruleset, every disputed behavior is fixed below rather than inferred. See the [JJ rules](https://www.jj.cn/news/320/20110927135600018503.shtml) and [rules-variation overview](https://ty.httpcn.com/baike/qipai_siguojunqi.shtml).

### Current implementation status

As of the current v1 milestone, the repository contains a deployable vertical
slice rather than the complete roadmap. The implemented and verified path is:

- Go HTTP/WebSocket service with passwordless username sessions, rooms,
  spectators, reconnect, host transfer, setup, gameplay, draw/resign flows,
  rematch, replay, profiles, and saved-layout APIs.
- Pure v1 game rules for deployment, movement, combat, visibility, clocks,
  elimination, team victory, and draws, with unit and projection tests.
- PostgreSQL 18 migrations, canonical event persistence, snapshots, active-room
  recovery, and profile/match history queries.
- React/Vite client for sign-in, room entry, setup, live play, spectator views,
  profile pages, and full-truth replay navigation.
- Dockerfile, Compose, Caddy TLS routing, health/readiness checks, operations
  documentation, and a GHCR publishing workflow.

Verification completed for this milestone includes `go test -race ./...`,
`go vet ./...`, the frontend production build, Docker image builds, Compose
migration/startup, Postgres room recovery after restart, HTTPS SPA fallback,
profile API access, and an authenticated WebSocket upgrade through Caddy.

The remaining unchecked roadmap items are intentional follow-up work. The most
important gaps are the exact production board topology, password/ownership
authentication, rate limiting and metrics, the full saved-layout frontend,
perspective replay, automated browser E2E coverage, load testing, and a
published-image Compose variant. See the unchecked items in Phases 0–7 below.

### Seats, teams, and room lifecycle

- Seats are `north`, `east`, `south`, and `west`.
- North/South form one team; East/West form the other.
- Players choose an open seat.
- The opening seat is selected randomly and persisted; turns proceed clockwise.
- Room phases are `lobby → setup → playing → finished → lobby`.
- All four seats must be occupied before setup begins.
- The host chooses visibility mode and clock preset before setup.
- If the host disconnects or leaves, host status transfers to the longest-present connected player.
- A rematch returns everyone to the lobby, clears all seats, and allows reseating under the same room code.
- Rooms use an eight-character, ambiguity-free invite code and have no password or approval step.
- Anyone with the code may take an open seat or join explicitly as a spectator.
- Spectators are capped by `MAX_SPECTATORS_PER_ROOM`, defaulting to 50.

### Armies and deployment

Each player has 25 pieces:

| Piece | Count |
|---|---:|
| 军旗 | 1 |
| 司令 | 1 |
| 军长 | 1 |
| 师长、旅长、团长、营长、炸弹 | 2 each |
| 连长、排长、工兵、地雷 | 3 each |

Deployment rules:

- All 25 pieces occupy the player's 23兵站 and two大本营 positions.
- 行营 must begin empty.
- 军旗 must occupy one of the two大本营.
- 地雷 must be placed in the final two rows.
- 炸弹 cannot be placed in the front row.
- Setup editing supports click-swap and drag-swap; every intermediate deployment remains server-valid.
- Saved layouts are named and stored in PostgreSQL by username.
- A valid built-in default layout is always available.
- When setup time expires, the server submits the player's current valid layout automatically.

### Board and movement

- Model the board as versioned nodes and edges rather than deriving movement from pixels.
- Node types are `station`, `camp`, and `headquarters`; edge types are `road` and `rail`.
- Public board geometry lives in a shared `board.v1.json` contract containing stable node IDs, topology, and normalized rendering coordinates.
- On roads, a movable piece travels one adjacent edge.
- On unobstructed railways, ordinary pieces may travel any distance on one continuous railway route but cannot turn at a right-angle junction.
- Engineers may follow any connected, unobstructed railway path, including turns.
- 军旗 and 地雷 never move.
- Any piece that enters or starts in大本营 cannot move again.
- A piece in行营 cannot be attacked.
- Allied pieces cannot attack each other and block occupied routes like any other piece.

### Combat and victory

Rank order is:

```text
司令 > 军长 > 师长 > 旅长 > 团长 > 营长 > 连长 > 排长 > 工兵
```

- Higher rank survives on the destination node.
- Equal ranks remove both pieces.
- 工兵 removes地雷 and survives.
- Any other ranked piece attacking地雷 is removed while the mine survives.
- 炸弹 and its opponent are both removed, including against地雷 or军旗.
- When a司令 is removed, that player's军旗 location becomes visible to everyone.
- Combat reveals the outcome, but not the opponent's exact rank unless the selected visibility mode already permits it.
- Allies never fight each other.

A player is eliminated when:

- Their军旗 is captured.
- They resign.
- They have no legal move at the start of their turn.
- They accumulate five missed turn deadlines during one match.

On elimination:

- All remaining pieces belonging to that player are removed.
- Their future turns are skipped.
- They remain connected as a spectator.
- Their teammate continues alone.

A team loses when both teammates are eliminated.

### Visibility modes

Rooms support all three modes from v1:

- `four_dark`: each player sees only their own ranks.
- `double_visible`: players also see their teammate's ranks.
- `fully_visible`: all active players see every rank.
- Live spectators always receive the public hidden view, even in a fully-visible room.
- Public view contains positions, owners, moves, removals, turn state, and explicitly revealed flags, but no other unrevealed ranks.
- After a match finishes, everyone with the replay link may inspect full truth or any seat perspective.

This follows the strongest lesson from existing hidden-information projects: maintain one canonical state and derive permission-scoped views server-side. See [Mistboard's architecture](https://github.com/brianhliou/mistboard#code), [online-junqi](https://github.com/samuelyuan/online-junqi), and the [four-player reference](https://github.com/jmctsh/open_junqi).

### Clocks, draws, and communication

Clock presets:

| Preset | Setup | Per turn | Elimination |
|---|---:|---:|---:|
| Fast | 60 seconds | 20 seconds | 5 cumulative misses |
| Standard | 120 seconds | 60 seconds | 5 cumulative misses |
| Relaxed | 300 seconds | 120 seconds | 5 cumulative misses |

- Server timestamps and deadlines are authoritative.
- A turn timeout skips the move and advances to the next active player.
- Disconnecting does not pause a clock.
- A draw offer succeeds only when every active player accepts.
- A rejection or successful move cancels the pending offer.
- One pending draw offer is allowed at a time.
- The server declares a draw after 70 consecutive moves without any piece being removed; any combat removal resets the counter.
- Active matches provide system messages only: no chat, teammate chat, reactions, or spectator reactions.

### Identity and profile policy

- Signing in requires only a username; there is no password, email, passkey, or ownership proof.
- Usernames are globally unique records and match `^[a-z][a-z_]{1,18}[a-z]$`.
- Anyone who enters an existing username can impersonate it and access its profile, match history, active games, and saved layouts. This is an explicitly accepted v1 limitation and must be stated beside the sign-in form.
- The server still issues an opaque, HttpOnly session cookie for connection continuity, rate limiting, and room reconnection.
- Profiles show completed matches, wins/losses/draws, results by visibility mode, and replay links.
- No rating or leaderboard is included.

## 2. Technical architecture, interfaces, and layout

### Selected stack

Use current patched releases within these majors:

- Go 1.26 with Chi v5.
- `coder/websocket` for WebSockets.
- PostgreSQL 18 with native `pgx/v5`; the current vertical slice uses explicit
  repository code and plain SQL migrations rather than `sqlc` or `tern`.
- React 19.2, TypeScript 6, and Vite 8.
- The current client uses pathname routing and local React hooks. React Router,
  TanStack Query, Zustand selectors, and Zod remain planned refinements.
- A shared CSS stylesheet and design tokens; no general-purpose component
  framework.
- Go standard testing with race coverage; Vitest is installed, while React
  Testing Library and Playwright remain planned additions.
- Caddy for static files, reverse proxying, WebSocket upgrades, and automatic TLS.
- Docker Compose with no Redis in v1.

The version choices align with the current supported releases and official policies. See [Go releases](https://go.dev/doc/devel/release), [React versions](https://react.dev/versions), [Vite releases](https://vite.dev/releases), and the [PostgreSQL policy](https://www.postgresql.org/support/versioning/).

### Runtime architecture

```text
Browser
  ├── HTTP API
  └── WebSocket
         |
       Caddy
         |
   Go application
  ├── session/profile service
  ├── room registry
  ├── serialized state and timer handling per active room
  ├── pure game engine
  ├── visibility projector
  └── replay/recovery service
         |
    PostgreSQL 18
```

- Each active room serializes commands and timer ticks under a per-room mutex;
  connection writes have their own per-participant write locks.
- The game engine receives state plus a command and returns a new state plus domain events; it has no HTTP, WebSocket, SQL, or UI imports.
- Persist accepted domain events before publishing them to clients.
- Keep active state in memory and persist a snapshot on the first event, every
  10 events, and when a match finishes.
- Recover active rooms from the latest canonical persisted event payload and
  participant records after restart.
- Store deadline timestamps so overdue turns can be resolved after recovery.
- Graceful shutdown closes the HTTP service; accepted events remain durable and
  active rooms can be reconstructed on the next start.
- A single backend instance owns rooms in v1; Redis and multi-instance routing remain future work.

### Repository layout

```text
army_chess/
├── server/
│   ├── cmd/army-chess/        # process entrypoint
│   ├── internal/game/         # pure state, rules, events, visibility, replay
│   ├── internal/rooms/        # room state, clocks, subscribers, recovery
│   ├── internal/httpapi/      # Chi handlers and middleware
│   └── internal/persistence/  # pgx repositories
├── web/
│   └── src/                   # React/Vite client
├── contracts/
│   ├── board.v1.json
│   ├── openapi.yaml
│   └── realtime.schema.json
├── db/
│   ├── migrations/
├── deploy/
│   ├── compose.yaml
│   └── Caddyfile
└── docs/
    ├── rules.md
    ├── architecture.md
    └── operations.md
```

### Data model

Create migrations for:

- `profiles`: immutable ID, unique username, timestamps.
- `sessions`: opaque token hash, profile ID, creation/expiry/activity timestamps.
- `saved_layouts`: owner profile, name, ruleset/board version, 25-piece JSON deployment.
- `rooms`: invite code, phase, host session, visibility mode, clock preset, spectator cap.
- `room_participants`: room, session/profile, role, seat, connection and ready state.
- `matches`: room, ruleset version, mode, clock preset, opening seat, outcome, timestamps.
- `match_seats`: match, seat, profile, team, elimination reason and turn.
- `match_events`: match and monotonically increasing sequence, event type, canonical JSONB payload.
- `match_snapshots`: match, sequence, canonical state JSONB.
- Aggregate profile statistics through indexed queries rather than mutable counters.

Canonical events and snapshots may contain hidden truth and must never be returned directly by live endpoints.

### Public interfaces

HTTP API:

```text
POST   /api/session
DELETE /api/session
GET    /api/me

POST   /api/rooms
GET    /api/rooms/{code}
POST   /api/rooms/{code}/join
POST   /api/rooms/{code}/leave       # planned; v1 client leaves by closing WebSocket

GET    /api/layouts
POST   /api/layouts
PUT    /api/layouts/{id}
DELETE /api/layouts/{id}

GET    /api/profiles/{username}
GET    /api/profiles/{username}/matches
GET    /api/matches/{id}/replay

GET    /healthz
GET    /readyz
GET    /metrics                      # planned
```

WebSocket endpoint:

```text
GET /api/rooms/{code}/ws
```

Every realtime envelope includes:

```text
type
requestId       # client commands
roomVersion     # monotonically increasing
payload
```

Client commands include seat selection, room settings, setup replacement, ready/unready, move, resign, draw offer/response, snapshot request, and rematch readiness.

Server messages include scoped snapshots, participant/phase updates, private deployment acknowledgements, turn deadlines, redacted move/combat events, elimination, draw state, match result, reconnect status, and structured errors.

- Deduplicate commands by session and `requestId`.
- Reconnect with the last received `roomVersion`; send missed public/scoped events when available, otherwise send a fresh scoped snapshot.
- Validate REST against OpenAPI and realtime payloads against JSON Schema.
- Generate TypeScript contract types; keep Go transport structs explicit and verify them with schema contract tests.
- Never use client-supplied player IDs, team IDs, current turns, combat outcomes, or clocks as authority.

## 3. UI and interaction design

### Visual system

Use a modern tactical command-table style:

- Charcoal/navy surfaces, restrained grid texture, and high-contrast status accents.
- Four distinct seat colors plus separate team indicators; never rely on color alone.
- Simplified Chinese throughout v1.
- Chinese two-character piece labels remain upright to the viewer when the board rotates.
- CSS custom properties define spacing, typography, colors, motion, and board dimensions.
- Respect `prefers-reduced-motion`; provide sound-effect and animation toggles.
- Use short movement/combat effects only, with no continuous decorative animation or background music.

### Board rendering

- Render roads, rails, camps, headquarters, and central geometry as responsive SVG.
- Render pieces and interaction targets as accessible DOM controls positioned from normalized board coordinates.
- Rotate the board so the active player's seat is at the bottom.
- Spectators start North-up and receive manual 90-degree rotation controls.
- Support click-select/click-destination, drag-and-drop, keyboard selection, Escape to cancel, and visible legal targets.
- Client-side legal-target previews are advisory; only server acceptance commits a move.
- Show selected piece, legal destinations, last move, current turn, deadline, disconnected seats, eliminations, revealed flags, and pending draw state.
- On narrow screens, use a fit-to-width/pannable board and bottom sheets for status/history; desktop remains the primary quality target.

### Screens

- Sign-in: username field, exact validation guidance, and explicit impersonation warning.
- Home: create room, join by code, recent matches, and profile summary.
- Lobby: seat map, teams, spectators, host marker, visibility mode, clock preset, and ready state.
- Setup: private board editor, saved-layout browser, validation feedback, reset/default, timer, and ready control.
- Game: central board, turn/team panels, event history, clock, resign and draw controls.
- Spectator: public-hidden board, participant status, rotation control, and no command controls.
- Result: outcome, elimination summary, rematch action, and replay link.
- Replay: timeline, step/play controls, speed, full-truth/seat-perspective selector, and board rotation.
- Profile: basic statistics and paginated match history.

Routes are `/`, `/room/:code`, `/profile/:username`, and `/replay/:matchId`.

## 4. Implementation phases

### Phase 0 — Rules and contracts

- [x] Write `docs/rules.md` with the executable v1 rules baseline, visibility modes, clocks, draws, and elimination behavior.
- [x] Create and manually verify the generated v1 board contract in `board.v1.json`.
- [x] Define engine commands, domain events, phases, seats, teams, visibility modes, and error codes.
- [x] Draft OpenAPI and realtime schemas before handlers.
- [ ] Produce low-fidelity wireframes for every screen and desktop/mobile game layouts.

Exit criteria: all legal movement examples, combat outcomes, visibility projections, and room transitions can be expressed without implementation-specific exceptions.

### Phase 1 — Foundation and deployment

- [x] Scaffold Go and React applications with dependency locking; strict linting and CI remain follow-up work.
- [x] Add PostgreSQL migrations, plain-SQL migration execution, local development Compose, Caddy routing, health checks, and a one-shot migration service.
- [x] Add structured JSON logging; request/room/match log correlation and Prometheus metrics remain follow-up work.
- [x] Serve the production Vite build from the Go service behind Caddy; runtime Compose contains Caddy, Go, PostgreSQL, and the migration job.

Exit criteria: one command starts the stack, migrations complete automatically, frontend reaches the API through Caddy, and health/readiness checks reflect PostgreSQL availability.

### Phase 2 — Pure game engine

- [x] Implement the v1 generated topology, deployment validation, legal movement, combat, turn order, timer handling, elimination, team victory, draws, and replayable events.
- [x] Implement canonical state plus four player projections and the public spectator projection.
- [x] Add persisted event reconstruction and ruleset/board version contracts; injected deterministic clocks and randomness remain follow-up work.
- [ ] Use injected clock and randomness interfaces for deterministic tests.

Exit criteria: the complete rules suite passes without importing HTTP, SQL, or WebSocket packages, and visibility tests prove no unauthorized rank is serialized.

### Phase 3 — Persistence and identity

- [x] Implement passwordless username sessions, profiles, saved-layout APIs, room/match records, event append, snapshots, recovery, replays, and statistics.
- [x] Enforce username and invite-code uniqueness at the database layer.
- [x] Store session tokens hashed; use `HttpOnly`, `Secure`, `SameSite=Lax` cookies.
- [x] Validate same-origin HTTP and WebSocket requests; bounded per-IP/session rate limits remain follow-up work.

Exit criteria: layouts and history survive restart, active matches reconstruct exactly, and database-write failure prevents state publication.

### Phase 4 — Realtime rooms

- [x] Implement the room registry, serialized per-room command handling, host transfer, seat selection, spectators, setup deadlines, turn deadlines, draw voting, resignations, rematch, reconnect, deduplication, and redacted fan-out.
- [x] Use scoped snapshots for each player and public snapshots for spectators.
- [x] Enforce the configured spectator cap and reject command attempts from spectators.

Exit criteria: four independent clients can complete a match; a fifth client can spectate without receiving hidden ranks; reconnect restores the correct scoped state.

### Phase 5 — Player frontend

- [x] Build the v1 sign-in, home, room lobby, setup editor, responsive board, clocks, system events, move controls, draw/resign flows, connection state, and error recovery.
- [ ] Add the saved-layout browser/editor, drag-and-drop setup, and the remaining narrow-screen board navigation refinements.
- [ ] Keep room state in Zustand with narrow selectors to avoid whole-board rerenders.
- [ ] Use TanStack Query only for durable HTTP resources and keep WebSocket state separate.
- [x] Add reduced-motion support and accessible board labels; keyboard behavior and narrow-screen board navigation remain follow-up work.

Exit criteria: all three visibility modes render correctly, player input remains responsive during events, and invalid or stale actions recover through server state rather than optimistic corruption.

### Phase 6 — Spectators, replays, and profiles

- [x] Implement public spectator mode, full postgame truth, basic timeline controls, profile statistics, match history, and same-room rematch/reseating.
- [ ] Add seat-perspective replay and richer replay controls.
- [x] Ensure live URLs never expose canonical events while completed replay endpoints intentionally permit full truth.

Exit criteria: full matches replay deterministically to the exact terminal state and live spectators cannot obtain hidden ranks through HTTP, WebSocket, page source, logs, or reconnect payloads.

### Phase 7 — Hardening and release

- [x] Add graceful restart/recovery, database backup guidance, security headers, and production Caddy/TLS guidance.
- [ ] Add retention settings, dependency scanning, container resource limits, and a tested restore drill.
- [x] Document upgrades, migrations, `pg_dump` backup, log inspection, and readiness checks; metrics and username deletion tooling remain.
- [ ] Load-test one room with four players plus 50 spectators and an aggregate of 25 active rooms/350 sockets.
- [ ] Publish a versioned rules reference in the UI.

Exit criteria: no race detector failures; p95 server command processing stays below 100 ms in the aggregate load test excluding network latency; restart recovers accepted moves without duplication or hidden-data leakage.

## 5. Test and acceptance plan

### Backend rules tests

- Table-driven tests for every deployment restriction and 25-piece inventory.
- Road, straight rail, curved rail, blocked rail, junction, and engineer-turn movement.
- Camp immunity, headquarters immobility, allied blocking, and illegal friendly attacks.
- Complete rank-versus-rank combat matrix, bombs, mines, engineers, flags, and commander reveal.
- Clock expiry, five cumulative misses, no-move elimination, resign, flag capture, piece removal, skipped turns, and team victory.
- Unanimous draw acceptance, rejection/cancellation, and the 70-move reset boundary.
- Deterministic replay and snapshot/event equivalence.
- Fuzz invariants: one piece per node, immutable piece identity, valid turn ownership, nonnegative inventories, and terminal states accepting no moves.
- Projection tests covering every role, mode, commander reveal, elimination, reconnect, and terminal replay.

### Service and persistence tests

- PostgreSQL integration tests against a real disposable PostgreSQL 18 instance.
- Migration-up tests from an empty database.
- Transaction failure, duplicate command, stale version, concurrent join, username collision, room-code collision, and event sequence tests.
- Restart recovery before/after snapshots and during overdue clocks.
- `go test -race` for room loops, connection registration, timer handling, and shutdown.
- HTTP/JSON Schema contract tests and malformed payload limits.
- Same-origin, spectator-command rejection, rate-limit, cookie, and canonical-state exposure tests.

### Frontend and end-to-end tests

- Vitest/Testing Library tests for stores, reducers, validation, orientation transforms, timers, legal-target presentation, setup editing, and scoped rendering.
- Accessibility tests for keyboard movement, focus order, labels, contrast, reduced motion, and dialog behavior.
- Playwright uses five isolated browser contexts: four players and one spectator.
- E2E scenarios cover all three visibility modes, seat contention, saved setup, timeout, combat, flag reveal, elimination, draw, reconnect, host transfer, rematch, replay, and restart recovery.
- Assert on captured network payloads that each context receives only authorized ranks.
- Verify current Chrome, Firefox, Safari/WebKit, and Edge-compatible Chromium; desktop is primary, mobile is functionally supported.

### Current v1 verification

- [x] Game-engine unit tests cover board construction, deployment, movement,
  combat, visibility, clocks, elimination, draws, and terminal flow.
- [x] HTTP and WebSocket integration tests cover session, room, scoped
  snapshots, spectator behavior, reconnect, and rematch behavior.
- [x] `go test -race ./...` and `go vet ./...` pass.
- [x] Frontend production build, Docker image build, Compose migrations,
  readiness, HTTPS SPA fallback, profile access, room recovery, and Caddy
  WebSocket upgrade were verified locally.
- [ ] PostgreSQL failure-injection, browser E2E, accessibility automation,
  load testing, fuzzing, and cross-browser acceptance remain outstanding.

### Explicit v1 exclusions and defaults

- No bots or AI.
- No public lobby, matchmaking, ratings, or leaderboard.
- No chat or reactions.
- No password/email recovery, ownership proof, or admin browser UI.
- No room passwords or admission approval.
- No Redis, multiple backend replicas, or distributed room ownership.
- No canvas/PixiJS renderer unless DOM/SVG profiling demonstrates a real need.
- No equal-priority mobile redesign; mobile remains responsive and usable.
- Store timestamps in UTC and display them in browser-local time.
- Use current patched releases within the selected major versions and commit dependency lockfiles.
