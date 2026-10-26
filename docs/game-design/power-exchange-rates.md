# Power exchange rates: damage, health and defense

Research, 2026-10-24. What is one more point of average damage (the Board Builder's damage slider) worth against more blank cards, and against a stronger defense, at every level from 5 to 20? Nothing here is applied to the game yet. The follow-up pricing study is briefed in [power-pricing-handoff.md](power-pricing-handoff.md), and its results are in [power-pricing-research.md](power-pricing-research.md).

**Method:**
- Blank-deck duels: the Standard template on the standard die, priced on its staircase. Both sides have blank decks (health only) and no riders. Defenses are none, Light Guard (prevents 0/1/2, average 1) and Holdfast (average 3.5).
- **Scoring:** the board-lab race model, the exact chance of emptying the other deck first. Each cell is averaged over 5 neighbouring healths so whole-card steps don't dominate.
- **Checked in real battles:** 17 points, 2,000 greedy battles each. The battles agree with the model within their noise (largest gap 0.017; table at the end).

## The rule: health × damage through the defense

A side survives about **(its health) ÷ (the damage that gets through its defense each round)** rounds. The side that survives longer wins. A matchup is even when

> **H_A × m_A = H_B × m_B**

where H is a side's health and m is its average damage per round **after the other side's defense roll**. The rule ranks the winner correctly in 98% of 1,500 random pairings (different boards, targets, defenses and healths). When battles last 3 or more rounds, it gives the exchange rates below to within about 0.1–0.3 blanks.

Everything else follows from it:

| You gain | Worth, in blank cards | In words |
|---|---|---|
| +1 average damage | **H_opponent ÷ m_you** | as many blanks as the hits you need to empty their deck |
| +1 average prevention | **H_you ÷ m_opponent** | as many blanks as the hits they need to empty yours |
| +1 blank | 1 | |

- **Additive, then multiplicative.** At equal health, damage and defense trade one for one: the side with the larger *damage + average prevention* wins. Health multiplies the result.
- **Percent for percent.** +10% damage through the defense is worth +10% health.
- **XP.** A blank is 10 XP, so the fair XP of +1 average damage is **10 × hits-to-kill**.

## Why low numbers are skewed

+1 damage is a big share of a small hit and a small share of a big one. The value of a point falls as 1/m. A defense makes this stronger, because it shrinks m and so makes every point of damage a larger share of what gets through.

**Blanks per +1 average damage.** Exact model; in brackets, the rule's H/m.

| Target | No defense, H 15 | H 20 | H 30 | Light Guard, H 20 | Holdfast, H 15 | H 20 | H 30 |
|---|---|---|---|---|---|---|---|
| 5 | 3.0 | 4.0 (4.1) | 6.0 | 5.0 | 8.7 | 11.7 (13.0) | 17.7 |
| 6.2 | 2.2 | 3.1 (3.2) | 4.8 | 3.8 | **5.3** | **7.1 (7.4)** | 10.8 |
| 8 | 2.0 | 2.3 (2.5) | 3.6 | 2.9 | 3.3 | 4.5 (4.5) | 6.7 |
| 10 | 1.1* | 2.0 (2.0) | 3.0 | 2.0 | 2.2 | 3.1 (3.1) | 4.6 |
| 12 | 1.2* | 1.3* (1.7) | 2.5 | 1.7 | 1.8 | 2.2 (2.3) | 3.5 |
| 14 | 1.0* | 1.4* (1.4) | 2.0 | 1.9* | 1.3* | 2.0 (1.9) | 3.0 |
| 19 | 1.0* | 1.0* (1.1) | 3.1* | 1.0* | 1.0* | 1.0* (1.3) | 2.0 |

\* Battles of 2 rounds or fewer: the rate snaps to whole kills (see "Battle length" below).

**What the table says:**
- **The opponent's defense matters as much as the level.** At today's 6.2 against Holdfast, with 15 health, +1 damage lifts damage through the defense from 2.72 to 3.43 (+26%). That is worth 5.3 blanks, about **55 XP**. With no defense the same point is +16% and worth 2.2 blanks (24 XP).
- **The value falls as the level rises.** At 15 health against Holdfast, +1 damage is worth 5.3 blanks at 6.2, 3.3 at 8, 2.2 at 10 and 1.3 at 14.
- **More health raises it in step.** The value of a point is linear in the health it has to chew through.

## Defense

At equal health, **a defense is worth exactly its average prevention in damage, at every level**:
- Light Guard ≈ +1.0 damage;
- Light Guard → Holdfast ≈ +2.5;
- none → Holdfast ≈ +3.5 (measured 3.4–3.7 from 5 to 14).

In blanks, its worth is **H × D̄ ÷ (T − D̄)**, where D̄ is its average prevention and T the attacker's average damage. That shrinks fast as the attacker's damage grows.

**Blanks a defense upgrade is worth at 20 health** (model):

| Attacker's target | none → Light | Light → Holdfast | none → Holdfast |
|---|---|---|---|
| 5 | 5.0 | 30.5 | 43.1 |
| 6.2 | 4.0 | **17.8** | **24.9** |
| 8 | 3.2 | 11.0 | 14.9 |
| 10 | 1.8 | 7.6 | 10.6 |
| 12 | 2.2 | 5.0 | 6.8 |

**What this means:**
- **Today, Holdfast more than doubles a deck's effective health.** At 6.2 it is worth about 1.25 × your health in blanks.
- **A flat defense fades as damage climbs.** At 12 it is worth about a third of a deck. If offense rises on the staircase while defenses stay flat, defense stops mattering and the hard-hitting boards' Holdfast advantage disappears with it. That is part of why the field looks even at high targets.
- **To keep a defense's role, it must scale with the target.** Holdfast takes 56% of the 6.2 target. Keeping that share means prevention that grows with the attacker's level, or a defense slider priced like the damage slider.
- **Small hits waste prevention.** The one-for-one trade bends when hits are close to the defense (prevention beyond the hit is wasted). That is the chunky-versus-smooth board effect in board-round-robin.md. Pricing on damage through the defense (m, not T) already captures it.

## Battle length: where balancing works

The rule needs a few rounds. The quantity to watch is **hits-to-kill, N = H ÷ m**:

| Hits to kill | Behaviour |
|---|---|
| 4 or more | Smooth; the rule predicts win rates within about 0.02 |
| 2.5–4 | Fine for exchange rates; win rates within about 0.05 |
| 2 or fewer | Knockout regime. Rates snap to whole kills (exactly 1.0, 2.0 blanks…), break points dominate, and a single point can swing 0.06 or nothing at all |

**At fixed health, the staircase walks straight into the knockout regime.** At 15 health against no defense, a target of 10 already means 2-round battles.
- **Field evenness:** the -sweep's field "evening out" above 12 is mostly this, because short battles are luck. Scaled so N stays the same, the board spread at 10 under Holdfast is as large as at 6.2 (standard deviation 0.063 against 0.039).
- **What to do:** any part of the game that raises damage should raise the health in front of it (enemy decks, the player's deck) to keep N at about 4–6.

**How big an edge is lopsided.** In these pure mirrors, the 60/40 line is only about **1.1–2 blanks** whenever N ≥ 3. That is 6–12% of health, or the same percentage of damage through the defense, whatever the level.
- Pure blank mirrors have little luck, so real decks with cards are softer: in the Starter mirror, 1 blank ≈ 0.065–0.09 score.
- Still, a slider price that is off by more than about 10% of a build's power produces lopsided matchups.

## What this means for pricing the slider

**1. Fair price.** At a reference state (health H, damage through the defense m), a fair price for +1 average damage is **10 × H/m XP**, about **10 × hits-to-kill**. At 6.2:

| Defense in the field | H 15 | H 20 | H 30 |
|---|---|---|---|
| Holdfast | 55 XP | 74 XP | 110 XP |
| Light Guard | 29 XP | 39 XP | 58 XP |
| None | 24 XP | 32 XP | 49 XP |

**2. A fixed price is self-balancing.** With the price fixed from the base state, "all damage" against "all blanks" stays even at every budget tried: Holdfast, 15 health, 55 XP a point, budgets of 30–240 XP, scores 0.46–0.52. Players are never forced into one dimension.

**3. Balanced builds win.** Because power is a product, the half-and-half build beats both extremes:

| Budget (Holdfast, 15 health) | Half and half against all damage | Against all blanks |
|---|---|---|
| 30–60 XP | 0.51–0.53 | 0.52–0.53 |
| 120 XP (8 blanks' worth) | 0.65 | 0.66 |
| 240 XP | 0.80 | 0.84 |

This is ordinary diminishing returns: the more you have of one, the less the next point of it is worth. It rewards spreading XP and punishes dumping everything into one stat once budgets are large next to the base.
- **The optimum:** a min-maxer ends up near **H/m = price ÷ 10**.
- **So the damage price sets battle length.** A 50-XP point steers players towards 5-hit fights, and a 30-XP point towards 3-hit fights. Set the price for the fight length you want.

**4. A fixed XP price can't be fair against every opponent.** +1 damage is worth 2.4× more against Holdfast than against no defense. Price against the defense the player will mostly face (Holdfast today), or price per percent of damage through the defense.

**5. Staircase rounding.** At some targets whole damage leaves a board well off its target: Ladder Pair averages 9.57 at a target of 10, 6% under the reference through Holdfast. In a balanced field that is about 1 blank, close to the 60/40 line. Pricing staircase steps on damage through the defense, as board-round-robin.md proposes for 6.2, would carry over.

**6. Expedition.** With wounds carrying over, damage and defense also cut the damage taken per fight (≈ m_enemy × H_enemy ÷ m_you), and blanks don't. Over a run, though, the number of fights a deck survives is again health × damage through ÷ (the enemy's product), so the same product rule governs runs.

**Decisions this leaves for the user:**
- Which defense and health the slider is priced against.
- Whether defenses get their own slider, or scale with the target.
- What fight length (hits to kill) the economy should steer towards.
- Whether enemy health scales with the player's damage.

## Three Starters, and what a defense is worth

The question: one Starter with no defense pushes its damage up, one has Holdfast, and one has Light Guard plus extra damage. All have blank decks. Where are all three even?

**Setup:**
- "Starter" here is the Standard template on the standard die, the generated board priced to the Starter's 6.2 level.
- Model: the race model (`ml/scripts/power_exchange/three.py`, `three_way.py`).

**No defense with more damage against Holdfast at 6.2.** Cards the Holdfast Starter needs to be even:

| No-defense damage | Against 15 cards | Against 20 cards | Against 30 cards |
|---|---|---|---|
| 6.2 (same damage) | 6.9 | 9.0 | 13.3 |
| 8 | 11.0 | 14.4 | 21.4 |
| 10 | 15.8 | 20.9 | 31.1 |
| 12 | 20.6 | 27.4 | 40.9 |

At equal damage, Holdfast plays like a deck 2.2× larger. At equal decks, Holdfast at 6.2 is worth no defense at about 10 damage.

**All three even at once.** For each no-defense Starter, the Holdfast Starter was solved first, then the one Light Guard build that ties both:

| No defense | Holdfast | Light Guard | Check (A–B, C–A, C–B) |
|---|---|---|---|
| 7.9 damage, 20 cards | 6.2 damage, 14.4 cards | 7.7 damage, 17.7 cards | 0.500, 0.499, 0.491 |
| 10.0, 20 | 6.2, 20.9 | 8.7, 21.0 | 0.500, 0.500, 0.501 |
| 12.0, 20 | 6.2, 27.4 | 10.1, 21.6 | 0.500, 0.502, 0.503 |
| 10.0, 30 | 6.2, 31.1 | 8.7, 31.2 | 0.500, 0.498, 0.494 |

At equal decks the rule is simple: **damage + average prevention is equal**. In the second row, 10 + 0 ≈ 6.2 + 3.5 ≈ 8.7 + 1.

**Even with one is not even with all (rock-paper-scissors).** Light Guard builds made even with the no-defense Starter (10 damage, 20 cards), then played against the Holdfast Starter (6.2, 20.9 cards):

| Light Guard build | Against no defense | Against Holdfast |
|---|---|---|
| 6.2 damage, 28.6 cards | 0.500 | **0.087** |
| 7.9, 22.6 | 0.500 | 0.364 |
| 8.7, 21.0 | 0.500 | 0.500 |
| 10.0, 17.9 | 0.500 | 0.587 |
| 13.9, 13.5 | 0.500 | **0.756** |

**Why:**
- Holdfast removes 3.5 from every hit. That takes 56% of a 6.2 hit but only 25% of a 14 hit.
- So against a strong defense, damage is worth far more than health, and against no defense they trade evenly.
- Health-heavy builds lose to defense; damage-heavy builds beat it.
- A defense, in turn, is worth most against opponents that hit softly.

**So no single number says what Holdfast is worth for every opponent.**
- **Damage depends on their defense:** +1 damage is worth more the more defense the opponent has.
- **Defense depends on their damage:** a defense is worth more the softer the opponent hits.
- **What a price can promise:** a price makes builds even against a chosen reference opponent. Builds that drift far apart in shape (all health against all damage) can still be lopsided against each other.

### The variable: effective cards against a reference opponent

Pick the reference opponent R: the enemy the player will mostly fight at that stage, with damage T_R and defense D_R. Then a build's strength against R is

> **Effective cards = cards × offense multiplier × defense multiplier**
>
> offense multiplier = m(your damage through D_R) ÷ m(6.2 through D_R)
> defense multiplier = m(T_R with no defense) ÷ m(T_R through your defense)

Here m is average damage per round after the defense roll, from the exact odds. Equal effective cards means even against R.

Every upgrade multiplies effective cards, so **its fair worth is a percentage of your health**: (multiplier − 1) × your cards. In XP that is the same percentage of your deck's XP value, at 10 XP per blank. A deck with real cards counts at its value in blanks (balancing-playbook.md, test 5).

**A defense as a percentage of your health, by how hard the reference opponent hits:**

| Opponent's damage | Light Guard | Holdfast | Light → Holdfast |
|---|---|---|---|
| 5 | 25% | 219% | 155% |
| 6.2 | 19% | **127%** | 91% |
| 8 | 14% | 78% | 56% |
| 10 | 11% | 53% | 38% |
| 12 | 9% | 41% | 29% |
| 15 | 7% | 30% | 22% |
| 20 | 5% | 21% | 15% |

The exact model agrees: Holdfast with 20 cards equals no defense with 44.9 cards at 6.2 (+125%), 30.6 at 10 (+53%) and 26.8 at 12 (+34%).

**+1 average damage as a percentage of your health, by the reference opponent's defense:**

| Your damage | No defense | Light Guard | Holdfast |
|---|---|---|---|
| 5 | 20% | 25% | 59% |
| 6.2 | 16% | 19% | **37%** |
| 8 | 13% | 15% | 23% |
| 10 | 10% | 11% | 15% |
| 12 | 8% | 9% | 12% |
| 15 | 7% | 7% | 9% |
| 19 | 5% | 6% | 7% |

**Worked example:** a 20-card deck (200 XP of health) facing enemies that hit for 6.2 behind Holdfast.
- Holdfast: 127% = about 254 XP. Light Guard: about 38 XP.
- +1 damage: 37% = about 73 XP.
- If the enemies hit for 10, Holdfast drops to 53% (about 106 XP).

**Two ways to charge it:**
- **Percentage of your deck** (the rule exactly). Fair for every deck size: a defense is worth more to a big deck, because it multiplies health. The price rises as the deck grows.
- **Fixed XP**, anchored at the reference deck size. Simpler. Small decks overpay and big decks underpay, which pushes players to grow health and defense together (the diminishing returns above).

Either way the percentage must follow the reference opponent's damage. If enemies hit harder as the campaign goes on, a flat defense's share falls with them.

## Pricing a defense in damage points

The question: Holdfast Starters at 6.2, 7.5, 9 and 11 average damage. What damage (same cards), or what cards (same damage), do a Light Guard Starter and a no-defense Starter need to be even? Computed with `ml/scripts/power_exchange/defense_price.py`.

**On paper:**
- **Same cards:** a defense is worth its average prevention in damage, so **damage + average prevention is equal**.
  - Light Guard needs Holdfast's damage + 2.5.
  - No defense needs Holdfast's damage + 3.5.
  - As a share of Holdfast's damage, that is (T + gap) ÷ T.
- **Same damage:** the damage that gets through is T − prevention, so the cards needed are **cards × (T − D̄_you) ÷ (T − D̄_Holdfast)**.
  - Light Guard: × (T − 1) ÷ (T − 3.5).
  - No defense: × T ÷ (T − 3.5).

**Exact, 20 cards each** (15 and 30 agree):

| Holdfast damage | Light Guard needs | No defense needs | Light Guard at the same damage needs | No defense at the same damage needs | Rounds |
|---|---|---|---|---|---|
| 6.2 | 8.7 (+2.5, +41%) | 9.7 (+3.5, +57%) | 37.8 cards (+89%; paper +93%) | 44.9 (+125%; paper +130%) | 3.8 |
| 7.5 | 10.1 (+2.5, +33%) | 11.3 (+3.7, +49%) | 32.1 (+60%; paper +63%) | 37.3 (+86%; paper +88%) | 3.0 |
| 9.0 | 11.4 (+2.5, +28%) | 12.2 (+3.3, +37%) | 29.0 (+45%; paper +45%) | 33.2 (+66%; paper +64%) | 2.9 |
| 11.0 | 13.5 (+2.6, +24%) | 15.0 (+4.1, +37%) | 27.4 (+37%; paper +33%) | 30.9 (+54%; paper +47%) | 2.0 |

**Notes:**
- **Prevention is barely wasted.** Against the Standard board, Light Guard prevents 0.99 of its 1.0 and Holdfast 3.46–3.48 of its 3.5, because hits are almost always larger than the roll.
- **The no-defense column wobbles** (+3.3 to +4.1) because whole-damage staircase steps are about 1 damage apart there.
- **Two-round fights are rough.** At 11 against 15 cards the fight is a 2-round knockout and the no-defense gap collapses to +2.6.
- **Light Guard against no defense directly** scores 0.455–0.540. That is the same whole-damage noise.

**The pricing rule this gives:**
- **A defense costs its average prevention in slider damage points:**
  - Light Guard = 1 point;
  - Holdfast = 3.5 points;
  - Light Guard → Holdfast = 2.5 points.
- **This holds at every level.** So pricing defenses in slider points keeps defense against offense balanced however high damage goes.
- **In XP:** points × the slider's XP per point. At 10 × hits-to-kill per point, 15 cards against Holdfast gives 55 XP a point: Light Guard ≈ 55, Holdfast ≈ 190, the upgrade ≈ 140. With 20 cards, 74 XP a point: about 74 / 260 / 185.
- **What changes with level is what both are worth in cards.** Holdfast in a 20-card deck, against an opponent of the same damage, is worth about 25 cards (250 XP) at 6.2, 17 (170) at 7.5, 13 (130) at 9 and 11 (110) at 11. That is the slider's own pricing question; defenses simply follow it.
- **New defenses:** price on effective prevention, E[min(hit, prevention)] against a standard hit, not the face value. It equals the face value whenever hits exceed the roll.

## Real-battle checks

Standard · standard die, greedy, 2,000 battles per point (±0.022), each side's own health and defense (`-exchange-check`):

| Point | Model | Battles | Rounds |
|---|---|---|---|
| Holdfast, 15 health: +1 damage against +5 blanks | 0.499 | 0.511 | 5.3 |
| … against +6 blanks | 0.434 | 0.443 | 5.4 |
| No defense, 20 health: +1 damage against +3 blanks | 0.502 | 0.509 | 3.6 |
| … against +4 blanks | 0.420 | 0.431 | 3.7 |
| Light Guard, 20 health: +1 damage against +2 blanks | 0.595 | 0.596 | 4.0 |
| … against +3 blanks | 0.538 | 0.541 | 4.0 |
| Holdfast, 30 health, 12 → 13: against +3 blanks | 0.502 | 0.503 | 3.8 |
| … against +4 blanks | 0.463 | 0.463 | 3.9 |
| Holdfast against Light Guard, 15 health: against +13 blanks | 0.516 | 0.522 | 5.4 |
| … against +14 blanks | 0.462 | 0.479 | 5.5 |
| Holdfast against no defense, 15 health: against +18 blanks | 0.500 | 0.510 | 5.4 |
| … against +19 blanks | 0.459 | 0.466 | 5.5 |
| Light Guard and +2.5 damage against Holdfast, 20 health | 0.489 | 0.485 | 4.0 |
| No defense and +3.5 damage against Holdfast, 20 health | 0.546 | 0.541 | 3.5 |
| Holdfast mirror, 20 health, against +1 blank | 0.430 | 0.446 | 7.1 |
| … against +2 blanks | 0.363 | 0.366 | 7.3 |
| No defense, 15 health, 12 → 13 (knockout regime) | 0.562 | 0.559 | 1.9 |

## Limits

- **No cards.** Measured with blank decks: cards add damage, prevention and variance. Whole decks can be expressed in blanks (balancing-playbook.md, test 5) and added to H.
- **One board shape.** Mirrors of one board shape. Other boards follow the same rule through their own m, and their shape effects are in board-round-robin.md.
- **The player.** Greedy players. The in-game AI plays blank decks the same way, because there is nothing to decide but the dice.

## Reproduce

From `dice-and-destiny-server/`. The scripts are in `ml/scripts/power_exchange/`; their data and outputs go to `ml/runs/exchange/` (ignored by git; `EXCHANGE_DIR` moves it):

```bash
go run ./cmd/board-lab -exchange-dump ml/runs/exchange/odds.json      # every default board's damage odds across the staircase
python3 ml/scripts/power_exchange/offense.py      # blanks per +1 damage
python3 ml/scripts/power_exchange/defense.py      # defense upgrades in blanks and in damage
python3 ml/scripts/power_exchange/lopsided.py     # the 60/40 line
python3 ml/scripts/power_exchange/builds.py       # equal-XP builds at the fair price
python3 ml/scripts/power_exchange/sensitivity.py  # the rule against the exact model on random pairings
python3 ml/scripts/power_exchange/fieldscale.py   # field spread at fixed health and at fixed hits-to-kill
python3 ml/scripts/power_exchange/defense_price.py # Holdfast at 6.2–11: damage or cards Light Guard and no defense need
python3 ml/scripts/power_exchange/three_way.py    # three Starters even at once (three.py: the helpers)
go run ./cmd/board-lab -exchange-check ml/scripts/power_exchange/checks-plan.json -battles 2000 -out ml/runs/exchange/checks.json
```
