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

Results are recorded below, under "First pass · B7 results" (the handoff
moved them here from `balance-lab.md`).

### B8 — Campaign and expedition (medium)

- Die and board choice when a new campaign or expedition starts (decision 2),
  stored in the campaign save.
- Rebuild rules, if any, follow the run design. Wounds, XP and deck rules
  are unchanged.

## First pass (built 2026-10-21, B1–B7)

Built from `dice-and-board-selection-handoff.md` on branch
`dice-board-selection`. B8 (the choice at campaign and expedition start) is
not built.

- **Dice (B1).** Four new dice in the `custom_adventurer_v1` pack, one per
  family order: Hearth Die 2-3-1 (Spark, Cinder, Kiln), Grove Die 3-1-1-1
  (Bramble, Root, Bloom, Acorn), Forge Die 2-2-1-1 (Anvil, Tongs, Rivet,
  Ingot) and Loom Die 2-1-2-1 (Thread, Needle, Spindle, Knot); 3-2-1 reuses
  `standard_d6`. Their engraved glyphs are placeholders. `content.DieRoles`
  names each symbol's role from face counts (main, side, odd, odd_b, apex),
  and the loaders publish it in the catalog (`roles` on each die). The pack
  joins only battles the custom adventurer fights in: the frozen v2/v3
  observation encoders refuse catalogs with more than 8 symbols.
- **Custom adventurer (B2).** `custom_adventurer`: the Starter's deck and
  resources, the standard die and a default Lean board (exactly
  `boardprice.Generate` of Lean at default picks). Its sheet
  (`loadout.Progress`) holds the die (`dice`, five of it), the generated
  abilities (`generated_abilities`, `board_` IDs only) and the Board Builder
  choice (`board_choice`); battles pin them in their catalog. The ability
  workshop never publishes this character's board.
- **Role defenses (B2).** `roll_dice` may name `dice: owner` with outcomes
  by `roles`; the engine resolves roles from the die that rolled. Holdfast
  and Holdfast+ (Guard and Guard+ by role) prevent main 1, side 2, odd 1,
  second odd 3, and give 5 energy on any apex: 3.5 per 3-dice roll on every
  board die, as Guard does on the standard die. That is the only whole-number
  solution for one role defense on every family; the second odd symbol
  carrying 3 is the price of that.
- **Menus (B3).** `content/progression_v1/board_templates.yaml` (pricing
  numbers, bands, jobs, dice, defenses, flex cost menus, 13 templates) and
  `riders.yaml` (exchange rates and 8 riders). Every entry has `unlock: open`.
  Draw (1 per card) and heal (1 per card) rates are first guesses; heal
  riders use the new `heal_cards` operation (most recently removed card
  first, to discard, leaving its wound).
- **Pricing (B4).** `internal/battle/boardprice` ports `board_balance.py`
  (parity: Starter, Lean and Monk multipliers 5.4850, 5.4706 and 7.5231,
  whole damage exact, use shares within 0.0015). `board_price` and
  `board_builder` ops are read-only; a Wide board prices in about 30 ms.
- **Generated abilities (B5).** `Builder.Generate`: IDs from the role cost,
  names from the cost in the die's symbols, full `rules_text`; `board_save`
  re-prices and writes them to the sheet.
- **Board Builder (B6).** A Character Creation tab for the custom
  adventurer: die picker with roles and odds, template picker with averages
  and no-hit rates, slot editor with live re-pricing, use bars and the keep
  table, Confirm through the authority.

### B7 results

