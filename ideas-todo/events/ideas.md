# Event Ideas

## After Cards Drawn Trigger Hook

We should think about using card draw as a trigger point for effects that happen after cards are drawn.

One possible direction: after drawing cards, the cards drawn this turn could create combo effects without needing to be played. For example, if the player draws two cards with the armor type, then this round their defensive ability could prevent 1 additional damage.

The trigger might care only about the cards drawn in the current draw event, or it might care about the newly drawn cards plus cards already in hand.

This could also become a hook for character board ability upgrades. For example, a defensive ability might normally say: roll 3D6, and on each 4 or higher prevent 1 damage. With an upgrade triggered or enabled by draw/combo state, that ability might become: roll 4D6, and on each 4 or higher prevent 1 damage.

Questions to revisit:

- Should draw-triggered effects inspect only newly drawn cards or the whole hand after drawing?
- Are these effects temporary for the current round, current segment, or until used?
- Should card types, tags, rarity, or card names drive these combos?
- Should character board upgrades listen to the same event hook as card combo effects?
- How should these effects be represented in battle state and events so the UI can explain why a defensive ability changed?
