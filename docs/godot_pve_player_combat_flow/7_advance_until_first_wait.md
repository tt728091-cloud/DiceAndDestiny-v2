# Story 7: Advance Until First Wait

## Purpose

Add an engine command that advances automatic segment work until the battle reaches a segment that must wait for player, enemy, or triggered-effect resolution.

For the default mocked player, battle startup should run through `ongoing_effects` and `income`, then stop at `offensive` because the actor can normally roll dice or play cards.

## Design Context

The constructor should create battle state. It should not secretly play through the turn.

Instead, the engine should expose an explicit command such as:

```text
advance_until_wait
```

or:

```text
start_turn
```

That command can repeatedly ask the current flow whether it should advance. The loop stops when a flow returns `wait_for_command` or `blocked`.

Expected default path:

```text
ongoing_effects
-> no pending effects
-> income
-> draw starting hand / gain starting energy
-> offensive
-> wait for player command
```

## Before Coding, Read

- `docs/godot_pve_player_combat_flow/3_segment_flow_completion_policy.md`
- `docs/godot_pve_player_combat_flow/5_full_combat_sheet_battle_setup.md`
- `docs/godot_pve_player_combat_flow/6_actor_segment_readiness_gates.md`
- `dice-and-destiny-server/internal/battle/engine/command.go`
- `dice-and-destiny-server/internal/battle/engine/engine.go`
- `dice-and-destiny-server/internal/battle/engine/income_flow.go`
- `dice-and-destiny-server/internal/battle/income/reward.go`
- `dice-and-destiny-server/internal/battle/card/draw.go`

## Scope

- Add a command for advancing until the next wait/block decision.
- Add command parsing payload for the new command.
- Use the flow decision policy from Story 3.
- Use actor readiness gates from Story 6 if available.
- Add an explicit max-auto-advance guard.
- Make income use the actor's starting hand size and starting energy for battle startup if this is the first income of the battle.
- Stop at offensive by default when the player can act.
- Return all emitted events and a final snapshot.

## Out Of Scope

- UI controls.
- C++ bridge changes.
- Ability selection.
- Dice rolling.
- Status effect resolution beyond placeholder readiness checks.
- Enemy AI beyond placeholder readiness.

## Requirements

- The command is explicit; `state.NewBattleFromSetup` does not auto-run segments.
- The command advances through automatic ready segments.
- The command stops when the current segment waits or blocks.
- The command includes an infinite-loop guard.
- On round 1 income, the default mocked player draws `starting_hand_size` cards from the combat sheet.
- On round 1 income, the default mocked player gains `starting_energy_points` from the combat sheet.
- For the default mocked player, the final segment after command execution is `offensive`.
- The final result includes events for segment advancement, cards drawn, and energy gained.
- The final snapshot includes updated hand/deck counts and energy.
- Re-running the command while already waiting in offensive should not advance unless the offensive segment has been resolved.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add focused tests proving:

- battle starts in `ongoing_effects`
- `advance_until_wait` advances to `offensive` for the default actor
- round 1 income draws `starting_hand_size`
- round 1 income gains `starting_energy_points`
- events are returned in deterministic order
- final snapshot reflects drawn cards and energy
- command stops at wait decisions
- command stops at blocked decisions
- command refuses or safely handles excessive auto-advance loops
- command does not mutate state after a rejected payload

## Definition Of Done

- There is an explicit command to run automatic segment work until the first required wait/block.
- The default mocked player reaches offensive with 4 cards and 2 energy.
- Automatic progression remains owned by the engine, not by constructors or the segment manager.
- `go test ./...` passes.
- No Godot, C++, or UI files are changed.
