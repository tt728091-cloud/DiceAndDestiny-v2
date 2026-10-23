# Balance lab

The balance lab plays real authority battles between a hero and an opponent with automated players and reports win rates. Its players are not trained models: they decide with the battle's own rules and content, so a changed card needs no retraining.

Code: `dice-and-destiny-server/internal/battle/lookahead/` and the `cmd/balance-lab` command.

## Players

| Player | How it decides |
|---|---|
| `random` | Any listed move that changes something. |
| `proxy` | The scripted "competent human" from the expedition tests. It rolls toward Swords and never plays offensive cards. |
| `greedy` | Rules-aware heuristic (level 0), version 2. Offensive dice follow an exact dice solver that uses the engine's own qualification rules and also plans the dice cards in hand (Nudge, Try Again) with the energy to pay for them. A dice card is played only when that beats rerolling or stopping with the card kept. Every card play is priced at 0.1 per energy point plus 0.25 for the card. Other cards are played only when a one-step check says they gain more than their price. Defense rolls the defensive ability first, then takes any free protection offered, then a prevention card. |
| `lookahead` | Look-ahead search (level 1, flat Monte Carlo) on top of `greedy`, version 2. For each decision it plays each candidate move to the end many times on fair copies of the battle (see Fairness): its own later moves by `greedy`, the opponent's by its script or by `greedy`. It overrides `greedy` only when the simulations show a clear gain (paired t ≥ 2.5), and then only if the override wins again on fresh futures it has never seen. |
| `greedy-v1`, `lookahead-v1` | The first versions, kept for comparison. Their dice solver ignored dice cards in hand, so they judged "Nudge now" against the dice as if no Nudge existed and played dice cards too early. `lookahead-v1` also overrode on its selection samples alone. |

Battles run through `mlsim.Environment`, the same authority path game sessions use. `TestLookaheadRunnerMatchesSession` checks that the lab reproduces a session's results seed for seed.

## Running it

From `dice-and-destiny-server`:

```bash
go run ./cmd/balance-lab -hero starter -opponent drowned_oracle_brine_mask \
  -players random,proxy,greedy,lookahead -battles 400 -budget 160 -out /tmp/results.json
```

- `-budget` is the number of simulated futures per decision.
- `-think-ms N` replaces the budget with a hard time limit per decision, for comparing players at equal cost. `-search-workers N` plays each decision's futures on N goroutines (in-game: CPUs − 1); `-workers` is the number of battles at once, so keep `-workers` × `-search-workers` within the machine.
- `-rounds N` stops each future after N rounds and scores the health race; it is about twice as fast.
- A player can override these flags for itself: `-players lookahead@rounds=0,lookahead@rounds=1@think-ms=50`. All listed players run interleaved, seed by seed, so time-limited players share the machine's load evenly.
- `-trace` prints every look-ahead override.
- `-opponent starter -foe greedy` runs a hero opponent played by `greedy` (or `greedy-v1`).
- `-seats both` plays every seed from each seat.
- `-card-value N` sets what holding a card is worth to the version-2 players.
- The rider rates (`lookahead.Rates`) are `-status-value` (a status an ability gives its user, per stack; default 0.5), `-protect-value` (Protect only; default the status value), `-energy-gain-value` and `-energy-spend-value` (default 0.1 each). `-energy-value N` sets both energy rates.
- `-bias ability=N` makes the greedy player favour an ability.
- Every player plays the same seeds, so results can be compared seed by seed: `-compare a.json[:player],b.json[:player]` pairs two players' battles by seed and seat and prints the mean score difference with its 95% interval, z and each side's win/draw/loss.
- `-record dir` saves value-network training positions from every battle (see below).

The command reads the published cards and never writes outside `-out` and `-record`.

## Fairness

