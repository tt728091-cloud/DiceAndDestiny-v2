# Offensive Post Ability Select UI Notes

Reference image: `battle-concept-29-offensive-post-ability-select.png`

This screen represents the Offensive segment after both player and enemy dice are revealed and both sides have selected an ability.

## Main State

- Top bar highlights `Offensive`.
- Round label shows the current round and segment, such as `Round 3 - Offensive`.
- Player and enemy selected ability preview tiles are highlighted.
- Full selected ability detail cards appear in the center.

## Center Ability Details

The center space shows two large selected ability panels:

- Player selected ability appears on the left with blue styling.
- Enemy selected ability appears on the right with orange/red styling.
- Each full panel includes the ability name, dice/symbol requirement, short rules text, and result icon strip.

Example player detail:

- `Two Pair`
- `Deal 5 damage.`

Example enemy detail:

- `Cinder Strike`
- `Deal 3 damage. Apply 2 Poison.`

## Dice Areas

Both dice areas are revealed:

- Player dice box shows five rolled dice.
- Enemy dice box shows five revealed dice.
- Each die has a face symbol and a small number in the upper-right corner.
- Enemy dice box also shows the selected enemy ability strip.

## Ability Rows

Ability rows remain visible as compact previews:

- Enemy selected ability tile is highlighted in orange/red.
- Player selected ability tile is highlighted in blue.
- The highlighted previews stay in place even when the full detail cards are shown in the center.

## Godot Setup Notes

Use this state after dice reveal and ability matching. The selected ability detail panels should be reusable components that can render either player or enemy data. Keep the small preview tile highlight as a persistent selection indicator.
