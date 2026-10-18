# Card value rules

This is the rule base for pricing General cards: their energy cost, what they do, and their XP. It was agreed on 2026-10-15 and is the starting point for every new card.

The numbers are theoretical by design. Simulated battles disagree in places, mostly on how much energy matters, because today's decks put little strain on the energy pool. We expect that to change as balancing adds more strain. Playtest, then adjust.

## 1. The currency

Everything is priced in **units (U)**.

| Thing | Worth | Why |
|---|---|---|
| 5 energy | **1U** | The base unit, so 1 energy = 0.2U |
| 1 card drawn | **1U** | A draw adds no health (the card already counted); it gives you a card to play, and an average card's effect nets about 1U |
| 1 Move on the map | **1U** | The map already uses this rate: 1 card = 1 Move = 5 energy (`expedition.yaml`) |
| 1 health | **2U** | One card of health: a point of damage prevented or dealt, a card sacrificed, removed or healed |

**The card you play costs nothing.** It goes to discard and still counts as health. Only its energy cost counts against it.

## 2. What a card is worth

Every card has two parts:

- **Body:** the one point of health it gives you just by being in your deck. It is worth **2U** whether or not you ever play the card.
- **Effect:** what playing the card gives you, minus its energy cost. This is the card's **net**.

**XP = 5 × (2 + net)**. Half points round up (17.5 → 18).

| Card | Body | Net effect | Total | XP |
|---|---|---|---|---|
| Blank card (no effect) | 2U | 0 | 2U | **10** |
| Base card | 2U | +1U | 3U | **15** |
| One step up | 2U | +2U | 4U | **20** |
| Two steps up | 2U | +3U | 5U | **25** |

- **A base card nets +1U, so it costs 15 XP.** Exceptions are set by hand: Try Again 13, Reclaim 13.
- **Each extra unit of effect costs +5 XP.**
- **Floor: 10 XP.** A card is never worth less than a blank card, because you can always just not play it. A card that nets 0 or less is priced at 10 and flagged for redesign.

## 3. Counting an effect

| Effect | How it is counted |
|---|---|
| Energy gained | 0.2U per energy (+15 energy = 3U) |
| Cards drawn | 1U each |
| Damage prevented | 2U per point, counted in full. Big blocks waste some points on small hits today (Prevent 2 / 3 / 4 saved 1.86 / 2.52 / 2.99 cards per play); that discount was dropped on 2026-10-16, because bigger hits are coming and the energy ladder already charges for big blocks |
| Saved cards stay in their piles ("original") | +1 XP flat, whatever the block's size (the user's choice, 2026-10-16; see Steady Guard) |
| Saved card goes to hand | +1U on top of the save |
| Damage dealt, dice changes | 2U per point of **expected** extra net damage (or damage avoided), from the exact dice calculator: 5 dice, 3 rolls, best holds, after the opponent's Guard |
| Sacrifice from hand | −3U: the card's health (2U) and the card in hand (1U) |
| Sacrifice from discard, or the card removes itself | −2U: health only |
| Revive or heal a removed card | +2U per card, +1U more if it comes back to hand |
| A chosen card instead of a random one | +0.5U |
| "Choose one" | The better option, +0.5U for having the choice. Exception: when every card in a tree has the same choice (Blood Price), its base already includes it |
| "Any time, Offense or Defense" instead of "before your first roll" | +0.5U |
| A block played "Any time" in Defense instead of "before your first defense roll" | +1 XP flat (see Steady Guard) |
| "Once per round" on a free card | Close to 0. You rarely hold two copies |

**Up-front cost.** When two cards in a tree net the same, the one that needs less energy to play is better: you can play it on an empty pool, and it doesn't compete with your other plays that turn. It costs **+1 XP** more for each 5 energy less (0.2U, about one energy) than the equal-net card it is compared with. Pay 10 gain 15 (15 XP), pay 5 gain 10 (16) and pay 0 gain 5 (17) all net +5 energy; pay 5 gain 15 (20) and pay 0 gain 10 (21) both net +10. The bonus is only between equal-net cards: a card with no equal-net partner in its tree is priced by its net alone, so Swift Breath (pay 5 gain 15) is 20 and Surge (pay 0 gain 15) is 25. Added 2026-10-16 with the Second Wind tree. The size of the bonus may not hold for every tree; check it in each tree review.

**Expected values for dice effects** (extra net damage per offense where you hold the card; the opponent averages 2.70):

