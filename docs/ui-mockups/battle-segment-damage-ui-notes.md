# Damage Segment UI Notes

Reference image: `battle-concept-32-damage-pending-removal.png`

This screen represents the Damage segment where damage is converted into pending card removals for each side.

## Main State

- Top bar highlights `Damage`.
- Round label shows the current round and segment, such as `Round 3 - Damage`.
- Player and enemy ability rows are removed.
- No selected ability detail cards are shown.
- Dice trays are inactive and blank.
- Center content focuses on card removal resolution.

## Pending Card Removal Rows

The old ability-row space is reused for pending removal cards:

- Upper center row shows enemy cards pending removal.
- Lower center row shows player cards pending removal.
- Each row has a short label and count, such as `Enemy Cards Pending Removal (7 Cards)`.
- Card thumbnails show name, cost, art/icon, and symbol strip.
- Cards have cracked, slashed, or burning overlays to show pending removal.

## Carousel Arrows

Both pending removal rows need left/right paging arrows:

- Arrows appear at the ends of each row.
- This supports cases where more than 10 damage/cards need to be reviewed.
- Godot should treat each row as a horizontally pageable list.

## Dice Areas

Both dice boxes are inactive:

- Player dice tray shows blank slots only.
- Enemy dice tray shows blank slots only.
- No roll buttons, roll counters, dice symbols, dice numbers, or selected ability strip should appear.

## Godot Setup Notes

Use reusable card-thumbnail components for the pending removal rows. The carousel should support paging, keyboard/controller navigation, and selecting a card for inspection later if needed. The Damage segment should not expose ability selection controls.
