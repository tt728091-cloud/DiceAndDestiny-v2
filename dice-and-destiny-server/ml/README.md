# Dice and Destiny battle intelligence

This directory contains Phase 2's CPU-first Maskable PPO environment, policies,
training loop, evaluation runners, checkpoint tournament, metrics, and replay
tools. The Python layer never implements battle rules. It owns independent
policy instances, encodes viewer-safe authority results, selects an index from a
masked candidate list, and returns that exact listed command to the real Go
authority through one persistent local simulator process per environment.

## Throughput profiles

Complete-game evaluation and training support explicit resource profiles. The
optimized benchmark keeps the exact authority, complete legal candidates,
frozen model inference, terminal metrics, and selected replays while using the
parity-proven ephemeral repository, compact training telemetry, and the binary
v1 observation transport.

```bash
# Fastest calibrated M3 Max profile: 12 workers, one Torch/BLAS thread each.
./scripts/ml.sh acceptance \
  --seat-a model:runs/phase2-training/seed-11/checkpoints/final.zip \
  --seat-b model:runs/phase2-training/seed-22/checkpoints/final.zip \
  --authority-mode ephemeral --telemetry-mode training \
  --transport-mode encoded --profile max \
  --output runs/throughput-acceptance

# Measured headroom profiles.
./scripts/ml.sh benchmark ... --profile balanced-80
./scripts/ml.sh benchmark ... --profile light-50

# Explicit custom process and thread budget.
./scripts/ml.sh benchmark ... --profile custom --workers 4 --torch-threads 1

# Diagnostic mode sends raw and Go-encoded decisions and fails on any bitwise
# Python/Go schema mismatch. Full mode retains the ordinary viewer-safe JSON.
./scripts/ml.sh benchmark ... --transport-mode parity --telemetry-mode full
```

Named M3 Max profiles resolve to `max=12x1`, `balanced-80=10x1`, and
`light-50=6x1`; the command prints and records the resolved budget. Percentages
are best-effort resource ceilings, not promised fractions of maximum
throughput. Custom budgets that oversubscribe the machine's logical CPU count
are rejected. Recalibrate on a materially different machine or power mode.

## Phase 3 frozen export

Phase 3 uses the accepted seed-11 final checkpoint. The graphical runtime does
not package Python, Torch, Stable-Baselines3, or an inference subprocess. A
reviewed exporter writes only the deterministic candidate-scoring actor tensors
and compatibility metadata to
`../../dice-and-destiny-client/models/learned/blade-warden-seed-11-final-v1.json`.
The Go runtime validates the checkpoint/parameter hashes, Phase 1 engine
revision, Phase 2 implementation revision, content hash, schema versions,
dimensions, and tensor shapes before allowing a player battle.

Reproduce the tracked export from `dice-and-destiny-server`:

```bash
./scripts/ml.sh export-policy \
  --checkpoint runs/phase2-training/seed-11/checkpoints/final.zip \
  --output ../../dice-and-destiny-client/models/learned/blade-warden-seed-11-final-v1.json \
  --model-id blade-warden-maskable-ppo-seed-11-final-v1 \
  --content-version 9eed6066ea8c95f8a60038647de935e88ed8d6e9cbc618229070a4d78945edc4 \
  --source-revision fff39759360ce89041940cec99410082af0671dd \
  --training-engine-revision 5d81e8f6de350a3bdcc3f4ccf42a3447443f83fe
```

Run the Phase 3 both-seat acceptance proxy from the same directory:

```bash
go run ./cmd/phase3-acceptance --battles 100 --output ml/runs/phase3-acceptance
```

The acceptance runner records every human-proxy and learned-policy authority
command, seeds, seats, results, per-decision inference latency, battle duration,
and all required failure counters. Its output remains ignored with other run
artifacts.

## Phase 2 decision-quality candidate

The selected Phase 2 checkpoint is the seed-22 mechanics-aware policy at
`runs/phase2-decision-quality-20260802-m3max-v2/selection/tactical-finetune/seed-22-lr1e-4-e60/corrective-cloned.zip`.
Its separate v2 export is pinned at
`../../dice-and-destiny-client/models/learned/blade-warden-decision-quality-seed-22-v2.json`
with SHA-256
`0e6ea5d84c316a7709c9c1b98b983e8f76e486c6e1625c479c55d2860bd23a86`.
The accepted v1 export remains the default graphical opponent.

From the repository root, launch the game normally and choose one of the
explicit `New v2` opponent entries for owner graphical acceptance:

```bash
./scripts/godot.sh
```

Both old-v1 and new-v2 opponents are available for Human Seat A and Human Seat
B. Selection is intentionally limited to fixed model-and-hash entries, not an
arbitrary path. The `DICE_AND_DESTINY_PHASE2_DECISION_MODEL=1` environment flag
remains available for automated/direct-runtime compatibility but is not needed
for the menu. Verify the native graphical selection without replacing the v1
integration:

