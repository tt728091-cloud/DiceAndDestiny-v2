# Bell Diver and Ribbon Eel

Two more Drowned Oracle familiars built like [Brine Mask](brine-mask.md): one
attack, one defense, a fixed keep rule, and up to three rolls. Their decks are
blank cards that only count as health. Select **Bell Diver · minion · keeps
every 5 and 6** or **Ribbon Eel · minion · keeps every even die** in the
opponent menu. Both use the existing `drowned_oracle/bell_diver.png` and
`drowned_oracle/ribbon_eel.png` art at the 320-pixel minion height.

| | Brine Mask | Bell Diver | Ribbon Eel |
| --- | --- | --- | --- |
| Health | 16 Brine Surge | 18 Ballast Stone (blank) | 15 Shed Ribbon (blank) |
| Keeps | every 3 | every 5 and 6 (Toll) | every even die (Snare) |
| Chance per die over three rolls | 42% | 70% | 88% |
| Attack | 2 per Brine: 2/4/6/8/10 | 3/4/5 Tolls: 4/5/8 | 3/4/5 Snares: 3/5/7 |
| Miss | no Brine (~7%) | fewer than 3 Tolls (~16%) | fewer than 3 Snares (~2%) |
| Defense (1D6) | Salt Veil: 1/2/3 | Brass Helm: 1–3 → 2, 4–6 → 3 | Slip the Current: 1–3 → 1, 4–5 → 2, 6 → 4 |

- **Bell Diver** is the armored bruiser: a heavy all-or-nothing attack with a
  real miss chance, the toughest deck, and the most reliable block.
- **Ribbon Eel** is the snaring striker: it almost always connects, but has the
  smallest deck and a swingy defense that is usually weak and sometimes slips
  most of a hit.
- Both start with one card in hand and zero energy. Income draws one card and
  grants no energy; hand limit three. Ordinary draw, damage, and hand-limit
  rules apply to their blank cards.

## Blank cards

A card with no `playable_during` timing and no `operations` is a blank card
(`content.IsBlankCard`). It is never a legal play and only counts as health.
A card with effects still requires a play timing. The card editor does not
publish effect-less cards, so the catalog round-trip test skips blank cards.

## Keep rule

`single_ability_policy.target_faces` lists every face the controller keeps;
the original `target_face` still works and both may be combined. Omitting
`card_id` means the minion never plays a card. The controller still submits
only ordinary legal keep/reroll commands against the shared authority.

Content lives in `dice-and-destiny-server/content/minions_v1`: the `toll`,
`snare`, and `murk` symbols, `bell_d6` and `eel_d6`, the two attacks and
defenses, the blank cards, and the two combatants. The dice have their own
stone colours and engraved symbols in `presentation/dice/stone_die.gd`.

## Hand-limit fix

Lower-health minions exposed a stall: when queued Venom or Curse work removed
hand cards after the hand-limit window opened, the window could demand a
negative discard and nobody had a legal action. A restored hand-limit window
is now re-checked; it reopens only for actors still over their limit, or the
segment continues.

Verification:

```sh
cd dice-and-destiny-server
go test ./...
cd ..
./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_drowned_oracle_minions.gd
```

The Go tests exhaust all 7,776 five-die rolls for each minion's keep rule and
complete 80 Venom games per minion across both seats, checking keep decisions,
that blank cards are never offered as plays, and starting health. Against the
scripted Venom test player, the player fails to win (loss or draw) about 15%
of games against Brine Mask, 16% against Bell Diver, and 20% against Ribbon Eel. The Godot test starts each
minion from the dropdown, checks art, blank cards, dice symbols and colours,
the battle badge, and the defense roll, then plays a full battle and rematch.