| Own dice | Expected | | Enemy dice and damage bonuses | Expected |
|---|---|---|---|---|
| Reroll 1 | 0.36 | | Reroll 1 enemy die | 1.48 |
| Reroll 1, keep higher | 0.24 | | Reroll 2 enemy dice | 1.77 |
| Reroll 2 | 0.45 | | ±1 enemy die | 2.11 |
| Reroll 2, keep higher | 0.33 | | ±2 enemy die | 2.24 |
| Reroll a 1–3 | 0.26 | | Set an enemy die | 2.38 |
| ±1 (1–5) | 1.18 | | Set 2 enemy dice | 2.62 |
| ±1 (1–6) | 1.34 | | Enemy 6 → 5 | 0.38 |
| ±1 or ±2 | 1.70 | | +1 / +2 / +3 / +4 before the roll | 0.41 / 1.24 / 2.17 / 3.14 |
| 2 dice ±1 (1–5 / 1–6) | 1.70 / 2.08 | | +2 / +3 after the roll | 1.89 / 2.87 |
| −1 only / +1 only | 0.84 / 0.73 | | +1 to every ability | 0.93 |
| Flip | 0.98 | | +2 if a pair | 1.23 |
| Copy a face / onto 2 | 1.03 / 2.69 | | +4 if a small straight | 1.79 |
| Set to 6 | 0.87 | | Reroll 1 / 2 / any defensive dice | 0.47 / 0.64 / 0.69 prevented |
| Set any face / 2 dice | 1.89 / 3.14 | | | |

These come from the Starter's board (5 standard dice; Strike, Guarded Strike, Measured Strike, Small Straight, Large Straight, Decisive Blow; Guard). Recompute them for other boards.

**Narrowed effects used by the downgrades:**

| Effect | Expected | | Effect | Expected |
|---|---|---|---|---|
| Reroll a 1–2 / a 1 / a 4–6 | 0.22 / 0.15 / 0.30 | | Reroll an enemy 4–6 / 1–3 / 6 | 1.22 / 1.27 / 0.26 |
| ±1 (1–4) / (2–4) | 0.99 / 0.94 | | Reroll a defensive 1–3 / 6 | 0.14 / 0.38 prevented |
| ±1 on a die showing 1–3 / 4–6 | 0.78 / 0.78 | | +2 with 4+ swords / 5 swords | 1.11 / 0.58 |
| | | | +2 / +3 with a small straight | 0.62 / 1.20 |

## 4. Pricing a tree from its base

Trees are priced **relative to their base**, which is pinned to its agreed value:

**variant net = base net + (variant's effect − base's effect) − (variant's energy − base's energy) ÷ 5**

This keeps any error in a base's absolute value from spreading through its tree. The clearest example is Strong Swing. Its +2 damage lands only about 1.24 times per play, but damage that Guard blocks still pushes the defender's cards into discard, so the base is pinned at +1U by agreement and its upgrades are priced from it.

**When a base's energy or effect changes, its upgrades shift with it.** Every node keeps its one change from its parent. Exceptions:
- A free node (Swift) stays free.
- Shared cards have one definition, so they follow the larger shift: Fate and Destiny went to 15 energy with Disrupt.

## 5. The map

The map has its own unit values (agreed 2026-10-17), because energy is scarcer there: it refills only when you rest, and a battle never gives back more than you brought in.

| On the map | Value | Why |
|---|---|---|
| 1 Move | **0.5U** (2 Move = 1U) | |
| 5 energy | **1U** | It buys 1 Move or 1 Influence. Paying 5 energy for 1 Move is a trade-down, but it is an option |
| 1 Influence | **1U** | At a village, 2 Influence mends 1 health (2U) |
| 1 health | 2U | unchanged |

**A card is worth its better use.** Each copy is played once per pass through the deck, on the map or in battle, so **XP = 5 × (2 + the better of its battle net and its map net)**. Battle cards are priced on their battle net; map-only cards on their map net (they are blank in battle).

- **Moving cards:** map net = Move − energy. The 1 Move any card gives is the same for every map card, so it is left out. Trailblaze (Move 4 for 5 energy) = 2U − 1U = **+1U, 15 XP**.
- **Site and camp cards:** here the card you play could have been 1 Influence (1U), and energy buys Influence too, so these are priced against that. Silver Tongue (2 Influence, free) = 2U − 1U = **+1U, 15 XP**. A 5-energy version would give exactly what the card and the energy already buy.
- **Information and stealth cards** (sight, scouting, marks, Hide, Sneak) have no unit. Each tree's base is pinned at 15 by agreement and its upgrades are judged as steps from it.
- **Surge check:** Surge (free +15 energy, 25 XP) is 3U on the map, but spent on moving it buys 3 Move. Trailblaze nets 3 Move too (Move 4 for 5 energy that could have bought 1). They tie for moving; Trailblaze is 10 XP cheaper and Surge also works in battle. Accepted (user, 2026-10-17).