- **Randomness:** each copy reseeds the dice, draws and damage picks.
- **Opponent's hand:** in each copy it is re-dealt from their hand plus draw pile. Only cards the battle has already shown stay in place, such as cards revealed for removal or being played.
- **Planning order:** offensive planning is private and simultaneous. The look-ahead player (and the lab's hero) always plans first, so a copy never holds the other side's private dice or choice.
- **What the human sees:** cards played during planning are public by the game's rules (`TestPublicPlanningCardPublishesResourcesWithoutHiddenChoices`). The human therefore sees the AI's planning cards and energy before planning; dice and the chosen attack stay hidden.

## Play it yourself

In the game menu, pick your character and the opponent **Look-ahead AI · mirror of your character and deck**. The AI plays your character with your exact deck and ability board. Alternate Seat A and Seat B.

Score your games from `dice-and-destiny-server`:

```bash
go run ./cmd/balance-lab -tally default
```

Over 20 games, 15 or more wins (draws count half) means you clearly outplay it (p ≈ 0.04). About 10 means it plays at your level.

The tally splits games by AI version. The game's opponent is version 3 from 2026-10-22 (below); logged games before that were version 2 (from 2026-10-15) or version 1.

Review your games move by move:

```bash
go run ./cmd/battle-review
```

This replays every logged mirror game through the engine. It writes an HTML page (under the workspace's ignored `battle-reviews/` folder) with:

- each roll laid out as the dice held, then the result under every rerolled or changed die;
- the AI's hidden dice and hand;
- each AI dice decision checked against the card-aware solver;
- every override, compared with the heuristic of the AI version that played.

## Results (2026-10-15)

All runs use unified defense on an M3 Max.

### Starter vs Brine Mask

Seeds 1–400, hero plans first.

| Player | Win | Draw | Loss | Win % (95% CI) | Time per decision |
|---|---|---|---|---|---|
| random | 0 | 0 | 400 | 0% | <0.1 ms |
| proxy | 267 | 55 | 78 | 66.8% (62.0–71.2) | <0.1 ms |
| greedy | 365 | 18 | 17 | 91.2% (88.1–93.6) | 0.3 ms |
| lookahead | 370 | 13 | 17 | 92.5% (89.5–94.7) | 0.95 s |

- **lookahead vs greedy:** lookahead was better on 16 seeds and worse on 12. That is not significant.
- **An earlier run let the minion plan first.** The look-ahead could then see its private plan and appeared significantly better (14 vs 4 seeds). That run is discarded.

### Starter mirror

Seeds 1–200, opponent played by greedy.

| Hero player | Win % | Draw % | Loss % | Score |
|---|---|---|---|---|
| greedy | 36.0 | 19.5 | 44.5 | 0.458 |
| lookahead | 49.0 | 13.5 | 37.5 | 0.557 |
| proxy | 6.0 | 9.5 | 84.5 | 0.107 |

### Measured Strike experiment

`-bias measured_strike=N` makes the greedy player favour Measured Strike (1,000 battles each).

| Bias | vs Brine Mask: picks per battle | Win % | Mirror: picks per battle | Score |
|---|---|---|---|---|
| 0 | 0.03 | 91.9 | 0.03 | 0.476 |
| +1 | 0.14 | 91.6 | | |
| +2 | 0.73 | 87.6 | 0.94 | 0.369 |
| +3 | 1.29 | 86.4 | | |
| +5 | 2.26 | 77.2 | | |

Choosing it more often lowers results in both matchups, for a player that spends energy the way `greedy` does. A player who spends the 5 energy better could value it more.

### Version 2 (dice cards in the solver, fresh-futures check)

A reviewed game showed the first version playing Nudge right after its first roll (2-6-6-4-5, two rerolls, two Nudges in hand).

| First move | Value | Chance of Large Straight | Nudges used |
|---|---|---|---|
| Reroll one 6 and keep the Nudges | 7.42 | 94% | 0.94 |
| Nudge the 2 to a 3 first (what v1 did) | 6.75 | 75% | 1.50 |

These values price a Nudge at 5 energy × 0.1. The Go solver and an independent exact calculation agree to the decimal (`TestDiceSolverKeepsNudgesForLater`).

Card value sweep for the version-2 heuristic (1,000 seeds vs Brine Mask; 2,000 mirror games vs `greedy-v1`):

| Card value | vs Brine Mask | Mirror score vs `greedy-v1` |
|---|---|---|
| — (`greedy-v1` itself) | 91.9% | 0.490 |
| 0 | 94.3% | 0.586 |
| 0.25 (default) | 94.6% | 0.579 |
| 0.5 | 94.7% | 0.569 |
| 1.0 | 89.6% | 0.425 (stops playing Strong Swing) |

Look-ahead comparison in the Starter mirror against `greedy-v1`: seeds 1–400 from each seat, the same 800 games for each player.

| Player | Score | Win / draw / loss |
|---|---|---|
| `lookahead-v1` | 0.580 | 50.9 / 14.3 / 34.9% |
| `greedy` (v2) | 0.589 | 50.5 / 16.8 / 32.8% |
| `lookahead` (v2) | **0.627** | 54.4 / 16.8 / 28.9% |

Paired on the same games:

- **v2 vs v1 look-ahead:** v2 gains +0.048 ± 0.039 (z = 2.4).
- **v2 look-ahead vs v2 heuristic:** the look-ahead gains +0.039 ± 0.022 (z = 3.5).
- **Brine Mask (seeds 1–400):** v2 look-ahead 92.8% vs v1 92.5%. No measurable change: that matchup is mostly dice.

With the fresh-futures check, the look-ahead overrides on about 1.3% of decisions (v1: 3.9%). Its slowest in-game decision fell from 1.6 s to 0.6 s.

### Other findings

- **The proxy understates the Starter.** It never plays a card. Earlier probes that used it measured the proxy, not the deck.
- **Most decisions are close.** For a typical decision, sensible options differ by under 4 points of win chance.
- **The overrides point at heuristic gaps:** Take Stock, Guarded Strike over Small Straight, and Try Again instead of a normal reroll.

### Character names no longer change play (fixed 2026-10-22)

The players misplayed Nudge and Try Again for any character whose display name sorts before "Play":

- **Why:** the authority sorts legal commands by their text, and a dice card lists a plain start ("Play Nudge") beside one start per die ("Adventurer · die 1: 2 → 1"). The greedy player valued every start of the card at its best use and took the first listed. For the Adventurer that was die 1, whatever it showed.
- **Who it hit:** the Adventurer, and the Board Builder's custom adventurer. Curse and Blade Warden were unaffected: their decks hold no die-targeting cards.
- **The fix:**
  - `Useful` drops a die-naming start when the card also has a plain start. The plain start then picks the die by the solver, as it always did for the Starter.
  - A die-naming start without a plain start is valued by its own die.
  - The search scores die-naming starts with their own card.
- **Test:** `TestRenamingAHeroChangesNoBattle` checks that a copy of the Starter named "Aardvark" plays every battle exactly as the Starter does.

Greedy, seeds 1–1,000 from both seats:

| Hero | vs Drowned Regent (before → after) | vs Brine Mask (before → after) | vs Starter, opponent greedy (before → after) |
|---|---|---|---|
| Starter | 0.501 → 0.501 | 0.922 → 0.922 | 0.495 → 0.495 |
| Adventurer | 0.390 → **0.501** | 0.869 → **0.922** | 0.370 → **0.495** |

The Starter's battles are unchanged, battle for battle. The greedy Adventurer now plays them exactly like the Starter: it never picks its one extra option, Guard+.

The look-ahead (`-budget 48`) was checked on seeds 1–30 from both seats against the Starter with the greedy player:
- The Starter's battles are unchanged.
- A copy of the Starter named "Aardvark" scored 0.342 before the fix. It now plays every battle exactly like the Starter (0.500).
- The Adventurer went from 0.325 to 0.508. It differs from the Starter in 4 of the 60 battles because the look-ahead also searches Guard+.

Earlier lab numbers for the Adventurer came from this bug, including the board lab's finding that every hero scores about 0.36 against the Starter.

## Card balance (Starter pool, 2026-10-15)

`cmd/card-balance` adds one copy of a card to the Starter's 12-card deck and plays 3,000 seeds against each opponent:

- the mirror against the unchanged deck, from both seats;
- the Warlord (Drowned Regent);
- Brine Mask, Bell Diver and Ribbon Eel.

Every card is played on the same seeds, by the greedy player calibrated for this run (draw value 0.75, energy value 0.05). That setting scores 0.599 against the default heuristic.

Each card's measured value is converted to a **fair XP price** at the base cards' average rate (all base cards cost 10 XP), separately for the mirror (score), the Warlord (score) and the small minions (health left, since wounds carry over). Fair prices are good to about ±1.5 XP.

```bash
go run ./cmd/card-balance -battles 3000 -out results.json -html report.html   # ~1.5 h on 14 cores
go run ./cmd/card-balance -from results.json -html report.html                # re-render only
```

### What stands out

- **Overpriced** (fair price well below cost):
  - Hoard (17 → ~10)
  - Bulwark (20 → ~15)
  - Brace (19 → ~14)
  - Clean Cut (18 → ~14)
  - Stalwart (17 → ~13)
  - Plenty (13 → ~9.5)
  - Shelter (14 → ~10.6)
  - Far Reach (16 → ~13.7)
  - Try Again, the weakest base card (10 → ~7.8)
- **Best value:**
  - Strong Swing and Nudge (10 → ~11);
  - the 8–9 XP drawback variants Lift, Press Down, Haymaker and Clumsy Touch (fair ~10).
- **Opponent decides defense against offense.** Prevention is worth far more against minions than in the mirror, and offense the reverse:

  | Card | Fair XP in the mirror | Fair XP against minions |
  |---|---|---|
  | Quick Block | 11.9 | 18.4 |
  | Steady Guard | 7.9 | 12.1 |
  | Strong Swing | 13.0 | 9.7 |

  Expedition fights are against minions and the Warlord, which favours defense.
- **What each toggle is worth:**

  | Toggle | Worth |
  |---|---|
  | 5 energy | 2.5–4 XP |
  | +1 prevention | 2–3 XP |
  | +1 Strong Swing damage | ~2 XP |
  | +1 card drawn | ~1 XP |
  | Second Wind's 15 energy instead of 10 (Surge) | ~0.4 XP |
  | "Saved cards return to their pile" | ~1 XP |
  | Playing the card after the roll instead of before | ~0 XP |
  | One card of health alone | ~7 XP |

- **Energy below 5 is wasted.** Take Stock at 1, 2, 3 or 4 energy measured the same as at 5 (fair XP 8.7 vs 8.6, 3,000 seeds × 5 opponents). Only 0 energy changes it (Quick Study, 12.6). Income is 5 per round and almost every card costs 0, 5 or 10, so a 1–4 cost still uses up a whole 5-energy play. The savings only matter once they add up to 5, which a 4-round battle rarely reaches. Energy costs work as a lever only in steps of 5, unless income or other costs stop being multiples of 5.
- **Not measured yet**, because the AI does not play these effects properly:
  - Two Fingers, Long Fingers, Double Toss;
  - Reckless Lunge, Finisher, Sweeping Arc, Honed Edge;
  - Lucky, Quick and Charmed Toss, Fresh Roll, Scrap Toss;
  - the set-a-die, copy and flip cards;
  - Focus and Deep Focus, Blood Price, Reclaim;
  - Parry, Disrupt, Dispel, Salve;
  - Reinforce, Triage, Field Dressing.

  Overthink measured worse than a blank card. The AI plays it when holding it would be better, so that result is not a verdict on the card.

## Cards on top of the board (2026-10-22)

How much each hero's deck adds to its board. The hero plays its mirror twice on the same seeds, from both seats:
- **with its deck**;
- **with no cards** (`-block-cards=all`): it never plays a card, so its cards only count as health, like blank cards. Its dice solver also stops planning around the dice cards it holds, and its look-ahead's simulated moves don't play them either.

Setup:
- **Hero:** the in-game opponent (version 3: `lookahead@rounds=1@budget=400`).
- **Opponent:** greedy, with its full deck.
- **Size:** 100 seeds, both seats, so 200 battles per side. Differences are paired battle by battle.
- **Damage:** the lab now records each side's damage by source from the authority's event stream: before defense, prevented by the defense roll, prevented by reactions (cards and Protect), and landed. A turn counts once it resolves: an attack, or a miss followed by another turn. A turn cut short by the battle's end doesn't count.
- **Errors:** no battle hit an engine error.

| Hero | Win score: deck / no cards | Attack before defense per turn | Other damage per turn (statuses, retaliation) | Damage landed per turn | Damage taken per enemy turn | Net health per round, cards add |
|---|---|---|---|---|---|---|
| Starter | 0.64 / 0.23 | 7.03 / 5.87 (**+1.16**) | 0 / 0 | 3.55 / 2.35 (+1.20) | 2.86 / 3.39 (−0.53) | **+1.74** ±0.30 |
| Curse | 0.88 / 0.57 | 4.58 / 4.40 (+0.18) | 1.31 / 1.10 | 3.73 / 3.39 (+0.34) | 2.95 / 3.21 (−0.26) | **+0.62** ±0.11 |
| Venom | 0.81 / 0.55 | 4.20 / 3.99 (+0.21) | 2.59 / 2.19 | 4.58 / 4.05 (+0.52) | 3.43 / 3.89 (−0.46) | **+0.84** ±0.21 |
| Blade Warden | 0.97 / 0.62 | 6.44 / 6.47 (−0.03) | 1.17 / 0.41 | 4.35 / 3.52 (+0.84) | 2.37 / 3.14 (−0.77) | **+1.44** ±0.22 |

The Curse, Venom and Blade Warden rows come from the first build, which counted every turn that rolled. That build's fixes (turn counting and the solver) change none of their battles: the greedy no-card runs replay identically, and their decks hold no card the solver plans with. Under 1% of their turns were cut short. A first Starter run without the solver fix is kept in `ml/runs/cards/before-solver-fix`.

- **The board estimate holds in battle.** With no cards, the Starter's board deals 5.87 per turn before defense with the search and 5.93 with greedy, and misses 1.5% of its turns (greedy). The board calculator, played with the AI's own values (Protect 0.5 a stack, energy 0.1 a point), gives 5.86 per turn and 1.4% misses, with the same mix of abilities within a few points. Run it as `board_balance.py --costs "A3/A4/A5, A2+B3, A2+C2, SS, LS, C4" --printed "4/5/6, 6, 4.5, 6, 8, 10"`.
  - **A first build skipped that fix.** Kept from its cards, the player still planned its keeps around the Nudges and Try Agains in hand. It missed 5% of its turns and averaged 5.74.
- **Every deck is worth about 0.3–0.4 win score in its mirror** (+0.27 to +0.41).
- **Only the Starter's cards add attack damage.**
  - **Starter:** +1.16 per turn before defense (+20%), and all of it gets through the defense: +1.20 landed (+51%). The defense takes a flat ~3.4 either way, as the board lab's pure duel found for boards. Large Straights rise from 17% of attacks without cards to 40% with them, and Nudge alone makes the difference: blocking only Nudge drops them to 19% (table below).
  - **The other decks** leave the attack nearly unchanged (+0.18, +0.21, −0.03). They add statuses instead: poison for Venom, bleed for Blade Warden (Sharpen Blade). They also add defense:
    - prevention per enemy attack: Emergency Ward 1.04; Venom's cards 0.43 (Antivenom Draught, Coagulate, Emergency Molt); Curse's 0.14 (Spiteful Ward);
    - smaller incoming attacks before defense, by 0.37–0.51 per enemy turn. This is probably the cards that act on the opponent: Curse's cursed dice, Antivenom Draught removing poison stacks, Tip It. The Starter's opponent also attacks 0.28 weaker against the deck, likely because more damage removes more of its own cards (cards are health), leaving it fewer Strong Swings and Nudges.
- **Only the search plays the Curse, Venom and Blade Warden cards.** Greedy plays none of them (its deck and no-card battles are identical), so in those three mirrors the greedy opponent never plays its own cards either. In the Starter mirror it plays its dice cards but never Steady Guard or Take Stock. Greedy's Starter numbers:
  - win score 0.53 with the deck, 0.23 without;
  - attack before defense 7.01 against 5.93 per turn;
  - damage taken barely moves (−0.04 per enemy turn).

**The Starter's cards one at a time.** The full deck against the same deck with one card type blocked, on the same seeds (both seats, 200 battles each). "Health per play" is the card's swing in health per round, dealt plus saved, times rounds per battle, divided by its plays per battle. It is net of the energy the card uses, since a blocked card's energy goes to the other cards. The card rules' effect is from `card-value-rules.md`: 1 health = 2 units.

| Card | Plays per battle | Win score | Attack before defense per turn | Damage taken per enemy turn | Health per play (95%) | Card rules' effect, in health |
|---|---|---|---|---|---|---|
| Nudge | 0.89 | +0.12 ±0.07 | +0.47 | −0.07 | **+2.6** ±0.9 | 1.2 (2.4U) |
| Take Stock | 0.24 | +0.04 ±0.03 | +0.10 | −0.02 | +3.0 ±1.9 | 1.0 (draw 2) |
| Second Wind | 0.49 | +0.04 ±0.04 | +0.16 | −0.05 | +2.0 ±1.2 | 1.5 (15 energy) |
| Strong Swing | 0.75 | +0.02 ±0.06 | +0.33 | +0.12 | +1.3 ±0.9 | about 1.5 (+2 damage, 1.24 lands) |
| Steady Guard | 1.30 | +0.07 ±0.06 | −0.01 | −0.37 | +1.2 ±0.55 | 1.0 (prevent 1) |
| Try Again | 0.48 | +0.03 ±0.05 | +0.15 | −0.02 | +1.1 ±1.4 | 0.35 (0.7U) |

- **Steady Guard and Strong Swing play at about their printed effect.**
- **Nudge plays at about twice its printed effect.** It is the only single card that clearly moves the win score: it turns near-straights into Large Straights (8 damage) and saves misses.
- **Take Stock, Second Wind and Try Again:** positive, but too noisy here to price. Take Stock is played only 0.24 times a battle.
- **The cards add up:** blocked one at a time, they sum to +1.85 health per round, against +1.74 for the whole deck.
- **Every card measures above the rules' net:** each base card is meant to net +1U (half a health) after its energy, and every card here measures more. This is one mirror (12 health, under 4 rounds), with the search playing both its own cards and the greedy opponent's.
- **These are values in the mirror.** The 2026-10-15 card balance run found prevention worth more against minions and offense less.

```bash
go build -o ml/runs/bin/balance-lab-cards ./cmd/balance-lab
S=lookahead@rounds=1@budget=400
LAB=ml/runs/bin/balance-lab-cards
$LAB -hero starter -opponent starter -foe greedy -players "$S,$S@block-cards=all,greedy,greedy@block-cards=all" -seats both -battles 100 -workers 15 -out cards/starter.json
# the same for curse, venom and blade_warden; then one card at a time (each against starter.json):
$LAB -hero starter -opponent starter -foe greedy -players $S@block-cards=nudge -seats both -battles 100 -workers 15 -out cards/each-nudge.json
python3 ml/scripts/card_value_report.py cards   # the paired tables above
```

## Cards against blank cards (2026-10-23)

**How many blank cards is each card worth?** A blank card is pure health: Trailblaze, a map card that can't be played in battle but still counts as health.

**Method:**
- Each variant replaces every copy of one card in the Starter's deck with *m* blanks.
- The card is worth as many blanks as it takes to score what the standard deck scores: the crossing, from a line through the variants nearest it.
- Per copy, that's the crossing divided by the copies.
- The card rules price a card's body (1 health, 2 units) at 10 XP, so a blank is 10 XP and a card worth *r* blanks is worth 10·*r* XP. The current base price, 15 XP, says every base card is worth 1.5 blanks.

**Setup:**
- **Matchup:** the Starter mirror; the opponent is greedy with the standard deck.
- **Hero:** greedy, with 2,000–4,000 battles per variant; or the in-game AI (version 3), with 600–800.
- **Seeds:** the same for every variant, from both seats.
- **Tool:** `balance-lab -hero-deck` sets the hero's deck.

**Check:** greedy never plays Steady Guard or Take Stock, and both measure 0.99 blanks per copy. A card that's never played is exactly a blank, as it should be.

**Blanks per copy** (95% intervals: greedy about ±0.1–0.15, the in-game AI as shown; "—" not measured):

| Card | XP now | Starter mirror, in-game AI | Starter mirror, greedy | Fair 20-card mirror, greedy | vs the Warlord, greedy | vs Brine Mask (health left), greedy | Suggested XP |
|---|---|---|---|---|---|---|---|
| Nudge | 15 | **1.68** ±0.33 | 1.79 | 1.86 | 1.60 | 1.27 | **17** |
| Strong Swing | 15 | — | 1.61 | 1.65 | 1.43 | 1.20 | **15–16** |
| Steady Guard | 15 | **1.31** ±0.18 | never played | — | — | — | **13** |
| Second Wind | 15 | — | 1.28 | 1.09 | 1.19 | 1.00 | **12–13** |
| Try Again | 13 | — | 1.22 | 1.34 | 1.19 | 1.04 | **12** |
| Take Stock | 15 | **1.00** ±0.40 | never played | — | — | — | **10** |

- **The ratios.** In the Starter mirror, a Nudge is worth about 1.7–1.8 blank cards: two Nudges ≈ 3½ blanks, and five Nudges ≈ 9 blanks. Three blanks are worth almost two Nudges, not one.
- **Suggested XP** is 10 per blank, taken from the close fights: the mirror, with the in-game AI where it was measured, and the Warlord.
  - **The rules' base price (15 XP = 1.5 blanks)** is about right on average, but it spreads the cards wrong. Nudge is underpriced. Steady Guard and Second Wind are a little overpriced, and Take Stock is overpriced by a third: the in-game AI plays it only about once every four battles, and drawing 2 for 5 energy adds nothing beyond the card's own health here.
  - **Against weak minions every effect is worth less** (Brine Mask: 1.0–1.3), because the fight is won either way and only health saved counts. Pricing for the Expedition, which is mostly minion fights, would push every effect toward the price of a blank.
- **Whole decks, in blank cards:**
  - **Starter deck:** its 12 cards play like **15½ blank cards** with greedy (a 15-blank deck scores 0.468, 16 blanks 0.542, the Starter deck 0.497). With the in-game AI, about **16**.
  - **All-blank decks:** 10 blanks score 0.15 and 20 blanks 0.77, about 0.065 per blank around 15.

**Not measured:** Strong Swing, Second Wind and Try Again with the in-game AI. For Nudge it agrees with greedy within the noise (1.68 against 1.79), and greedy plays dice cards the same way, so greedy's numbers stand in for those three.

**Theories tested** (greedy unless noted; same seeds throughout):

1. **Every copy is worth the same.**
   - **Fewer copies:** taking out one of the two Nudges costs 1.75 blanks, against 1.79 per copy for both. Strong Swing: 1.60 against 1.61.
   - **Extra copies:** adding a third or fourth Nudge, Strong Swing, Try Again or Second Wind scores within 0.013 of the per-copy prediction, in all eight cases.
2. **Deck size doesn't change the rates.** With both sides on 20 cards (battles of 6.8 rounds instead of 3.7), the rates are the same within the noise: Nudge 1.86, Strong Swing 1.65, Try Again 1.34, Second Wind 1.09. An earlier test that gave only the hero 20 cards inflated them (Nudge 2.08), because the hero won 90% and scores near the top bunch together.
3. **The opponent does change them.** Against the Warlord, an even fight for the Starter (0.517), attack cards are worth about 0.2 blanks less than in the mirror. Against Brine Mask, they're worth little beyond their own health.
4. **Energy matters.** Second Wind (+15 energy for 10) is worth 1.57 blanks per copy in an energy-hungry deck (4 Nudges, 4 Strong Swings) against 1.28 in the standard deck.
5. **Swaps lose a little.** Dropping one card entirely for more copies of another scores 0.02–0.035 below the prediction:
   - 2 Nudges → 3 Try Agains;
   - 2 Strong Swings → 2 more Nudges;
   - 2 Try Agains → 2 more Second Winds.

   Variety, synergy (Nudge sets up the Large Straight that Strong Swing boosts) and energy all play a part. It's half a blank at most.
6. **Same price today, very different strength.** Five 12-card decks that all cost 172–176 XP today score 0.29–0.67. The measured rates predict every one within 0.035:

   | Deck (12 cards) | XP now | Worth in blanks | Predicted | Scored |
   |---|---|---|---|---|
   | 4 Nudge, 4 Strong Swing, 2 Second Wind, 2 Try Again | 176 | 18.6 | 0.70 | 0.67 |
   | 3 Nudge, 3 Strong Swing, 2 Second Wind, 2 Try Again, 1 Steady Guard, 1 Take Stock | 176 | 17.2 | 0.62 | 0.62 |
   | 2 Nudge, 2 Strong Swing, 4 Try Again, 4 Second Wind | 172 | 16.8 | 0.59 | 0.57 |
   | Standard deck | 176 | 15.5 | 0.50 | 0.50 |
   | 4 Try Again, 2 Second Wind, 3 Take Stock, 3 Steady Guard | 172 | 13.4 | 0.35 | 0.32 |
   | 4 Steady Guard, 4 Take Stock, 2 Try Again, 2 Second Wind | 176 | 12.9 | 0.32 | 0.29 |

   Greedy never plays Steady Guard or Take Stock, so the last two rows overstate the gap for a stronger player. The in-game AI check below covers that.
7. **Same measured value, much closer strength.** Decks rebuilt to the standard deck's value at the measured rates score 0.45–0.54:
   - 11 cards with 4 Nudges: 0.450;
   - 14 cards with 4 Try Agains and 3 Second Winds: 0.475;
   - 11 cards with 4 Strong Swings: 0.457;
   - 15 blanks: 0.468; 16 blanks: 0.542.

   The ones that pile up one expensive card come in 0.03–0.065 under, which is theories 4 and 5 again.

8. **The in-game AI confirms it.** It played the two most different same-price decks, 500 battles each, and played Steady Guard 1.9 and Take Stock 0.9 times a battle in the second. The standard deck scores 0.611. For the measured rates:
   - each deck is valued in blanks (16.2 for the standard deck);
   - in-game AI rates where measured, greedy's for the rest;
   - each blank ≈ 0.065 win score.

   | Deck (176 XP now) | Today's prices predict | Measured rates predict | Scored |
   |---|---|---|---|
   | 4 Nudge, 4 Strong Swing, 2 Second Wind, 2 Try Again | 0.61 | 0.74 | **0.681** ±0.036 |
   | 4 Steady Guard, 4 Take Stock, 2 Try Again, 2 Second Wind | 0.61 | 0.48 | **0.427** ±0.039 |

   The measured rates predicted a 0.25 gap between the two, and the gap was 0.25. Both decks pile up cards, and both sit about 0.05 under the prediction, as in theory 5.

```bash
go build -o ml/runs/bin/balance-lab-blanks ./cmd/balance-lab
# one variant: both Nudges replaced by 3 blanks
ml/runs/bin/balance-lab-blanks -hero starter -opponent starter -foe greedy -players greedy \
  -hero-deck steady_guard:3,try_again:2,strong_swing:2,take_stock:2,second_wind:1,trailblaze:3 -seats both -battles 1000 -out nudge-to-3.json
# the sweeps (plans and runner in ml/runs/blanks): python3 sweep.py plan-greedy.json, …
python3 ml/scripts/card_exchange_report.py ml/runs/blanks/greedy                                   # exchange rates
python3 ml/scripts/card_exchange_report.py ml/runs/blanks/greedy-budget --rates=ml/runs/blanks/greedy  # decks priced both ways
python3 ml/scripts/card_exchange_report.py ml/runs/blanks/greedy-brine --metric=health              # minions: health left
```

## AI value network (2026-10-21)

The first pass of [`ai-value-network-plan.md`](ai-value-network-plan.md) (phases A0–A4; brief: [`ai-value-network-handoff.md`](ai-value-network-handoff.md)). All runs are on an M3 Max (16 cores) shared with other work. Results files and binaries live under the server's ignored `ml/runs/`.

### Equal-time benchmark (T1, phase A0)

Players are compared at the same thinking time. `-think-ms` is a hard limit per decision: successive halving gets the first half, a promising option's extra futures run to three quarters and the fresh-futures check to the limit; a future still running at the limit is abandoned, and every option keeps only the futures all of them finished. Without `-think-ms` the search is unchanged (battle for battle identical to before).

**Think time: 100 ms per decision on 7 goroutines, 2 battles at a time.** The logged in-game v2 opponent takes 74.5 ms at the median (p90 248 ms, max 1.2 s) on 15 goroutines with budget 160, which in the lab plays about 163 futures per searched decision (56 ms per decision). 100 ms on 7 goroutines plays 231 futures at Rounds 0 on an otherwise idle machine, so the lab matches the game in both wall time and work. During these runs the board thread's `board-lab` shared the machine, so the players got about half of that (the futures columns). All players run interleaved, seed by seed, so they shared the load equally.

Controls are version-2 look-ahead at Rounds 0 (v2 itself), 1 and 2, all with today's health-race evaluator `tanh(2 × margin)`; greedy is shown for reference. Seeds 1–400 from both seats (800 paired battles per player).

**Starter mirror (opponent greedy):**

| Player | Score | Win / draw / loss | vs v2 (95% CI, z) | Futures per searched decision | ms per decision |
|---|---|---|---|---|---|
| greedy | 0.501 | 323 / 156 / 321 | −0.011 (−0.017 … −0.005, z −3.4) | — | 0.1 |
| v2 (Rounds 0) | 0.512 | 334 / 151 / 315 | — | 125 | 50.9 |
| **Rounds 1** | **0.605** | 413 / 142 / 245 | **+0.093 (+0.064 … +0.122, z +6.3)** | 246 | 47.8 |
| Rounds 2 | 0.522 | 342 / 151 / 307 | +0.010 (−0.001 … +0.021, z +1.8) | 143 | 49.5 |

**Starter vs Brine Mask:**

| Player | Score | Win / draw / loss | vs v2 (95% CI, z) | Futures per searched decision | ms per decision |
|---|---|---|---|---|---|
| greedy | 0.914 | 712 / 39 / 49 | −0.012 (−0.018 … −0.005, z −3.6) | — | 0.1 |
| v2 (Rounds 0) | 0.926 | 726 / 30 / 44 | — | 215 | 50.9 |
| **Rounds 1** | **0.953** | 745 / 34 / 21 | **+0.026 (+0.012 … +0.041, z +3.6)** | 406 | 49.8 |
| Rounds 2 | 0.924 | 724 / 31 / 45 | −0.002 (−0.009 … +0.006, z −0.5) | 245 | 51.0 |

- **The best control is Rounds 1.** At equal time, stopping each future after one round and scoring the health race beats full playouts clearly in both matchups: it plays about twice the futures, and the near-term health race is a better guide than a greedy playout to the end. Rounds 2 is barely different from full playouts, because most battles last only three or four rounds.
- At this budget full playouts gain only +0.011 over greedy; the 2026-10-15 run (1 s per decision) found +0.039.
- ms per decision averages every decision, including the ones decided without search.
- **Other hero mirrors.** Every hero the lab knows runs: Starter, Adventurer, Curse, Venom, Blade Warden and Venom Goblin. Seeds 1–200 from both seats (400 paired battles per player), opponent greedy:

  | Mirror | greedy | v2 (Rounds 0) | Rounds 1 (vs v2, z) | Rounds 2 (vs v2, z) | Futures per searched decision (R0 / R1 / R2) |
  |---|---|---|---|---|---|
  | Venom | 0.489 | 0.706 | **0.823** (+0.116, z +4.4) | 0.805 (+0.099, z +3.9) | 109 / 305 / 156 |
  | Blade Warden | 0.521 | 0.691 | **0.950** (+0.259, z +10.8) | 0.877 (+0.186, z +8.3) | 148 / 425 / 203 |
  | Curse | 0.504 | 0.752 | **0.826** (+0.074, z +3.0) | 0.781 (+0.029, z +1.3) | 105 / 390 / 157 |

  Rounds 1 is the best control in every mirror. The greedy player plays no Venom, Curse, Blade Warden or Venom Goblin cards, and the look-ahead finds them through its candidates, so in these mirrors any look-ahead beats the greedy opponent by a wide margin.

```bash
LAB=ml/runs/bin/balance-lab-6d7b580   # commit 6d7b580
P=greedy,lookahead@rounds=0,lookahead@rounds=1,lookahead@rounds=2
$LAB -hero starter -opponent starter -foe greedy -players $P -think-ms 100 -search-workers 7 -workers 2 -seats both -battles 400 -out t1/starter-mirror.json
$LAB -hero starter -opponent drowned_oracle_brine_mask -foe script -players $P -think-ms 100 -search-workers 7 -workers 2 -seats both -battles 400 -out t1/starter-brine.json
$LAB -compare t1/starter-mirror.json:lookahead@rounds=1,t1/starter-mirror.json:lookahead@rounds=0
# other mirrors: -hero H -opponent H -foe greedy -battles 200 with the same players and flags (H = venom, blade_warden, curse)
```

### Rider rates (T2, phase A0b)

One `Rates` value now feeds the greedy player, its dice solver and the ability tier values. The defaults are the version-2 values (status 0.5 per stack, energy 0.1 gained and spent); with them, greedy and look-ahead play is identical battle for battle to before.

The variants value riders at the board pricing's rates (2026-10-15: Protect 1 per stack, energy 0.2 per point). Each is the greedy player against greedy v2, seeds 1–1,000 from both seats (2,000 paired battles per matchup):

| Variant | Starter mirror: score (paired difference, 95% CI) | Vs Brine Mask: score (paired difference, 95% CI) | Guarded Strike / Measured Strike picks per battle (mirror · Brine Mask) |
|---|---|---|---|
| Today's rates | 0.495 | 0.922 | 0.39 / 0.05 · 0.31 / 0.05 |
| (a) Protect 1, energy gained 0.2 | 0.466 (−0.029, −0.052 … −0.007, z −2.5) | 0.921 (−0.002, −0.014 … +0.010) | 1.34 / 0.09 · 1.30 / 0.08 |
| (b) (a) plus energy spent 0.2 | 0.393 (−0.102, −0.126 … −0.079, z −8.5) | 0.904 (−0.018, −0.031 … −0.006, z −2.9) | 1.46 / 0.09 · 1.47 / 0.09 |

- **Decision: today's rates stay.** Adoption needed the lower 95% bound of the paired difference to be at least −0.01 in both matchups; both variants fail in the mirror, and (b) also against Brine Mask. The in-game opponent is unchanged.
- Valuing Protect at 1 more than triples Guarded Strike (5 damage and 2 Protect) at the expense of the straights, and the greedy player loses more for it, as with Measured Strike before. Raising the energy rate also makes it spend less, so battles run longer (mirror rounds 3.6 → 4.2–4.4).
- A stronger player may use these riders better; the value network measures them instead (Measured values, below).
- In variant (a) "energy gained" is one rate for every gain: ability tiers and energy cards alike.

```bash
LAB=ml/runs/bin/balance-lab-a9228fc   # go build -o $LAB ./cmd/balance-lab at commit a9228fc
P=greedy,greedy@protect-value=1@energy-gain-value=0.2,greedy@protect-value=1@energy-gain-value=0.2@energy-spend-value=0.2
$LAB -hero starter -opponent starter -foe greedy -players $P -seats both -battles 1000 -workers 3 -out t2/starter-mirror.json
$LAB -hero starter -opponent drowned_oracle_brine_mask -foe script -players $P -seats both -battles 1000 -workers 3 -out t2/starter-brine.json
$LAB -compare t2/starter-mirror.json:greedy@protect-value=1@energy-gain-value=0.2,t2/starter-mirror.json:greedy
```

### Value network data (T3, phase A1)

**Encoder (`value-v1`, `internal/battle/valuenet`).** 148 inputs, read only from the seat's viewer-safe snapshot, the battle's pinned catalog and the dice each side holds (public table setup that the snapshot lists only for the viewer). No card, ability, status or character ID is an input:

- each side: health in each pile (as shares of its maximum), maximum health, energy, hand limit, dice;
- each side's board: the dice solver's `Fresh()` value and its chance of ending with no ability qualified (without cards), the offensive abilities it can afford, the best tier's value and the tiers' pooled effect summary (`mlsim.V2OperationSummary`), active card bonuses;
- each side's defenses: how many, and the best one's expected prevention, energy, damage and statuses per roll (every die resolves its own outcome);
- each side's statuses by polarity: count, stacks, fill (stacks ÷ cap), persistent, consumable and expiring stacks, the effect summary of their triggers and operations, and their defense-dice and attack-damage modifiers;
- the own hand pooled: cards, affordable cards, cost (0, 1–5, 6–10, 11+), timing (before the roll, after it, defense, damage reaction) and effect (damage bonus, prevention, energy, draw, dice, extra rolls, statuses on self or enemy, status removal, other), plus `Fresh()` with the dice cards in hand;
- the round and whether the seat is the one the authority is waiting on.

The feature names and a schema hash are published with it (`valuenet.Features()`, `valuenet.SchemaHash()`); the encoder pins its own rates, so tuning the players never moves its inputs. Tests: forking a position with several seeds (re-dealt hidden hand, reseeded dice) and tampering with the opponent's hidden hand and private dice leave every input unchanged (Starter, Venom and Curse); encoding and recording are byte-deterministic; the width matches the schema.

**Recording.** `balance-lab -record <dir>` saves both seats' views at the start of every round's offensive planning (before anyone rolls), and when the battle ends each seat's result (1 / 0.5 / 0) and final health margin. Datasets are float32 shards with a JSON header (features, meta columns, shard SHA-256s, content hashes), sorted so the same battles always give the same bytes.

**Data.** Greedy self-play over every pairing the lab supports: the six heroes against themselves and each other, and each hero against the four minions (45 pairings), seeds 100,001–100,350 from both seats. Look-ahead v2 (`-budget 96`) against greedy or the minion's script on nine training pairings, seeds from 200,001. Training seeds never overlap the evaluation seeds (1–400).

**Held out: Venom (hero) and Bell Diver (opponent).** Each has its own die (`venom_d6`, `bell_d6`) and Venom its own statuses. Every battle that involves either, in either seat, is kept out of training and validation.

| Player | Hero | Training battles | Training positions | Held-out battles | Held-out positions |
|---|---|---|---|---|---|
| greedy | Starter | 5,600 | 43,622 | 1,400 | 12,756 |
| greedy | Adventurer | 4,900 | 42,000 | 1,400 | 13,932 |
| greedy | Curse | 4,200 | 46,962 | 1,400 | 17,706 |
| greedy | Blade Warden | 3,500 | 30,350 | 700 | 6,634 |
| greedy | Venom Goblin | 2,800 | 21,640 | 700 | 6,848 |
| greedy | Venom | — | — | 4,900 | 49,866 |
| look-ahead | Starter | 700 | 5,024 | — | — |
| look-ahead | Adventurer | 250 | 1,926 | — | — |
| look-ahead | Blade Warden, Venom Goblin, Curse | 260 | 2,304 | — | — |
| **Total** | | **22,210** | **193,828** | **10,500** | **107,742** |

Positions per battle run from about 6.5 (Starter vs minions) to 14 (Curse mirror). The per-pairing table is printed by `./scripts/ml.sh value-train --data 'runs/value-v1/*' --held-out-hero venom --held-out-opponent drowned_oracle_bell_diver --mix-only`.

```bash
LAB=ml/runs/bin/balance-lab-301be9a   # commit 301be9a
# greedy, for each pairing (heroes: starter adventurer curse venom blade_warden venom_goblin)
$LAB -hero H -opponent O -foe greedy -players greedy -seats both -battles 350 -first-seed 100001 -workers 14 -record ml/runs/value-v1/greedy-H-vs-O
$LAB -hero H -opponent MINION -foe script -players greedy -seats both -battles 350 -first-seed 100001 -workers 14 -record ml/runs/value-v1/greedy-H-vs-MINION
# look-ahead, e.g.
$LAB -hero starter -opponent starter -foe greedy -players lookahead -budget 96 -seats both -battles 100 -first-seed 200001 -workers 14 -record ml/runs/value-v1/lookahead-starter-vs-starter
```

### Training and offline check (T4, phase A2)

`ml/dice_destiny_ml/value_train.py` (`./scripts/ml.sh value-train`, or `uv run python -m dice_destiny_ml.value_train` from `ml/`) trains an MLP of two hidden layers of 128 with two heads: the win probability (sigmoid, log loss on 1 / 0.5 / 0) and the final health margin (tanh, squared error). Inputs are standardized on the training rows and the scaling is folded into the first layer at export. Train and validation split by battle (10% of battles validate); every battle with the held-out hero or opponent is kept out of both. The export (`dice-and-destiny-value-v1`) holds the tensors, the encoder's schema hash and width, the datasets' header SHA-256s, the training content hashes, the metrics, and a few check rows the Go loader must reproduce.

Baselines on the held-out set: a logistic fit on the health margin (own health share minus the opponent's; fitted on the training rows: slope 7.08) and today's evaluator `tanh(2 × margin)` read as a probability `(1 + tanh) / 2`.

**One training setting was tried, and it passed:** GELU, 2 × 128, margin weight 1, Adam 1e-3, weight decay 1e-5, batch 1,024, early stopping on validation log loss (best at epoch 11 of 16, 8 s on two threads).

| Set (positions) | Network: log loss / Brier | Logistic on margin | `tanh(2 × margin)` |
|---|---|---|---|
| Validation (19,378) | **0.397** / 0.095 | 0.517 / 0.140 | 0.542 / 0.147 |
| Held out: Venom or Bell Diver (107,742) | **0.415** / 0.117 | 0.506 / 0.153 | 0.541 / 0.164 |
| Held-out matchup, Venom vs Bell Diver (8,110) | **0.228** / 0.051 | 0.508 / 0.162 | 0.546 / 0.174 |

Calibration on the held-out set (positions: mean predicted → mean outcome):

| Predicted | Network | Logistic on margin | `tanh(2 × margin)` |
|---|---|---|---|
| 0.0–0.1 | 20,118: 0.042 → 0.036 | 8,680: 0.048 → 0.021 | 1,903: 0.071 → 0.002 |
| 0.1–0.2 | 10,168: 0.143 → 0.115 | 8,113: 0.149 → 0.101 | 5,180: 0.154 → 0.019 |
| 0.2–0.3 | 7,077: 0.243 → 0.247 | 8,992: 0.255 → 0.224 | 7,822: 0.250 → 0.078 |
| 0.3–0.4 | 5,496: 0.349 → 0.400 | 7,472: 0.350 → 0.320 | 12,435: 0.350 → 0.216 |
| 0.4–0.5 | 6,415: 0.454 → 0.454 | 7,792: 0.436 → 0.432 | 13,709: 0.444 → 0.392 |
| 0.5–0.6 | 7,402: 0.553 → 0.501 | 33,436: 0.515 → 0.516 | 39,353: 0.519 → 0.537 |
| 0.6–0.7 | 7,606: 0.649 → 0.520 | 7,472: 0.650 → 0.680 | 12,435: 0.650 → 0.784 |
| 0.7–0.8 | 5,813: 0.750 → 0.614 | 8,992: 0.745 → 0.776 | 7,822: 0.750 → 0.922 |
| 0.8–0.9 | 7,359: 0.856 → 0.736 | 8,113: 0.851 → 0.899 | 5,180: 0.846 → 0.981 |
| 0.9–1.0 | 30,288: 0.969 → 0.940 | 8,680: 0.952 → 0.979 | 1,903: 0.929 → 0.998 |

- The network separates positions far better than the health race (a third of the held-out positions sit above 0.9 or below 0.1, against a tenth for the logistic fit).
- On the unseen characters it is overconfident from 0.6 to 0.9: positions it rates 0.75 win 61% of the time. `tanh(2 × margin)` is badly underconfident: its 0.85 wins 98%.

```bash
cd ml && OMP_NUM_THREADS=2 uv run --python 3.12 python -m dice_destiny_ml.value_train \
  --data 'runs/value-v1/*' --held-out-hero venom --held-out-opponent drowned_oracle_bell_diver \
  --output runs/value-v1-models/value-v1-a.json --activation gelu --margin-weight 1.0
# sha256 4f8c370d57a7a75bec03926fb815f59b7525ac2f2098e1be3bc7f8b8f7c3337f, 476 KB
```

### Value network in the search (T5, phase A3)

With `SearchPolicy.Evaluator` set, every future stops at the first offensive-planning start after `Rounds` (the positions the network learned from), and the network's win probability p scores it as 2p − 1, on the same scale as `tanh(2 × margin)`; its expected final margin takes the health margin's place in `MarginWeight`. A position it cannot encode falls back to the health race. Without an evaluator the search is unchanged (battle for battle identical to before).

Same settings as T1 (100 ms per decision, 7 goroutines, 2 battles at a time), all players interleaved in one run per matchup, seeds 1–400 from both seats (held-out matchup: seeds 1–150). "Value" is the T4 network (`value-v1-a.json`, sha256 `4f8c370d…`).

**Starter mirror (opponent greedy, 800 paired battles):**

| Player | Score | vs v2 (95% CI, z) | vs Rounds 1 (95% CI, z) | Futures per searched decision | ms per decision |
|---|---|---|---|---|---|
| v2 (Rounds 0) | 0.505 | — | −0.084 (z −5.7) | 81 | 53.0 |
| Rounds 1 (best T1 control) | 0.589 | +0.084 (+0.055 … +0.113, z +5.7) | — | 154 | 48.1 |
| Rounds 2 | 0.510 | +0.005 (−0.003 … +0.013, z +1.2) | | 90 | 51.0 |
| **Value, Rounds 1** | 0.588 | +0.083 (+0.052 … +0.113, z +5.2) | **−0.001 (−0.022 … +0.020, z −0.1)** | 149 | 47.9 |
| Value, Rounds 2 | 0.515 | +0.010 (+0.001 … +0.019, z +2.2) | −0.074 (−0.103 … −0.045, z −5.0) | 92 | 51.4 |

**Starter vs Brine Mask (800 paired battles):**

| Player | Score | vs v2 (95% CI, z) | vs Rounds 1 (95% CI, z) | Futures per searched decision | ms per decision |
|---|---|---|---|---|---|
| v2 (Rounds 0) | 0.927 | — | | 175 | 52.5 |
| Rounds 1 | 0.948 | +0.021 (+0.007 … +0.035, z +2.9) | — | 332 | 50.5 |
| Rounds 2 | 0.930 | +0.003 (−0.003 … +0.009, z +1.0) | | 196 | 52.2 |
| **Value, Rounds 1** | 0.951 | +0.024 (+0.009 … +0.038, z +3.3) | +0.003 (−0.006 … +0.011, z +0.6) | 309 | 48.9 |
| Value, Rounds 2 | 0.929 | +0.003 (−0.002 … +0.007, z +1.2) | −0.019 (−0.033 … −0.005, z −2.6) | 197 | 51.3 |

**Held-out matchup, Venom vs Bell Diver (300 paired battles):** too one-sided to tell players apart. v2 scores 0.985, Rounds 1 and 2 0.998, the value player at Rounds 1 1.000 (+0.002 over Rounds 1, 95% CI −0.002 … +0.005) and at Rounds 2 0.998.

**Exit: failed.** The value player beats v2 clearly (+0.083, z +5.2) but not the best control: at Rounds 1 it plays as well as the health race and no better (−0.001, z −0.1); the rule needed z ≥ 2.5 over both. It loses nothing against Brine Mask (lower bound −0.006 against Rounds 1, +0.009 against v2). **Nothing shipped in-game**; the opponent stays version 2.

- **Timing at in-game settings** (budget 160, 15 goroutines, one battle at a time, Starter mirror seeds 1–30 from both seats): the value player at Rounds 1 takes 21.5 ms per decision (29.9 ms per searched decision) against v2's 57.9 ms (80.7 ms), and Rounds 1 with the health race 19.9 ms (27.3 ms). This part of the exit passes; only the strength test failed.
- The network judges positions far better offline (log loss 0.40 against 0.54 for the health race) but chooses no better. The search compares options on the same futures and only overrides the greedy player on a clear paired gain, so what matters is how the evaluator ranks one round ahead, and there the health race already carries most of the signal.
- The value player costs slightly fewer futures than the health race at equal time (149 against 154 per decision): futures run on to the next planning start and each evaluation takes about 75 µs.
- At equal time, cutting futures after one round is worth far more than the evaluator at the cut (+0.084 in the mirror). If the in-game opponent should get stronger now, Rounds 1 with the health race is the measured candidate. That was not part of this handoff, so it is not shipped.

```bash
LAB=ml/runs/bin/balance-lab-3b79cdf   # commit 3b79cdf
M=ml/runs/value-v1-models/value-v1-a.json
P=lookahead@rounds=0,lookahead@rounds=1,lookahead@rounds=2,lookahead@rounds=1@value-model=$M,lookahead@rounds=2@value-model=$M
$LAB -hero starter -opponent starter -foe greedy -players $P -think-ms 100 -search-workers 7 -workers 2 -seats both -battles 400 -out t5/starter-mirror.json
$LAB -hero starter -opponent drowned_oracle_brine_mask -foe script -players $P -think-ms 100 -search-workers 7 -workers 2 -seats both -battles 400 -out t5/starter-brine.json
$LAB -hero venom -opponent drowned_oracle_bell_diver -foe script -players $P -think-ms 100 -search-workers 7 -workers 2 -seats both -battles 150 -out t5/venom-bell.json
$LAB -hero starter -opponent starter -foe greedy -players lookahead@rounds=0,lookahead@rounds=1,lookahead@rounds=1@value-model=$M -budget 160 -search-workers 15 -workers 1 -seats both -battles 30 -out t5/ingame-timing.json
```

### Measured values (T6, phase A4)

`cmd/value-probe` replays the held-out battles (the same seeds and greedy play, so exactly the held-out positions). At every round's offensive-planning start it changes one thing on a fork of the battle (fresh randomness), encodes the acting seat's view and reads the network's change in win chance. The changes go through the engine's own paths where it has one:

- **+5 energy:** the engine's energy gain (within the cap);
- **+1 card in hand:** the engine's ordinary draw from the draw pile;
- **+1 stack of each status in the catalog:** the engine's status path with its stacking rules; positive statuses on the acting seat, negative ones on its opponent. Card-preparation markers (a stack without its card's bonus cannot occur) and map-only statuses are left out;
- **the unit, one more card removed from the opponent:** picked the way damage picks (discard pile, then draw pile, then hand), with the public counts others see during private planning moved too.

Each change ÷ the unit's change is its damage-equivalent. Per-position ratios use the positions where the unit moves the win chance by at least 0.005; "ratio of means" uses every position. On the held-out positions one more card removed is worth **+0.038 win chance** on average (+0.051 when the acting seat is below half health).

**Held-out positions (Venom or Bell Diver battles, 53,871):**

| Change | Positions | Mean | Median | IQR | Ratio of means | 2026-10-15 rate |
|---|---|---|---|---|---|---|
| +5 energy | 53871 | 0.00 | -0.01 | -0.04 … 0.03 | -0.01 | 1.0 (0.2 per point) |
| +1 card in hand (from the draw pile) | 42941 | 0.54 | 0.61 | 0.39 … 0.78 | 0.58 | — |
| enemy +1 Bleed (bleed) | 53871 | -0.35 | -0.22 | -0.72 … 0.04 | -0.14 | — |
| enemy +1 Blind (blind) | 53871 | -2.27 | -1.57 | -4.05 … -0.03 | -1.29 | — |
| enemy +1 Curse Count (curse_count) | 53871 | -0.30 | -0.17 | -0.64 … 0.08 | -0.12 | — |
| enemy +1 Cursed Entangle (cursed_entangle) | 53871 | -0.34 | -0.22 | -0.69 … 0.05 | -0.15 | — |
| enemy +1 Entangle (entangle) | 53871 | -2.18 | -1.74 | -4.09 … -0.27 | -1.17 | — |
| enemy +1 Grave Debt (grave_debt) | 53871 | -0.36 | -0.24 | -0.75 … 0.03 | -0.15 | — |
| enemy +1 Incubation (incubation) | 53871 | -0.34 | -0.22 | -0.69 … 0.05 | -0.15 | — |
| enemy +1 Poison (poison) | 53871 | -0.19 | -0.03 | -0.65 … 0.23 | -0.01 | — |
| enemy +1 Shaken (shaken) | 53871 | -0.36 | -0.24 | -0.75 … 0.02 | -0.16 | — |
| enemy +1 Volatile Poison (volatile_poison) | 53871 | -0.28 | -0.15 | -0.71 … 0.12 | -0.08 | — |
| self +1 Ambush (ambush) | 53871 | 0.00 | 0.00 | 0.00 … 0.00 | 0.00 | — |
| self +1 Catalyst (catalyst) | 53871 | 0.00 | 0.00 | 0.00 … 0.00 | 0.00 | — |
| self +1 Conqueror (conqueror) | 53871 | -1.02 | -1.03 | -1.31 … -0.76 | -1.03 | — |
| self +1 Fortified (fortified) | 53871 | -1.02 | -1.03 | -1.31 … -0.76 | -1.03 | — |
| self +1 Lieutenant (lieutenant) | 53871 | 0.00 | 0.00 | 0.00 … 0.00 | 0.00 | — |
| self +1 Lieutenant's Favor (lieutenants_favor) | 53871 | 0.00 | 0.00 | 0.00 … 0.00 | 0.00 | — |
| self +1 Protect Status (protect) | 53871 | 0.00 | 0.00 | 0.00 … 0.00 | 0.00 | 1.0 |

Medians by round and own health (positions in brackets):

| Change | rounds 1–2 | rounds 3–5 | rounds 6+ | health ≥ half | health < half |
|---|---|---|---|---|---|
| unit: win-chance change per card removed (mean) | +0.0412 (21000) | +0.0355 (27969) | +0.0372 (4902) | +0.0343 (42139) | +0.0508 (11732) |
| +5 energy | 0.00 (21000) | -0.01 (27969) | -0.02 (4902) | -0.00 (42139) | -0.01 (11732) |
| +1 card in hand (from the draw pile) | 0.60 (20767) | 0.62 (20563) | 0.73 (1611) | 0.61 (39183) | 0.64 (3758) |
| enemy +1 Blind (blind) | -2.49 (21000) | -1.15 (27969) | 0.20 (4902) | -2.24 (42139) | -0.00 (11732) |
| enemy +1 Poison (poison) | -0.09 (21000) | 0.00 (27969) | -0.02 (4902) | -0.05 (42139) | 0.00 (11732) |
| self +1 Fortified (fortified) | -1.18 (21000) | -0.87 (27969) | -0.78 (4902) | -1.14 (42139) | -0.67 (11732) |
| self +1 Protect Status (protect) | 0.00 (21000) | 0.00 (27969) | 0.00 (4902) | 0.00 (42139) | 0.00 (11732) |

**How to read it.** These are the network's values, and most of them are not yet values of the riders:

- **Self-statuses are not identified.** No positive status ever exists at an offensive-planning start in the training data: Protect expires at the round's Ongoing Effects, before planning, and Ambush, Lieutenant and the other map conditions never occur in lab battles. 61 of the 148 inputs never vary in training, every positive-status input among them, so the network learned nothing from them: Protect, Ambush, Catalyst, Lieutenant and Lieutenant's Favor measure exactly 0. Fortified and Conqueror move only through the defense's expected prevention, which in training varies only between characters, so their negative value is a character effect.
- **Enemy debuffs come out negative.** In training these statuses appear only when the seat's own character inflicts them (Curse's statuses; poison, bleed and entangle from Venom Goblin's and Blade Warden's abilities), so the network learned "the enemy carries this debuff, so I am that character", not the debuff's effect.
- **Energy is nearly worthless to these players:** −0.01 per 5 energy on held-out positions and +0.04 on Starter positions, against 1.0 in board pricing. The data's greedy player plays no Venom, Curse, Blade Warden or Venom Goblin cards, and the held-out battles are mostly those characters.
- **A card moved from the draw pile to the hand** is worth 0.58 (median 0.61) on held-out positions and 0.21 (median 0.03) on Starter positions. Card pricing counts a drawn card as half a health.

For comparison, on in-distribution positions (the Starter's training battles, 21,811 positions: `-held-out=false -only greedy-starter-vs-`) the unit is +0.056 win chance, +5 energy 0.04 (median 0.04), a card in hand 0.21 (median 0.03), Protect 0.00, Poison on the enemy −0.21 and Blind on the enemy −1.12: the same pattern.

| Rider | 2026-10-15 exchange rate | Measured, held out (ratio of means) | Measured, Starter positions |
|---|---|---|---|
| +1 Protect on self | 1.0 | 0.00 (not identified) | 0.00 (not identified) |
| +5 energy | 1.0 (0.2 per point) | −0.01 | 0.04 |

**No rates or content were changed.** To measure riders the next pass needs positions where they vary on their own: record more decision points (before defense, mid-round) or add riders to recorded positions at random and play those positions out, and data from players that use them (the look-ahead, phase A6). Until then these numbers should not replace the agreed rates.

```bash
go build -o ml/runs/value-probe ./cmd/value-probe
ml/runs/value-probe -value-model ml/runs/value-v1-models/value-v1-a.json -workers 2 -out ml/runs/lab/t6-probe.json
ml/runs/value-probe -value-model ml/runs/value-v1-models/value-v1-a.json -held-out=false -only greedy-starter-vs- -workers 1
```

## AI network v2 (2026-10-22)

Steps N1–N6 of [`ai-value-network-plan.md`](ai-value-network-plan.md) ("Network v2"; brief: [`ai-network-v2-handoff.md`](ai-network-v2-handoff.md)). The machine (M3 Max, 16 cores) was fully loaded by these runs and, at times, the board thread's; budget-mode results do not depend on load, and time-limited players always run interleaved.

### In-game opponent version 3 (decision 1)

The user left the choice to the AI thread; the measured candidate from T1 and T5 ships. Version 3 is version 2's search with every simulated future stopped after one round and scored by the health race, at budget 400 (version 2: futures to the end, budget 160). Both at in-game settings in budget mode (so machine load cannot change the result), seeds 1–200 from both seats:

| Matchup | v2 (Rounds 0, budget 160) | v3 (Rounds 1, budget 400) | v3 − v2 (95% CI, z) | ms per decision, v2 → v3 |
|---|---|---|---|---|
| Starter mirror (opponent greedy) | 0.551 | 0.626 | **+0.075** (+0.030 … +0.120, z +3.25) | 907 → 719 |
| Starter vs Brine Mask | 0.931 | 0.955 | **+0.024** (+0.002 … +0.045, z +2.15) | 700 → 503 |

- Version 3 wins clearly in the mirror, loses nothing against Brine Mask, and takes about a quarter less time per decision. (The ms are from a machine loaded several times over and only compare the two.)
- It overrides the greedy player more often (8–12% of searched decisions against 2–4%).
- `LookaheadArchitecture` is now `flat-monte-carlo-health-race-v3`; `-tally` and `battle-review` name v1, v2 and v3 games apart.

```bash
LAB=ml/runs/bin/balance-lab-fdd231c   # commit fdd231c
P=lookahead@rounds=0@budget=160,lookahead@rounds=1@budget=400
$LAB -hero starter -opponent starter -foe greedy -players $P -seats both -battles 200 -workers 5 -out v3b/starter-mirror.json
$LAB -hero starter -opponent drowned_oracle_brine_mask -foe script -players $P -seats both -battles 200 -workers 5 -out v3b/starter-brine.json
```

### Decision benchmark (N1)

Instead of asking how well a network predicts who wins, the benchmark asks how much value a player's move loses in real decisions.

- **Recording.** `balance-lab -label FILE` records the hero's decisions in real battles (with `-label-sample`, a share of them). For each decision it measures every option on the same futures (one seed per future for every option, as the search does), by successive halving: a quarter of `-label-futures` for every option, half for the better half, all of them for the best two. Futures follow `-label-rounds` (default 1, the health race). Options are the search's, wider: the solver's best 8 rerolls and die choices (`-label-top-rerolls`), best 4 ability choices (`-label-top-abilities`), and every other useful move. The recorder works on a copy of the decision with its own randomness, so a recorded battle plays exactly as it would unrecorded (`TestRecorderLeavesBattlesUnchanged`), and it records only the hero, who plans first, so no record holds the other side's private dice. Records keep the whole battle state (the pinned catalog once per file), so new encoders can read old records.
- **Scoring.** `balance-lab -score FILES -players …` asks each player for its move on every recorded decision and reports the **value lost**: the measured best option's value minus the chosen option's, on the search's scale (win +1, loss −1, plus 0.2 × the final health margin; half of it is win chance). It also reports how often the player chose the best option and how often it kept greedy's pick, overall and by kind of decision. The best option is picked on the same futures that measure it, so every player's loss carries the same small upward bias; differences between players are fair.
- **Training arrays.** `cmd/decision-data` turns records into the network's inputs (below) and targets.

### Benchmark v1 and headroom

Benchmark v1: decisions of the Rounds 1 look-ahead (budget 250) against the greedy player or a minion's script, 30% of its decisions with two or more options, seeds 300,001–300,020 from both seats, 512 futures for the best two options. 12 matchups, 4,110 decisions; the held-out characters (Venom, Bell Diver) are in their own slice (1,025 decisions).

Value lost per decision on the first six finished matchups (2,152 decisions):

| Kind | Decisions | Greedy | Rounds 1 search (budget 250) |
|---|---|---|---|
| All | 2,152 | 0.0298 | **0.0109** |
| Choosing a defense | 394 | 0.0480 | 0.0036 |
| Defense reactions | 253 | 0.0237 | 0.0023 |
| Offensive reaction cards (Curse) | 61 | 0.1661 | 0.0029 |
| Offensive cards | 339 | 0.0198 | 0.0133 |
| Offensive dice | 660 | 0.0224 | 0.0196 |

- **Greedy's biggest errors are on defense**: it always rolls its defensive ability first, when playing Steady Guard or Take Stock first is often worth several points of win chance; and Curse's greedy player never uses its reaction cards. The search already fixes most of these.
- **What the search leaves is offense**: about 80% of its remaining loss is dice and card decisions, where options differ by little and its futures cannot separate them.
- Futures stopped after one round score some equivalent moves differently: two kinds of pass that lead to the same battle get different health-race values because the cut falls at a different moment; played to the end they are identical. Re-measured with full playouts (`-relabel … -label-rounds 0`), the Starter mirror's 320 decisions rank the players the same way (greedy 0.0293, the search 0.0229, round 1's network 0.0190), with wider intervals (±0.006–0.007).

### Policy network (N3, N4)

`cmd/decision-data` reads each record from the deciding seat's view: value-v2 (value-v1's 148 inputs plus the decision point: segment and stage, the seat's dice and the solver's view of them, the expected damage, statuses and energy of the solver's play, the selected ability, attacks in flight) and option-v1 for each option (what the move does: its kind, whether greedy picks it, the solver's value and expected results for a reroll, die choice or dice card, an ability tier's value and effects, a defense's expected effect, a card's cost, timing and effects; never IDs). Viewer safety (forks and hidden-hand or private-dice changes leave every input unchanged at every kind of decision), determinism and width are tested.

`ml/dice_destiny_ml/policy_train.py` trains a state trunk (2 × 128) and an option scorer (128, 64) on each option's measured value against greedy's pick, weighted by its precision, plus value and result heads; it picks the margin by which a pick must beat greedy's on validation. The Go player (`valuenet.NetworkPlayer`, lab player `network`) plays the network's pick among the search's options, with no search at all.

**Round 1 (N1b):** labels from the Rounds 1 look-ahead's battles, its options measured with greedy making the later moves (7 matchups, 12,333 decisions, 128 futures). On the benchmark it loses 0.0184 per decision against greedy's 0.0308 (held-out slice: 0.0210 against 0.0268). In battles (network alone against greedy, Starter mirror, seeds 1–2,000 from both seats):

| The network decides | Score − greedy (95% CI, z) |
|---|---|
| defense only | **+0.092** (+0.077 … +0.107, z +11.8) |
| everything | −0.002 (z −0.3) |
| offense only | −0.052 (z −5.7) |

On defense the network alone is as strong as 100 ms of search. On offense it loses even though each of its offense moves measures better than greedy's: every label assumes greedy finishes the turn, and the dice solver's plan for a turn is consistent while the network's step-by-step changes are not.

**Round 2 (N2):** the player is round 1's network on defense with greedy elsewhere, and its options are measured with that player making its own later moves (12 matchups, 11,731 decisions, 96 futures, half of each battle's decisions). Trained on rounds 1 and 2 together:

