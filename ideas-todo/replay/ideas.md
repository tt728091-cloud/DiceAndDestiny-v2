# Replay Ideas

## Replay-Accurate Draw Event Ordering

We need to think about whether battle events should be detailed enough to replay exact battle state transitions.

Current Story 11D draw behavior can internally do this:

```text
draw one card from deck
deck becomes empty
reshuffle discard into deck
draw another card from the reshuffled deck
```

But the current event stream reports this as:

```text
discard_reshuffled
cards_drawn with both cards
```

That is fine for a coarse battle log, but it may be wrong if events become the source for replaying the battle. A replay system would need to know that one card was drawn before the reshuffle and one card was drawn after the reshuffle.

Possible replay-friendly event shape:

```text
cards_drawn with the card drawn from the original deck
discard_reshuffled
cards_drawn with the card drawn from the reshuffled discard pile
```

Another possible shape:

```text
draw_sequence_started
cards_drawn from deck
discard_reshuffled
cards_drawn from reshuffled deck
draw_sequence_finished
```

Questions to revisit:

- Are events intended to be presentation/log facts only, or replay-authoritative state transitions?
- If events are replay-authoritative, should every state mutation have a matching event in exact order?
- Should `cards_drawn` include a source such as deck, reshuffled deck, or discard?
- Should draw requests emit one aggregated event or multiple ordered events?
- Do we need separate event shapes for replay/internal history and viewer-facing presentation?
