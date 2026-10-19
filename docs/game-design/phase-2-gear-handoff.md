# Phase 2-I handoff: gear, gates, training and the gold pass

**Status:** ready to implement (written 2026-10-19).
**Plan of record:** [`open-world-plan.html`](open-world-plan.html) §1 Principles, §8 "Progression: XP, gear, training", §10 "Gold". Phase 0 and Phase 1B/1C are merged (`9dcc4de`).
**Scope:** Phase 2 parts 2A (gear core), 2B (gates on card-tree nodes), 2C (training and proficiency), 2E (gold pass) and 2F (first gear content). **Potions (2D) are Phase 2-II, a separate handoff after this one is merged and played.** 1A (enemy budgets, the balance lab) is being done in other threads; don't touch it.

**Decisions the user made for this handoff (2026-10-19):**
- Phase 2 is split: this handoff is everything except potions.
- Gear applies to **Expedition saves only**. The linear campaign, the battle setup menu and the editors' progression saves stay unchanged.
- Gear trees are authored by **extending the card tree forge** (an admin workshop), stored in the tracked authored-content directory.
- Gear XP is priced with the **card-value-rules units now**; effects with no unit get placeholder prices, marked as such, to be re-priced when 1A's table lands.

---

## Prompt for the implementing model

> You are working in the Dice and Destiny repository. Read `AGENTS.md` first and follow it exactly, especially: run Godot only through `./scripts/godot.sh`, keep the Go server the authority, keep every number in configuration, never let the view leak hidden state, make every ongoing battle effect a visible status, and never write to the tracked `content/authored/` directory from a test.
>
> Then read `docs/game-design/phase-2-gear-handoff.md` and implement Phase 2-I: tasks 2.1 to 2.9, with every test the handoff lists. Work on a new branch named `phase-2-gear` created from local `main`. Commit each numbered task separately with a clear message. Do not push or merge unless the user asks.
>
> Before changing code, confirm each "where it lives" pointer in the handoff against the current code; line numbers may have moved. The handoff makes design choices marked **Decision**. Follow them. If one contradicts the code, `AGENTS.md` or the plan page, stop and report it rather than guessing. Card and gear names must be original (never echo Dice Throne names).
>
> Don't touch the 1A work (`cmd/balance-lab`, `cmd/card-balance`, `cmd/battle-review`, `internal/battle/lookahead`, the look-ahead policy, enemy budgets) or potions (Phase 2-II).
>
> When you finish, run the full verification list in the handoff, do the live check, and reply with the final report it describes.

---

## Background (read this first)

### The rules this phase must obey (plan §1, §8)

- **XP is the only driver of power.** Gear, its tree nodes, card upgrades and training-gated cards are all bought with XP. Every XP spend is 100% refundable; nothing ever lowers total XP.
- **Gold buys gear, never power.** Owning or wearing gear does nothing until XP is spent on it. Gold also pays training fees.
- **Any gear can be worn.** Investing XP in some items (or some of their nodes) needs a **proficiency**.
- **Gear remembers its nodes.** Taking an item off refunds all its XP; putting it back on shows its remembered nodes and their cost, and the player can re-buy them.
- **Losing gear downgrades the cards it gates** to the highest step that doesn't need it, refunding the XP, after a warning that lists the affected cards. A **frozen** (wounded) copy is never downgraded; it keeps its XP tied up until it heals, then downgrades if the gear is still gone.
- **Gear swaps follow deck-change rules**: only where the deck is open (sites and camp, the same once-per-visit Influence fee). The hometown's free rework is Phase 3.
- **Battle rules don't change.** Gear may only set a battle up: a battle-start status, a stat, an ability bonus carried by a visible status.

### How XP works today

