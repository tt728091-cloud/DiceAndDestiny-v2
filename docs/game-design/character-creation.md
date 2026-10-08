# Character Creation

## Inspect the configured loadout (implemented)

Open **Character Creation** beside **Start Battle** in battle setup. Choose
Adventurer, Venom, Curse, or Blade Warden in the left rail. The initial selection
matches battle setup; closing the viewer preserves the setup selections.

- The roster shows each character's portrait; the selected character stands in a
  framed showcase below it.
- The header shows stat tiles: health (the sum of decklist quantities, with total
  and unique counts), available XP in Progression, starting energy and income,
  and opening hand, hand limit and draw. Type chips show the card pools.
- Abilities shows the board as Offense and Defense columns. Click to inspect full
  rules, qualification tiers, cost, usage limits, and saved-card destination.
- Deck rows show card art, an energy pip, the copy count and quick **+**/**−**
  buttons. Search the current deck by name or rules. Click a card for its battle
  card preview, copy controls, full rules, play-window chips and destination.
  An energy curve beside the deck heading counts copies at 0, 1–4, 5, 6–9 and 10+ energy.
- Dice shows each die's faces as tiles, plus how often each symbol appears across
  every equipped face.
- The last three tabs carry a launch icon: they open full-screen workshops.
- Back or Escape returns to setup. Reload definitions rereads the files without
  starting a battle or touching saved player data.

The native `character_catalogs` request initializes or migrates owned loadout
records as needed. It does not replace a learned session, draw cards, create a
battle, or write battle history.
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
- The deck pane shows equipped cards and quantities. Use a row's **+**/**−** to add or
  remove one copy in place (in Progression these open the usual buy/sell review), or
  select a card and edit **Copies in deck**, **−**, or **+ Add a copy**. Zero removes
  it from the draft.
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
  `user/character_loadouts/progression/<character>.json`, shared by both editing
  modes. Content YAML remains unchanged. Corrupt records display an error;
  they are never silently overwritten or granted replacement XP.
- Battle setup displays the saved health total. New battles and rematches read the
  saved deck for the human seat only, even in a same-character mirror matchup.
  Opening draws, card instances, and maximum health use that deck. Active battles
  retain their pinned setup. Replay records include the starting deck, so later
  edits cannot change a replay. Sessions without a loadout root keep templates.

The Sandbox mode retains free deck editing. Dice remain inspection-only.

## Phase 3: XP purchases and upgrades (implemented)

Choose **Progression · XP** at the top of Character Creation, or **Progression · XP
purchases** in battle setup. Both modes use one saved deck per character.
Sandbox applies free deck edits; Progression buys, sells, and upgrades that same
deck. Switching modes changes the editing controls, not the equipped cards.
Both begin with the combatant template (Adventurer: two Brace and one Brace+).

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
  initial XP allowance. A fresh Adventurer has **100 available XP + 130 XP in
  starter cards = 230 XP**. The summary displays available XP, current deck value,
  and their combined card budget. Selling all starter cards makes all 230 XP
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
  with only base Guard so the ability upgrade can be tested. Card decks are
  shared; the existing mode-specific ability boards remain separate. Other characters can buy cards; they
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
  in the launcher-isolated runtime root. This is the canonical deck for both
  modes and includes XP, ability board, and revision. Old separate saves migrate
  once: the most recently saved deck wins when both exist. The retired unedited
  Adventurer Progression starter is corrected to two Brace and one Brace+.
  Customized decks, available XP, and purchased ability investment are preserved.
  Legacy Sandbox files remain on disk as archives and cannot overwrite later edits.
- New battles/rematches use the selected mode. Deck and ability-board overrides
  affect only the human participant and are recorded in replays. Existing battles
  retain their starting loadout. Sandbox edits preserve available XP by adjusting
  the deck allocation and total budget by the deck-value difference. These freely
  added cards can subsequently be sold in Progression; Sandbox is a free editor.
  Admin budget changes record the included free allocation so later reads cannot
  credit it twice. Selling the entire shared deck blocks battle start in both modes.

### Admin economy controls

In Progression mode, **Admin settings** opens searchable Cards and
Abilities tabs with pool-type selectors, card prices, and editable character
types and total Progression budgets for all four characters. Card overrides apply globally by
card ID, with the same price for buying and selling. The panel previews each
character's deck value, upgrade investment, and available XP. **Apply economy
changes** validates every character and saves the changes together; closing the
panel discards its edits.

