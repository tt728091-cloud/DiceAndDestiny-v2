# Player die and board selection plan

Plan for letting the player choose an offensive die, then an ability-board
layout, with each ability's payoff priced from the die and the board together.
Harder abilities pay more, and every combination averages the same per turn.

Status: decisions agreed, ready to build (2026-10-21). Build brief:
`dice-and-board-selection-handoff.md`. Research and pricing method:
`dice-throne-reference/ability-layout-analysis.md` and
`dice-throne-reference/analysis/board_balance.py`. Companion plan:
`ai-value-network-plan.md`, which supplies measured rider values (A4) and the
AI that plays and tests these boards.

## The player's choices

1. **Die.** One of the die families, shown as a strip of six faces with each
   symbol's role:

   | Family | Orders | Roles |
   |---|---|---|
   | {3,2,1} | 3-2-1, 2-3-1 | main 3, side 2, apex 1 |
   | {3,1,1,1} | 3-1-1-1 | main 3, three 1-face (apex on face 6) |
   | {2,2,1,1} | 2-2-1-1, 2-1-2-1 | two 2-face, two 1-face (apex on face 6) |

2. **Board template** for that family (from the analysis):

   | Family | Templates |
   |---|---|
   | {3,2,1} | Lean, Standard, Compact, Wide, Double Ladder, Combo |
   | {3,1,1,1} | Rainbow, Wide Rainbow, Suits |
   | {2,2,1,1} | Standard, Two Mains, Spellbook, Thief |

3. **Flex slots.** For each open slot the player picks a cost from the
   family's menu, within the slot's difficulty band (Easy ≥ 75%, Medium
   50–75%, Hard < 50%), and a **rider** from a rider menu (Protect, energy,
   draw, heal, a status).

4. **Preview.** The authority prices the board and shows:
   - each ability's damage plus rider;
   - how often each ability is used;
   - the average per turn and the no-hit rate;
   - the keep policy, like the Monk page.

   The player confirms, and the abilities are written to their sheet.

The core slots are fixed per template, apart from their riders:

- main ladder;
- straights;
- apex ×4;
- ultimate.

## How a slot is priced

The method agreed for `board_balance.py`:

1. Relative payoff = (1 ÷ chase odds) ^ 0.2, anchored on a guaranteed hit.
2. Play the board exactly (every hand, every keep, 3 rolls) to get use shares.
3. Multiply every payoff by target ÷ Σ(use × relative). The target is the
   Starter's level, 6.2 per turn.
4. Round to whole damage, keeping harder ≥ easier, within 2% of the target.
   Play the rounded board again and report it.

**Riders.** A slot's priced value is damage plus its rider at the exchange
rates: today Protect 1 and energy 0.2, replaced by measured rates from A4.
Damage = value − rider, with a floor so no attack deals 0. Riders change play,
so pricing always plays the board with the riders included; that is how
`EXCHANGE` already works.

The Monk check showed why the method matters. Its printed board averages 8.58
but leaves the Fist ladder nearly unused and overprices LS and 5 Lotus. Priced
on the curve, every slot is used.

## What exists and what is missing

| Need | Today | Gap |
|---|---|---|
| Dice | `BattleDieDefinition` in pack `dice/`. Playable dice are all 3-2-1 (`standard_d6`, `venom_d6`, `curse_d6`). | No {3,1,1,1} or {2,2,1,1} player die, no 2-3-1 order, and no symbols or art for them. |
| Die on the player's sheet | `CombatantDefinition.dice_loadout` (5× own die) | The sheet (`loadout.Progress`) has no dice field. The die comes only from combatant content. |
| Ability requirements | Symbol counts, number patterns (pairs, straights), exact faces, tiers | None. Every template cost can be authored today. |
| Board on the sheet | `loadout.Progress.Abilities` overrides the combatant's board | Works. Generated abilities need storage (below). |
| Ability fits dice | `engine.AbilityFitsDice` | Works. |
| Damage pricing | Python script only | No Go pricing and no op. Damage is hand-authored. |
| Defense on the chosen die | Guard / Guard+ roll `standard_d6` by `dice_id` | Defense must follow the player's die, or each die needs its own defense outcomes. |
| Abilities naming symbols | Adventurer abilities name `sword` / `shield` / `gold_coin` | Generated abilities use the chosen die's symbol IDs. General cards name no symbols. |
| AI playing any board | Dice solver uses the engine's qualification (identical dice only) | Works for 5 identical dice. Its `TierValue` rider rates differ from the exchange rates. |
| Max rolls | `MaxRolls: 3` hard-coded | Fine. Pricing assumes 3. |

## Decisions (agreed 2026-10-21)

1. **Riders and costs: open in the first pass.**
   - The player picks each flex slot's cost and each slot's rider freely
     from the menus.
   - Later they will be limited the way gear is: few choices at the start,
     more unlocked as the character gains power.
   - So every menu is data with a place for an unlock rule, and in the first
     pass every option is unlocked.
2. **Where: Character Creation first, then campaign start.**
   - First a "Board Builder" tab in Character Creation (admin and sandbox).
   - Then the choice at the start of a new campaign or expedition.
   - Rebuilding during a run (at camp, for XP) comes later.
3. **A new custom adventurer owns the chosen die.**
   - Venom, Curse and Blade Warden keep their authored boards, because their
     passives assume their dice.
