# Story 07: Minimal Battle State

## Purpose

Create the minimum authoritative Go battle state that the engine can operate on.

The current segment tests use a local fake battle state. This story promotes that shape into production Go state so the upcoming engine/flow layer has real state to mutate.

## Design Context

Use the segment manager as the source of initial segment state:

```text
ongoing_effects, round 1
```

The state package should own mutable battle state. The segment package should still only own segment identity and deterministic advancement.

Reference:

```text
docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md
```

## Scope

- Add `dice-and-destiny-server/internal/battle/state/battle.go`.
- Define a minimal `Battle` type with battle ID and segment state.
- Add a constructor or initialization helper for new battles.
- Use `segment.Manager.InitialState()` instead of duplicating starting segment values.
- Keep this Go-only.

Possible shape:

```go
type Battle struct {
	ID      string
	Segment segment.State
}
```

## Out Of Scope

- Decks, hands, actors, resources, health, statuses, or queued damage.
- Card draw.
- Dice rolling.
- Authority command refactor.
- Godot, C++, or UI changes.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add focused Go tests proving:

- a new battle preserves the requested battle ID
- a new battle starts at `ongoing_effects`
- a new battle starts at round `1`
- battle state uses the segment package instead of duplicating segment literals
- invalid empty battle ID is rejected if validation is added in this story

## Definition Of Done

- Production `state.Battle` exists.
- Focused Go state tests pass.
- Existing segment manager tests still pass.
- No Godot, C++, or UI files are changed.
