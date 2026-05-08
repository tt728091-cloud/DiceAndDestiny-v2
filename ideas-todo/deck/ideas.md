# Deck Ideas

## No Cards Available To Draw

We need to think through the intended behavior when a draw is requested but there are no cards available in either the deck or discard pile.

Current Story 11D behavior reports this as a short draw with `deck_empty=true`, but we should review later whether the game needs additional rules or presentation for this case.

Questions to revisit:

- Should the battle continue normally when a requested draw cannot be fully satisfied?
- Should the player receive a different resource, status, warning, or penalty?
- Should the event shape distinguish between a short draw after some cards were drawn and a zero-card draw?
- Should UI or logs present this as normal exhaustion, a special state, or an error-like condition?

## Maximum Hand Size

We need to decide how many cards a player or enemy can hold in hand at one time.

There should probably be a maximum hand size, but the source of that limit is still open. It could be driven by actor stats such as strength, constitution, dexterity, or another attribute, or it could come from a separate combat/deck rule.

Questions to revisit:

- Should players and enemies use the same maximum hand size rule?
- Which stat or actor property should determine maximum hand size?
- What happens when a draw would exceed maximum hand size?
- Are excess cards left in the deck, sent to discard, removed, or ignored?
- Should maximum hand size be visible in battle snapshots and UI?