| Matchup (seeds from both seats) | Network on defense | Network on everything |
|---|---|---|
| Starter mirror (1–2,000) | +0.095 (z +12.2) | **+0.061** (z +6.8) |
| Starter vs Brine Mask (1–1,000) | +0.026 (z +5.2) | +0.018 (z +3.0) |
| Starter vs Drowned Regent | +0.121 (z +11.9) | +0.107 (z +9.1) |
| Starter vs Bell Diver (held out) | +0.040 (z +7.5) | +0.029 (z +4.6) |
| Curse mirror | +0.234 (z +19.5) | +0.236 (z +17.6) |
| Blade Warden mirror | +0.261 (z +20.3) | **+0.366** (z +29.2) |
| Venom mirror (held out) | −0.051 (z −4.0) | +0.053 (z +3.7) |
| Venom vs Bell Diver (held out) | −0.012 (z −2.2) | −0.070 (z −8.9) |

- One round of policy iteration turned the network's full play from −0.002 into +0.061 in the Starter mirror; the network now beats greedy on everything in every trained matchup, by a lot where greedy ignores a character's cards (Curse, Blade Warden).
- It does not yet carry over to Venom, which it never saw: Venom has its own statuses and cards. A deployed network trains on every character.

**An engine fault the network found.** In Venom battles the authority sometimes lists a card play it then refuses: Venom mirror, seed 117, round 1, the offensive reselect window, "Measured Dose" committed with `status_id: gain` and the Venom as target is listed by `LegalActions` and rejected with `command "planning_commit_cards" is not allowed`. The network made it in 25 of 2,000 Venom mirror battles and 10 of 2,000 against Bell Diver; greedy never plays it. A human could press it too. It is engine behaviour outside this thread's scope, so it is reported, not changed; the lab now keeps such battles as errors, leaves them out of every score and comparison, and reports them.