## 6. The base cards

| Card | Energy | Effect | Pays | Gets | Net | XP |
|---|---|---|---|---|---|---|
| Second Wind | 10 | Gain 15 energy | 2U | 3U | +1U | 15 |
| Take Stock | 5 | Draw 2 | 1U | 2U | +1U | 15 |
| Steady Guard | 5 | Prevent 1 | 1U | 2U | +1U | 15 |
| Strong Swing | 10 | +2 damage before the roll | 2U | about 3U (1.24 lands, the rest still mills) | +1U | 15 |
| Nudge | 7 | ±1 on one die (1–5) | 1.4U | 2.4U | +1U | 15 |
| Try Again | 3 | Reroll 1 die | 0.6U | 0.7U (measured; set by hand) | +0.1U | 13 |
| Disrupt | 10 | Reroll 1 enemy die | 2U | 3U | +1U | 15 |
| Reclaim | 5 | A chosen card from discard to hand | 1U | 1.5U | +0.5U | 13 |
| Salve | 5 | Remove 1 debuff stack, before the roll | 1U | 2U (assumed) | +1U | 15 |
| Dispel | 5 | Remove 1 enemy buff stack, after your defense roll | 1U | 2U (assumed) | +1U | 15 |
| Blood Price | 0 | Sacrifice 1 from hand, then draw 4 or gain 20 energy | 3U | 4U | +1U | 15 |
| Trailblaze | 0 | Map: 3 Move | 1U | 3U | +2U map, 0 battle | 15 (open) |

**Nudge at 7 energy is deliberate.** Energy currently arrives in 5s (start 10, +5 per round, every other cost a multiple of 5), so 7 tends to play like 10. Keep it as a test of finer energy costs while balancing adds strain on the energy pool.

## 7. Every tree

**Change** is the node's one change from its parent. Energy and XP show old → new. **Effect value** is gross, before energy. A † marks a note below the table.

### Second Wind (energy)

Reviewed 2026-10-16. The up-front column is the bonus from section 3, counted only against the equal-net card named.

```
Second Wind ─┬─ Even Breath ─┬─ Easy Breath ──┐
             │               └─ Swift Breath ─┴─ Calm Breath ── Surge ──┐
             ├─ Focus ──────────────────────────────────────────────────┴─ Deep Focus
             └─ Shallow Breath (downgrade)
```

| Card | Change | Energy | Effect value | Net | Up-front | XP |
|---|---|---|---|---|---|---|
| Second Wind | base | 10 | +15 energy = 3U | +1.00U | — | **15** |
| Even Breath | pay 5, gain 10 | 5 | +10 energy = 2U | +1.00U | +1 vs Second Wind | **16** |
| Easy Breath | pay 0, gain 5 | 0 | +5 energy = 1U | +1.00U | +2 vs Second Wind | **17** |
| Swift Breath | pay 5, gain 15 | 5 | +15 energy = 3U | +2.00U | — | **20** |
| Calm Breath | pay 0, gain 10 | 0 | +10 energy = 2U | +2.00U | +1 vs Swift Breath | **21** |
| Surge † | pay 0, gain 15 | 0 | +15 energy = 3U | +3.00U | — | **25** |
| Focus | costs 5; choose draw 2 or +10 energy | 5 | best option 2U, +0.5 choice | +1.50U | — | **18** |
| Deep Focus | free; choose draw 3 or +15 energy | 0 | best option 3U, +0.5 choice | +3.50U | — | **28** |
| Shallow Breath † | gains 12 instead of 15 | 10 | +12 energy = 2.4U | +0.40U | — | **12** |

- Each energy card's map effect gains the same amount as its battle effect.
- † **Surge:** On the map, +15 energy buys 3 Move, which ties Trailblaze's net for moving; Trailblaze is 10 XP cheaper and Surge also works in battle. Settled with the map units (§5).
- † **Shallow Breath:** Downgrade: same cost, smaller gain, less XP.

### Take Stock (draw)

Reviewed 2026-10-16. The free line (Glance, Quick Study, Close Study) is priced with the up-front bonus from section 3 against the equal-net card on the 5-energy line.

```
Take Stock ─┬─ Plenty ── Hoard
            ├─ Glance ── Quick Study ── Close Study
            └─ Overthink (downgrade)
```

