# Phase 2 PPO training-throughput optimization report

Date: 2026-08-03
Branch: `codex/phase2-training-throughput-optimization`
Baseline revision: `918a4241d98ed39801c7b591361a24fd305a8e8f`
Status: **owner review required; do not commit, push, or start a final model campaign**

## Result

The unchanged-population, unchanged-PPO v2 training recipe improved from
**320.8213 to a three-seed median 715.4446 learner steps/s**, or **2.2300×**.
Complete games/s improved from **3.4669 to a three-seed median 7.0375**, or
**2.0299×**. The selected seed-22 run reached 719.0970 steps/s and completed
the same 202,752 authority timesteps and 528 optimizer updates. Its collection
time fell from 313.454 to 219.751 seconds and PPO update time fell from 317.945
to 61.631 seconds. Across all three full runs there were no truncations,
authority rejects, invalid indices, stale submissions, or wrong-seat
submissions.

The 3× stretch was not reached. The late-run plateau is honest: the original
flat population makes every published historical checkpoint a separate member.
A bounded eight-entry per-worker LRU prevents unbounded live cache growth, but
uniform draws from an increasingly large history cause late checkpoint misses.

The corrected category-balanced population was implemented and measured as a
separate learning ablation. It reached 811.4762 steps/s for seed 22, but its
three-seed heldout tactical/broad screen regressed. It is retained behind
`--opponent-selection-mode category-balanced` and is **not** the default or the
engineering throughput claim.

## Scope and preserved contracts

The implementation retains the real Go authority, exact rules/RNG, all legal
candidates and original-command submission, viewer filtering, the final v2
progress mask, terminal +1/-1/0 rewards, terminal summaries, replay evidence,
the accepted v1/v2 artifacts, and Python/export/Go inference paths. PPO remains
256 rollout steps per worker, batch size 256, eight epochs, learning rate
`3e-4`, gamma `.995`, GAE lambda `.95`, entropy coefficient `.01`, 5,000
mechanics-v2 imitation decisions, five imitation epochs, 12 max-profile
collectors, full transport, and the declared random/mechanics/accepted/history
population.

No result in this report declares a new gameplay model accepted. The selected
decision-quality seed-22 checkpoint and both shipped exports remain untouched.

## Artifacts and host

Ignored evidence root:

`dice-and-destiny-server/ml/runs/phase2-throughput-optimization-20260803-m3max-v2`

The host was an Apple M3 Max Mac15,9 with 16 logical CPUs and 64 GiB RAM,
macOS 26.5.2, Python 3.12.7, Torch 2.13.0, SB3/sb3-contrib 2.9.0, and AC power.
All reference and selected runs reported no macOS thermal or performance
warning. Swap remained 1040.75 MiB used across the measured runs.

Reverified retained hashes:

| Artifact | SHA-256 |
| --- | --- |
| Accepted v1 checkpoint | `e2e98c2ecadfe9e06892676962e751a116cb07183aa193882d73927866d760b8` |
| Selected corrected v2 checkpoint | `e3b9fb8c393f17c6d7a70a563a55e2596e83651f16c9e0fa226096b082f772e2` |
| Accepted v1 export | `dea4a6681fcd12d368dee6e720bb0742643a25dd46366080ab4e79fba59e3937` |
| Selected v2 export | `0e6ea5d84c316a7709c9c1b98b983e8f76e486c6e1625c479c55d2860bd23a86` |
| ML lock | `2a0f2aa2c52170969a65b3999b50c008116c22971cfc7261c2fc6cfd64626588` |

## Baseline reproducibility

Three exact 21,504-step current-code references reproduced the same final
parameter hash, episode trajectory, imitation result, and zero safety counts:

| Run | Steps/s | Games/s | Wall s | Collection s | PPO update s |
| ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 312.8315 | 3.2005 | 68.7399 | 34.4345 | 33.7276 |
| 2 | 313.5162 | 3.2075 | 68.5898 | 34.3187 | 33.6961 |
| 3 | 312.9931 | 3.2021 | 68.7044 | 34.3978 | 33.7230 |
| Median | **312.9931** | **3.2021** | **68.7044** | **34.3978** | **33.7230** |

Final parameter SHA-256 for all three was
`82b31604d6f8c20f5f84220b518dc8ce8ae8b134f0bf4948f4e0a4943b4a4834`.

The exact full reference was:

