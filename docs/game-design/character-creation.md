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

- **Deck & Library** opens by default, with the card library on the left and
  the current deck on the right. Both lists remain visible, have independent
  searches and scroll positions, and share the inspector. **Swap sides** exchanges
  their positions without losing filters, scrolling, selection, or draft changes.
  Drag the middle divider to change their widths.
- The deck pane shows equipped cards and quantities. Select a card and edit **Copies in
  deck**, or use **Add a copy**. Zero removes it from the draft.
- The library pane searches all cards in the selected character’s validated content
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

The Sandbox mode retains free deck editing. Dice remain inspection-only.

## Phase 3: XP purchases and upgrades (implemented)

Choose **Progression · XP** at the top of Character Creation, or **Progression · XP
purchases** in battle setup. Sandbox and Progression have independent loadouts.
Progression begins with the character template plus any configured starter
loadout overrides. It does not import a freely edited Sandbox deck.

- Each character has an authoritative XP balance. The current development
  allowance is **100 XP**, granted once on initial progression creation. Reopening
  the screen, reloading definitions, changing modes, and restarting battles do not
  refill it. Victory rewards are still the next phase.
- Progression deck/library rows and the inspector show each card's unit price,
  owned quantity, and total value, for example **10 XP × 3 = 30 XP total**.
  Unowned library cards show the unit price with a zero owned total. Values refresh
  after trades, upgrades, and admin price changes. Ability rows show their total
  configured upgrade-path cost; starter abilities currently have no base XP charge
  and are labeled **0 XP total · included starter ability**. Guard+ shows 25 XP.
  For converging future upgrade paths, the displayed configured value is the least
  expensive path, rather than a historical spending receipt. Sandbox stays free.
- The default card price is **10 XP**; per-character overrides are supported.
  Buying a copy immediately equips it and increases maximum health by one.
- **Sell a copy** removes one equipped card and refunds its current configured
  purchase price. Starter cards and upgraded cards can both be sold; Brace+
  currently sells for 20 XP. Buying and selling use the same price configuration.
- Starter cards represent an already invested card budget, in addition to the
  initial XP allowance. A fresh Adventurer has **100 available XP + 120 XP in
  starter cards = 220 XP**. The summary displays available XP, current deck value,
  and their combined card budget. Selling all starter cards makes all 220 XP
  available; this does not grant extra XP or reset an existing balance.
- An empty Progression deck can be saved while rebuilding. Battle setup disables
  Start Battle and explains that at least one card is required. The authority
  also rejects an empty deck without replacing the current battle. Sandbox keeps
  its existing minimum of one card when applying a deck.
- Hovering an upgrade button (or focusing it with the keyboard) opens a
  side-by-side comparison beside the inspector. Current wording is marked in
  gold where it changes; upgraded wording is marked in green. Unchanged wording
  stays neutral. Full rules, energy, card timing/destination, ability qualification
  tiers and usage limits are included. Each column scrolls independently for long
  definitions. Disabled upgrades still show the comparison. Downgrade buttons
  also compare both tiers. Clicking opens the existing transaction review and
  dismisses the hover; changing entries clears it. Hovering never changes a save.
- Upgrading a card replaces exactly one owned copy and preserves health.
  The Adventurer example is **Brace → Brace+ for 10 XP**. Buying an additional
  Brace+ directly costs **20 XP**.
- Upgrading an ability replaces its existing offensive/defensive slot. The first
  configured example is **Guard → Guard+ for 25 XP**. Progression Adventurer starts
  with three base Braces and only base Guard so these upgrades can be tested.
  Its Sandbox starter remains unchanged. Other characters can buy cards; they
  show no further upgrade until paths are authored for their abilities/cards.
- Upgraded abilities also offer **Downgrade → previous tier · +N XP**. This
  reverses a configured ability-upgrade edge, restores the previous ability in
  the same slot, and refunds that edge's configured XP cost. Guard+ → Guard
  currently returns 25 XP. The preview shows both rules and the resulting balance;
  cancelling changes nothing. Health and total budget stay unchanged, while
  upgrade investment becomes available XP again. The restored tier can be
  upgraded again. Every downgrade requires an equipped upgraded tier, a valid
  configured reverse path, sufficient invested upgrade XP, and a current quote
  and revision. It cannot remove an ability slot or refund twice. Ability tier
  changes retain their review prompt, independently of card buy/sell preferences.
- Selecting a purchase or sale opens a preview of XP before/after, health
  before/after, and full before/after rules for upgrades. Cancel changes nothing.
  Confirm saves immediately; there is no Apply step in Progression.
- Card buy and sell previews each offer **Do not show again**. After a successful
  confirmation, future transactions of that type happen directly. **Confirm buys**
  and **Confirm sales** below the deck lists independently restore or skip these
  prompts. Preferences apply across characters and persist in the workspace's
  `character_preferences.cfg`; cancelling a preview does not change them. Upgrade
  reviews remain enabled. Direct trades use the same authority validation and
  immediate saves as reviewed trades.
- Transactions validate ownership, source/target definitions, ability type, count
  limits, XP, quoted price, and save revision in the authority. A stale or repeated
  transaction cannot spend or refund twice. XP, deck, ability board, and revision are stored in
  one atomic save. Errors never partially spend or equip an upgrade.
- Progression saves live under `user/character_loadouts/progression/<character>.json`
  in the launcher-isolated runtime root. The separate file includes XP, deck,
  ability board, and revision. Existing Sandbox saves need no migration.
- New battles/rematches use the selected mode. Deck and ability-board overrides
  affect only the human participant and are recorded in replays. Existing battles
  retain their starting loadout. Free Sandbox edits cannot fund Progression.

