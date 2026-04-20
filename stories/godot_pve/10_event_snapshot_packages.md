# Story 10: Event And Snapshot Packages

## Purpose

Move event and snapshot shapes out of local authority structs and into proper Go packages.

The engine should produce domain events and snapshot-ready state. Authority should package those into JSON without owning their gameplay meaning.

## Design Context

Events describe what happened.

Snapshots describe current read-only battle state after events have been applied.

Reference:

```text
docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md
```

## Scope

- Add `dice-and-destiny-server/internal/battle/event/event.go`.
- Add `dice-and-destiny-server/internal/battle/snapshot/snapshot.go`.
- Define typed event shapes needed for segment advancement.
- Define snapshot shape for battle ID, current segment, and round.
- Add snapshot builder from `state.Battle`.
- Update engine and authority tests to use the shared event/snapshot types.

Possible initial events:

```text
segment_advanced
segment_entered
```

Possible initial snapshot:

```json
{
  "battle_id": "battle-1",
  "segment": "income",
  "round": 1
}
```

## Out Of Scope

- Card-specific event payloads.
- Dice-specific event payloads.
- Damage-specific event payloads.
- Godot UI changes.
- C++ bridge changes.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add or update Go tests proving:

- segment advancement event includes the selected fields, such as `from`, `to`, `round`, and possibly `completed_turn`
- snapshot includes battle ID, current segment, and round
- authority JSON output remains stable
- event/snapshot packages do not import Godot, C++, or UI code

## Definition Of Done

- Event package owns domain event shapes.
- Snapshot package owns read-only battle snapshot shape.
- Authority no longer defines local event/snapshot structs if shared packages replace them.
- Go tests pass.
- No Godot, C++, or UI files are changed.