- The sheet is `loadout.Progress` (`internal/battle/loadout/progression.go:50`). Its total is `Budget` (`total_budget`) = `XP + DeckValue + CollectionValue + WoundValue + UpgradeSpent`.
- `settlePurchase` (`progression.go:~726`) recomputes the values after every transaction and sets **`UpgradeSpent` to whatever is left over** (`~739`). It is not tracked; anything new without its own term silently lands there. The refund kinds are hard-coded at `~727`.
- `initializeBudget` (`economy_admin.go:~32`) and `reconcileBudget` (`economy_admin.go:~51`) also compute the sum. `creditXP` (`expedition_save.go:~146`) is the only thing that raises the total; `keepsTotalXP` (`campaign_save.go:~123`) refuses a write that lowers it.
- Expedition deck changes go through `ExpeditionDeckChange` (`expedition_save.go:~194`): `DeckChangeCost` gate → `applyPurchase` → `settlePurchase` → `keepsWounds` → `PayDeckChange` → `ApplyDeckChange` → `finishExpeditionWrite` (which runs `syncSheetFromState`, `validateProgress` and `checkStateMatchesSheet`).
- The Prepare screen (`dice-and-destiny-client/app/screens/campaign/campaign_prepare.gd`) gets its offers from `campaign_loadout` (`learned/campaign.go:~261`), each a `PreviewPurchase` with `cost`, `xp_change`, `available` and `reason`, and submits expedition trades through `expedition_deck_change`.

### Gear and items today

- `expedition.Gear` (`internal/battle/expedition/pack.go:~148`) with `kind` (`consumable` | `gear`), `effect`, `amount`, `price`; `GearEffects = mend_roll, night_sight, remove_fortified`; files `content/expedition_v1/gear/{bandage,lantern,siege_hooks}.yaml`. Ownership is `State.Inventory map[string]int`; effects are looked up in the inventory where they're used (`sightRange` in `generate.go:~174`, `campHeal` in `actions.go:~543`, `siteBattle` in `actions.go:~1184`, the view at `view.go:~611, ~834`).
- Shops: `useSite` `case "shop"` (`actions.go:~853`) spends `State.Gold` and adds to `Inventory`.

### Card trees today

- `content.CardTree` / `CardTreeNode` / `CardTreeEdge` (`internal/content/card_tree.go`). All published trees live in one file, `content/authored/authored_cards.json`, under `"trees"` (`AuthoredCards`, `internal/content/card_authoring.go:~30`).
- **Edge `Requirements` are deck rules, not purchase gates**: by design they never block an upgrade (`learned/card_tree_test.go:~122` asserts "deck gates must not block upgrades"; `docs/game-design/card-trees.md` §deck rules). Gear/training/proficiency gates are a **new, separate** concept on nodes.
- Every tree step is judged in `loadout.TreeTransition` (`internal/battle/loadout/card_tree.go:~88`): edge legality, `e.Access.Check`, cost = `target.Value() - start.Value()`. Tree bases are bought with `buy_card` (`progression.go:~522`). There is **no** existing "highest ungated step" or automatic trade-down function.
- The forge: `card_tree_workshop.gd` (the screen, admin when `admin_token` is set), `card_tree_canvas.gd`, `card_tree_layout.gd`, `card_tree_node.gd`, `card_tree_edge_chip.gd`, `card_tree_forge.gd`, `card_tree_diff.gd` in `app/screens/character/`. Character Creation opens the admin workshop from its Admin panel ("Manage card trees", `character_creation.gd:~1127`). Ops: `card_trees`, `validate_card_tree`, `publish_card_tree` (`learned/card_tree.go`, `handleCardTrees`).

### Battle hooks gear can use

- **Battle-start statuses:** `startBattle` (`expedition/battle.go:~55`) builds `pb.PlayerStatuses` (Ambush, then `s.battleStatuses(r)`); they become preset statuses (`learned/campaign.go` `presets()`, `deck_loadout_assembler.go`). Stacks respect each status's `stack_limit`.
- **Status modifiers** (`content.StatusModifiers`, `battle_library.go:~226`, read by field, never by ID): `defense_dice_bonus`, `first_roll_bonus_dice`, `attack_damage_bonus` (all damaging offensive abilities), `hand_size_delta`, and the map-only `carried_move`, `hidden_sight`, `camp_bandages`.
- **Per-ability bonuses:** `state.RuntimeAbilityModifier` (`internal/battle/state/settled.go:~94`) with a `ProgramBonus` (`damage`, or `status_id` + `stacks`), tied to a `StatusID` that must have stacks on the actor. Today only card programs create it (`ability_bonus`, `engine/card_program.go:~945`). `resolvedOffensiveOperations` (`engine/settled_engine.go:~1742`) applies them, and the look-ahead AI already reads them.
- **Map stats:** hand size `effectiveHandSize` / `effectiveHandLimit` (`generate.go:~135`); base energy `State.BaseEnergy`, set once from the combatant at `expedition_save.go:~112` and reset into `Energy` in `beginRest` (`actions.go:~458`); sight `sightRange` (`generate.go:~174`); terrain `moveCost` (`actions.go:~140`); camp healing `campHeal` (`actions.go:~491`) and `innBonus` (`view.go:~842`).