**Round 3:** the player is round 2's network on everything, and its options are measured with that network making all its own later moves (12 matchups, 13,432 decisions). It did not help: on everything the network scores +0.007 (round 3's data alone), +0.043 (rounds 2–3) and +0.054 (rounds 1–3) over greedy in the Starter mirror, against round 2's +0.061; on defense alone every round gives +0.092–0.095. Leaving the dice decisions (rerolls, die choices) to the exact solver (`-solver-dice`) gives +0.072 with round 2's network, so dice account for part of the offense loss and its card and ability choices for the rest.

**Head to head.** Every comparison above is against the greedy player, which the search models exactly. Played against each other, neither knows the other: the Rounds 1 search (budget 250, about 100 ms per decision) as the hero against a network opponent (`-foe network@…`; it never searches, so it never sees the hero's private dice), Starter mirror, seeds 1,001–1,300 from both seats:

| Search against | Search's score (95% CI) |
|---|---|
| the network on defense, greedy elsewhere | **0.498** (0.462 … 0.535) |
| the network on everything except dice | 0.527 (0.491 … 0.563) |

**The network on defense plays as well as 100 ms of search, at about 1 ms per decision.** It is not stronger.

**The network inside the search** (its pick as the default on defense, its best options searched; budget 250, Starter mirror, seeds 1,001–1,300 from both seats): +0.013 over the plain search (−0.011 … +0.037, z +1.1). The search already finds most of what the network knows about defense.

**N5's exit is not passed:** no network player beats the Rounds 1 search by z ≥ 2.5 in the Starter mirror, so nothing network-based ships in-game. What stops it:

- **Offense.** Each label assumes the rest of the turn is played by someone else, while dice and card plays depend on each other within a turn: the solver plans a turn as a whole, the network move by move. Round 3 shows more rounds of the same labels do not fix it. The next design lets the network set the solver's exchange rates for the position (what a status stack, an energy point or a card in hand is worth in damage) instead of overriding its moves, so every turn is still planned exactly.
- **Characters it has not seen.** Venom (held out) is not learned; a network for the game trains on every character.
- **Label noise and myopia.** One-round futures do not count energy or what a card in hand will do later.

```bash
# Benchmark v1 (per matchup; the 12 are in ml/runs/bench/v1/run.sh), seeds from 300,001
$LAB -hero starter -opponent starter -foe greedy -players lookahead@rounds=1@budget=250 -seats both -battles 20 -first-seed 300001 \
  -workers 4 -label bench/v1/starter-vs-starter.jsonl.gz -label-futures 512 -label-sample 0.3 -label-workers 2
$LAB -score 'bench/v1/*.jsonl.gz' -players greedy,lookahead@rounds=1@budget=250,network@policy-model=M.json
# Round 1 labels (ml/runs/train/d1/run.sh): look-ahead heroes, greedy continuation, 128 futures, seeds from 400,001
$LAB … -players lookahead@rounds=1@budget=100 -battles 30 -first-seed 400001 -label … -label-futures 128 -label-sample 1
# Rounds 2 and 3 (ml/runs/train/d2, d3): the previous network plays and continues the futures
$LAB … -players network@policy-model=N1b.json@network-segments=defensive -first-seed 600001 -label … -label-futures 96 -label-sample 0.5 \
  -label-policy-model N1b.json -label-network-segments defensive
go run ./cmd/decision-data -in 'train/d1/*.jsonl.gz' -out data/d1
cd ml && uv run python -m dice_destiny_ml.policy_train --data runs/data/d1 runs/data/d2 --benchmark runs/data/b1 --output N2.json
# Games: the network alone, by segment, and head to head
$LAB -hero starter -opponent starter -foe greedy -players greedy,network@policy-model=N2.json@network-segments=defensive,network@policy-model=N2.json -seats both -battles 2000
$LAB -hero starter -opponent starter -foe network@policy-model=N2.json@network-segments=defensive -players lookahead@rounds=1@budget=250 -seats both -battles 300 -first-seed 1001
```

The models are in the ignored `ml/runs/lab/n4/` (round 2 on rounds 1–2 data: `policy-n2-d12.json`).

### Rider values by paired futures (N2)

`value-probe -paired` measures riders without a network. At the start of each of the hero's rounds (before anyone rolls) it forks the battle into identically seeded copies, adds one rider to a copy through the engine's own paths, and plays every copy to the end with the greedy player (the minions with their scripts). Copies share their seeds, so they roll the same dice until the rider changes what happens. A rider's worth is its mean change in result divided by the unit's, one more card removed from the opponent, with a 95% interval by the delta method over positions.

Ten matchups (Starter mirror, Starter against Brine Mask and Drowned Regent, Adventurer against Ribbon Eel, Curse, Blade Warden, Venom and Venom Goblin mirrors, Curse against Starter, Venom against Bell Diver), seeds 500,001–500,040 from both seats: 3,834 positions, 8 paired futures per position and change. The unit moves the result by +0.128 (6.4 points of win chance).

| Rider | Damage-equivalent (95%) | 2026-10-15 rate |
|---|---|---|
| self +1 Protect | **0.87 ± 0.07** | 1.0 |
| +5 energy | 0.02 ± 0.01 | 1.0 (0.2 per point) |
| +1 card in hand (from the draw pile) | 0.04 ± 0.09 | — |
| enemy +1 Poison | 1.18 ± 0.08 | — |
| enemy +1 Volatile Poison | 2.49 ± 0.14 | — |
| enemy +1 Bleed | 0.86 ± 0.06 | — |
| enemy +1 Blind | 0.99 ± 0.08 | — |
| enemy +1 Cursed Entangle | 0.56 ± 0.07 | — |
| enemy +1 Entangle | 0.45 ± 0.05 | — |
| enemy +1 Incubation | 0.15 ± 0.02 | — |
| enemy +1 Curse Count | 0.12 ± 0.03 | — |
| enemy +1 Grave Debt | 0.02 ± 0.01 | — |
| enemy +1 Shaken | 0.00 | — |
| self +1 Fortified (map condition) | 4.25 ± 0.24 | — |
| self +1 Conqueror (map condition) | 4.25 ± 0.24 | — |
| self +1 Lieutenant's Favor (map condition) | 2.49 ± 0.14 | — |
| self +1 Ambush, Catalyst, Lieutenant | 0.08, 0.09, 0.00 | — |

- **Protect is measured: 0.87 damage per stack** (interval ±0.07), a little under the agreed 1. The greedy player spends Protect whenever it is offered, so this is the value of a player who uses it.
- **Energy is not measured yet.** The greedy player barely spends energy (it plays no Venom, Curse, Blade Warden or Venom Goblin cards), so 5 energy is worth almost nothing *to it*. That is a fact about the player, not the rider.
- **Decision 2 applied** (below, with the network's measurements): Protect qualifies; the rate lives in the board thread's `riders.yaml`, so it is handed over, not changed here.
- Status values are averages over these matchups; a status matters more against characters with fewer ways to shed it.

**With the network playing both sides** (round 2's network, dice left to the solver; the same matchups without Venom, 3,285 positions; `-policy-model`, `-changes`):

| Rider | Greedy players | Network players | 2026-10-15 rate |
|---|---|---|---|
| self +1 Protect | 0.87 ± 0.07 | **0.61 ± 0.07** | 1.0 |
| +5 energy | 0.02 ± 0.01 | **0.14 ± 0.05** | 1.0 (0.2 per point) |
| +1 card in hand (from the draw pile) | 0.04 ± 0.09 | **0.39 ± 0.10** | half a health in `card-value-rules.md` |

A rider's worth depends on who plays it: the stronger player, who already defends better, gets less from Protect and more from energy and cards. **Hand-over to the board thread (decision 2):** Protect 0.6 per stack, from the stronger player; energy keeps the agreed 0.2 per point until a player that plays energy cards well measures it (this one still loses on offense), though both measurements put it far lower; a card in hand measures close to the card rules' half a health.

```bash
go build -o ml/runs/bin/value-probe-paired ./cmd/value-probe
ml/runs/bin/value-probe-paired -paired -matchups starter:starter:greedy,starter:drowned_oracle_brine_mask:script,starter:drowned_regent:script,adventurer:drowned_oracle_ribbon_eel:script,curse:curse:greedy,blade_warden:blade_warden:greedy,venom:venom:greedy,venom_goblin:venom_goblin:greedy,curse:starter:greedy,venom:drowned_oracle_bell_diver:script \
  -first-seed 500001 -battles 40 -workers 3 -futures 8 -out ml/runs/riders/paired-v1.json
```

## Limits and next steps

- **Dice solver:** handles pools of identical dice. Mixed pools fall back to plain simulation.
- **Opponent models:** the model of a hero opponent is `greedy`, so the look-ahead plays best against players like it.
- **Next:** an equal-XP deck sampler using `Economy.Value`, a hero × minion matrix, and the same matrix at each player level.
