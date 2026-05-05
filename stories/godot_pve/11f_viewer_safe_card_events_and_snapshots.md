# Story 11F: Viewer-Safe Card Events And Snapshots

## Purpose

Define how card information appears in events and snapshots without leaking hidden state.

This is especially important for future PvP, where player 1 must not see player 2's hand contents.

## Design Context

The responsibility split should be:

```text
state.Battle = full authoritative truth
event package = facts that happened
snapshot package = viewer-safe read model after facts happened
```

In PvP or hidden-information contexts, both event output and snapshot output may need viewer-aware filtering.

Example:

```text
player-1 sees own drawn card IDs
player-1 sees opponent drew N cards, not opponent card IDs
```

## Before Coding, Read

- `README.md`
- `docs/v2-planning/godot_pve/01_portable_go_authority_architecture.md`
- `docs/v2-planning/godot_pve/03_story_testing_policy.md`
- `docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md`
- `stories/godot_pve/11_income_flow_card_draw_hook.md`
- `stories/godot_pve/11a_battle_setup_player_combat_state.md`
- `dice-and-destiny-server/internal/battle/event/event.go`
- `dice-and-destiny-server/internal/battle/event/event_test.go`
- `dice-and-destiny-server/internal/battle/snapshot/snapshot.go`
- `dice-and-destiny-server/internal/battle/snapshot/snapshot_test.go`
- `dice-and-destiny-server/internal/battle/engine/command.go`
- `dice-and-destiny-server/internal/battle/authority.go`

Assume actor/player state exists from Story 11A. If current code differs, inspect locally.

## Scope

- Decide and implement the first viewer-safe card snapshot shape.
- Add a viewer identity input to snapshot building if needed, such as:

```go
snapshot.FromBattleForViewer(battle, viewerActorID)
```

- For own actor, snapshot may include hand card IDs.
- For opponent/other actor, snapshot should include counts or public info only.
- Decide whether `cards_drawn` event needs a viewer-safe public form now or whether that waits until multiplayer command/session work.
- Keep current PvE behavior passing.

## Out Of Scope

- Multiplayer transport/session implementation.
- Authentication.
- Network server.
- Full opponent visibility policy.
- UI rendering.
- Godot, C++, or UI changes.

## Requirements

- Snapshot package must not expose mutable `state.Battle` directly.
- Viewer-safe snapshot must not leak another actor's hand contents.
- Own actor card visibility and opponent card count visibility should be explicit in tests.
- If event filtering is added, tests must prove own draw shows card IDs and opponent draw hides card IDs.
- Authority remains a JSON boundary and does not make gameplay visibility decisions directly.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add or update snapshot/event tests proving:

- viewer sees own hand card IDs
- viewer sees opponent hand count, not opponent hand card IDs
- snapshot JSON shape is stable
- snapshot package remains free of Godot/C++/UI imports
- event visibility behavior is explicit if implemented

## Definition Of Done

- Card visibility has a viewer-safe snapshot shape.
- Hidden card state is not leaked through snapshots.
- Event visibility is either implemented or explicitly deferred with tests/docs explaining why.
- `go test ./...` passes.

