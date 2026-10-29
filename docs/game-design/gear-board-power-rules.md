# Gear and the ability board: power rules

**Status:** rules for the Phase 2 gear work, written 2026-10-26.
**Read with:**
- [`board-xp.md`](board-xp.md): the priced ability board, the damage level, defenses and board XP.
- [`power-pricing-research.md`](power-pricing-research.md): the exchange rates.
- [`phase-2-gear-handoff.md`](phase-2-gear-handoff.md): the gear system as first built on `phase-2-gear`.

Decisions marked **Decision (user)** are the user's. Those marked **Decision (proposed)** are defaults chosen to keep the rules consistent. They can be changed, but only together with the rule they support.

## Why the first gear pass has to change

The first pass of gear was written before the board's damage became one priced slider. Its Reedcutter Spear base node gives Strike +1 damage on every tier for 20 XP. The pricing model (`internal/battle/boardprice`, which re-plans the player's keeps for every board) shows what that does to the Starter board at its standard level:

| Board | Avg value per turn | XP | Turns that end on Strike |
|---|---|---|---|
| Standard (4/5/7, 4, 5, 6, 7, 8) | 6.21 | — | 56% |
| Spear: Strike +1 on every tier | about 7.1 | **20** | **85%** |
| The slider step with the same gain (5/6/8, 5, 6, 7, 8, 10) | about 7.2 | 40 | 56% |

Two things go wrong:

1. **Power rises faster than XP.** About a full point of average damage costs 20 XP instead of 40.
   - Any flat price for a targeted bonus is wrong, because a tier's +1 is worth anything from about 0 to 24 XP, depending on how often that tier ends a turn.
   - On the Starter board, Gold Coin ×4 +1 is worth 0.4 XP, Sword ×4 +1 is worth 15 XP, and Sword ×5 +1 is worth 24 XP.
2. **The board collapses into one ability.** Strike goes from 56% of turns to 85%. Guarded Strike, Measured Strike and both straights drop to 3–5% each. The slider exists to keep that mix stable while the board gets stronger.

There is also a hard conflict. A save on a priced board fights with generated abilities (`board_main3_main4_main5`), not `adventurer_strike`, so an `ability_bonus` that names `adventurer_strike` does nothing there. Since 2026-10-25, every new Starter, Adventurer and Custom Adventurer campaign and expedition is on a priced board.

## Rule 1: gear never raises one ability or tier

**Decision (user):** no gear effect may add damage, or an on-hit effect, to a named ability or a single tier.

- The gear loader refuses `ability_bonus` (with `damage` or with `applies`), with an error that points to this document.
- Keep the engine's `RuntimeAbilityModifier` and `ProgramBonus` support. Card programs still use it, and unused engine capabilities stay (a big status phase is coming).
- Retire the statuses that only carried those bonuses (`keen_point`, `fen_toxin_tip`, `volatile_tip`) if nothing else uses them.

## Rule 2: gear unlocks damage headroom, and XP buys it on the slider

**Decision (user):** a gear node can give **+1 damage headroom**. The player may then raise their board's damage level by one more point of average damage over where they stand, paying the usual board XP. The more of these nodes a character has, the higher their board may climb.

- **New effect:** `board_damage_cap` with `points` (a positive whole number). Each bought node with this effect, on a worn item, adds its points.
- **The cap.** In an expedition, the board's damage level may be at most the standard level plus the headroom:
  - Headroom = `free_damage_points` (configuration) + the points from bought nodes on worn gear.
  - The cap is measured in average damage per turn over the standard step: the highest allowed step is the last staircase step whose `average` ≤ the standard step's `average` + headroom (with a 1e-9 tolerance).
  - One point of headroom is one point of average damage, which costs 40 XP on the slider (`economy.yaml` `board_xp.damage_point`).
- **`free_damage_points` lives in `content/progression_v1/expedition.yaml` under `gear.board`.**
  - **Decision (proposed): 0.** A character climbs only as far as gear allows. This is a playtest placeholder.
- **Price of the unlock node.**
  - **Decision (proposed):** a node whose effects are all unlocks (`board_damage_cap`, `board_rider` below) costs **0 XP**. It grants no power by itself: headroom is paid for on the slider, and a rider is a value-neutral swap.
  - So the gear validation that every node has `xp > 0` becomes "`xp > 0` unless every effect on the node is an unlock".
  - Owning or wearing gear still does nothing until XP is spent on the board.
