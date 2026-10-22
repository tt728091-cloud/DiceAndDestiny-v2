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

**Ladder step (added 2026-10-22).** Each harder tier of one ability deals at
least 1 more damage than the tier below it (`ladder_step` in
`board_templates.yaml`); otherwise nobody chases 4 Sword when it pays what 3
Sword pays. The curve alone is too flat for that (the Starter's ladder prices
5.5 / 5.6 / 6.3), so before rounding a ladder too flat for the step is spread
around its average: the closest damages (least squares) that keep the step,
never below the minimum damage. The multiplier is then solved again so the
stepped board still averages the target, and only roundings that keep the
step are played. Rounding drift is measured against the stepped board.
`board_balance.py` has no step; the Go parity test runs without it.

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
  Since 2026-10-22 ladders keep the ladder step (above): Lean's Sword ladder
  went from 5 / 5 / 6 to 4 / 5 / 7, and 4 Sword is used on 24% of turns
  instead of 5.5%.
- **Generated abilities (B5).** `Builder.Generate`: IDs from the role cost,
  names from the cost in the die's symbols, full `rules_text`; `board_save`
  re-prices and writes them to the sheet.
- **Board Builder (B6).** A Character Creation tab for the custom
  adventurer: die picker with roles and odds, template picker with averages
  and no-hit rates, slot editor with live re-pricing, use bars and the keep
  table, Confirm through the authority.

### B7 results

**The lab's AI bug (found 2026-10-22).** The AI work found why every
non-Starter hero scored about 0.36 against the Starter: the greedy player
plays Nudge and Try Again on an arbitrary die for characters whose display
name sorts before "Play" (`mlsim` lists legal actions sorted by their text,
and a dice card's per-die targets then come first). "Custom Adventurer" and
"Adventurer" are both affected, so the earlier runs (2026-10-21, and the
first one with the ladder step) were skewed and are replaced here. These
runs use copies of the content with both characters renamed "Zz …", which
the bug does not touch. With that, every control scores 0.495, and the
Adventurer and the Starter give identical scores (Guard+ never decides a
battle), so the two reference columns below match. Rerun after the AI fix
lands.

**Findings** (2026-10-22, ladder step on):

- **At 1000 battles, 20 of the 23 default boards are inside 0.45–0.55.**
  Outside: Standard on the Loom Die (0.348) and Spellbook on the Forge Die
  (0.418), both weak, and Lean on the standard die (0.437), just under. The
  Grove Die boards sit low as a family (0.453–0.466). The 200-battle table
  further down counts 14 of 23 inside; its cells are only good to ±0.07.
- **The Hearth Die is slightly stronger than the standard die with the same
  board,** about 4 points of win rate on every ladder template (Lean +0.045,
  Standard +0.038, Wide +0.049, Double Ladder +0.035). Combo, which has no
  main ladder, goes the other way (−0.019). Every Hearth ladder board is
  inside the band (0.48–0.52); the standard-die ones sit at its low edge.
- **It is not Nudge.** With the custom adventurer's two Nudge swapped for
  two more Try Again, the gap is about the same (+0.035, +0.029, +0.043,
  +0.021). Pricing gives both dice the same average per turn (equal on
  Standard, Wide and Double Ladder; Lean's rounding leaves the standard die
  2% lower), so most of the gap comes from something pricing does not
  measure: how the AI plays each die, or something besides average damage
  per turn that decides battles. Not measured yet.
- **The 2-2-1-1 dice differ more:** Standard 0.458 on the Forge Die against
  0.348 on the Loom Die, Two Mains 0.537 against 0.478, Spellbook 0.418
  against 0.522.
- Every default board beats every minion (0.873–0.950).
- Random boards (random flex costs, a random rider on every slot that takes
  one) average 0.382. Every rider lowers the score a little per slot, +5
  energy the most (−0.077), Protect and Heal 1 least (about −0.02): at these
  rates riders are slightly overpriced for the greedy player, whose own
  values (Protect 0.5 per stack, energy 0.1 per point, `lookahead`
  `TierValue`) are below the pricing rates.
- The rider sweep finds no rider that is always best or always worst; "No
  rider" is best on 6 of 13 templates.
- Following the plan, nothing was tuned by hand. Before correcting any die
  or board: once the AI fix lands, measure each board's damage per offensive
  turn in battle against its priced average. If a die deals less than its
  price in the AI's hands, the gap is in the play (perhaps the Hearth Die's
  layout is simply easier to play well); if every board deals its price, a
  measured per-die correction belongs in the pricing, not in hand edits.
  Then the weak outliers above, the energy rate, and the Grove Die family.
  **Answered by the pure duel below:** the board and die alone show no
  Hearth edge, and what separates boards is the damage that gets through
  the defense, not the average.

