# Character Creation

## Inspect the configured loadout (implemented)

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

## Phase 2: owned deck editing (implemented)

- **Deck** shows equipped cards and quantities. Select a card and edit **Copies in
  deck**, or use **Add a copy**. Zero removes it from the draft.
- **Card library** searches all cards in the selected character’s validated content
  catalog, including cards absent from its starter deck. The catalog is the same
  base/extension composition used by battle assembly; this is not a global library
  across incompatible content packs and is not yet an unlock system.
- Health and unique/total card counts update immediately. Each character retains
  its own draft when switching characters; an asterisk marks unsaved changes.
- **Apply deck** validates and atomically saves the selected character. **Revert**
  restores its saved deck. **Reset to template** restores the authored starter deck
  in the draft; Apply is still required. Back/Escape and Reload protect unsaved work
  with Keep editing / Discard changes.
- Editor limits are 1–100 total cards and 1–20 copies per included card. A draft may
  be empty or over the total limit, but cannot be applied. These are editor bounds,
  not XP costs. IDs, integer counts, duplicates, bounds, and catalog availability
  are checked by the Go authority. Invalid saves leave the last saved file intact.
- Owned decks are versioned JSON under the launcher-isolated runtime directory
  `user/character_loadouts/<character>.json`. Content YAML remains unchanged.
  Corrupt/stale decks display an error and can be repaired by applying a valid deck.
- Battle setup displays the saved health total. New battles and rematches read the
  saved deck for the human seat only, even in a same-character mirror matchup.
  Opening draws, card instances, and maximum health use that deck. Active battles
  retain their pinned setup. Replay records include the starting deck, so later
  edits cannot change a replay. Sessions without a loadout root keep templates.

Abilities and dice remain inspectable but are not editable. XP, upgrade purchases,
and reward unlocks remain future work.

## Next phases

1. **Spend experience.** Add an authoritative XP balance, configured prices, and
   explicit upgrade links. Purchases consume XP and change card instances or
   ability slots atomically. Preview the before/after result before purchase.
2. **Battle rewards and discovery.** Victory awards XP and unlocks potential
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

Phase 2 validation additionally covers native invalid-input/atomic-save behavior,
all four saved character decks in both seats, complete customized battles and
replay after saved-deck changes, and same-character opponent isolation.
`verify_character_deck_editing.gd` uses pointer/keyboard interactions to add a new
card, type quantities, apply, revert, reset, protect unsaved changes, reopen the
saved deck, verify menu health, and start a real battle with the added copies.
It also captures editor layouts at 1024, 1280, and 1920 widths.
