# Story 5: Full Combat Sheet Battle Setup

## Purpose

Convert the loaded character combat sheet from Story 1 into battle setup and battle state so actors carry their real combat loadout, not only card zones.

This story is the bridge between content/save loading and authoritative battle state.

## Design Context

Story 1 loads an authored character combat sheet:

```text
character combat sheet YAML
-> loaded character combat sheet
```

This story makes that state usable by battle creation:

```text
loaded run/player combat state
-> BattleSetup / ActorSetup
-> state.NewBattleFromSetup
-> battle state with actor loadout
```

The engine still does not need to resolve cards, abilities, dice requirements, statuses, or tokens yet. The point is to preserve the data at the correct boundary so later stories can consume it.

## Before Coding, Read

- `docs/godot_pve_player_combat_flow/1_character_combat_sheet_yaml.md`
- `stories/godot_pve/11g_build_battle_setup_from_run_player_state.md`
- `dice-and-destiny-server/internal/battle/setup/run_player.go`
- `dice-and-destiny-server/internal/battle/state/battle.go`
- `dice-and-destiny-server/internal/battle/snapshot/snapshot.go`
- `dice-and-destiny-server/internal/battle/engine/engine.go`

## Scope

- Expand `state.ActorSetup` and `state.ActorState` to carry combat sheet data.
- Preserve existing card zone behavior.
- Add character metadata to actor state.
- Add resource metadata to actor state:
  - starting hand size
  - starting energy
  - current energy points
- Add decklist composition to actor state separately from ordered card zones.
- Add dice loadout to actor state.
- Add ability loadout to actor state.
- Add current statuses and tokens to actor state.
- Add a setup conversion function from the loaded character combat sheet into `state.BattleSetup`.
- Update snapshots only enough to prove the data is available without exposing hidden/private data incorrectly.

## Out Of Scope

- Card effect resolution.
- Ability selection or activation.
- Dice rolling.
- Dice requirement matching.
- Status effect behavior.
- Token behavior.
- Enemy AI.
- Godot UI.
- C++ bridge changes.

## Requirements

- Battle setup can be built from a loaded character combat sheet.
- Battle state preserves:
  - actor ID
  - character ID/name/class
  - decklist card ID/count entries
  - ordered card zones
  - starting hand size
  - starting energy
  - current energy points
  - dice loadout
  - offensive ability IDs
  - defensive ability IDs
  - statuses
  - tokens
- Decklist and ordered card zones remain separate.
- State constructors copy slices/maps and do not alias input data.
- Existing `state.NewBattle` and existing minimal setup tests still pass.
- The old JSON run-player setup path either keeps working as a narrow compatibility path or is adapted cleanly into the richer setup shape.
- Snapshot data should remain viewer-safe:
  - viewer can see their own hand card IDs
  - opponents should expose counts and public metadata only
  - dice and ability visibility should be explicit

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add focused tests proving:

- a loaded character combat sheet can create `state.BattleSetup`
- `state.NewBattleFromSetup` preserves the full actor loadout
- decklist and ordered deck both survive setup conversion
- dice loadout survives setup conversion
- ability loadout survives setup conversion
- resources survive setup conversion
- statuses and tokens survive setup conversion
- modifying input after setup conversion does not mutate battle setup
- modifying setup after battle creation does not mutate battle state
- existing card draw/income tests still pass

## Definition Of Done

- Battle creation can start from the character combat sheet data loaded in Story 1.
- The battle state has all player loadout data needed by later dice, ability, trigger, and segment stories.
- Existing minimal setup behavior remains compatible.
- `go test ./...` passes.
- No Godot, C++, or UI files are changed.