```bash
./scripts/godot.sh --headless --script res://tests/phase3/verify_learned_battle_v2.gd
./scripts/godot.sh --headless --script res://tests/phase3/verify_learned_battle.gd
```

## Optimized 5M v3 opponent

The finalized optimized seed-22 policy is exported separately at
`../../dice-and-destiny-client/models/learned/blade-warden-optimized-5m-seed-22-v3.json`
and pinned with SHA-256
`529a6b4d6ad347d5ba86b5e000cb5fceec306414cdf0af3405713a2bc5c32ebb`.
It uses the same v2 observation/action contract as the decision-quality model.
The local battle menu exposes it as `Strongest v3` for Human Seat A and Human
Seat B while preserving both earlier opponents.

## Current global champion opponent

Winner-health-v2 checkpoint 193 at 3,603,456 PPO steps is the current global
champion. It defeated checkpoint 38 by 1,681/226/1,093 in a fresh, seat-swapped
3,000-game title match (59.80% adjusted score). Its source checkpoint SHA-256 is
`804fff1caaf5ddf15e4c8714d9c2d0b2563681ddb6f61477e832e5afde93c010`.
The pinned gameplay export is
`../../dice-and-destiny-client/models/learned/blade-warden-global-champion-cp193-winner-health-v2.json`
with SHA-256
`cd3d7451e91d071274c874b017e3c7e73358862090fd71954e282e5bf505b814`.
The menu exposes it as `Global Champion CP193` for either human seat. CP38 and
CP480 are retained separately as prior global champions, and every older export
remains playable. The canonical forward-looking registry is
`champions/current-global.json`.

Reproduce the current champion export from `dice-and-destiny-server`:

```bash
./scripts/ml.sh export-policy \
  --checkpoint /absolute/path/to/ml/runs/v2-winner-health-v2-cp35-continuation-cp38-20260806-8h-1k-only/intervals/interval-193/raw-ppo-challenger.zip \
  --output /absolute/path/to/dice-and-destiny-client/models/learned/blade-warden-global-champion-cp193-winner-health-v2.json \
  --model-id blade-warden-global-champion-cp193-winner-health-v2 \
  --content-version 9eed6066ea8c95f8a60038647de935e88ed8d6e9cbc618229070a4d78945edc4 \
  --source-revision 1a8aa21c7ed020a0a4482ebab734b256e6f64609 \
  --training-engine-revision 1a8aa21c7ed020a0a4482ebab734b256e6f64609 \
  --policy-family v2
```

### Reversible global hill climb

The `global-hillclimb-*` commands preserve the complete current global model
and optimizer, train exactly one requested 50,000-step interval (52,224 actual
rollout-aligned steps), and run one fresh 1,000-game seat-swapped gate against
that current global. A challenger passes only when its draw-adjusted score is
strictly above 50% and every safety counter is zero. A pass is promoted
atomically and becomes the next interval's parent. A failure is preserved as an
artifact, but both weights and optimizer roll back to the last accepted global
before another RNG seed is tried.

The checkpoint-480 observation/action contracts, network, reward, PPO recipe,
opponent mix, historical selection, curriculum, teacher frequency, and seat
schedule are frozen for the entire campaign. Only predeclared training RNG
seeds and disjoint evaluation seed banks vary. No corrective update runs after
PPO. `global-hillclimb-preflight` verifies the source recipe, exact initial
checkpoint, optimizer, 1,002 historical checkpoints, seed manifest, and frozen
controls before the campaign timer starts. Each decision is appended to a
hash-chained history, and a failed checkpoint can never become a future parent.

Run every command from `dice-and-destiny-server` through `scripts/ml.sh`. The
wrapper builds the local Go bridge once per invocation and uses the locked Python
environment in `ml/.venv`. Godot is not started.

### Winner-health reward for new training families

New `train` runs default to the versioned `winner-health-v2` terminal PPO
reward. A learner win uses the learner's remaining-health fraction; a learner
loss uses the opposing winner's remaining-health fraction:

```text
learner win:  +0.5 + 1.0 * learner_remaining_health / learner_max_health
draw:          0
learner loss: -0.5 - 1.0 * opponent_remaining_health / opponent_max_health
```

The health fraction is clamped to `[0, 1]`, making the reward zero-sum with a
range of `[-1.5, +1.5]`. It is applied only at a terminal battle. Evaluation
and champion promotion remain pure win/draw/loss scoring (`1`, `0.5`, `0`).
Use `--reward outcome-only-v1` only when intentionally reproducing a historical
recipe; phase-2 matrix runs read and preserve their recorded reward explicitly.
Use `--reward winner-health-v1` to reproduce CP38's earlier 0.75 outcome plus
0.75 health-coefficient recipe.

## Versioned contract

- Environment: `dice-and-destiny-ml-env-v1`
- Observation: `dice-and-destiny-observation-v1`
- Actions: `dice-and-destiny-action-candidates-v1`
- Capacity: 128 deterministic authority candidates; unused positions are zero
  padded and masked invalid.