The invariant is **total budget = available XP + current deck value + upgrade
investment**. Repricing cards preserves the total budget and changes available
XP by the opposite amount. For example, setting Adventurer's budget to 260 and
its two starter Braces to 12 XP yields 134 XP in the deck and 126 XP available.
A price reduction releases XP; increasing a budget from 220 to 260 adds 40 XP.
Ability upgrade spending remains invested. Card upgrade costs are at least the
increase in card value, or the authored upgrade price if higher, so repricing
cannot create free XP through upgrading and selling.

Changes that would leave any character with negative available XP are rejected
before settings are committed. Raise the affected budget in the same edit, or
close and sell cards first. Cards and abilities are never silently removed.
Existing active battles keep their pinned loadouts.

Admin also includes a central **Item preview** column. Hover an item name or
its edit controls, or focus a control with the keyboard, to inspect it. Cards
use the same standard-size artwork and full rules as the character inspector;
abilities show rules, dice requirements, tier results, usage, and prevention
destination. The preview remains available while reading or scrolling, updates
with draft price/type changes, and identifies eligibility for the selected
character. Previewing never changes the draft. Tab changes and searches clear
hidden selections. `verify_admin_item_preview.gd` covers these interactions,
all four characters, save/reopen, and three viewport sizes.

Both Admin item tabs offer **Name · A–Z** and **Type → Name · A–Z** sorting.
The latter groups items by their current draft type, with alphabetical names
inside each labeled group. The adjacent type filter can show just Venom,
Curse, General, or another configured type, and works together with search.
Changing an item's type immediately updates its group/filter membership.
Sorting and filtering preserve unsaved edits; browse choices survive tab
changes and reopening Admin within the character screen.
`verify_admin_sorting.gd` checks both lists, combined filtering/search,
draft edits, save/reopen, previews, and three viewport sizes.

These are administrator tools. Card prices and card, ability and character
types are catalog-wide and persist in the tracked
`dice-and-destiny-server/content/authored/economy_admin.json`, so they can be
committed with the code. Budgets are reconciled against this player's ledgers
and stay in `user/character_loadouts/economy_admin.json`; a budget-only edit
never changes the tracked file. Both are separate from authored YAML and
Sandbox decks. The admin revision is the sum of both files' revisions; every
progression read, trade, and battle start reconciles its ledger against that
settings revision. The response refreshes all characters immediately, and an
interrupted refresh is safely completed on the next read. Stale admin edits and
stale trades are rejected. Existing saves acquire a budget ledger while preserving
their current XP and cards; authored ability-upgrade costs are recovered for older
saves without spending history. The admin budget can explicitly correct historical
balances. Reloading never reapplies the same budget adjustment twice.

### Character and item pool types

Every character has one configured type, and every card and ability belongs to
one pool. **General** is always available; other pools require the character's
matching type. The initial types are General (Adventurer), Venom, Curse, and
Blade Warden. Venom/Curse pack cards and abilities use their respective types;
Alchemist's Gamble is Venom. Existing Blade Warden abilities retain the Blade
Warden type (including its authored Venom Strike). Generic cards and Adventurer
abilities are General. Pool type is distinct from card timing or offensive /
defensive ability kind.

The library shows eligible cards, while owned decks retain incompatible items
with a visible warning. Buying and upgrading check the target type in the native
authority. Sandbox deck saves and new battles also enforce types. Existing
battles keep their pinned definitions. Admin reassignment never deletes owned
items or changes their XP value: a conflicting card remains sellable, and a
conflicting ability or card blocks battle until its type/character type is
corrected (or the card removed). Admin previews show affected item counts.

In **Admin settings**, use **Cards · price & type** or **Abilities · type** to
search for an item and select its pool. Character selectors appear beside their
budgets. **Apply economy changes** saves all overrides atomically; Close discards
them. Changes affect both Sandbox and Progression eligibility. Changing an item
to General makes it available to every character regardless of its source pack;
custom battles load and pin all required character-pack definitions.

Authored assignments live in the same `progression_v1/economy.yaml` configuration:

```yaml
access:
  types: {general: General, venom: Venom, curse: Curse, blade_warden: Blade Warden}
  character_types: {adventurer: general, venom: venom, curse: curse, blade_warden: blade_warden}
  card_types: {pinprick: venom, black_fingerprint: curse}
  ability_types: {needlefang: venom, hexbrand: curse}
```

