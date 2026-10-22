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

The tally splits games by AI version. The game's opponent is version 2 from 2026-10-15; earlier logged games were version 1.

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

## AI value network (2026-10-21)

The first pass of [`ai-value-network-plan.md`](ai-value-network-plan.md) (phases A0–A4; brief: [`ai-value-network-handoff.md`](ai-value-network-handoff.md)). All runs are on an M3 Max (16 cores) shared with other work. Results files and binaries live under the server's ignored `ml/runs/`.

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

## Limits and next steps

- **Dice solver:** handles pools of identical dice. Mixed pools fall back to plain simulation.
- **Opponent models:** the model of a hero opponent is `greedy`, so the look-ahead plays best against players like it.
- **Next:** an equal-XP deck sampler using `Economy.Value`, a hero × minion matrix, and the same matrix at each player level.
