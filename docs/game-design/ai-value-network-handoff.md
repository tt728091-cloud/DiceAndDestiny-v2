# Handoff: AI value network (first pass)

**Status:** ready to implement (written 2026-10-21).

**Plan of record:** [`ai-value-network-plan.md`](ai-value-network-plan.md),
phases A0, A0b, A1, A2, A3 and A4. Results so far are in
[`balance-lab.md`](balance-lab.md).

**Scope:** six tasks, T1–T6. Later handoffs, written after this one's
results are in:

- A4b, the balance finder;
- A5, the policy prior;
- A6, deeper search and self-play;
- re-recording data with random boards once the board work merges.

**Running in parallel:**
- The Board Builder ([`dice-and-board-selection-handoff.md`](dice-and-board-selection-handoff.md),
  branch `dice-board-selection`) runs in another thread.
- So does gear (`phase-2-gear`).

The ownership rules below keep them apart.

**What the user wants from the AI (2026-10-21):**

1. **Balancing:** find cards, abilities and boards that are out of balance.
2. **An opponent:** an AI that plays well against the player.
3. **Valuing effects:** measure what statuses, energy and other riders are
   worth, because they are very hard to estimate by hand.

This pass builds the foundation for all three: a fair benchmark, aligned
rates, a value network, a stronger search and the first measured values.

**Decisions the user made:**

- **Build on the balance lab.** Do not revive the PPO stack (the old
  candidate policies and the gym training loop).
- **Rider rates.** The AI values riders at the board pricing's exchange rates
  (Protect 1 per stack, energy 0.2 per point), tested in the lab first (T2).

---

## Prompt for the implementing model