### Admin economy controls

In Progression mode, **Admin settings** opens a searchable card-price list and
editable total budgets for all four characters. Card overrides apply globally by
card ID, with the same price for buying and selling. The panel previews each
character's deck value, upgrade investment, and available XP. **Apply economy
changes** validates every character and saves the changes together; closing the
panel discards its edits.

The invariant is **total budget = available XP + current deck value + upgrade
investment**. Repricing cards preserves the total budget and changes available
XP by the opposite amount. For example, setting Adventurer's budget to 260 and
its three starter Braces to 12 XP yields 126 XP in the deck and 134 XP available.
A price reduction releases XP; increasing a budget from 220 to 260 adds 40 XP.
Ability upgrade spending remains invested. Card upgrade costs are at least the
increase in card value, or the authored upgrade price if higher, so repricing
cannot create free XP through upgrading and selling.

Changes that would leave any character with negative available XP are rejected
before settings are committed. Raise the affected budget in the same edit, or
close and sell cards first. Cards and abilities are never silently removed.
Existing active battles keep their pinned loadouts.

These are local administrator tools for this workspace. Overrides persist in
`user/character_loadouts/economy_admin.json`, separate from authored YAML and
Sandbox decks. A single atomic settings replacement is the commit point; every
progression read, trade, and battle start reconciles its ledger against that
settings revision. The response refreshes all characters immediately, and an
interrupted refresh is safely completed on the next read. Stale admin edits and
stale trades are rejected. Existing saves acquire a budget ledger while preserving
their current XP and cards; authored ability-upgrade costs are recovered for older
saves without spending history. The admin budget can explicitly correct historical
balances. Reloading never reapplies the same budget adjustment twice.

### Configuring the economy

Edit `dice-and-destiny-server/content/progression_v1/economy.yaml`. No code changes
are needed for supported prices, allowances, and upgrade paths:

```yaml
schema_version: 1
starting_xp: 100
default_card_price: 10
characters:
  adventurer:
    card_prices: {brace_plus: 20}
    card_upgrades:
      brace: {to: brace_plus, xp: 10}
    ability_upgrades:
      adventurer_guard: {to: adventurer_guard_plus, xp: 25}
```

Optional `starting_decklist` and `starting_abilities` override the initial
Progression loadout. Starting values apply only when the save is first created;
changing them never overwrites existing progression. Prices/paths are reread for
catalog reads, purchases, and sales. Use Reload definitions to refresh the UI. A transaction
quoted before a price change is rejected until refreshed. Unknown configuration
keys, missing definitions, invalid prices, and mismatched ability types are errors.
New mechanical effects still need engine support; links between existing authored
cards/abilities and economy values are configuration-only.

This phase exposes the character's supported card catalog. Discovery-based
availability, reward XP, unequipped inventory, and branching run
progression remain future work.

## Next phases

1. **Battle rewards and discovery.** Victory awards XP and unlocks potential
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

The split-view checks exercise simultaneous visibility, separate searches and
scrolling, inspection from either pane, live quantity updates in both lists,
swapping without losing context, pointer dragging of the divider, and keeping
both panes and the inspector inside the viewport at all three tested widths.

Phase 3 native tests cover one-time grants for all characters, sandbox isolation,
card and ability upgrades, atomic failures, stale prices/revisions, concurrent
requests, configuration-only price changes, both human seats, complete battles,
and replayed upgraded ability boards. `verify_xp_progression.gd` tests purchase
previews/cancellation, pointer purchases, XP/health updates, persisted mode changes,
insufficient XP, three viewport sizes, and a real menu-launched progression battle.

Sales tests cover selling every starter card for all four characters, empty-deck
persistence, rebuilding and completing battles, configured upgraded-card prices,
unowned-card rejection, stale quotes/revisions, and concurrent sale requests.
`verify_xp_sales.gd` validates sale previews/cancellation, pointer sales and
buybacks, the full 220 XP Adventurer budget, empty-deck menu restrictions,
Sandbox/other-character isolation, reopening, three viewport sizes, and buying
back a card before starting a real battle.

`verify_transaction_preferences.gd` covers independent buy/sell opt-outs,
cancellation, disk persistence, restoring prompts, retained upgrade reviews,
direct trades with fresh revisions, XP/ownership limits, and dialog/toggle layout
at 1024, 1280, and 1920 widths.

Admin economy tests cover all-character revaluation, higher/lower prices, budget
increases/reductions, stale commands, invalid/over-budget updates, migration with
an upgraded ability, and repeated reads. `verify_economy_admin.gd` enters prices
and budgets through the UI, checks previews, cancellation and reopening, trades
at the updated prices, verifies global overrides and persistence, and captures
layouts at three window widths.

Ability downgrade checks cover refunds with zero available XP, repeated/invalid
refund rejection, persisted boards and budget totals, repurchasing, unchanged
active battles, and completed battles using the restored tier from both seats.
`verify_ability_downgrades.gd` exercises pointer upgrade/downgrade/cancellation,
full before/after rules, reopening, and three viewport sizes.

`verify_upgrade_comparisons.gd` validates pointer hover and keyboard focus,
changed-word highlighting, full card/ability rules, disabled buttons, placement
at three sizes, mouse scrolling, stale-preview dismissal, and successful card
and ability purchases after hovering. Hover comparisons are read-only.

`verify_entry_xp_totals.gd` validates per-copy and quantity totals in the deck,
library, and inspector, zero-owned cards, updated prices after admin edits,
upgrade/downgrade values, and containment of the added labels at three sizes.
