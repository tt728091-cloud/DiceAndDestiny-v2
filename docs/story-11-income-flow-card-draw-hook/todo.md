# Story 11 Follow-Up TODOs

These notes capture design gaps found during the Story 11 walkthrough. Review these before closing the story or before starting the next gameplay story.

## Card Zone Model

- Current code only models `Deck` and `Hand`.
- Game card zones should include:
  - deck
  - hand
  - discard pile for played cards that are not permanently removed
  - permanently removed pile for damage/lost cards
- Add discard and permanently removed zones to battle card state when the next card-zone story is scoped.

## Empty Or Short Deck Draw Behavior

- Current draw logic only draws up to the number of cards currently in `zones.Deck`:

```go
drawCount := count
if drawCount > len(zones.Deck) {
	drawCount = len(zones.Deck)
}
```

- This does not handle the intended behavior where discard cards can be reshuffled into the deck if the deck cannot satisfy a draw.
- Define and implement explicit behavior for:
  - deck has enough cards
  - deck is short but discard has cards
  - deck is empty but discard has cards
  - deck and discard are both empty

## Income Choice Model

- Current income draw accepts only a non-negative card count and treats negative counts as hard errors:

```go
case count < 0:
	return nil, fmt.Errorf("%w: count must be non-negative", ErrInvalidDraw)
```

- Future income may allow different income choices, such as drawing fewer cards in exchange for more action points.
- Replace the placeholder card-count-only income behavior with an explicit income choice/config model before expanding income mechanics.

## Shuffle And Draw Order

- Current draw logic draws from the current deck order:

```go
drawn := append([]string(nil), zones.Deck[:drawCount]...)
```

- This is correct only if the deck was already deterministically shuffled before drawing.
- Add deterministic shuffle support before real battle setup relies on randomized deck order.
- Decide where shuffle belongs:
  - battle setup shuffle
  - reshuffle discard into deck
  - both

## Battle Setup Deck Source

- Current `state.NewBattle` hardcodes the player deck:

```go
Deck: []string{"card-1", "card-2", "card-3"}
```

- Future battle setup should receive or build the player deck from saved player/run state.
- Add a battle setup path that accepts prepared player combat state instead of hardcoding decks in `state.NewBattle`.
