# Phase 1 throughput optimization report

Date: 2026-08-02  
Machine: MacBook Pro Mac15,9, Apple M3 Max, 12 performance + 4 efficiency cores, 64 GB, AC power  
Baseline source HEAD: `93f13c75874f7065444eaab5018b6f6fc953de54` (working tree intentionally dirty for this uncommitted review)  
Status: **PASS — Phase 1 gates A–K pass; owner review is required before commit, push, or Phase 2 decision-quality work.**

## Result

The primary benchmark improved from the documented **1.8474 complete games/s**
to a final three-run median of **59.5993 complete games/s**. This is **32.26×**
the documented rate and clears both the 20 games/s target and 30 games/s
stretch target. The three exact-final-code 1,000-game results were 59.5718,
59.5993, and 59.9546 games/s.

Every final run reproduced the accepted Phase 2 workload exactly:

- 1,000 complete, seat-swapped seed-11-final versus seed-22-final battles;
- 493 seed-11 policy wins, 463 seed-22 policy wins, and 44 draws;
- 202.271 mean accepted actions and the exact historical action-frequency map;
- zero crashes, rejects, invalid indices, stale actions, wrong-seat actions,
  unexplained truncations, or fallbacks;
- a terminal summary for every battle and retained representative/draw replays;
- unchanged terminal health and replayable original authority commands.

The fastest point in the full worker sweep was 60.2108 games/s at 12 workers.
Fourteen and sixteen workers did not improve throughput and materially raised
latency, so the calibrated maximum profile uses 12 workers with one Torch/BLAS
thread per worker.

## Frozen workload and baseline

The apples-to-apples Phase 2 acceptance reproduction used the unchanged
accepted checkpoints, seeds, seat swap, policies, content, rules, viewer
filtering, and 1,000-game terminal contract. On the current machine it completed
in 442.160437 seconds, or **2.261622516 games/s**, rather than the historical
1.8474 games/s. The outcome/action corpus matched the historical report exactly;
the speed difference is disclosed as a machine/software/run-condition effect,
not an optimization.

The separate pre-change Go complete-authority benchmark measured a median of
approximately **202.17 ms/game (4.95 games/s), 137.87 MB/game, and 990,500
allocations/game**. Its CPU and allocation profiles are retained beneath
`baseline/profiles/` in the ignored Phase 1 run tree.

Primary optimized workload:

```bash
cd dice-and-destiny-server
./scripts/ml.sh acceptance \
  --seat-a model:runs/phase2-training/seed-11/checkpoints/final.zip \
  --seat-b model:runs/phase2-training/seed-22/checkpoints/final.zip \
  --authority-mode ephemeral \
  --telemetry-mode training \
  --transport-mode encoded \
  --profile max \
  --output runs/throughput-phase1-20260802-m3max-v1/final-soak/run-1
```

The `acceptance` default supplies 500 seeds per pairing and swaps seats, for
1,000 complete games. No battle length, content, policy, legal decision,
reaction, rule, terminal record, or model inference is removed.

## Final architecture

The smallest retained high-performance design is:

1. The ordinary battle engine remains the sole rule and mutation authority.
2. Immutable settled content is cached by its pinned catalog hash. The hash is
   stored only as transient in-memory state and recomputed after persistence
   recovery; checkpoint/replay JSON is unchanged.
3. The simulation assembler pins validated `battle_v1` content for its
   long-lived process. Production file assemblers preserve their existing
   per-battle filesystem behavior.
4. Simulation snapshots omit the immutable public catalog at construction time
   because the ML response already omitted it. Production authority snapshots
   still include the catalog.
5. Candidate canonicalization computes each JSON sort key once.
6. The training-only `Ephemeral` repository holds one disposable battle without
   deep-cloning the growing persisted history on every load/save. It still calls
   the exact engine, sequences exact events, and is inaccessible to production
   persistence callers.
7. Training telemetry avoids constructing the applied command result that the
   synchronous step caller discards. The next decision is still obtained from
   an exact viewer-filtered authority open; terminal commands still construct a
   complete result.
8. The Go simulator produces the frozen 4,224-value v1 observation and action
   mask, packs float32 values and mask bits into base64, and retains the exact
   original command list privately for indexed submission. `parity` mode sends
   both raw and encoded decisions and performs a bitwise Python/Go comparison.
