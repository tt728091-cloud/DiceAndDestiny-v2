# Story 11H: Load Player Run State From Save For Battle Setup

## Purpose

Load the player's saved/run deck state from a file and make it available for battle setup.

Story 11G assumes the run/player state is already loaded in memory. This story owns the earlier step:

```text
save file on disk
-> loaded run/player state
-> Story 11G converts loaded run/player state into BattleSetup
-> state.NewBattleFromSetup starts combat
```

## Why This Is Separate From Story 11G

These are different responsibilities:

```text
11H:
  file IO and save schema
  read player deck data from disk
  validate saved run/player state

11G:
  take already-loaded run/player state
  convert it into battle setup
  do not read files
```

Keeping them separate prevents save-file logic from leaking into battle state creation or card draw rules.

## Design Context

The long-term architecture expects a run/save layer outside a single battle.

The battle authority should not know how to read files. Combat should receive prepared setup data.

Recommended dependency direction:

```text
save/run loading layer
-> run/player state
-> setup/encounter bridge from Story 11G
-> state.BattleSetup
-> state.NewBattleFromSetup
-> battle authority state
```

Do not make `state.Battle`, `card.DrawCards`, `IncomeFlow`, or `segment` read save files.

## Before Coding, Read

- `README.md`
- `docs/v2-planning/godot_pve/00_high_level_architecture.md`
- `docs/v2-planning/godot_pve/01_portable_go_authority_architecture.md`
- `docs/v2-planning/godot_pve/03_story_testing_policy.md`
- `docs/story-11-income-flow-card-draw-hook/todo.md`
- `stories/godot_pve/11a_battle_setup_player_combat_state.md`
- `stories/godot_pve/11g_build_battle_setup_from_run_player_state.md`
- any setup/run package files added by Story 11G
- any existing save-related docs or code found locally

Assume Stories 11A and 11G are implemented. If the current code differs, inspect what exists locally and adapt to the repo's actual state.

## Scope

- Add a minimal saved player/run state file format for the deck needed to start combat.
- Add a loader that reads saved player/run state from a file path.
- The loaded shape should be compatible with the Story 11G conversion step.

Possible saved JSON shape:

```json
{
  "actor_id": "player",
  "deck": ["strike", "guard", "focus"]
}
```

Possible Go shape:

```go
type SavedRunPlayerState struct {
	ActorID string   `json:"actor_id"`
	Deck    []string `json:"deck"`
}

func LoadRunPlayerState(path string) (RunPlayerState, error)
```

Names and package placement may change to match the repo's actual layout.

## Out Of Scope

- Godot save UI.
- Save-slot selection UI.
- Binary save formats.
- Encryption/compression.
- Full campaign/run progression.
- Deckbuilder validation rules.
- Enemy setup.
- Encounter rewards.
- Starting the battle directly from the loader.
- Shuffling, discard, removed piles, action points, dice, damage.
- C++ bridge changes.

## Requirements

- A saved file can define the player's actor ID and deck.
- Loader returns an in-memory run/player state object, not a `state.Battle`.
- Loader validates:
  - file exists/read errors are reported clearly
  - JSON is valid
  - actor ID is present
  - deck is present
  - empty deck behavior is explicit, preferably rejected unless a reviewed reason allows it
- Loader copies deck data into the returned state.
- Loaded state can be passed into the Story 11G conversion function.
- Battle setup and battle authority remain independent of file IO.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add focused loader/setup tests proving:

- valid save file loads actor ID and deck
- invalid JSON is rejected
- missing actor ID is rejected
- missing deck is rejected
- empty deck behavior is explicit
- missing file/read error is reported
- loaded state can be converted by Story 11G into `state.BattleSetup`
- converted setup can start a battle via Story 11A's battle setup constructor

Use temporary test files/directories. Do not depend on a user-local real save file.

## Definition Of Done

- Player deck can be loaded from a saved/run state file into an in-memory run/player state object.
- Loaded state feeds the Story 11G setup conversion path.
- Battle authority and card draw code do not read files.
- `go test ./...` passes.
- No Godot, C++, or UI files are changed.

