# AI value network plan

Plan for growing the look-ahead opponent into an AlphaZero / TD-Gammon style
hybrid: the exact dice solver and the search stay, and a small learned network
judges positions and guides the search. It builds on the balance lab and does
not revive the PPO stack.

Status: ready to build (2026-10-21). Build brief for A0–A4:
`ai-value-network-handoff.md`. Companion plan:
`dice-and-board-selection-plan.md`, which needs the measured rider values this
plan produces (phase A4).

## What the AI is for

The user's three long-term uses (agreed 2026-10-21):

1. **Balancing:**
   - find cards, abilities and boards that are out of balance;
   - measure how far out they are.
2. **The opponent:** an AI to play against the player that is strong and
   plays any character, die and board.
3. **Valuing effects:** measure what statuses, energy, cards in hand and
   other riders are worth in damage. These are very hard to estimate by
   hand, and both board pricing and card pricing depend on them.

Speed is not the problem. Logged in-game look-ahead decisions take 75 ms at
the median and over 1 s on under 1% of decisions. The dice tables are
already cached per battle (`lookahead/dice.go`, `tableKey`).

The plan serves the three uses like this:

- **Strength (use 2).** The search plays every candidate to the end of the
  battle with the greedy player. A value network lets it stop after a round
  or two and spend the time on more candidates or deeper search.
- **Measured values (uses 1 and 3).** Two sets of hand-set rates exist
  today:
  - the greedy player's (status 0.5 per stack, energy 0.1, in `TierValue`);
  - the board pricing's (Protect 1, energy 0.2, in `board_balance.py`).

  A value network can measure what each rider is worth in real battles, and
  how much any card or ability is worth against its price.
- **Any board (uses 1 and 2).** Players will pick their own die and board.
  The network must judge positions for boards it has never seen, so it
  describes abilities and cards by what they do, never by ID.

## What exists

| Piece | Where | Use in this plan |
|---|---|---|
| Exact dice solver | `lookahead/dice.go` | Keeps planning every offensive turn. Its `Fresh()` value of a turn is also a strong network input. |
| Flat Monte Carlo search | `lookahead/search.go` | Gets the network through the existing `Playout.Evaluate` hook. `SearchPolicy.Rounds` already cuts playouts; today's evaluator is `tanh(2 × health margin)`. |
| Hidden-hand handling | `lookahead.Fork` | Re-deals unseen hands in every simulated future. Unchanged. |
| Viewer-safe view | `snapshot.FromBattleForViewer`, `Simulator.View` | The only state the encoder may read. |
| Effect summaries | `mlsim/encoding_v2.go` (`v2OperationSummary`), `encoding_v3.go` | Starting point for an effect-only encoder. |
| Pure-Go MLP inference | `learned/policy_v3.go` (`layer.linear`, activations), JSON tensor files with SHA-256 pins | Reused for the value network. |
| Torch environment | `ml/pyproject.toml` | Offline training only. No gym bridge or PPO. |
| Parallel battle runner | `lookahead/lab.go` `RunBattles`, `cmd/balance-lab` | Generates data and judges every phase. |

Not reused: the PPO candidate policies (Blade Warden only, no value head,
ID-hashed or vocabulary-indexed inputs, pinned to an old content hash).

## Design

### Position and target

- **Position:** the start of each offensive planning step, before the first
  roll, seen by the seat about to act. Inside a turn the solver is exact, so the
  network only judges positions between turns.
- **Target:** the battle result for that seat (win 1, draw 0.5, loss 0), plus
  a second output for the final health margin (cards left, as a fraction of
  max health). The margin output helps training and gives finer comparisons.

### Inputs (effect-only encoder, `value-v1`)

All read from the seat's viewer-safe view:

- **Each side:** health in each pile (hand, draw, discard, removed), max health,
  energy, rolls and dice.
- **Statuses:** for each, its polarity, stacks, cap, expiry and an effect
  summary (prevention, damage, resource, dice).
- **Own hand:** each card's cost, timing and effect summary, pooled into a
  fixed size.