- Reward: +1 victory, -1 defeat, 0 draw. There is no reward shaping.
- Safety truncation: 1,200 authority actions (`action_limit`). Acceptance runs
  require zero truncations.

`reset(seed, seat_models)` creates a new in-memory Go authority and fresh Blade
Warden mirror battle. The simulator process remains alive, but the prior battle,
pending inputs, rewards, policy episode state, and action trace are discarded.
`observe(seat_id)` and `legal_actions(seat_id)` return the real viewer-filtered
authority result. `step(action_index)` submits the original listed command
unchanged. Terminal responses include winner, metrics, and a replay record.

### Observation v1 (4,224 float32 values)

The first 128 values contain seat identity; public round, segment, stage and
priority; both actors' public health/resources/zone counts; the viewer's private
hand/card definitions and planning dice; revealed opponent dice/planning state;
status buckets; selected/qualified ability buckets; and terminal flags. Stable
SHA-256 buckets encode authored IDs. The encoder reads no object except the
viewer-safe snapshot returned for the acting seat.

The remaining `128 * 32` values describe candidates by command type, pass/roll
role, self/opponent targets, dice subsets, card and ability buckets, revealed
die adjustments, and a stable payload bucket. Each valid row corresponds to the
same-index original authority command. The mask is true only for populated rows.
The masked PPO actor uses shared candidate-scoring weights, so a command's score
does not depend on its padded row position. A separate value MLP consumes the
whole observation.

## Required runners

```bash
# 1. Environment smoke and deterministic replay
./scripts/ml.sh smoke --seed 20260801

# 2. Random versus random
./scripts/ml.sh evaluate --seat-a random --seat-b random --swap --episodes 100 --output runs/random-v-random

# 3. Heuristic versus random
./scripts/ml.sh evaluate --seat-a heuristic --seat-b random --swap --episodes 100 --output runs/heuristic-v-random

# 4. Three-seed training
./scripts/ml.sh train --seeds 11 22 33 --timesteps 20000 --workers 4 --output runs/training

# 5. Model versus random
./scripts/ml.sh evaluate --seat-a model:runs/training/seed-11/checkpoints/final.zip --seat-b random --swap --episodes 500 --output runs/model-v-random

# 6. Model versus heuristic
./scripts/ml.sh evaluate --seat-a model:runs/training/seed-11/checkpoints/final.zip --seat-b heuristic --swap --episodes 500 --output runs/model-v-heuristic

# 7. Two separately specified model artifacts (add --swap for matched seats)
./scripts/ml.sh evaluate --seat-a model:/path/a.zip --seat-b model:/path/b.zip --swap --episodes 100 --output runs/model-v-model

# 8. Checkpoint tournament
./scripts/ml.sh tournament --checkpoints /path/initial.zip /path/middle.zip /path/final.zip --episodes 100 --output runs/tournament

# 9. Recorded battle replay
./scripts/ml.sh replay runs/model-v-random/replays/eval-0-9000000.json

# 10. Performance benchmark
./scripts/ml.sh benchmark --seat-a random --seat-b random --episodes 200 --output runs/benchmark

# Mandatory 1,000-game same/different-checkpoint soak (500 matched seeds x 2 seats)
./scripts/ml.sh acceptance --seat-a model:/path/a.zip --seat-b model:/path/b.zip --output runs/acceptance

# Generate plots and the artifact index
./scripts/ml.sh report --evaluations /path/to/evaluation-a /path/to/evaluation-b --output runs/report
```

Every evaluation writes `statistics.html`. Champion-gated campaigns also write
`checkpoint-statistics.html` beside each evaluated checkpoint and compare the
challenger with its frozen global-champion opponent, previous checkpoint, and
family best. The campaign root writes both the combined `statistics.html`
overview and a standalone `gameplay-statistics.html` sheet. It also preserves
the same values in `checkpoint-metrics.json` and registers the required report
artifacts in `metrics-artifacts.json`, so completed runs can be compared without
replaying evaluations. The checkpoint and gameplay pages include offensive ability-versus-pass rates
both before and after the offensive reaction window; first-, second-, and
third-roll ability-selection percentages; reaction-caused ability preservation,
loss, gain, or switching; and authoritative per-round attack, bleed, poison,
resolved, and actual damage dealt and taken.

The untrained `initial.zip` checkpoint is saved first. Training then uses a
documented heuristic-demonstration warm start (real authority observations and
listed actions, no invented commands) before terminal-reward PPO. This prevents
an untrained deterministic network from cycling indefinitely on legal no-op keep
actions; it is imitation pretraining, not reward shaping, and is reported
separately from PPO optimizer updates.

Training episodes are marked `training=true` and feed only the fixed learner
seat's experience to PPO; evaluation uses the separate runner and never calls
an optimizer. Learner seat assignment alternates by episode. Opponents are drawn
reproducibly from random, heuristic, and frozen saved checkpoints, avoiding two
exclusively simultaneous moving policies. Each checkpoint is a fresh inference
instance; identical-policy mirrors still instantiate the artifact twice.