| Metric | Current-code full |
| --- | ---: |
| Requested/completed steps | 200,000 / 202,752 |
| Learner steps/s | **320.8213** |
| Complete games/s | **3.4669** |
| Episodes / updates | 2,191 / 528 |
| Wall / collection / update s | 631.978 / 313.454 / 317.945 |
| Final parameter hash | `8ed12ca9f75184997a6a7f8793bf01a2f92a1b46808b4e1d7c8793f99a77d8f0` |
| Config SHA-256 | `102643edebf381de9752cda0a7caca6a13e27be78f1c3086898bb1f430d27fad` |
| Summary SHA-256 | `8a52601ba37b53bd8ff49cf71a8626361dabf9f2241aaba0341e2b0906a6d661` |
| Replay SHA-256 | `0e637d759d805985543f75551c624f3649cf012d05164ea175f8f5076dd3f2cd` |

Its episode population was 122 random, 129 mechanics-v2, 121 accepted v1,
and 1,819 historical. The difference from the older handoff measurement of
309.6875 steps/s and 1.9826 games/s is disclosed: current progress-mask code
completes more games per second. Engineering multipliers use the freshly frozen
current-code reference.

## Profiling before hot-path changes

Opt-in instrumentation reproduced the exact short-run final parameter hash at
311.0874 steps/s, only 0.6% below the uninstrumented median.

Main-process timings for 21,504 steps:

| Path | Aggregate seconds |
| --- | ---: |
| Vector-environment wait | 29.129 |
| Rollout-buffer add | 0.180 |
| GAE | 0.006 |
| PPO evaluate-actions | 22.695 |
| PPO backward | 7.753 |
| Optimizer step | 1.569 |
| Gradient clip | 0.741 |

The dense action-logit/distribution path consumed 21.604 of 33.727 PPO-update
seconds. Candidate observations were only 1.997% nonzero. There were 20,648
candidate decisions with 230,742 valid candidates, mean 11.176 of 128 slots.
Every spawned worker reported Torch intra-op 12, inter-op 16, and unset BLAS
limits even though the parent was configured for one thread.

Standard profiles corroborated the nested timers:

- Python cProfile: pipe reads, dense `CandidateActionNetV2.forward`, `tanh`,
  schema-v2 construction, backward, and mechanics scoring dominated.
- PyTorch profiler: `tanh` on `[256,128,96]` tensors used 50.06% of CPU
  self-time; candidate concatenation/activations allocated hundreds of MiB per
  minibatch.
- Go benchmark: 7.7499 ms/battle, 11,408,509 B/battle, 44,558 allocs/battle.
  State cloning, canonical action construction, JSON, snapshots, and repository
  event copies led allocation space.
- macOS: no thermal/performance warning and no swap growth.

Profiles are retained under `profiles/python-current-code`,
`profiles/torch-current-code`, and `profiles/go-current-code`.

## Kept engineering changes

### Explicit resources

Learner intra-op, learner inter-op, learner BLAS, worker intra-op, worker
inter-op, and worker BLAS controls are separate. Spawned workers configure all
three limits before policy inference. The named M3 Max max profile resolves to
12 workers at one intra/inter/BLAS thread each and a four-thread/one-inter-op
learner. `--torch-threads` remains a backward-compatible custom-profile
fallback.

The sparse learner-thread microbenchmark selected four threads:

| Learner threads | PPO-like updates/s |
| ---: | ---: |
| 1 | 91.05 |
| 2 | 104.79 |
| 4 | **115.23** |
| 6 | 114.11 |
| 8 | 109.22 |

The no-imitation collector sweep was:

| Workers | Steps/s | Games/s |
| ---: | ---: | ---: |
| 6 | 567.96 | 4.38 |
| 8 | 630.76 | 5.03 |
| 10 | 631.01 | 5.32 |
| 12 | **637.77** | **5.50** |

### Opponent manager and RNG contract

Each worker owns long-lived random and mechanics policies, a permanent accepted
model cache, and an eight-entry historical LRU. Checkpoint directories are
expanded into one immutable manifest per episode. Model sampling uses a private
CPU Torch generator seeded from episode seed XOR a seat salt, so policy object
lifetime and global Torch state do not choose opponent actions. Every terminal
record retains exact policy identity.

