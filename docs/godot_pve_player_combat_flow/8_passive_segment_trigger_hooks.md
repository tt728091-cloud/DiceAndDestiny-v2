# Story 8: Passive Segment Trigger Hooks

## Purpose

Add a trigger hook system for character, status, token, card, and ability effects that need to run at segment boundaries.

The first implementation should support no-op defaults and one test passive that proves a segment trigger can emit events and optionally stop progression.

## Design Context

Some characters have nothing to do during `ongoing_effects`, so the segment should auto-advance.

Other characters may have automatic passives:

```text
Artifice enters ongoing_effects
-> gains synth tokens
-> no player choice needed
-> continue
```

Some effects may require player resolution:

```text
ongoing_effects begins
-> triggered effect asks player to choose one card to discard
-> segment blocks/waits
-> player resolves choice
-> segment can continue
```

The old repo used broad callbacks and hooks around segment transitions. In v2, keep hooks inside the engine/domain layer and return explicit events/decisions.

## Before Coding, Read

- `docs/godot_pve_player_combat_flow/3_segment_flow_completion_policy.md`
- `docs/godot_pve_player_combat_flow/6_actor_segment_readiness_gates.md`
- `docs/godot_pve_player_combat_flow/7_advance_until_first_wait.md`
- `docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md`
- `dice-and-destiny-server/internal/battle/engine/flow.go`
- `dice-and-destiny-server/internal/battle/event/event.go`
- Reference only:
  - `/Users/daddymere/games/Dice-and-Destiny/docs/deep-dive-2025-10-12/10_CALLBACKS_AND_HOOKS.md`

## Scope

- Add trigger points for segment entry and exit.
- Start with a narrow set:
  - `on_enter_ongoing_effects`
  - `on_exit_ongoing_effects`
  - `on_enter_income`
  - `on_exit_income`
  - `on_enter_offensive`
  - `on_exit_offensive`
- Add a trigger result shape that can return:
  - events
  - actor readiness changes
  - wait/block decision
- Register no-op trigger handlers for default actors.
- Add one test-only passive handler, such as gaining a token on `on_enter_ongoing_effects`.
- Ensure trigger handlers do not know about Godot or transport concerns.

## Out Of Scope

- Full scripting language for effects.
- Full status system.
- Full token system.
- Artifice implementation beyond a test-style passive if useful.
- Card effect interpreter.
- UI prompts for blocked trigger choices.
- C++ bridge changes.

## Requirements

- Segment flows can invoke trigger hooks during enter/exit.
- Trigger hooks can emit events.
- Trigger hooks can mutate authoritative battle state through controlled domain helpers.
- Trigger hooks can say progression may continue.
- Trigger hooks can say progression must wait or block.
- No-op triggers are the default behavior.
- Trigger ordering is deterministic.
- Trigger errors reject the command/result cleanly.
- The segment manager remains unaware of triggers.
- Events from triggers are included in engine command results.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add focused tests proving:

- no-op triggers do not stop automatic advancement
- `on_enter_ongoing_effects` can emit a token-gained event
- trigger mutation updates battle state
- a trigger can force a wait/block decision
- trigger events are returned before the segment advances away from the triggering segment
- trigger ordering is deterministic for multiple actors
- trigger errors produce rejected engine results

## Definition Of Done

- Segment trigger hooks exist as engine/domain concepts.
- Default character sheets can have no-op trigger behavior.
- Future character-specific passives have a clear place to attach.
- Triggers can either allow auto-advance or stop progression for resolution.
- `go test ./...` passes.
- No Godot, C++, or UI files are changed.
