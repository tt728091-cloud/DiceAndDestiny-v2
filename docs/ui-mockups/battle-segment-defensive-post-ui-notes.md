# Defensive Post Ability Select UI Notes

Reference image: `battle-concept-31-defensive-post-ability-select.png`

This screen represents the Defensive segment after the player has selected a defense against the incoming enemy attack.

## Main State

- Top bar highlights `Defensive`.
- Round label shows the current round and segment, such as `Round 3 - Defensive`.
- Incoming enemy attack remains on the center-right.
- Selected player defense appears full-size on the center-left.
- The selected player defensive preview tile is highlighted in the lower row.

## Center Ability Details

The center mirrors the offensive post-select screen, but the player card is defensive:

- Player defense detail appears on the left with blue styling.
- Enemy incoming attack detail remains on the right with orange/red styling.

Example player defense:

- `Guard Stance`
- `Reduce damage by 3. Gain Guarded 1.`

Example enemy attack:

- `Cinder Strike`
- `Deal 3 damage. Apply 2 Poison.`

## Player Defense Row

The lower player row remains defensive:

- Show 10 compact defensive ability tiles.
- Highlight the chosen defense with a blue border/glow.
- Do not show offensive names like Pair, Full House, or Straight in this row during Defensive.

## Godot Setup Notes

Use this state after defense selection but before final damage/card removal resolution. The left and right center detail panels should support comparison: incoming damage/status on the right, mitigation/prevention on the left.
