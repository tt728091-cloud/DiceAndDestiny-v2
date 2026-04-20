# Story 11: Income Flow Card-Draw Hook

## Purpose

Implement the first real segment behavior hook: entering income triggers card draw.

The income flow owns when card draw happens in the battle loop. The card package owns how card draw works.

## Design Context

The dependency direction should be:

```text
engine -> IncomeFlow -> card.DrawCards -> state.Battle
```

The dependency direction should not be:

```text
segment -> card
```

Reference:

```text
docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md
```

## Scope

- Add the minimum card state needed to draw cards.
- Add or expand `dice-and-destiny-server/internal/battle/card/`.
- Implement deterministic card draw from deck to hand.
- Update `IncomeFlow.OnEnter` so it calls the card draw rule.
- Emit a card draw event from the engine/domain path.
- Keep the segment package free of card imports.

Possible card rule shape:

```go
func DrawCards(battle *state.Battle, actorID string, count int) ([]event.Event, error)
```

## Out Of Scope

- Full deckbuilding.
- Shuffling, unless required by the simplest card draw implementation.
- Card effects.
- Discard piles, unless needed for explicit empty-deck behavior.
- Dice rolling.
- Damage.
- Godot, C++, or UI changes.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add focused card tests proving:

- drawing moves cards from deck to hand
- draw order is deterministic
- empty deck behavior is explicit

Add engine/domain tests proving:

- advancing into income calls `IncomeFlow.OnEnter`
- `IncomeFlow.OnEnter` calls card draw behavior
- card draw events are returned through the engine
- battle state reflects the drawn cards
- `segment` package still does not import card code

## Definition Of Done

- Entering income can draw cards through engine flow.
- Card mechanics live in the card package.
- Segment package remains loop-position only.
- Go tests pass.
- No Godot, C++, or UI files are changed.