**Pure board duel** (2026-10-22, `go run ./cmd/board-lab -pure`). This duel
measures the board and its die alone:
- **Both sides:** the custom adventurer's resources, a deck of blank cards (health only, nothing to play), Holdfast as the one defense, and no riders.
- **Opponent:** a reference board, the Standard template on the standard die (6.23 per turn). Greedy players, paired seeds, 2000 battles per board, so a cell is good to about ±0.02.
- **Scratch content:** the battles run on a copy of the content with a blank card and the reference character added; the real content is never written.
- **Controls:** the reference against itself scores 0.497, 0.510 and 0.499 in the three runs.

With it comes a **race model**. Each round both sides attack with the
board's priced odds per tier (`UseWhole`), the other side's defense roll
comes off, and the model computes the chance of reaching the other's health
first. It is exact and takes milliseconds. Across all 69 cells it predicts
the battles to within their noise: typical error 0.006–0.007, correlation
0.98–0.99. It can therefore stand in for the battles when checking a board.

- **The board and die alone show no Hearth edge.** Equal averages give
  equal scores on both dice: Standard 0.497/0.495, Compact 0.497/0.495, Wide
  0.495/0.504, Double Ladder 0.521/0.525. Lean differs (0.435 standard,
  0.499 Hearth) only because its rounding leaves the standard die at 6.10
  against 6.23; Combo's rounding goes the other way. The rest of the Hearth
  edge in the full lab comes from the cards (Try Again, Strong Swing and
  the extra prevention of Steady Guard), not from the board.
- **Average damage per turn is the wrong thing to equalize.** What decides a
  board is the damage that gets *through* the defense. Holdfast takes about
  3.5 of the ~6.2, so about 2.8 gets through. The score follows that number
  (correlation 0.97–0.98) far better than the average (0.62–0.65).
  - **Small gaps grow:** a 2% lower average is about 5% less through the
    defense and about 5 points of win rate. Lean on the standard die (6.10)
    scores 0.435 and Standard on the Loom Die (6.08) 0.447, both inside the
    pricing's 2% tolerance.
  - **Chunky boards gain:** a flat defense takes a bigger share of small hits,
    so boards that hit rarely but hard come out ahead. Combo on the standard
    die (5.6% no hit) scores 0.578, Spellbook on the Loom Die 0.599 and Two
    Mains on the Forge Die 0.558. The smooth Grove Die boards lose: Rainbow
    0.463, Suits 0.476.
  - **Longer battles widen every gap.** At 30 health the scores span
    0.414–0.651; at 15 they span 0.435–0.599.
  - **Without a defense the chunky boards flip to weak** (35 health, the same
    battle length): Spellbook on the Forge Die 0.416, Two Mains on the Loom
    Die 0.429. Every combatant in the game has a defense (the engine requires
    one), so this is only a stress test. It does mean no price on the
    average can make chunky and smooth boards equal against both strong and
    weak defenses.
- **Proposed fix:** price each board on its damage through a standard
  defense roll (Holdfast's, which equals Guard's 3.5 on the standard die),
  aiming every board at the reference's 2.78, with a tighter whole-number
  tolerance on that number. The race model then checks each board in
  milliseconds against 0.5 ± 0.02 at 15 health, and the full lab with cards
  is rerun afterwards to see what the cards add. Not implemented yet.

