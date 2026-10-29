# Pricing board damage and defenses

Research, 2026-10-24, branch `power-pricing-research`. The brief is in [power-pricing-handoff.md](power-pricing-handoff.md); the groundwork is in [power-exchange-rates.md](power-exchange-rates.md). Nothing in the game has been changed.

**The question:**
- **Cards:** blank cards stay at 10 XP each.
- **What to find:** what should one point of average damage per round on a board cost, and what should a defense cost, priced from the average damage it prevents per round?
- **What counts as working:** characters built on equal XP stay close (mild rock-paper-scissors, nothing lopsided) as they add cards, raise damage and upgrade defenses, at every damage level from 6.2 to 20.

## The answer

| What you buy | Price |
|---|---|
| A blank card (1 health) | 10 XP (unchanged) |
| +1 average damage per round on the board | **40 XP** |
| A defense | **40 XP per point of average damage it prevents per round** |

The two rates are the same. A point of damage and a point of prevention are worth the same, at every level.

**A build's total XP** is therefore simply:

> **XP = 10 × cards + 40 × (board average − 6.2) + 40 × defense's average prevention**

**Defenses at that rate:**

| Defense | Average prevention | XP | Upgrade from the one above |
|---|---|---|---|
| No defense | 0 | 0 | |
| Light Guard (1 die: 0/1/2) | 1.0 | 40 | 40 |
| Double Light Guard (hypothetical, 2 dice) | 2.0 | 80 | 40 |
| **Holdfast** | 3.5 | **140** | 60 |
| Heavy Guard (hypothetical: Holdfast with apex preventing 2) | 4.5 | 180 | 40 |
| Bulwark (hypothetical: Heavy Guard with side preventing 3) | 5.5 | 220 | 40 |

Light Guard → Holdfast is 100 XP.

**The damage slider:** each staircase step costs 40 XP × the average damage it adds. A step adding 0.73 costs 29 XP.

**Flat, not a percentage.** I tested percentage prices and they were no better. The flat rate is also what makes the system the same at every level (next section).

**The one number to keep in proportion: a point is worth 4 cards.**
- If the card price ever changes, keep damage and prevention points at 4× the card price.
- The 4 is the fight length: players who spend well end up with decks about 4 of the opponent's hits deep.

## Why it works at every level

**1. Power is health × damage that gets through.**
- Both sides survive their health ÷ the other's damage through their defense, and the longer survivor wins.
- So damage, prevention and cards each multiply a build's strength (power-exchange-rates.md, checked in real battles).

**2. Flat prices keep the game the same shape at every level.**
- Double every build's cards, damage and defense, and every matchup stays the same, while every price doubles too.
- So a field priced fairly at one level is priced fairly at all of them.
- Only a flat price per point has this property. A percentage price makes the game drift as levels rise.
- Whole-number damage and whole cards are the only things that break it, and they matter less as numbers grow.

**3. Prevention and damage trade one for one.** At equal cards, characters are even when their *damage + average prevention* is equal.
- With one rate for both, those builds cost the same XP, whatever the rate.
- Checked from level 9 to 20 with 15–45 cards and all six defenses: every pair lands within 0.40–0.60 at level 9 and 0.44–0.56 from 14 up. The exception is where a defense nearly cancels the damage it faces (see "Where it bends").

**4. Cards and damage trade at the rate that sets the fight length.**
- Builds with the same damage and cards proportional to the damage that gets through are even.
- They cost the same XP exactly when a point costs 10 XP × the hits-to-kill.
- From 9 to 20 damage, every pair lands within 0.44–0.56.

**5. Percentages fail for these reasons:**
- **A percentage of your deck, charged when you buy:** cheap while the deck is small, so buy damage and defense first, then cards. Damage then grows exponentially per XP while cards grow only linearly, and damage-heavy builds run away.
- **A percentage of your level (a point costs 4 × level in XP):** tested. It plays no better than flat 40 (0.46–0.55), and it pushes decks far larger at high levels.
- **Order:** the flat price depends only on the finished build, never on the order things were bought.

## The proof: characters at their best split

