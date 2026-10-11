# Campaign progression

The campaign is the battle → XP → deck change → next battle loop. Open it from the main menu with **Campaign**.

## Campaign saves

Every campaign is a named **save slot**: the player's own character sheet, copied from a starting character sheet when the campaign begins. You can keep as many campaigns as you like, for the same or different characters.

- **New campaign:** choose a starting sheet (Starter or Adventurer), name it (the default is "Starter campaign", "Starter campaign 2", …) and **Begin campaign**. The save starts with the sheet's deck and ability board plus `bonus_xp` (100) to spend.
- **Continue** opens a save's encounter path. **Delete** removes it after a confirmation. Other campaigns and the original sheet are never affected.
- **The original sheet is never changed.** The character template, Character Creation's Progression and Sandbox saves, and admin budget overrides never touch a campaign save, and campaign play never touches them.
- **Card and ability definitions are not copied.** A save stores card and ability IDs, so catalog edits (repricing, rules changes) reach campaigns already in progress. A repriced card keeps the save's total XP and moves the difference into XP to spend. Locking definitions per campaign belongs to future content versioning (see `TODO.md`).

## Flow

1. **Campaign** opens the list of campaign saves and the New campaign panel. Continuing a save shows its health (equipped cards), XP to spend, run number and victories, and the encounter path, with each encounter marked Cleared, Next or Ahead. **All campaigns** returns to the list.
2. **Fight · <encounter>** starts the next encounter. It uses the save's deck and ability board, Seat A, and the unified defense rule.
3. When the battle ends, the authority records the result in the campaign save:
   - **Victory:** adds the encounter's XP and advances to the next encounter. After the last encounter, the run counter increases and the next run starts again at the first encounter. XP and cards carry over.
   - **Defeat or draw:** earns nothing; the same encounter is offered again.
4. The result screen shows the reward. **Continue · Spend XP & Next Battle** returns to the Campaign screen.
5. **Prepare · Deck & Abilities** opens the campaign's own deck screen (see below). Character Creation stays the admin editor for card trees, cards and abilities outside the campaign.

The client never names a reward amount. The battle session knows it is a campaign battle and which encounter it is. When the battle becomes terminal, it calls `loadout.RecordCampaignBattle`, which commits the XP and the campaign position in one atomic write to the save. Each battle ID is recorded once. A battle whose encounter is no longer the save's next encounter earns nothing.

## Prepare screen

`app/screens/campaign/campaign_prepare.gd` shows one campaign save and nothing else:

- **Left:** your deck (art, copies, energy, and how many upgrades are open), any stored cards, and your abilities. **+ Add a new card** is at the bottom.
- **Center:** select a deck card and its card tree appears. The card is highlighted, cards you hold glow gold, and cards you can reach from it pulse. Click another card you hold to switch to it; click any other card to preview it and see how it differs.
- **Right:** the selected card and what you can do with it:
  - **Upgrade** (pay the XP difference) or **Trade down** (get the difference back). Copies move along a tree connection and stay in your deck, so health never changes. With more than one copy that could go, a prompt asks how many: 1 up to "All N", defaulting to all, with the total XP shown. Cancel and Esc spend nothing.
  - **Buy another copy** (tree bases only), **Sell a copy** for its full XP, or **Store a copy** (−1 health, keeps its XP). Stored copies can be added back or sold.
- **Add a new card** lists every card-tree base with its card face and price. Buying one adds it to your deck.
- **Abilities** show the equipped tier's rules and the next or previous tier, with **Upgrade** or **Downgrade** (refund). Starter and Adventurer can upgrade Guard to Guard+ for 25 XP.

Unaffordable or illegal options stay visible but disabled, with the authority's reason ("Not enough XP.", copy limits, deck requirements).

The authority prepares this view with the `campaign_loadout` op. It dry-runs every offer through `loadout.PreviewPurchase`, the same rules `loadout.Buy` applies, so the screen never offers a trade the authority would refuse. Tree trades use the `tree_card_deck` purchase kind (`loadout.UpgradeDeckTreeCard`), which keeps the copy equipped. Its `count` trades several copies in one all-or-nothing purchase, with `expected_cost` the batch total; other kinds refuse a count. Each trade offer carries `max_count`: the largest batch the authority would accept now, given copies held, XP, copy limits and deck rules. The older `tree_card` kind used by the Card Trees workshop moves the result into the collection.

## Card-tree cards only