9. Independent evaluation games use spawned worker processes. Every worker owns
   independent policy instances, Go authority process, repository, seed shard,
   encoder, and artifact namespace. Torch and BLAS are capped explicitly.
10. PPO training now uses `SubprocVecEnv` when workers exceed one; the one-worker
    reproducibility lane keeps `DummyVecEnv`.

Production `InMemory` and `Disk` repositories, persistence validation, recovery,
Godot integration, the shipped Go model, content, and accepted checkpoints were
not weakened or replaced.

## Optimization iterations

All Go numbers below are repeated complete-battle benchmark medians or the
representative median when explicitly noted.

| Iteration | Result | Allocation result | Decision |
|---|---:|---:|---|
| Pre-change Go authority | 202.17 ms/game, 4.95 games/s | 137.87 MB, 990.5k allocs | Baseline |
| Settled-library cache | ~107.71 ms/game | 113.85 MB, 532.2k allocs | Kept; removed repeated catalog decode |
| One-time canonical sort keys | ~101.79 ms/game | 110.07 MB, 509.8k allocs | Kept |
| Ephemeral repository | ~73.00 ms/game | 28.94 MB, 338.8k allocs | Kept only after exact parity |
| Omit discarded snapshot catalog | ~13.68 ms/game | 14.70 MB, 66.8k allocs | Kept; production snapshots unchanged |
| Transient settled-catalog hash | ~8.57 ms/game | 14.51 MB, 65.9k allocs | Kept |
| Simulation participant-content cache | ~7.36 ms/game | 13.25 MB, 53.1k allocs | Kept |
| Training result telemetry | ~7.53 ms/game | 11.32 MB, 43.1k allocs | Kept alongside full mode |
| Final single-lane Go authority | **6.7646 ms/game, 147.83 games/s** | **11.14 MB, 42.4k allocs** | Kept |
| Go-side binary v1 schema, serial models | **9.40 games/s** | transport reduction | Kept; prior full JSON serial result was 7.37 games/s |
| Spawned rollout processes | 31.39 games/s at 4; 60.21 at 12 | independent memory per worker | Kept |

The final Go single-lane result is about 29.9× the pre-change Go benchmark,
uses 8.1% of the prior bytes/game, and uses 4.3% of the prior allocations/game.
The primary model benchmark remains slower per lane because every accepted
decision still performs frozen Torch inference.

## Rejected or bounded routes

- The first ephemeral parity corpus exposed nondeterministic defense-event
  ordering caused by iteration over `DefenseSelections`. The fast path was not
  accepted until defense finalization used sorted authority actor order and the
  full fixed/generated corpus passed repeatedly. This also removed latent event
  nondeterminism from normal authorities.
- Fourteen and sixteen workers were rejected as maximum settings: throughput
  fell slightly while p95 game latency increased to 260.86 and 300.72 ms.
- Full raw viewer JSON remains available for diagnostics but was rejected as
  the primary transport because it repeated Python schema work and transferred
  data the model did not consume.
- Centralized inference batching was profiled conceptually against the measured
  path but not retained. Independent one-thread inference scaled through the 12
  performance cores, and p95 inference was only 0.619 ms at the peak. A shared
  batcher would add ordering, failure, and latency complexity after the machine
  was already saturated.
- Asynchronous episode-summary I/O was not retained. Measured summary and
  representative-replay I/O was negligible relative to collection, while
  direct deterministic writes are simpler and more recoverable.
- Go-native policy evaluation was not substituted for the primary benchmark;
  the workload continues to execute the same frozen Python checkpoints and
  therefore remains apples-to-apples with Phase 2.

## Worker saturation curve

Each point is a complete 1,000-game run with exact gameplay outputs. CPU is
normalized to 100% for all 16 logical CPUs. Peak RSS is the sum of per-worker
Python and child-Go high-water marks and is therefore a conservative process
estimate, not Activity Monitor resident memory at one instant.

