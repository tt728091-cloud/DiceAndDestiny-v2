# Story 6: `lobby/lookupassignment`

## Story Goal

Create the assignment recording and polling lookup flow inside `lobby` so a client can poll for session connection details.

This story owns the callback receiver and the lookup endpoint.

## Scope

- add `POST /v1/lobby/match-assignments`
- record assignment state for both clients
- add `GET /v1/lobby/clients/{client_id}/assignment`
- return `searching` before assignment exists
- return `assigned` after callback is recorded
- add focused automated tests

## Exact Contracts

### Callback Request

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

### Lookup Request

`GET /v1/lobby/clients/{client_id}/assignment`

### Searching Response

```json
{
  "status": "searching"
}
```

### Assigned Response

```json
{
  "status": "assigned",
  "match_id": "match-1",
  "session_id": "session-1",
  "session_ws_url": "ws://localhost:8081/v1/game-session/ws/session-1"
}
```

## Acceptance Criteria

- `lobby` accepts the assignment callback
- `lobby` stores assignment state for both clients
- lookup returns `searching` before assignment
- lookup returns `assigned` with the correct payload after callback
- focused automated tests pass

## Tests Required

- Go test for assignment callback handling
- Go test for lookup before assignment
- Go test for lookup after assignment
- Go test for assigned response shape

## Touched Paths

```text
v2/server/lobby/internal/app/lookupassignment/
v2/server/lobby/internal/transport/http/
```

## Rules And Constraints

- server only
- do not implement websocket join behavior here
- keep assignment storage behind a clean boundary
- keep polling semantics simple for the first milestone

## Out Of Scope

- session seating
- reconnect
- client-side polling logic

## Assumptions For Now

- assignment state may be in-memory for the first milestone
- timeout and retention rules can stay minimal in this story

## Completion Proof

This story is complete when `lobby` can record assignment callbacks and return either `searching` or `assigned` deterministically with passing focused tests.