**The test (`chars.py`):**
- Each defense is a character, with the Standard board on the standard die.
- At a total XP budget above the base (15 blank cards, 6.2 damage, no defense), each character picks how to split its XP between damage and cards.
- It picks the split that does best against the other characters (an equilibrium found by the race model).
- **Prices are fair** when every character, playing its best split, scores about 0.5.
- **Nothing is lopsided** when every matchup between characters stays between 0.40 and 0.60.

**Four characters at 40 XP a point** (Heavy Guard costs 180, so it joins at +180):

| XP above base | No defense | Light Guard | Holdfast | Heavy Guard | Matchups |
|---|---|---|---|---|---|
| 100 | 0.508 · 6.9 dmg, 22 cards | 0.492 · 6.2, 21 | | | 0.49 |
| 150 | 0.545 · 7.6, 24 | 0.462 · 7.8, 20 | 0.494 · 6.2, 16 | | 0.41–0.50 |
| 200 | 0.548 · 9.7, 21 | 0.515 · 8.7, 21 | 0.453 · 7.3, 17 | 0.484 · 6.2, 17 | 0.43–0.51 |
| 250 | 0.555 · 9.8, 26 | 0.517 · 9.7, 22 | 0.445 · 7.9, 19 | 0.484 · 6.9, 19 | 0.38–0.50 |
| 300 | 0.529 · 10.7, 27 | 0.508 · 9.7, 27 | 0.458 · 8.1, 23 | 0.505 · 7.4, 22 | 0.42–0.54 |
| 400 | 0.533 · 11.2, 35 | 0.487 · 11.3, 30 | 0.482 · 9.7, 27 | 0.498 · 8.7, 27 | 0.43–0.50 |
| 500 | 0.497 · 12.7, 39 | 0.492 · 11.7, 39 | 0.503 · 10.7, 33 | 0.508 · 9.8, 32 | 0.49–0.51 |
| 600 | 0.489 · 13.9, 44 | 0.495 · 12.9, 44 | 0.501 · 11.7, 39 | 0.515 · 11.1, 37 | 0.51–0.52 |
| 750 | 0.500 · 16.4, 49 | 0.491 · 14.9, 51 | 0.498 · 13.9, 45 | 0.510 · 12.7, 46 | 0.48–0.53 |
| 900 | 0.505 · 18.9, 54 | 0.510 · 16.8, 58 | 0.489 · 16.1, 51 | 0.496 · 14.9, 52 | 0.46–0.52 |
| 1100 | 0.493 · 18.4, 76 | 0.498 · 19.7, 67 | 0.504 · 18.4, 62 | 0.505 · 17.0, 64 | 0.49–0.52 |

Each cell is the character's score at its best split, then its average damage and cards. A character may mix a few nearby builds; the numbers are their average.

**What the table shows:**
- **Every damage level is covered.** Damage climbs from 6.2 to about 20 as XP grows, and every character stays within 0.44–0.56.
- **From +500 XP up**, every character is within 0.49–0.52 and every matchup within 0.46–0.53.

**Six characters**, adding Double Light Guard (2.0) and Bulwark (5.5):
- characters 0.45–0.55 and matchups 0.38–0.65 up to +500 XP;
- from +600 XP, 0.48–0.53 and matchups 0.44–0.57;
- at +1100 XP, every character within 0.495–0.506.

**Other boards** (every character on the same board):

| Board | Characters, early | Characters, +600 and up |
|---|---|---|
| Spellbook · Loom (chunky) | 0.46–0.56 | 0.48–0.52 |
| Rainbow · Grove (smooth) | 0.46–0.55 | 0.48–0.51 |
| Combo · standard die | 0.47–0.55 | 0.48–0.52 |

**Board and defense mixed:** 4 boards × 4 defenses, 16 characters.
- **Early:** characters 0.45–0.57.
- **At +900 XP:** characters 0.48–0.51 and no lopsided matchup (0.42–0.60).

**The best rates, scanned** (`chars_scan.py`, `kfine.py`):
- **Damage rates:** 30, 40, 50 and 60 XP a point.
- **Defense:** 0.7–1.25 × the damage rate.
- **The defense rate:** **exactly the damage rate is best at every damage rate.**
  - Cheaper defense: big defenses win (at 0.9×, Bulwark averages 0.60).
  - Dearer defense: no defense wins (at 1.1×, 0.59).