- **Losing headroom.** Unequipping the item, selling the node, or losing the gear lowers the cap.
  - The board's level then steps down to the highest allowed step and refunds the difference.
  - This follows the card-gate downgrade rule: list it in the unequip / sell offer's `warning` first.
  - It never lowers the total XP.
- **When.** Raising the level is still an expedition board change (`campaign_board_change`), so it happens only at a site or camp and pays the visit's Influence (already built). Gear swaps follow the deck-change rules as before.
- **Scope.** Expedition saves only, like all gear. Linear campaigns and the Board Builder in Character Creation keep the uncapped slider.

### What the player sees

- **Prepare's Ability board panel:**
  - **Headroom line.** It shows the headroom and where it comes from, e.g. "Damage headroom: +2 (Reedcutter Spear +1, Iron Bracers +1)".
  - **Locked steps.** Steps above the cap are shown locked with the reason "needs more damage headroom from gear". The ▶ arrow and each tier's "Raise to N" button stop at the cap.
- **The view.** `campaign_board` returns `cap: {points, step, sources: [{item, instance, node, points}]}`. Each staircase step above the cap is marked `locked: true`, and each tier raise beyond it carries `locked` and `reason`. The server refuses a board change above the cap.

## Rule 3: gear unlocks a rider that takes the place of an ability's damage

**Decision (user):** a weapon such as the Spear can let one ability (e.g. the Strike ladder: 3, 4 or 5 swords) carry a weak poison **instead of** some of its damage. The board still rises evenly. When the level goes up a point, the other abilities gain their +1 damage as usual, but the poisoned ability takes the poison in place of its +1. A later gear node can do the same for another ability, replacing that ability's +1 with poison too.

The board already has this mechanism: **slot riders** (`content/progression_v1/riders.yaml`). A rider's worth at the `riders.yaml` rates comes off its slot's damage. For example, Strike 4/5/7 with a 1-point rider becomes 3/4/6 + the rider. So a rider never adds power: it changes the form of an ability's value, not its amount. The model confirms that the board stays balanced:

| Starter board, standard level | Avg value per turn | XP | Turns that end on Strike |
|---|---|---|---|
| No rider (Strike 4/5/7) | 6.21 | — | 56.5% |
| Strike carries a 1-point rider (3/4/6 + rider) | 6.21 | **0** | 56.5% |
| For contrast: 1 point added on top of Strike only | 6.96 | about 30 | **84.7%** |
| For contrast: 1 point added on top of every attack | 7.21 | about 40 | 57.4% |

So gear sells power only through damage headroom (Rule 2, bought on the slider at 40 XP per point). The poison is a free choice of how one ability carries its share. In the user's terms: the Spear's +1 node lets the board climb one point (40 XP on the slider), and its poison node lets the Strike ladder take Fen Sting in place of the point it would have gained.

- **New effect:** `board_rider` with `rider` (an id in `riders.yaml`) and `jobs` (the slot jobs it unlocks it for, e.g. `[ladder]`). The node unlocks that rider on the board's slots of those jobs. A later item or node can unlock the same rider for more jobs (e.g. `[straight]`).
- **Gear-only riders.** Gear riders are marked `unlock: gear` in `riders.yaml`.
  - Expedition Prepare offers them only on slots that a bought node on worn gear unlocks.
  - The admin Board Builder in Character Creation may offer every rider, like its uncapped slider.
- **The choice.**
  - Picking or removing a rider is a board change (`campaign_board_change`), so it happens only at a site or camp and pays the visit's Influence.
  - It sets the slot's rider in the save's board choice (`Choice.Slots[].Rider`).
  - Only slots with `rider: true` in the template take one. Fixed-rider slots, such as the Starter's Guarded Strike (+2 Protect) and Measured Strike (+5 energy), keep theirs.
  - A slot holds one rider, so poison replaces any other rider on that slot.
  - `minimum_damage: 1` still holds: no tier drops to 0 damage.
- **XP: none.** The swap costs 0 XP at any level. The board's average damage doesn't change, so its level XP stays put, and the total never moves.
  - The staircase is recomputed with the rider, so the level must be re-mapped to the step with the same average. Reuse the Board Builder's pricing of slot picks.
  - Any difference left by whole-damage rounding is settled by the usual board-change rule: the change moves XP, never the total.
  - A test must show that a pick, then a removal, returns the board's XP exactly.