Campaign decks may use only **card-tree cards**: tree bases, variants and shared cards in any published tree. Standalone and legacy cards (for example Blade Warden's Tip It) cannot be used, but any owned card can still be sold or stored for its XP.

- Prepare only offers card-tree bases and tree trades. A non-tree card in the deck is flagged, and selecting it offers only Sell and Store.
- Prepare's purchases send `tree_cards_only`, and the authority refuses buying, equipping or upgrading into a non-tree card.
- `campaign_purchase` always applies `tree_cards_only`, so no client can add a non-tree card to a campaign save.
- A save that still holds a non-tree card (for example one converted from older progress) cannot start a battle. The authority names the cards (`tree_conflicts` in the campaign status), and the Campaign screen disables **Fight** and says what to sell or store.

Both campaign characters' starting decks are already all card-tree cards.

## Configuration

`dice-and-destiny-server/content/progression_v1/campaign.yaml`:

| Field | Meaning |
| --- | --- |
| `characters` | Starting sheets a new campaign can copy. Each must have the General type, so campaign decks only use General cards and trees. The first is the default. |
| `bonus_xp` | XP a new campaign save starts with, on top of the sheet's deck and abilities (100). |
| `encounters[].id` / `name` / `description` | Stable ID and the text shown on the path. |
| `encounters[].opponent` | A scripted minion combatant (one with `single_ability_policy`), such as `drowned_oracle_brine_mask`. |
| `encounters[].opponent_count` | 1 or 2 copies of that minion. |
| `encounters[].xp` | XP a victory awards (base-10 scale; a base card costs 10). |

The starting sheets are Starter (the default) and Adventurer. The encounters are ordered by difficulty (see [Bell Diver and Ribbon Eel](enemies/bell-diver-and-ribbon-eel.md)):

| Encounter | Opponent | Victory XP |
| --- | --- | ---: |
| Tide Pool | Brine Mask | 20 |
| Sunken Belfry | Bell Diver | 25 |
| Kelp Narrows | Ribbon Eel | 30 |

The runtime pins one opponent per session, so the client re-initializes it with each encounter's minion (model key `minion:<definition>:<count>`). Another playable character also needs adding to the playable-character lists in `learned.CharacterCatalogs` and `Session.resetLoadout`.

## Storage

Campaign saves live in `<loadout root>/campaigns/<save id>.json` (`loadout.CampaignSave`). Each holds its name, character, creation time and `sheet`: deck, stored cards, ability board, XP and budget, `earned_xp`, and `campaign` (next encounter, victories, defeats, runs completed, and recent recorded battle IDs). Save IDs are `<character>-<UTC timestamp>` and are checked against a strict pattern, so a request cannot reach outside the folder.

Runtime ops:

| Op | Does |
| --- | --- |
| `campaign_status` | Lists encounters, starting sheets and saves |
| `campaign_new` | Starts a save from a sheet |
| `campaign_delete` | Deletes a save |
| `campaign_loadout` | Returns Prepare's view of a save |
| `campaign_purchase` | Applies one Prepare trade |
| `reset` | With `encounter` and `campaign_save`, starts a save's battle |

**Converting older progress:** before campaigns had their own saves, campaign progress lived in the character's progression save (`progression/<character>.json`). The first `campaign_status` converts any such progress into a campaign save named "<Character> campaign" (with `migrated_from`), copying its deck, stored cards, abilities, XP and position, then clears the `campaign` field from the progression save.

## Tests

- `internal/battle/learned/campaign_test.go` plays real campaign battles. It covers:
  - victory XP and advancing, including run wrap-around;
  - defeat or draw earning nothing;
  - out-of-order and non-General characters being refused;
  - duplicate recording being refused;
  - spending XP between battles changing the next battle's deck;
  - rewards surviving admin budget overrides.
- `internal/battle/learned/campaign_test.go` also checks that every Prepare offer matches the real purchase: available offers succeed with the shown XP change, and refused offers fail. It also checks that tree trades keep health.
- `internal/battle/learned/campaign_test.go` also checks:
  - saves are independent copies: trading in one changes no other save, the progression save or the template;
  - admin budget overrides do not reach saves;
  - deletion removes only that save, and save IDs cannot escape the folder;
  - older progress converts into a save once.
  - batched tree trades: the full batch works, batches beyond the copies held or the XP available are refused without changes, and `max_count` matches the limit.
- `tests/presentation/verify_campaign.gd` drives the loop with the pointer:
  - menu → Campaign → New campaign (Starter, default name) → a second campaign from the Adventurer sheet;
  - Progression-mode purchases do not reach the campaign;
  - delete with confirm and cancel;
  - Continue → battle → reward screen → Continue → Prepare purchase → the next encounter uses the new deck.
- `tests/presentation/verify_campaign_prepare.gd` drives Prepare with the pointer:
  - select a card and see its tree;
  - preview a tree card;
  - the count prompt (defaults to all, Cancel, Esc, choose one);
  - upgrade and trade down in place, and upgrade both Try Again copies in one go;
  - buy a base, sell, store and re-add;
  - upgrade and sell back Guard;
  - fit at 1024, 1280 and 1920 widths.
