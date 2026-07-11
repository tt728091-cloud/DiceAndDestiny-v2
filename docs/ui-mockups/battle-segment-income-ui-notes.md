# Income Segment UI Notes

Reference image: `battle-concept-34-income-card-draw-energy.png`

This screen represents the Income segment where the player draws a card and gains energy.

## Main State

- Top bar highlights `Income`.
- Round label shows the current round and segment, such as `Round 3 - Income`.
- No player or enemy abilities are listed.
- No damage-removal rows are shown.
- Dice trays are inactive and blank.
- Center content focuses on draw plus energy gain.

## Center Draw Flow

The center should show a clear draw-card flow:

- Deck/card-back indicator on the left.
- Arrow from deck toward the new card.
- Newly drawn card displayed large in the middle.
- Arrow from the card toward the hand or resource area.

The drawn card should use the same card visual language as hand cards:

- Energy cost.
- Card name.
- Card art.
- Bottom symbol/effect strip.

## Energy Increase

Income also shows energy gain:

- Use a blue energy gem or resource medallion.
- Show concise text like `+1 Energy`.
- The player's profile energy value should remain visible in the left profile box.

## Dice Areas

Both side dice boxes are inactive:

- Player dice tray shows blank slots only.
- Enemy dice tray shows blank slots only.
- No roll buttons, roll counters, dice symbols, dice numbers, or selected ability strip should appear.

## Godot Setup Notes

Use this as the template for start-of-turn resource gain. The center should be able to animate deck-to-card-to-hand movement and energy increment feedback. Keep the hand visible at the bottom so the new card can visually join it after the income animation completes.