**Read the Starter column with the controls.** On this harness any hero
other than the Starter scores about 0.36 against the Starter even with the
Starter's own board, die and deck (the Adventurer too; reproduced bit for
bit on a clean `main` with the unchanged `cmd/balance-lab`), while every
mirror scores about 0.50. It is a pre-existing asymmetry of the greedy lab
(the AI work's area), not of the boards. The Adventurer, the Starter's board
plus Guard+, plays an equal board level (0.505), so it is the fair Starter
reference here.

**Findings.**

- Against the Adventurer, 18 of the 23 default boards are inside 0.45–0.55.
  Families: {3,2,1} 0.500, {2,2,1,1} 0.508, {3,1,1,1} 0.464 (slightly weak;
  all three Grove templates are at or below 0.477). Outside: Standard on the
  Hearth Die (0.407) and on the Loom Die (0.425) are weak; Double Ladder on
  the Hearth Die (0.588), Ladder Pair on the Forge Die (0.570) and Combo on
  the standard die (0.552) are strong.
- Every default board beats every minion (0.83–0.94).
- Random boards (random flex costs and riders on every slot that takes one)
  average 0.396 against the Adventurer, below the defaults (no riders).
  Every rider lowers the score a little per slot; energy the most (+5
  energy −0.05, +10 energy −0.04), then Heal 2 and Draw 1 (about −0.03),
  Protect least (+1 Protect about 0). At these rates riders are slightly
  overpriced for the greedy player, whose own values (Protect 0.5 per stack,
  energy 0.1 per point, `lookahead` `TierValue`) are below the pricing rates.
- The rider sweep finds no rider that is always best or always worst. "No
  rider" is best on 6 of 13 templates against either reference; the energy
  riders are the most frequent worst (7 of 13 against the Starter, 5 against
  the Adventurer).
- Following the plan, nothing was tuned by hand. Candidates once A4's
  measured rates land: a lower energy rate, and a small target raise for the
  {3,1,1,1} family.

`go run ./cmd/board-lab` · greedy hero (look-ahead's `GreedyPolicy`) · the Starter and the Adventurer as greedy players, minions by their own scripts · unified defense · paired seeds (each seed from both seats).

Score = (wins + ½ draws) / battles for the custom adventurer. Starter mirror band 0.45–0.55.

**Controls** (1000 battles each):

| Matchup | Score |
|---|---|
| Starter vs the Starter (greedy mirror) | 0.495 |
| Custom Adventurer with the Starter's own board, standard die, vs the Starter | 0.362 |
| The Adventurer (the Starter's board plus Guard+) vs the Starter | 0.362 |
| Custom Adventurer with the Starter's own board, standard die, vs the Adventurer | 0.505 |

**Every die × template at default picks** (200 battles per cell; band = inside 0.45–0.55):

| Template | Die | Avg / turn | No hit | Starter | Adventurer | Bell Diver | Brine Mask | Ribbon Eel | Starter band | Adventurer band |
|---|---|---|---|---|---|---|---|---|---|---|
| Lean | Standard D6 3-2-1 | 6.14 | 1.3% | 0.390 | 0.537 | 0.870 | 0.890 | 0.865 | **out** | in |
| Lean | Hearth 2-3-1 | 6.16 | 1.4% | 0.398 | 0.490 | 0.855 | 0.870 | 0.877 | **out** | in |
| Standard | Standard D6 3-2-1 | 6.16 | 1.3% | 0.357 | 0.475 | 0.855 | 0.877 | 0.855 | **out** | in |
| Standard | Hearth 2-3-1 | 6.23 | 1.7% | 0.315 | 0.407 | 0.865 | 0.848 | 0.833 | **out** | **out** |
| Compact | Standard D6 3-2-1 | 6.16 | 1.3% | 0.360 | 0.485 | 0.875 | 0.865 | 0.858 | **out** | in |
| Compact | Hearth 2-3-1 | 6.23 | 1.7% | 0.372 | 0.480 | 0.885 | 0.890 | 0.845 | **out** | in |
| Wide | Standard D6 3-2-1 | 6.19 | 1.3% | 0.367 | 0.515 | 0.870 | 0.905 | 0.877 | **out** | in |
| Wide | Hearth 2-3-1 | 6.24 | 1.1% | 0.352 | 0.482 | 0.890 | 0.873 | 0.880 | **out** | in |
| Double Ladder | Standard D6 3-2-1 | 6.25 | 0.9% | 0.270 | 0.455 | 0.840 | 0.873 | 0.835 | **out** | in |
| Double Ladder | Hearth 2-3-1 | 6.25 | 1.0% | 0.440 | 0.588 | 0.902 | 0.882 | 0.868 | **out** | **out** |
| Combo | Standard D6 3-2-1 | 6.31 | 5.6% | 0.445 | 0.552 | 0.890 | 0.902 | 0.873 | **out** | **out** |
| Combo | Hearth 2-3-1 | 6.23 | 5.1% | 0.398 | 0.530 | 0.887 | 0.895 | 0.843 | **out** | in |
| Rainbow | Grove 3-1-1-1 | 6.27 | 0.8% | 0.345 | 0.477 | 0.860 | 0.897 | 0.843 | **out** | in |
| Wide Rainbow | Grove 3-1-1-1 | 6.17 | 0.7% | 0.340 | 0.458 | 0.830 | 0.835 | 0.840 | **out** | in |
| Suits | Grove 3-1-1-1 | 6.29 | 0.5% | 0.292 | 0.458 | 0.868 | 0.882 | 0.833 | **out** | in |
| Standard | Forge 2-2-1-1 | 6.29 | 1.4% | 0.398 | 0.542 | 0.935 | 0.848 | 0.860 | **out** | in |
| Standard | Loom 2-1-2-1 | 6.08 | 1.8% | 0.280 | 0.425 | 0.845 | 0.870 | 0.853 | **out** | **out** |
| Two Mains | Forge 2-2-1-1 | 6.22 | 6.5% | 0.415 | 0.495 | 0.895 | 0.870 | 0.850 | **out** | in |
| Two Mains | Loom 2-1-2-1 | 6.08 | 5.4% | 0.357 | 0.458 | 0.848 | 0.845 | 0.825 | **out** | in |
| Spellbook | Forge 2-2-1-1 | 6.17 | 6.1% | 0.320 | 0.510 | 0.848 | 0.858 | 0.835 | **out** | in |
| Spellbook | Loom 2-1-2-1 | 6.31 | 7.0% | 0.412 | 0.532 | 0.895 | 0.887 | 0.873 | **out** | in |
| Ladder Pair | Forge 2-2-1-1 | 6.26 | 0.6% | 0.395 | 0.570 | 0.875 | 0.885 | 0.863 | **out** | **out** |
| Ladder Pair | Loom 2-1-2-1 | 6.26 | 0.5% | 0.378 | 0.532 | 0.895 | 0.885 | 0.865 | **out** | in |

- Against the Starter, 23 of the default boards are outside the band: Lean on the Standard D6 (0.390); Lean on the Hearth Die (0.398); Standard on the Standard D6 (0.357); Standard on the Hearth Die (0.315); Compact on the Standard D6 (0.360); Compact on the Hearth Die (0.372); Wide on the Standard D6 (0.367); Wide on the Hearth Die (0.352); Double Ladder on the Standard D6 (0.270); Double Ladder on the Hearth Die (0.440); Combo on the Standard D6 (0.445); Combo on the Hearth Die (0.398); Rainbow on the Grove Die (0.345); Wide Rainbow on the Grove Die (0.340); Suits on the Grove Die (0.292); Standard on the Forge Die (0.398); Standard on the Loom Die (0.280); Two Mains on the Forge Die (0.415); Two Mains on the Loom Die (0.357); Spellbook on the Forge Die (0.320); Spellbook on the Loom Die (0.412); Ladder Pair on the Forge Die (0.395); Ladder Pair on the Loom Die (0.378). Mean by family: {2211} 0.369; {3111} 0.326; {321} 0.372;
- Against the Adventurer, 5 of the default boards are outside the band: Standard on the Hearth Die (0.407); Double Ladder on the Hearth Die (0.588); Combo on the Standard D6 (0.552); Standard on the Loom Die (0.425); Ladder Pair on the Forge Die (0.570). Mean by family: {2211} 0.508; {3111} 0.464; {321} 0.500;

**300 random boards** (`RandomBoard`, seed 7, 40 battles each; a single board's 95% interval is about ±0.15, so the spread includes noise):

| Opponent | Mean | Median | 10th–90th percentile | Inside the band |
|---|---|---|---|---|
| Starter | 0.283 | 0.275 | 0.163–0.412 | 13 of 300 |
| Adventurer | 0.396 | 0.400 | 0.250–0.537 | 83 of 300 |

Rider effect on the score against the Starter across the random boards (least squares on how many slots carry each rider; "No rider" is the baseline):

| Rider | Slots carrying it | Effect per slot | ±95% |
|---|---|---|---|
| +1 Protect | 275 | -0.002 | 0.013 |
| +2 Protect | 280 | -0.017 | 0.013 |
| +5 energy | 363 | -0.045 | 0.012 |
| +10 energy | 193 | -0.030 | 0.015 |
| Draw 1 | 292 | -0.024 | 0.013 |
| Heal 1 | 288 | -0.010 | 0.012 |
| Heal 2 | 182 | -0.035 | 0.015 |

Rider effect on the score against the Adventurer across the random boards (least squares on how many slots carry each rider; "No rider" is the baseline):

| Rider | Slots carrying it | Effect per slot | ±95% |
|---|---|---|---|
| +1 Protect | 275 | -0.001 | 0.015 |
| +2 Protect | 280 | -0.015 | 0.015 |
| +5 energy | 363 | -0.051 | 0.014 |
| +10 energy | 193 | -0.042 | 0.017 |
| Draw 1 | 292 | -0.027 | 0.015 |
| Heal 1 | 288 | -0.010 | 0.014 |
| Heal 2 | 182 | -0.033 | 0.018 |

**Rider sweep against the Starter** (each template's first flex slot, on the first die of its family, with every rider; 200 battles each):

| Template | Slot | No rider | +1 Protect | +2 Protect | +5 energy | +10 energy | Draw 1 | Heal 1 | Heal 2 | Best | Worst |
|---|---|---|---|---|---|---|---|---|---|---|---|
| Combo | flex_easy | 0.445 | 0.412 | 0.432 | 0.388 | 0.385 | 0.417 | 0.422 | 0.440 | No rider | +10 energy |
| Compact | flex_easy | 0.360 | 0.375 | 0.375 | 0.357 | 0.357 | 0.360 | 0.365 | 0.375 | +1 Protect | +5 energy |
| Double Ladder | flex_medium | 0.270 | 0.328 | 0.335 | 0.318 | 0.315 | 0.323 | 0.335 | 0.305 | +2 Protect | No rider |
| Ladder Pair | flex_medium | 0.395 | 0.395 | 0.388 | 0.388 | 0.385 | 0.390 | 0.390 | 0.388 | No rider | +10 energy |
| Lean | flex_a | 0.390 | 0.400 | 0.403 | 0.385 | 0.388 | 0.403 | 0.398 | 0.383 | +2 Protect | Heal 2 |
| Standard | flex_a | 0.398 | 0.378 | 0.367 | 0.375 | 0.370 | 0.372 | 0.360 | 0.338 | No rider | Heal 2 |
| Rainbow | flex_a | 0.345 | 0.347 | 0.352 | 0.340 | 0.350 | 0.350 | 0.360 | 0.360 | Heal 1 | +5 energy |
| Spellbook | flex_a | 0.320 | 0.318 | 0.310 | 0.310 | 0.307 | 0.312 | 0.312 | 0.300 | No rider | Heal 2 |
| Standard | flex_easy | 0.357 | 0.357 | 0.362 | 0.350 | 0.352 | 0.350 | 0.355 | 0.367 | Heal 2 | +5 energy |
| Suits | flex_a | 0.292 | 0.343 | 0.338 | 0.300 | 0.320 | 0.347 | 0.352 | 0.360 | Heal 2 | No rider |
| Two Mains | flex_hard | 0.415 | 0.388 | 0.393 | 0.372 | 0.357 | 0.370 | 0.375 | 0.385 | No rider | +10 energy |
| Wide | flex_a | 0.367 | 0.357 | 0.352 | 0.350 | 0.350 | 0.350 | 0.352 | 0.360 | No rider | +5 energy |
| Wide Rainbow | flex_a | 0.340 | 0.360 | 0.383 | 0.347 | 0.362 | 0.352 | 0.362 | 0.372 | +2 Protect | No rider |

No rider is best or worst on every template. Best on: Heal 1 1; Heal 2 2; No rider 6; +1 Protect 1; +2 Protect 3; worst on: +10 energy 3; +5 energy 4; Heal 2 3; No rider 3; (of 13 templates). A score's 95% interval is about ±0.07.

**Rider sweep against the Adventurer** (each template's first flex slot, on the first die of its family, with every rider; 200 battles each):

| Template | Slot | No rider | +1 Protect | +2 Protect | +5 energy | +10 energy | Draw 1 | Heal 1 | Heal 2 | Best | Worst |
|---|---|---|---|---|---|---|---|---|---|---|---|
| Combo | flex_easy | 0.552 | 0.573 | 0.580 | 0.555 | 0.520 | 0.562 | 0.573 | 0.595 | Heal 2 | +10 energy |
| Compact | flex_easy | 0.485 | 0.455 | 0.465 | 0.438 | 0.458 | 0.445 | 0.448 | 0.458 | No rider | +5 energy |
| Double Ladder | flex_medium | 0.455 | 0.477 | 0.475 | 0.480 | 0.470 | 0.480 | 0.495 | 0.455 | Heal 1 | No rider |
| Ladder Pair | flex_medium | 0.570 | 0.575 | 0.580 | 0.580 | 0.578 | 0.573 | 0.583 | 0.583 | Heal 1 | No rider |
| Lean | flex_a | 0.537 | 0.547 | 0.515 | 0.532 | 0.507 | 0.525 | 0.535 | 0.525 | +1 Protect | +10 energy |
| Standard | flex_a | 0.542 | 0.522 | 0.507 | 0.520 | 0.502 | 0.497 | 0.495 | 0.477 | No rider | Heal 2 |
| Rainbow | flex_a | 0.477 | 0.480 | 0.500 | 0.480 | 0.490 | 0.495 | 0.495 | 0.490 | +2 Protect | No rider |
| Spellbook | flex_a | 0.510 | 0.492 | 0.468 | 0.482 | 0.463 | 0.475 | 0.485 | 0.480 | No rider | +10 energy |
| Standard | flex_easy | 0.475 | 0.455 | 0.463 | 0.453 | 0.453 | 0.450 | 0.455 | 0.458 | No rider | Draw 1 |
| Suits | flex_a | 0.458 | 0.520 | 0.485 | 0.475 | 0.463 | 0.492 | 0.505 | 0.465 | +1 Protect | No rider |
| Two Mains | flex_hard | 0.495 | 0.465 | 0.443 | 0.438 | 0.430 | 0.432 | 0.438 | 0.463 | No rider | +10 energy |
| Wide | flex_a | 0.515 | 0.470 | 0.482 | 0.470 | 0.485 | 0.485 | 0.482 | 0.487 | No rider | +1 Protect |
| Wide Rainbow | flex_a | 0.458 | 0.468 | 0.482 | 0.482 | 0.482 | 0.482 | 0.477 | 0.492 | Heal 2 | No rider |

No rider is best or worst on every template. Best on: Heal 1 2; Heal 2 2; No rider 6; +1 Protect 2; +2 Protect 1; worst on: Draw 1 1; +10 energy 4; +5 energy 1; Heal 2 1; No rider 5; +1 Protect 1; (of 13 templates). A score's 95% interval is about ±0.07.

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