Omitted assignments default to General. New types can be added to `access.types`
and assigned here without code changes. Admin dropdowns use that registry;
overrides are stored in `economy_admin.json` under `card_types`, `ability_types`,
and `character_types`, and take precedence over authored values. Unknown types
and definition IDs are rejected on save. This adds eligibility rules to the
existing ability upgrade system; a general-purpose ability library/equipment
editor remains a later feature.

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

Phase 3 native tests cover one-time grants for all characters, shared-deck migration and mode switching,
card and ability upgrades, atomic failures, stale prices/revisions, concurrent
requests, configuration-only price changes, both human seats, complete battles,
and replayed upgraded ability boards. `verify_xp_progression.gd` tests purchase
previews/cancellation, pointer purchases, XP/health updates, persisted mode changes,
insufficient XP, three viewport sizes, and a real menu-launched progression battle.

Sales tests cover selling every starter card for all four characters, empty-deck
persistence, rebuilding and completing battles, configured upgraded-card prices,
unowned-card rejection, stale quotes/revisions, and concurrent sale requests.
`verify_xp_sales.gd` validates sale previews/cancellation, pointer sales and
buybacks, the full 230 XP Adventurer budget, empty-deck menu restrictions,
shared empty-deck restrictions and other-character isolation, reopening, three viewport sizes, and buying
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

The admin dialog uses a viewport-bounded scroll container so content height
changes cannot push the dialog off-screen behind its dimmer. Each opening
detaches the previous dialog before rebuilding controls from current settings.
`verify_admin_reopening.gd` checks 24 consecutive save/reopen cycles across four
window sizes (including 3456×2048), with Guard+ equipped and Brace prices
alternating between 10 and 11 XP. It also checks layout after validation content
grows and clears, closing/canceling, and keyboard dismissal.

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

`verify_access_types.gd` validates library filtering, moving cards between pools,
General access, character-type overrides, ability upgrade restrictions, preserved
owned conflicts, cancellation, persistence, and admin layouts at three sizes.
Native tests also reject forged purchases/Sandbox saves, validate battle-start
restrictions, and load cross-pack General cards into real battles.

## General card expansion

The `content/general_v1/cards` extension adds eight optional General cards to all
four character libraries. Starter decks are unchanged. Add them with the existing
Sandbox editor or buy them in Progression; each starts at the configured default
price of 10 XP and can be repriced/retyped in Admin settings. Reload definitions
refreshes the library; new battles pin the updated catalog and owned loadout.

| Card | Energy | Timing | Effect |
| --- | ---: | --- | --- |
| Matchmaker | 1 | Offense, after | Set one owned offensive die to another owned die's face. |
| Turn the Die | 1 | Offense, after | Flip one owned offensive die: 1↔6, 2↔5, 3↔4. |
| Disrupt | 1 | Offense, reaction only | Reroll one revealed enemy offensive die; recheck the selected attack's qualification. |
| Second Guard | 1 | Defense, roll review only | Reroll chosen dice of that defense before it applies. Its rewards resolve once. |
| Reclaim | 2 | Offense, any time | Return one discarded non-recovery card to hand. Does not restore removed cards or heal. |
| Reinforce | 1, optionally 2 | Defense, any time | Prevent 2 from one source, or pay 1 extra energy to prevent 4. |
| Dispel | 1 | Offense and Defense, any time | Remove one enemy positive status stack, unless its definition is `dispel_immune`. |
| Triage | 1 | Defense, any time | Prevent 1 from one source and save a specific revealed threatened card from that source, including against overage. Other sources can still threaten it. |

These cards, like the Adventurer starter cards (Brace, Brace+, Nudge, Try Again,
Strong Swing, Take Stock, Second Wind), are program cards: their effects,
per-effect saved-card destinations and Before / After / Any time timing live in
`program`, and their rules text and `Play: …` line are generated from it. Reinforce
and Triage save to `original`. Playing a prevention card still sends it to its
play destination; saving it never rewinds its play. Triage protects the selected
reservation against that source for this damage batch, persisting through
reconciliation and save/reload; it does not grant blanket immunity. Reclaim
excludes other recovery cards, avoiding repeatable recovery loops, and respects
existing pending damage reservations.