| Template | Die | Avg / turn | Through Holdfast | No hit | Holdfast, 15 health: model | battles | Holdfast, 30 health: model | battles | No defense, 35 health: model | battles |
|---|---|---|---|---|---|---|---|---|---|---|
| Lean | Standard D6 3-2-1 | 6.10 | 2.64 | 0.6% | 0.445 | 0.435 | 0.421 | 0.414 | 0.447 | 0.449 |
| Lean | Hearth 2-3-1 | 6.23 | 2.78 | 0.9% | 0.499 | 0.499 | 0.497 | 0.501 | 0.485 | 0.482 |
| Standard | Standard D6 3-2-1 | 6.23 | 2.78 | 0.8% | 0.500 | 0.497 | 0.500 | 0.510 | 0.500 | 0.499 |
| Standard | Hearth 2-3-1 | 6.23 | 2.78 | 0.9% | 0.499 | 0.495 | 0.499 | 0.506 | 0.498 | 0.499 |
| Compact | Standard D6 3-2-1 | 6.23 | 2.78 | 0.8% | 0.500 | 0.497 | 0.500 | 0.510 | 0.500 | 0.499 |
| Compact | Hearth 2-3-1 | 6.23 | 2.78 | 0.9% | 0.499 | 0.495 | 0.499 | 0.506 | 0.498 | 0.499 |
| Wide | Standard D6 3-2-1 | 6.23 | 2.78 | 0.9% | 0.500 | 0.495 | 0.500 | 0.506 | 0.500 | 0.499 |
| Wide | Hearth 2-3-1 | 6.25 | 2.80 | 0.9% | 0.508 | 0.504 | 0.511 | 0.515 | 0.503 | 0.498 |
| Double Ladder | Standard D6 3-2-1 | 6.32 | 2.85 | 0.5% | 0.528 | 0.521 | 0.540 | 0.537 | 0.525 | 0.522 |
| Double Ladder | Hearth 2-3-1 | 6.32 | 2.85 | 0.5% | 0.528 | 0.525 | 0.541 | 0.538 | 0.523 | 0.526 |
| Combo | Standard D6 3-2-1 | 6.31 | 3.01 | 5.6% | 0.581 | 0.578 | 0.617 | 0.612 | 0.517 | 0.507 |
| Combo | Hearth 2-3-1 | 6.23 | 2.91 | 5.1% | 0.543 | 0.538 | 0.564 | 0.562 | 0.477 | 0.480 |
| Rainbow | Grove 3-1-1-1 | 6.21 | 2.73 | 0.3% | 0.464 | 0.463 | 0.448 | 0.449 | 0.484 | 0.477 |
| Wide Rainbow | Grove 3-1-1-1 | 6.32 | 2.85 | 0.5% | 0.508 | 0.503 | 0.512 | 0.524 | 0.522 | 0.505 |
| Suits | Grove 3-1-1-1 | 6.23 | 2.75 | 0.2% | 0.472 | 0.476 | 0.459 | 0.461 | 0.491 | 0.495 |
| Standard | Forge 2-2-1-1 | 6.29 | 2.84 | 1.4% | 0.523 | 0.527 | 0.533 | 0.540 | 0.510 | 0.518 |
| Standard | Loom 2-1-2-1 | 6.08 | 2.65 | 1.8% | 0.446 | 0.447 | 0.422 | 0.421 | 0.450 | 0.456 |
| Two Mains | Forge 2-2-1-1 | 6.22 | 2.95 | 6.5% | 0.562 | 0.558 | 0.590 | 0.577 | 0.463 | 0.472 |
| Two Mains | Loom 2-1-2-1 | 6.08 | 2.77 | 5.4% | 0.496 | 0.490 | 0.494 | 0.478 | 0.436 | 0.429 |
| Spellbook | Forge 2-2-1-1 | 6.17 | 2.89 | 6.1% | 0.543 | 0.551 | 0.561 | 0.566 | 0.411 | 0.416 |
| Spellbook | Loom 2-1-2-1 | 6.31 | 3.05 | 7.0% | 0.600 | 0.599 | 0.643 | 0.651 | 0.504 | 0.501 |
| Ladder Pair | Forge 2-2-1-1 | 6.31 | 2.84 | 0.3% | 0.522 | 0.539 | 0.531 | 0.533 | 0.519 | 0.521 |
| Ladder Pair | Loom 2-1-2-1 | 6.31 | 2.84 | 0.2% | 0.522 | 0.533 | 0.531 | 0.534 | 0.521 | 0.529 |

