# Card pricing model

Research note (2026-10-15). It proposes a logic system for setting XP and energy prices for General cards, and starting prices for every card in the 12 General card trees. It assumes characters built only from General cards on the Starter's board (5 standard dice, 3 rolls, Strike / Guarded Strike / Measured Strike / Small Straight / Large Straight / Decisive Blow, Guard).

The numbers come from two sources:

- **Simulated battles.** About 400,000 real authority battles played by the balance lab's `greedy` player (see [balance-lab.md](balance-lab.md)).
- **Exact dice calculations.** Exhaustive enumeration of 5 dice and 3 rolls with optimal holds.

They are a starting point for playtesting, not final prices.

## 1. The short version

1. **Price everything in one currency: cards of health.** One card is one point of health. A point of prevented damage saves one card. A point of dealt damage removes one enemy card.
2. **A card is worth its body plus how often its effect gets used.** The body (the card's own health) is guaranteed: it counts in deck, hand and discard whether or not you ever draw it. The effect is not. In the Starter mirror a single copy is played about **0.4–0.6 times per battle**.

   **Value ≈ 1 + 0.45 × (effect actually realized per play)**
3. **Your method was right, with two corrections.**
   - Weight the effect by the play rate.
   - Use the prevention that actually lands (Prevent 2 saves 1.86 cards per play, Prevent 3 saves 2.52), not the printed number.

   With both corrections your arithmetic gives 10 / 12.7 / 14.7 XP for Prevent 1 / 2 / 3. The simulator measured **10 / 12.3 / 14.2**. Your linear 10 / 15 / 20 overprices the bigger blocks.
4. **The body is most of a base card.** A blank card is worth about 6.5 XP on a scale where Steady Guard is 10. Upgrades buy only the effect, so one honest step is about +2 to +3 XP.
5. **Energy is lumpy: it comes in chunks of 5.**
   - Raising Income from 5 to 6 or 7 changed **nothing** in 5,000 games.
   - Dropping it to 4 cost as much as losing a whole card of health.
   - A 0-energy version of a card is worth about +3 XP. A 10-energy version is worth about −2 XP (−1 for blocks, −6 for draw).
   - Energy is a weak lever for balancing defensive cards. Use XP for that.
6. **Damage dealt is worth more than damage prevented.** The cause is a mill effect. Guard and Steady Guard send saved cards to discard, so every attack pushes the defender's live hand and draw-pile cards into discard even when it is fully blocked. Bigger attacks therefore cost the defender options as well as health (section 3).
7. **The current prices are off in a consistent pattern.**
   - The Strong Swing and Nudge trees are underpriced by about 20–40% (50–60% in the short 12-card mirror).
   - The "Heavy" (10-energy) versions of strong cards are the best XP deals in the game.
   - The Steady Guard upgrades, Parry, "keep the higher" rerolls, Blood Price and the capstones (Mastery, Fate, Destiny) are overpriced.

## 2. The model

### Currency

**1 H** = one card of health = one point of damage that gets through.

### Value of one copy in a deck

```
V = body + plays × effect − energy
```

| Term | What it is | Typical value now |
|---|---|---|
| body | The card's own health. It is always realized. | 1 H, about 6.5 XP (7.7 in 12-card decks, 5.1 in 24-card decks) |
| plays | How often one copy is played per battle | 0.35–0.6 |
| effect | H realized per play, after waste (see the tables below) | Prevent 1 = 1.0; Strong Swing ≈ 1.2 net damage |
| energy | The price of the card's energy cost | Small for blocks; larger for offense and draw (section 4) |

**XP = 10 × V / V(Steady Guard).** Steady Guard is the anchor at 10 XP. It also lands the average base card at about 10.

### Your Steady Guard example, step by step

Your reasoning was: Prevent n is worth n + 1 (the prevention plus the card's own health), so XP should rise in proportion, giving 10 / 15 / 20.

**Correction 1: the effect isn't always used.** A copy is played about 0.45 times per battle. Some copies are never drawn before the battle ends (battles last about 3.5–4 rounds with 12 cards). Others get milled or removed first, or no damage gets through while they are in hand.

**Correction 2: big blocks waste some prevention.** Guard absorbs the bottom of each hit first (about 3.5 on average). The prevention that actually lands per play in the Starter mirror is:

| Card | Printed | Realized per play (mirror) | vs Brine Mask | vs Regent | Holding for the better of the next 2 hits (mirror) |
|---|---|---|---|---|---|
| Prevent 1 | 1 | 1.00 | 1.00 | 1.00 | 0.99 |
| Prevent 2 | 2 | 1.86 | 1.77 | 1.90 | 1.95 |
| Prevent 3 | 3 | 2.52 | 2.31 | 2.64 | 2.80 |
| Prevent 4 | 4 | 2.99 | 2.65 | 3.16 | — |

**Result:** V = 1 + 0.45 × realized gives 1.45 / 1.84 / 2.13, which is **10 / 12.7 / 14.7 XP**. The simulator measured 10 / 12.3 / 14.2 (the average of three matchups, section 5). That is about **+2.3 XP per extra point of prevention**, not +5.

**Your one-point-hit worry.** That case is exactly what the realized column measures.
- Against an enemy whose hits always get through for 1, the realized value is 1.0 for every card, so Prevent 2 and Prevent 3 are worth the same as Prevent 1 (about 10 XP).
- Don't price for the worst case. Price for the encounter mix you actually ship, and recompute the table when that mix changes.
- With several attackers, Guard covers only one attack. The others arrive at full size, so big blocks get *more* useful in group fights, not less.
- To remove the waste instead of discounting for it, use "pay more when it's big" designs. Reinforce already does this: Prevent 2, or Prevent 4 for 5 more energy. Those cards can safely cost more.

**Energy for the example.** Your energy rule divided by the total value, health included. But you never pay energy for the body; you pay only when you play the card. Energy also mostly fails to bind at defense time, where Slow Block (10 energy) is worth only 0.8 XP less than Steady Guard (5 energy). So raising Prevent 2 to 7.5 and Prevent 3 to 10 energy would cost them about 1 XP of value each, and you would then have to cut their XP a little more. The cleaner design is flat 5 energy for the line, with the price carried by XP.

## 3. Why damage beats blocking: the mill

Each attack reserves its full size in cards: discard first, then draw pile, then hand. A block releases cards in hand → draw → discard order, and **Guard and Steady Guard send released cards to discard**. So a big attack pushes the defender's live cards into discard even when it is completely blocked.

Measured on the defender in the Starter mirror (600 battles):

| Where the reserved card was | What happened | Per battle |
|---|---|---|
| discard | removed | 8.3 |
| discard | saved, stayed in discard | 6.1 |
| draw pile | **saved, but moved to discard** | 2.8 |
| hand | **saved, but moved to discard** | 1.9 |
| draw pile | removed | 1.7 |
| hand | removed | 0.1 |

The defender loses about **4.7 live cards per battle to saves**, and only 1.9 to actual removal. Consequences:

- **Gross damage matters, not only net damage.** +2 damage removes 2 more health *and* mills up to 2 more live cards. In the 12-card mirror, a point of bonus damage was worth about 1.5× a point of prevention (section 5). The premium shrinks to about 1.2× with 24-card decks.
- **Original-pile saves (Shelter, Stalwart, Bulwark, Guard+) keep live cards live.** They measured +0.5 to +1.5 XP, not the +4 they cost now.
- **Cards deep in a 12-card draw pile often never get drawn.** This is part of why the effect weight is only about 0.45.

If the mill is not intended, the lever is the destination rule, not card prices. Changing it would change every number in this note.

## 4. Base values every card shares

All measured in the Starter mirror against an unchanged opponent, 5,000 games each. "Cards of health" uses a blank card (a map card, which has no effect in battle) measured the same way.

| Change for one side | Worth (cards of health) | Approx. XP |
|---|---|---|
| +1 blank card (pure health) | 1.00 | 6.5 (7.7 at 12 cards, 5.1 at 24) |
| +1 opening card (hand 5) | +0.84 | ~5.5 |
| +1 draw per round (Income 2 cards) | +0.67 | ~4.5 |
| +10 starting energy | +0.98 | ~6.5 |
| +5 starting energy | +0.50 | ~3.5 |
| −5 starting energy | −1.11 | ~−7 |
| Income energy 6 (+1 per round) | 0.00 | 0 |
| Income energy 7 (+2 per round) | +0.03 | 0 |
| Income energy 4 (−1 per round) | −1.11 | ~−7 |
| A 4th roll attempt (exact dice) | +0.45 net damage per offense | |
| One fewer roll (Entangle; exact dice) | −0.56 net damage per offense | |

What follows from these:

- **Energy is spent in 5s** because every card costs 0, 5 or 10. Income 5 is exactly one card per round. Extra income that doesn't reach the next multiple of 5 within a 4-round battle is wasted.
- **The planned +1/+2/+3 Income relics** (run-relic decisions) would do almost nothing at today's battle length, unless some cards cost 3, 4, 6, 7 or 8. Non-multiple-of-5 costs are what would make fine-grained energy matter. Starting-energy relics do work: +5 is worth half a card.
- **Card-based energy is worth less than starting energy.** It arrives later. Shallow Breath (+5) measured as a blank card. Second Wind (+10) was worth about +4.5 XP over a blank.

### Energy cost of the same card

| Pair | 0 energy vs 5 | 10 energy vs 5 |
|---|---|---|
| Prevent 2 (Quick Block / Firm Stance) | +2.2 | |
| Prevent 1 (Slow Block / Steady Guard) | | −0.8 |
| +2 before roll (Quick Jab / Haymaker vs Strong Swing) | +2.9 | −1.6 |
| ±1 die (Flick / Clumsy Touch vs Nudge) | +3.5 | −1.4 |
| Draw 2 (Quick Study / Overthink vs Take Stock) | +4.5 | −5.4 |
| Reroll 1 (Weary Toss vs Try Again) | | +0.1 |

The Swift (+3 to +4 XP) and Heavy (−2 XP) conventions are about right. The problem is that the bases they hang from are mispriced.

## 5. What was measured

**Method.**
- Each test adds one copy of a card to the 12-card Starter deck. Score = win + ½ draw.
- The modified deck plays the same seeds as the unchanged deck, from both seats.
- Fair XP = 10 × score gain(card) / score gain(Steady Guard).
- Players are `greedy` version 2, with energy 0.05, draw 0.75 and card 0.25. These are the settings in `cmd/card-balance`. With the default greedy settings, Take Stock is never played and energy never matters.

**Three contexts:**
- the Starter mirror (4,000 games per card);
- the same mirror with 24-card decks (every count doubled; 3,000 games; battles last 7.4 rounds instead of 3.5);
- the Drowned Regent (1,500 games).

| Card | Now | Mirror 12 | Mirror 24 | Regent | Average |
|---|---|---|---|---|---|
| blank card (body only) | — | 7.7 | 5.1 | 6.6 | 6.5 |
| Steady Guard (P1) | 10 | 10 | 10 | 10 | 10 |
| Firm Stance (P2) | 13 | 12.3 | 11.9 | 12.5 | 12.3 |
| Brace (P3) | 19 | 14.6 | 14.5 | 13.6 | 14.2 |
| Shelter (P1, original piles) | 14 | 11.1 | 10.1 | 10.4 | 10.5 |
| Stalwart (P2, original piles) | 17 | 13.7 | — | 12.4 | 13.0 |
| Bulwark (P3, original piles) | 20 | 15.9 | — | 13.5 | 14.7 |
| Slow Block (P1, 10E) | 8 | 9.4 | 9.5 | 8.8 | 9.2 |
| Quick Block (P2, 0E) | 16 | 14.6 | 14.4 | 14.5 | 14.5 |
| Strong Swing (+2 before roll) | 10 | 16.1 | 12.1 | 10.3 | 12.8 |
| Mighty Blow (+3 before) | 13 | 20.0 | 14.7 | 11.3 | 15.3 |
| Opportunist (+2 after roll) | 14 | 17.0 | 14.2 | 10.6 | 13.9 |
| Clean Cut (+3 after) | 18 | 19.7 | 16.4 | 11.6 | 15.9 |
| Haymaker (+2 before, 10E) | 8 | 13.3 | 11.1 | 9.0 | 11.2 |
| Quick Jab (+2 before, 0E) | 14 | 20.3 | — | 11.0 | 15.7 |
| Waiting Blade (+2 until it lands) | 13 | 17.4 | — | 11.1 | 14.3 |
| Nudge | 10 | 15.2 | 12.3 | 9.4 | 12.3 |
| Reach | 13 | 16.0 | — | 9.4 | 12.7 |
| Far Reach | 16 | 18.5 | — | 11.1 | 14.8 |
| Flick (0E) | 14 | 19.3 | — | 12.2 | 15.8 |
| Clumsy Touch (10E) | 8 | 13.2 | — | 8.7 | 10.9 |
| Press Down | 9 | 13.7 | — | 9.0 | 11.4 |
| Lift | 8 | 12.7 | — | 8.3 | 10.5 |
| Try Again | 10 | 9.6 | 6.3 | 7.6 | 7.8 |
| Weary Toss (10E) | 8 | 9.2 | — | 6.7 | 7.9 |
| Take Stock (D2) | 10 | 10.9 | 9.7 | 7.1 | 9.2 |
| Plenty (D3) | 13 | 12.8 | 11.0 | 7.1 | 10.3 |
| Hoard (D4) | 17 | 14.7 | — | 8.7 | 11.7 |
| Quick Study (D2, 0E) | 14 | 18.0 | — | 9.4 | 13.7 |
| Overthink (D2, 10E) | 8 | 4.8 | — | 2.7 | 3.8 |
| Second Wind (+10E) | 10 | 13.4 | 11.3 | 8.3 | 11.0 |
| Surge (+15E) | 13 | 14.1 | 11.8 | 8.4 | 11.4 |
| Shallow Breath (+5E) | 8 | 7.8 | — | 7.3 | 7.6 |

**Reading the table.**
- The 90% intervals are about ±1 XP for blocks and ±2 XP for offense cards in the mirror.
- Where Mirror 24 is blank, the average fills it with the mean of the other two contexts.
- The prevention ladder is the same in every context.
- Offense and dice cards are strongest in the short 12-card mirror and weakest against the Regent, whose hits are big and whose defense is a single die.

### Exact dice values (for cards the simulated player can't play)

The calculator enumerates 5 dice, 3 rolls and optimal holds for the whole Starter board, scoring damage net of the opponent's Guard. **Uplift** is the extra expected net damage on an offense where you hold the card.

On the ten 5-energy cards measured above, the uplift predicts the mirror price as **XP ≈ 4.6 + 9.8 × √uplift**, with a typical error of 0.8 XP. The average over contexts is about 0.8 × the mirror price. 10-energy cards then lose about 2 XP.

| Card | Uplift | | Card | Uplift |
|---|---|---|---|---|
| Try Again (reroll 1) | 0.36 | | Mastery (set 2 to any face) | 3.14 |
| Lucky Toss (reroll 1, keep higher) | **0.24** | | Precision (set 1 to any face) | 1.89 |
| Scrap Toss (reroll a 1–3) | 0.26 | | Weighted (set 1 to 6) | 0.87 |
| Double Toss (reroll 2) | 0.45 | | Mimic (copy a face) | 1.03 |
| Charmed Throw (reroll 2, higher) | **0.33** | | Echo (copy onto 2) | 2.69 |
| Nudge (±1, 1–5) | 1.18 | | Overturn (flip) | 0.98 |
| Press Down / Lift | 0.84 / 0.73 | | +1 before roll | 0.41 |
| Reach (±1, 1–6) | 1.34 | | +1 to every ability (Sweeping Arc) | 0.93 |
| Far Reach (±2) | 1.70 | | +2 before roll (Strong Swing) | 1.24 |
| Two Fingers (2 dice ±1) | 1.70 | | +2 after roll (Opportunist) | 1.89 |
| Long Fingers (2 dice ±1, 1–6) | 2.08 | | +3 before / after | 2.17 / 2.87 |
| Reckless Lunge (+2 if pair) | 1.23 | | Finisher (+4 if small straight) | 1.79 |

**Enemy dice cards.** These are played after the opponent's dice are revealed and are scored as the expected net damage removed:

| Card | Reduction | Card | Reduction |
|---|---|---|---|
| Disrupt (reroll 1) | 1.48 | Shove (±2) | 2.24 |
| Scatter (reroll 2) | 1.77 | Tamper (set any) | 2.38 |
| Jostle (±1) | 2.11 | Blunt (6 → 5) | 0.38 |
| Set 2 enemy dice | 2.62 | | |

The opponent averages 2.70 net damage per offense, so one Disrupt removes 55% of a round's damage. The Starter's board is fragile: one changed die breaks a straight or a symbol set.

**Defense rerolls** (one Guard roll, against the Starter's natural attack mix):

| Card | Prevention gained per defense |
|---|---|
| Parry | 0.47 |
| Twin Parry | 0.64 |
| Full Parry | 0.69 |

## 6. Rules of thumb (the logic system)

1. **Body ≈ 6.5 XP.** Any effect must be worth more than that to beat a blank card.
2. **Blocks:** XP ≈ 7.2 + 2.8 × realized prevention per play (section 2 table).
   - Original piles: +1 XP.
   - 0 energy: +2.
   - 10 energy: −1.
3. **Damage and dice cards:** XP ≈ 0.8 × (4.6 + 9.8 × √uplift), using the dice calculator.
   - 0 energy: +3.
   - 10 energy: −2.
   - "After the roll" is worth most of one extra point of damage before the roll: +2 after gains 1.89, between +2 before (1.24) and +3 before (2.17).
4. **Draw:** about +1.3 XP per extra card on a 5-energy draw card. Draw is very energy-sensitive (+4.5 at 0E, −5.4 at 10E).
5. **Energy cards:** +5 ≈ a blank card; +10 ≈ 11 XP; +15 ≈ 11.5 XP. Energy only matters when it completes another 5.
6. **Health traded for resources (Blood Price):** the sacrifice costs a whole body (about 6.5 XP) plus whatever the sacrificed card did. In short battles the payoff must be about double today's to break even.
7. **Combining two lines** (your +10 rule): price the combination as the better parent plus the realized value of the added part. The two halves rarely both pay off on the same play.
8. **Verify, then playtest.**
   - Add one copy and compare it with Steady Guard and a blank card on the same seeds, in the mirror and against two minions.
   - Accept a card within ±15% of its rule-of-thumb price.
   - Then play it against the look-ahead mirror.

## 7. Suggested starting prices

**Now** = current XP. **Suggested** = the number to playtest.
- *m* = measured (section 5).
- *d* = exact dice calculator plus the fit.
- *e* = estimate from the rules.

Energy is unchanged unless noted.

### Steady Guard (prevention)

| Card | Effect | Now | Suggested | Basis |
|---|---|---|---|---|
| Steady Guard | P1 → discard | 10 | 10 | anchor |
| Firm Stance | P2 → discard | 13 | 12 | m |
| Brace | P3 → discard | 19 | 14 | m |
| Shelter | P1 original | 14 | 11 | m |
| Stalwart | P2 original | 17 | 13 | m |
| Bulwark | P3 original | 20 | 15 | m |
| Slow Block | P1, 10E | 8 | 9 | m |
| Quick Block | P2, 0E | 16 | 15 | m |
| Reinforce | P2 original, or P4 original for +5E | 20 | 16 | e (the choice removes waste) |
| Triage | save one threatened card, original | 16 | 11 | e (≈ Shelter) |
| Field Dressing | save one card → hand | 19 | 12 | e (Shelter + a draw) |
| Warden's March / Sentry's Watch | P1 + map effect | 14 / 13 | 10 + map value | Steady Guard in battle |

### Strong Swing (damage bonus)

| Card | Effect | Now | Suggested | Basis |
|---|---|---|---|---|
| Strong Swing | +2 before roll | 10 | 13 | m |
| Mighty Blow | +3 before | 13 | 15 | m |
| Opportunist | +2 after roll | 14 | 15 | m, d (after > before; the simulated player under-uses it) |
| Clean Cut | +3 after | 18 | 17 | m, d |
| Haymaker | +2 before, 10E | 8 | 11 | m |
| Quick Jab | +2 before, 0E | 14 | 16 | m |
| Waiting Blade | +2, kept until it lands | 13 | 14 | m |
| Sweeping Arc | +1 to all abilities | 13 | 11 | d |
| Honed Edge | +1 for the battle, 10E | 20 | 11 | d (about 13 in long battles) |
| Reckless Lunge | +2 if pair | 8 | 13, or change the condition | d (Strike always has a pair, so it equals Strong Swing) |
| Finisher | +4 if small straight | 15 | 14 | d |

### Nudge (own dice)

| Card | Effect | Now | Suggested | Basis |
|---|---|---|---|---|
| Nudge | ±1, 1–5 | 10 | 12 | m |
| Reach | ±1, 1–6 | 13 | 13 | m |
| Far Reach | ±1/±2 | 16 | 15 | m |
| Two Fingers | 2 dice ±1 | 14 | 14 | d |
| Long Fingers | 2 dice ±1, 1–6 | 18 | 15 | d |
| Flick | ±1, 0E, once per round | 14 | 16 | m |
| Clumsy Touch | ±1, 10E | 8 | 11 | m |
| Press Down / Lift | −1 / +1 | 9 / 8 | 11 / 10 | m |
| Overturn | flip | 9 | 11 | d |
| Mimic | copy a face | 13 | 12 | d |
| Weighted | set 1 to 6 | 17 | 11 | d |
| Echo | copy onto 2, 10E | 18 | 14 | d |
| Precision | set 1 to any face, 10E | 22 | 12 | d |
| Mastery | set 2 to any face, 10E | 35 | 16 | d |
| Fate (shared) | set any participant's die, 10E | 25 | 15 | d (Precision or Tamper, plus the choice) |
| Destiny (shared) | set 2 of any participant's dice, 10E | 40 | 16 | d |

### Try Again (rerolls)

These are weak by construction. A full 4th roll is worth only +0.45 net damage, and Try Again already gets 80% of that. Either price the line as filler (below) or change what it does.

| Card | Effect | Now | Suggested | Basis |
|---|---|---|---|---|
| Try Again | reroll 1 | 10 | 8 | m |
| Lucky Toss | reroll 1, keep higher | 13 | 7 | d (worse than Try Again) |
| Double Toss | reroll 2 | 14 | 9 | d |
| Charmed Throw | reroll 2, keep higher | 18 | 8 | d |
| Quick Toss | Lucky Toss at 0E | 16 | 10 | d |
| Fresh Roll | reroll any, uses a roll attempt | 17 | ? | Looks like a normal reroll; check the design |
| Weary Toss | reroll 1, 10E | 8 | 8 | m |
| Scrap Toss | reroll a 1–3 | 8 | 8 | d |
| Parry / Twin / Full | reroll 1 / 2 / any defensive dice | 13 / 16 / 19 | 9 / 9 / 10 | d |
| Heavy Parry | Parry, 10E | 11 | 8 | d |

### Disrupt (enemy dice, reaction)

| Card | Effect | Now | Suggested | Basis |
|---|---|---|---|---|
| Disrupt | reroll 1 enemy die | 10 | 13 | d |
| Scatter | reroll 2 | 14 | 14 | d |
| Jolt | Disrupt, 0E, once per round | 14 | 16 | d |
| Stumble | Disrupt, 10E | 8 | 11 | d |
| Jostle | ±1 | 13 | 15 | d |
| Shove | ±2 | 16 | 15 | d |
| Tamper | set any face, 10E | 22 | 14 | d |
| Blunt | a 6 → 5 | 9 | 9 | d |

These assume the attacker's chosen ability is locked when you change its die. If the attacker can re-pick or respond, the values drop. Verify these in play before trusting them.

### Take Stock (draw)

| Card | Effect | Now | Suggested | Basis |
|---|---|---|---|---|
| Take Stock | draw 2 | 10 | 9 | m |
| Plenty | draw 3 | 13 | 10 | m |
| Hoard | draw 4 | 17 | 12 | m |
| Quick Study | draw 2, 0E | 14 | 14 | m |
| Overthink | draw 2, 10E | 8 | redesign | m (worse than a blank card) |

### Second Wind (energy)

| Card | Effect | Now | Suggested | Basis |
|---|---|---|---|---|
| Second Wind | +10 energy | 10 | 11 | m |
| Surge | +15 energy | 13 | 11 | m |
| Shallow Breath | +5 energy | 8 | 8 | m (≈ blank) |
| Focus | draw 2 (+5E) or +10 energy | 13 | 12 | e |
| Deep Focus | draw 3 (+5E) or +15 energy | 17 | 13 | e |

### Reclaim, Blood Price, Salve, Dispel, Pathfinder (estimates only)

These weren't simulated. The simulated player can't play them, or (Salve, Dispel) they have no targets in a General-only battle.

| Card | Now | Suggested | Note |
|---|---|---|---|
| Reclaim | 10 | 10 | Replays your best used card; also rescues it from discard, which is removed first |
| Salvage / Rescue / Restock | 14 / 14 / 8 | 13 / 11 / 7 | Restock's card goes back into a draw pile that gets milled |
| Revive / Rebirth | 20 / 26 | 10 / 12 | Health-neutral by the revive-trade rule; worth about Reclaim / Salvage |
| Blood Price | 10 | 6 | A sacrificed card costs a body; payoffs need about 2× to break even |
| Deep Cut / Blood Oath / Covenant | 14 / 13 / 18 | 7 / 7 / 8 | |
| Old Wounds / Martyr / Nick | 15 / 14 / 8 | 7 / 7 / 4 | Old Wounds' discard sacrifice is the cheapest cost in the tree |
| Desperation | 18 | 4 | Two bodies is a large cost |
| Salve tree, Dispel tree | 10–36 | keep the ladder; ~8 as a base in General-only play | Salve has nothing to cleanse and Dispel can only strip Protect. Value comes from Venom, Curse and status minions |
| Pathfinder (map cards) | 10–15 | blank in battle (~6.5) + map value | Price them on the map economy |

## 8. Design flags found along the way

1. **Blocks mill your own cards** (section 3). This is the biggest structural effect on card values.
2. **Income above 5 does nothing.** Fine-grained energy needs costs that aren't multiples of 5, or longer battles.
3. **Heavy downgrades of strong cards are bargains.** Haymaker, Clumsy Touch, Press Down, Lift and Reckless Lunge are each worth 2.5–5 XP more than they cost. An optimizing player would fill a deck with them.
4. **Reckless Lunge's condition never applies** to its best target, Strike.
5. **"Keep the higher" is a downgrade on this board.** Lucky Toss, Charmed Throw and Quick Toss are priced as upgrades.
6. **Overthink is worse than a blank card.**
7. **Capstones cost 1.5–2.5× their battle value:** Mastery, Destiny, Fate, Precision, Tamper, Rebirth, Honed Edge. That's fine if they're meant as aspirational, but players who do the math won't buy them.
8. **Enemy-dice reactions are strong on this board** because straights and symbol sets are fragile.
9. **Salve and Dispel are dead in General-only battles.**
10. **Base cards aren't equal.** Strong Swing (13) and Nudge (12) are worth more than Try Again (8) and Take Stock (9), all priced at 10. Either price the bases differently, or keep 10 and adjust their effects. For example, Strong Swing at +1 before the roll comes out around 9 in the dice calculator.

## 9. Limits

- **The `greedy` player is simple.**
  - It plays a block whenever damage gets through, so it never holds Brace for a big hit. Human values for big blocks are a bit higher; the holding column in section 2 is the ceiling.
  - Its after-roll timing isn't optimal, so Opportunist and Clean Cut are probably undervalued.
- **One copy added to one deck.** This measures marginal value at 12 (or 24) cards. Several copies, other decks, other boards or other matchups will move the numbers. The Regent column shows how much: offense cards lose about a third.
- **Stand-in opponent.** The energy and draw tests used an opponent that is byte-identical to the Starter except for its character ID (`adventurer`). It scored only 0.30 against the Starter, cause not investigated. The paired deltas are still valid; their unit was re-measured with a blank card in the same setup.
- **One-turn dice values.** The dice calculator scores one offense in isolation. The fitted conversion absorbs the average multi-turn effects, but not combos between cards.
- **Rerun after rule changes.** Changes to the destination rule, battle length (deck sizes, enemy health) or Income would invalidate these numbers.
- **Tools.** The record-saving battle probe (a variant of `cmd/card-balance`) and the Python dice calculator behind sections 3–5 were scratch tools and are not in the repo yet.
