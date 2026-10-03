# Minimal battle UI, second pass

Five new concepts generated using built-in image_gen, based on the original game screenshot for art and the supplied Slay the Spire screenshot for UI scale. Prior designs were rejected because they remained crowded. These reduce the size, framing and persistent text of the HUD rather than simply moving large panels around.

- 06-ground-level.png: compact dice/ability cluster lower left; short character-local health bars; bottom card fan.
- 07-paired-character-dice.png: small dice banks beneath each character; compact abilities above the hand.
- 08-dice-on-hand.png: player dice just above the hand; enemy dice above the enemy; tiny vertical ability shortcuts beside cards.
- 09-edge-dice.png: narrow columns of dice at opposing edges, keeping center open.
- 10-whisper-labels.png: compact named abilities above cards for discoverability, with minimal framing.

## Information access

These are resting-state concept images, not interactive implementations. Cards and dice persist. Health and active status counts remain visible. Full card text, ability names/requirements/rules, status explanations, per-die identity/modifiers and detailed pile counts must be available through hover, click/tap or controller focus. Expanded views can be pinned. Critical per-die changed state must remain marked without hovering. The reduced detail visibility is a deliberate tradeoff and does not mean removing it from the game.

The enemy Curse 2 / Count 3 badges are illustrative active statuses added to demonstrate scale and placement. These are not readings from the original screenshot. Generated icons and requirement motifs are placeholders, not authoritative game rules. Some secondary pile counters are omitted from individual renderings; these remain available from the associated pile or actor inspection. The compact phase dots must expose full phase names and should also show the current phase in text in implementation. Enemy ability shortcuts in the upper right should remain clearly distinguished from player ability shortcuts.

Recommended starting points: 07 for ownership clarity; 08 for concentrating player interaction around the hand. Validate actual dice click targets, UI scaling and expanded rule legibility before implementation. Small visible icons can have larger invisible hit areas.

No game code was changed. Original prompts are in prompts.md.