| Workers | Games/s | Scaling efficiency | Normalized CPU | Peak RSS | Game p50/p95 ms | Inference p95 ms | Foreground wake p95 ms |
|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | 9.7585 | 100.0% | 7.73% | 0.38 GB | 100.13 / 141.03 | 0.277 | 9.89 |
| 2 | 17.4877 | 89.6% | 15.60% | 0.69 GB | 107.45 / 151.06 | 0.328 | 8.25 |
| 4 | 31.3906 | 80.4% | 30.99% | 1.37 GB | 116.46 / 164.60 | 0.377 | 8.60 |
| 6 | 42.2910 | 72.2% | 44.12% | 2.05 GB | 125.88 / 175.54 | 0.405 | 9.99 |
| 8 | 50.9208 | 65.2% | 57.11% | 2.73 GB | 136.56 / 191.97 | 0.434 | 9.23 |
| 10 | 57.4883 | 58.9% | 68.06% | 3.41 GB | 146.65 / 204.51 | 0.461 | 10.03 |
| 12 | **60.2108** | 51.4% | 76.41% | 4.08 GB | 162.38 / 227.10 | 0.619 | 9.25 |
| 14 | 59.7516 | 43.7% | 83.37% | 4.76 GB | 186.30 / 260.86 | 0.868 | 8.50 |
| 16 | 59.7110 | 38.2% | 87.76% | 5.44 GB | 211.24 / 300.72 | 0.970 | 10.05 |

Memory pressure remained at 89–91% free throughout calibration. Swap was already
1,064.75 MB used before the tests and did not grow. `pmset -g therm` reported no
thermal, performance, or CPU-power warning before or after every measured
profile.

## Final repeated soaks

| Run | Games/s | Game p50/p95 ms | Step p50/p95 ms | Inference p95 ms | CPU | Peak RSS |
|---:|---:|---:|---:|---:|---:|---:|
| 1 | 59.5718 | 162.58 / 231.49 | 0.232 / 0.945 | 0.609 | 76.01% | 4.10 GB |
| 2 | 59.5993 | 163.39 / 229.47 | 0.231 / 0.938 | 0.639 | 76.86% | 4.11 GB |
| 3 | 59.9546 | 162.33 / 228.32 | 0.230 / 0.928 | 0.614 | 76.46% | 4.11 GB |

Median: **59.5993 games/s**. Range: 0.3828 games/s (0.64% of the median).
All safety counters were zero. Every run retained `eval-0-9000000` plus the
first draw replay `eval-0-9000008`; both replayed successfully through the
ordinary normal/full authority path.

The maximum profile also completed a **31m46s sustained soak**: 140,000 exact
model-v-model games at 73.4440 games/s, 82.19% normalized CPU, 6.87 GB summed
worker high-water RSS, 158.02/225.06 ms game p50/p95, 0.603 ms inference p95,
and 0.228/0.934 ms step p50/p95. It produced zero authority rejects, invalid
indices, stale or wrong-seat actions, and truncations. Swap remained at the
pre-existing 1,064.75 MB, memory free percentage rose from 90% to 92%, and
`pmset -g therm` reported no thermal, performance, or CPU-power warning. A
preceding 108,000-game pass completed in 24m20s at 73.9769 games/s with the
same zero-failure and host-health results; it was retained but not mislabeled
as satisfying the 30-minute floor. The sustained rate is higher than the
1,000-game figures because fixed worker startup and two checkpoint loads per
worker are amortized over 140 times as many games; the primary gate remains the
conservative three-run 1,000-game median.

```bash
./scripts/ml.sh benchmark \
  --seat-a model:runs/phase2-training/seed-11/checkpoints/final.zip \
  --seat-b model:runs/phase2-training/seed-22/checkpoints/final.zip \
  --episodes 70000 --seed-start 20000000 --swap --save-replays none \
  --authority-mode ephemeral --telemetry-mode training \
  --transport-mode encoded --profile max \
  --output runs/throughput-phase1-20260802-m3max-v1/final-soak/max-30-minute-v2
```

## Resource profiles

The percentages name resource/headroom goals and do not promise a percentage
of maximum throughput.

| Profile | Resolved budget | Games/s | CPU | Peak RSS | Game p95 ms | Foreground wake p95 ms |
|---|---|---:|---:|---:|---:|---:|
| `max` | 12 workers × 1 Torch/BLAS thread | 59.9474 | 76.64% | 4.08 GB | 230.01 | 9.94 |
| `balanced-80` | 10 × 1 | 57.1493 | 68.14% | 3.40 GB | 206.65 | 9.97 |
| `light-50` | 6 × 1 | 42.2312 | 43.95% | 2.05 GB | 175.40 | 9.42 |
| `custom` example | 4 × 1 | 31.5249 | 30.96% | 1.37 GB | 161.25 | 7.77 |

