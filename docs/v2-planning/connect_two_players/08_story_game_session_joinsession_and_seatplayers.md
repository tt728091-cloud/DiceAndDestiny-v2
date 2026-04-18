# Story 8: `game-session/joinsession` and `seatplayers`

## Story Goal

Create the websocket join and seating flow for `game-session` so two clients can join the same session and receive deterministic seats.

This story owns join validation, seat assignment, and `session_joined`.

## Scope

- accept websocket joins for a known `session_id`
- process `join_session`
- assign deterministic seats to the first and second joiners
- emit `session_joined`
- reject duplicate joins and overflow joins
- add focused automated tests

## Exact Contract

### Client Join Message

```json
{
  "type": "join_session",
  "session_id": "session-1",
  "client_id": "client-1"
}
```

### Server Join Ack

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

## Acceptance Criteria

- valid `join_session` messages are accepted for the target session
- first and second joiners receive deterministic seats
- accepted clients receive `session_joined`
- `session_joined` includes `client_id`, `seat`, and `session_player_token`
- duplicate joins are rejected cleanly
- overflow joins are rejected cleanly
- focused automated tests pass

## Tests Required

- Go test for join validation
- Go test for deterministic seat assignment
- Go test for `session_joined` payload shape
- Go test for duplicate join rejection
- Go test for overflow join rejection

## Touched Paths

```text
v2/server/game-session/internal/app/joinsession/
v2/server/game-session/internal/app/seatplayers/
v2/server/game-session/internal/domain/session/
v2/server/game-session/internal/domain/seat/
v2/server/game-session/internal/transport/websocket/
```

## Rules And Constraints

- server only
- do not emit `session_start` in this story
- do not tie identity to the current socket alone
- issue `session_player_token` now even though reconnect is not implemented yet
- keep seat assignment separate from transport plumbing so reconnect can be added later

## Out Of Scope

- `session_start`
- full scenario proof
- reconnect behavior

## Assumptions For Now

- `session_id` and allowed clients were already provisioned by Story 7
- `client_id` has already come through `lobby`
- token generation can be simple if the boundary is clean and testable

## Completion Proof

This story is complete when two valid clients can join one session, receive deterministic seats and `session_joined`, invalid joins are rejected, and the focused tests pass.
