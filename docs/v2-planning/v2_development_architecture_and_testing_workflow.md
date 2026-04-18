# V2 Development Architecture and Testing Workflow

## 1. Purpose

This document turns the V2 development rules into a working design.

It explains:

- how the repo should be structured
- how server and client work should be sliced
- how deterministic tests should be organized
- how prompts should be refined into stories
- how the first V2 milestone should be built

This document is intentionally more practical than the rules doc.

## 2. Initial Runtime Topology

V2 begins with three server processes:

1. `lobby`
2. `matchmaker`
3. `game-session`

### Responsibility Split

#### `lobby`

- client entry point
- receive join/connect requests
- forward matchmaking requests
- return session connection details once assigned

#### `matchmaker`

- queue players
- pair compatible players
- assign them to a `game-session`
- remain thin early on

#### `game-session`

- authoritative gameplay runtime
- player seating
- session start
- gameplay state
- gameplay events
- later game capabilities such as draw-card, roll-dice, damage, status effects

### Transport Split

#### `lobby`

- HTTP + JSON

Reason:

- simple control-style behavior
- easy deterministic tests

#### `matchmaker`

- HTTP + JSON

Reason:

- queueing and assignment are request/response flows early on

#### `game-session`

- WebSocket + JSON

Reason:

- multiplayer gameplay is event-driven
- server must push state and events to connected clients

Internal bootstrap for the first milestone may also use HTTP + JSON so `matchmaker` can provision sessions before clients join.

## 3. Repository Structure

Recommended V2 structure:

```text
v2/
  server/
    lobby/
      cmd/
      internal/
        app/
          registerclient/
          requestmatch/
          lookupassignment/
        transport/
          http/
        scenario/
          register_client/
          request_match/
        support/
    matchmaker/
      cmd/
      internal/
        app/
          queuematchrequest/
          pairplayers/
          assignsession/
        transport/
          http/
        scenario/
          pair_players/
        support/
    game-session/
      cmd/
      internal/
        app/
          provisionsession/
          joinsession/
          seatplayers/
          startsession/
          drawcard/
          rolloffense/
        domain/
          client/
          player/
          session/
          seat/
          deck/
          hand/
          round/
          segment/
        transport/
          http/
          websocket/
          jsonmsg/
        scenario/
          testkit/
          connect_two_players/
          drawcard/
        support/
          clock/
          rng/
          ids/
  client/
    game/
      features/
        lobbyconnect/
        cardhand/
        drawcard/
      screens/
      transport/
      state/
      testkit/
  content/
  tools/
    content-admin/
    content-validate/
```

## 4. Why This Structure

This is a hybrid structure, not a pure technical-layer tree and not a pure capability-only tree.

### Benefits

- feature prompts can target narrow folders like `drawcard`
- shared concepts like `deck` and `player` stay centralized
- transport stays separate from game rules
- scenario tests have a stable home
- future growth does not force duplication of core concepts

### Why Not Pure Capability-Only

If every folder is feature-only, then features tend to re-own shared concepts:

- player state
- deck state
- session state
- protocol translation

That usually leads back to tangling.

## 5. Development Flow For A Capability

The normal flow for a gameplay capability is:

1. create or extend the domain rule
2. expose it through an application service
3. prove it with a deterministic server test
4. lock the protocol contract
5. consume it in client state
6. render it in client UI
7. add end-to-end proof where appropriate

### Important Constraint

Do not combine server and client implementation in the same story unless there is a special reason to do so.

## 6. Bootstrap Identity Model

The first milestone needs a clean answer to "who am I in this session?" without overloading gameplay setup data.

Use three separate identity values:

- `client_id`: stable bootstrap identity created by `lobby`
- `seat`: authoritative gameplay identity inside a `game-session`, such as `player-1` or `player-2`
- `session_player_token`: opaque reconnect token minted by `game-session` for future resume logic

`character_id` may travel through matchmaking and later session startup, but it is not the identity primitive. A client should know who it is in the game from `client_id`, `your_seat`, and `session_player_token`, not from character selection.