- **Losing the unlock** (unequip, sell, lost gear) removes the rider. The slot's damage comes back in its place, settled the same way, and the unequip or sell offer's `warning` lists it first.
- **Hover contract.** The generated ability's rules text and tooltip must name the rider on every tier that carries it.
- **Real fights.** A rider's rate is its theoretical value. Fen Sting's realises about 0.7–0.8 per application in real fights (below), so at rate 1.0 the swap is a slight downgrade in raw damage. That is the safe direction: the swap can never buy power.
  - Also check whether a status rider still applies when the attack's damage is fully prevented or blocked. If it does, poison gains against heavy defenders; measure that before changing the rate.
  - The rate stays in configuration (`rates.status.fen_sting`) so playtesting can tune it.

### The weak poison

**Decision (user):** a new, very weak poison that is worth exactly one damage per application.

- **Name:** **Fen Sting** (`fen_sting`). The name must be original; change it if you have a better one.
- **Rules:** at Ongoing Effects, roll once per stack:
  - faces 1–2 deal 1 damage;
  - faces 3–4 do nothing;
  - faces 5–6 remove one stack.
- **Content.** Author it like `battle_v1/statuses/poison.yaml`: `roll_dice` with `one_per_status_stack`, face outcomes, polarity negative, persistent. Its `rules_text` must state the rolls and when it ends (the mandatory visible status rules).
- **Value: exactly 1 expected damage per stack over an unlimited fight.** Each roll deals 1 with chance 1/3 and ends the stack with chance 1/3, so E = ⅓(1 + E) + ⅓E, which gives E = 1. Stacks roll independently, so each new application adds exactly 1.
- **Stack limit: 6 (placeholder).** A simulation of one application per round shows that limits above 5 change almost nothing.
- **What fights actually realise** is lower, because stacks still on the enemy when the battle ends are lost: about 0.70 damage per application in 6-round fights and 0.80 in 10-round fights. See "Real fights" above.
- **Rider:** `fen_sting_1` in `riders.yaml`, `unlock: gear`, jobs including `ladder`.
  - Operations: `apply_status` on the hit target (`selected_targets`), `status_id: fen_sting`, `stack_count: 1`.
  - Rate: `rates.status.fen_sting: 1.0`, so the ladder pays about 1 damage per tier for it.
  - The rider checker (`boardprice` `checkRider`) must accept a status applied to the target, not only to self.
- **For comparison:**
  - The existing Poison is worth 2 per stack (E = ⅔(1 + E)).
  - Volatile Poison is worth 8 per stack (E = ⅔(2 + E) + ⅙E).
  - As riders, those would take 2 and 8 damage off every tier of their slot. The Volatile one would leave every tier at the 1-damage floor, then saturate its stack limit of 3. That's why the Spear's 15 XP Poison and Volatile nodes can't stay as they are.

## Rule 4: everything else gear does to battles is priced

Gear effects that change battle power must be priced with the same exchange rates as the board:
- 40 XP per point of average damage per turn;
- 40 XP per point of average prevention per roll;
- blocks priced as a share of total XP (`board-xp.md`).

Map-only effects keep the card-value-rules units.

**Review Greywharf Plate's `plated` battle-start status** (20 XP) against these rates, and reprice it or say why it already fits.

## Converting the shipped gear

| Item | Now | Under these rules |
|---|---|---|
| Reedcutter Spear, base (20 XP) | `ability_bonus` Strike +1 damage | `board_damage_cap: 1` (0 XP) |
| Reedcutter Spear, Fen Toxin (15 XP) | Poison on Strike hits | `board_rider: {rider: fen_sting_1, jobs: [ladder]}` (0 XP). The ladder may then take Fen Sting in place of about 1 damage per tier, for 0 XP |
| Reedcutter Spear, Volatile Draught (15 XP) | Volatile Poison on Strike hits | Remove it, or make it a node that unlocks `fen_sting_1` for one more job (e.g. `[straight]`), or a second `board_damage_cap: 1`. Don't keep a Volatile Poison rider: it is worth 8 damage per stack |
| Greywharf Plate | `plated` start status; Bulwark gate | Keep the gate. Price `plated` by Rule 4 |
| Lantern, Ember Pouch, Fenwalker Boots | map stats, Improvisation, terrain | Unchanged (map effects) |

Other weapons, bracers and trinkets can then be the item family that sells damage headroom (`board_damage_cap`). Shields and armour can later unlock defenses on the board's defense menu (e.g. Bastion and Twin Deflect behind a shield), each still bought at its board price. Note that idea for a later pass; it isn't required now.