Checkpoints are written in a hidden staging directory and published with one
same-filesystem `os.replace`; `*.zip` manifests therefore cannot observe a
partial checkpoint. The legacy mode performs the original single uniform draw
from declared static members plus every visible historical checkpoint.
Category-balanced mode first draws one nonempty category uniformly and then a
member uniformly; it is non-default pending owner-quality work.

In the instrumented short legacy run, thread isolation preserved the exact
prechange parameter hash and improved 311.09 to 317.76 steps/s. Opponent caching
then raised throughput to 353.22 steps/s and cut collection from 33.34 to 26.57
seconds. Persistent mechanics outcome tables reduced aggregate mechanics
inference from 6.26 to 2.03 seconds.

### Versioned sparse v2 actor

`SparseCandidateMaskablePolicyV2` keeps the public 128-logit action API and the
same actor tensors, but embeds/scores only rows selected by the legal-action
mask and scatters valid scores back into the padded tensor. The sparse route is
used by rollout `forward`, PPO `evaluate_actions`, prediction/
`get_distribution`, and behavior cloning. Dense v1/v2 classes remain loadable.

Tests compare dense/sparse valid logits, probabilities, deterministic actions,
seeded samples, log-probabilities, entropy, values, actor/critic gradients,
all-128 capacity, and empty masks. Declared tolerances are `rtol=1e-6,
atol=1e-7` for forward probability/logit quantities and `rtol=2e-5,
atol=2e-6` for gradients. Old v1, old dense-v2, and new sparse-v2 checkpoints
load under three distinct policy classes.

The sparse short profile reduced behavior cloning from 4.95 to 0.72 seconds,
PPO update from 33.72 to 8.28 seconds, and update action logits from 21.66 to
2.53 seconds. A four-thread learner then reduced PPO update to 6.73 seconds.

## End-to-end selected result

The selected engineering result uses `legacy-flat`, the dense critic, full
transport, CPU eager execution, 12 workers, worker 1/1/1 threads, learner
4/1/4 threads, and the unchanged PPO/imitation recipe.

| Metric | Baseline full | Optimized full | Multiplier |
| --- | ---: | ---: | ---: |
| Learner steps/s | 320.8213 | **719.0970** | **2.2414×** |
| Complete games/s | 3.4669 | **6.4798** | **1.8689×** |
| Wall seconds | 631.978 | **281.954** | 2.2414× faster |
| Collection seconds | 313.454 | **219.751** | 1.4264× faster |
| PPO update seconds | 317.945 | **61.631** | 5.1591× faster |
| Episodes | 2,191 | 1,827 | trajectory-dependent |
| Optimizer updates | 528 | 528 | unchanged |

The optimized population was 115 random, 111 mechanics-v2, 104 accepted v1,
and 1,497 historical. Different counts reflect a numerically diverged training
trajectory and episode lengths; the selection rule and weights were unchanged.

Selected artifact hashes:

| Artifact | SHA-256 |
| --- | --- |
| Config | `792240890b626a9eeb75af12e0c2e433f94eac7ca68d4e18f97eaaf934822217` |
| Summary | `0d308e540c19c3254d637055bfb81f8b9c5497ba43bb29b7de87917eee8e6943` |
| Replay | `0f908c9264a1246bbbc6cc4e728737c6147c46e4de11a2f2d5501cfa095348b7` |
| Final checkpoint | `751a322f5014bbce5f092cc59420679038afcea0835b3677825b3fdd14a02cff` |
| Final parameter state | `f211b0c96f56c02018361dbc00b6e882610be60482cec8246f93d6b8c6eaeed5` |

Projected at the measured full-run rate: 2M decisions in 46.35 minutes, 10M in
3.86 hours, and 100,000 games in 4.29 hours. The current-code reference projects
2M in 1.73 hours, 10M in 8.66 hours, and 100,000 games in 8.01 hours.

### Repeated short and full runs

Three identical 21,504-step legacy-flat seed-22 runs reproduced the same
episode trajectory and final parameter SHA-256
`bdb3a50fd662670444163fcc2afcb84109b4c7bc8c34e7b70bc4e98787b0054c`:

| Run | Steps/s | Games/s | Wall s | Collection s | PPO update s |
| ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 692.5075 | 6.8916 | 31.0524 | 23.7415 | 6.7253 |
| 2 | 694.2398 | 6.9088 | 30.9749 | 23.7084 | 6.6876 |
| 3 | 693.4960 | 6.9014 | 31.0081 | 23.7323 | 6.7063 |
| Median | **693.4960** | **6.9014** | **31.0081** | **23.7323** | **6.7063** |

