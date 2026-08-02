# Dice and Destiny battle intelligence

This directory contains Phase 2's CPU-first Maskable PPO environment, policies,
training loop, evaluation runners, checkpoint tournament, metrics, and replay
tools. The Python layer never implements battle rules. It owns independent
policy instances, encodes viewer-safe authority results, selects an index from a
masked candidate list, and returns that exact listed command to the real Go
authority through one persistent local simulator process per environment.

## Phase 3 frozen export

Phase 3 uses the accepted seed-11 final checkpoint. The graphical runtime does
not package Python, Torch, Stable-Baselines3, or an inference subprocess. A
reviewed exporter writes only the deterministic candidate-scoring actor tensors
and compatibility metadata to
`../dice-and-destiny-client/models/learned/blade-warden-seed-11-final-v1.json`.
The Go runtime validates the checkpoint/parameter hashes, Phase 1 engine
revision, Phase 2 implementation revision, content hash, schema versions,
dimensions, and tensor shapes before allowing a player battle.

Reproduce the tracked export from `dice-and-destiny-server`:

```bash
./scripts/ml.sh export-policy \
  --checkpoint runs/phase2-training/seed-11/checkpoints/final.zip \
  --output ../dice-and-destiny-client/models/learned/blade-warden-seed-11-final-v1.json \
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

Run every command from `dice-and-destiny-server` through `scripts/ml.sh`. The
wrapper builds the local Go bridge once per invocation and uses the locked Python
environment in `ml/.venv`. Godot is not started.

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