## Where the code lives

All of this is on the board XP work (`power-pricing-research`). It must be on the gear branch before this pass starts:

- **The staircase.** `boardprice.Staircase` steps have `Average` and `Damage`. Level XP comes from `boardprice.XPRates.LevelXP`, and the rates from `economy.yaml` `board_xp`.
- **Board changes and their XP.**
  - `loadout/board_xp.go`: `applyBoardChange` (the cap check and the rider pick go here; campaign changes currently allow only the level and defenses), `boardValue`, `HoldsBoard`, `AdoptBoard`, `ExpeditionBoardChange`.
  - The ledger: `Budget = XP + DeckValue + CollectionValue + WoundValue + UpgradeSpent + BoardValue` (plus `GearValue` on the gear branch).
- **Views.** `learned/board_xp.go`: `campaignBoardView` and `boardXPView` (add `cap`), `staircaseView` and `tierRaises` (mark locked steps and raises), `campaignBoardChange`.
- **Riders.** `boardprice/builder.go` `Resolve` reads each slot's rider (`SlotPick.Rider`, else the template's `default_rider`), and the pricing takes its value off the slot's damage. `boardprice/generate.go` `ability()` appends the rider's operations to every tier, and `rulesText` must name it. `menus.go` validates riders against slot jobs; add the `unlock: gear` check there.
- **Client.** `app/screens/campaign/campaign_prepare.gd`: `_render_board`, the level arrows, and the "Raise to N" buttons.
- **Gear.** The sheet's worn gear and bought nodes are `Progress.Gear` on `phase-2-gear`. The loader and effects vocabulary are in `internal/content/gear.go`.

## Tests

- **Go, gear.** The gear loader refuses `ability_bonus`. It accepts `board_damage_cap` and `board_rider` with 0-XP unlock nodes, and refuses a 0-XP node with any non-unlock effect.
- **Go, the cap.**
  - With no headroom, an expedition board can't rise above the standard level, and the error names the gear needed.
  - With a Spear (+1), it can rise to the last step within +1.0 average and no higher.
  - Two headroom sources stack.
  - The view marks the locked steps and raises.
- **Go, losing headroom.** Unequipping the Spear steps the level down to the cap, refunds exactly the difference, keeps the total XP, and lists the change in the unequip warning first.
- **Go, the rider.**
  - Without the unlock, the server refuses `fen_sting_1` on any slot. With the Spear's node, it is accepted on the ladder (`jobs: [ladder]`) and refused on other slots and on fixed-rider slots.
  - Picking it on the Starter's Strike gives 3/4/6 + Fen Sting at the standard level. The board's average and level XP stay the same, the total doesn't move, and a pick then a removal returns the XP exactly.
  - Every Strike tier applies one Fen Sting stack on a hit, and its rules text and tooltip name it.
  - Losing the unlock removes the rider, restores the damage, and lists it in the warning first.
- **Go, the poison.** Fen Sting's outcomes are faces 1–2 / 3–4 / 5–6. An engine test rolls a stack through Ongoing Effects to damage, no-op and removal.
- **Go, round trips and total XP.** The `campaign_refund_test.go` round trip covers a cap raise, a level climb, the rider, then unequipping. `TestTotalXPNeverDropsAndIsReported` stays green.
- **Godot, by pointer** (`verify_expedition_gear.gd`), at camp:
  - wear the Spear and buy its headroom node;
  - ▶ climbs only to the cap, and "Raise to N" beyond it is disabled with the reason;
  - pick Fen Sting on Strike, see Strike's damage drop by one with the poison named, fight, and see the Fen Sting status on the enemy's status bar with its full rules on hover;
  - unequip the Spear and see the warning, then the step-down and the refund.
- **Seeds.** Keep the seeded expedition and region tests green. If their outcomes change, find new seeds with the probes rather than weakening the assertions.

## Open questions to bring back to the user

- **Free headroom.** Should `free_damage_points` be 0 (gear-only growth) or a small allowance?
- **Unlock node price.** Are 0-XP unlock nodes right, or should an unlock node carry a small fee?
- **Fen Sting's rate.** Keep 1.0 (the ladder gives up 1 damage per tier) until battles are measured, or ship the simulated ~0.75 (so it gives up less)? With whole-damage rounding both may round to 1.
- **Which jobs the Spear unlocks.** The ladder only (proposed), or also the straights?
