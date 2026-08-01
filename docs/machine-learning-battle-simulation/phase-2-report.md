# Phase 2 battle-intelligence completion report

Date: 2026-08-01

Status: **PASS — all mandatory Phase 2 acceptance categories A–G pass.**

Accepted Phase 1 baseline: `5d81e8f6de350a3bdcc3f4ccf42a3447443f83fe`

This report covers the first reusable learning and self-play system for Blade
Warden mirror battles. It intentionally stops at owner review. Nothing has been
committed or pushed, and no roster balancing, production enemy integration,
new content, networking, or Phase 3 work is included.

## Architecture implemented

`cmd/battle-ml-sim` is a persistent JSON-lines bridge around the real Go battle
authority. `reset` creates a fresh in-memory repository, a fresh authority, and
a fresh Blade Warden versus Blade Warden battle. The bridge process survives
actions and episodes; Godot and networking are absent from the training loop.

The versioned contract is:

- Environment: `dice-and-destiny-ml-env-v1`
- Observation: `dice-and-destiny-observation-v1`
- Action candidates: `dice-and-destiny-action-candidates-v1`
- Observation shape: 4,224 `float32` values (128 viewer-state values plus 128
  candidate rows of 32 features)
- Action capacity: 128 deterministically ordered, complete authority commands;
  unused rows are zero-padded and masked
- Reward: +1 victory, -1 defeat, 0 draw, with no reward shaping
- Safety limit: 1,200 authority actions; a limit hit is a reported truncation

Only the viewer-safe authority result for the acting seat reaches the encoder.
It contains public battle state and that viewer's permitted private state. It
does not read the opponent's hand, hidden planning selection, private dice,
unrevealed commitments, or future random state. The policy chooses an index in
the enumerated list. The bridge submits the corresponding original command
unchanged, and the Go authority remains the only state mutation boundary.

Evaluation creates one policy object for Seat A and a second policy object for
Seat B. The objects may load the same artifact for a mirror or different
artifacts for cross-play. Matched-seed evaluation swaps assignments. No policy
shares recurrent or episode state (the v1 policy itself is feed-forward).

## Algorithm, experience flow, and training budget

The learner is CPU-first Maskable PPO. The finite, variable legal-command set
makes explicit invalid-action masking a natural fit, while PPO provides a small,
stable on-policy optimizer. A shared candidate scorer prevents padded row
position from becoming an action identity: context MLP `[64,64]`, candidate MLP
`[64,64]`, score MLP `[64,1]`, value MLP `[128,128]`.

Each of four persistent vector environments resets through the authority,
alternates the learner seat, and samples reproducibly from random, heuristic,
and frozen historical opponents. Learner decisions add viewer observation,
mask, selected index, terminal reward, and completion metadata to the PPO
rollout. After each 256-step worker rollout, PPO consumes the batch for eight
epochs. Evaluation uses a separate path that never creates an optimizer or
updates weights.

The raw untrained actor often selects the legal but non-progressing planning
`keep` action. A separately measured behavior-cloning warm start therefore uses
5,000 heuristic decisions collected from real authority episodes, followed by
terminal-reward PPO. This is documented imitation pretraining, not reward
shaping. Random baseline policy remains random over progress-capable legal
commands when possible, while the complete candidate set remains visible.

Three independent seeds (11, 22, 33) each requested 20,000 steps and completed
20,480 vector-aligned PPO steps, 160 PPO optimizer updates, and 5,000 imitation
examples. Across all seeds that is 61,440 PPO steps, 480 PPO updates, 15,000
imitation examples, and 585 completed training episodes. Training elapsed
113.91 s, 110.68 s, and 111.79 s respectively (179.8–185.0 steps/s).

Each seed produced `initial`, `behavior-cloned`, 5k, 10k, 15k, 20k, and `final`
checkpoints. Parameter hashes changed from initial to final for all three seeds.
Final hashes are:

