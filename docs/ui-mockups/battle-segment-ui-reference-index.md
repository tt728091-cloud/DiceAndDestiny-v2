# Battle Segment UI Reference Index

These are the current Godot UI reference screens for the battle segment flow.

## Baseline Layout

- `battle-concept-27-mirrored-player-profile.png`
- `battle-concept-27-godot-ui-notes.md`

Use this as the base layout for shared left/right columns, actor profile boxes, enemy roster, card zones, dice trays, combat log, and hand presentation.

## Segment States

| Segment | State | Image | Notes |
| --- | --- | --- | --- |
| Effects | Poison roll | `battle-concept-33-effects-poison-roll.png` | `battle-segment-effects-ui-notes.md` |
| Income | Draw + energy | `battle-concept-34-income-card-draw-energy.png` | `battle-segment-income-ui-notes.md` |
| Offensive | Pre ability select | `battle-concept-28-offensive-pre-ability-select.png` | `battle-segment-offensive-pre-ui-notes.md` |
| Offensive | Post ability select | `battle-concept-29-offensive-post-ability-select.png` | `battle-segment-offensive-post-ui-notes.md` |
| Defensive | Pre ability select | `battle-concept-30-defensive-pre-ability-select.png` | `battle-segment-defensive-pre-ui-notes.md` |
| Defensive | Post ability select | `battle-concept-31-defensive-post-ability-select.png` | `battle-segment-defensive-post-ui-notes.md` |
| Damage | Pending removal | `battle-concept-32-damage-pending-removal.png` | `battle-segment-damage-ui-notes.md` |

## Shared Implementation Rules

- Keep the same dark fantasy visual style across every segment.
- Keep the left and right side columns stable unless the segment explicitly changes a box.
- Highlight only the active segment in the top bar.
- Use blank/inactive dice trays in Effects, Income, and Damage.
- Show ability rows only in Offensive and Defensive.
- Use full center detail panels only when a selected or resolving item needs focus.
- Keep text concise and rely on icons, cards, dice, and clear spatial grouping.
