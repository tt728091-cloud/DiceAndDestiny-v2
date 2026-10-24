# Balancing playbook

How to tell whether cards, decks and their prices are still balanced, and how to re-measure them after the game changes. This is the reference for future balancing passes. It covers:
- **When to run one**, and which test answers which question.
- **The commands to copy.**
- **The mistakes already made once.**

The evidence behind every number here is in [balance-lab.md](balance-lab.md). The pricing rules are in [card-value-rules.md](card-value-rules.md), and board pricing in [dice-and-board-selection-plan.md](dice-and-board-selection-plan.md).

## When to run a balancing pass

Run one when any of these change. The test to run is in brackets; the tests are in "Tests" below.

| What changed | What to re-measure |
|---|---|
| A card's energy cost, effect or XP | That card at its new values (2), then every other card in the decks that use it (1). Costs share one energy pool, so neighbours move too. |
| A new card or tree enters a deck or the General pool | The new card (1); a price check with decks at equal price (4) |
| Tree upgrades (variants) re-costed or re-priced | Each changed variant against its base (1 or 2) |
| The energy economy: starting energy (10), income (+5 a round), cap, costs that cross 10 | Everything (1); the threshold checks in (2) |
| Hand size, draw rules, Income draw, deck size or health | Everything (1); whole decks in blanks (5) |
| Battle rules: defense flow, damage order, simultaneous play, new statuses | Deck on top of the board (6); everything (1) |
| A board, die or ability changes | The board alone against its estimate (7); damage through the defense (board docs) |
| The in-game AI changes (a new look-ahead version, or the network) | Re-baseline (1) with the new player: card values depend on who plays them |
| The opponents change (new minions or Warlord, a different Expedition fight mix) | (1) against the fights that matter, with health left for minions (8) |
| Before a release or a playtest round | (1) and (4) on the current decks |

## The yardstick: blank cards

- **A blank card** is pure health: Trailblaze, a map card that can never be played in battle but still counts as health and can be lost to damage. A deck plays like "N blank cards".
- **What a card is worth:** swap all its copies for *m* blanks and find the *m* that scores what the original deck scores. Divide by the copies to get blanks per copy.
- **XP from blanks:** 1 blank = 10 XP, the price card-value-rules.md gives a card's body (1 health). The XP that matches a card is 10 × its blanks per copy. A 15-XP card should be worth 1.5 blanks.
- **Whole decks:** a deck plays like the all-blank deck with its total value. For example, the Starter deck on the original costs played like about 15½ blanks, and a 15-blank deck scored 0.468, against 0.497 for the Starter deck.

**Why this yardstick:** it measures every card in one unit, whatever it does (damage, prevention, draw, energy), and in the context of a real deck and real battles.

## Which player

**The in-game AI** (`lookahead@rounds=1@budget=400`, version 3) is the reference. It is slow: about 25–30 battles a minute on 15 workers, so 600 battles take 20–30 minutes.

**Greedy** (`greedy`) is about a hundred times faster: 2,000 battles take about 10 seconds alone on the machine. On dice cards (Nudge, Try Again, Strong Swing, Second Wind) it agrees with the in-game AI within the noise. It has blind spots, though; it never plays:
- Steady Guard;
- Take Stock costing 3 or more;
- any Curse, Venom or Blade Warden card.

A card it never plays measures exactly 1 blank. That's a useful sanity check, but not a value.

**So:** sweep with greedy, use the in-game AI for the cards greedy misplays, and confirm the final numbers with the in-game AI. Never price a card from a player that doesn't play it.

## Which opponent

| Opponent | Use it for | Notes |
|---|---|---|
| Starter mirror (greedy foe, its normal deck) | Close fights; the default | Battles last about 3.6–3.9 rounds. 1 blank ≈ 0.065–0.09 win score. |
| The Warlord (`drowned_regent`, `-foe script`) | An even boss fight (the Starter scores about 0.5) | Attack cards are worth about 0.2 blanks less than in the mirror; Second Wind about a blank. |
| Weak minions (Brine Mask, Bell Diver) | Expedition fights | The Starter wins about 90%, so win rates bunch together. Measure **health left** instead (wounds carry over): `--metric=health`. Effects are worth only 1.0–1.3 blanks there. |