- Seed 11: `b53c6633893bbaca6dd429edb6f1aeb94585e6c24cf05c2dbc1451e2fd758f7a`
- Seed 22: `695b864e28964907b4ecb81a14082ec6ddf1b2089034117c0412573438842181`
- Seed 33: `4bfe13ce1082015715af684bc9d0d21f495055f3de60382e30547f2d22a188d4`

## Evidence of learning

All following evaluations were deterministic inference with updates disabled.
Held-out seed ranges 9,700,000, 9,800,000, and 9,900,000 were disjoint from the
training seed construction, and each result used 50 matched seeds with both
seat assignments (100 games).

| Final checkpoint | Wins vs random | 95% Wilson interval | Seat A / Seat B wins | Safety |
| --- | ---: | ---: | ---: | --- |
| Seed 11 | 100/100 | 96.30%–100% | 50 / 50 | 0 rejects, invalids, stale, wrong-seat, truncations |
| Seed 22 | 100/100 | 96.30%–100% | 50 / 50 | 0 rejects, invalids, stale, wrong-seat, truncations |
| Seed 33 | 99/100 | 94.55%–99.82% | 51 / 49 | 0 rejects, invalids, stale, wrong-seat, truncations |

Every lower confidence bound is above 50%. The transparent heuristic also beat
the corrected random baseline 100/100 (95% Wilson lower bound 96.30%). Against
the heuristic, seed 11 final scored 51 wins, 43 losses, and 6 draws; its raw win
interval was 41.35%–60.58%, so this run shows competitive performance but does
**not** prove superiority to the heuristic.

The initial policy failed to progress and hit the 1,200-action safety limit; its
truncation is attributed to the initial actor and scored as a loss, never as a
draw. This prevents a direct terminal win-rate comparison, but is itself a
material benchmark failure. In contrast, final policies completed every
held-out baseline and acceptance episode. On a terminal cross-play benchmark,
seed 11 final beat its learned 5k checkpoint 70–24 with 6 draws (adjudicated
score 73% versus 27%).

The seed 11 checkpoint tournament ratings increased from initial 1368.88 to
behavior-cloned 1495.77, 5k 1564.01, and final 1571.34. In the tournament matrix
the final checkpoint scored 0.80 against behavior-cloned, 1.00 against initial,
and 0.55 against 5k. The larger final-vs-5k run above supplies the stronger
later-checkpoint comparison. These held-out outcomes, not weight changes or
loss alone, are the learning claim.

## Self-play and seat bias

Two independent seed 11 final instances completed 100 mirror games: Seat A 48,
Seat B 48, draws 4, truncations 0. Among decisive games Seat A was 50.0% with a
95% Wilson interval of 40.19%–59.81%.

Different final artifacts also work: seed 22 beat seed 11 by 59–36 with 5 draws
in a 100-game swapped run. The small run had Seat A 37 and Seat B 58 wins. The
larger acceptance run is the primary seat-effect measurement: among its 956
decisive games Seat A won 497 (51.99%, 95% Wilson 48.82%–55.14%). Because this
interval includes 50%, Phase 2 found no statistically measurable initiative or
reaction-priority seat advantage. This is not proof that the effect is exactly
zero; more content and larger targeted experiments remain appropriate.

## 1,000-battle acceptance soak

One persistent evaluation process ran seed 11 final versus seed 22 final over
500 matched seeds and both seat assignments:

- 1,000/1,000 episodes recorded, with unique battle IDs and recorded seeds
- Seed 11: 493 wins; seed 22: 463 wins; draws: 44
- Seat A: 497 wins; Seat B: 459 wins; draws: 44
- Crashes: 0; authority rejects: 0; invalid indices: 0
- Stale submissions: 0; wrong-seat submissions: 0; truncations: 0
- Mean/median/p95 duration: 202.27 / 199.5 / 278 authority actions
- Mean remaining health: 3.2445
- Elapsed: 541.30 s; throughput: 1.8474 games/s
- Mean/p95 inference latency: 0.4683 / 0.6938 ms

The run's machine-readable checker returned `acceptance_passed: true` with an
empty failure list. The 1,000 resets occurred in one automated invocation; each
reset replaced the repository, authority, battle, pending input, metrics, and
trace without restarting the process.

