# Cinematic Duel — symbols-only recipes

Five static UI concept images generated with the built-in image generation tool. No game code changed.

- [Stacked symbol rows](01-stacked-symbol-rows.png)
- [Inline symbol groups](02-inline-symbol-groups.png)
- [Die-face chips](03-die-face-chips.png)
- [Borderless recipes](04-borderless-recipes.png)
- [Separate recipe capsules](05-tier-capsules.png)

## Shared changes

Ability names remain. Requirements use repeated icons only: no ingredient words, quantities, multipliers or explanatory recipe text. An information affordance indicates full ability details on hover/focus. Card names and costs remain visible. Player/enemy piles stay removed, and the cinematic battlefield stays open.

## Visual verification

Each final image was inspected for these recipe groups:
- Needlefang: three alternatives with three, four, and five tooth glyphs.
- Venom Gland: two vial glyphs and one spiral.
- Fever Spike: two alternatives: two teeth / one vial / one spiral; two teeth / two vials / one spiral.
- Terminal Bite: three spirals.

The ability title emblems are decorative identifiers, distinct from the recipe glyphs beneath them. Separator lines, slashes or capsule boundaries distinguish alternative groups. In implementation, each alternative should focus/highlight as a whole, with the current ability effects available on hover, keyboard/controller focus or tap.

Sample 2 emphasizes a compact horizontal bar; sample 3 makes each required die especially countable; sample 4 removes most framing.

[Full prompts](prompts.md)

