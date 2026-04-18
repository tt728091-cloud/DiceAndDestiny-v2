# Story 5: `matchmaker/assignsession`

## Story Goal

Take a formed pair, assign a `session_id`, provision `game-session`, and notify `lobby` of both client assignments so polling can work.

This story owns session assignment, internal provisioning of `game-session`, and the internal callback to `lobby`.

## Scope

- assign a `session_id` to a formed match
- provision `game-session` with the new session
- create the assignment callback payload
- call `POST /v1/lobby/match-assignments`
- add focused automated tests

## Exact Provisioning Contract

### Request

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

### Response

`201 Created`

```json
{
  "status": "session_created",
  "session_id": "session-1",
  "match_id": "match-1"
}
```

## Exact Callback Contract

### Request

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

### Response

`202 Accepted`

```json
{
  "status": "recorded",
  "match_id": "match-1",
  "session_id": "session-1"
}
```

## Acceptance Criteria

- a formed pair receives a `session_id`
- `matchmaker` provisions `game-session` with the new session and allowed clients
- callback payload contains both client assignments
- callback payload contains the websocket target for the session
- `matchmaker` sends the callback to `lobby`
- focused automated tests pass

## Tests Required

- Go test for assignment payload creation
- Go test with a fake `game-session` provision sink verifying provisioning payload
- Go test with a fake `lobby` callback sink verifying callback contents
- Go test proving a `session_id` is attached to the formed match

## Touched Paths

```text
v2/server/matchmaker/internal/app/assignsession/
v2/server/matchmaker/internal/transport/http/
```

## Rules And Constraints

- server only
- do not implement polling lookup in this story
- provision `game-session` before notifying `lobby`
- keep session host selection behind a boundary so it can evolve later
- use a fake `lobby` callback sink in focused tests

## Out Of Scope

- lobby assignment storage
- game-session websocket handling after provisioning

## Assumptions For Now

- a single game-session host can be assumed for the first milestone
- the websocket target may be derived from fixed local config in this story

## Completion Proof

This story is complete when a formed pair can receive a `session_id`, `matchmaker` provisions `game-session`, `matchmaker` emits the correct callback to `lobby`, and the focused tests pass.