Avoid matchups the hero wins 80% or more of the time when measuring by wins: the differences shrink as scores approach the top.

## Tests

Build the lab once per code change, then run from `dice-and-destiny-server/`:

```bash
go build -o ml/runs/bin/balance-lab ./cmd/balance-lab
```

A **plan** is a JSON file listing deck variants (see `ml/scripts/deck_sweep.py` for the format). `deck_sweep.py` plays every variant and records a manifest. `card_exchange_report.py` turns the results into blanks and XP. Keep plans and results under `ml/runs/` (ignored by git).

The standard Starter deck, used in the examples:

```json
{"steady_guard": 3, "nudge": 2, "try_again": 2, "strong_swing": 2, "take_stock": 2, "second_wind": 1}
```

### 1. Each card's value in blanks

Variants needed:
- `std+0` … `std+6`: the standard deck plus 0–6 blanks (`"trailblaze": k`). This is the blank curve.
- `<card>-to-<m>`: every copy of the card replaced by *m* blanks, for *m* = 0 to 2 × copies + 2.

```bash
python3 ml/scripts/deck_sweep.py ml/runs/<pass>/plan-greedy.json          # player greedy, 1,000 seeds (2,000 battles) per variant
python3 ml/scripts/card_exchange_report.py ml/runs/<pass>/greedy          # blanks per copy, XP at 10 per blank
```

- **Sizes and precision:**
  - **Greedy:** 2,000 battles per variant gives about ±0.15 blanks per copy (95%).
  - **The in-game AI:** use `std+0` (600 battles) plus two bracketing variants per card (400–600 battles each), for about ±0.2–0.4 per copy. Pick brackets around the greedy or estimated value, for example Steady Guard to 3 and 6 blanks, Take Stock to 2 and 4.
- **Other characters:** set `"hero"` and `"opponent"` in the plan and use their own decks. Their decks are many single copies, so measure groups of cards (block a role at once) rather than one card at a time, with the in-game AI.

### 2. A card at other energy costs

```bash
python3 ml/scripts/card_cost_variants.py ml/runs/<pass>/authored nudge:6,7,8,9,10 take_stock:1,2,3   # scratch published cards, new IDs like nudge_e9
# plan: "authored_root": "../authored", variants "std" (the normal deck) and "<card>@<energy>" (the card swapped for <card>_e<energy>),
# including the card at its current cost as the control
python3 ml/scripts/deck_sweep.py ml/runs/<pass>/plan-costs.json
python3 ml/scripts/card_exchange_report.py ml/runs/<pass>/costs --costs=ml/runs/<pass>/greedy --now=nudge:8,try_again:2,strong_swing:9,steady_guard:3,take_stock:3,second_wind:10
```

- **Pairing:** each cost is compared battle by battle with the control at the current cost, which is about three times tighter than comparing absolute values. Use 3,000 seeds (6,000 battles) per cost with greedy.
- **`--now`** must list the cards' current costs. These are the controls; the script's defaults are the costs before 2026-10-23.
- **Only the hero changes.** The opponent keeps the normal cards, so this answers "what if only this card changed". To change a card for both sides, see test 3.

### 3. A new set of costs for everyone

To try costs on both sides without touching the real cards:
1. Copy `content/authored` into a scratch folder and edit the costs there.
2. Point every plan at it with `"authored_root"`.
3. Re-run test 1 in full.

Only when a set is chosen, apply it to `content/authored/authored_cards.json`:
- edit both the card entry and its tree's base node;
- bump `revision` (the card editor uses it to detect conflicting saves);
- update card-value-rules.md §6 and the tree tables;
- run `go test ./...` and `./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd`.

### 4. Checking a set of prices

- **Swaps the measured prices call even should play even.** Build decks that replace cards with others (plus blanks) at equal measured value, for example 2 Nudges → 2 Try Agains + 1 blank. Check each scores like the standard deck, paired battle by battle.
- **Swaps today's prices call even should differ by what the measured prices predict.**
- **Then do the same with the in-game AI** for the two most different equal-price decks.