4. **Defenses are a design space as wide as offensive abilities.** The first
   pass only has to make defense work on any die:
   - A defense rolls the owner's die, with outcomes written by symbol role
     (main / side / apex / odd) instead of face numbers.
   - Today Guard names `standard_d6` and lists outcomes by face number
     (1–3 prevent 1, 4–5 prevent 2, 6 more). Those only match the roles on
     a 3-2-1 die.
   - The first defense menu holds role-based versions of the existing
     defenses. They are priced so their expected prevention on the chosen die
     equals Guard's on the 3-2-1 die.
   - The menu is data, so wider defenses (statuses, counters, energy, cards)
     are added later without code changes. Their pricing is a later design.
5. **Generated abilities are stored with the player.**
   - They live in the loadout or campaign save, not in the tracked
     `content/authored/` directory.
   - Each battle pins them in its catalog like any other ability, so saves
     and replays keep the board they started with.
6. **Upgrades are a later design.** Pricing at a higher target, for example
   6.2 → 7, would give the same board scaled up, which is a candidate for
   levels.

## Phases

### B1 — Dice catalog (medium)

- One playable die per family order:
  - 3-2-1 (`standard_d6` already);
  - 2-3-1;
  - 3-1-1-1;
  - 2-2-1-1;
  - 2-1-2-1.
- New symbols with names, icons and art. All faces are numbered 1–6 in face
  order, so straights work unchanged.
- **Content tests:** every die loads, and its roles come out as expected.

### B2 — Die on the sheet (medium)

- A dice field on the player's sheet that overrides the combatant's
  `dice_loadout` (always 5 of one die).
- Board validation runs `AbilityFitsDice` against the chosen die.
- Defense rolls follow decision 4.
- **Tests:**
  - a battle with a non-standard die completes in the authority;
  - save / reload keeps the die;
  - the dice tooltips and hover contract show the right faces.

### B3 — Templates and riders as content (small)

- `content/progression_v1/board_templates.yaml`:
  - each template's slots, with role-based costs (`main3`, `apex4`, `SS`,
    and so on);
  - the slot's job and difficulty band;
  - the menu of costs each flex slot may take, per family.
- `content/progression_v1/riders.yaml`:
  - each rider's operations;
  - its exchange rate;
  - which slot jobs may take it.
- Strict loaders, and every shipped template must price inside its target.

### B4 — Pricing in Go (medium)

- **A pricing solver in a new package, `internal/battle/boardprice`.**
  - It is the exact solver without cards: multisets, keeps, backward
    induction.
  - It decides qualification with the engine's own rules.
  - It must not import or change `internal/battle/lookahead`, which belongs
    to the AI work.
  - Merging the two solvers is a later refactor on the AI side.
- **Port the four pricing steps.** Parity tests against `board_balance.py`
  outputs:
  - Starter 6.2 / turn;
  - the Lean table;
  - the Monk table (printed 8.58).
- **A `board_price` op.** Given a die, template and slot picks, it returns:
  - damage and rider per ability;
  - use shares (exact and rounded);
  - average and no-hit rate;
  - the keep policy for the preview.

  It only reads; the client never computes prices.
- **A random board generator** (seeded) for lab tests and for the AI plan's
  training data (A1).

### B5 — Generated abilities (medium)

- Turn a priced board into real ability definitions with the chosen die's
  symbol IDs.
- Write `presentation.rules_text` with the dice requirement, damage and rider,
  following the ability hover contract.
- Store them per decision 5. A battle pins them like authored abilities.
- **Tests:**
  - every generated ability validates;
  - it qualifies on its die;
  - it shows the full hover text.

### B6 — Board Builder screen (large)

- Die picker:
  - face strip;
  - symbol roles;
  - chase odds of each cost.
- Template picker:
  - slot list with difficulty bands;
  - each template's average and no-hit rate.
- Slot editor (cost and rider). Each change calls `board_price` for a live
  preview, with use bars and the keep table on demand.
- Confirm writes the board through the authority. Pointer and headless
  checks through `./scripts/godot.sh`.

### B7 — Lab validation (medium)

The 6.2 average is a static check. The battle is the real one, because
defense, cards, riders and statuses all interact.

- **Matrix:** every die × template at default picks, plus a few hundred
  random boards from B4.
- **Opponents:** Starter mirror, Brine Mask and the other minions.
- **Players:** greedy, plus look-ahead for the close cases. Run them from a
  new `cmd/board-lab` that uses only the public API of `lookahead`, which
  belongs to the AI work.
- **Pass:** every default board scores within an agreed band of the Starter
  (for example 0.45–0.55 in the mirror), and no rider choice is always best
  or always worst.
- If a whole family runs strong or weak, adjust its target. If one slot
  does, adjust its rider rate. Never hand-edit one ability's damage.

Record results in `balance-lab.md`.

### B8 — Campaign and expedition (medium)

- Die and board choice when a new campaign or expedition starts (decision 2),
  stored in the campaign save.
- Rebuild rules, if any, follow the run design. Wounds, XP and deck rules
  are unchanged.

## Links to the AI plan

| AI plan phase | Gives this plan | Needs from this plan |
|---|---|---|
| Now (v2 look-ahead) | Plays and tests any 5-identical-dice board in B7 | — |
| A1 data | — | B4's random board generator, so the network learns many dice and boards |
| A4 rider values | Measured exchange rates to replace Protect 1 / energy 0.2 before B7's final pricing | Boards with riders to measure on |
| A3 value player | A stronger, faster tester for B7 and the in-game mirror | — |

**Small early fix.** Align the AI's `TierValue` rates (status 0.5 per stack,
energy 0.1) with the board exchange rates, so the AI chases riders the way
the pricing assumes. Do it behind a greedy lab test; A4 later sets both from
measurements.

**Suggested order:**

1. Decisions.
2. B1–B4 and A0–A2 in parallel.
3. A3 and A4.
4. B5–B7, with the measured rates.
5. B8.