- **Opponent hand:** count only.
- **Boards, both sides:** the solver's expected value of a fresh turn
  (`Fresh()`), its no-hit chance and the value of each tier's effect summary.
  This covers any die and board without naming them.
- **Defense:** the expected prevention of each defensive ability.
- **Round number and who acts first.**

No card, ability, status or character ID enters the network.

### Network

A small MLP (two hidden layers of about 128). It is trained in torch and
exported to the existing JSON tensor format, then evaluated in pure Go. At this
size one evaluation takes microseconds, far below one playout.

## Phases

Each phase ends with a lab result. A phase that fails its exit test stops the
plan at that point; the earlier phases still leave something useful.

### A0 — Equal-time benchmark (small)

- Add a thinking-time budget to `balance-lab` (fixed ms per decision instead
  of a fixed number of futures), so players are compared at the same cost.
- **Control players:** v2 look-ahead with `Rounds` = 0, 1 and 2, using today's
  health-margin evaluator. The network must beat the best of these, not just
  full playouts.
- **Matchups:** Starter mirror, Starter vs Brine Mask, Venom, Curse and Blade
  Warden mirrors. All runs use paired seeds from both seats.

**Exit:** a baseline table in `balance-lab.md`.

### A0b — Align the AI's rider rates (small)

- Make the greedy player's `TierValue` use the same exchange rates as board
  pricing: Protect 1 per stack, energy 0.2.
- Read the rates from one place, so the AI and pricing can never disagree
  again. Other self-statuses keep 0.5 until A4 measures them.
- **Lab test:** greedy at the new rates against greedy v2.
  - Starter mirror and vs Brine Mask, paired seeds.
  - The Measured Strike pick rate is reported too.

**Exit:** the new rates are no worse in either matchup, or the result is
recorded and the old rates stay.

### A1 — Encoder and recorder (medium)

- `value-v1` encoder in Go, with a test that it never reads hidden state (the
  same idea as `TestViewNeverLeaksHiddenState`) and that it is deterministic.
- `balance-lab -record <file>` writes each position's inputs and, when the
  battle ends, its targets.
- **Data mix:**
  - bulk greedy-vs-greedy self-play across all characters and minions;
  - a smaller look-ahead set for quality;
  - random equal-XP decks, once the deck sampler exists;
  - random dice and boards from the board builder once phase B4 of the
    companion plan exists. Until then, a few hand-made boards on the Monk die
    and others.
- **First target:** about 200k positions (about 10k battles).

**Exit:** a recorded dataset and a small report of its matchup mix.

### A2 — Train and check offline (medium)

- `ml/dice_destiny_ml/value_train.py` trains on the recorded data and exports
  the tensors.
- **Comparisons:** log loss and calibration against two baselines: a logistic
  fit on health margin, and the current `tanh` evaluator.
- **Held-out test:** one character and one die/board are kept out of training
  to test generalization.

**Exit:** the network beats both baselines on the held-out set.

### A3 — Plug into the search (small)

- A `Playout.Evaluate` that encodes the view and runs the network.
- `SearchPolicy` uses it when `Rounds` > 0. Without a model file it behaves
  exactly as v2.
- **Lab test:** the new player against v2 and the A0 controls at equal time.

**Exit:**
- a paired gain in the Starter mirror with z ≥ 2.5;
- no measurable loss vs Brine Mask;
- in-game median decision time no worse than today.

**Ship:** if the exit test passes, the in-game opponent becomes
`flat-monte-carlo-value-v3` (`LookaheadArchitecture`). Logs record the model's
SHA-256.

### A4 — Measured rider values (small)

- On recorded turn-start positions, change one thing and read the network's
  value change:
  - +1 Protect on self;
  - +5 energy;
  - +1 stack of each status;
  - +1 card in hand;
  - 1 more card removed from the opponent (the damage unit).
- Each value change, divided by the change for 1 damage, is that rider's
  damage-equivalent. Report the average and the spread by round and health.
- **Feeds the companion plan:**
  - these numbers replace the agreed exchange rates (Protect 1, energy 0.2)
    in board pricing;
  - the greedy player's `TierValue` rates switch to them as well, behind a
    lab test.

**Exit:** an exchange-rate table in `balance-lab.md`, with the lab result for
greedy at the new rates.

