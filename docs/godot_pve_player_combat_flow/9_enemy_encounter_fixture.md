# Story 9: Enemy Encounter Fixture

## Purpose

Add a minimal enemy actor fixture and encounter setup path so battles can contain the loaded player and at least one potential enemy.

The enemy does not need real AI yet. It should exist in battle state as an actor with combat data and readiness behavior.

## Design Context

The player character combat sheet creates one actor during battle setup. PvE battles need an authority-owned enemy actor as well:

```text
player character combat sheet
+ enemy fixture
-> encounter setup
-> BattleSetup
-> battle state with player and enemy
```

Enemies are not presentation clients. They live on the authority side and later submit commands through the same engine boundary as any other actor action.

For now, the enemy can be a simple placeholder such as `mock_goblin`.

## Before Coding, Read

- `docs/godot_pve_player_combat_flow/1_character_combat_sheet_yaml.md`
- `docs/godot_pve_player_combat_flow/5_full_combat_sheet_battle_setup.md`
- `docs/godot_pve_player_combat_flow/6_actor_segment_readiness_gates.md`
- `docs/v2-planning/godot_pve/00_high_level_architecture.md`
- `dice-and-destiny-server/internal/battle/enemy/README.md`
- `dice-and-destiny-server/internal/battle/setup/run_player.go`
- `dice-and-destiny-server/internal/battle/state/battle.go`

## Scope

- Add a minimal enemy content/fixture shape.
- Add a mocked enemy fixture, likely `mock_goblin`.
- Add encounter setup that combines:
  - loaded player character combat sheet
  - enemy actor fixture
  - battle ID
- Preserve both actors in battle setup/state.
- Add enemy readiness defaults.
- Add snapshot behavior that does not expose hidden enemy details unless explicitly public.

## Out Of Scope

- Enemy AI.
- Intent generation.
- Hidden/reveal combat behavior.
- Enemy card playing.
- Enemy dice rolling beyond placeholder data.
- Rewards.
- Map/run progression.
- Godot enemy UI.
- C++ bridge changes.

## Requirements

- An encounter setup function can build a battle setup with player and enemy actors.
- Enemy actor has:
  - actor ID
  - display name
  - class/type/category
  - resources
  - dice loadout or explicit empty/no-dice behavior
  - ability IDs or explicit empty/no-ability behavior
  - card zones or explicit empty/no-card behavior
  - statuses and tokens
- Enemy data is copied into battle state without aliasing fixture input.
- The player remains loaded from the character combat sheet path.
- The enemy can participate in segment readiness evaluation.
- If the enemy has no action in a segment, it can be ready or auto-passed.
- If a future enemy action is pending, the segment readiness model can wait on it.
- Snapshot visibility for enemy hand/deck/hidden data is explicit.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add focused tests proving:

- encounter setup creates both player and enemy actors
- player data still comes from loaded character combat sheet
- enemy fixture data is preserved in battle state
- mutating enemy fixture input does not mutate battle state
- snapshots include public enemy actor presence
- snapshots do not expose hidden enemy hand card IDs to the player
- readiness evaluation includes the enemy actor
- default inert enemy does not block `ongoing_effects` or `income`

## Definition Of Done

- A battle can be created with the mocked player and a mocked enemy.
- Enemy data has a clear fixture/setup path separate from player save loading.
- The enemy participates in global segment readiness rules.
- `go test ./...` passes.
- No Godot, C++, or UI files are changed.
