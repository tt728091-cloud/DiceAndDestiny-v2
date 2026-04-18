# Story 9: `game-session/startsession`

## Story Goal

Emit `session_start` once both seats are filled so each connected client receives its startup identity and the player seat map.

This story owns startup emission only.

## Scope

- detect when both seats are filled
- emit `session_start` to both clients
- keep the payload reconnect-ready
- add focused automated tests and contract tests

## Exact Contract

### Client 1 Example Payload

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

### Client 2 Example Payload

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

## Acceptance Criteria

- once both seats are filled, `session_start` is emitted
- both clients receive the event
- payload contains `session_id`, `match_id`, player seat map, `your_client_id`, `your_seat`, and `session_player_token`
- payload shape remains reconnect-ready
- focused automated tests pass

## Tests Required

- Go test for startup emission trigger
- contract test for `session_start`
- contract test for `session_joined` if the serializer boundary is shared

## Touched Paths

```text
v2/server/game-session/internal/app/startsession/
v2/server/game-session/internal/transport/jsonmsg/
v2/server/game-session/internal/transport/websocket/
```

## Rules And Constraints

- server only
- keep payload minimal; do not drag gameplay state into startup yet
- do not use `character_id` as the identity hook
- keep reconnect as a later epic while preserving the identity seams now

## Out Of Scope

- reconnect logic
- client consumption of `session_start`
- gameplay initialization beyond session bootstrap

## Assumptions For Now

- `match_id` is already available from earlier stories
- both websocket clients are already connected and seated

## Completion Proof

This story is complete when the second seated player triggers `session_start`, both clients receive the correct payload, and the focused tests pass.