Commands print the resolved budget to stderr and record it in `summary.json`.
Named profiles reject hidden overrides; `custom` accepts explicit worker and
thread values but rejects budgets whose worker × thread count exceeds the
machine's logical CPU count. Recalibration is required on materially different
hardware or power mode.

The sustained light-50 PPO smoke used six spawned environments, 4,608 completed
learner steps, 13 complete games, 12 optimizer updates, behavior cloning,
intermediate and final checkpoints, and changed parameters. It completed at
775.7 learner steps/s. Timings were 4.441 s collection, 1.026 s PPO updates, and
0.0168 s artifact I/O, with zero imitation authority rejects or truncations.
Evaluation code never invokes an optimizer or changes model weights.

## Artifact and telemetry modes

- `--telemetry-mode full`: ordinary applied-command viewer result; required for
  diagnostics and learned-session transcript consumers.
- `--telemetry-mode training`: exact mutation/event persistence with a compact
  nonterminal scheduling result; complete terminal result remains mandatory.
- `--transport-mode full`: ordinary viewer-safe snapshot plus exact legal
  commands over JSON.
- `--transport-mode encoded`: base64 little-endian float32 v1 observation,
  bit-packed mask, candidate count/types; raw snapshot/commands stay inside Go.
- `--transport-mode parity`: both forms; Python fails on any unequal mask bit or
  float32 value.
- replay retention: `none`, `representative`, or `all`. Final acceptance used
  representative retention; replay actions are always the original commands.

The ignored evidence root is:

`dice-and-destiny-server/ml/runs/throughput-phase1-20260802-m3max-v1/`

It contains baseline profiles, all iteration results, Go/Python encoding parity,
the complete worker sweep, resource calibrations, PPO smoke/checkpoints, Phase 3
acceptance, final soaks, replays, and `artifact-index.json`. Tracked configuration
and measurements are in `ml/configs/throughput-phase1.json` and
`ml/configs/throughput-phase1-results.json`.

## Exact parity and safety evidence

- `TestEphemeralAuthorityMatchesNormalAuthorityCorpus`: 5 fixed plus 20 generated
  seeds; exact actor order, candidates, original submitted command, viewer result
  and events, hidden state, RNG cursor, terminal result, and health. Passed three
  consecutive times and under the race detector.
- `TestTrainingTelemetryMatchesFullAuthorityCorpus`: 5 fixed plus 7 generated
  seeds; full normal versus ephemeral/training transitions, candidate order,
  hidden state after every action, complete accepted event stream, replay, and
  terminal metrics.
- Go/Python `parity` transport: 50 complete model games and approximately 10,000
  decisions with bitwise-equal 4,224 float32 observations and action masks.
- The 100-game serial full/encoded and serial/4-worker corpora were exactly
  equal after removing duration/path metadata.
- The reproduced baseline and final 1,000-game episode files were exactly equal
  after removing duration/path and newly reported RNG-cursor metadata.
- Final representative replays produced by the optimized path replayed through
  the normal/full repository and authority path.
- Phase 3 both-seat acceptance: 100 battles, 7,781 model decisions, zero errors,
  fallbacks, hidden leaks, invalid/stale/wrong-seat actions, timeouts, restarts,
  or truncations.

## Verification

Passed on the reported working tree:

```text
go test -count=1 ./...
go vet ./...
go test -count=1 -race ./internal/battle ./internal/battle/engine ./internal/battle/snapshot ./internal/battle/mlsim
cd ml && uv run --python 3.12 pytest -q                  # 12 passed
cd ml && uv run --python 3.12 ruff check .
cd ml && uv lock --check
./scripts/ml.sh smoke --seed 20260801                   # replay verified
./scripts/ml.sh evaluate ... --episodes 1 --swap        # Phase 3 checkpoint smoke
go run ./cmd/phase3-acceptance --battles 100 ...        # acceptance_passed
dice-and-destiny-server/scripts/build_native.sh
./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_battle_scene.gd
git diff --check
```

The presentation check passed its functional assertions and emitted its existing
Godot exit-time RID/ObjectDB leak diagnostics. No functional assertion failed.