The previous `general_choice` definitions remain supported by the engine for
pinned battles and replays; `internal/battle/engine/testdata/general_v1_legacy`
keeps their regression coverage. `shipped_general_cards_test.go` plays each
shipped card through the program handler; `verify_general_cards.gd` covers the UI.
The pack is separate from frozen training content (`battle_v1`), so the enemy AI's
content inputs are unchanged.

## Card Creation: configurable General cards

**Card Creation** is a tab in the character creator and opens a full-width
workspace. It edits or clones all 21 current General cards. Choose a template
and **Edit template** or **Create a copy**. The latter supplies a new ID and name.
Edit the Card, Effects & Choices, and Upgrades tabs, then **Validate and preview**
and **Publish for future battles**. The right side renders a live battle card of the
draft. Card fields are grouped into Identity (with an artwork picker), Cost &
economy, Playing the card and Play windows; each effect is a numbered panel with its
target and values in two-column forms. **Open in character deck** selects the published
card in the existing deck editor, where Sandbox additions or Progression
purchases use the usual save, XP, and copy-limit rules. The shared library exposes published cards in both
Sandbox and Progression. Publishing does not equip a card automatically.

The native authority returns a versioned capability registry. The form uses it
for effect parameters and validation; generated rules use those same values.
Ordered effects can be added, removed and reordered. Guided controls also
cover nested named choices, additional option costs, dice conditions, target
filters, and upgrade branches. Optional advanced JSON uses the same validator
and is validated before replacing the form's draft. Live validation refreshes
the rules preview without disconnecting the controls from the draft. Unpublished
changes are guarded when closing, switching templates, or creating another copy. Publishing is atomic and rejects stale catalog revisions and
changes that would invalidate an existing saved deck or exceed its XP budget.

### Compatibility guidance

The effect and target registries drive both authority validation and the form:

- Only compatible effects and play windows are offered together. Nested options
  participate in that compatibility calculation. Window changes caused by an
  effect edit are reported visibly.
- Owned-card movement cannot target enemy private piles or recover removed cards.
  Defensive rerolls target only the current player's unfinished defensive dice.
  Enemy offensive dice require the revealed offensive-reaction window.
- A reroll of your own offensive dice cannot be restricted to before the first
  roll. A qualified-ability filter likewise requires a roll. Exact self-character
  targeting cannot ask for more than one character.
- Sacrifice is locked to chosen, exact, owned-card targeting. Its sequence position
  cannot be moved below a reward, and a second sacrifice cannot be added to the
  same sequence. Nested option costs remain supported.
- Drawn-this-play filters require an earlier draw for yourself, including draws
  inherited by a nested option. Conditions expose only fields for their selected
  requirement type; incompatible bounds are rejected.
- The form shows target count only for exact/up-to selection, rounds only for
  N-round durations, and on-hit status stacks only when a status is selected.
  Die bounds, sell prices, and upgrade prices constrain related numeric inputs.
- Every supported effect has a guided default. Empty effect sequences, missing
  windows, duplicate names, stale revisions, invalid IDs/references, excessive
  nesting and invalid saved-deck changes still fail authoritative publication.

### Configurable values

| Area | Supported values |
| --- | --- |
| Identity / presentation | ID, name, illustration path; generated summary and rules |
| Costs / economy | Energy, buy XP, sell XP, copy limit 1–100, branching upgrade IDs and XP |
| Play | Hand/deck/discard source piles; hand/deck/discard/removed play destination |
| Timing | Offensive planning/reaction, defense selection/reaction, damage reaction; before/after first roll or unrestricted; per-round/per-battle uses (0 = unlimited) |
| Targets | Self/enemy/any where eligible; one/exact N/up to N/all; chosen or random |
| Draw / energy | Amount and recipients; ordinary draws never recycle discard |
| Prevention | Amount or individually chosen threatened cards; original/discard/hand/deck/removed destination |
| Movement / sacrifice | Source piles, destination, quantity; exclude recovery cards/definition IDs; cards drawn during this play |
| Dice | Chosen allowed faces; configurable adjustment amounts and bounds/wrapping; opposite-face sum; copy another die; reroll with replace/higher/lower result; optionally spend one offensive roll attempt |
| Status | Existing status ID, recipient, stacks; removal quantity and positive/negative/ID filters |
| Ability preparation | Damage and/or an existing on-hit status; selected abilities; dice qualification conditions; stack cap and stack/replace/refresh policy |
| Duration | Offensive exit, end of current round's damage, next qualifying use, N rounds including the current round, or battle end |
| Composition | Ordered effects; named options with extra energy and their own ordered effects; conditions use the existing structured dice requirements |