- **The damage rate: 40 is the most even.**
  - 50 and 60 are lopsided early (to 0.40–0.61 at +250 XP).
  - 30 works until characters reach the slider's cap of 20.

**Real battles** (`board-lab -exchange-check`, greedy players, 2,000 battles each, ±0.022):
- Each character's most-played build at +150, +400, +600 and +900 XP, every pair: 19 matchups, from 6.2 to 19 damage and 16 to 60 cards.
- The new defenses were written as abilities and played.
- **Mean gap from the model 0.007, largest 0.018.**

## Where it bends

**1. The early game: the first ~400 XP above a 15-card base.**
- **What happens:** no-defense characters run about 0.03–0.05 high and Holdfast about 0.05 low.
- **Causes:**
  - The base deck has far fewer cards than the 4-hits-deep ratio (25 cards at 6.2 against no defense).
  - Holdfast is a 140-XP lump out of a small budget, so the Holdfast character is stuck at 15–19 cards.
- **Things tried:**

  | Fix | Effect |
  |---|---|
  | A price that rises with level (25 XP a point at the start, 40 from level 9.2) | Slightly better (worst character 0.043 from even, against 0.056) |
  | Variations of that ramp | No better |
  | **A 25-card base deck** | **Every character within 0.47–0.53 at every budget.** Best results by far |
  | Finer damage steps | Same result, so it isn't the 0.25 grid |

- **What it means:** none of this is lopsided at the character level. If the early game matters most, start characters nearer 4 hits deep rather than changing the rates.

**2. Single builds are sharper than characters, early.**
- **Early:** among each character's five best builds, 11–22% of pairings at +150–400 XP are lopsided (as low as 0.26). That falls to 7% at +600 and 0% at +900. The other prices tried do about the same: 11–18% early, 0–3% from +600.
- **Cause:** with blank decks, fights are 3–4 hits long and the Standard board's hits are tightly bunched (mostly 6–7 at 6.2). One card decides whether a deck dies in 3 or 4 hits. For example, Light Guard with 21 cards dies to three 7-damage hits while 22 cards needs four, and that swings a matchup by about 0.1 (0.61 in real battles).
- **Real decks are softer:** cards add variance, and in the Starter mirror 1 blank is worth only 0.065–0.09.

**3. Defenses close to the damage they face.**
- **Example:** Bulwark (5.5) against 6.6 damage lets about 1 through, and matchups go to 0.24–0.35.
- **Why:** little gets through, so prevention is wasted unevenly.
- **Not a price problem:** keep a defense's average under about half the smallest hits it will face, or give big defenses only to characters past the early levels.

**4. The slider cap at 20.**
- Past about +1,100 XP characters can't buy more damage, only cards and defense, so defenses gain (four characters at 50 XP a point, +1,800: Heavy Guard 0.63, no defense 0.39).
- If characters will earn more than that, the staircase needs to go higher.

**5. Spread hardly matters.**
- Three 3.5 defenses (steady 3-or-4, Holdfast 0–6, swingy 0-or-7) and two 2.0 ones all play within 0.48–0.52 from +400 XP.
- The swingy one runs up to 0.06 low at +150 XP (0.02 by +250), because a 7-roll wastes prevention on 6-damage hits: it actually stops 3.07 at 6.2, the full 3.5 from 9 damage up.
- **The rule for a new defense:** price its average. Only if its big rolls exceed the hits it will face, price the average it actually stops, E[min(hit, roll)].

**6. Decks have to grow with damage.** At 40 XP a point, well-spent characters carry:

| Damage | Cards |
|---|---|
| 8–10 | about 20–25 |
| 11–12 | about 30–35 |
| 14 | about 45 |
| 17–19 | about 55 |
| 20 | 60–75 |

That is what keeping fights about 4 hits long means.
- **Smaller decks at high damage** would mean shorter fights (a lower rate at high levels). That pushes towards 2-hit knockouts, where nothing balances (power-exchange-rates.md, "Battle length").
- **Enemies scale too:** enemy health should grow with player damage in the same proportion.

