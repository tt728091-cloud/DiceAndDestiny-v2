# Minimal offensive phase — five visual directions

Generated with the built-in image generation tool on 2026-09-15. These are static design explorations, not implemented screens or standalone production sprites.

## Samples

- [Cinematic duel](01-cinematic-duel.png)
- [Tabletop miniatures](02-tabletop-miniatures.png)
- [Ink and ash](03-ink-and-ash.png)
- [Command rail](04-command-rail.png)
- [Orbiting sigils](05-orbiting-sigils.png)

## Shared interaction design

The current Venom versus Blade Warden screenshot supplies the battle state. The older battle-concept-28 image supplies aesthetic context only.

- Health: short bar plus exact fraction beside each battlefield actor.
- Energy and effects: small icon/count badges; focus or click exposes exact names, rules, duration and stacks. Bleed should use an unmistakable blood drop in production; generated examples sometimes resemble a flame. Use the same energy icon for both actors.
- Character information: select the sprite/nameplate or Inspect to reveal the full character sheet, human/AI identity and battle metadata.
- Player abilities: four named icons; focus or click opens the complete recipe, tiers and effect text. Keep this pinned while selecting or comparing.
- Enemy abilities: an expandable book shows all five abilities and complete recipes.
- Cards: all five hand cards remain identifiable with their energy costs. Hover, keyboard focus or click raises one card to reveal its full rules. Provide touch/controller equivalents; do not make information hover-only.
- Card piles: counters open each pile with its existing visibility rules. Enemy values are deck 1, hand 4, discard 7, removed 8. Player values are deck 2, hand 5, discard 0, removed 17.
- Dice: hide the empty pre-roll slots. Roll brings all five dice onto the center stage; retain visible keep/reroll controls and exact results while the player is deciding. Collapse settled dice to a review control afterward. Enemy dice remain inspectable via the enemy dice control or Inspect in layouts without a dedicated button.
- Turn controls: show remaining rolls, active phase, round, Roll and Skip; make auto-pass available through settings.
- Log and developer transcript: expandable history/Inspect panels preserve access without permanently filling the battlefield.
- Center animation stage: reserve for dice throws, effects tied to the receiving sprite, and temporary card-loss/tearing presentations; keep outcome details replayable in the log. The examples show the calm planning state before these animations.

These images show the closed/default controls. They do not visually demonstrate every expanded information state. Generated icon shapes, repeated controls (especially the duplicated enemy ability access in sample 5), and spacing still need normal UI design refinement after selecting a direction.

## Recommended starting point

Sample 2 emphasizes a tactile tabletop with a large clear dice area. Sample 5 places abilities beside the player and offers the clearest route toward an open central effects stage. Sample 3 explores a distinctive ink art style, although its characters could be smaller in implementation.

## Generation prompts

See [prompts.md](prompts.md) for the full shared prompt and all five direction prompts.

