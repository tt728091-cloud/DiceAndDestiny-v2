# Handoff: XP prices for board damage and defenses

Research task, written 2026-10-24. **Done the same day; results in [power-pricing-research.md](power-pricing-research.md).** It follows [power-exchange-rates.md](power-exchange-rates.md); read that first, since it holds everything found so far.

## The goal

Find XP prices for the two things a player buys besides cards:
- **the average damage per round on their board** (the Board Builder's damage slider, staircase 5–20);
- **the defense they pick**, priced from the average damage it prevents per round.

Each price may be a flat rate or, more likely, a percentage. Blank cards are the fixed yardstick: **1 blank card = 1 health = 10 XP**, and that rate does not change. (If the flat card rate truly has to move, say so and why, but the user wants to keep it.)

**Prove the prices keep builds close at every level.** That means at 6.2 (today's Starter level) and then as the slider climbs towards 20, as characters add cards, raise damage and upgrade defenses. Builds of equal XP should form a mild rock-paper-scissors: no build so strong that everyone wants it, and none so weak that nobody does. A matchup is **lopsided** when one side wins 60% or more (see board-balance-philosophy in the user's notes and board-round-robin.md). Perfect balance is not the goal; staying out of the lopsided zone is.

**The three variables:**
1. **Average damage per round:** the board's target on its staircase.
2. **Defense:** none (prevents 0), Light Guard (1 on average), Holdfast (3.5), and at least one new, stronger hypothetical defense (for example 4.5 on average) to test that the math extends. Others (2, 5.5) are welcome.
3. **Deck size:** blank cards only, to keep variance down.

**Time:** about 4 hours. End early if it's settled, and say so. If 4 hours isn't enough, still report what was found and say what more is needed.

## What is already known

From power-exchange-rates.md, using the race model and checked in real battles:
- **The rule:** a matchup is even when health × damage-through-the-defense is equal (H_A·m_A = H_B·m_B).
- **What a point is worth:** +1 average damage is worth H_opponent ÷ m blanks (the hits needed to empty their deck); +1 prevention is worth H_you ÷ m_opponent.
- **A defense in damage points:** it is worth exactly its average prevention, at every level from 6.2 to 11. Light Guard = 1 point, Holdfast = 3.5, Light Guard → Holdfast = 2.5. Prevention is barely wasted against the Standard board (Holdfast prevents 3.46 of 3.5).
- **A defense as a share of your health:** D̄ ÷ (T_opponent − D̄). Holdfast is +127% against 6.2 damage and +53% against 10.
- **The catch, intransitivity:** builds even against one opponent can be lopsided against another, because health-heavy, low-damage builds lose to strong defenses. In blank mirrors this ranged from 0.09 to 0.76. Any pricing scheme has to be judged on a whole field of builds, not on pairs.
- **Diminishing returns:** at a fixed price, all-damage against all-blanks stays even, but balanced builds beat both extremes once budgets are large next to the base deck.
- **Battle length:** below about 3 hits-to-kill, fights become 1–2 round knockouts and every rate snaps to whole kills. Health has to scale with damage. Report where a scheme breaks for this reason, separately from where it is simply mispriced.
- **The 60/40 line** in blank mirrors is only about 1–2 blanks (6–12% of power), so prices have to be fairly precise.

## Tools (in this checkout once the research branch is in)

From `dice-and-destiny-server/`:
- **`go run ./cmd/board-lab -exchange-dump ml/runs/exchange/odds.json`:** run this first. It writes every default board's damage odds across the staircase (step 0.1) and each defense's prevention odds per die. `ml/runs/` is ignored by git, so a fresh checkout must regenerate it.
- **`ml/scripts/power_exchange/race.py`:** the race model in Python (port of `cmd/board-lab/duel.go`): `defend`, `kill_rounds`, `race_rounds`, `score`, window-averaged scores and solvers. It reads `$EXCHANGE_DIR/odds.json` (default `ml/runs/exchange/`).
- **The other scripts there:**
  - `three.py`: builds as (target, defense, health), fractional health.
  - `three_way.py`, `defense_price.py`, `offense.py`, `defense.py`, `builds.py`, `lopsided.py`, `sensitivity.py`, `fieldscale.py`.
- **`go run ./cmd/board-lab -exchange-check <plan.json> -battles 2000 -out <out.json>`:** plays listed points as real blank-deck battles with greedy players. Each side has its own target, health and defense. See `ml/scripts/power_exchange/checks-plan.json` for the format. About 15–20 s per 2,000-battle point.
- **The model's accuracy:** the race model matched real battles within their noise at every checked point (largest gap 0.017). Use it for the search, and confirm the final prices with real battles.

**Adding a new defense:**
- **In the race model** a defense is only a prevention distribution (`prevent[k]` = chance of preventing k). Add it in Python.
- **For real battles** it needs an ability:
  - add it like `lightGuardAbility()` in `cmd/board-lab/duel.go`;
  - give it a name in `defenseAbility` and its odds in `defenseOdds`;
  - write it in `writeScratch`.
- Holdfast's odds depend on the die (3 dice by role), so keep to the standard die unless studying dice.

**What "the Starter" means here:** the Standard template on the standard die (label `Standard · Standard D6`), the generated board priced to the Starter's 6.2. Other boards follow the same rule through their own damage-through. Check at least one chunky board (Spellbook · Loom) and one smooth one (Rainbow · Grove) at the end.

## Suggested plan

1. **Write down the candidate schemes**, each with its parameters. Blank = 10 XP in all of them.
   - **Flat:** XP per average-damage point and XP per prevention point.
   - **Percentage of deck:** damage and defense each cost a percentage of the deck's health XP.
   - **Percentage of total XP.**
   - **Multiplicative:** price each upgrade by how much it multiplies "effective cards" against a reference opponent (power-exchange-rates.md, "The variable").
   - **Anything better you find.**
2. **Build the field.**
   - Start from a base character, for example 15 blanks, 6.2 and no defense, or what the user's character sheet starts with; ask if unclear.
   - For several total XP budgets (for example +0, +100, +250, +500, +1000 XP above the base), enumerate every affordable build under each scheme: cards, a damage target on the staircase, and a defense.
   - Include the progression paths a real player would take: all damage, all cards, all defense, and mixes.
3. **Score each field** with the race model, every build against every other.
   - Use exact healths, averaged over a small health window as `window_score` does.
   - **Measures:**
     - each build's mean score against the field;
     - its worst matchup;
     - the share of lopsided pairings;
     - the spread between the best and worst build;
     - whether any one build or path dominates.
4. **Tune each scheme's parameters** to minimise lopsidedness and spread at every budget and damage level. Note where whole-damage steps (the staircase) or short fights stop it.
5. **Prove it at all levels.**
   - Show tables from 6.2 up the slider (7.5, 9, 11, 14, 17, 20 or similar), with the 4.5 defense included.
   - Confirm the key points and a sample of the fields with real battles (`-exchange-check`, 2,000 battles a point).
6. **Report.**
   - **The recommended prices:**
     - the cost of a defense from its average prevention per round;
     - the cost of the offensive damage per round;
     - flat or percentage, as formulas plus a worked table in XP.
   - **The proof:**
     - field results at each budget and level;
     - real-battle checks;
     - where it fails and why (knockouts, intransitivity, rounding);
     - what to do about it (for example, health scaling).

## Rules

- **Research only.** Don't change game content, pricing code, `board_templates.yaml` or `content/authored/`.
- **Where things go:** scratch outputs in `ml/runs/` (ignored); new scripts in `ml/scripts/power_exchange/`.
- **Results:** write them in a new doc, `docs/game-design/power-pricing-research.md`, linked from power-exchange-rates.md.
- **Follow `AGENTS.md`:** use `./scripts/godot.sh` for any Godot run (none should be needed), and run Go from `dice-and-destiny-server/`.
- **Commits:** don't commit unless the user asks.
- **Plain words.** Report in plain words with tables; the user reads the summaries, not the code.