Reconnect is intentionally a later epic.

This first milestone should only make reconnect possible later. It should not implement reconnect flows yet.

That means the bootstrap design must preserve these seams:

- `game-session` owns seat identity instead of inferring identity from the current socket alone
- `session_player_token` is issued now, even if it is not consumed yet
- identity data is available in stable protocol messages, not only hidden in in-memory transport wiring
- seating and session start remain separate capabilities so reconnect can later re-enter seating/session ownership rules cleanly

## 7. Test Architecture

## 7.1 Server Test Layers

### Focused module tests

Use these for:

- rule behavior
- state mutation
- event creation
- edge cases

Examples:

- deck draw behavior
- seat assignment logic
- invalid join cases

### Scenario tests

Use these for:

- process-to-process or service-to-service flows
- connection lifecycle
- message delivery outcomes
- multi-step capability proofs

Examples:

- two clients join through lobby and matchmaker
- both connect to the same game-session
- both receive `session-start`

### Contract tests

Use these for:

- exact JSON shapes
- required fields
- event naming
- protocol compatibility between server and client

## 7.2 Client Test Layers

### Feature tests

Use these for:

- state update from a received protocol message
- rendering behavior for a feature widget or screen piece
- user interaction inside one client feature

### Fixture-driven protocol tests

Use canned protocol messages to drive client tests without needing a live server.

Example:

- feed `session-start` or `card-drawn` fixture into client transport/state
- assert state and UI outcome

## 8. Deterministic Scenario Harness Design

Server-side scenario tests should be built in Go and should be the source of truth.

Recommended shared harness area:

```text
server/game-session/internal/scenario/testkit/
```

Suggested harness capabilities:

- boot services in-process or test-controlled mode
- create fake or lightweight clients
- send HTTP join/match requests
- open websocket connections
- capture outbound JSON messages
- wait for specific events with time-bounded assertions
- expose helpers for seating, session assignment, and startup

### Why Go Scenario Tests Instead Of Shell As Truth

- better control over timing
- better websocket handling
- easier assertions
- reusable helpers
- less brittle output parsing

Shell scripts can still exist as debug helpers, but not as the primary proof layer.

## 9. First Milestone Contract: `connect_two_players`

The first milestone should use this bootstrap flow.

### 9.1 Lobby register client

Request:

`POST /v1/lobby/clients`

```json
{
  "display_name": "test-client-1"
}
```

Response:

`201 Created`

```json
{
  "client_id": "client-1",
  "status": "registered"
}
```

### 9.2 Lobby request match

Request:

`POST /v1/lobby/clients/{client_id}/match-requests`

```json
{
  "character_id": "test-character"
}
```

Response:

`202 Accepted`

```json
{
  "client_id": "client-1",
  "status": "searching"
}
```

### 9.3 Matchmaker queue request from lobby

This is internal service-to-service HTTP, not a public client endpoint.

Request:

`POST /v1/matchmaker/requests`

```json
{
  "client_id": "client-1",
  "character_id": "test-character"
}
```

Response when queued:

`202 Accepted`

```json
{
  "status": "queued",
  "client_id": "client-1"
}
```

Response when matched:

`202 Accepted`

```json
{
  "status": "matched",
  "match_id": "match-1",
  "session_id": "session-1"
}
```

### 9.4 Matchmaker assignment callback to lobby

When a pair is formed, `matchmaker` must tell `lobby` the assignment for both clients so `lobby` can answer polling lookups.

Request:

`POST /v1/lobby/match-assignments`

```json
{
  "match_id": "match-1",
  "session_id": "session-1",
  "assignments": [
    {
      "client_id": "client-1",
      "session_ws_url": "ws://localhost:8081/v1/game-session/ws/session-1"
    },
    {
      "client_id": "client-2",
      "session_ws_url": "ws://localhost:8081/v1/game-session/ws/session-1"
    }
  ]
}
```

Response:

`202 Accepted`