General templates: Antidote, Battle Focus, Brace, Brace+, Dispel, Disrupt,
Emergency Ward, Loaded Die, Matchmaker, Nudge, Reclaim, Reinforce, Second Guard,
Second Wind, Sharpen Blade, Strong Swing, Take Stock, Tip It, Triage, Try Again,
and Turn the Die. Conversion follows their operation definitions; executing a
program does not branch on the card's name or ID.

### Rules and deliberate boundaries

- Programs use version 1. An effect sequence has 1–32 steps, choices have 1–16
  options, and nesting is limited to four levels. Amounts and target counts are
  bounded. Unknown fields, effects, parameters and incompatible windows are
  rejected instead of silently ignored. A genuinely new mechanic still needs
  one engine capability; creating more cards from these capabilities does not.
- Card-pile selection is restricted to owned live cards; enemy dice must be
  revealed. Removed cards cannot be recovered. Movement never duplicates cards.
- Sacrifice is an exact, chosen, unconditional cost at the start of a sequence,
  with one sacrifice step per sequence. It excludes the played card. Selecting
  targets does not pay a partial cost; after all targets are selected, removal
  precedes rewards. Every removed card loses health and appears in wound history.
  The played card itself may separately have `removed` as its play destination.
- `original` means the live pile, not a previous reservation's pile. Explicit
  `removed` destinations permanently remove saved cards: they avoid that attack's
  removal but still lose health from the authored removal effect. Saved-card
  feedback follows the live destination; already removed cards never return.
- Ability bonuses have visible authoritative preparation statuses. Dispelling,
  consuming or expiring stacks removes the corresponding bonus. Conditions on
  an ability bonus are checked when the attack resolves. Other step conditions
  are checked when the step is reached; a failed condition skips that step.
- Exact targeting requires the full count; if a later effect cannot find that
  many eligible targets, it has no effect. Up-to targeting can stop early.
  Multi-die selections count dice, not alternative face choices for one die.
- Sell prices cannot exceed buy prices. Upgrade XP is at least the increase in
  deck value, preserving the existing shared XP-budget invariant. Existing Admin
  price overrides take precedence over a Workshop buy price; Workshop sale
  prices remain independently authored.
- In battle, selecting an authored card opens its legal choices in the existing
  scrolling action rail. The card remains highlighted in hand; no modal selector
  is added. The authority rebuilds legal choices after each effect and rejects
  stale input. Pending selections and ordered execution survive save/reload.
- Definitions live in the tracked
  `dice-and-destiny-server/content/authored/authored_cards.json`, with a
  revision; commit it to share published cards. Existing battles
  and replays retain their pinned definitions; future battles/rematches use the
  published revision. Legacy definitions keep their original execution path
  until explicitly revised in the Workshop.

### Verification

`card_program_test.go` covers ordered draw/selection, cancellation and stale
commands, save/reload, sacrifice/health/wounds, multi-target dice, die edits,
reroll policies, status effects, stack consumption, duration expiry, usage limits,
and all five prevention destinations from all three live piles in both flows.
`card_authoring_test.go` covers all General templates, publication/reload,
revisions, catalog pinning, economy branches and copy limits, rejected saved-deck
incompatibilities, and complete authored-deck battles against one and two minions.

The compatibility matrix exercises 1,920 owner/count-mode/selection/window
combinations, all 256 ordered effect pairs, enum choices, numeric boundaries,
nested dependencies and incompatible configurations.

Godot tests `verify_card_creation_guided.gd`, `verify_card_workshop.gd`, `verify_program_card_battle.gd`, and
`verify_workshop_progression.gd` cover every template and effect form, disabled conflicts, nested choices and
conditions, publication and deck handoff, real pointer card selection, renamed-card
in-hand reroll animation, prices and upgrade branches. The battle test creates
and equips its card through Card Creation and the character deck editor before
starting the real native battle.
Existing General-card, unified-defense and Progression checks remain regression
coverage. Run every Godot test through the repository's `scripts/godot.sh`.
