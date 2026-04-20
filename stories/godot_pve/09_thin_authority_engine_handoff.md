# Story 09: Thin Authority Engine Handoff

## Purpose

Refactor `authority.go` so it stops owning gameplay action logic and delegates command handling to the Go engine.

The current `roll_dice` behavior in `authority.go` is spike code. This story turns authority into a JSON boundary around the engine.

## Design Context

Authority should mostly:

```text
receive command JSON
decode transport-level command envelope
pass command into engine/flow
receive engine result
encode result JSON
return it
```

Authority may reject malformed JSON. It should not know gameplay details such as whether `roll_dice` is legal in `offensive`.

Reference:

```text
docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md
```

## Scope

- Add or expand `dice-and-destiny-server/internal/battle/command/`.
- Move command envelope parsing out of gameplay action logic.
- Update `battle.HandleCommand` so it delegates structurally valid commands to the engine.
- Add a temporary `advance_segment` command only if needed to prove engine handoff.
- Remove hard-coded gameplay checks from `authority.go`.
- Preserve portable Go authority boundaries.

## Out Of Scope

- Real card draw.
- Real dice rolling.
- Damage resolution.
- C++ bridge changes.
- Godot headless contract update, unless this story explicitly changes the stable boundary and cannot be tested otherwise.
- UI changes.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Update Go authority tests proving:

- invalid JSON is rejected by the authority boundary
- valid command JSON reaches the engine
- unsupported command types are rejected by command/engine code, not hard-coded gameplay checks in authority
- `advance_segment` returns expected event and snapshot shape if that command is added
- `authority.go` does not contain `roll_dice` gameplay validation

## Definition Of Done

- `authority.go` is a thin JSON command/result boundary.
- Gameplay command meaning lives outside `authority.go`.
- Go authority tests pass.
- Engine tests pass.
- No C++ bridge changes are made.
- No UI changes are made.