Independent full campaigns completed the numerically changed sparse-path
three-seed gate:

| Seed | Steps/s | Games/s | Wall s | Games | Collection s | PPO update s |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 11 | 715.4446 | 7.1420 | 283.393 | 2,024 | 222.365 | 60.411 |
| 22 | 719.0970 | 6.4798 | 281.954 | 1,827 | 219.751 | 61.631 |
| 33 | 710.5922 | 7.0375 | 285.328 | 2,008 | 222.966 | 61.738 |
| Median | **715.4446** | **7.0375** | **283.393** | **2,008** | **222.365** | **61.631** |

All three completed 202,752 steps and 528 optimizer updates with zero safety
failures. Seed-11/33 final checkpoint SHA-256 values were respectively
`425f199e574c6772cb07b06e5811252ef8581a26ce84b921d8fa1f3a92b39806`
and `837113f18065219cd8ed705941599f0168ba0845541c4d1d5074390c91dfba57`.

## Ablations and rejected routes

| Route | Evidence | Disposition |
| --- | --- | --- |
| Category-balanced population | Seed-22 full 811.48 steps/s; three 200k seeds; tactical/broad regressions | Retained explicit, rejected as default |
| Base-only critic | 789.14/814.51/819.81 steps/s at 21,504 steps; PPO update about 4.44 s | Versioned experiment, not promoted without three full quality campaigns |
| Encoded v2 transport | 1,151.16 decisions/s and 102.7 MB/1k vs full 1,188.85 and 32.0 MB | Rejected |
| Parity transport | 807.34 decisions/s and 133.7 MB/1k | Diagnostic only |
| `torch.compile` | Inductor generated module could not load missing Homebrew libc++ runtime | Rejected; host toolchain untouched |
| MPS | Resident 0.879× CPU; transfer-inclusive 0.421×; max probability delta `2.98e-7`, zero argmax mismatches | Rejected |
| PPO epochs/batch/rollout/KL | Profiler found a valid-row engineering route; changing these changes learning | Not attempted |
| Asynchronous/off-policy learner | Algorithm change and unnecessary for target | Not attempted |

The base-only critic owns less than 25% of the dense policy parameter count and
has promising short broad metrics, but its expected full-run gain is small once
legacy historical collection dominates. Selecting it requires wall-clock-to-
quality evidence from three full campaigns, so owner review is required first.

## Quality, export, and regression evidence

The untuned sparse models are throughput/quality probes, not replacements for
the tactically corrected accepted v2 model. Their three-seed heldout results
were:

| Seed | Tactical exact | Immediate bias errors | Broad semantic/type | Wins-losses-draws vs v1 | Raw / adjusted | Wilson 95% raw |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 11 | .0500 | 15 | .6159 / .9470 | 839-139-22 | .839 / .850 | [.8149, .8605] |
| 22 | .0333 | 60 | .5563 / .8212 | 610-329-61 | .610 / .6405 | [.5794, .6398] |
| 33 | .0417 | 32 | .5430 / .8013 | 740-226-34 | .740 / .757 | [.7119, .7662] |

Each full-game comparison used 1,000 seat-swapped games against accepted v1
and had zero truncations, authority rejects, invalid indices, stale submissions,
or wrong-seat submissions. All three broad action-type scores exceeded accepted
v1's .7483. Tactical exact agreement remained weak, while immediate-bias errors
were below accepted v1's 94. The accepted corrected v2 artifact remains the
gameplay baseline and no new model is accepted here.

The new export SHA-256 is
`f26267147dddcf21434247cfc40fd9793072bdeb9c023723cc531daad88e809c`.
Go native inference matched Python through all 187 actions and the winner of a
complete self-play replay. The export points to checkpoint SHA-256
`751a322f...a02cff` and parameter SHA-256 `f211b0c9...eaeed5`.

Phase 1 v1 regression used identical seeds/outcomes in three 1,000-game encoded
runs: 59.9564, 60.4694, and 60.7410 games/s, median 60.4694. This retains
101.46% of the frozen 59.5993 median, with zero safety failures.

Passing checks:

