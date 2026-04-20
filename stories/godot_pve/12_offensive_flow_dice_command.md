# Story 12: Offensive Flow Dice Command

## Purpose

Move dice rolling into the offensive segment flow instead of keeping spike dice behavior in `authority.go`.

The offensive flow owns whether a dice command is legal during offensive. The dice package owns how dice are rolled.

## Design Context

The target command path is:

```text
authority JSON boundary
-> command parser
-> engine
-> current segment flow
-> dice package
-> events + snapshot
```

`authority.go` should not know that `roll_dice` belongs to offensive.

Reference:

```text
docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md
```

## Scope

- Add real command handling path for `roll_dice`.
- Route commands through the engine to the current segment flow.
- Let `OffensiveFlow` accept or reject `roll_dice`.
- Let the dice package own dice rolling mechanics.
- Use deterministic dice behavior in tests.
- Emit dice rolled events from the engine/domain path.
- Remove remaining spike dice behavior from `authority.go` if it still exists.

## Out Of Scope

- UI dice rendering.
- Godot input changes.
- C++ bridge changes.
- Complex dice abilities.
- Damage from dice results.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add focused dice tests proving:

- dice rolling can be deterministic in tests
- dice results stay within allowed symbols or values

Add engine/domain tests proving:

- `roll_dice` is accepted during offensive
- `roll_dice` is rejected outside offensive
- dice rolled event is emitted through the engine
- authority JSON test proves the command/result contract without putting gameplay validation in `authority.go`

## Definition Of Done

- Offensive dice command is handled by engine/flow/domain packages.
- `authority.go` does not contain dice gameplay logic.
- Go tests pass.
- No C++ bridge changes are made.
- No UI changes are made.
