# Story 7: `game-session/provisionsession`

## Story Goal

Create the internal provisioning entry point for `game-session` so `matchmaker` can create a known session before websocket joins begin.

This story owns session bootstrap inside `game-session`. It does not handle websocket joins yet.

## Scope

- add `POST /v1/game-session/sessions` for internal service-to-service use
- create a session record for the provided `session_id` and `match_id`
- record the allowed `client_id` values for later join validation
- add focused automated tests

## Exact Contract

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

## Acceptance Criteria

- valid provisioning requests create a known session
- session record stores `session_id`, `match_id`, and allowed clients
- duplicate provisioning for the same session is rejected cleanly
- focused automated tests pass

## Tests Required

- Go test for provisioning request validation
- Go test proving the session record is created
- Go test proving allowed clients are stored
- Go test for duplicate provisioning rejection

## Touched Paths

```text
v2/server/game-session/internal/app/provisionsession/
v2/server/game-session/internal/domain/session/
v2/server/game-session/internal/transport/http/
```

## Rules And Constraints

- server only
- this is an internal service-to-service entry point, not a public client API
- keep allowed client storage explicit so websocket joins can validate against it later
- do not add seating or websocket behavior in this story

## Out Of Scope

- `join_session`
- seat assignment
- `session_start`

## Assumptions For Now

- internal HTTP is acceptable here because this is service bootstrap, not player-facing transport
- session storage may be in-memory for the first milestone if hidden behind a clean boundary

## Completion Proof

This story is complete when `game-session` can be provisioned with a known session and allowed clients, and the focused tests pass.