- `go test ./...`
- `go vet ./...`
- `go test -race ./internal/battle/...`
- `dice-and-destiny-server/scripts/build_native.sh`
- `uv lock --check`
- `ruff check dice_destiny_ml tests`
- 29 ML tests
- 100-battle Phase 3 acceptance
- Python/export/Go v2 complete-replay parity
- `verify_battle_authority.gd`
- `verify_learned_battle_v2.gd`
- `verify_learned_battle.gd`

All Godot commands ran through repository-root `./scripts/godot.sh`. Workspace
runtime history, snapshots, active battle, accepted artifacts, and other
worktrees were not changed or cleaned.

## Sustained soak

The unchanged-population sustained soak under `soak/legacy-1.3m-seed-22`
completed 1,302,528 steps, 14,242 games, and 3,392 PPO updates in 2,092.749
seconds (34:52.75). It sustained 622.4005 steps/s and 6.8054 games/s. This is
1.9400× the freshly frozen full reference and 2.0098× the handoff's 309.6875
steps/s orientation baseline; late legacy-flat checkpoint misses explain the
gap from the 200k median.

The 12-worker summed RSS filled its bounded eight-model historical caches early,
then plateaued: approximately 32.73 GiB during initial fill and 31.33-31.43 GiB
through the observed 625k-1.3M interval. System memory remained 87-88% free,
swap remained exactly 1040.75 MiB used, and macOS reported no thermal or
performance warning. All 14,242 episode records were terminal (13,533 victories,
709 draws), with zero authority rejects, invalid indices, stale submissions,
wrong-seat submissions, or truncations. No staged/partial checkpoint zip was
visible.

Soak artifact SHA-256 values:

| Artifact | SHA-256 |
| --- | --- |
| Config | `3e37b03262ff2f74b2a52e817f61f13ee31c249b77ec5ec3106e31bab3e113df` |
| Summary | `16643bd5704a6110c141a77a545a6892285b5731b20cb737e874eaedd35365ab` |
| Representative replay | `0f908c9264a1246bbbc6cc4e728737c6147c46e4de11a2f2d5501cfa095348b7` |
| Final checkpoint | `dbeb3cc7552529e46e4c3dae2ee389044d71cc9971b610a0103a74c610dcf23a` |
| Final parameter state | `24d46af53c22b301d9e438fd219e9df3a41d6a2059d7f25ebb12ee751acc203d` |

## Gates A-K

| Gate | Result | Evidence |
| --- | --- | --- |
| A · Baseline reproducibility | **PASS** | Three deterministic short and one full current-code reference, hashes/config/timing/population/safety retained |
| B · Authority and privacy parity | **PASS** | No engine/transport semantic change; existing full/parity corpora and Go/Godot suites pass |
| C · Process resources | **PASS** | Separate learner/worker intra/inter/BLAS controls; every instrumented process reports 1-thread workers and 4/1 learner; worker sweep retained |
| D · Opponent contract | **PASS** | Explicit legacy/category selection, seeded model generator, immutable manifest, atomic publish, permanent/LRU caches, per-episode identity |
| E · Sparse correctness | **PASS** | Dense/sparse logits/probability/action/sample/logprob/entropy/gradient/capacity/empty-mask tests plus BC path |
| F · End-to-end throughput | **PASS** | Three-seed median 715.4446 steps/s, 2.2300× full baseline; repeated short/full games/timing/host evidence retained |
| G · Learning quality | **PASS for engineering candidate; category correction rejected** | Three seeds each pass tactical/broad/1,000-game v1 evidence with zero safety; tactical limitation disclosed; accepted v2 unchanged |
| H · Sustained stability | **PASS** | 1.3025M steps over 34:52.75; bounded RSS plateau, unchanged swap/pressure, no thermal or safety failure |
| I · Checkpoint/export/runtime | **PASS** | Old v1/v2 and new versioned class load; 187-action Python/Go replay parity with strict export hash |
| J · Regression | **PASS** | Phase 1 101.46% retention; Go/vet/race, lock/lint/tests, native, Phase 3, authority and both Godot verifiers pass |
| K · Report and owner gate | **PASS / OWNER HOLD** | Kept/rejected routes, artifacts, commands, limitations and gates recorded; no commit or push |

## Reproduction commands

All ML paths below are relative to `dice-and-destiny-server/ml` because
`scripts/ml.sh` changes to that directory.