### 5. Whole decks in blank cards

Add all-blank decks (`blank-10` … `blank-20`) to a plan as the yardstick, and name the decks to value `old-…` or `new-…`. The report gives each one's value in blanks, the score that value predicts, and the score it got:

```bash
python3 ml/scripts/card_exchange_report.py ml/runs/<pass>/decks --rates=ml/runs/<pass>/greedy
```

### 6. What a deck adds on top of its board

```bash
ml/runs/bin/balance-lab -hero starter -opponent starter -foe greedy -seats both -battles 100 -workers 15 \
  -players "lookahead@rounds=1@budget=400,lookahead@rounds=1@budget=400@block-cards=all" -out ml/runs/<pass>/starter.json
python3 ml/scripts/card_value_report.py ml/runs/<pass>
```

`-block-cards=all` (or `=card+card`) keeps a player from playing those cards. They stay as health, and its dice solver stops planning with them. The report gives attack before defense, damage landed, prevention and damage taken per turn, with paired differences.

### 7. A board alone against its estimate

The no-card run of test 6 gives the board's damage per turn before defense. Compare it with the board calculator (`docs/game-design/dice-throne-reference/analysis/board_balance.py --costs … --printed …`), using the AI's own values for Protect and energy. For the Starter they agreed within 1%: 5.87 measured against 5.86 estimated, and 1.5% against 1.4% misses.

### 8. Expedition fights

Run test 1 against the minions with `"foe": "script"`, and analyse with `--metric=health`. A card is then worth the health it saves compared with a blank.

## Running long jobs

- **Background jobs stop after 2 hours.** `deck_sweep.py` skips variants already saved, so a stopped sweep resumes by running it again. Split long in-game AI work into plans of under 2 hours, or hand over to a second job once one variant finishes.
- **Sharing the machine:** run greedy at low priority (`nice -n 10`, 4–5 workers) next to the in-game AI (11 workers), so the slow runs keep most of the machine.
- **Scratch files:** scratch published cards and results stay under `ml/runs/`. Never write `content/authored` during an experiment.

## Lessons (don't repeat these)

**Energy:**
- **Change one thing at a time, then measure everything again.** A cost change moves every other card's value. Cheaper cards make battles shorter and put more cards into play, and every card draws on the same energy pool. Re-costing four cards at once raised even the unchanged cards: Second Wind went from 1.28 to 1.59 blanks.
- **Single points of energy matter, most at thresholds:** what fits in the starting 10, and what fits in a round's +5.
  - **Pairs that sum to 10 play better:** Nudge 7 + Try Again 3, or Nudge 8 + Try Again 2.
  - **More than 10 hits a cliff:** Strong Swing at 11 is played half as often, and at 12 almost never.
  - **The old claim that costs of 1–4 play like 5 was wrong.** It came from a player that misplayed those cards.
- **Don't tune energy alone toward a price.** The first re-cost did that and bunched the costs at 0–1 and 10. Set the costs for feel and spread, then set the XP from measurement.
- **Energy-gain cards depend on the deck around them.** Second Wind is worth more in decks of expensive cards. Its cost hardly matters, because it pays you back; only its net gain counts, and more than +5 adds little to the Starter deck.

**Copies and decks:**
- **Copies are linear.** One copy, a third or a fourth is each worth the same.
- **Piling up one card loses a little:** decks with 4 Nudges or 3 Second Winds come in 0.03–0.06 under the sum of their cards.
- **Deck size doesn't change values when both sides grow.** Giving only one side more health pushes scores toward the top, where differences shrink.

**Boards:**
- **Damage through the defense decides fights more than average damage.**

**Measurement traps:**
- **A no-card player must also stop its dice solver planning with the cards it holds** (`SkipCards`, set by `-block-cards`). Otherwise it chases straights for Nudges it can't play and misses 5% of its turns instead of 1.4%.
- **The simulator's own `damage_by_seat` metric undercounts in training mode.** The lab reads damage from the battle's event stream instead.
- **Tests that hard-code a card's cost:** pin the cost inside the test (as `TestMapCardsAndEnergyRules` and `TestDiceSolverKeepsNudgesForLater` do), never loosen the assertion.