### Package boundary

`loadout` imports `expedition`, so **`expedition` can't read the sheet**. Today the deck lives on the sheet and is mirrored into the state's piles (`syncSheetFromState` / `checkStateMatchesSheet`). Gear works the same way (Task 2.3).

---

## Task 2.1: The gear content model and storage

**Where it lives (new):** `internal/content/gear.go` (types, validation); storage in a new tracked file `content/authored/authored_gear.json` read and written like `authored_cards.json` (`card_authoring.go`: read, apply, atomic write, revision); loaded into the `BattleLibrary` (`lib.Gear`), next to `lib.CardTrees`.

**A gear item:**

```jsonc
{
  "id": "fine_plate", "name": "…", "description": "…",
  "slots": ["body"],               // a two-handed weapon lists ["main_hand", "off_hand"]
  "rank": "rare",                  // common | rare | epic (config)
  "price": 90,                     // gold; checked against the rank's range in config
  "proficiency": "heavy_armour",   // optional: needed to invest XP in any node
  "art": "res://…",                // optional illustration for the canvas
  "root": "base",                  // the base-effect node
  "nodes": [ {"id": "base", "xp": 20, "effects": [ … ], "proficiency": "…", "x": 0, "y": 0}, … ],
  "edges": [ {"id": "e1", "from": "base", "to": "n2"} ]
}
```

- **Decision: one tree per item; the base node is the root.** A node can only be bought when its parent is bought (like card-tree steps, but nodes **add up**: buying `n2` keeps `base`). Selling a node sells everything bought below it, and refunds all of it.
- **Effect vocabulary** (each node has one or more effects; the loader refuses unknown kinds):

| Effect | Params | Applies to |
| --- | --- | --- |
| `unlock_card_node` | `tree`, `node` | the matching card-tree node's gear gate (Task 2.5) |
| `stat` | `stat` ∈ `hand_size`, `hand_limit`, `base_energy`, `energy_kept_at_rest`, `sight_day`, `sight_night`; `amount` | map and battle setup (Task 2.3) |
| `terrain_cost` | `terrain`, `amount` (negative = cheaper), optional `when: day\|night` | `moveCost` |
| `camp_heal_bonus` | `roll` ∈ `basic_aid`, `bandage`; `amount` | `campHeal` |
| `battle_start_status` | `status`, `stacks` | `startBattle` |
| `ability_bonus` | `ability`, `damage`, `status` (the visible status that carries it) | battle setup (Task 2.4) |
| `map_ability` | `ability` ∈ `improvisation`; its numbers (e.g. `discard: 2`, `move: 3`) | a new map action (Task 2.4) |
| `potion_slots` | reserved for Phase 2-II: the loader refuses it with "potions arrive in Phase 2-II" |

- **Slots, ranks and price ranges are configuration** in `content/progression_v1/expedition.yaml`:

  ```yaml
  gear:
    slots: [main_hand, off_hand, head, cloak, body, gloves, belt, feet, trinket_1, trinket_2]
    ranks: {common: {price: [20, 60]}, rare: {price: [60, 140]}, epic: {price: [140, 300]}}
  ```

  Item `slots` name slot kinds; `trinket` fills either trinket slot. Replace `gold.prices.gear` with the per-rank ranges (the strict loader must refuse the old key). The numbers are placeholders for playtesting.
- Validation (the whole catalog is refused on any problem, naming the item and field): unique ids; known slots, rank, proficiency, status, ability, tree and node; price within the rank's range; one root, no cycles, every node reachable; every node has `xp > 0` and at least one effect; an `unlock_card_node` names a node whose gate names this item (and vice versa, see 2.5); `ability_bonus.status` is a status whose rules text describes the bonus.
- **Consumables stay in the region pack** (`bandage`, `siege_hooks`, `kind: consumable`). `kind: gear` leaves the pack: the Lantern moves to authored gear (Task 2.9) and the pack loader refuses `kind: gear`.
- **Script tests get an empty authored root** (AGENTS.md). Follow how the card-tree tests seed trees (`learned/card_tree_test.go` `treeFixture`) for gear fixtures. Add one test that reads the **tracked** `content/authored/authored_gear.json` read-only and validates it against the shipped library, so the committed content can't rot.