## Reproducibility and artifacts

The run used an Apple M3 Max (16 CPU cores, 64 GB RAM), macOS 26.5.2, CPU
inference/training, Python 3.12.7, Torch 2.13.0, Stable-Baselines3 2.9.0,
sb3-contrib 2.9.0, Gymnasium 1.3.0, and NumPy 2.5.1. `uv.lock` pins the complete
Python graph. Per-seed `config.json` records source revision/dirty state, content
hash, schemas, architecture, exact hyperparameters, seed rules, opponent pool,
reward, packages, and runtime. `ml/configs/phase-2.json` records the aggregate
training and evaluation design.

The same seed-104 reproducibility probe was run twice (1,000 imitation examples,
512 PPO steps, four updates). Both runs produced the identical initial hash
`5d0cf208...ca1ac6`, final hash `28bd8d1d...93faed`, imitation loss
0.8149033785, accuracy 0.89, and PPO statistics; only wall-clock time differed.
Deterministic reset and replay are additionally covered by Go and Python tests.

Primary artifact locations (relative to `dice-and-destiny-server`):

- Checkpoints/config/curves source data: `ml/runs/phase2-training/seed-{11,22,33}`
- Acceptance summary and all episodes: `ml/runs/phase2-acceptance`
- Evaluation summaries: `ml/runs/phase2-evaluation` and `ml/runs/phase2-evaluation-v2`
- Tournament matrix/ratings: `ml/runs/phase2-tournament/tournament.json`
- Training curves: `ml/runs/phase2-report/training-curves.png`
- Checkpoint rating: `ml/runs/phase2-report/checkpoint-ratings.png`
- Baseline confidence plot: `ml/runs/phase2-report/baseline-win-rates.png`
- Duration/health distributions: `ml/runs/phase2-report/battle-distributions.png`
- Action frequency: `ml/runs/phase2-report/action-frequency.png`
- Machine-readable artifact index: `ml/runs/phase2-report/artifact-index.json`
- Successful terminal replay: `ml/runs/phase2-acceptance/replays/eval-0-9000005.json`
- Surprising draw replay: `ml/runs/phase2-replay-draw/replays/eval-0-9000008.json`
- Failure replay (untrained non-progress truncation):
  `ml/runs/phase2-replay-failure/replays/eval-0-9650000.json`
- Exact reproducibility pair: `ml/runs/repro-a` and `ml/runs/repro-b`

The acceptance action-frequency totals were: pass 89,645,
planning-select-ability 37,056, roll-dice 33,168, planning-roll 20,750,
commit-interaction 14,226, planning-commit-cards 2,718, planning-reroll 2,486,
and planning-pass 2,222.

## Exact reproduction commands

Run from `dice-and-destiny-server`; every command uses the locked environment
and persistent Go bridge:

```bash
./scripts/ml.sh smoke --seed 20260801
./scripts/ml.sh evaluate --seat-a random --seat-b random --swap --episodes 100 --output runs/random-v-random
./scripts/ml.sh evaluate --seat-a heuristic --seat-b random --swap --episodes 100 --output runs/heuristic-v-random
./scripts/ml.sh train --seeds 11 22 33 --timesteps 20000 --workers 4 --output runs/phase2-training
./scripts/ml.sh evaluate --seat-a model:runs/phase2-training/seed-11/checkpoints/final.zip --seat-b random --episodes 50 --seed-start 9700000 --swap --output runs/phase2-evaluation-v2/seed-11-final-v-random
./scripts/ml.sh evaluate --seat-a model:runs/phase2-training/seed-11/checkpoints/final.zip --seat-b heuristic --episodes 50 --seed-start 9200000 --swap --output runs/phase2-evaluation/seed-11-final-v-heuristic
./scripts/ml.sh evaluate --seat-a model:runs/phase2-training/seed-11/checkpoints/final.zip --seat-b model:runs/phase2-training/seed-11/checkpoints/final.zip --episodes 50 --seed-start 9400000 --swap --output runs/phase2-evaluation/seed-11-final-mirror
./scripts/ml.sh evaluate --seat-a model:runs/phase2-training/seed-11/checkpoints/final.zip --seat-b model:runs/phase2-training/seed-11/checkpoints/step-000005000.zip --episodes 50 --seed-start 9300000 --swap --output runs/phase2-evaluation/seed-11-final-v-step-5000
./scripts/ml.sh tournament --checkpoints runs/phase2-training/seed-11/checkpoints/initial.zip runs/phase2-training/seed-11/checkpoints/behavior-cloned.zip runs/phase2-training/seed-11/checkpoints/step-000005000.zip runs/phase2-training/seed-11/checkpoints/final.zip --episodes 5 --output runs/phase2-tournament
./scripts/ml.sh acceptance --seat-a model:runs/phase2-training/seed-11/checkpoints/final.zip --seat-b model:runs/phase2-training/seed-22/checkpoints/final.zip --output runs/phase2-acceptance
./scripts/ml.sh replay runs/phase2-replay-draw/replays/eval-0-9000008.json
./scripts/ml.sh benchmark --seat-a random --seat-b random --episodes 200 --output runs/phase2-random-soak
```

