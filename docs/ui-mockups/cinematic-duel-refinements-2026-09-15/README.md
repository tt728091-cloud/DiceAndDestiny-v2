# Cinematic Duel refinements

Five static screenshots generated with the built-in image generation tool. Original cinematic scene retained; player and enemy pile counters removed. No game code changed.

## Samples

- [Medallions with recipes](01-medallions-and-recipes.png)
- [Compact recipe tiles](02-compact-recipe-tiles.png)
- [Symbol recipe strips](03-symbol-recipe-strips.png)
- [Needlefang hover details](04-needlefang-hover.png)
- [After-roll matching](05-after-roll-matches.png)

Recommended direction: sample 2's readable compact tiles, sample 4's hover detail, and sample 5's qualified-tier highlighting. Sample 1 stays closest to the first Cinematic Duel.

## Verified recipes

Checked against dice-and-destiny-server/content/venom_v1/abilities/*.yaml.

| Ability | Visible target |
| --- | --- |
| Needlefang | 3 Fang OR 4 Fang OR 5 Fang |
| Venom Gland | 2 Gland + 1 Coil |
| Fever Spike | 2 Fang + 1 Gland + 1 Coil OR 2 Fang + 2 Gland + 1 Coil |
| Terminal Bite | 3 Coil |

Needlefang hover shows base tiers: 3 Fang gives 4 damage + 1 Poison; 4 Fang gives 3 damage + 2 Poison; 5 Fang gives 2 damage + 3 Poison. More Fangs change the damage/poison tradeoff. Show current modified values in an implementation, and allow choosing any qualified tier.

## Interaction intent

- Exact targets always visible. Separate lines/chips denote alternative tiers.
- Hover, keyboard/controller focus, or tap opens complete rules. Allow pinning a detail panel.
- Sample 5's example roll is Fang/Fang/Fang/Gland/Coil. Only Needlefang's 3-Fang tier and Fever Spike's base tier qualify. The player has held three Fangs and can reroll the other two, with two rolls remaining.
- Dice appear after a roll and remain accessible for hold/reroll decisions; the idle battlefield has no empty dice boxes.
- Removed pile counters are a presentation choice for this iteration, not a change to health or card-zone mechanics.
- Generated artwork is a design reference. Exact bars, icon consistency, typography and focus behavior should be set deterministically during implementation.

[Full generation prompts](prompts.md)