```json
{
  "status": "recorded",
  "match_id": "match-1",
  "session_id": "session-1"
}
```

### 9.5 Matchmaker provisioning callback to game-session

`matchmaker` must provision `game-session` before websocket joins begin.

Request:

`POST /v1/game-session/sessions`

```json
{
  "session_id": "session-1",
  "match_id": "match-1",
  "allowed_clients": [
    "client-1",
    "client-2"
  ]
}
```

Response:

`201 Created`

```json
{
  "status": "session_created",
  "session_id": "session-1",
  "match_id": "match-1"
}
```

### 9.6 Lobby assignment lookup

The first milestone uses polling.

Request:

`GET /v1/lobby/clients/{client_id}/assignment`

Response while still searching:

```json
{
  "status": "searching"
}
```

Response once assigned:

```json
{
  "status": "assigned",
  "match_id": "match-1",
  "session_id": "session-1",
  "session_ws_url": "ws://localhost:8081/v1/game-session/ws/session-1"
}
```

### 9.7 Game-session websocket join

Connection URL:

`GET /v1/game-session/ws/session-1`

First client message:

```json
{
  "type": "join_session",
  "session_id": "session-1",
  "client_id": "client-1"
}
```

Immediate join ack from server:

```json
{
  "type": "session_joined",
  "session_id": "session-1",
  "client_id": "client-1",
  "seat": "player-1",
  "players_connected": 1,
  "players_required": 2,
  "session_player_token": "session-player-token-1"
}
```

### 9.8 `session_start` payload

Server sends this to both connected clients after both seats are filled.

Payload sent to client 1:

```json
{
  "type": "session_start",
  "session_id": "session-1",
  "match_id": "match-1",
  "players": [
    {
      "seat": "player-1",
      "client_id": "client-1"
    },
    {
      "seat": "player-2",
      "client_id": "client-2"
    }
  ],
  "your_client_id": "client-1",
  "your_seat": "player-1",
  "session_player_token": "session-player-token-1"
}
```

Payload sent to client 2:

```json
{
  "type": "session_start",
  "session_id": "session-1",
  "match_id": "match-1",
  "players": [
    {
      "seat": "player-1",
      "client_id": "client-1"
    },
    {
      "seat": "player-2",
      "client_id": "client-2"
    }
  ],
  "your_client_id": "client-2",
  "your_seat": "player-2",
  "session_player_token": "session-player-token-2"
}
```

This payload is intentionally minimal. It proves connection, seating, and identity ownership without dragging early gameplay state into the bootstrap contract.

## 10. Prompt Refinement Workflow

The intended future workflow is:

1. user gives broad request
2. prompt-refinement skill reads the V2 rules doc
3. skill expands request into epic and stories
4. user reviews/refines the breakdown
5. implementation begins story by story

### Target output format for refinement

- `Epic`
- `Stories`
- `Acceptance Criteria`
- `Tests`
- `Touched Paths`
- `Open Questions`

## 11. Worked Example: `connect_two_players`

This is the first V2 milestone and the first worked example.

### Epic

`connect_two_players`

### Goal

Prove the server-side bootstrap path through:

- `lobby`
- `matchmaker`
- `game-session`

### Required proof

- both clients connect and are seated
- both clients receive a `session-start` event
- the bootstrap contract leaves room for a future reconnect epic without redesigning identity

### Implementation-Sized Story Breakdown

These stories are intentionally smaller than the earlier outline. Each one should produce one useful slice of behavior plus the test that proves it.

#### Story 1. `lobby/registerclient` creates client identities

Acceptance criteria:

- `POST /v1/lobby/clients` accepts a valid register request
- `lobby` creates and returns a new `client_id`
- invalid register requests are rejected cleanly
- focused automated tests pass

Tests:

- Go tests for request validation
- Go tests for generated `client_id` response shape

Touched paths:

- `server/lobby/internal/app/registerclient/...`
- `server/lobby/internal/transport/http/...`

Open questions:

- exact display name validation rules

