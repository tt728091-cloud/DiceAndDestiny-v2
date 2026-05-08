# Story 11D: Draw With Discard Reshuffle

## Purpose

Update card draw so a short or empty deck can reshuffle discard into deck, then continue drawing.

Story 11 explicitly drew only up to the number of cards currently in `Deck`. That is incomplete for the intended game rules.

## Design Context

This story depends on:

- Story 11B full card zones, especially `Discard`
- Story 11C deterministic shuffle support

Card package owns how draw and reshuffle work. `IncomeFlow` should still only call the draw rule; it should not implement deck/discard movement.

## Before Coding, Read

- `README.md`
- `docs/story-11-income-flow-card-draw-hook/todo.md`
- `docs/v2-planning/godot_pve/03_story_testing_policy.md`
- `docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md`
- `stories/godot_pve/11b_full_card_zone_state.md`
- `stories/godot_pve/11c_deterministic_deck_shuffle.md`
- `dice-and-destiny-server/internal/battle/card/draw.go`
- `dice-and-destiny-server/internal/battle/card/draw_test.go`
- `dice-and-destiny-server/internal/battle/state/battle.go`
- `dice-and-destiny-server/internal/battle/event/event.go`

Assume Stories 11B and 11C are implemented. If current code differs, inspect locally and preserve deterministic behavior.

## Scope

- Update card draw behavior:

```text
draw from deck
if deck reaches zero and more cards are still needed and discard has cards:
  shuffle discard into deck
  clear discard
  continue drawing
if deck and discard cannot satisfy requested count:
  draw as many as possible
  return explicit short/empty event data
```

Important ordering rule:

```text
Always draw every available card from the current deck before reshuffling discard.
Never merge discard into a non-empty deck.
Never shuffle remaining deck cards together with discard cards.
Only when deck is empty and the draw request still needs more cards should discard be shuffled into a new deck.
```

Example:

```text
request: draw 2
deck:    [deck-card-1]
discard: [discard-card-1, discard-card-2, discard-card-3]

step 1: draw deck-card-1 from deck
step 2: deck is now empty and 1 more card is needed
step 3: shuffle discard into a new deck
step 4: clear discard
step 5: draw 1 card from the shuffled deck
```

The discard pile must be shuffled before becoming the new deck. It must not be copied into the deck in discard order.

Required reshuffle scenarios:

```text
Scenario A: deck has some cards but not enough
request: draw 2
deck:    [deck-card-1]
discard: [discard-card-1, discard-card-2, discard-card-3]

expected:
  draw deck-card-1 first
  deck reaches zero
  shuffle discard into a new deck
  clear discard
  draw 1 more card from the shuffled deck
```

```text
Scenario B: deck is empty and discard has enough cards
request: draw 2
deck:    []
discard: [discard-card-1, discard-card-2, discard-card-3, discard-card-4]

expected:
  shuffle discard into a new deck
  clear discard
  draw 2 cards from the shuffled deck
  remaining shuffled cards stay in deck
```

```text
Scenario C: deck is empty and discard does not have enough cards
request: draw 5
deck:    []
discard: [discard-card-1, discard-card-2, discard-card-3, discard-card-4]

expected:
  shuffle discard into a new deck
  clear discard
  draw all 4 shuffled cards
  report explicit short/empty result for the 1 card that could not be drawn
```

These cases should prove both paths:

```text
non-empty deck becomes empty during a draw, then discard reshuffles
already-empty deck at draw start immediately reshuffles discard
```

- Add or update events if needed to make reshuffle visible, such as:

```text
discard_reshuffled
cards_drawn with requested_count/drawn_count/deck_empty
```

Keep event shape minimal and consistent with current event package patterns.

## Out Of Scope

- Card play into discard.
- Damage into removed.
- Income choice/resource model.
- Viewer privacy filtering.
- Godot, C++, or UI changes.

## Requirements

- Deck with enough cards draws without touching discard.
- Short deck with discard first draws all remaining deck cards, then reshuffles discard only after deck reaches zero, then continues drawing.
- Empty deck with discard reshuffles immediately and draws from the shuffled discard pile.
- Empty deck with enough discard cards completes the requested draw and leaves remaining shuffled cards in deck.
- Empty deck with too few discard cards draws what it can and reports an explicit short/empty result.
- Empty deck and empty discard draws zero and reports explicit result.
- Discard is empty after it is reshuffled into deck.
- Removed pile is never reshuffled.
- Tests must prove deterministic order after reshuffle.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add focused card tests proving:

- deck has enough cards
- deck short and discard has cards, including the edge case `draw 2` with `deck` containing 1 card and `discard` containing 3 cards
- discard is not merged into a non-empty deck before drawing existing deck cards
- deck empty and discard has enough cards, including the edge case `draw 2` with `deck` empty and `discard` containing 4 cards
- deck empty and discard has too few cards, including the edge case `draw 5` with `deck` empty and `discard` containing 4 cards
- deck empty and discard empty
- removed cards are not used for draw or reshuffle
- discard is shuffled before becoming the new deck, not copied into deck in discard order
- reshuffle order is deterministic under the chosen test shuffler/seed

Update engine/authority tests only if the card draw event shape changes.

## Definition Of Done

- Draw behavior handles short deck and discard reshuffle.
- Empty/short deck behavior is explicit and tested.
- Removed cards are never drawn.
- `IncomeFlow` still delegates to card draw rather than owning deck logic.
- `go test ./...` passes.
