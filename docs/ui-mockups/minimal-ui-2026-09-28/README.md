# Five minimal battle UI concepts

Generated with the built-in image generation tool on 2026-09-28, using the two supplied screenshots as references. These are design studies, not changes to the running game. Final prompts and correction prompts are in prompts.md.

## Recommendation

Start with 02-character-anchored.png. Keep cards and both dice banks persistently visible. Put health, energy, and status badges beside their owner. Reduce the visual weight of abilities and utility controls. Use full-rule inspection on hover, click or controller focus, with a pinnable detail panel. If full ability rules must always be visible, use 05-open-rulebook.png.

The supplied Slay the Spire screenshot uses local grouping, icon/count status indicators, and a quiet battlefield to reduce visual competition. Dice and Destiny can use those principles while retaining its additional mechanics. Large pale ability panels, repeated labels and disconnected information groups currently compete with cards and combatants.

## Concepts

1. **Bottom control dock** — A horizontal ability strip and bottom group of player dice, roll controls and cards. Character health remains near the combatants. Enemy dice remain visible above the dock. Good for concentrating decisions in one area; the dock uses more vertical space.
2. **Character-anchored controls** — Each combatant owns a compact health/resource/dice group. Abilities form a small grid beside the hand. Best ownership clarity and closest conceptual match to the supplied Slay the Spire layout; full rules need inspection.
3. **Slim command sidebar** — Player health, resources, dice and abilities share one aligned left panel. The hand uses the remaining bottom width. Good for scanning and mouse movement; reduces horizontal battlefield space.
4. **Top tactical strip** — Both participants' health, resources and dice sit at the top, with abilities and hand below. Good for direct comparisons; player attention travels between top and bottom.
5. **Compact open rulebook** — Four complete ability entries share a two-by-two ledger alongside the cards. No need to inspect to read ability rules; denser than the other options and requires a minimum readable UI scale.

## Information and interaction contract

All five final concepts show five cards, five player dice, five opponent dice, four ability choices, health, energy, pile counts, round/phase information, roll allowance, Roll/Skip and utility access. Cards and dice keep stable positions across phases; unavailable actions can lose emphasis without disappearing or shifting layout.

In concepts 1–4, ability names and requirements remain visible; complete rules and explanations are available through hover, click or controller focus. This preserves access to information, but does not keep all rule text simultaneously visible. Concept 5 keeps all four full ability descriptions on screen.

The screenshot has no active statuses. Zero-status indicators preserve that fact in the examples. In implementation, active effects should use distinct icon + stack/duration badges, with readable names and rules through inspection. Individual dice need visible changed-state markers, locks and targeting feedback; the shared All clean label applies only when every die is actually clean. Do not rely on color alone or hide decision-critical dice state in a tooltip.

These generated images illustrate arrangement and hierarchy; typography, icon semantics, precise game wording and small-window behavior need implementation-level verification. Do not use the generated text as the authoritative rules source. No game code was edited or run.

