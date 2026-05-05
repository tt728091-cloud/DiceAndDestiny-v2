# Story 11G: Build Battle Setup From Run Player State

## Purpose

Create the bridge that turns saved/run player state into the `BattleSetup` / actor combat setup used to start a battle.

Story 11A should make battle creation accept prepared player combat setup. This story answers where that setup comes from in the real game flow.

## Design Context

The intended long-term chain is:

```text
saved/run player state
-> encounter setup layer
-> BattleSetup / ActorSetup
-> state.NewBattleFromSetup
-> battle authority state
```

The high-level architecture describes an encounter layer that:

- chooses enemies
- builds initial player combat state from run state
- builds initial enemy combat state
- defines win/loss conditions
- provides encounter rewards after victory

This story should convert initial player card zone setup, but only `Deck` is required for the first implementation. The shape should not block later support for starting cards in hand, discard, or removed piles. Do not build the full encounter engine yet.

## Before Coding, Read

- `README.md`
- `docs/v2-planning/godot_pve/00_high_level_architecture.md`
- `docs/v2-planning/godot_pve/01_portable_go_authority_architecture.md`
- `docs/v2-planning/godot_pve/03_story_testing_policy.md`
- `docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md`
- `docs/story-11-income-flow-card-draw-hook/todo.md`
- `stories/godot_pve/11a_battle_setup_player_combat_state.md`
- `dice-and-destiny-server/internal/battle/state/battle.go`
- any new setup files added by Story 11A

Assume Story 11A is implemented. If the current code differs, inspect what exists locally and adapt to the repo's actual state.

## Scope

- Add a minimal run/player combat setup source shape.
- Add a small setup/encounter-style package or state helper that converts that source into `state.BattleSetup`.
- Keep the required behavior deliberately narrow. It only needs enough information to provide the player's starting combat deck today.
- Shape the conversion so it can later carry full initial card zones without another conceptual rewrite.

Possible shape:

```go
type RunPlayerState struct {
	ActorID string
	Cards   RunCardZones
}

type RunCardZones struct {
	Deck    []string
	Hand    []string
	Discard []string
	Removed []string
}

func BattleSetupFromRunPlayer(player RunPlayerState) (state.BattleSetup, error)
```

For the first implementation, only `Cards.Deck` is required. `Hand`, `Discard`, and `Removed` may be omitted or copied through only if Story 11A/11B state shapes already support them. Alternative names are fine if they better match the repo's actual package layout.

## Out Of Scope

- Save file IO.
- Godot save system.
- Profile loading.
- Deckbuilder validation.
- Enemy setup.
- Encounter rewards.
- Map/campaign/run progression.
- Full encounter engine.
- Shuffling, discard reshuffle behavior, action points, dice, damage.
- Godot, C++, or UI changes.

## Requirements

- A run/player setup object can be converted into battle setup.
- The converted battle setup preserves actor ID and initial card zone setup.
- `Deck` is required for the first implementation.
- The conversion shape should not prevent later starting-hand support, such as a player beginning combat with specific cards already in hand.
- Deck slices are copied, not aliased.
- If `Hand`, `Discard`, or `Removed` are present and supported by the current state shape, they should also be copied, not aliased.
- Empty actor ID is rejected.
- Empty deck behavior is explicit. Prefer rejecting an empty deck unless there is already a local pattern that supports empty decks for tests.
- The conversion layer should not mutate `state.Battle` directly; it should produce setup input for battle creation.
- `state.NewBattleFromSetup` remains the battle state creation boundary.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add focused tests proving:

- run/player state with actor `player` and deck `["strike", "guard"]` produces battle setup with that actor and deck
- if current setup/state supports initial hand, run/player state can preserve starting hand cards in setup
- modifying the original run/player deck after conversion does not mutate the setup deck
- modifying any original input zone slice after conversion does not mutate setup card zones
- missing actor ID is rejected
- empty deck behavior is explicit
- the produced setup can be passed into `state.NewBattleFromSetup` and used by Story 11 income draw behavior

Do not add trivial getter/setter tests.

## Definition Of Done

- There is a clear bridge from run/player initial card zone state into battle setup.
- Battle setup remains the input to battle state creation.
- The story does not implement save loading or a full encounter engine.
- Story 11A behavior still works.
- `go test ./...` passes.
- No Godot, C++, or UI files are changed.
