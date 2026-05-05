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
if deck cannot satisfy requested count and discard has cards:
  shuffle discard into deck
  clear discard
  continue drawing
if deck and discard cannot satisfy requested count:
  draw as many as possible
  return explicit short/empty event data
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
- Short deck with discard uses deterministic reshuffle and continues drawing.
- Empty deck with discard reshuffles and draws.
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
- deck short and discard has cards
- deck empty and discard has cards
- deck empty and discard empty
- removed cards are not used for draw or reshuffle
- reshuffle order is deterministic under the chosen test shuffler/seed

Update engine/authority tests only if the card draw event shape changes.

## Definition Of Done

- Draw behavior handles short deck and discard reshuffle.
- Empty/short deck behavior is explicit and tested.
- Removed cards are never drawn.
- `IncomeFlow` still delegates to card draw rather than owning deck logic.
- `go test ./...` passes.

