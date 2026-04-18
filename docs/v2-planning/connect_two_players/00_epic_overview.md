# Epic: `connect_two_players`

## Purpose

This epic proves the first server-only V2 bootstrap path:

1. a client registers with `lobby`
2. the client requests a match through `lobby`
3. `lobby` forwards the request to `matchmaker`
4. `matchmaker` queues and pairs two clients
5. `matchmaker` assigns a `game-session`
6. `matchmaker` provisions `game-session` with the new session
7. `matchmaker` notifies `lobby` of both assignments
8. each client polls `lobby` for assignment details
9. each client connects to `game-session` over websocket
10. `game-session` seats both clients
11. both clients receive `session_joined`
12. both clients receive `session_start`

## What This Epic Must Prove

- V2 can run with three server processes from day one:
  - `lobby`
  - `matchmaker`
  - `game-session`
- the bootstrap path is deterministic and testable
- identity is cleanly separated:
  - `client_id` from `lobby`
  - `seat` from `game-session`
  - `session_player_token` from `game-session`
- the design is reconnect-ready without implementing reconnect yet

## Final Success Condition

A deterministic Go scenario test proves that:

- two test clients register with `lobby`
- both request a match
- `matchmaker` pairs them
- `lobby` exposes assignment details for both
- both clients connect to `game-session`
- both clients are seated as `player-1` and `player-2`
- both clients receive `session_joined`
- both clients receive `session_start`

## Contract Summary

### Lobby register

`POST /v1/lobby/clients`

Request:

```json
{
  "display_name": "test-client-1"
}
```

Response:

```json
{
  "client_id": "client-1",
  "status": "registered"
}
```

### Lobby request match

`POST /v1/lobby/clients/{client_id}/match-requests`

Request:

```json
{
  "character_id": "test-character"
}
```

Response:

```json
{
  "client_id": "client-1",
  "status": "searching"
}
```

### Matchmaker queue request

`POST /v1/matchmaker/requests`

Request:

```json
{
  "client_id": "client-1",
  "character_id": "test-character"
}
```

### Matchmaker callback to lobby

`POST /v1/lobby/match-assignments`

Request:

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

### Matchmaker provisioning callback to game-session

`POST /v1/game-session/sessions`

Request:

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

```json
{
  "status": "session_created",
  "session_id": "session-1",
  "match_id": "match-1"
}
```

### Lobby assignment lookup

`GET /v1/lobby/clients/{client_id}/assignment`

Searching response:

```json
{
  "status": "searching"
}
```

Assigned response:

```json
{
  "status": "assigned",
  "match_id": "match-1",
  "session_id": "session-1",
  "session_ws_url": "ws://localhost:8081/v1/game-session/ws/session-1"
}
```

### Game-session websocket join

Client message:

```json
{
  "type": "join_session",
  "session_id": "session-1",
  "client_id": "client-1"
}
```

Server response:

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

### Session start

Client-specific payload example:

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

## Story Order

1. `lobby/registerclient`
2. `matchmaker/queuematchrequest`
3. `lobby/requestmatch`
4. `matchmaker/pairplayers`
5. `matchmaker/assignsession`
6. `lobby/lookupassignment`
7. `game-session/provisionsession`
8. `game-session/joinsession` and `seatplayers`
9. `game-session/startsession`
10. full bootstrap scenario proof

## Global Rules For Every Story In This Epic

- server only; do not touch client code
- each story includes the code and the focused tests for that slice
- prefer meaningful code comments over extra implementation docs
- V1 code may be reused only if it fits V2 boundaries and tests
- do not couple player identity to the active websocket connection alone
- keep reconnect as a later epic, but do not block it architecturally

## Shared Touched Areas

```text
v2/server/lobby/
v2/server/matchmaker/
v2/server/game-session/
```

## Out Of Scope

- reconnect behavior
- client-side bootstrap
- gameplay state beyond session startup
- character payload expansion beyond `character_id` as matchmaking input
