# Effects Segment UI Notes

Reference image: `battle-concept-33-effects-poison-roll.png`

This screen represents the Effects segment where active status effects resolve before Income.

## Main State

- Top bar highlights `Effects`.
- Round label shows the current round and segment, such as `Round 3 - Effects`.
- No player or enemy abilities are listed.
- No damage-removal rows are shown.
- Dice trays are inactive and blank.
- Center content focuses on one resolving status effect.

## Poison Resolution Card

The center shows a large status-effect card for player poison:

- Title: `Poison` or `Poisoned`.
- Green poison icon is prominent.
- The card clearly points to or visually affects the player.
- The card text explains the roll result:
  - `1-4: take damage`
  - `5-6: remove poison`
- The bottom of the card shows the poison dice roll area.

## Poison Roll Area

The poison roll should be represented on the bottom of the status card:

- Show a d6 roll slot or dice graphic.
- Show the `1-4` and `5-6` result ranges near the dice.
- This is not the regular player offensive dice tray.

## Dice Areas

Both side dice boxes are inactive:

- Player dice tray shows blank slots only.
- Enemy dice tray shows blank slots only.
- No roll buttons, roll counters, dice symbols, dice numbers, or selected ability strip should appear.

## Godot Setup Notes

Use this as the template for resolving any start-of-round status effect. The center status card should be data-driven so other effects can reuse the same layout with different icon, color, roll rules, and outcome text.