Inside 0.45–0.55: 17 of 23 boards at 15 health with Holdfast, 15 at 30
health, and 20 with no defense. "Through Holdfast" is the average damage
per turn after the reference's Holdfast roll on the standard die. Ladder
Pair normally carries two defenses; here it has Holdfast only, like every
other board.

**Hearth and standard, with and without Nudge** (1000 battles per cell
against the reference; a cell is good to about ±0.03, a difference between
two cells to about ±0.045):

| Template | Die | Priced avg / turn | With Nudge | Without Nudge |
|---|---|---|---|---|
| Lean | Standard D6 3-2-1 | 6.10 | 0.437 | 0.444 |
| Lean | Hearth 2-3-1 | 6.23 | 0.482 | 0.479 |
| Standard | Standard D6 3-2-1 | 6.23 | 0.475 | 0.466 |
| Standard | Hearth 2-3-1 | 6.23 | 0.513 | 0.495 |
| Compact | Standard D6 3-2-1 | 6.23 | 0.475 | 0.466 |
| Compact | Hearth 2-3-1 | 6.23 | 0.513 | 0.495 |
| Wide | Standard D6 3-2-1 | 6.23 | 0.472 | 0.462 |
| Wide | Hearth 2-3-1 | 6.25 | 0.521 | 0.505 |
| Double Ladder | Standard D6 3-2-1 | 6.32 | 0.486 | 0.490 |
| Double Ladder | Hearth 2-3-1 | 6.32 | 0.521 | 0.511 |
| Combo | Standard D6 3-2-1 | 6.31 | 0.548 | 0.423 |
| Combo | Hearth 2-3-1 | 6.23 | 0.529 | 0.384 |
| Rainbow | Grove 3-1-1-1 | 6.21 | 0.455 | 0.424 |
| Wide Rainbow | Grove 3-1-1-1 | 6.32 | 0.466 | 0.455 |
| Suits | Grove 3-1-1-1 | 6.23 | 0.453 | 0.437 |
| Standard | Forge 2-2-1-1 | 6.29 | 0.458 | 0.424 |
| Standard | Loom 2-1-2-1 | 6.08 | 0.348 | 0.358 |
| Two Mains | Forge 2-2-1-1 | 6.22 | 0.537 | 0.406 |
| Two Mains | Loom 2-1-2-1 | 6.08 | 0.478 | 0.326 |
| Spellbook | Forge 2-2-1-1 | 6.17 | 0.418 | 0.325 |
| Spellbook | Loom 2-1-2-1 | 6.31 | 0.522 | 0.394 |
| Ladder Pair | Forge 2-2-1-1 | 6.31 | 0.483 | 0.467 |
| Ladder Pair | Loom 2-1-2-1 | 6.31 | 0.489 | 0.455 |

**Full lab run** (200 battles per cell):