#### Story 2. `matchmaker/queuematchrequest` accepts queue requests

Acceptance criteria:

- `POST /v1/matchmaker/requests` accepts a queue request from `lobby`
- unmatched requests are stored as queued
- queued response shape matches the contract
- focused automated tests pass

Tests:

- Go tests for queue request validation
- Go tests for queued response shape

Touched paths:

- `server/matchmaker/internal/app/queuematchrequest/...`
- `server/matchmaker/internal/transport/http/...`

Open questions:

- queue expiry policy for abandoned requests

#### Story 3. `lobby/requestmatch` forwards match requests to `matchmaker`

Acceptance criteria:

- `POST /v1/lobby/clients/{client_id}/match-requests` accepts a valid request
- `lobby` forwards the correct payload to `matchmaker`
- `lobby` returns `status=searching` to the client
- focused automated tests pass

Tests:

- Go tests for lobby request handling
- Go tests with a fake `matchmaker` client verifying forwarded payload

Touched paths:

- `server/lobby/internal/app/requestmatch/...`
- `server/lobby/internal/transport/http/...`

Open questions:

- whether `character_id` is required at first match request or can be optional later

#### Story 4. `matchmaker/pairplayers` pairs two queued clients

Acceptance criteria:

- two compatible queued requests are paired deterministically
- pairing creates a `match_id`
- focused automated tests pass

Tests:

- Go tests for queue pairing
- Go tests for deterministic first-match behavior

Touched paths:

- `server/matchmaker/internal/app/pairplayers/...`

Open questions:

- future compatibility rules beyond first-in-first-out

#### Story 5. `matchmaker/assignsession` creates session assignments and notifies `lobby`

Acceptance criteria:

- a formed pair receives a `session_id`
- `matchmaker` provisions `game-session` with the new session and allowed clients
- `matchmaker` calls `POST /v1/lobby/match-assignments`
- callback includes both client assignments and websocket target
- focused automated tests pass

Tests:

- Go tests for assignment payload creation
- Go tests with a fake `game-session` provision sink verifying provisioning payload
- Go tests with a fake `lobby` callback sink verifying callback contents

Touched paths:

- `server/matchmaker/internal/app/assignsession/...`
- `server/matchmaker/internal/transport/http/...`

Open questions:

- session host selection strategy if more than one game-session host exists later

#### Story 6. `lobby/lookupassignment` records and returns assignments

Acceptance criteria:

- `lobby` accepts `POST /v1/lobby/match-assignments`
- `lobby` records assignment state for both clients
- `GET /v1/lobby/clients/{client_id}/assignment` returns `searching` before assignment
- the same lookup returns `assigned` with `match_id`, `session_id`, and `session_ws_url` after callback
- focused automated tests pass

Tests:

- Go tests for assignment callback handling
- Go tests for pre-assignment and post-assignment lookup results

Touched paths:

- `server/lobby/internal/app/lookupassignment/...`
- `server/lobby/internal/transport/http/...`

Open questions:

- polling timeout and assignment retention rules

#### Story 7. `game-session/provisionsession` creates known sessions

Acceptance criteria:

- `game-session` accepts `POST /v1/game-session/sessions`
- session record stores `session_id`, `match_id`, and allowed clients
- duplicate provisioning is rejected cleanly
- focused automated tests pass

Tests:

- Go tests for provisioning request validation
- Go tests proving session creation and stored allowed clients
- Go tests for duplicate provisioning rejection

Touched paths:

- `server/game-session/internal/app/provisionsession/...`
- `server/game-session/internal/domain/session/...`
- `server/game-session/internal/transport/http/...`

Open questions:

- whether provisioning should later move to another internal transport

#### Story 8. `game-session/joinsession` and `seatplayers` establish player seats

Acceptance criteria:

- `join_session` accepts a valid `client_id` for the target `session_id`
- first and second joiners are assigned deterministic seats
- each accepted client receives `session_joined`
- `session_joined` includes `client_id`, `seat`, and `session_player_token`
- duplicate joins and overflow joins are rejected cleanly
- focused automated tests pass

