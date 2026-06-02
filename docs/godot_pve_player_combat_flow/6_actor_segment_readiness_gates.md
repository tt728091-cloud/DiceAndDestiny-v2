# Story 6: Actor Segment Readiness Gates

## Purpose

Add per-actor readiness gates so every global segment can decide whether each actor is ready, waiting, blocked, or automatically passed.

The battle should advance only when all actors required for the current segment are resolved.

## Design Context

Segments are global:

```text
ongoing_effects -> income -> offensive -> defensive -> damage_resolution
```

All combat participants are in the same segment at the same time. No actor advances independently.

However, actors can have different readiness states inside a segment:

```text
player is stunned -> auto_passed in offensive
enemy can attack -> waiting for enemy action or AI command
ongoing effect needs a choice -> blocked until resolved
default no-op ongoing effects -> ready immediately
```

Story 3 defines the general flow decision model. This story adds actor-level reasons behind the global decision.

## Before Coding, Read

- `docs/godot_pve_player_combat_flow/3_segment_flow_completion_policy.md`
- `docs/godot_pve_player_combat_flow/5_full_combat_sheet_battle_setup.md`
- `docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md`
- `dice-and-destiny-server/internal/battle/engine/flow.go`
- `dice-and-destiny-server/internal/battle/engine/engine.go`
- `dice-and-destiny-server/internal/battle/segment/manager.go`
- Reference only:
  - `/Users/daddymere/games/Dice-and-Destiny/docs/deep-dive-2025-10-12/04_SEGMENT_LIFECYCLE.md`

## Scope

- Add actor-level segment readiness state.
- Add a small vocabulary for actor segment readiness:
  - `ready`
  - `waiting_for_command`
  - `blocked`
  - `auto_passed`
- Add reason strings or reason codes for readiness decisions.
- Update segment flows to aggregate actor readiness into one flow decision.
- Add helper behavior for actors who cannot act in a segment.
- Add placeholder status checks for future statuses such as `stunned`.
- Keep readiness evaluation inside engine/flow/domain packages, not `segment.Manager`.

## Out Of Scope

- Full status effect implementation.
- Full enemy AI.
- Ability resolution.
- Damage resolution.
- Godot UI.
- C++ bridge changes.

## Requirements

- Each segment flow can evaluate every actor participating in the battle.
- A segment can auto-advance only when every required actor is resolved.
- If any actor is `waiting_for_command`, the global flow decision waits.
- If any actor is `blocked`, the global flow decision blocks.
- If an actor cannot act because of a status or rule, that actor can become `auto_passed`.
- `auto_passed` actors should emit an event or be visible in testable flow result data.
- The segment manager remains deterministic and unaware of why a segment advances.
- Readiness state should be serializable or convertible into snapshots/events later.
- Default placeholder flows should still be able to return `ready_to_advance` when no actors have pending work.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add focused tests proving:

- all actors ready means the flow can advance
- one actor waiting means the flow waits
- one actor blocked means the flow blocks
- an actor with a placeholder stunned state is auto-passed in offensive
- player auto-passed and enemy waiting means the segment waits
- player auto-passed and enemy auto-passed means the segment can advance
- segment manager does not import readiness, status, card, dice, or ability packages
- flow decision aggregation is deterministic

## Definition Of Done

- Actor-level segment readiness exists in code and tests.
- Global segment advancement depends on all relevant actor readiness decisions.
- Stunned/disabled style behavior has a placeholder path without implementing the full status system.
- `go test ./...` passes.
- No Godot, C++, or UI files are changed.
