# Dice Presentation

Owns dice visual components, dice animation, and dice result presentation.

Dice rule behavior belongs in Go authority code.

## Carved-stone dice

`stone_die.gd` draws every die: a stone block seen slightly from
the front, the face symbol engraved into the top, and the number cut into the
upper-right corner. It is procedural, so it needs no imported art.

- Pale stone is the player's; enemy-owned standard dice are slate. Faction dice
  keep their own stone everywhere (`curse_d6` violet, `venom_d6` green,
  `brine_d6` teal).
- `BattleDiceTray` composes a `BODY` stone with its own symbol, number and
  Curse labels. Other dice (defense, effect, Curse previews) call
  `STONE_DIE.dress(control, die_id, face, enemy_owned, mark)` after setting
  their accessible text.
- A new die symbol needs a carved shape in `stone_die.gd` (`_build_glyph`); until it has
  one, its catalog glyph is engraved as text.