| Card | Change | Energy | Effect value | Net | Up-front | XP |
|---|---|---|---|---|---|---|
| Take Stock | base | 5 | draw 2 = 2U | +1.00U | — | **15** |
| Plenty | draw 3 | 5 | draw 3 = 3U | +2.00U | — | **20** |
| Hoard † | draw 4 | 5 | draw 4 = 4U | +3.00U | — | **25** |
| Glance | free, draw 1, once per round | 0 | draw 1 = 1U | +1.00U | +1 vs Take Stock | **16** |
| Quick Study | free, draw 2, once per round | 0 | draw 2 = 2U | +2.00U | +1 vs Plenty | **21** |
| Close Study † | free, draw 3, once per round | 0 | draw 3 = 3U | +3.00U | +1 vs Hoard | **26** |
| Overthink † | costs 7 | 7 | draw 2 = 2U | +0.60U | — | **13** |

- Each card's map effect draws the same number as its battle effect.
- "Once per round" on the free cards is worth about 0 (you rarely hold two copies); it stops a hand of free draw cards from chaining.
- † **Hoard, Close Study:** playtest watch. With a hand limit of 6 (discard down at the end of the round) and 10 + 5 energy a round, the last card of a big draw is often discarded unplayed. If they feel weak, count the 4th (or 3rd free) draw as about 0.5U.
- † **Overthink:** Downgrade: Take Stock for 2 more energy (1.4U), same timing. 5 × 2.6 = 13 exactly. Energy that isn't a multiple of 5 is priced at its exact share of a unit.

### Steady Guard (prevention)

Two switches shape a block, and they interact through the save order. Every save, from a card or a rolled defense, gives back hand cards first, then the draw pile, then discard. So whoever saves first gets the live cards.

- **Destination:** discard, or the saved cards stay in their piles.
- **Timing:** before your first defense roll, or any time.

Worst to best: discard/before (the base), discard/any time, pile/before, pile/any time. A pile block played before Guard keeps the live cards that Guard would have sent to discard.

**The backbone** is discard/before, and each extra point costs 2 more energy: +2U − 0.4U = +1.6U, or +8 XP. Each switch then adds a flat **+1 XP**, whatever the block's size (the user's choice, 2026-10-16). A per-point pile bonus made the big pile blocks run away (Citadel 49).

| Family | Energy | Discard, before | Discard, any time | Pile, before | Pile, any time |
|---|---|---|---|---|---|
| Prevent 1 | 5 | **Steady Guard 15** | Watchful Guard 16 | Cover 17 | Shelter 18 |
| Prevent 2 | 7 | **Hold Fast 23** | Firm Stance 24 | Hunker Down 25 | Stalwart 26 |
| Prevent 3 | 9 | **Shield Wall 31** | Brace 32 | Dig In 33 | Bulwark 34 |
| Prevent 4 | 11 | **Fortress 39** | Rampart 40 | Entrench 41 | Citadel 42 |
| Save a chosen card | 5 | Snatch 16 | Pull Clear 17 | Steady Hands 18 | Triage 19 |
| Choose prevent 2, or 4 for 4 more | 7 | Double Up 42 | Shore Up 43 | Buttress 44 | Reinforce 45 |

- **Save to hand:** Bandage (before) 20, Field Dressing (any time) 21.
- **How the families are priced:**
  - Save a chosen card is the prevent 1 row + 1.
  - The choice family is Citadel + 3 (0.5U for the choice).
- **Edges:** each family is a diamond (discard/before → discard/any time and pile/before → pile/any time). The backbone runs Steady Guard → Hold Fast → Shield Wall → Fortress.
  - Steady Guard → Snatch, and Fortress → Double Up.
  - Steady Hands → Bandage → Field Dressing, and Triage → Field Dressing.
