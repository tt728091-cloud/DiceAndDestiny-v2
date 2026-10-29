# Ability board XP

**Status:** built 2026-10-24 on branch `power-pricing-research`.

The Custom Adventurer's ability board costs XP. That includes its damage level and its defense. You buy them in Character Creation's Board Builder, and between battles in a campaign or expedition, the same way you buy cards.

Since 2026-10-25 the Starter and the Adventurer hold a priced board in campaigns and expeditions too (see [The Starter's and Adventurer's board](#the-starters-and-adventurers-board)).

The prices come from [`power-pricing-research.md`](power-pricing-research.md): 40 XP per point of average damage, 40 XP per point of average prevention, and cards at 10 XP. All numbers live in `content/progression_v1/economy.yaml` under `board_xp`.

## Prices

| What | Price | Example |
|---|---|---|
| Damage level | `damage_point` (40) XP per point of average damage per turn that the level's step adds over the standard step (6.2) | A step that adds 0.25 average costs 10 XP |
| Flat defense | `prevention_point` (40) XP per point of average prevention per roll, on the board's die | Holdfast 3.5 → 140 XP |
| Block defense (faces that prevent a whole attack) | block chance × `block.share` (0.5) × (total XP + `block.offset` (200)) | Clean Deflect (1 in 6) at 900 total XP → 92 XP |
| Extra | `defense_extra` adds flat XP to one defense | Holdfast+ (saved cards stay put) = Holdfast + 38 |

- **Damage level.** The level is a target on the board's staircase (`boardprice/staircase.go`). Each step raises at least one ability and lowers none.
  - Its XP is rounded once, for the whole climb from the standard step: XP = round(40 × (step average − standard average)). So the per-step prices always add up to the total.
  - The standard level is free, and it is the lowest a board can go.
  - Stepping back down refunds the difference.
- **Raising one tier.** Players can also ask for one more point on a single tier instead of moving the slider. The staircase's `raises` (one per tier, in report order) give the first step above the shown one where that tier deals more:
  - its target and the tier's new damage;
  - the XP that climb adds, which is the difference between the two steps' level XP, so the same 40 XP per point of average applies;
  - the other tiers the climb raises on the way, because the board rises as a whole to stay balanced. Prepare and the Board Builder list them on the button and in its tooltip.
  - A tier already at the top has none (null).
  - On a new Starter board, for example, Gold Coin ×4 8 → 9 costs 1 XP, Sword ×4 5 → 6 costs 9 XP (and also lifts Gold Coin ×4), and Sword ×5 7 → 8 costs 40 XP (and lifts every other tier too).
- **Defenses.** The authority reads a defense's value from its outcomes on the board's die (`boardprice.ValueDefense`).
  - Flat defenses: the average prevention per roll.
  - Blocks: some faces prevent `BlockAmount` (100), the rest nothing. The value is the chance that at least one die rolls a block face.
  - A defense can't mix the two kinds.
  - Energy outcomes are not priced.
- **Block prices grow with the character.** A block stops a share of every hit, so it is worth more as damage grows. Its price is recomputed from the sheet's total XP whenever the sheet is read or XP is earned. For example, a victory worth 30 XP raises a 1-in-6 block's price by about 2–3 XP, which comes out of the XP to spend.

## The defense menu

The defense menu is in `content/progression_v1/board_templates.yaml` under `defenses`. All of these defenses roll the owner's die by roles, so they work on every board die. Every defense slot may also be left empty ("No defense", 0 XP).

| Defense | Dice | Outcome | Value | XP |
|---|---|---|---|---|
| Light Guard | 1 | Holdfast's role table | 7/6 per roll | 47 |
| Firm Guard | 2 | Holdfast's role table | 7/3 per roll | 93 |
| Holdfast | 3 | Holdfast's role table | 3.5 per roll | 140 |
| Holdfast+ | 3 | Holdfast's table, saved cards stay in their pile | 3.5 per roll | 178 |
| Bastion | 4 | Holdfast's role table | 14/3 per roll | 187 |
| Clean Deflect | 1 | Apex (face 6) prevents all damage | blocks 1 in 6 | ½ × (total + 200) ÷ 6 |
| Twin Deflect | 2 | Any apex prevents all damage | blocks 11 in 36 | ½ × (total + 200) × 11/36 |

The research found that blocks around 30% are worth about 11% more than this formula charges (an exact rate of 0.56 × the block chance). In a mixed field they ran a few points weak, so the shipped share is 0.5.

## The Starter's and Adventurer's board

In campaigns and expeditions, the Starter and the Adventurer level up their abilities the same way. `campaign.yaml` `boards` gives each one a board:

- **Die and template.** The standard die and the `starter` template in `board_templates.yaml`. The template has the same shapes as the authored board:
  - a Sword ×3/4/5 ladder (Strike);
  - Sword ×2 + Shield ×3 with +2 Protect (Guarded Strike);
  - Sword ×2 + Coin ×2 with +5 energy (Measured Strike);
  - the two straights;
  - Coin ×4 (Decisive Blow).
- **Default riders.** The two riders are the template's `default_rider`s.
- **Defense.** The board carries one defense. The default is Holdfast, which is the same as Guard on this die. Guard+ becomes Holdfast+, which costs Holdfast + 38 XP, like the Guard → Guard+ upgrade.
- **Names.** Generated abilities are named by their dice ("Sword ×3/4/5", "Short Run"), because ability names must be unique and the authored Strike, Guarded Strike, Measured Strike and the others keep theirs.
- **Damage at the standard level.** The pricing rounds the same shapes a little differently from the authored numbers, at the same 6.2 average:
  - Strike 4/5/7 instead of 4/5/6;
  - Guarded Strike 4 + 2 Protect instead of 5 + 2;
  - Measured Strike 5 + 5 energy instead of 4 + 5;
  - straights 6 and 7 instead of 6 and 8;
  - Decisive Blow 8 instead of 10.

**New saves** start on the board, the same way the Custom Adventurer's saves do:
- the board is part of the kit;
- the save has 150 XP to spend;
- Holdfast holds 140 XP.

**Saves from before boards** keep their authored abilities until their first board change (`loadout.AdoptBoard`):
- That change first moves the save onto its board as part of its kit. The total XP grows by the board's value, so the XP to spend doesn't change.
- `replaces` maps the authored defense to its menu defense (Guard → Holdfast, Guard+ → Holdfast+).
- XP already spent on Guard → Guard+ moves from `upgrade_spent` onto the board.
- After that move, the change itself is charged as usual.
- Prepare marks such a save with a "Moving onto the priced board" note (`moving` in `campaign_board`).

Battles pin the save's board for any character: `SheetBoard.Character` names whose board it is. The editors' Starter and Adventurer sheets in Character Creation keep their authored boards.

## The XP ledger

The board's XP is part of the sheet's total XP, like the deck:

`total = XP + DeckValue + CollectionValue + WoundValue + UpgradeSpent + BoardValue`

- `BoardLevelXP` is stored when the authority saves a board.
- `BoardValue` is BoardLevelXP plus each defense's price at the total. It is recomputed on every read.
- Authored boards hold no XP. A sheet holds a priced board (`loadout.HoldsBoard`) if it belongs to the Custom Adventurer, or if it has a board choice (a campaign save on its `campaign.yaml` board).
- **The starting board is part of the kit.** A new sheet adds its board's value to the total, the same as its deck. So a new Custom Adventurer still has the starting 150 XP to spend, and dropping Holdfast gives 140 XP back.
- **Migration.** Sheets saved before boards held XP (no `board_value`) add their current board's value to the total once, so their XP to spend doesn't change.
- A board change never moves the total XP. It moves XP between "to spend" and "on the board", and one the XP left can't pay for is refused.

## Where you buy it

- **Character Creation → Board Builder** (`board_builder.gd`):
  - ◀ / ▶ and the slider step the damage level from the standard level up. Each step shows its XP.
  - Each tier has a "+ to N · X XP" button that jumps to the cheapest level where that tier deals one more point; the tooltip names what else it raises.
  - Each defense slot lists every defense with its value and what picking it changes against the saved defense in that slot ("yours", "costs 47 XP", "93 XP back"), with its own price. ▶ cycles them.
  - The preview shows what the board holds and what saving costs.
  - Confirm (`board_save`) saves the level and charges or refunds the XP. It is disabled when the XP doesn't cover the board.
- **Campaigns and expeditions** (`campaign_prepare.gd`):
  - The Custom Adventurer can start a campaign (`campaign.yaml`). It starts from the Board Builder's die, template and slot picks, at the standard level with the template's default defense (`startBoard`). The editor's own level and defense are not copied, so everyone with that build starts with the same kit.
  - In Prepare, the deck list has an "Ability board" row. Selecting it lets you step the level up or down, raise one tier with its "Raise to N · X XP" button, pick each slot's defense, and Apply. Each defense button shows what it changes against the saved defense, the same way as in the Board Builder.
  - Campaigns keep the die, template and slot picks. Only the level and defenses change for now.
  - In an expedition, a board change counts as a deck change: it is allowed only at a site or camp, and the first change of a visit spends that visit's Influence and opens the deck.
- **Battles.** Campaign and expedition battles fight with the save's own die, board and generated abilities (`ResetRequest.CustomBoard`), never the editor's sheet. Replays record that board too.

## Ops

| Op | Reads / writes | Request | Result |
|---|---|---|---|
| `board_price` | reads (never creates a sheet) | `board` (choice with `level`, `defenses`) | `staircase.steps[].xp`, `staircase.lowest`, `staircase.raises[]` (per tier: step, at, from, to, average, xp, also), and `xp`: available, total, held, level_xp, value, cost, left, affordable, reason, defenses[] with xp/blocks/prevention/block and `change` (per slot: its price less the saved defense in that slot, negative for XP back), `none_change`, `saved_defenses`, rates |
| `board_save` | writes the editors' sheet | `board`, `revision` | `xp`, `xp_total`, `board_value` |
| `campaign_board` | reads a campaign or expedition save | `campaign_save`, optional `board` | `choice`, `saved`, `tiers`, `abilities`, `staircase` (with `raises`), `xp`, `defense_slots` |
| `campaign_board_change` | writes the save (expedition: at a site or camp, paying Influence) | `campaign_save`, `board`, `revision`, `expected_cost` (negative for a refund) | `xp`, `xp_total`, `board_choice`, `ability_board`, `view` (expedition) |

## Tests

- **Go:**
  - `boardprice/xp_test.go`: the menu defenses price the same on every die, the level XP climbs, empty slots and levels resolve.
  - `engine/role_outcomes_test.go`: `TestBoardDefensesResolveByRoleOnEveryDie`.
  - `learned/board_xp_test.go`:
    - the editor charges XP;
    - an expedition buys the level and a block at a site, paying Influence;
    - a campaign battle fights with the save's board;
    - no-defense and block boards fight full battles;
    - a block's price grows with earned XP;
    - a new Starter campaign holds its board, buys a level and fights with it;
    - an older Starter save moves onto its board at its first change, with its XP to spend unchanged and Guard+ becoming Holdfast+;
    - every tier's raise is the cheapest climb that raises it (`TestTierRaisesAreTheCheapestClimb`).
  - `learned/campaign_refund_test.go` `roundTripBoard`: every board change round-trips exactly.
- **Godot:**
  - `tests/presentation/verify_board_builder.gd`: step XP; a level the XP can't cover can't be confirmed; cycling the defense by pointer.
  - `tests/presentation/verify_campaign_board.gd`: Prepare's board by pointer in a campaign, at an expedition camp, and for a Starter expedition.
  - `tests/presentation/verify_campaign_prepare.gd`: Holdfast → Holdfast+ through the board, in place of the old Guard → Guard+ upgrade.

## Not built yet

- Rebuilding the die, template or slot picks during a campaign.
- A defense slider: more defense levels between the menu's fixed defenses.
- Pricing riders in XP (they still trade damage at the exchange rates).
- Enemy health or damage scaling with the player's level.