## The 2 × 2 test: fixed builds at one XP pool

**The builds:**
- Damage 7 or 12 (staircase averages 6.91 and 12.00).
- No defense or Holdfast.
- Every leftover XP in blank cards, priced at the flat rates.
- Cards follow from the other two choices, so each XP pool has 4 builds.

The scripts are `eight.py`, `eight_sweep.py`, `best4.py` and `pct2x2.py`. The real battles are `eight-checks-plan.json`, 2,000 battles a pairing (±0.022), and they agree with the model within 0.008.

**+400 XP** (the smallest pool where 12 damage with Holdfast still has cards). Builds:

| Build | Damage | Cards |
|---|---|---|
| Low, no defense | 6.9 | 52 |
| High, no defense | 12.0 | 31 |
| Low, Holdfast | 6.9 | 38 |
| High, Holdfast | 12.0 | 17 |

**Real battles, row against column.** Each cell is win / draw / loss % and the score:

| | Low, no defense | High, no defense | Low, Holdfast | High, Holdfast | Average |
|---|---|---|---|---|---|
| Low, no defense | — | 7/80/13 (0.47) | 0/2/98 (0.01) | 15/39/46 (0.35) | 0.28 |
| High, no defense | 13/80/7 (0.53) | — | 19/69/12 (0.53) | 59/40/1 (0.79) | 0.62 |
| Low, Holdfast | 98/2/0 (0.99) | 12/69/19 (0.47) | — | 18/36/45 (0.36) | 0.61 |
| High, Holdfast | 46/39/15 (0.65) | 1/40/59 (0.21) | 45/36/18 (0.64) | — | 0.50 |

**Larger pools get worse.** At +600 and +900 the low-damage, no-defense build loses almost everything, and the high-damage, Holdfast build wins almost everything.

**So fixed builds at one pool are not even.** There are two separate reasons.

**1. Some of these builds are badly built for their pool.**
- The flat rates are calibrated so that each single trade breaks even when the decks are about 4 hits deep against what they face.
- At +400, the 52-card, 7-damage deck is 15 hits deep against Holdfast. Its next damage point would be worth far more than 4 cards.
- **Each single swap is even at one pool, always at about 4 hits deep:**

  | Swap | Even at | Cards | Real battles |
  |---|---|---|---|
  | 7 ↔ 12 damage, no defense | +355 | 47 vs 27 | 22/64/14 (0.54) |
  | 7 ↔ 12 damage, Holdfast | +365 | 34 vs 14 | 28/39/33 (0.47) |
  | No defense ↔ Holdfast at 7 damage | +160 | 28 vs 14 | 27/41/32 (0.47) |
  | No defense ↔ Holdfast at 12 damage | +545 | 46 vs 32 | 16/63/21 (0.48) |

  The rates are consistent from 7 to 12 damage. They are fair for builds whose deck suits their damage, and they punish builds that pile up cards on low damage, or damage on few cards.

**2. A flat defense against both 7 and 12 damage can't be fair to both.**
- Holdfast takes 3.5 off every hit: half of a 7-damage hit, 29% of a 12-damage hit.
- **No card counts at all make the four builds even** (searched exhaustively, ignoring prices). The best possible worst matchup is 0.40–0.60 with about 20-card decks, 0.27–0.73 at about 30 cards, and 0.21–0.79 at about 60. Longer fights magnify it.