Software: macOS 26.5.2, Go 1.21.6 darwin/arm64, Python 3.12.7, NumPy 2.5.1,
Torch 2.13.0, Stable-Baselines3 2.9.0, sb3-contrib 2.9.0, Godot 4.6.2.

## Hashes and preservation

| Artifact | SHA-256 |
|---|---|
| Phase 2 seed-11 final checkpoint | `e2e98c2ecadfe9e06892676962e751a116cb07183aa193882d73927866d760b8` |
| Phase 2 seed-22 final checkpoint | `75692ceb47232f5ab0c7a54c4b1b429220535253f32c19f41e583ee03e49af3d` |
| Phase 2 seed-33 final checkpoint | `f60e9cc2d31577056b98ef481ae5b31ca4dcd0c9c1352758d8118638192c8df2` |
| Phase 3 frozen export | `dea4a6681fcd12d368dee6e720bb0742643a25dd46366080ab4e79fba59e3937` |
| ML lock | `2a0f2aa2c52170969a65b3999b50c008116c22971cfc7261c2fc6cfd64626588` |

Accepted checkpoints, the frozen export, user runtime state, developer history,
snapshots, and other worktrees were not modified or deleted. No commit or push
was made.

## Phase 1 gates A–K

| Gate | Result | Evidence |
|---|---|---|
| A · Baseline reproducibility | **PASS** | Exact 1,000-game Phase 2 workload reproduced at 2.2616 games/s; historical 1.8474 difference disclosed; hashes/config/replays retained. |
| B · Authority parity | **PASS** | Fixed/generated full-state/event corpus, training telemetry corpus, bitwise Go/Python encoder corpus, exact 100/1,000-game episode comparisons, normal replay. |
| C · Safety | **PASS** | Three final soaks and Phase 3 acceptance have zero required failure counters; viewer privacy/state leakage tests pass. |
| D · Primary throughput | **PASS** | Three exact-final-code 1,000-game runs; median 59.5993 games/s; game, step, inference p50/p95 and variability reported. |
| E · Saturation | **PASS** | Required 1–16 worker curve plus 31m46s/140,000-game max soak; CPU/RSS/swap/thermal/responsiveness, collection/inference/update/I/O timings recorded. |
| F · Single-lane efficiency | **PASS** | Before/after CPU/allocation profiles and repeated benchmarks: 4.95 to 147.83 Go games/s, 137.87 to 11.14 MB/game. |
| G · Training integration | **PASS** | Six-process real-authority PPO smoke performs BC, rollout collection, 12 updates, resets, checkpoints, terminal metrics, and parameter change. |
| H · Artifact modes | **PASS** | Full, training, encoded, parity, and replay retention modes are explicit, tested, deterministic, and labeled in every summary. |
| I · Regression | **PASS** | Full Go/vet/race, ML lint/lock/tests, Phase 2 smoke/replay, Phase 3 acceptance, native, Godot authority and presentation checks pass. |
| J · Maintainability | **PASS** | Production persistence unchanged; fast paths are opt-in and documented; abandoned alternatives are absent; parity tests guard every boundary. |
| K · Resource controls | **PASS** | Max/balanced-80/light-50/custom resolve explicit caps, print/record settings, preserve exact corpus, include resource/response measurements, and pass multiprocess PPO smoke. |

## Remaining risks and owner options

- The 12-worker plateau is specific to this Mac15,9, AC power state, software
  stack, model size, and v1 workload. The resolver deliberately warns through
  recorded machine settings rather than claiming portable percentages.
- CPU is computed from worker and Go-child resource usage; the lightweight
  coordinator is not included. RSS is a summed high-water estimate. Activity
  Monitor will not exactly match either measure.
- `memory_pressure`, `sysctl vm.swapusage`, and `pmset -g therm` are coarse macOS
  observations, not package-temperature telemetry. No warning or swap growth was
  observed.
- Central inference batching is an owner option if future models become much
  larger or accelerators replace CPU inference. It is not justified by this
  v1 plateau.
- The remaining single-lane Go allocation leaders are authoritative state clone,
  viewer snapshot, legal-candidate construction, and event sequencing. Further
  work would require more invasive immutable/mutable state separation and is not
  needed to meet Phase 1.
- Phase 2 decision-quality work remains explicitly out of scope and blocked on
  owner acceptance of this report.
