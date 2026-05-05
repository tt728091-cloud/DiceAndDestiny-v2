# Story 11B: Full Card Zone State

## Purpose

Expand card state from only `Deck` and `Hand` to the full set of zones needed by the game:

```text
deck
hand
discard pile
permanently removed pile
```

This story adds state shape only. It should not implement card play, damage, or discard reshuffle draw behavior yet.

## Design Context

Story 11 added minimal card draw state. Story 11A should provide actor combat setup and a player deck source. This story builds on that by preparing the zone model needed for real card lifecycle rules.

Card zone meanings:

- `Deck`: cards available to draw from.
- `Hand`: cards currently held and available for future play.
- `Discard`: played or spent cards that are not permanently removed.
- `Removed`: cards permanently lost, such as from damage.

## Before Coding, Read

- `README.md`
- `stories/godot_pve/11_income_flow_card_draw_hook.md`
- `stories/godot_pve/11a_battle_setup_player_combat_state.md`
- `docs/story-11-income-flow-card-draw-hook/todo.md`
- `docs/v2-planning/godot_pve/03_story_testing_policy.md`
- `docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md`
- `dice-and-destiny-server/internal/battle/state/battle.go`
- `dice-and-destiny-server/internal/battle/card/draw.go`
- `dice-and-destiny-server/internal/battle/card/draw_test.go`
- `dice-and-destiny-server/internal/battle/engine/engine_test.go`

Assume Story 11A is implemented. If it is not, adapt to the actual local state shape but do not reintroduce hardcoded production deck setup.

## Scope

- Expand `state.CardZones` to include:

```go
type CardZones struct {
	Deck    []string
	Hand    []string
	Discard []string
	Removed []string
}
```

- Update battle setup/state tests for the new fields.
- Update `card.DrawCards` tests to assert discard and removed piles are preserved.
- Keep `DrawCards` drawing only from `Deck` for this story.
- Keep empty-deck behavior as explicit current behavior until Story 11D.

## Out Of Scope

- Card play.
- Moving played cards into discard.
- Damage and permanent removal behavior.
- Reshuffling discard into deck.
- Shuffling.
- Income choices/resources.
- Snapshot card visibility.
- Godot, C++, or UI changes.

## Requirements

- Card state can represent all four zones.
- Existing draw behavior still moves cards from deck to hand.
- Drawing cards must not modify discard or removed piles.
- Tests must make the intended zone preservation explicit.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add or update focused tests proving:

- new battle setup initializes deck, hand, discard, and removed zones predictably
- drawing from deck to hand leaves discard unchanged
- drawing from deck to hand leaves removed unchanged
- current empty deck behavior remains explicit until the reshuffle story

## Definition Of Done

- `CardZones` can hold deck, hand, discard, and removed cards.
- Current Story 11 draw tests still pass with expanded state.
- No gameplay behavior beyond zone shape is added.
- `go test ./...` passes.