### A4b — Balance finder (medium)

- **Card audit:**
  - for every card in a pool, the network's value change from having it in
    hand, against its XP price under `card-value-rules.md`;
  - plus the existing `cmd/card-balance` win-rate test for the outliers.
- **Ability and board audit:**
  - for every ability on a board, the change in the board's value without
    it (its worth), against what pricing expected;
  - for every die and template, the lab score.
- **Output:** a ranked list of the most over- and under-priced cards,
  abilities and boards, saved as a report in `balance-lab.md`. Fixes go
  through the normal pricing docs, never hand edits inside the lab.

**Exit:** the first report, with each top outlier confirmed or rejected by a
win-rate test.

### A5 — Policy prior (medium)

- Train a second output on the look-ahead's own choices: which candidate it
  picked, and how close the others came.
- Use it to choose which options get search time, replacing the hand rules
  (`TopRerolls` 3, top 2 abilities).

**Exit:** a gain at equal time, as in A3.

### A6 — Deeper search and self-play loop (large, optional)

- Replace flat search with a shallow tree:
  - chance branches for dice;
  - a re-dealt hidden hand per branch;
  - network values at the leaves.
- Then repeat: play with the current network, retrain on those games, test in
  the lab. Stop when a round's gain is inside the noise.

## Progress notes

- 2026-10-21, T2 (A0b): the AI's rider rates live in one Go place,
  `lookahead.Rates`. They move to the board work's
  `content/progression_v1/riders.yaml` once that merges. Greedy at the
  board-pricing rates lost to greedy v2 in the Starter mirror, so the
  version-2 rates stay (results in `balance-lab.md`).

- 2026-10-21, T1 (A0): `balance-lab -think-ms` compares players at equal
  time. At 100 ms per decision (the in-game median and work), Rounds 1
  with the health race is the best control: +0.093 over full playouts in
  the Starter mirror (z +6.3), +0.026 against Brine Mask (z +3.6).
- 2026-10-21, T3 (A1): the `value-v1` encoder (148 effect-only inputs from
  the viewer-safe view) and `balance-lab -record`. 193,828 training
  positions from greedy self-play over 45 pairings plus 1,210 look-ahead
  battles; Venom and Bell Diver held out.
- 2026-10-21, T4 (A2): the network beats both baselines on the held-out
  set (log loss 0.415 against 0.506 and 0.541) with the first setting.
- 2026-10-22, T5 (A3): exit failed. The value player gains +0.083 over v2
  in the Starter mirror but nothing over Rounds 1 with the health race
  (−0.001, z −0.1). Nothing shipped; the in-game opponent stays v2.
- 2026-10-22, T6 (A4): `cmd/value-probe` measures riders on the held-out
  positions. Most are not identified yet: positive statuses never exist
  at a planning start in the data, enemy debuffs only appear with the
  character that inflicts them, and the greedy data barely uses energy.
  No rates changed; the next pass needs positions where riders vary on
  their own.

## Rules that stay

- The network sees only the viewer-safe view, and the search keeps
  `Fork`'s re-dealing. The AI never sees the human's hand or future dice.
- The exact solver keeps planning dice; the network never decides a reroll by
  itself.
- A missing or failed model falls back to v2 play. It is never an error and
  never an illegal action.
- After a content change, rerun the lab regression ("value player still beats
  v2"). Retrain only if it stops winning. Effect-only inputs should make this
  rare, which is why the old PPO models are not reused.
- Model files hold the training content hash for reference, but load without
  a content pin. The lab regression is the guard instead.

## Risks

- **Too little variety in the data.** If only the Starter and minions are
  recorded, the network will misjudge new boards. Random dice and boards from
  the board builder are part of the data plan, not an extra.
- **Wrong values from the greedy self-play.** Greedy play is weaker than
  look-ahead, so its outcomes undervalue things only a stronger player uses
  well (the Measured Strike experiment). The look-ahead subset and A6 address
  this.
- **Truncation alone may be enough.** If `Rounds` = 1 with the health-margin
  evaluator already matches the network, stop at A0. That outcome is
  recorded as a result, not a failure.
