# Offensive Pre Ability Select UI Notes

Reference image: `battle-concept-28-offensive-pre-ability-select.png`

This screen represents the start of the Offensive segment before the player has rolled or selected an ability, and before enemy dice have been revealed.

## Main State

- Top bar highlights `Offensive`.
- Round label shows the current round and segment, such as `Round 3 - Offensive`.
- No player or enemy ability is selected yet.
- The center battle sigil stays mostly open.
- Enemy ability previews are visible across the upper center.
- Player ability previews are visible across the lower center.

## Player Dice Area

The player dice box is active, but no roll has happened yet:

- Five player dice slots are visible.
- Dice faces are blank.
- No symbols or numbers are shown on the dice.
- `Rolls Used` has no filled blue dots.
- `Rolls Left` is `3`.
- `Roll` and `Pass` buttons are visible.

## Enemy Dice Area

The enemy dice box is not revealed yet:

- Enemy dice tray is empty.
- No enemy dice faces are visible.
- No enemy selected ability strip is visible.

## Ability Rows

Both sides show compact ability previews:

- Enemy ability previews are shown along the upper center.
- Player ability previews are shown along the lower center.
- Each ability tile should show a small icon, name, and dice/symbol requirement.
- No tile should have a selected glow in this state.

## Godot Setup Notes

Use this as the initial Offensive command state. The player dice tray should be interactive here. Bind `Roll`, `Pass`, `rolls_used`, and `rolls_left` to battle state. Enemy dice and selected enemy ability should stay hidden until reveal.