Tests:

- Go tests for join validation
- Go tests for deterministic seat assignment
- Go tests for duplicate and overflow rejection

Touched paths:

- `server/game-session/internal/app/joinsession/...`
- `server/game-session/internal/app/seatplayers/...`
- `server/game-session/internal/domain/session/...`
- `server/game-session/internal/domain/seat/...`
- `server/game-session/internal/transport/websocket/...`

Open questions:

- session player token generation strategy

#### Story 9. `game-session/startsession` emits startup state

Acceptance criteria:

- once both seats are filled, `session_start` is emitted
- both clients receive the event
- event contains `session_id`, `match_id`, player seat map, `your_client_id`, `your_seat`, and `session_player_token`
- event shape is reconnect-ready even though reconnect behavior is not implemented in this epic
- focused automated tests pass

Tests:

- Go tests for event emission trigger
- contract tests for `session_joined` and `session_start` JSON

Touched paths:

- `server/game-session/internal/app/startsession/...`
- `server/game-session/internal/transport/jsonmsg/...`
- `server/game-session/internal/transport/websocket/...`

Open questions:

- whether `character_id` should be added later as setup data in a follow-up epic

#### Story 10. Scenario proof for full bootstrap

Acceptance criteria:

- scenario boots testable `lobby`, `matchmaker`, and `game-session`
- two test clients register with `lobby`
- both request a match
- `matchmaker` pairs them, provisions `game-session`, and notifies `lobby`
- both clients poll assignment, connect to `game-session`, receive `session_joined`, and receive `session_start`
- scenario test passes deterministically

Tests:

- Go scenario/integration test

Touched paths:

- `server/.../internal/scenario/testkit/...`
- `server/.../internal/scenario/connect_two_players/...`

Open questions:

- whether services run in-process for tests or via test-managed boot wrappers

## 12. Future Example Shape: `draw_card`

This is not the first milestone, but it illustrates the workflow.

### Epic

`draw_card`

### Expected sequencing

1. deck draw rule
2. draw-card application service
3. server-side draw-card scenario proof
4. draw-card protocol message contract
5. client-side hand state update
6. client-side card rendering
7. end-to-end verification

### Example refined output

#### Story 1. Server draw-card rule

Acceptance criteria:

- top card moves from deck to hand
- empty-deck case is handled
- deterministic tests pass

#### Story 2. Server draw-card scenario

Acceptance criteria:

- from a valid session state, draw-card flow is triggered
- correct event is emitted to intended client(s)
- deterministic scenario test passes

#### Story 3. Draw-card protocol contract

Acceptance criteria:

- JSON message shape is locked
- contract tests pass

#### Story 4. Client hand-state update

Acceptance criteria:

- draw-card message updates local hand state
- client-side tests pass

#### Story 5. Client card rendering

Acceptance criteria:

- drawn card appears correctly in hand UI
- client-side tests pass

## 13. Content Tooling Placement

Content tooling should stay in the same repo for now, but outside runtime services.

Reason:

- shares schemas and validation with the runtime codebase
- easier iteration early on
- avoids premature repo splitting

Recommended placement:

```text
v2/tools/content-admin/
v2/tools/content-validate/
v2/content/
```

## 14. V1 Reuse Policy In Practice

For `lobby` and `matchmaker`, V1 code may be reviewed for opportunistic reuse.

Reuse decision rule:

- keep it if it is already thin, acceptable, and adaptable
- move it into V2 structure
- add V2 tests
- rewrite any part that violates V2 boundaries or testing rules

## 15. Relationship Between This Doc And The Rules Doc

Use the rules doc when:

- shaping prompts
- deciding if a story is too large
- checking definition of done
- deciding whether work should be server-only or client-only

Use this working design doc when:

- planning the V2 folder layout
- setting up the first milestone
- designing scenario harnesses
- writing the first epic/story breakdowns
- creating examples for prompt refinement
