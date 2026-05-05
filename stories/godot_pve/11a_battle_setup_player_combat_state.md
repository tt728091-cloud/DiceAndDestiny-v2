# Story 11A: Battle Setup Provides Player Combat State

## Purpose

Replace the Story 11 hardcoded player deck with explicit battle setup data.

This is the first follow-up to Story 11 because every later deck story needs a real source for the starting player deck and actor card state. Do not implement saved-game loading yet; this story only creates the in-memory setup shape that future saved/run state can feed.

## Design Context

Story 11 added the first income card-draw hook:

```text
engine -> IncomeFlow -> card.DrawCards -> state.Battle
```

The current placeholder battle state hardcodes:

```go
Deck: []string{"card-1", "card-2", "card-3"}
```

That should not remain the long-term battle setup path.

Near-term direction:

```text
state.Battle
  Actors map[string]ActorState

ActorState
  Cards CardZones
```

Even if only cards are used in this story, `ActorState` gives future income resources and dice a natural home.

## Before Coding, Read

- `README.md`
- `stories/godot_pve/11_income_flow_card_draw_hook.md`
- `docs/story-11-income-flow-card-draw-hook/todo.md`
- `docs/v2-planning/godot_pve/01_portable_go_authority_architecture.md`
- `docs/v2-planning/godot_pve/03_story_testing_policy.md`
- `docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md`
- `dice-and-destiny-server/internal/battle/state/battle.go`
- `dice-and-destiny-server/internal/battle/state/battle_test.go`
- `dice-and-destiny-server/internal/battle/card/draw.go`
- `dice-and-destiny-server/internal/battle/card/draw_test.go`
- `dice-and-destiny-server/internal/battle/engine/command.go`
- `dice-and-destiny-server/internal/battle/engine/engine_test.go`
- `dice-and-destiny-server/internal/battle/authority_test.go`

Assume Story 11 is implemented. If the current code differs, inspect what exists locally and adapt to the repo's actual state.

## Scope

- Add a minimal battle setup shape, such as:

```go
type BattleSetup struct {
	Actors []ActorSetup
}

type ActorSetup struct {
	ID   string
	Deck []string
}
```

- Add a minimal actor state shape, such as:

```go
type ActorState struct {
	Cards CardZones
}
```

- Move player card zones under actor state if the current code still uses `Battle.Cards map[string]CardZones`.
- Add a battle constructor that accepts setup data, such as:

```go
func NewBattleFromSetup(id string, setup BattleSetup) (Battle, error)
```

- Keep or adapt `NewBattle(id string)` only as a default/test convenience if needed by existing command tests.
- Ensure setup deck slices are copied into battle state so callers cannot mutate battle state by holding the original slice.
- Update `card.DrawCards` and engine tests to use actor card state if the state shape changes.

## Out Of Scope

- Saved player profile loading.
- Run state loading.
- Deckbuilding UI.
- Full player resources.
- Dice state.
- Discard piles.
- Removed piles.
- Shuffling.
- Discard reshuffle draw behavior.
- Godot, C++, or UI changes.

## Requirements

- Battle setup can provide an actor ID and starting deck.
- New battle state preserves the requested battle ID.
- New battle state starts at `ongoing_effects`, round `1`.
- Provided actor deck becomes authoritative card state.
- The hardcoded Story 11 deck is no longer the only way to create battle card state.
- `IncomeFlow` can still draw for the configured player actor used by current tests.
- Keep authority portable and transport-focused.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add or update focused state/card/engine tests proving:

- battle setup with actor `player` and deck `["strike", "guard"]` creates actor card state with that deck
- setup deck input is copied, not aliased
- missing actor ID is rejected
- empty battle ID is still rejected
- income draw still moves a card from the configured actor deck to hand
- command/authority tests still pass with whatever default setup path remains

Do not add trivial getter/setter tests.

## Definition Of Done

- Starting player deck can come from explicit battle setup data.
- Actor card state has a clear home for future card/resources/dice state.
- Story 11 card draw behavior still works.
- `go test ./...` passes.
- No Godot, C++, or UI files are changed.