> You are working in the Dice and Destiny repository. Read `AGENTS.md` first and follow it exactly, especially: run Godot only through `./scripts/godot.sh`, keep the Go server the authority, and never let the AI read hidden state (the human's hand or future dice).
>
> Then read `docs/game-design/ai-value-network-handoff.md` and implement tasks T1 to T6 in order, with every test it lists. Also read the plan it points to and `docs/game-design/balance-lab.md`. Work on a new branch named `ai-value-network` created from local `main`. Commit each task separately with a clear message. Do not push or merge unless the user asks.
>
> Before changing code, confirm each "where it lives" pointer against the current code; line numbers may have moved. The handoff makes choices marked **Decision**. Follow them. If one contradicts the code or `AGENTS.md`, stop and report it rather than guessing.
>
> Each task has an exit test. When a task's exit test fails, follow that task's stop rule. Some say record the result and continue, some say stop the handoff and report. Never weaken a test or cherry-pick seeds to pass.
>
> Don't touch the board work: `internal/battle/boardprice`, `cmd/board-lab`, `content/progression_v1/board_templates.yaml`, `content/progression_v1/riders.yaml`, the custom adventurer, or the Board Builder screen. Don't touch the gear work (Phase 2) either. Don't change cards, abilities or other content to improve results.
>
> When you finish (or stop), reply with the final report described at the end of the handoff.

---

## Background

### What exists (confirm before relying on it)

Server paths are relative to `dice-and-destiny-server/`.

| Thing | Where it lives | Notes |
|---|---|---|
| Greedy player | `internal/battle/lookahead/greedy.go` (`NewGreedyPolicy`, about line 81) | `EnergyValue` 0.1, `CardValue` 0.25 and `DrawValue` 0.25. `EnergyValue` prices energy gained **and** energy spent on cards. |
| Tier values | `lookahead/dice.go` `TierValue` (about line 685) and `offensiveTierValues` (about 657) | Damage counts in full, a self-status 0.5 per stack, energy 0.1 per point. These are hard-coded and separate from `EnergyValue`. |
| Exact dice solver | `lookahead/dice.go` `DiceSolver` | Tables are cached per board key, and it solves identical dice only. `Fresh()` values a whole turn. |
| Search | `lookahead/search.go` `SearchPolicy` | Fields: `Budget` futures, `MaxBudget`, `MaxThink`, `TopRerolls` 3, `MinT` 2.5, `Confirm` and `Workers`. |
| Playout cut-off | `search.go` `Rounds` (about line 41); `playout.go` `Playout{Rounds, Evaluate}` (about 18–41) | When `Rounds` > 0, a future stops and is scored by `evaluate` = `tanh(2 × health margin)` (`search.go`, about 364). **This is where the value network plugs in.** |
| Hidden information | `lookahead/sim.go` `Fork` | Clones and reseeds the battle, and re-deals unseen opponent hands. |
| Viewer-safe view | `lookahead/sim.go` `Simulator.View`; `snapshot.FromBattleForViewer` | Opponents' hands show as counts. |
| Effect summaries | `internal/battle/mlsim/encoding_v2.go` `v2OperationSummary` (about line 439) | Folds operations into damage, prevent, status stacks, resource, roll dice, modify and total. |
| Lab runner | `lookahead/lab.go` `RunBattles`; `cmd/balance-lab` | Flags: `-hero`, `-opponent`, `-foe`, `-players`, `-seats`, `-battles`, `-first-seed`, `-workers`, `-budget`, `-rounds`, `-energy-value`, `-card-value`, `-draw-value`, `-bias`, `-out`. There is **no paired-comparison output** yet; the v2 results were paired by hand. |
| In-game opponent | `internal/battle/learned/lookahead_policy.go` | Budget 160, `Workers` = CPUs − 1, `MaxThink` 5 s. The `LookaheadArchitecture` label is written to battle logs. |
| Pure-Go MLP code | `internal/battle/learned/policy_v3.go` (`layer.linear` and activations, about lines 50–72) | A JSON tensor format. Reuse the approach; don't couple to the PPO loaders. |
| Python | `ml/` (`pyproject.toml` with torch, `uv.lock`; `scripts/ml.sh`) | `ml/runs/` and `ml/.venv/` are git-ignored. |
| Logged in-game timing | `dice-and-destiny-client/.godot/runtime/workspace/client/user/phase3/learned-battles.jsonl` | `model_decision.inference_ms`. v2's median is 75 ms, p90 248 ms, max 1.2 s. |

### Baselines already measured (`balance-lab.md`, M3 Max)

| Matchup | Players | Result |
|---|---|---|
| Starter vs Brine Mask, seeds 1–400 | greedy / lookahead | 91.2% / 92.5% |
| Starter mirror against `greedy-v1`, 800 games | `lookahead` v2 | score 0.627 |
| Same | `greedy` v2 | 0.589 |
| Same, paired | look-ahead over greedy | +0.039 ± 0.022 |

The Brine Mask matchup is mostly dice. Expect small differences there.

---

## Ownership

**This thread owns:**
- `internal/battle/lookahead`
- `internal/battle/mlsim`
- `internal/battle/learned/lookahead_policy.go`
- `cmd/balance-lab`, `cmd/card-balance` and `cmd/battle-review`
- `ml/`
- a new `internal/battle/valuenet` package
- `docs/game-design/ai-value-network-plan.md`
- `docs/game-design/balance-lab.md`

**The board thread** will add `content/progression_v1/riders.yaml` to hold
the exchange rates. Until it merges:

- **Decision:** this thread keeps the rates in **one** Go place (a `Rates`
  value in `lookahead`).
- Add a line to the plan's progress notes saying the rates move to
  `riders.yaml` after merge. Don't create `riders.yaml` here.

## Tasks

### T1 — Equal-time benchmark (A0)

Players must be compared at the same cost.

- **A thinking-time budget.** Add `-think-ms N` to `balance-lab` for
  look-ahead players, implemented as a hard per-decision time limit in
  `SearchPolicy`.
  - Futures are spread over candidates as now.
  - A partial round is fine.
  - Keep the existing `-budget` mode.
- **A paired comparison.** Add `-compare a.json,b.json` (or an equivalent).
  Results must hold per-seed, per-seat scores. It reports:
  - the mean difference with its 95% interval and z;
  - wins / draws / losses for each.
- **Control players.** v2 look-ahead with `-rounds 0`, `1` and `2`, using
  today's health-margin evaluator, all at the same `-think-ms`.
  - Pick a think time close to the in-game median; 100 ms is suggested.
  - Use `-workers` so the run fits the machine.
- **Matchups:**
  - Starter mirror (opponent greedy), seats both, 400 seeds;
  - Starter vs Brine Mask, 400 seeds;
  - any other hero mirrors the lab already supports. Report which work.

**Exit:** a baseline table in `balance-lab.md`. No stop rule: this task
always continues.

### T2 — Align the AI's rider rates (A0b)

- **One rates value.** A `Rates` struct in `lookahead`, used by `TierValue`
  and the greedy player. Defaults stay exactly today's values, so nothing
  changes unless asked:
  - self-status per stack 0.5, with per-status overrides (Protect);
  - energy gained;
  - energy spent.
- **Flags:** `-protect-value`, `-status-value`, `-energy-gain-value` and
  `-energy-spend-value`. Keep `-energy-value` as a shortcut that sets both
  energy values.
- **Variants**, each greedy against greedy v2:
  - **(a)** tier values only: Protect 1, energy gained 0.2;
  - **(b)** (a) plus spending energy at 0.2.
- **For each variant:**
  - Starter mirror and Starter vs Brine Mask, 1,000 paired seeds each;
  - report Guarded Strike and Measured Strike picks per battle.
- **Decision:**
  - adopt the best variant as the new defaults if it is no worse in either
    matchup (the lower 95% bound of the paired difference ≥ −0.01);
  - otherwise keep today's defaults and record the result.
  - If adopted, the in-game opponent changes with it. Update
    `LookaheadArchitecture` to a new label and say so in the report.

**Exit:** a results table in `balance-lab.md`. No stop rule.

### T3 — Encoder and recorder (A1)

- **Package `internal/battle/valuenet`.** The `value-v1` encoder reads only
  `Simulator.View` for one seat, as the plan describes:
  - each side's health in each pile, max health, energy and dice;
  - statuses by polarity, stacks, cap, expiry and effect summary;
  - the own hand pooled by card cost, timing and effect summary;
  - the opponent's hand count;
  - each side's solver `Fresh()` value and no-hit chance;
  - each defense's expected prevention;
  - round number and who acts first.

  **No card, ability, status or character ID** may be an input. Export
  `v2OperationSummary` from `mlsim` if you need it, and keep its output
  unchanged. Publish the feature names and a schema hash with the encoder.
- **Recording.** `balance-lab -record <dir>` writes, for each seat at each
  offensive-planning start (before the first roll):
  - the features;
  - the seed and round;
  - the hero and opponent;
  - when the battle ends, the targets: result (1 / 0.5 / 0) and final health
    margin.

  Store as float32 shards plus a JSON header, under
  `ml/runs/value-v1/<dataset>/` (git-ignored).
- **Data:**
  - greedy self-play over every hero and opponent pairing the lab supports,
    at least 8,000 battles;
  - plus look-ahead play (v2, `-budget 96`) for at least 1,000 battles;
  - target about 200k positions.
  - **Keep one hero and one opponent entirely out of training as a held-out
    set**, and name them in the report.
- **Tests:**
  - **Viewer safety:** encode a position, then `Fork` it with several seeds,
    which re-deals the hidden hand and reseeds dice. The features must be
    identical.
  - **Determinism:** the same battle encodes to the same bytes.
  - **Width:** the encoder's width matches its published schema.

**Exit:** the dataset and a short mix report (positions per matchup). No stop
rule.

### T4 — Train and check offline (A2)

- **Training.** `ml/dice_destiny_ml/value_train.py`, with a CLI entry
  `value-train`:
  - an MLP of 2 × 128, ReLU or GELU;
  - two heads: win probability (sigmoid, log loss) and final margin
    (tanh, MSE);
  - a battle-level split, so no battle is in both train and validation.
- **Export:** JSON in the format `dice-and-destiny-value-v1` with:
  - tensors;
  - encoder schema hash;
  - feature count;
  - dataset hashes;
  - training content hash;
  - final metrics.
- **Baselines on the held-out set:**
  - a logistic fit on health margin alone;
  - today's `tanh(2 × margin)` mapped to a probability.

  Report log loss, Brier score and a 10-bin calibration table.
- **Stop rule:** if the network does not beat **both** baselines on log loss
  for the held-out set, stop the handoff here and report. Don't try more
  than three training settings; list the ones you tried.

### T5 — Value network in the search (A3)

- **`valuenet.Load` and `valuenet.Evaluate(b, hero) float64`:**
  - encode the view, run the MLP in pure Go;
  - return 2p − 1, the same scale as today's `tanh` evaluator;
  - the margin head may feed `MarginWeight`.
- **Decision:** the shipped model file is **embedded with `go:embed`** in
  `valuenet` (it should be well under 1 MB), so no path plumbing is needed.
  It is checked by its SHA-256 at load.
- **`SearchPolicy` gets an `Evaluator` field.** When it is nil, behavior is
  exactly v2's. `balance-lab` gets `-value-model <path>` for testing
  unshipped models.
- **Lab test.** Same think time as T1, Rounds 1 and 2, the value player
  against v2 (Rounds 0) and the best T1 control:
  - Starter mirror, seats both, 400 seeds;
  - Brine Mask, 400 seeds;
  - the held-out matchup.
- **Exit:**
  - a paired gain in the Starter mirror with z ≥ 2.5 over **both** v2 and the
    best control;
  - no loss vs Brine Mask, with the lower 95% bound ≥ −0.01;
  - lab ms per decision at in-game settings no worse than v2's.
- **If it passes:**
  - the in-game opponent (`learned/lookahead_policy.go`) uses the value
    player;
  - `LookaheadArchitecture` becomes `flat-monte-carlo-value-v3`;
  - battle logs record the model SHA-256;
  - if the model fails to load, the opponent plays v2 and logs a diagnostic.
    It must never be an error for the player.
- **Stop rule:** if it fails, ship nothing in-game, record the results, and
  continue to T6 only if T4 passed. T6 uses the network offline.

### T6 — Measured rider values (A4)

- **A probe**, either `cmd/value-probe` or a `balance-lab -probe` mode. Over
  the held-out positions it applies one change at a time to the battle
  state, through the engine's own status and resource paths where possible:
  - +1 Protect on self;
  - +5 energy;
  - +1 stack of each self-status and each enemy debuff in the catalog;
  - +1 card in hand (moved from the draw pile);
  - **the unit:** one more card removed from the opponent.

  It encodes the position from the acting seat and reads the value change.
- **Report each change ÷ the unit as a damage-equivalent:**
  - mean, median and interquartile range;
  - split by round (1–2, 3–5, 6+) and by own health (above or below half).
- **Write the table into `balance-lab.md` under "Measured values".**
  - **Do not change any rates or content from it.** The user decides what to
    adopt.
  - Put the 2026-10-15 exchange rates (Protect 1, energy 0.2) next to the
    measured ones.

**Exit:** the table. No stop rule.

## Verification

```bash
cd dice-and-destiny-server && go test ./...
cd .. && ./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd
cd dice-and-destiny-server/ml && uv run pytest
```

Notes:
- If a test already fails on `main`, prove it on a clean `main` checkout and
  say so.
- The `learned` package takes about 8 minutes.
- Long lab runs belong in the background. Record the exact commands in
  `balance-lab.md` so every result can be reproduced.

**Live check** (only if T2 or T5 changed the in-game opponent):

1. Start the game (`./scripts/godot.sh`) and play one battle against the
   look-ahead mirror.
2. From `learned-battles.jsonl`, report:
   - the logged architecture label;
   - the model SHA, if any;
   - the median and maximum `inference_ms` against v2's 75 ms / 1.2 s.

## Out of scope

- A4b (the balance finder), A5 (the policy prior), A6 (tree search and
  self-play).
- Random boards from the board work.
- The board work itself, and `riders.yaml`.
- The gear work.
- Content or card changes.
- Reviving or retraining the PPO policies.
- Pushing or merging.

## Final report

Reply with:

1. a summary and the commit for each task (T1–T6), and where you stopped if
   a stop rule fired;
2. the test commands you ran, with pass/fail results;
3. the T1 baseline table and the chosen think time;
4. the T2 variant results and whether the defaults changed;
5. the dataset mix, and the held-out hero and opponent;
6. the T4 metrics against both baselines, with the training settings tried;
7. the T5 lab results with z-scores and timing, and whether it shipped
   in-game;
8. the T6 measured-values table;
9. anything that contradicted this handoff or `AGENTS.md`, and design choices
   it didn't cover.
