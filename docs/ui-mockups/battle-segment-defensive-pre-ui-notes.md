# Defensive Pre Ability Select UI Notes

Reference image: `battle-concept-30-defensive-pre-ability-select.png`

This screen represents the Defensive segment after an enemy attack has been chosen and revealed, but before the player selects a defense.

## Main State

- Top bar highlights `Defensive`.
- Round label shows the current round and segment, such as `Round 3 - Defensive`.
- The enemy incoming attack is shown in the center-right.
- The player defense detail area on the center-left is empty.
- Player defensive ability previews are visible along the lower center.

## Incoming Enemy Attack

The enemy attack that got through is shown as a full detail panel:

- Enemy attack card appears on the center-right.
- It uses orange/red enemy styling.
- It includes name, dice/symbol requirement, short rules text, and result icons.

Example:

- `Cinder Strike`
- `Deal 3 damage. Apply 2 Poison.`

## Player Defense Options

The lower player row changes from offensive abilities to defensive abilities:

- Show 10 compact defensive ability tiles.
- Examples: Shield, Guard, Evade, Cleanse, Counter, Block, Parry, Ward, Barrier, Armor Up.
- Tiles should use blue player styling.
- No player defensive tile is selected yet.

## Dice Areas

Dice trays can remain visible from the previous reveal, but the key interactive focus is defense selection:

- Player dice tray remains in its left-side box.
- Enemy dice tray remains in its right-side box.
- The center should make clear that the enemy attack is pending defense.

## Godot Setup Notes

Use this state when the game is waiting for the player to choose a defense response. The enemy attack detail panel should be locked/read-only. The player defensive ability row should be interactive and should drive the transition to the defensive post-select state.