**Tests (`internal/content/gear_test.go`):** a valid item loads; one refusal per validation rule above; `potion_slots` is refused; an old `gold.prices.gear` config key is refused; the tracked file validates.

## Task 2.2: Gear on the character sheet, with its own XP term

**Where it lives:** `loadout.Progress` (`progression.go:50`), `settlePurchase`, `initializeBudget`, `reconcileBudget`, `applyPurchase`, `PreviewPurchase`; the refund-kind lists (`settlePurchase:~727`, `learned/campaign.go:~280`, `learned/campaign_refund_test.go:~52`, and the client's kind→message match in `campaign_prepare.gd:~451`).

**Do:**

1. Add to `Progress` (omitted when empty, so every existing sheet and the linear campaign are untouched):

   ```go
   Gear      []OwnedGear `json:"gear,omitempty"`
   GearValue int         `json:"gear_value,omitempty"`
   Training  []string    `json:"training,omitempty"`  // Task 2.6
   ```

   `OwnedGear{Item string; Instance string; Slot string /* "" = carried, not worn */; Bought []string /* active nodes */; Remembered []string}`. An item can be owned more than once (two daggers), hence `Instance`.
2. **`GearValue` is a real term of the budget**: `Budget = XP + DeckValue + CollectionValue + WoundValue + GearValue + UpgradeSpent`. Add it in `initializeBudget`, `reconcileBudget`, `settlePurchase` (computed from the bought nodes' XP on worn items) and the `totalXP.check` helper in `learned/campaign_total_xp_test.go`. Gear XP must never land in `UpgradeSpent`.
3. **New purchase kinds** (Expedition saves only; `BuyCampaign` and the editors refuse them with a clear error):
   - `gear_equip` (`ID` = instance, `TargetID` = slot): wear an owned item; costs no XP. If the item has `Remembered` nodes they stay inactive until re-bought.
   - `gear_unequip` (`ID` = instance): take it off. **Refunds every bought node's XP**, moves them to `Remembered`, then runs the gate downgrade (Task 2.5). Previewable: the offer lists the affected cards and frozen copies (the warning).
   - `gear_node` (`ID` = instance, `TargetID` = node): buy a node on a worn item. Checks the parent is bought and the proficiency (item and node) is held.
   - `gear_node_sell` (`ID` = instance, `TargetID` = node): sell a node and everything bought below it; refund; gate downgrade.
   Add the two refund kinds (`gear_unequip`, `gear_node_sell`) to every refund-kind list above.
4. **Deck-change rules:** these kinds go through `ExpeditionDeckChange`, so they need the deck open and pay the same once-per-visit Influence (`DeckChangeCost` / `PayDeckChange`). Buying an item with gold does **not** need the deck open (Task 2.7).
5. `campaign_loadout` returns, for Expedition saves, a `gear` section: slots (with what's worn), owned items, each item's tree (nodes with state bought / remembered / available / locked and the reason), and the four kinds of offer with `cost`, `xp_change`, `available`, `reason` and, for unequip/sell, `warning` (the cards that would downgrade and the frozen copies that would wait).

**Tests (follow `loadout/refund_test.go` and `learned/campaign_refund_test.go`):**
- Equip, buy base, buy a child node, sell the child, sell the base, unequip: XP moves by exactly each cost and the total never changes; `GearValue` tracks the bought nodes; `UpgradeSpent` is untouched.
- Re-equip shows the remembered nodes; re-buying them costs the same XP.
- A node without its parent, a node needing a proficiency the hero lacks, a node on an item that isn't worn, and any gear kind on a linear-campaign save are refused with a reason.
- Extend `TestCampaignXPSpendsRoundTripExactly` (`/expedition`) and `TestTotalXPNeverDropsAndIsReported` to include gear transactions.

## Task 2.3: The expedition state mirrors worn gear

**Where it lives:** `expedition.State`; `syncSheetFromState` / `checkStateMatchesSheet` / `finishExpeditionWrite` (`loadout/expedition_save.go`); the map stat functions listed in Background.

**Do:**

1. **Decision: the sheet owns gear; the state holds a read-only mirror of the worn items' active effects**, written only by the loadout layer after a gear transaction or on load (`syncGearToState`), and checked equal to the sheet in `checkStateMatchesSheet`. Suggested field: `State.Gear []WornGear{Instance, Item, Slot string; Effects []GearEffect}`. Expedition rules read only this mirror.
2. Apply the effects at their plug points: `hand_size` / `hand_limit` in `effectiveHandSize` / `effectiveHandLimit` (and so in `Setup` for battles); `base_energy` by recomputing `BaseEnergy` = combatant starting energy + gear (the next rest uses it; don't change current energy mid-turn); `energy_kept_at_rest` in `beginRest` (`Energy = BaseEnergy + min(kept, energy before the rest)`); `sight_day` / `sight_night` in `sightRange` (and enemy sight stays as it is, like today's Lantern); `terrain_cost` in `moveCost` (never below 1, never makes impassable terrain passable); `camp_heal_bonus` in `campHeal`; `battle_start_status` in `startBattle`.
3. The view shows the worn gear (item, slot, active effects as text from the authority) and its stat contributions in the existing tooltips (hand, energy, sight, terrain legend, camp healing). Lists never null; nothing hidden leaks.
4. **Old saves:** a save with no `gear` loads with an empty mirror; a save whose `Inventory` holds the old `lantern` migrates it to an owned, carried (unworn) Lantern on the sheet with no nodes bought, and logs one line telling the player. Follow the legacy patterns from Phase 1B (migrate on the first action; `View` shows a migrated copy).
5. **Seeds:** no test hero owns gear unless a test gives it, so seeded runs must not change. If the Lantern migration changes a seeded run, find out why; per AGENTS.md, find a new seed with the env-gated probes rather than weakening an assertion.

**Tests:** each stat effect changes its map value and, where it applies, the battle setup (`BattleSetup.HandSize`, `PlayerStatuses`); `checkStateMatchesSheet` refuses a mirror that disagrees with the sheet; the Lantern migration; `TestViewNeverLeaksHiddenState` still passes.

## Task 2.4: The two new effect kinds: ability bonuses and Improvisation

**Ability bonuses (`ability_bonus`):**
- **Decision: carried by a visible battle-start status** (AGENTS.md "Mandatory visible status effects"). At battle setup the worn effect adds its status (e.g. "Honed Edge: +1 damage to Strike") **and** seeds a `RuntimeAbilityModifier` for that ability with `ProgramBonus {"damage": N}`, `StatusID` = that status, lasting the whole battle. The engine already applies these and skips them if the status has no stacks; the look-ahead AI already reads them.
- Author the status in the status catalog with polarity, stacks, expiry ("lasts the whole battle") and full rules text. The attack tooltip shows the bonus through the existing presenter (AGENTS.md "Ability hover contract"); don't add a separate hint.
- Seed it where battle presets are built (`learned/campaign.go` `presets()` / `deck_loadout_assembler.go`); don't add a new engine rule.

**Improvisation (`map_ability`):**
- A new map action (e.g. `improvise`), legal only while a worn item grants it: discard `discard` cards from hand to Spent for `move` Move this turn (numbers on the gear node). It counts as playing cards for the "a turn needs a card played or discarded" rule. Once per turn unless the node says otherwise.
- The view lists available map abilities with their rules; the map screen offers a clearly labelled control in the hand area.

**Tests:** a battle started with the spear worn has the status, and Strike's resolved damage is +1 at every tier while other abilities are unchanged; with the status removed the bonus stops; the tooltip shows the bonus. Improvise: legal only with the boots worn, moves the cards to Spent, grants the Move, refused twice in a turn, refused without enough cards; a Godot pointer test clicks the control.

## Task 2.5: Gates on card-tree nodes

**Where it lives:** `content.CardTreeNode` (`card_tree.go:~21`), `ValidateCardTrees`, `loadout.TreeTransition` (`loadout/card_tree.go:~88`), `buy_card` in `applyPurchase`, the card tree workshop's node inspector, and the Prepare screen's trade rows.

**Do:**

1. Add `Requires []NodeGate` to `CardTreeNode`, where a gate is exactly one of `{gear: <item>}`, `{training: <id>}`, `{proficiency: <id>}`. All gates on a node must hold. **This is not the edge `Requirements` deck rule**; leave that untouched and keep `"deck gates must not block upgrades"` passing.
2. When a gate holds:
   - `gear`: the item is worn **and** its `unlock_card_node` node for this tree and node is bought;
   - `training`: the sheet's `Training` contains it (Task 2.6);
   - `proficiency`: the character's proficiency list (Task 2.6) contains it, including any added by training.
3. **Buying:** `TreeTransition` refuses a step **into** a gated node whose gates don't hold, with a reason naming what's missing ("Needs Fine Plate with Bulwark unlocked"). Steps **down** (refunds) are always allowed. `buy_card` checks gated bases the same way. Offers then show the reason automatically.
4. **Losing a gate** (unequip, node sold, later: theft or breakage) runs `downgradeUngated`: every **live** copy of a node whose gate no longer holds steps down to the highest-value ungated node reachable through reverse reversible edges (the base if nothing else), refunding the difference. **Decision:** "highest step" means highest node `Value()`. **Frozen (wounded) copies are never moved**: they keep their node and XP, and are recorded as `pending_downgrade`; when a wound heals (camp aid, bandage, site mending, region win), the authority re-checks and downgrades any healed copy whose gate still doesn't hold. Copies in the collection (stored) follow the same rule as live ones.
5. `ValidateCardTrees`: a `gear` gate names an existing item that has an `unlock_card_node` pointing back at this node; training and proficiency ids exist in their catalogs. Validation needs the gear catalog, so validate trees after gear is loaded.
6. **The forge:** the card tree workshop's node inspector gets a "Requires" editor (add/remove gear, training or proficiency gates); the canvas shows a lock badge with the requirement on gated nodes. Player views and Prepare show what each gated node needs.
7. **First gate (Decision):** only **Bulwark** (`steady_guard_paths`, `node_7`) is gated, by Fine Plate. Its fallback when the gate is lost is Dig In (33 XP), the highest-value ungated step below it. Publish this through the normal tree publish path so `authored_cards.json` and its revision update together.

**Tests:** buying Bulwark without the plate (or with the plate but its unlock node unbought) is refused with the reason; with both it succeeds; unequipping the plate downgrades a live Bulwark copy to Dig In and refunds exactly 1 XP, after a preview that lists it; a wounded Bulwark copy is not moved, then downgrades when mended; deck rules still never block upgrades; tree validation refuses a dangling gate.

## Task 2.6: Training and proficiency

**Proficiency:**
- A proficiency list lives in configuration (`content/progression_v1/economy.yaml`, e.g. `proficiencies: [light_armour, heavy_armour, blades, …]`), and each character gets `characters.<id>.proficiencies` (`loadout.CharacterEconomy`, `progression.go:~20`).
- **Decision (placeholder, the user may change it):** the Starter has every proficiency except `heavy_armour`; the Adventurer has all of them. So the Starter needs training before investing in Fine Plate.

**Training:**
- Trainings are region-pack content (`content/expedition_v1/training/*.yaml`): `id`, `name`, `description`, and what it gives: `grants_proficiency: <id>` and/or it satisfies `{training: <id>}` gates on card-tree nodes.
- A site offers training with a gold fee: `trainers: [{training: <id>, fee: <gold>}]` in the site YAML (validated: known training, fee within a configured range `gold.prices.training` in `expedition.yaml`). It shows as a site service; paying it adds the training to the sheet's `Training` (through `Rewards`, the way XP is credited, because the state can't write the sheet). **Training enables nothing by itself**: XP still buys the gated step.
- **First content (Decision):** Greywharf trains `heavy_armour` (a proficiency); Stillwater Monastery offers one training that unblocks one existing card-tree node of your choice that's an energy-heavy upgrade (the plan's example is "the +3 energy variant at a monastery"). Pick a node in a published tree, gate it with `{training: …}`, and name it in the report.
- Gold never buys XP: training costs gold, but the gated step still costs its XP.

**Tests:** paying a fee records the training once (a second payment is refused), spends exactly the fee, and is refused without the gold or at night when the site is closed; a training-gated node is refused before and allowed after; `heavy_armour` training lets the Starter invest in Fine Plate; the save round-trips.

## Task 2.7: The gold pass: shops sell gear

**Where it lives:** `useSite` `case "shop"` (`actions.go:~853`), the site YAML `shop` lists, `ViewService`, the map screen's site panel (`expedition_screen.gd` `_render_site`).

**Do:**
1. A shop may list authored gear items as well as consumables. **Decision:** a shop entry may carry a stock limit (`{item: fine_plate, stock: 1}`; plain ids stay unlimited) so rare stock stays scarce; the state tracks what's been bought.
2. Buying gear spends gold (the item's `price`) and adds an **owned, carried** instance to the sheet (through `Rewards`). It doesn't need the deck open; wearing it does.
3. **Selling gear back** to a shop for a configured share of its price (`gold.sell_share` in `expedition.yaml`, placeholder 0.5): only for carried, unworn instances with no bought nodes. (Remembered nodes are forgotten when the item is sold.)
4. The shop shows rank, slots, the base effect's text and its XP cost, so the player knows the item does nothing until XP is spent.
5. `docs/game-design/expedition.md` §10 economy table: gear now comes from shops by rank; prices in config.

**Tests:** buying spends exactly the price and adds a carried instance; stock limits hold across save/reload; buying with too little gold is refused; selling refunds the configured share and is refused for a worn item or one with bought nodes; gold never changes XP.

## Task 2.8: The gear workshop (extending the forge)

**Where it lives (new):** `app/screens/character/gear_tree_workshop.gd`, reusing `card_tree_canvas.gd`, `card_tree_layout.gd`, `card_tree_node.gd` and `card_tree_edge_chip.gd` (they only need a card-shaped dictionary per node: name, art, XP; make the energy pip optional and add a node-kind icon). Server ops `gear_trees`, `validate_gear_tree`, `publish_gear_tree` (admin token and revision checks exactly like `handleCardTrees` in `learned/card_tree.go`; `DisallowUnknownFields`).

**Do:**
- **Decision: a separate workshop screen that shares the canvas**, not a mode flag inside `card_tree_workshop.gd` (which is card-specific throughout). Open it from Character Creation's Admin panel beside "Manage card trees" ("Manage gear").
- Admin features: item list; new item; item fields (name, description, slots, rank, price, proficiency, art); add/remove/connect nodes on the canvas; a node inspector with XP (±5 steps like the forge) and an effect editor for every kind in the vocabulary (pickers for slots, stats, statuses, abilities, card trees and nodes); undo/redo; live validation on a timer; publish. A published `unlock_card_node` and the matching card-node gate must be publishable together or one must explain what's missing (validation names the other side).
- Player view: read-only canvas inside Prepare's gear section (Task 2.2) with the node states and buy/sell buttons.

**Tests:**
- Go: publishing needs admin; a stale revision is refused; an invalid item is refused with nothing written; publishing writes only `authored_gear.json` (in a disposable authored root).
- Godot (new `tests/presentation/verify_gear_workshop.gd`, headless, pointer-driven): create an item, add a base and a child node with effects, see live validation catch a missing effect, publish, reopen and see it; a player opens Prepare, equips the item, buys both nodes, sees the remembered nodes after unequip and re-equip.

## Task 2.9: First gear content

All names below are working names; **choose original final names** (no Dice Throne echoes) and report them. All XP prices follow `docs/game-design/card-value-rules.md`; add a §6 "Gear" to that doc that states how each effect is priced, marks every placeholder, and lists each item's nodes and prices. Suggested anchors: +1 sight = 15 XP (the Lookout precedent); battle-start statuses priced by what the same status costs on a card; a stat with no unit gets a placeholder and a note.

| Item (working name) | Slot · rank · gold | Base node | Further nodes |
| --- | --- | --- | --- |
| **Fine Plate** | body · rare · stock 1 at Greywharf | a visible battle-start status: +1 defensive die (`defense_dice_bonus` 1) | Unlock Bulwark (`unlock_card_node`); needs `heavy_armour` |
| **Spear** | main hand (one-handed) · common | `ability_bonus`: +1 damage to Strike (`adventurer_strike`) via a visible status | a second +1 to Strike |
| **Trail boots** | feet · common | `map_ability`: Improvisation (discard 2 for Move 3) | marsh costs 1 less Move |
| **Ember pouch** | belt · common | `energy_kept_at_rest` 5 | `energy_kept_at_rest` +5 |
| **Lantern** (migrated) | trinket · common · 40 | `sight_night` +1 | `sight_day` +1 |

- **Decision: no status-applying offence on gear yet.** The plan mentions a poison-tipped spear; status application is reserved for player types (Venom, Curse), so the spear's tree is damage-only until the user rules on gear that applies statuses. Keep the vocabulary able to express it later (`ability_bonus` with a `status_id` payload exists in the engine) but don't ship it.
- Put the items in Greywharf's and Saltreed's shops (Fine Plate only at Greywharf, stock 1).

**Tests:** the shipped items validate; a scripted playthrough buys and wears each one and sees its effect where it applies (map value or battle setup).

---

## Docs

- `docs/game-design/expedition.md`: a new "Gear" section (slots, owning vs wearing, XP nodes, remembered nodes, gates and downgrades, training, proficiency, shops), and dated decision-log lines.
- `docs/game-design/expedition-progress.md`: numbered implementation notes (the sheet mirror, the `GearValue` term, the frozen-copy downgrade, the Lantern migration).
- `docs/game-design/card-trees.md`: node gates vs edge deck rules.
- `docs/game-design/card-value-rules.md` §6 "Gear" (Task 2.9).
- `AGENTS.md`, Expedition section: one bullet for gear (sheet-owned, mirrored into the state, `GearValue` in the budget, gear changes only where the deck is open, gates step down live copies but never frozen ones) and one for training/proficiency.

## Verification

Run all of these from the repository root, following `AGENTS.md`:

```bash
cd dice-and-destiny-server && go test ./...
cd .. && ./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_expedition_map.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_expedition_statuses.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_expedition_chart.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_card_trees.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_card_tree_templates.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_character_creation.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_gear_workshop.gd
```

plus any campaign Prepare test you touch (find them with `grep -l campaign_prepare dice-and-destiny-client/tests -r`).

Notes:
- Headless runs give the verdict. For live checks, drive the game with the inspector's `pointer` route (`python3 dice-and-destiny-client/devtools/inspect_game.py pointer …`), never the system cursor.
- The `learned` package takes about 8 minutes.
- Seeds are fixed; don't weaken assertions (AGENTS.md).
- If a test already fails on `main`, prove it on a clean `main` checkout and say so.

**Live check** (`DICE_AND_DESTINY_INSPECTOR=1 ./scripts/godot.sh`):
- start an expedition as the Starter; buy the Spear at Greywharf; at camp, wear it and buy its base node; start a fight and hover Strike: the bonus shows in the tooltip and the status on the status bar;
- try to buy Fine Plate's Bulwark node without training (refused with the reason); pay the `heavy_armour` training; buy the plate's nodes; buy Bulwark; take the plate off and read the warning; confirm Bulwark steps down to Dig In with the refund;
- wear the boots and use Improvisation;
- open the gear workshop from Character Creation's Admin panel and view an item.

Save screenshots in `dice-and-destiny-client/.godot/runtime/expedition-evidence/`.

## Acceptance criteria

1. Gear is owned on the sheet; its bought nodes count in `GearValue`, a real term of the budget; every gear XP spend refunds exactly; total XP never drops.
2. Owning or wearing an item does nothing until XP is spent; unequipping refunds all its XP and keeps the remembered nodes.
3. Gear changes and gear XP purchases happen only where the deck is open, paying the same visit fee; buying an item costs gold only.
4. A gated card-tree node can't be bought without its gear unlock, training or proficiency, and Prepare shows what it needs; edge deck rules still never block upgrades.
5. Losing a gate steps live copies down to the highest ungated step with a refund, after a warning; frozen copies wait and step down when healed.
6. Every battle effect of gear is a visible status with full rules and expiry; ability bonuses show in the attack tooltip.
7. Training costs gold, records once, and only unblocks; proficiency lists are configuration.
8. The gear workshop authors and publishes items to `authored_gear.json`; tests never write the tracked directory.
9. The five items ship, validate and work in a playthrough; their prices and every placeholder are documented in `card-value-rules.md` §6.
10. The linear campaign, the editors' progression saves and seeded expedition outcomes are unchanged.
11. Every command in the verification list passes, apart from any failure proven to exist on `main`.

## Out of scope

- Potions, potion slots and potion skill (Phase 2-II).
- The hometown's free rework, the story skip, content versioning for saves (Phase 3).
- Theft, breakage, materials, expert smiths and reputation-gated stock (later phases); the downgrade code should be reusable for theft and breakage, but nothing triggers them yet.
- Gear that applies statuses to enemies (waiting on a user decision).
- Linear-campaign gear; enemy gear.
- 1A: enemy budgets, the fairness readout, fight logs.
- Pushing or merging the branch.

## Final report

Reply with:
1. a summary of each task (2.1–2.9) and its commit;
2. the test commands you ran, with pass/fail results;
3. the final gear effect vocabulary and anything you added beyond it;
4. the shipped items with their final names, nodes, XP and gold prices, and which numbers are placeholders;
5. the training-gated node you chose at the monastery, and why;
6. anything that contradicted this handoff, the plan page or `AGENTS.md`, and how you handled it;
7. design choices this handoff didn't cover;
8. the paths to the live-check screenshots.
