# Story 11C: Deterministic Deck Shuffle

## Purpose

Add deterministic deck shuffle support so draw order is meaningful and testable.

Story 11 currently draws from whatever order is already in `Deck`. That is acceptable only if the deck has already been deterministically shuffled during battle setup or discard reshuffle.

## Design Context

Randomness in battle authority must be deterministic and testable. Do not use uncontrolled global randomness.

The card package should own deck shuffle mechanics. Battle setup or income/draw flow should decide when shuffle happens.

Near-term dependency direction should remain:

```text
engine/setup -> card shuffle/draw rules -> state.Battle
```

Do not make `segment` import card or randomness.

## Before Coding, Read

- `README.md`
- `docs/story-11-income-flow-card-draw-hook/todo.md`
- `docs/v2-planning/godot_pve/03_story_testing_policy.md`
- `docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md`
- `stories/godot_pve/11a_battle_setup_player_combat_state.md`
- `stories/godot_pve/11b_full_card_zone_state.md`
- `dice-and-destiny-server/internal/battle/card/draw.go`
- `dice-and-destiny-server/internal/battle/card/draw_test.go`
- `dice-and-destiny-server/internal/battle/state/battle.go`

Assume Stories 11A and 11B are implemented. If current code differs, inspect locally and preserve the dependency boundaries.

## Scope

- Add deterministic shuffle support in the card package.
- Choose a testable shape, such as:

```go
type Shuffler interface {
	Shuffle(cards []string)
}
```

or a deterministic function that accepts a seed/source.

- Add tests using a fake shuffler or fixed seed.
- Decide the first place shuffle is applied:
  - preferred: battle setup can shuffle the provided deck when requested
  - defer discard reshuffle use to Story 11D

## Out Of Scope

- Discard reshuffle draw behavior.
- Card play.
- Damage/removal.
- Deckbuilding.
- Saved player/run loading.
- Godot, C++, or UI changes.

## Requirements

- Shuffle behavior is deterministic in tests.
- Shuffle does not lose or duplicate cards.
- Draw still consumes from the top/front of the shuffled deck.
- Tests should not depend on uncontrolled randomness.
- The public card draw rule remains understandable.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add focused card/setup tests proving:

- shuffle reorders cards deterministically with a fake shuffler or fixed seed
- shuffle preserves the same card IDs
- draw after shuffle draws from the shuffled order
- no global uncontrolled randomness is used by tests

## Definition Of Done

- Deterministic shuffle support exists in card/setup code.
- Tests prove stable shuffled order.
- Draw order is clearly based on current deck order after shuffle.
- `go test ./...` passes.

