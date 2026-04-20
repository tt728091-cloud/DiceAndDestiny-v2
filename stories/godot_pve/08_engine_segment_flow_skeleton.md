# Story 08: Engine Segment-Flow Skeleton

## Purpose

Create the Go engine layer that connects segment state to segment flow hooks.

The segment manager should stay focused on where the battle is in the loop. The engine should orchestrate what happens when entering or leaving a segment.

## Design Context

The target shape is:

```text
segment.Manager:
  calculates deterministic next segment

engine.Engine:
  calls current flow OnExit
  asks segment.Manager for next segment
  updates state.Battle
  calls next flow OnEnter

SegmentFlow:
  owns segment-specific flow decisions
```

Reference:

```text
docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md
```

## Scope

- Add `dice-and-destiny-server/internal/battle/engine/`.
- Define an `Engine` type.
- Define a `SegmentFlow` interface or equivalent.
- Define `Context`, `FlowResult`, and `FlowDecision` shapes.
- Add placeholder flows for:
  - `ongoing_effects`
  - `income`
  - `offensive`
  - `defensive`
  - `damage_resolution`
- Add an engine method that advances a `state.Battle` through one segment boundary.
- Keep the segment manager as the only code that calculates the next segment.

Possible flow shape:

```go
type SegmentFlow interface {
	ID() segment.Segment
	OnEnter(ctx *Context) (FlowResult, error)
	CanAdvance(ctx *Context) (FlowDecision, error)
	OnExit(ctx *Context) (FlowResult, error)
}
```

## Out Of Scope

- Real card draw.
- Real dice rolling.
- Damage resolution.
- Authority JSON refactor.
- Godot, C++, or UI changes.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add Go engine/domain tests proving:

- engine calls the current flow's `OnExit`
- engine calls `segment.Manager.Advance`
- engine updates `state.Battle.Segment`
- engine calls the next flow's `OnEnter`
- entering `income` resolves to `IncomeFlow`
- missing flow for a segment is an error
- invalid segment state returns an error
- the `segment` package still does not import card, dice, damage, enemy, Godot, or UI code

## Definition Of Done

- Engine package exists with segment flow skeleton.
- Placeholder flows are registered.
- Engine can advance from `ongoing_effects` into `income`.
- Engine tests pass.
- Existing segment and state tests pass.
- No Godot, C++, or UI files are changed.