## Current state (2026-10-23)

These changes are on the branch `ai-value-network`, not yet merged into `main`.

**The six Starter base cards are re-costed and re-priced, and each whole tree moved with its base after the user reviewed it** (card-value-rules.md §6–7):

| Card | Energy | Measured, mirror (blanks per copy) | XP | How its tree moved |
|---|---|---|---|---|
| Nudge | 8 | 1.77 | 18 | +1 energy, +3 XP (Fate and Destiny too, also in Disrupt's tree) |
| Strong Swing | 9 | 1.64 | 16 | −1 energy, +1 XP |
| Steady Guard | 3 | 1.50 (in-game AI) | 15 | −2 energy, XP unchanged |
| Take Stock | 3 | 1.31 (in-game AI) | 13 | −2 energy, −2 XP (free cards stay free) |
| Second Wind | 10, gains 15 | 1.32 | 13 | −2 XP (Shallow Breath 11), energy unchanged |
| Try Again | 2 | 1.25 | 13 | −1 energy, XP unchanged |

Against the Warlord: Nudge 15, Strong Swing 14, Try Again 12, Second Wind 10 XP.

**How a tree is moved:** `go run ./cmd/card-tree-shift -tree <tree> -base-xp <XP> -energy <shift> [-set-xp "Card=XP"] -loadout-root <copy of the saves> [-dry-run]`.
- **What it does:** publishes the way the card editor does. It checks the tree, checks the saves still load, and bumps the revision. Free cards stay free, and shared cards keep the same XP in every tree.
- **Before running it:** copy the saves (`dice-and-destiny-client/.godot/runtime/workspace/client/user/character_loadouts` of the checkout you play in) to a scratch folder and point `-loadout-root` there.

**Open:**
- **Upgrades never measured:** only the bases were measured; the upgrades moved by the same steps. Measuring them (test 1 or 2 on decks using them) would confirm the hand-set steps.
- **Second Wind's upper cards:** in battle only the net gain counted. Pay 0 gain 5 and pay 5 gain 10 played like Second Wind, and net +10 was only about 13 XP. Their prices keep the hand-set steps, which also count map value; that map value isn't measured.
- **Free draw limit:** the free Take Stock cards (Glance, Quick Study, Close Study) lack the "once per round" limit the pricing rules describe.
- **The unit formula:** in card-value-rules.md (5 energy = 1 draw = half a health) it no longer matches the measured base cards. Decide whether measurement replaces it.
- **Mirror or minions:** prices follow the Starter mirror, and minion fights value effects lower. Decide which the prices should follow.
- **Other characters:** Curse, Venom, Blade Warden and the Adventurer have not been measured card by card.

**Results:** balance-lab.md, sections "Cards on top of the board", "Cards against blank cards", "Energy costs of the Starter's base cards", "Re-costed base cards" and "Spread costs".

## Tools

| Tool | Does |
|---|---|
| `cmd/balance-lab` | Plays battles. `-players` (`name@flag=value`), `-block-cards`, `-hero-deck` and `-foe-deck`, `-authored-root`, `-seats both`, `-out`. |
| `ml/scripts/deck_sweep.py` | Plays a plan of deck variants; resumes after a stop. |
| `ml/scripts/card_exchange_report.py` | Blanks per copy and XP (test 1); cost variants (`--costs`, `--now`); price checks (`--rates`); health left (`--metric=health`). |
| `ml/scripts/card_cost_variants.py` | Scratch published cards with cost variants under new IDs. |
| `ml/scripts/card_value_report.py` | Deck on top of the board: damage before and after defense, prevention, paired differences. |
| `cmd/card-tree-shift` | Moves a whole card tree with its base (energy and XP), publishing and validating like the card editor. |
| `docs/game-design/dice-throne-reference/analysis/board_balance.py` | A board's exact average per turn and ability use. |
