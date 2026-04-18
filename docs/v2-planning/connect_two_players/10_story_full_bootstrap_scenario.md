# Story 10: Full Bootstrap Scenario Proof

## Story Goal

Prove the entire `connect_two_players` epic end to end with a deterministic Go scenario test.

This story is the integration proof for the epic. It should use the real service boundaries for the first milestone flow.

## Scope

- boot testable `lobby`, `matchmaker`, and `game-session`
- register two clients
- request matches for both
- allow `matchmaker` to pair and assign them
- let both clients poll `lobby` for assignment
- connect both clients to `game-session`
- assert both receive `session_joined`
- assert both receive `session_start`
- add the deterministic scenario test

## Flow To Prove

1. client 1 registers with `lobby`
2. client 2 registers with `lobby`
3. client 1 requests match
4. client 2 requests match
5. `lobby` forwards both requests to `matchmaker`
6. `matchmaker` queues and pairs them
7. `matchmaker` assigns `match_id` and `session_id`
8. `matchmaker` provisions `game-session`
9. `matchmaker` notifies `lobby`
10. both clients poll `lobby` and receive assignment
11. both clients connect to `game-session`
12. both clients receive `session_joined`
13. both clients receive `session_start`

## Acceptance Criteria

- scenario uses deterministic Go test infrastructure
- all three services participate in the tested flow
- both clients end connected and seated
- both clients receive `session_joined`
- both clients receive `session_start`
- scenario test passes deterministically

## Tests Required

- one Go scenario/integration test for the full flow
- helper assertions for received HTTP and websocket messages as needed

## Touched Paths

```text
v2/server/.../internal/scenario/testkit/
v2/server/.../internal/scenario/connect_two_players/
```

## Rules And Constraints

- server only
- prefer Go scenario harness over shell scripts as the truth layer
- if possible, run services in-process or with test-managed wrappers to keep timing deterministic
- do not add client code to prove this story

## Out Of Scope

- Godot client integration
- reconnect scenario coverage
- gameplay beyond startup

## Assumptions For Now

- all earlier stories are already complete
- the scenario harness can use fake clocks, fake IDs, or other deterministic dependencies where needed

## Completion Proof

This story is complete when one deterministic Go scenario test proves the full bootstrap flow from registration through `session_start`.