- **Removed (2026-10-16):**
  - Warden's March: 2 Move for 5 energy is no better than walking with the card and its energy.
  - Slow Block: the base is now the weakest block.
  - The free prevent 2 family (Quick Hold, Quick Block, Quick Cover, Quick Shelter): free blocks work against putting pressure on energy.
  - The map-sight family (Sentry's Watch, Lookout, High Ground, Watchtower): exploration cards should be their own cards, with nothing to do with battle. See the to-do in section 8.
- Full counting overprices the big blocks against today's small hits. Check them in playtesting once enemies hit harder.

### Strong Swing (damage bonus)

Reviewed 2026-10-16. The user set the anchor: +2 for 10 energy, played before the roll, is 15 XP. "Kept until it lands" is a little better than "after the roll". The steps:

- **Each extra point of damage:** 2 more energy, +2U − 0.4U = +1.6U, or **+8 XP** (the same steps as the block ladder).
- **After the roll, on an ability that qualifies:** **+4 XP**.
- **Kept until it lands** (a miss doesn't use it up): **+5 XP**.

Both switches remove the miss waste of a bonus played before the roll, so they aren't combined. An after-the-roll bonus goes on an ability that already qualifies.

| Damage | Energy | Before the roll | After the roll | Kept until it lands |
|---|---|---|---|---|
| +2 | 10 | **Strong Swing 15** | Opportunist 19 | Waiting Blade 20 |
| +3 | 12 | **Mighty Blow 23** | Clean Cut 27 | Patient Blow 28 |
| +4 | 14 | **Crushing Blow 31** | Sure Strike 35 | Bide 36 |
| +5 | 16 | **Devastate 39** | Killing Stroke 43 | Long Wait 44 |

- **Edges:** the backbone runs Strong Swing → Mighty Blow → Crushing Blow → Devastate. Each backbone card → its after-the-roll and kept-until-it-lands versions.
- **Removed (2026-10-16):**
  - Quick Jab: free, which works against energy pressure.
  - Reckless Lunge: Strike always has a pair, so it was the base card.
  - Haymaker: below the base.
  - Sweeping Arc: nets 0.
  - Honed Edge: can't be priced with confidence (35 counted in full, about 12 by what lands).
  - Finisher.

### Nudge (own dice)

Reviewed 2026-10-16. Prices are set by hand, the user's call. The measurements are kept for reference: Starter board, extra damage per offense.

| Card | Energy | Effect | Measured | XP |
|---|---|---|---|---|
| Lift | 7 | +1 only (1–5) | 0.73 dmg | **12** |
| Press Down | 7 | −1 only (1–5) | 0.84 dmg | **13** |
| Nudge | 7 | ±1 on one die (1–5), base | 1.18 dmg | **15** |
| Reach | 7 | ±1, can reach 6 | 1.34 dmg | **17** |
| Weighted † | 7 | set one die to 6 | 0.87 dmg | **20** |
| Mimic † | 7 | copy another die's face | 1.03 dmg | **22** |
| Precision | 12 | set one die to any face | 1.89 dmg | **23** |
| Fate (shared) | 15 | set any participant's die, also as a reaction | about 2.6 dmg | **26** |
| Mastery | 12 | set 2 dice to any face | 3.14 dmg | **31** |
| Destiny (shared) | 15 | set 2 of any participant's dice | about 3.5 dmg | **34** |

- **Edges:**
  - Nudge → Reach → Weighted → Precision.
  - Nudge → Mimic → Precision.
  - Nudge → Press Down → Lift.
  - Precision → Mastery and Fate, and both → Destiny.
- † **Weighted, Mimic:** playtest watch. The user judged that the Starter board measurement undervalues a sure 6 and copying a face (they complete straights and sets), so they are priced well above it. Precision, Fate and Mastery were raised to keep the paths climbing.
- **Fate and Destiny** are shared with Disrupt: one definition (15 energy) and, by the user's rule, the same XP in both trees.
- **Removed (2026-10-16):** Clumsy Touch, Overturn, Two Fingers, Far Reach, Flick (free), Long Fingers and Echo.

### Try Again (rerolls)

Reviewed 2026-10-16. The tree is no longer free. The measurement scores rerolls low (one reroll adds 0.36 damage, 0.7U, because the test player already has 3 rolls with the best holds, and it counts expected damage only). Like Nudge, the prices are set by hand. Rerolls cost less energy than Nudge: a random change, not a sure one.

| Card | Energy | Effect | Measured | XP |
|---|---|---|---|---|
| Scrap Toss | 3 | reroll a die showing 1–3 | 0.26 dmg | **12** |
| Try Again | 3 | reroll 1 of your dice, base | 0.36 dmg | **13** |
| Double Toss | 5 | reroll up to 2 of your dice | 0.45 dmg | **15** |
| Full Parry | 5 | reroll any of your defensive dice | 0.69 prevented | **16** |
| One More Roll | 7 | +1 offensive roll attempt this Offense (4 instead of 3), any time | not measured | **17** |

- **Edges:** Try Again → Scrap Toss, Try Again → Double Toss → One More Roll, and Try Again → Full Parry.
- **One More Roll** uses the new `extra_roll` card effect. It raises this Offense's roll count, even after the last roll, and the count resets to 3 at the next round's Ongoing Effects. The roll counter shows it.
- **3 energy** is an experiment like Nudge's 7. Energy arrives in 5s, so within a round a 3 can play like a 5, but the leftover carries over.
- **Removed (2026-10-16):** Lucky Toss, Charmed Throw and Quick Toss ("keep the higher" is a downgrade on this board), Weary Toss (a twin of Scrap Toss), Fresh Roll (it used a normal roll attempt, so it did nothing), Parry, Twin Parry and Heavy Parry.

### Disrupt (enemy dice)

Reviewed 2026-10-16. These cards change an enemy's attack dice after they are revealed, which breaks the attack the enemy chose, so they measure far above rerolling your own dice.

| Card | Energy | Effect | Measured | XP |
|---|---|---|---|---|
| Disrupt | 10 | reroll 1 enemy die, base | 1.48 dmg avoided | **15** |
| Jostle | 10 | ±1 on an enemy die | 2.11 dmg avoided | **21** |
| Tamper | 15 | set an enemy die to any face | 2.38 dmg avoided | **25** |
| Fate (shared) | 15 | set any participant's die, after your roll or as a reaction | about 2.6 | **26** |
| Destiny (shared) | 15 | set 2 of any participant's dice | about 3.5 | **34** |

- **Edges:** Disrupt → Jostle → Tamper → Fate → Destiny.
- **Tamper** is set by hand above Jostle, like Precision in Nudge: it does more, for 5 more energy.
- **Shared cards cost the same XP in every tree** (the user's rule, 2026-10-16). Fate and Destiny are 26 and 34 in both Disrupt and Nudge.
- **Removed (2026-10-16):** Stumble, Scatter, Shove, Jolt (free) and Blunt (worse than a blank card).

### Reclaim (recovery)

Reviewed 2026-10-16. The revive trade was dropped, so Revive and Rebirth heal: each revived card returns to hand and heals 1 health. Healing 1 to hand is worth 3U (2U health + 1U in hand), more than a prevent to discard. At the user's request healing costs more energy than a block.

| Card | Energy | Effect | Calculation | XP |
|---|---|---|---|---|
| Reclaim † | 5 | a chosen discard card to hand, before the roll | 1U + 0.5 choice − 1U; set at 13 | **13** |
| Restock | 5 | up to 2 chosen discard cards to the draw pile | 2 × 0.5U + 0.5 choice − 1U = +0.5U | **13** |
| Rescue | 5 | Reclaim, any time in Offense or Defense | + 0.5U timing | **15** |
| Salvage | 5 | up to 2 discard cards to hand | 2 × 1.5U − 1U | **20** |
| Revive | 7 | heal 1: a chosen removed card to hand | 3U + 0.5 choice − 1.4U = +2.1U | **21** |
| Rebirth | 9 | heal 2: up to 2 chosen removed cards to hand | 6U + 0.5 − 1.8U = +4.7U | **34** |

- **Edges:** Reclaim → Salvage, Reclaim → Rescue, Reclaim → Restock, Salvage and Rescue → Revive, and Revive → Rebirth.
- † **Reclaim:** 13 XP set by the user (the rule gives 12.5).
- **In an expedition**, healing in battle doesn't heal map wounds (wounds never shrink in battle).

### Blood Price (sacrifice)

| Card | Change | Energy | Effect value | Net | XP (old → new) |
|---|---|---|---|---|---|
| Blood Price | base: sacrifice 1 from hand, draw 4 or +20 energy | 0 | 4U − 3U | +1.00U | 10 → **15** |
| Deep Cut | draw 5 or +25 energy | 0 | 5U − 3U | +2.00U | 14 → **20** |
| Blood Oath | any time, Offense or Defense | 0 | 4U − 3U, any time +2 XP flat | +1.00U | 13 → **17** |
| Covenant | draw 5 or +25 energy, any time | 0 | 5U − 3U, any time +2 XP flat | +2.00U | 18 → **22** |
| Old Wounds | sacrifice from discard | 0 | 4U − 2U (no hand card lost) | +2.00U | 15 → **20** |
| Martyr | removes itself instead | 0 | 4U − 2U | +2.00U | 14 → **20** |
| Desperation | sacrifice 2, draw 7 or +40 energy | 0 | best option 40 energy = 8U, − 6U | +2.00U | 18 → **20** |

- **Reviewed 2026-10-17.** Nick (draw 3 or +15 energy, net 0) was removed. The any-time switch is +2 XP flat (user's call), like the flat steps in Steady Guard.
- **Why a sacrificed hand card costs 3U but playing a card costs nothing extra:** a card in hand is worth about 1U because of the effect it delivers when played. Playing a card collects that effect, so only its energy is charged. A sacrificed card never delivers its effect and loses its health: 1U + 2U.
- **Edges:** Blood Price → Deep Cut, Blood Oath, Old Wounds and Martyr; Deep Cut and Blood Oath → Covenant; Deep Cut → Desperation.

### Salve and Dispel (status removal)

Reviewed 2026-10-17. Each tree is a 2 × 4 grid. The rows are sizes on the prevent ladder: one stack is priced like 1 damage (2U), clearing a status like 3 stacks and clearing up to 2 statuses like 4. The columns are two timings, and the better timing adds a flat +2 XP, like Blood Price's any-time switch.

| Size | Energy | Salve: before your offensive roll and before your defense roll | Salve: any time in Offense and Defense | Dispel: after your defense roll | Dispel: any time in Offense and Defense |
|---|---|---|---|---|---|
| remove 1 stack | 5 | **Salve 15** | Remedy 17 | **Dispel 15** | Unravel 17 |
| remove 2 stacks of one status | 7 | Double Dose 23 | Tonic 25 | Strip 23 | Peel Away 25 |
| clear one status | 9 | Purge 31 | Sweat It Out 33 | Shatter 31 | Sunder 33 |
| clear up to 2 statuses | 11 | Clean Slate 39 | Panacea 41 | Nullify 39 | Unmake 41 |

- **Edges, both trees:** each card upgrades one step to the right (better timing) and one step down (bigger size); the right column only goes down.
- **Why the timings:** before your defense roll, the incoming attack's statuses are still pending, so a Salve then cleanses only what is already on you; any time also covers after the defense roll, when the defended attack's statuses have landed but not yet ticked at Ongoing Effects. Enemies use their buffs before they attack or defend, so Dispel's weaker column is the narrow Defense, after your roll.
- **Map conditions come back each round.** Fortified, Conqueror, Lieutenant's Favor and the Lieutenant marker are `restore_each_round` statuses: at every round's Ongoing Effects they return to their battle-start stacks, so a removal lasts for the rest of that round. Lieutenant's Favor adds its damage when the Warlord's attack is finalized, so dispelling it in Defense does not change that attack.
- **No status is immune to removal** today; the `dispel_immune` flag stays available for later.
- **Removed (2026-10-17):** Crude Remedy and Labored Rite (the base column covers them), and Poultice, Disenchant and the old Panacea and Nullify ("1 or 2 stacks from up to 2 statuses"); the names Panacea and Nullify moved to the clear-2 cards.
- **Siege Dispel** (added 2026-10-18), 9 energy, **30 XP**, an upgrade from Shatter: after your defense roll, clear one enemy's Fortified or Conqueror **for the rest of the battle**. The status doesn't come back at Ongoing Effects (the `for_battle` switch on Remove statuses). It can't help this round's defense, so it pays off from the next round on: narrower than Shatter, but lasting. The price is a placeholder (about Shatter's) until battles on the map are measured.

### The map trees (reviewed 2026-10-17)

Five trees built from one list of map ideas. Map-only cards are blank in battle and still count as health. "(status)" marks an effect that lasts past the play: it shows as a visible map status (Carried Move, Hidden, Temporary Bandage) or as a mark, and map-only statuses never enter a battle.

**Pathfinder (moving and exploring).** Move 4 for 5 energy at every node; upgrades are steps from the base.

| Card | Energy | Effect | Value | XP |
|---|---|---|---|---|
| **Trailblaze** | 5 | Move 4 | 2U − 1U | **15** |
| Lantern Road | 5 | Move 4; Day costs at Night | base + flat 1 (only forest is cheaper by Day) | **16** |
| Know the Land | 5 | Move 4; name a terrain, it costs 1 this turn | base + about 0.5U | **18** |
| Steady Pace | 5 | Move 4; leftover Move carries to your next turn this round (status) | base + about 0.5U | **18** |
| Trail Sense | 5 | Move 4; exploring costs 1 less this turn | base + 0.5U | **18** |
| Forced March | 5 | Move 4; every hex costs 1 | base + about 1U | **20** |
| Ford the Way | 5 | Move 4; enter one lake or mountain hex for 2 Move; a turn that ends there goes back to the hex you came from (no Move, no fight) | base + about 1U | **20** |
| Far Explore | 5 | Move 4; explore an empty slot up to 4 hexes away that touches a placed tile | Trail Sense + about 0.5U | **20** |
| Slip Away (shared with Unseen) | 8 | Move 4 + Sneak | 2U + 2U − 1.6U | **22** |

- Paths: Trailblaze → Lantern Road → Know the Land → Forced March and Ford the Way; Trailblaze → Steady Pace; Trailblaze → Trail Sense → Far Explore; Forced March → Slip Away.

**Lookout (seeing and hunting).** All free: they give no Move.

| Card | Effect | XP |
|---|---|---|
| **Lookout** | +1 sight this turn | **15** |
| Scout Ahead | see everything on revealed hexes within 3 this turn | **17** |
| Read the Enemy | +1 sight; every enemy on the tiles you can see into is revealed, with its XP | **18** |
| Hunter's Mark | mark the Warlord (in sight) or any Lieutenant (even unseen): you see where it is, not the map around it, for 3 rests | **23** |
| Lure | bait the defenders of an adjacent keep, tower or occupied site out: they are not Fortified (your action) | **23** |
| Run Him Down | with an enemy marked, the quest gains 1 round. Once per quest | **30** |

- Paths: Lookout → Scout Ahead → Hunter's Mark → Run Him Down; Lookout → Read the Enemy → Lure.

**Unseen (stealth and ambush).** All free except the shared Slip Away.

| Card | Effect | XP |
|---|---|---|
| **Hide** | until the end of your next turn enemies see you 1 hex less (status), so you can start that turn unseen | **15** |
| Fall Back | play it when you provoke a fight: step back to the hex you came from instead | **18** |
| Sneak | no provoked fights this turn | **20** |
| High Ground | your next fight this turn counts as from cover (an Ambush if the habit allows) | **20** |
| Sharper Ambush | High Ground, and that Ambush rolls 5+5 | **28** |
| Slip Away (shared) | see Pathfinder | **22** |

- Paths: Hide → Fall Back → Sneak → Slip Away; Hide → High Ground → Sharper Ambush.

**Wayfarer (sites and camp).** Free, priced against the card's own 1 Influence (see §5).

| Card | Effect | XP |
|---|---|---|
| **Silver Tongue** | counts as 2 Influence at a site or camp | **15** |
| Haggle | at this site or camp, every Influence payment this visit costs 1 less, never below 1 | **18** |
| Field Medic | at camp: +1 to every basic-aid roll this camp | **20** |
| Spare Bandage | gain a temporary bandage (status): used at camp like a bandage, gone when that camp breaks | **30** |

- Paths: Silver Tongue → Haggle; Silver Tongue → Field Medic → Spare Bandage. Rest on the Road was dropped.
- † **Haggle's floor of 1** keeps monastery mending (1 Influence) from becoming free and unlimited.

**Foresight (look at your deck, then act).** Works in battle (Offense, before your first roll, unless noted) and on the map.

| Card | Energy | Effect | XP |
|---|---|---|---|
| **Read Ahead** | 5 | look at the top 3, draw 1, put the rest back in any order | **15** |
| Read the Road | 0 | look at the top 3, put them back in any order | **12** |
| Quick Read | 5 | Read Ahead, any time in Offense or Defense | **17** |
| Read Further | 5 | look at the top 5, draw 1, the rest back in any order | **18** |
| Clear the Path | 5 | look at the top 3, draw 1, any of the rest to the bottom, the others back in any order | **18** |
| Pick the Way | 0 | Read Ahead for free | **20** |
| True Path | 0 | look at the top 5, draw 1, any to the bottom, the others in any order | **25** |

- Paths: Read Ahead → Read the Road, Quick Read, Read Further, Clear the Path and Pick the Way; Read Further, Clear the Path and Pick the Way → True Path.
- **In battle the draw pile is shuffled for every draw**, so looking makes the top known: the looked-at cards are taken at random, as draws would be, and become the known top in the order you choose. Draws take the known top first and the known bottom last.

## 8. Open questions and cards to redesign

**Open, kept at placeholder values:**
- **Status stacks (Salve, Dispel).** One stack is assumed to be worth about 1 damage (2U). Measure each status's stacks in damage once Venom, Curse and status minions are in play.
- **Parked map ideas (2026-10-17):** Look ahead (see the next tile before placing it) and Choose the tile (draw 2 tiles, place 1). For another day.
- **The XP economy around the cards** was scaled by 1.5× to match (2026-10-15): starting XP and campaign `bonus_xp` 150, campaign encounters 30 / 38 / 45, expedition groups, events and the early-strike reward ×1.5, and Guard → Guard+ 38. (The gold → XP trainer was removed later; gold never buys XP.) `default_card_price` stays 10, the blank-card price. Existing saves keep their stored budgets.

**Cards that the new numbers broke:** none left. The 2026-10-16 tree review removed or redesigned every one (see each tree's section).

## 9. Pricing a new card

1. Write what it pays and what it gets, in units (sections 1 and 3).
2. If it belongs to a tree, start from the base's net and add the differences (section 4).
3. XP = 5 × (2 + net), half points up, floor 10. If another card in the tree nets the same, add +1 XP for each 5 energy this one costs less (section 3).
4. A base card should net about +1U (15 XP). One step up adds about +1U (+5 XP).
5. If the card nets 0 or less, redesign it rather than price it below 10.
6. **A downgrade keeps the cost to play and gives less**, and costs less XP. 1 energy is worth 1 XP, so Shallow Breath (pay 10, gain 12) is 12 XP. Making a card cheaper or free to play is an upgrade, not a downgrade.
7. Then playtest it.
