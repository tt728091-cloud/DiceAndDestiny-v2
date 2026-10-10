# Campaign progression

The campaign is the battle → XP → deck change → next battle loop. Open it from the main menu with **Campaign**.

## Flow

1. The **Campaign** screen opens on the Starter; a picker switches to the Adventurer. Each character keeps its own XP, deck and place in the campaign. The screen shows the character's health (equipped cards), XP to spend, run number and victories. It also shows the encounter path, with each encounter marked Cleared, Next or Ahead.
2. **Fight · <encounter>** starts the next encounter. It uses the character's Progression deck and ability board, Seat A, and the unified defense rule.
3. When the battle ends, the authority records the result in the character's progression ledger:
   - **Victory:** adds the encounter's XP and advances to the next encounter. After the last encounter, the run counter increases and the next run starts again at the first encounter. XP and cards carry over.
   - **Defeat or draw:** earns nothing; the same encounter is offered again.
4. The result screen shows the reward. **Continue · Spend XP & Next Battle** returns to the Campaign screen.
5. **Prepare · Deck & Abilities** opens the campaign's own deck screen (see below). Character Creation stays the admin editor for card trees, cards and abilities outside the campaign.

The client never names a reward amount. The battle session knows it is a campaign battle and which encounter it is. When the battle becomes terminal, it calls `loadout.RecordCampaignBattle`, which commits the XP and the campaign position in one atomic ledger write. Each battle ID is recorded once. A battle whose encounter is no longer the character's next encounter earns nothing.

## Prepare screen

`app/screens/campaign/campaign_prepare.gd` shows one campaign character and nothing else:

- **Left:** your deck (art, copies, energy, and how many upgrades are open), any stored cards, and your abilities. **+ Add a new card** is at the bottom.
- **Center:** select a deck card and its card tree appears. The card is highlighted, cards you hold glow gold, and cards you can reach from it pulse. Click another card you hold to switch to it; click any other card to preview it and see how it differs.
- **Right:** the selected card and what you can do with it:
  - **Upgrade one copy** (pay the XP difference) or **Trade one copy down** (get the difference back). The copy moves along a tree connection and stays in your deck, so health never changes.
  - **Buy another copy** (tree bases only), **Sell a copy** for its full XP, or **Store a copy** (−1 health, keeps its XP). Stored copies can be added back or sold.
- **Add a new card** lists every card-tree base with its card face and price. Buying one adds it to your deck.
- **Abilities** show the equipped tier's rules and the next or previous tier, with **Upgrade** or **Downgrade** (refund). Starter and Adventurer can upgrade Guard to Guard+ for 25 XP.

Unaffordable or illegal options stay visible but disabled, with the authority's reason ("Not enough XP.", copy limits, deck requirements).

The authority prepares this view with the `campaign_loadout` op. It dry-runs every offer through `loadout.PreviewPurchase`, the same rules `loadout.Buy` applies, so the screen never offers a trade the authority would refuse. Tree trades use the `tree_card_deck` purchase kind (`loadout.UpgradeDeckTreeCard`), which keeps the copy equipped. The older `tree_card` kind used by the Card Trees workshop moves the result into the collection.

## Card-tree cards only

Campaign decks may use only **card-tree cards**: tree bases, variants and shared cards in any published tree. Standalone and legacy cards (for example Blade Warden's Tip It) cannot be used, but any owned card can still be sold or stored for its XP.

- Prepare only offers card-tree bases and tree trades. A non-tree card in the deck is flagged, and selecting it offers only Sell and Store.
- Prepare's purchases send `tree_cards_only`, and the authority refuses buying, equipping or upgrading into a non-tree card.
- The campaign shares its deck with Progression mode, which can still equip legacy cards. When it does, the authority refuses to start a campaign battle and names the cards (`tree_conflicts` in the campaign status). The Campaign screen disables **Fight** and says what to sell or store.

Both campaign characters' starting decks are already all card-tree cards.

## Configuration

`dice-and-destiny-server/content/progression_v1/campaign.yaml`:

| Field | Meaning |
| --- | --- |
| `characters` | Characters that may enter. Each must have the General type, so campaign decks only use General cards and trees. The first is the default. |
| `encounters[].id` / `name` / `description` | Stable ID and the text shown on the path. |
| `encounters[].opponent` | A scripted minion combatant (one with `single_ability_policy`), such as `drowned_oracle_brine_mask`. |
| `encounters[].opponent_count` | 1 or 2 copies of that minion. |
| `encounters[].xp` | XP a victory awards (base-10 scale; a base card costs 10). |

The characters are Starter (the default) and Adventurer. The encounters are ordered by difficulty (see [Bell Diver and Ribbon Eel](enemies/bell-diver-and-ribbon-eel.md)):

| Encounter | Opponent | Victory XP |
| --- | --- | ---: |
| Tide Pool | Brine Mask | 20 |
| Sunken Belfry | Bell Diver | 25 |
| Kelp Narrows | Ribbon Eel | 30 |

The runtime pins one opponent per session, so the client re-initializes it with each encounter's minion (model key `minion:<definition>:<count>`). Another playable character also needs adding to the playable-character lists in `learned.CharacterCatalogs` and `Session.resetLoadout`.

## Ledger

The campaign uses the per-character progression ledger shared with Progression mode (`<loadout root>/progression/<character>.json`):

- `campaign`: next encounter, victories, defeats, runs completed, and recent recorded battle IDs.
- `earned_xp`: total battle rewards. An admin budget override is a total fixed at the time it is saved. Rewards earned later are added to it, just like free deck edits.

The `starting_xp` allowance in `economy.yaml` is still the new-character XP.

## Tests

- `internal/battle/learned/campaign_test.go` plays real campaign battles. It covers:
  - victory XP and advancing, including run wrap-around;
  - defeat or draw earning nothing;
  - out-of-order and non-General characters being refused;
  - duplicate recording being refused;
  - spending XP between battles changing the next battle's deck;
  - rewards surviving admin budget overrides.
- `internal/battle/learned/campaign_test.go` also checks that every Prepare offer matches the real purchase: available offers succeed with the shown XP change, and refused offers fail. It also checks that tree trades keep health.
- `tests/presentation/verify_campaign.gd` drives the loop with the pointer: menu → Campaign (Starter, with the Adventurer picker) → battle → reward screen → Continue → Prepare purchase → the next encounter uses the new deck.
- `tests/presentation/verify_campaign_prepare.gd` drives Prepare with the pointer:
  - select a card and see its tree;
  - preview a tree card;
  - upgrade and trade down in place;
  - buy a base, sell, store and re-add;
  - upgrade and sell back Guard;
  - fit at 1024, 1280 and 1920 widths.
