# Unified defense presentation

After defense selection (including any optional Catalyst choice), the client submits the sole legal roll command immediately. The authority's roll checkpoint stays behind the existing board. Once both defenses are revealed, a single pair of side panels animates their dice, reveals the landed benefits, reduces incoming damage, and sends status gains to the appropriate profile. Dice and results use the same controls throughout.

The internal rules stages and authoritative dice values are unchanged. Cosmetic rolling faces do not determine the outcome. The title remains **Defense** during rolling and results. A refresh preserves the animation clock for the same roll; a changed roll gets a new sequence.

## Pacing

Edit these values under `[dice_and_destiny]` in `dice-and-destiny-client/project.godot`, then restart the game:

| Setting | Default | Purpose |
| --- | ---: | --- |
| `presentation/defense_roll_seconds` | 1.2 | Both sides' dice roll before landing. |
| `presentation/defense_effects_seconds` | 2.4 | Prevention, damage changes, and status flights. |
| `presentation/defense_hold_seconds` | 1.5 | Read the finished result before automatic advancement. |

The shared accessor is `presentation/battle/defense_timing.gd`. Auto-pass waits through the combined 5.1-second sequence, then uses its existing brief button highlight. Actual reaction choices and the existing Disable auto-pass control remain available. Manual passage can still advance sooner.

## Verification

`verify_defense_result_animation.gd` checks the selection-to-roll handoff, simultaneous rolling, authoritative landing values, stable panel/die instances, effect ordering, configurable timing, the final viewing pause, source-specific blocks, Catalyst caps, and Incubation behavior. Graphical screenshots cover rolling, landing, and settled results. Additional checks cover automatic passage, enemy-only defense, the battle scene, and cinematic layouts.