**What would make every 2 × 2 combination even:**
- **The design:**
  - **Defense:** prevents a *percentage* of each hit (tested: a Ward preventing 37%, which is Holdfast's 3.5 against a 9.5 hit).
  - **Prices:** damage and defense cost a *percentage of the deck*. A build's "power XP" is 10 × cards × (damage ÷ 6.2) × (1 ÷ 0.63 with the Ward).
- **The result:** all four builds land within 0.42–0.58 at every deck size from 20 to 70 cards (model; the engine has no percentage prevention yet).
- **The catch:** cards are then no longer a flat 10 XP. For a 12-damage character a card effectively costs 19 XP, or 31 with the Ward. Equivalently, the board's and defense's percentage keeps applying as cards are added.
- **Half measures don't work:**
  - A percentage defense with flat prices is still lopsided (0.07–0.93 at its best pool).
  - Percentage prices with Holdfast stay intransitive.

**Draws are common.** When two decks run out in the same round the battle is a draw.
- Builds close in strength often finish in the same round: 80% draws for 7 damage/52 cards against 12 damage/31 cards in real battles.
- Real cards add variance and will lower this.
- If draws should be rare, the game needs a tie-break (for example, the larger remaining deck wins).

## Solving the rounds equation (follow-up, same day)

The user's fairness rule: the rounds it takes each side to defeat the other should be about equal.

> rounds for B to defeat A = A's cards ÷ (B's damage − A's prevention), and the same the other way

Together with equal total XP (within one blank card), the question is what price form makes that hold for every combination of damage, defense and cards, at every XP total. The scripts are `math_fit.py`, `pct_solution.py`, `pct_variants.py` and `closed_form.py`.

**1. The best possible fit is multiplicative.** Take the four builds (7 or 12 damage, no defense or Holdfast) and choose card counts freely to fit the rule as closely as possible, by least squares or by minimising the worst pair.
- **The best card counts:**
  - Holdfast multiplies cards by 0.595 at either damage.
  - 7 → 12 damage multiplies cards by 0.489 with or without a defense.
- **Flat prices can't produce that.** A flat Holdfast price would have to take 40.5% of a 33-card deck and 40.5% of a 16-card deck, which are different numbers of XP.
- **Percentages of total XP can,** with offense as a share of total XP and defense as a share of what's left: cards = XP ÷ 10 × f(damage) × g(defense).

**2. A flat-prevention defense leaves a gap no price can close.** Even at the best fit, some pairs are about 19% apart in rounds to defeat with Holdfast between 7 and 12 damage.
- **The gap is ½ × |ln[(1 − D/O_low) ÷ (1 − D/O_high)]|,** where D is the defense's average prevention and O_low, O_high are the lowest and highest damage it faces.
- **Why:** Holdfast takes half of a 7-damage hit but under a third of a 12-damage hit.
- **What 19% does:** blank-deck fights are close to deterministic, so it becomes lopsided. With percentage pricing at its best, scores run 0.16–0.84 at 400–600 XP and 0.02–0.98 at 1,500.
- **Shrinking D or narrowing the damage range shrinks it** (scores at 400 / 1,500 XP):

  | Field | Gap | Scores |
  |---|---|---|
  | 7–12 damage, Holdfast | 19% | 0.19–0.81 / 0.02–0.98 |
  | 7–12, Double Light Guard (2) | 8% | 0.34–0.66 / 0.15–0.85 |
  | 7–12, Light Guard (1) | 3.4% | 0.42–0.58 / 0.30–0.70 |
  | 9–12, Holdfast | 7.7% | 0.35–0.65 / 0.22–0.78 |
  | 12–18, Holdfast | 6.6% | 0.46–0.54 / 0.32–0.68 |

**3. With percentage prevention the system has an exact solution.**
- **The defense:** prevents a share p of every hit, rounded at random so the share is exact.
- **The formula:**

  > **cards = total XP ÷ 10 × (6.2 ÷ damage) × (1 − p)**

  - **The board** costs (1 − 6.2 ÷ damage) of your total XP: 7 damage 10%, 9 damage 31%, 12 damage 48%, 18 damage 66%.
  - **The defense** costs p of what's left: a 37% Ward costs 37%.
  - **Every remaining 10 XP is a blank card.**
- **Why it's exact:** everyone at the same XP then has the same cards × damage ÷ (1 − p), so every pair takes exactly the same rounds to defeat each other. That holds for every combination at every XP total, and the 6.2 base still counts as the starting multiplier.
- **Race model:** all four builds score 0.39–0.61 at 400–1,500 XP, with 7 against 12 damage and 7 against 18. What's left is whole cards and dice.
- **The engine has no percentage prevention yet**, so these are model results; the model matched real battles at every one of 55 checked points so far.

**Worked example, Ward 37%:**

| Build | 400 XP | 600 XP |
|---|---|---|
| 1. 7 damage, no defense | board 41 + 35 cards | board 61 + 53 cards |
| 2. 7 damage, Ward | board 41 + Ward 133 + 22 cards | board 61 + Ward 199 + 33 cards |
| 3. 12 damage, no defense | board 193 + 20 cards | board 290 + 30 cards |
| 4. 12 damage, Ward | board 193 + Ward 76 + 13 cards | board 290 + Ward 115 + 19 cards |

Every pair takes the same rounds to defeat each other: 5.1 against 5.1, 2.9 against 3.0, 4.4 against 4.4. The pairings score 0.46–0.54 at 400 XP and 0.43–0.57 at 600.

**What this changes in practice:**
- **Shares keep up with XP.** The board's and defense's shares grow as total XP grows. Equivalently, a character's card allowance is XP ÷ 10 × (6.2 ÷ damage) × (1 − p), and each card is still 10 XP out of that allowance.
- **Draws dominate when balance is this exact.** Equal rounds mean both decks often empty in the same round: 48–94% draws in the model. Real cards add variance, but the game may want a tie-break.

## The balancing point (follow-up, same day)

The four builds (low damage 7 or high, no defense or a defense, the rest in blank cards) had to land inside 40–60 against each other at every XP level, with the high damage and the defense free to choose. The scripts are `balance_search.py`, `balance_pct.py`, `robust.py`, `robust_scan.py`, `page_data2.py` and `make_page2.py`. The interactive page is `power-pricing-balanced.html`.

**What blocks it.**
- **Blank-deck fights are very predictable.** At 900 XP, a 4% gap in rounds to defeat is already 38/62.
- **The tolerance:** every pair must be within about 2–3% in rounds.
- **Flat-prevention defenses can't manage that across levels:**
  - Their interaction with low against high damage grows as fights lengthen.
  - The best meaningful flat defense (1.0) holds inside 40–60 at only 21–52% of XP levels.
  - Holdfast-sized flat defenses manage about 24–76 at best.

**The balancing point:**
- **The defense:** prevents a *share* of each hit.
- **The prices:**

  > **cards = XP ÷ 10 × (6.2 ÷ damage) × (1 − defense share)**, rounded to the nearest card

- **What a build pays:**
  - **The board:** (1 − 6.2 ÷ damage) of total XP. 7 damage costs 10%, 9 damage 31%, 12 damage 48%, 14 damage 55%.
  - **The defense:** its share of what's left.
  - **Each card:** 10 XP of the remainder.

**Results:**

| Configuration | Inside 40–60 (every 10 XP, 400–1,500) | Worst | At 400 / 650 / 900 |
|---|---|---|---|
| **High 9, 15% defense (recommended)** | **100%** | 40–60 | 43–57 |
| High 14, 20% defense | 99% | 38–62 | 0 lopsided of 18 |
| High 9, 37% defense (Holdfast strength) | 96% | 36–64 | 2 of 18 |
| Today's engine: high 8, flat defense 0.5 (costs 7%) | 95% | 37–63 | 0 of 18; real battles 41–57 |

- **The full search** (high 9–14 against shares of 10–37%) lands inside at 87–100% of levels. A bigger gap or a bigger defense costs a little tightness.
- **What's left** is whole-card breakpoints in small decks.
- **Real battles:** 18 matchups for the engine-playable configuration, average gap 0.5 points, largest 1.3.
- **Engine work:** percentage prevention needs a new effect.

## Rolled defenses, priced by XP (follow-up, same day)

The defense stays a roll with flat amounts on its faces, like Holdfast, and only its XP price may be flat or a percentage. The scripts are `rolled.py`, `rolled_small.py`, `page_data3.py` and `make_page3.py`. The interactive page is `power-pricing-rolled-defense.html`.

**What works: block defenses.** Some faces prevent the whole attack (amount 99), the rest nothing.
- **Why it works:** a full block takes the same share of a 7-damage hit as of a 12-damage one, so it can't favour one damage level.
- **Recommended:** roll 2 dice; any 6 prevents all damage from the attack (blocks 31% of attacks), with high damage about 9.
  - Inside 40–60 at 99–100% of XP levels from 400 to 1,500.
  - Real battles: all 18 matchups at 400 / 650 / 900 between 41 and 59.
- **Other block defenses:** 1 die blocking on 5–6, and 3 dice blocking on any 6 (Holdfast strength), are inside at 95%.

**What can't be balanced: small amounts on every face, as Holdfast does.** The whole search, every high damage and every price:

| Defense | Inside 40–60 |
|---|---|
| Holdfast | 0% of levels (worst 18–82). Real battles: 9 of 18 matchups lopsided, down to 24–76 |
| Light Guard (1.0) | 5–54% |
| A 0.5 defense with high 8 | 95% |

**The prices have to be percentages of total XP:**
- **The board:** costs (1 − 6.2 ÷ damage) of total XP.
- **The defense:** a share of what's left. For the block defenses that share is about their block rate: 30% for the 2-dice block, 32% for the 5–6 block, 42% for the 3-dice block.
- **The cards:** the rest at 10 XP each.
- **Flat prices failed:** the best pair (60 XP per damage point, 170 XP for the 2-dice block) was inside at only 4% of levels.

## Holdfast's price curve (follow-up, same day)

Holdfast stays exactly as it is, and characters choose their own damage. The scripts are `holdfast_curve.py`, `holdfast_matrix.py`, the equilibria in `holdfast_eq.json` and the checks in `holdfast-checks-plan.json`. The page is `power-pricing-holdfast-curve.html`.

**The setup.** At each total XP (300–1,500, no starting deck):
- **Two characters**, no defense and Holdfast, each pick their damage (board 40 XP per point above 6.2) and fill the rest with 10-XP cards.
- **Holdfast's fair price** is the XP that makes it win half the time.

**The results:**
- **The fair price is 130–145 XP at every level from 300 to 1,300 XP.** That's 3.5 average prevention × 40 XP a point = 140, about 14 cards.
- **The cap:** above 1,300 the slider's cap of 20 distorts it.
- **As a share of total XP** it falls from 43% to 11%, so it reads as a discount, but the XP price is constant.

**Why it's flat.** Holdfast's worth depends on two things:
- **The opponent's damage:** it doubles a deck's staying power against 7-damage hits, adds 41% against 12 and 24% against 18. Your own damage doesn't matter.
- **Your deck size,** because that share is a share of your cards.

When characters choose their damage, the Holdfast player plays about 3 points less damage with the same cards. As XP grows, opponents hit harder but decks grow, and the two cancel.

**At these prices** (Holdfast 140, Light Guard 40, board 40 a point):
- **Character scores:** no defense, Light Guard and Holdfast score 0.47–0.54 against the field at 400 / 650 / 900.
- **Real battles:** Holdfast against no defense, most-played builds, 48–52% at all three levels; 8 of 9 pairings inside 40–60. Light Guard against Holdfast at 650 was 39%.

**Why the earlier 2 × 2 failed.** It locked the builds at 7 and 12 damage. A 7-damage deck of 50–80 cards against Holdfast is a mismatched build at any price: Holdfast halves its hits. Holdfast is fairly priced for characters who pick a damage level that suits their XP.

## Block defenses in the big table (follow-up, same day)

The scripts are `block_curve.py` and `full_grid2.py`; the page is `power-pricing-full-grid-blocks.html`.

**Fair price.** Found the same way as Holdfast's: characters pick their damage, board 40 XP a point. Each block defense keeps a steady share of the XP left after the board, at every level:

| Block defense | Blocks | Fair price (share of the XP left after the board) | In XP at 400 → 1,300 total |
|---|---|---|---|
| 1 die, a 6 prevents all damage | 17% | 15–16% | 53 → 122 XP |
| 2 dice, any 6 | 31% | 28% | 103 → 257 XP |
| 1 die, 5–6 | 33% | 30% | 113 → 282 XP |

The rule of thumb is a share of about 0.9 × the block chance. A block stops a share of every hit, so its worth in XP grows as opponents hit harder. That's why it has to be priced as a percentage and not flat; Holdfast stays flat at about 140.

**In the full field** (damage 6.2–18 × no defense / Holdfast / 1-die block / 2-dice block, at these prices):
- **The best no-defense build** averages 67–79%.
- **Holdfast's best** averages 71–89%: it leads from 650 XP up, because the field includes many low-damage builds.
- **The blocks' best** averages 63–77%, about 2–8 points under no defense.
- **Possible adjustment:** against a field like this the blocks could be a little cheaper, about 13% and 25% of what's left. Against well-built opponents their fair prices are as above.

## Block price from total XP (follow-up, same day)

The "share of the XP left after the board" rule depends on the board the character picked, so the game can't read it off directly.
The same fair prices fit a formula that uses only two numbers the game knows, the block chance and the character's total XP:

**Block price = block chance ÷ 2 × (total XP + 200)**

This is the same as a flat part plus a percentage of total XP: 1 die, a 6 → 17 XP + 8.3% of total XP.

| Defense | Block chance | Formula | 400 XP | 650 | 900 | 1,300 |
|---|---|---|---|---|---|---|
| 1d10, a 10 | 10% | 10 + 5% | 30 (model 31) | 42 (41) | 55 (57) | 75 (71) |
| 1 die, a 6 | 16.7% | 17 + 8.3% | 50 (53) | 71 (71) | 92 (95) | 125 (122) |
| 2 dice, any 6 | 30.6% | 31 + 15.3% | 92 (103) | 130 (143) | 168 (190) | 229 (257) |
| 1 die, 5–6 | 33.3% | 33 + 16.7% | 100 (113) | 142 (163) | 183 (209) | 250 (282) |

- Up to about a 1-in-6 block, the formula is within 3 XP of the model.
- For blocks around 30%, it comes out about 11% under the strict fair price. That matches the mixed-field result above, where those blocks were slightly too expensive.
- For an exact fit, use a rate of 0.56 × block chance instead of 0.5 for the bigger blocks.
- The price rises with level.
- Holdfast-style flat prevention stays flat at 140.

## Limits

- **Blank decks, so no cards' effects.** Whole decks can be counted in blanks (balancing-playbook.md, test 5) and added to the card count.
- **Mostly one board.** The Standard board, with spot checks on three others. Defenses are rolled on the standard die (on the Grove die Holdfast is swingier).
- **The model.** The race model and greedy players, checked against real battles here (19 points) and in power-exchange-rates.md (17 points).
- **The base.** The base is a 15-card, 6.2, no-defense character. A different base shifts only the early game.

## Reproduce

From `dice-and-destiny-server/`:

```bash
go run ./cmd/board-lab -exchange-dump ml/runs/exchange/odds.json       # damage odds of every default board (ignored by git)
cd ml/scripts/power_exchange
python3 families.py        # the two even families from 6.2 to 20
python3 drift.py           # how far one character can move damage into cards before it turns lopsided
python3 chars.py 40 1.0    # characters at their best split (flat 40)
python3 chars_scan.py      # damage rate x defense multiplier
python3 kfine.py           # defense multiplier 0.9–1.1, six characters
python3 level_scan.py      # rising (level) prices; scan3.py: percentage of level, flat 30/50, other boards
python3 ramp_scan.py       # early-game ramps
python3 spread.py          # defenses with the same average and different spread
python3 mixed.py           # 4 boards x 4 defenses
python3 purebuilds2.py     # single builds, exact health
python3 final_tables.py    # the tables above (step 0.1)
python3 eight.py 400 600 900   # the 2 x 2 fixed builds; eight_sweep.py: every pool and the single swaps;
                           # best4.py: best possible card counts; pct2x2.py: percentage defense and prices
python3 mk_checks.py       # writes the real-battle plan (copy kept here as price-checks-plan.json)
cd ../../.. && go run ./cmd/board-lab -exchange-check ml/scripts/power_exchange/price-checks-plan.json -battles 2000 -out ml/runs/exchange/price-checks.json
```

`field.py` holds the engine:
- the race model vectorised over health;
- the defenses, including the hypothetical ones;
- `use_board` to switch boards.

`cmd/board-lab` now also plays `light2`, `heavy` and `bulwark` as `-exchange-check` defenses.