`go run ./cmd/board-lab` · greedy hero (look-ahead's `GreedyPolicy`) · the Starter and the Adventurer as greedy players, minions by their own scripts · unified defense · paired seeds (each seed from both seats).

Score = (wins + ½ draws) / battles for the custom adventurer. Starter mirror band 0.45–0.55.

**Controls** (1000 battles each):

| Matchup | Score |
|---|---|
| Starter vs the Starter (greedy mirror) | 0.495 |
| Custom Adventurer with the Starter's own board, standard die, vs the Starter | 0.495 |
| The Adventurer (the Starter's board plus Guard+) vs the Starter | 0.495 |
| Custom Adventurer with the Starter's own board, standard die, vs the Adventurer | 0.495 |

**Every die × template at default picks** (200 battles per cell; band = inside 0.45–0.55):

| Template | Die | Avg / turn | No hit | Starter | Adventurer | Bell Diver | Brine Mask | Ribbon Eel | Starter band | Adventurer band |
|---|---|---|---|---|---|---|---|---|---|---|
| Lean | Standard D6 3-2-1 | 6.10 | 0.6% | 0.460 | 0.460 | 0.895 | 0.897 | 0.920 | in | in |
| Lean | Hearth 2-3-1 | 6.23 | 0.9% | 0.515 | 0.515 | 0.920 | 0.917 | 0.882 | in | in |
| Standard | Standard D6 3-2-1 | 6.23 | 0.8% | 0.477 | 0.477 | 0.907 | 0.892 | 0.912 | in | in |
| Standard | Hearth 2-3-1 | 6.23 | 0.9% | 0.545 | 0.545 | 0.917 | 0.927 | 0.902 | in | in |
| Compact | Standard D6 3-2-1 | 6.23 | 0.8% | 0.477 | 0.477 | 0.907 | 0.892 | 0.912 | in | in |
| Compact | Hearth 2-3-1 | 6.23 | 0.9% | 0.545 | 0.545 | 0.917 | 0.927 | 0.902 | in | in |
| Wide | Standard D6 3-2-1 | 6.23 | 0.9% | 0.485 | 0.485 | 0.905 | 0.897 | 0.910 | in | in |
| Wide | Hearth 2-3-1 | 6.25 | 0.9% | 0.570 | 0.570 | 0.927 | 0.932 | 0.917 | **out** | **out** |
| Double Ladder | Standard D6 3-2-1 | 6.32 | 0.5% | 0.495 | 0.495 | 0.930 | 0.900 | 0.938 | in | in |
| Double Ladder | Hearth 2-3-1 | 6.32 | 0.5% | 0.560 | 0.560 | 0.905 | 0.938 | 0.927 | **out** | **out** |
| Combo | Standard D6 3-2-1 | 6.31 | 5.6% | 0.565 | 0.565 | 0.925 | 0.950 | 0.930 | **out** | **out** |
| Combo | Hearth 2-3-1 | 6.23 | 5.1% | 0.560 | 0.560 | 0.932 | 0.945 | 0.938 | **out** | **out** |
| Rainbow | Grove 3-1-1-1 | 6.21 | 0.3% | 0.427 | 0.427 | 0.873 | 0.912 | 0.905 | **out** | **out** |
| Wide Rainbow | Grove 3-1-1-1 | 6.32 | 0.5% | 0.453 | 0.453 | 0.892 | 0.902 | 0.917 | in | in |
| Suits | Grove 3-1-1-1 | 6.23 | 0.2% | 0.425 | 0.425 | 0.887 | 0.902 | 0.882 | **out** | **out** |
| Standard | Forge 2-2-1-1 | 6.29 | 1.4% | 0.487 | 0.487 | 0.948 | 0.890 | 0.902 | in | in |
| Standard | Loom 2-1-2-1 | 6.08 | 1.8% | 0.340 | 0.340 | 0.900 | 0.902 | 0.873 | **out** | **out** |
| Two Mains | Forge 2-2-1-1 | 6.22 | 6.5% | 0.595 | 0.595 | 0.925 | 0.917 | 0.910 | **out** | **out** |
| Two Mains | Loom 2-1-2-1 | 6.08 | 5.4% | 0.470 | 0.470 | 0.910 | 0.912 | 0.920 | in | in |
| Spellbook | Forge 2-2-1-1 | 6.17 | 6.1% | 0.415 | 0.415 | 0.910 | 0.890 | 0.900 | **out** | **out** |
| Spellbook | Loom 2-1-2-1 | 6.31 | 7.0% | 0.530 | 0.530 | 0.950 | 0.925 | 0.922 | in | in |
| Ladder Pair | Forge 2-2-1-1 | 6.31 | 0.3% | 0.468 | 0.468 | 0.900 | 0.905 | 0.930 | in | in |
| Ladder Pair | Loom 2-1-2-1 | 6.31 | 0.2% | 0.525 | 0.525 | 0.895 | 0.900 | 0.907 | in | in |

- Against the Starter, 9 of the default boards are outside the band: Wide on the Hearth Die (0.570); Double Ladder on the Hearth Die (0.560); Combo on the Standard D6 (0.565); Combo on the Hearth Die (0.560); Rainbow on the Grove Die (0.427); Suits on the Grove Die (0.425); Standard on the Loom Die (0.340); Two Mains on the Forge Die (0.595); Spellbook on the Forge Die (0.415). Mean by family: {2211} 0.479; {3111} 0.435; {321} 0.521;
- Against the Adventurer, 9 of the default boards are outside the band: Wide on the Hearth Die (0.570); Double Ladder on the Hearth Die (0.560); Combo on the Standard D6 (0.565); Combo on the Hearth Die (0.560); Rainbow on the Grove Die (0.427); Suits on the Grove Die (0.425); Standard on the Loom Die (0.340); Two Mains on the Forge Die (0.595); Spellbook on the Forge Die (0.415). Mean by family: {2211} 0.479; {3111} 0.435; {321} 0.521;

**300 random boards** (`RandomBoard`, seed 7, 40 battles each; a single board's 95% interval is about ±0.15, so the spread includes noise):

| Opponent | Mean | Median | 10th–90th percentile | Inside the band |
|---|---|---|---|---|
| Starter | 0.382 | 0.388 | 0.212–0.550 | 73 of 300 |
| Adventurer | 0.382 | 0.388 | 0.212–0.550 | 73 of 300 |

Rider effect on the score against the Starter across the random boards (least squares on how many slots carry each rider; "No rider" is the baseline):

| Rider | Slots carrying it | Effect per slot | ±95% |
|---|---|---|---|
| +1 Protect | 275 | -0.016 | 0.016 |
| +2 Protect | 280 | -0.022 | 0.017 |
| +5 energy | 363 | -0.077 | 0.016 |
| +10 energy | 193 | -0.026 | 0.019 |
| Draw 1 | 292 | -0.029 | 0.017 |
| Heal 1 | 288 | -0.023 | 0.016 |
| Heal 2 | 182 | -0.033 | 0.020 |

Rider effect on the score against the Adventurer across the random boards (least squares on how many slots carry each rider; "No rider" is the baseline):

| Rider | Slots carrying it | Effect per slot | ±95% |
|---|---|---|---|
| +1 Protect | 275 | -0.016 | 0.016 |
| +2 Protect | 280 | -0.022 | 0.017 |
| +5 energy | 363 | -0.077 | 0.016 |
| +10 energy | 193 | -0.026 | 0.019 |
| Draw 1 | 292 | -0.029 | 0.017 |
| Heal 1 | 288 | -0.023 | 0.016 |
| Heal 2 | 182 | -0.033 | 0.020 |

**Rider sweep against the Starter** (each template's first flex slot, on the first die of its family, with every rider; 200 battles each):

| Template | Slot | No rider | +1 Protect | +2 Protect | +5 energy | +10 energy | Draw 1 | Heal 1 | Heal 2 | Best | Worst |
|---|---|---|---|---|---|---|---|---|---|---|---|
| Combo | flex_easy | 0.565 | 0.562 | 0.542 | 0.540 | 0.512 | 0.527 | 0.550 | 0.525 | No rider | +10 energy |
| Compact | flex_easy | 0.477 | 0.475 | 0.480 | 0.480 | 0.477 | 0.480 | 0.477 | 0.480 | +2 Protect | +1 Protect |
| Double Ladder | flex_medium | 0.495 | 0.492 | 0.497 | 0.477 | 0.477 | 0.490 | 0.492 | 0.500 | Heal 2 | +5 energy |
| Ladder Pair | flex_medium | 0.468 | 0.458 | 0.468 | 0.458 | 0.460 | 0.460 | 0.465 | 0.460 | No rider | +1 Protect |
| Lean | flex_a | 0.460 | 0.440 | 0.440 | 0.440 | 0.425 | 0.432 | 0.438 | 0.430 | No rider | +10 energy |
| Standard | flex_a | 0.487 | 0.475 | 0.472 | 0.472 | 0.475 | 0.475 | 0.463 | 0.443 | No rider | Heal 2 |
| Rainbow | flex_a | 0.427 | 0.430 | 0.435 | 0.427 | 0.430 | 0.430 | 0.430 | 0.427 | +2 Protect | No rider |
| Spellbook | flex_a | 0.415 | 0.417 | 0.417 | 0.403 | 0.417 | 0.415 | 0.420 | 0.415 | Heal 1 | +5 energy |
| Standard | flex_easy | 0.477 | 0.475 | 0.480 | 0.480 | 0.477 | 0.480 | 0.477 | 0.480 | +2 Protect | +1 Protect |
| Suits | flex_a | 0.425 | 0.427 | 0.422 | 0.425 | 0.420 | 0.420 | 0.420 | 0.420 | +1 Protect | +10 energy |
| Two Mains | flex_hard | 0.595 | 0.573 | 0.568 | 0.555 | 0.537 | 0.552 | 0.562 | 0.560 | No rider | +10 energy |
| Wide | flex_a | 0.485 | 0.485 | 0.485 | 0.485 | 0.485 | 0.485 | 0.485 | 0.485 | No rider | No rider |
| Wide Rainbow | flex_a | 0.453 | 0.472 | 0.470 | 0.465 | 0.458 | 0.463 | 0.465 | 0.468 | +1 Protect | No rider |

No rider is best or worst on every template. Best on: Heal 1 1; Heal 2 1; No rider 6; +1 Protect 2; +2 Protect 3; worst on: +10 energy 4; +5 energy 2; Heal 2 1; No rider 3; +1 Protect 3; (of 13 templates). A score's 95% interval is about ±0.07.

**Rider sweep against the Adventurer** (each template's first flex slot, on the first die of its family, with every rider; 200 battles each):

| Template | Slot | No rider | +1 Protect | +2 Protect | +5 energy | +10 energy | Draw 1 | Heal 1 | Heal 2 | Best | Worst |
|---|---|---|---|---|---|---|---|---|---|---|---|
| Combo | flex_easy | 0.565 | 0.562 | 0.542 | 0.540 | 0.512 | 0.527 | 0.550 | 0.525 | No rider | +10 energy |
| Compact | flex_easy | 0.477 | 0.475 | 0.480 | 0.480 | 0.477 | 0.480 | 0.477 | 0.480 | +2 Protect | +1 Protect |
| Double Ladder | flex_medium | 0.495 | 0.492 | 0.497 | 0.477 | 0.477 | 0.490 | 0.492 | 0.500 | Heal 2 | +5 energy |
| Ladder Pair | flex_medium | 0.468 | 0.458 | 0.468 | 0.458 | 0.460 | 0.460 | 0.465 | 0.460 | No rider | +1 Protect |
| Lean | flex_a | 0.460 | 0.440 | 0.440 | 0.440 | 0.425 | 0.432 | 0.438 | 0.430 | No rider | +10 energy |
| Standard | flex_a | 0.487 | 0.475 | 0.472 | 0.472 | 0.475 | 0.475 | 0.463 | 0.443 | No rider | Heal 2 |
| Rainbow | flex_a | 0.427 | 0.430 | 0.435 | 0.427 | 0.430 | 0.430 | 0.430 | 0.427 | +2 Protect | No rider |
| Spellbook | flex_a | 0.415 | 0.417 | 0.417 | 0.403 | 0.417 | 0.415 | 0.420 | 0.415 | Heal 1 | +5 energy |
| Standard | flex_easy | 0.477 | 0.475 | 0.480 | 0.480 | 0.477 | 0.480 | 0.477 | 0.480 | +2 Protect | +1 Protect |
| Suits | flex_a | 0.425 | 0.427 | 0.422 | 0.425 | 0.420 | 0.420 | 0.420 | 0.420 | +1 Protect | +10 energy |
| Two Mains | flex_hard | 0.595 | 0.573 | 0.568 | 0.555 | 0.537 | 0.552 | 0.562 | 0.560 | No rider | +10 energy |
| Wide | flex_a | 0.485 | 0.485 | 0.485 | 0.485 | 0.485 | 0.485 | 0.485 | 0.485 | No rider | No rider |
| Wide Rainbow | flex_a | 0.453 | 0.472 | 0.470 | 0.465 | 0.458 | 0.463 | 0.465 | 0.468 | +1 Protect | No rider |

No rider is best or worst on every template. Best on: Heal 1 1; Heal 2 1; No rider 6; +1 Protect 2; +2 Protect 3; worst on: +10 energy 4; +5 energy 2; Heal 2 1; No rider 3; +1 Protect 3; (of 13 templates). A score's 95% interval is about ±0.07.

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
