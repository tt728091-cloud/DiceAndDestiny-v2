# Character Creation

## Phase 1: inspect the configured loadout (implemented)

Open **Character Creation** beside **Start Battle** in battle setup. Choose
Adventurer, Venom, Curse, or Blade Warden in the left rail. The initial selection
matches battle setup; closing the viewer preserves the setup selections.

- Health is the sum of decklist quantities, with total and unique card counts.
- Abilities lists offense and defense separately. Click to inspect full rules,
  qualification tiers, cost, usage limits, and saved-card destination.
- Deck lists configured cards, quantities, costs, and summaries. Search the
  current deck by name or rules. Click a card for its normal card preview and
  full rules, play timing, and destination.
- Dice shows configured types, quantities, and each face's number/symbol.
- Opening hand, starting energy, hand limit, and round income remain visible.
- Back or Escape returns to setup. Reload definitions rereads the files without
  starting a battle or touching saved player data.

The native `character_catalogs` request is read-only and does not initialize or
replace a learned session, draw cards, create a battle, or write history/saves.
Each character gets its own validated catalog with the same base/extension
composition as battle assembly, avoiding cross-character symbol overrides.
The screen uses the existing presentation helpers and card renderer, and restores
the previous presentation catalog when closed. Legacy persisted JSON shapes are
unchanged; the viewer endpoint has an explicit client-facing loadout envelope.

No quantities, XP, upgrades, unlocks, or template files are editable in this phase.

## Next phases

1. **Edit a draft loadout.** Add/remove copies and search a separate available-card
   library. Recompute health immediately. Provide Apply, Revert, and reset to the
   character template. Validate IDs/counts in the authority and persist an owned
   loadout, rather than overwriting shared content definitions.
2. **Spend experience.** Add an authoritative XP balance, configured prices, and
   explicit upgrade links. Purchases consume XP and change card instances or
   ability slots atomically. Preview the before/after result before purchase.
3. **Battle rewards and discovery.** Victory awards XP and unlocks potential
   cards/abilities. Discovery adds a choice to the available library; equipping
   it remains a separate purchase. Open this same screen between battles with
   the run's loadout and balance.

Keep character templates, unlocked content, and the equipped run loadout distinct.
Map exploration is outside this feature's current scope.

## Validation

- Go catalog tests compare all four preview loadouts/cards with actual battle
  assembly and verify that runtime catalog reads preserve the active session.
- `verify_character_creation.gd` exercises all four characters, every list/detail,
  health totals, deck search, pointer selection, menu/Back navigation, and layout
  at 1024, 1280, and 1920 window widths.
- Full Go suite, native authority smoke test, and Adventurer menu-to-battle test.

Run Godot checks through `./scripts/godot.sh` from the repository root.