Exact training-reproducibility probe (run twice with different output roots):

```bash
./scripts/ml.sh train --seeds 104 --timesteps 512 --workers 1 --rollout-steps 128 --batch-size 128 --epochs 1 --imitation-decisions 1000 --imitation-epochs 3 --checkpoint-interval 256 --output runs/repro-a
./scripts/ml.sh train --seeds 104 --timesteps 512 --workers 1 --rollout-steps 128 --batch-size 128 --epochs 1 --imitation-decisions 1000 --imitation-epochs 3 --checkpoint-interval 256 --output runs/repro-b
```

## Mandatory acceptance checklist

| Category | Status | Evidence |
| --- | --- | --- |
| A. Two-model gameplay | **PASS** | Separate same/different checkpoint instances, complete terminal battles, fixed/swapped seats, no human input |
| B. Repeated automated episodes | **PASS** | One 1,000-game process; all records/IDs/seeds present; zero crashes, rejects, stale, wrong-seat, invalid, or unexplained truncations |
| C. Actual learning | **PASS** | 3 seeds, real-authority experience, 480 PPO updates, changed parameters, initial/intermediate/final checkpoints, all held-out random lower CIs >50%, final beats 5k, tournament rises, heuristic measured |
| D. Self-play evidence | **PASS** | 100-game independent final mirror, different-final and final-vs-early cross-play, matched swaps, seat/draw/truncation/CIs reported |
| E. Reproducibility | **PASS** | Revision/content/schemas/architecture/config/seeds/pool/reward/checkpoints/eval seeds/packages/hardware captured; identical-seed probe matches exactly |
| F. Metrics and artifacts | **PASS** | Curves, ratings/matrix, baselines, mirror, distributions, health, actions, safety, throughput, latency, replays, checkpoints, JSON config/index, this report |
| G. Regression safety | **PASS** | Full Go tests/vet/race, ML tests/lint/lock, Godot authority verification, formatting/diff checks all pass; authority-only mutation maintained |

## Remaining limitations and risks

- This is one Blade Warden mirror matchup; it is not evidence of roster-wide
  balance or production-grade enemy behavior.
- Seed 11 was competitive with, but did not statistically beat, the heuristic.
- The untrained deterministic policy can loop on legal `keep`; safety attribution
  and imitation warm start handle it, but a future schema could represent action
  progress more directly.
- The 128-candidate and 1,200-action caps are versioned v1 limits. New content
  must verify those bounds before reuse.
- Training uses four vectorized persistent environments in one Python process,
  not multi-process simulation. Authority throughput, not MPS compute, is the
  present bottleneck.
- The seat-effect interval rules out only large effects in this matchup and
  sample. Targeted reaction/initiative experiments may still find small effects.
- Run artifacts and checkpoints are intentionally ignored by Git because of
  their size; preserve or publish them separately before cleaning the workspace.