```bash
# Selected unchanged-population full candidate.
./scripts/ml.sh train \
  --seeds 22 --timesteps 200000 --profile max \
  --rollout-steps 256 --batch-size 256 --epochs 8 \
  --learning-rate 0.0003 --checkpoint-interval 5000 \
  --imitation-decisions 5000 --imitation-epochs 5 --device cpu \
  --observation-schema dice-and-destiny-observation-v2 \
  --imitation-teacher mechanics-v2 --transport-mode full \
  --sparse-actor --critic-architecture dense \
  --opponent-selection-mode legacy-flat \
  --opponents random mechanics-v2 \
    model:runs/phase2-training/seed-11/checkpoints/final.zip historical \
  --output runs/phase2-throughput-optimization-20260803-m3max-v2/optimized-full-legacy/run-1

# Remaining full seeds used for the three-seed gate.
./scripts/ml.sh train \
  --seeds 11 33 --timesteps 200000 --profile max \
  --rollout-steps 256 --batch-size 256 --epochs 8 \
  --learning-rate 0.0003 --checkpoint-interval 5000 \
  --imitation-decisions 5000 --imitation-epochs 5 --device cpu \
  --observation-schema dice-and-destiny-observation-v2 \
  --imitation-teacher mechanics-v2 --transport-mode full \
  --sparse-actor --critic-architecture dense \
  --opponent-selection-mode legacy-flat \
  --opponents random mechanics-v2 \
    model:runs/phase2-training/seed-11/checkpoints/final.zip historical \
  --output runs/phase2-throughput-optimization-20260803-m3max-v2/optimized-full-legacy/additional-seeds

# Sustained unchanged-population soak.
./scripts/ml.sh train \
  --seeds 22 --timesteps 1300000 --profile max \
  --rollout-steps 256 --batch-size 256 --epochs 8 \
  --learning-rate 0.0003 --checkpoint-interval 5000 \
  --imitation-decisions 5000 --imitation-epochs 5 --device cpu \
  --observation-schema dice-and-destiny-observation-v2 \
  --imitation-teacher mechanics-v2 --transport-mode full \
  --sparse-actor --critic-architecture dense \
  --opponent-selection-mode legacy-flat \
  --opponents random mechanics-v2 \
    model:runs/phase2-training/seed-11/checkpoints/final.zip historical \
  --verbose 0 \
  --output runs/phase2-throughput-optimization-20260803-m3max-v2/soak/legacy-1.3m-seed-22

# Full-game quality gate.
./scripts/ml.sh acceptance \
  --seat-a model:runs/phase2-throughput-optimization-20260803-m3max-v2/optimized-full-legacy/run-1/seed-22/checkpoints/final.zip \
  --seat-b model:runs/phase2-training/seed-11/checkpoints/final.zip \
  --episodes 500 --seed-start 91000000 --swap \
  --observation-schema dice-and-destiny-observation-v2 \
  --transport-mode full --authority-mode ephemeral \
  --telemetry-mode training --profile max --save-replays representative \
  --output runs/phase2-throughput-optimization-20260803-m3max-v2/quality/legacy-seed-22/vs-accepted-v1-1000

# Static/runtime regression suite.
go test ./...
go vet ./...
go test -race ./internal/battle/...
./scripts/build_native.sh
cd ml && uv lock --check
uv run --python 3.12 ruff check dice_destiny_ml tests
uv run --python 3.12 pytest -q
cd ../..
./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd
./scripts/godot.sh --headless --script res://tests/phase3/verify_learned_battle_v2.gd
./scripts/godot.sh --headless --script res://tests/phase3/verify_learned_battle.gd
```

## Remaining risks and owner decision

- The bounded historical cache has a high summed RSS footprint on this host;
  soak evidence must demonstrate a plateau and no swap/pressure growth.
- Sparse and dense math agree within tight tolerances but can produce different
  long training trajectories. No new model is accepted by this report.
- The category-balanced population is semantically cleaner but failed the
  current short quality screen; it needs a deliberate curriculum/quality study.
- The base-only critic is promising but not selected without full three-seed
  wall-clock-to-heldout-quality evidence.
- Full transport still dominates collector bytes; a future typed/hybrid message
  must preserve viewer privacy and exact candidate/command parity.

Owner review should decide whether to keep the production dense critic, whether
to authorize a separate category-balanced learning study, and whether a future
actor-only historical inference loader is worth reducing the current bounded
cache footprint. Do not start 2M/10M final campaigns, commit, or push before
that review.
