# Observation V3 eight-hour global-high-watermark campaign report

Date: 2026-08-05 (America/Chicago)  
Baseline: `origin/main` at `1a8aa21c7ed020a0a4482ebab734b256e6f64609`  
Campaign: `v3-global-highwater-campaign-20260804-8h-main`  
Outcome: engineering success; gameplay promotion did not pass

## Executive result

The requested controller changes were implemented and verified before the clock started. The
campaign then used the complete eight-hour window, trained 170 immutable 52,224-step checkpoints
across 16 fresh families, completed 106 required 1,000-game gates and a final independent
1,000-game comparison, and stopped only after the checkpoint that crossed the deadline had been
saved and evaluated. No checkpoint 171 started.

The strongest campaign result was **family 8, checkpoint 6, campaign attempt 73**, using a
128-wide, two-layer GELU entity encoder. Its campaign-bank result against the required raw V3
checkpoint 400 was **460 wins, 56 draws, and 484 losses**, or **48.80% draw-adjusted score** with
adjusted 95% interval **45.7124%-51.8967%**. Because 48.80% did not exceed 50%, the controller
correctly did not start a 3,000-game confirmation.

The separate final bank measured the same checkpoint at **460 wins, 64 draws, and 476 losses**, or
**49.20% draw-adjusted score**, with adjusted 95% interval **46.1104%-52.2957%**. This also failed
the strict `>50%` first-stage requirement. No new global champion was created; raw checkpoint 400
remains unchanged.

Key identities:

- Best checkpoint path:
  `dice-and-destiny-server/ml/runs/v3-global-highwater-campaign-20260804-8h/attempts/attempt-073/raw-ppo-challenger.zip`
- Best checkpoint SHA-256:
  `30da04d44d3b664918e9eff80b2bc49fe80373310a426852611b3eb0bf0b1612`
- Best optimizer SHA-256:
  `5616eb62c4f8ddbf38e759e55eb7eb2f322ab87119a1e293f377b0d84f9be3be`
- Frozen global path:
  `dice-and-destiny-server/ml/runs/v3-seed22-optimized-5m-20260803-m3max/seed-22/checkpoints/step-002001600.zip`
- Frozen global SHA-256:
  `9cc042d0e355dceb49e98d82296c7017c9fc810296d87b151c2dc9abf8be43b8`
- Final seed-bank SHA-256:
  `02811f6ed51874e537a47e498751d50841e5c056e877987e9135e42951ef2d12`

## Implemented campaign rules

The new campaign controller enforces the following policy:

1. Every family uses the checkpoint-400 PPO recipe without any within-family control changes.
2. Every family starts from a system-random seed, a fresh Mechanics-V2 behavior-cloning warm
   start of 5,000 decisions for five epochs, and zero PPO timesteps. No old learner checkpoint is
   used as a parent.
3. Checkpoints 1-4 train without evaluation. Checkpoint 5 receives the family's first 1,000-game
   test against raw checkpoint 400 and establishes the high watermark.
4. Every later checkpoint is tested for 1,000 games against raw checkpoint 400. Its adjusted
   score must strictly exceed the current family high watermark to pass and replace it. A tie is a
   miss.
5. A miss does not roll back, branch, or alter settings. Training continues from that exact raw
   learner and optimizer. Three consecutive misses fail the family.
6. A new family changes exactly one architecture setting—width, depth, or activation—from the
   previous family, gets a new system-random seed, and repeats the full fresh warm-start process.
7. A 3,000-game fresh confirmation is permitted only after a 1,000-game adjusted score strictly
   exceeds 50%. Promotion requires the independent 3,000-game score also to exceed 50%, with all
   safety counters zero.
8. The timer includes training and checkpoint evaluations. A checkpoint already in progress at
   the deadline finishes its save and required evaluation exactly once; no next interval starts.

The exact checkpoint-400 recipe remained fixed for all 170 intervals:

| Setting | Value |
|---|---:|
| Learning rate | `3e-4` |
| PPO epochs | `8` |
| Clip range | `0.2` |
| Target KL | disabled |
| Entropy coefficient | `0.01` |
| Batch size | `256` |
| Rollout size | `256` per worker |
| Gamma | `0.995` |
| GAE lambda | `0.95` |
| Value-loss coefficient | `0.5` |
| Reward | `+1/-1/0`, no shaping |
| Post-PPO teacher correction | disabled |

The training opponent curriculum also matches the checkpoint-400 setup: random, Mechanics-V2,
the accepted V1 phase-2 seed-11 checkpoint, and immutable snapshots from the current family only,
using the legacy-flat category schedule. Internal history snapshots were preserved every 5,000
nominal steps and never crossed family boundaries.

## Preflight and reproducibility

The preflight passed before the timed loop:

- Power: AC, internal battery 100% charged; battery launch override disabled.
- Full-window mode: required; time budget exactly 28,800 seconds.
- Source revision:
  `1a8aa21c7ed020a0a4482ebab734b256e6f64609+tree:d1fef6453b844fde0201d2e70f77279306112209ae8ce59ef4995bcf6202ea16`
- Source-tree SHA-256:
  `d1fef6453b844fde0201d2e70f77279306112209ae8ce59ef4995bcf6202ea16`
- Observation-manifest logical SHA-256:
  `5ede31f0126be159f4a7f3ff04a94973643a237d586f806818792c9ce29316e2`
- Frozen-content SHA-256:
  `33d540af82133e8b8ad7e633c4cca6e352367da9d72f0dd930bc22ebcd263fd0`
- Signed family-plan logical SHA-256:
  `1017703d61dba0c54e9cbce97733fe93ff74493ba79a9e1fc595d5c8d59a8a0e`
- Seed-bank manifest logical SHA-256:
  `cff51c09954ccbbe11489fca858a4167fb870575170aa3ef471e926ee1044d64`
- Predeclared capacity: 512 checkpoint 1K banks, 512 independent 3K confirmation banks, one
  final bank, one development anchor, and 32 fresh model families.
- Resource profile: 12 workers, 12 inference tasks, two I/O tasks, one BLAS/Torch thread per
  worker, and four learner threads on 16 logical CPUs.

The final audit re-read and hash-validated all 1,026 seed-bank files. They contain **1,025,000
distinct seed values with zero overlap**. It also reloaded all 32 prebuilt warm starts and proved
that each has a unique system-random seed, checkpoint hash, and optimizer hash; each records 5,000
teacher decisions, five epochs, zero PPO timesteps, and zero safety violations.

## Timing and workload

- Start: `2026-08-05T04:29:57.450089+00:00` (2026-08-04 23:29:57.450 CDT)
- Deadline: `2026-08-05T12:29:57.450089+00:00` (07:29:57.450 CDT)
- Finish: `2026-08-05T12:32:01.554287+00:00` (07:32:01.554 CDT)
- Authoritative monotonic elapsed time: **28,923.897 seconds** (8h02m03.897s)
- Required deadline grace: **123.897 seconds**
- Checkpoint 170 started with **119.696 seconds remaining**, crossed the deadline during training,
  was saved, and completed its required 1K gate. No checkpoint 171 started.
- Campaign flags: `window_fully_used=true`, `stopped_after_deadline_grace=true`,
  `status=completed`.

Work performed:

- 170 intervals x 52,224 steps = **8,878,080 PPO steps**.
- Training time: **24,064.085 seconds** (6h41m04.085s).
- Training interval mean/median/min/max: **141.553 / 140.047 / 95.319 / 203.517 seconds**.
- Weighted training throughput: **368.935 steps/second**.
- 106 checkpoint evaluations = **106,000 games** in **4,715.326 seconds**
  (44.484 seconds/evaluation; 22.480 games/second).
- Final comparison: **1,000 games** in **45.615 seconds**.
- Total evaluation workload: **107,000 games**.
- Controller, artifact, family-switch, and other measured overhead: **98.872 seconds**.
- Across checkpoint gates: **29,551 wins, 6,638 draws, 69,811 losses**, or **31.0094%** adjusted.
- High-watermark decisions: 46 passes (including the 16 checkpoint-5 baselines) and 60 misses.
- No 3,000-game confirmation ran because no checkpoint's 1K score exceeded 50%.

Every 1K evaluation used 500 distinct seeds twice: the challenger played exactly 500 games in
seat A and 500 in seat B. The final audit checked all 107,000 episode rows, not only summaries.

## Family progression

Each transition below changes exactly one architecture setting. All families use the same PPO,
reward, teacher, opponent, observation, action, seat, and evaluation settings.

| Family | Seed | Architecture | One change | CP5 baseline | Best checkpoint | Best score | Attempts | Stop |
|---:|---:|:---|:---|---:|:---|---:|---:|:---|
| 1 | 1608707705 | 96/2/tanh | initial | 33.00% | CP8 / A8 | 36.55% | 11 | three misses |
| 2 | 831113317 | 128/2/tanh | width 96->128 | 20.75% | CP6 / A17 | 31.50% | 9 | three misses |
| 3 | 3124683675 | 128/3/tanh | depth 2->3 | 27.70% | CP5 / A25 | 27.70% | 8 | three misses |
| 4 | 197437566 | 128/3/relu | tanh->relu | 24.05% | CP8 / A36 | 33.30% | 11 | three misses |
| 5 | 2737081123 | 160/3/relu | width 128->160 | 25.35% | CP7 / A46 | 28.50% | 10 | three misses |
| 6 | 4015865241 | 160/2/relu | depth 3->2 | 28.60% | CP7 / A56 | 30.40% | 10 | three misses |
| 7 | 1542953645 | 160/2/gelu | relu->gelu | 26.80% | CP5 / A64 | 26.80% | 8 | three misses |
| 8 | 3339090428 | 128/2/gelu | width 160->128 | 42.60% | CP6 / A73 | **48.80%** | 9 | three misses |
| 9 | 2743539072 | 128/1/gelu | depth 2->1 | 34.95% | CP8 / A84 | 46.55% | 11 | three misses |
| 10 | 2681725836 | 128/1/tanh | gelu->tanh | 28.00% | CP16 / A103 | 44.30% | 19 | three misses |
| 11 | 47658382 | 192/1/tanh | width 128->192 | 38.15% | CP7 / A113 | 38.70% | 10 | three misses |
| 12 | 2242765118 | 192/2/tanh | depth 1->2 | 12.25% | CP10 / A126 | 23.90% | 13 | three misses |
| 13 | 4016722853 | 192/2/relu | tanh->relu | 26.55% | CP12 / A141 | 37.40% | 15 | three misses |
| 14 | 255878473 | 96/2/relu | width 192->96 | 43.20% | CP5 / A149 | 43.20% | 8 | three misses |
| 15 | 3033912036 | 96/3/relu | depth 2->3 | 40.50% | CP5 / A157 | 40.50% | 8 | three misses |
| 16 | 3855443949 | 96/3/gelu | relu->gelu | 39.10% | CP7 / A167 | 42.35% | 10 | deadline after three misses |

Family 10 is useful evidence for the no-rollback rule: after an early sequence containing misses,
the unchanged continuous learner climbed from its 28.00% baseline to 44.30% at checkpoint 16.

## Every required 1,000-game checkpoint gate

W-D-L and percentages are from the challenger perspective. Checkpoint 5 establishes the family
baseline; therefore its previous-high and delta fields are intentionally blank.

| Family | CP | Attempt | W-D-L | Adjusted | Previous high | Delta | Decision | 95% CI |
|---:|---:|---:|---:|---:|---:|---:|:---|:---|
| 1 | 5 | 5 | 293-74-633 | 33.00% | — | — | pass | 30.16-35.97% |
| 1 | 6 | 6 | 244-55-701 | 27.15% | 33.00% | -5.85 pp | fail | 24.48-29.99% |
| 1 | 7 | 7 | 306-65-629 | 33.85% | 33.00% | +0.85 pp | pass | 30.98-36.84% |
| 1 | 8 | 8 | 327-77-596 | 36.55% | 33.85% | +2.70 pp | pass | 33.62-39.58% |
| 1 | 9 | 9 | 264-66-670 | 29.70% | 36.55% | -6.85 pp | fail | 26.95-32.61% |
| 1 | 10 | 10 | 239-68-693 | 27.30% | 36.55% | -9.25 pp | fail | 24.63-30.14% |
| 1 | 11 | 11 | 280-65-655 | 31.25% | 36.55% | -5.30 pp | fail | 28.45-34.19% |
| 2 | 5 | 16 | 182-51-767 | 20.75% | — | — | pass | 18.35-23.37% |
| 2 | 6 | 17 | 287-56-657 | 31.50% | 20.75% | +10.75 pp | pass | 28.70-34.45% |
| 2 | 7 | 18 | 250-50-700 | 27.50% | 31.50% | -4.00 pp | fail | 24.82-30.35% |
| 2 | 8 | 19 | 221-60-719 | 25.10% | 31.50% | -6.40 pp | fail | 22.51-27.88% |
| 2 | 9 | 20 | 215-70-715 | 25.00% | 31.50% | -6.50 pp | fail | 22.42-27.78% |
| 3 | 5 | 25 | 250-54-696 | 27.70% | — | — | pass | 25.02-30.56% |
| 3 | 6 | 26 | 220-32-748 | 23.60% | 27.70% | -4.10 pp | fail | 21.07-26.33% |
| 3 | 7 | 27 | 227-60-713 | 25.70% | 27.70% | -2.00 pp | fail | 23.09-28.50% |
| 3 | 8 | 28 | 205-44-751 | 22.70% | 27.70% | -5.00 pp | fail | 20.21-25.40% |
| 4 | 5 | 33 | 216-49-735 | 24.05% | — | — | pass | 21.50-26.80% |
| 4 | 6 | 34 | 202-36-762 | 22.00% | 24.05% | -2.05 pp | fail | 19.54-24.67% |
| 4 | 7 | 35 | 235-51-714 | 26.05% | 24.05% | +2.00 pp | pass | 23.42-28.86% |
| 4 | 8 | 36 | 300-66-634 | 33.30% | 26.05% | +7.25 pp | pass | 30.45-36.28% |
| 4 | 9 | 37 | 194-52-754 | 22.00% | 33.30% | -11.30 pp | fail | 19.54-24.67% |
| 4 | 10 | 38 | 257-41-702 | 27.75% | 33.30% | -5.55 pp | fail | 25.06-30.61% |
| 4 | 11 | 39 | 247-55-698 | 27.45% | 33.30% | -5.85 pp | fail | 24.77-30.30% |
| 5 | 5 | 44 | 213-81-706 | 25.35% | — | — | pass | 22.75-28.14% |
| 5 | 6 | 45 | 152-63-785 | 18.35% | 25.35% | -7.00 pp | fail | 16.07-20.87% |
| 5 | 7 | 46 | 246-78-676 | 28.50% | 25.35% | +3.15 pp | pass | 25.79-31.38% |
| 5 | 8 | 47 | 198-60-742 | 22.80% | 28.50% | -5.70 pp | fail | 20.31-25.50% |
| 5 | 9 | 48 | 137-49-814 | 16.15% | 28.50% | -12.35 pp | fail | 14.00-18.56% |
| 5 | 10 | 49 | 151-49-800 | 17.55% | 28.50% | -10.95 pp | fail | 15.32-20.03% |
| 6 | 5 | 54 | 255-62-683 | 28.60% | — | — | pass | 25.89-31.48% |
| 6 | 6 | 55 | 203-57-740 | 23.15% | 28.60% | -5.45 pp | fail | 20.64-25.86% |
| 6 | 7 | 56 | 271-66-663 | 30.40% | 28.60% | +1.80 pp | pass | 27.63-33.32% |
| 6 | 8 | 57 | 196-51-753 | 22.15% | 30.40% | -8.25 pp | fail | 19.69-24.83% |
| 6 | 9 | 58 | 248-49-703 | 27.25% | 30.40% | -3.15 pp | fail | 24.58-30.09% |
| 6 | 10 | 59 | 250-70-680 | 28.50% | 30.40% | -1.90 pp | fail | 25.79-31.38% |
| 7 | 5 | 64 | 242-52-706 | 26.80% | — | — | pass | 24.15-29.63% |
| 7 | 6 | 65 | 223-58-719 | 25.20% | 26.80% | -1.60 pp | fail | 22.61-27.98% |
| 7 | 7 | 66 | 214-44-742 | 23.60% | 26.80% | -3.20 pp | fail | 21.07-26.33% |
| 7 | 8 | 67 | 172-49-779 | 19.65% | 26.80% | -7.15 pp | fail | 17.31-22.23% |
| 8 | 5 | 72 | 393-66-541 | 42.60% | — | — | pass | 39.57-45.69% |
| 8 | 6 | 73 | 460-56-484 | 48.80% | 42.60% | +6.20 pp | pass | 45.71-51.90% |
| 8 | 7 | 74 | 281-55-664 | 30.85% | 48.80% | -17.95 pp | fail | 28.07-33.78% |
| 8 | 8 | 75 | 321-54-625 | 34.80% | 48.80% | -14.00 pp | fail | 31.91-37.81% |
| 8 | 9 | 76 | 328-63-609 | 35.95% | 48.80% | -12.85 pp | fail | 33.03-38.97% |
| 9 | 5 | 81 | 312-75-613 | 34.95% | — | — | pass | 32.06-37.96% |
| 9 | 6 | 82 | 372-72-556 | 40.80% | 34.95% | +5.85 pp | pass | 37.79-43.88% |
| 9 | 7 | 83 | 393-65-542 | 42.55% | 40.80% | +1.75 pp | pass | 39.52-45.64% |
| 9 | 8 | 84 | 433-65-502 | 46.55% | 42.55% | +4.00 pp | pass | 43.48-49.65% |
| 9 | 9 | 85 | 382-87-531 | 42.55% | 46.55% | -4.00 pp | fail | 39.52-45.64% |
| 9 | 10 | 86 | 328-82-590 | 36.90% | 46.55% | -9.65 pp | fail | 33.96-39.94% |
| 9 | 11 | 87 | 400-75-525 | 43.75% | 46.55% | -2.80 pp | fail | 40.71-46.84% |
| 10 | 5 | 92 | 256-48-696 | 28.00% | — | — | pass | 25.31-30.86% |
| 10 | 6 | 93 | 242-71-687 | 27.75% | 28.00% | -0.25 pp | fail | 25.06-30.61% |
| 10 | 7 | 94 | 254-75-671 | 29.15% | 28.00% | +1.15 pp | pass | 26.42-32.04% |
| 10 | 8 | 95 | 236-82-682 | 27.70% | 29.15% | -1.45 pp | fail | 25.02-30.56% |
| 10 | 9 | 96 | 265-88-647 | 30.90% | 29.15% | +1.75 pp | pass | 28.11-33.83% |
| 10 | 10 | 97 | 295-75-630 | 33.25% | 30.90% | +2.35 pp | pass | 30.40-36.23% |
| 10 | 11 | 98 | 328-70-602 | 36.30% | 33.25% | +3.05 pp | pass | 33.38-39.33% |
| 10 | 12 | 99 | 355-67-578 | 38.85% | 36.30% | +2.55 pp | pass | 35.88-41.91% |
| 10 | 13 | 100 | 366-72-562 | 40.20% | 38.85% | +1.35 pp | pass | 37.20-43.27% |
| 10 | 14 | 101 | 365-72-563 | 40.10% | 40.20% | -0.10 pp | fail | 37.11-43.17% |
| 10 | 15 | 102 | 386-62-552 | 41.70% | 40.20% | +1.50 pp | pass | 38.68-44.78% |
| 10 | 16 | 103 | 401-84-515 | 44.30% | 41.70% | +2.60 pp | pass | 41.25-47.39% |
| 10 | 17 | 104 | 376-87-537 | 41.95% | 44.30% | -2.35 pp | fail | 38.93-45.03% |
| 10 | 18 | 105 | 379-69-552 | 41.35% | 44.30% | -2.95 pp | fail | 38.34-44.43% |
| 10 | 19 | 106 | 343-68-589 | 37.70% | 44.30% | -6.60 pp | fail | 34.75-40.75% |
| 11 | 5 | 111 | 346-71-583 | 38.15% | — | — | pass | 35.19-41.20% |
| 11 | 6 | 112 | 345-76-579 | 38.30% | 38.15% | +0.15 pp | pass | 35.34-41.35% |
| 11 | 7 | 113 | 355-64-581 | 38.70% | 38.30% | +0.40 pp | pass | 35.73-41.76% |
| 11 | 8 | 114 | 326-65-609 | 35.85% | 38.70% | -2.85 pp | fail | 32.94-38.87% |
| 11 | 9 | 115 | 348-64-588 | 38.00% | 38.70% | -0.70 pp | fail | 35.04-41.05% |
| 11 | 10 | 116 | 328-70-602 | 36.30% | 38.70% | -2.40 pp | fail | 33.38-39.33% |
| 12 | 5 | 121 | 109-27-864 | 12.25% | — | — | pass | 10.36-14.43% |
| 12 | 6 | 122 | 56-22-922 | 6.70% | 12.25% | -5.55 pp | fail | 5.31-8.42% |
| 12 | 7 | 123 | 158-55-787 | 18.55% | 12.25% | +6.30 pp | pass | 16.26-21.08% |
| 12 | 8 | 124 | 110-37-853 | 12.85% | 18.55% | -5.70 pp | fail | 10.92-15.07% |
| 12 | 9 | 125 | 134-36-830 | 15.20% | 18.55% | -3.35 pp | fail | 13.11-17.56% |
| 12 | 10 | 126 | 214-50-736 | 23.90% | 18.55% | +5.35 pp | pass | 21.36-26.64% |
| 12 | 11 | 127 | 185-72-743 | 22.10% | 23.90% | -1.80 pp | fail | 19.64-24.78% |
| 12 | 12 | 128 | 160-28-812 | 17.40% | 23.90% | -6.50 pp | fail | 15.18-19.87% |
| 12 | 13 | 129 | 101-30-869 | 11.60% | 23.90% | -12.30 pp | fail | 9.76-13.73% |
| 13 | 5 | 134 | 236-59-705 | 26.55% | — | — | pass | 23.91-29.37% |
| 13 | 6 | 135 | 254-64-682 | 28.60% | 26.55% | +2.05 pp | pass | 25.89-31.48% |
| 13 | 7 | 136 | 274-74-652 | 31.10% | 28.60% | +2.50 pp | pass | 28.31-34.04% |
| 13 | 8 | 137 | 289-66-645 | 32.20% | 31.10% | +1.10 pp | pass | 29.38-35.16% |
| 13 | 9 | 138 | 225-47-728 | 24.85% | 32.20% | -7.35 pp | fail | 22.27-27.62% |
| 13 | 10 | 139 | 291-67-642 | 32.45% | 32.20% | +0.25 pp | pass | 29.62-35.41% |
| 13 | 11 | 140 | 297-65-638 | 32.95% | 32.45% | +0.50 pp | pass | 30.11-35.92% |
| 13 | 12 | 141 | 331-86-583 | 37.40% | 32.95% | +4.45 pp | pass | 34.45-40.44% |
| 13 | 13 | 142 | 272-70-658 | 30.70% | 37.40% | -6.70 pp | fail | 27.92-33.63% |
| 13 | 14 | 143 | 260-67-673 | 29.35% | 37.40% | -8.05 pp | fail | 26.61-32.25% |
| 13 | 15 | 144 | 304-68-628 | 33.80% | 37.40% | -3.60 pp | fail | 30.94-36.79% |
| 14 | 5 | 149 | 392-80-528 | 43.20% | — | — | pass | 40.16-46.29% |
| 14 | 6 | 150 | 381-79-540 | 42.05% | 43.20% | -1.15 pp | fail | 39.03-45.13% |
| 14 | 7 | 151 | 374-72-554 | 41.00% | 43.20% | -2.20 pp | fail | 37.99-44.08% |
| 14 | 8 | 152 | 316-64-620 | 34.80% | 43.20% | -8.40 pp | fail | 31.91-37.81% |
| 15 | 5 | 157 | 369-72-559 | 40.50% | — | — | pass | 37.50-43.57% |
| 15 | 6 | 158 | 366-77-557 | 40.45% | 40.50% | -0.05 pp | fail | 37.45-43.52% |
| 15 | 7 | 159 | 350-73-577 | 38.65% | 40.50% | -1.85 pp | fail | 35.68-41.71% |
| 15 | 8 | 160 | 218-50-732 | 24.30% | 40.50% | -16.20 pp | fail | 21.74-27.05% |
| 16 | 5 | 165 | 358-66-576 | 39.10% | — | — | pass | 36.12-42.16% |
| 16 | 6 | 166 | 343-60-597 | 37.30% | 39.10% | -1.80 pp | fail | 34.36-40.34% |
| 16 | 7 | 167 | 384-79-537 | 42.35% | 39.10% | +3.25 pp | pass | 39.32-45.44% |
| 16 | 8 | 168 | 384-71-545 | 41.95% | 42.35% | -0.40 pp | fail | 38.93-45.03% |
| 16 | 9 | 169 | 357-83-560 | 39.85% | 42.35% | -2.50 pp | fail | 36.86-42.92% |
| 16 | 10 | 170 | 369-72-559 | 40.50% | 42.35% | -1.85 pp | fail | 37.50-43.57% |

## Checkpoint hashes

The campaign state and hash-chained ledger contain both checkpoint and optimizer hashes for every
interval. The following is the SHA-256 of each immutable raw PPO checkpoint:

```text
attempt-001  ecd6baba321d93ef5aee30f6ba17bfa3c5c43963dea10c08a5fd85a510d4c187
attempt-002  686836ad79fa2ed732352ed49ec77f50e08b1e93f71e13576d76012b03db52af
attempt-003  29213f15bb439832dd8700951a3c938f4d7d147cd6b881843c4cd231deeac651
attempt-004  da0b2ebb53ca645a00b6a3f5e997e770a6dc825287ea05167cc1846bc16439ff
attempt-005  15545285589874d781243c36ad0d89e315ed289d92e45348bcc37518cb4e8376
attempt-006  bab3e4a55a71188c6fe13510c9630cbee1bf362f87efde2c6203dfd1fa269937
attempt-007  61154bf9ae9f3b825ef5614d48c689866014d05949b718027a5b5f86baa83a29
attempt-008  414e5f1f6c26fccb2a501fb1cba74822f4271ff060d55d3fdd2f8214893ef778
attempt-009  ccd21b19ff61059a2cf1b62645b8985492ffd3a7735582142d0500426b1e606e
attempt-010  baf5c2eb64554b2537b26e217ba4d4f6143aba29da3a5322ec77c4137afb9992
attempt-011  a51482a038c7a254d416d98473568df8c7a059635eefaf9ec9bdc99ee802a238
attempt-012  c3bcf1e3fe854ba4dd5f5010023b843085db41e2c9c11e05a031ac2d8e8277d0
attempt-013  e182581fbbddb8f55906b7935b6d4e604f61b8ab5f710013fde112c30c34fa53
attempt-014  96f66f9c82b1daa66bdee2fb3e7817ca874f8d259849d52a96b6c1f5c48d4df0
attempt-015  0e56d256373b986eaeb261ddaea1d130bdaaf2e8c4f587e58f089c52135a7eb6
attempt-016  973e8379e0cfc973ce9df00ada0b743c603654f84cc4be32bb2635944a705f29
attempt-017  996fa39911e35e30ab89088da42716bd5b8030211f2bbb2e66e489ec657efced
attempt-018  e39a0f6d3a4fdea48b2e4843de82272be2b352245332bc15ce74a3a9055e8f64
attempt-019  1f0335de2d565d94e4edd839acae2bcd4843f523184298637d0c69c0209dafbe
attempt-020  6df30a6296aebf5d309e1fbef504d704a02e0840cd99b277a4dbcd181f3c2341
attempt-021  ce44673c43fd7e818c960fb1827b7c5141e42112a8a4563dfb0abbc0a17b1202
attempt-022  b72a419545d639a38e448621924843f19bb122b2956f9012f9ebad44479378cd
attempt-023  72884b7d638d0e8f6c3871bb27fc25f681f972e83a5287db83d31a5da8201b54
attempt-024  dbd4c13dea4c3c36aabbe1421636a139244366bc3c5de691d3d4511ccd2ae85f
attempt-025  cb9209e5bac7bad8e6f4ac33a322b92457016384695669cafe67629ba21ccc82
attempt-026  501c88db465b0b65036de2d9fa6d15be52bb9ee383f3e04d6881c27cdab68465
attempt-027  1b891bf50a1b5f7c4420b62108a14fb66e1d5afe5cf516b57caedeb41413af81
attempt-028  0a4b8d8515496da670284e744092f3192864cfdabf69ff3750b0b884d352462c
attempt-029  46309eaa2d52cdde1d24d1a00b2bd1394157efb1979b1fca454567174a67da74
attempt-030  39b8c688ad2d03f8d674f16ebf87bd69f7bc973435b625c2e0dbcfa5b036cabf
attempt-031  e751ba62f96d0ebbd758bace2aa071ba58c22209b0e8e5a4fc2362d89efe46cc
attempt-032  b92bc1c56841df3232d78a466171c758c62d1f0001afaf41943570b62aa2ee3e
attempt-033  a33526ec2e11cb2bafe936a974eb5d715478e86bc41ddb0b2141487f233541e2
attempt-034  f3c6db343eb2d0a81b66ef21242584367d91f2aaa89d81d95e1445931cf69f9b
attempt-035  af2f1cfbe9e561e180cb5d53b6aba64b51a0c2000441f0e5b2cd72c62af970a4
attempt-036  553760c5d6a2cb2199919fceeee62bd4acb52a22c83613d69717265645000ab4
attempt-037  29ef9c62b1755c3de1acbd1294dec3a8a1fd37fb0ccce4f285dc80de3bea1c98
attempt-038  323ecd66b53699c9afaa1c9421e02dce131b5c6e0567878cc7f6298886fd3934
attempt-039  04bb3b0f636c12e65abe6b4d2ebbd2dd352d56ee63899d3203a436e3f7056de0
attempt-040  d40f4575c024816d5b7e57e57866fa2f8995c107516536a223cb7ccd32e6de05
attempt-041  a9f8cf0025f4979478aaf447b8740ee7cea843aac5a970a2f6fb94ba1e3e6965
attempt-042  e3300528601891131e91b6ec208633c96a3821074ab47d46de1a578c72175e55
attempt-043  36cad909d2f6dd373816097b07707c7bef5df84887911692c6e91606f91794d3
attempt-044  67e2132fdecaf13ab13530061e00c4aa004f6fe46d950ed84ba3fdec091a3c74
attempt-045  da45507bf86c52260e35a0eb332b348d5ef6f0167bb78354a7863f7da50b4b99
attempt-046  3f39dadb9cccd0feb8f87a5593c4178ee5b0a42a69f9f6fd5ed8193cdebf16bb
attempt-047  d43b52f60813414324c8403895a9b30cce85983e90f7a7a45baef004dfeb38de
attempt-048  3982bdbae8b69712e0113b7fb367c50ed81d1fae75bce53209e2554e384566fe
attempt-049  04a7fc7b0e23780b38b5bbb2077fb845cee731071ad5be8f0b30cf3027bad9ef
attempt-050  ee8052366a7873ed62468275550e45c2514c4977033264c3f4b633455e611b69
attempt-051  d8054fb364ece5f79aef0ecb457395840880a29429e7c456ecc892311ba873aa
attempt-052  cc336810cd3bcc0a50b444ec2bee797c782a36a3a4e8826dd2e0988f187fdcb9
attempt-053  85398b0362c4f4496c18d87e46c1df7d7143eceb9b1b6a0dc40a6b7790be2e10
attempt-054  79141fc8d5a66f54114d6fe626d4921d530a2243ef69b7331b129d0f2056c8ce
attempt-055  c8993867ea74e348c229a7cc54e72df2a7ced41e1ff22844805cc0a8a5afa6a4
attempt-056  b9245b6140538376db109b5e8c5d587b3a98a082c7c401c7fa320d1bbf87461a
attempt-057  07f12c5c8c5da544b73595fa381df3ba9e2ddb86434ee61795810ed9afa79ba2
attempt-058  bed57302fa66efca557d8b27920ac80bc09aa5ef16d76de48e86ab4bd4ce4326
attempt-059  ba87ba3ef2036152f81a1eb74e1d3777dab1fd55d87ada3853ea96d868bc1b95
attempt-060  9ba63a1e014c2d5740c814e5a2eba3c63ead63241dba6bbbc6a28444cdf5b065
attempt-061  33267faed0c6ceaf6d2e1cebb5c9b02bc4ac3b26d39da245fba97d9cdbb8d7b0
attempt-062  f776d9d8df5bdaedc41e7ce45fb81509eaf55e9ab43739a69484073572cc8360
attempt-063  3f89ee34e6bc4def886a4105e4eb3cd32f2d940856b3123dcaccc57b4ec0bb0a
attempt-064  4bf34eafce6cf2712d4cbc6ee295dae2b5379c789629e85a439c72d2ded00b50
attempt-065  19bc7d30adc40df71c0322cd608f038cc900ed76aefd3f15ea265d34d8af05d7
attempt-066  b2119e393f18ee36a94b556e29cbc1aca39a93ad4119de9d650f3ae9d8de9e21
attempt-067  f6b0cb17e19d95b86a7be34763518f988b0a2d9db36779c4b0f3650570706834
attempt-068  824a4bd6836f5f313a5b81969bf114206cefab919036e4ef2fa8734782b9e773
attempt-069  59e4630c3d8c8da062eeb61bb0ffc43224c73c7855c0027983d590ca2827a5eb
attempt-070  235e77113da0b1e9759f12206b945ee7fa1ca2376df5780da22362bec8eda53a
attempt-071  5c2216776643f2ad7dd213d84983eaca50a58b8529b68bbbbf7b761a1e1bdf99
attempt-072  e3d95346cf2b7153f9a73ff6ec8638aba04795cc21687df114c9afe1f7f36213
attempt-073  30da04d44d3b664918e9eff80b2bc49fe80373310a426852611b3eb0bf0b1612
attempt-074  7fc8b2c2e4b0273f60df373052ad89772cf4f57f41a03be1cccdc1379e733fbd
attempt-075  f59780dfbf2d25d223e7fa18706138a471de7ca8620ead310cf9b1fc91d77011
attempt-076  a7e8a56e3641f162cb42ac3e25352ac598115eaef71d9e2eb6d6d3b9e6735b10
attempt-077  fd75ba3ecfad9d58182aec7250d6c573a33c7bb42533d4ad4704c2931b949e04
attempt-078  bee922ad646a6c3828ffe5bed8b351b9f1f48fde15d7d1e26d720840a211753b
attempt-079  0c3eeeca8f8c9f00c7fe29a7263525de6383f3479692a83c30a72c36b7507477
attempt-080  4974e772c0614506bce4f672e8deac76ace8361bbf8b13e9ffdf875abc435eef
attempt-081  43c041ac0ee4c20e16d0d08a8fef4146c9b2038ace2a0a2df163babf0a9f3103
attempt-082  59b9c68077191c61a149a61af71e17db674a1501372245ff9cf8d392f37a3723
attempt-083  b09851f9efcd64773e3f674bba1d7045b079a8cd948befcea7eb785567aaca8a
attempt-084  ac794bc2d19bdc893701b832ccb61913ed329c7b8dd8c2832c15f1afc304f830
attempt-085  680e6f07ab8ba4aee77df9abfed8d398573669a988ee57c9447adb61353ea150
attempt-086  cc1dccfaae673f5f4f749e166d88223f3daa09a8312a1d188440cdad59146930
attempt-087  7280fe46c471d546ab384e2aa659015cbc29852df803eb16071eafbaed16c469
attempt-088  62514752242f0f521cd0f5e50a93279a56c4107ec6dddfacdfad8831e9d6c00c
attempt-089  62d93270a1e1c10232572d63385df78b6d3e1214fdc6912820fbf802fbc083a2
attempt-090  3d64420e930952a57f43d836e7f7e39175ec7c7fdfaa56f6831f5e7d22e37334
attempt-091  a6cac991f7732313a8da7654cfdf0fa5faea41436312850628691b4ed6610b2c
attempt-092  45c914051d546123c28d260ba9c6fdd5a1d73a971229fce856d9a0a44064d3d5
attempt-093  59d8a3bce7547d2be3a67139be1e91825497bcaed3a7d77eeece5009ba67f577
attempt-094  2916ef922512ede040c8843a7956e57aefa6ac1b841333ff76bddfaaed53237c
attempt-095  7eff7fc11ef39891119b742a5efa8ae6a9bb53de20558bf1b7fe98d91273a0fb
attempt-096  8dfce82c3fbc4b2f747c0e2d801c8575fed9c55919329c863ad69ff29e24621a
attempt-097  0cfbdce6126e73f23fca5482169708a4d54ac5c6312495c9e07520668ef8babc
attempt-098  a1d15ef5eea45009b6abf3041a2216bc5d07a37cb3b0fd84877519d2d177129d
attempt-099  ead57aa2089e4dfd4b7fc073daca7998c195bc0e5a7ea5a8c757f70617e67f14
attempt-100  e1278ec0dc48dc67e17189b25afcd34580330ef9f15eaf71da6753ca38792392
attempt-101  81646f2a80316a2464393b8b92ddc1c31223c3c7b10c587283b63accef6d42ed
attempt-102  6f0f31d40a7cb4521ee762153cced9f97f1ea1e62146bb993939ddfdb28ee460
attempt-103  361a971f903ba1a96ac53ed73b05a75b0df8a82d107ba9b398b5d2b741f16c7e
attempt-104  39f4bcf5987adb443e489b1ae797260f74db9914634370a8e0684ea664906d6c
attempt-105  bdf5e478cc50243ee8ca873266eb61d2f70107208b00ba9cfe4c8dca1050fdfd
attempt-106  7d150050181ab58342b284d55e4b24977849d8cb191bebb14d2bd72fe86f31dc
attempt-107  e976e45fa5f7314713fdcf6debe45c1df138b74f623fa6ce77c2f268e7c229db
attempt-108  9baeaccd7a752ca5a025d5ebfdd49aec1b71c25232128156025bfd77c927ac47
attempt-109  deddf8c9bdb071d3f23842fecee5bc4703ce012c4d91518430547754885441be
attempt-110  87fbf57e21ba384557e1cff3f0616dae48cdc2a35579bad5421139db725bb7d7
attempt-111  ed9ae29e0b9a54ad8c6f4868dbe6c60b3d70ac11a844f7f7581213c1bff9aab7
attempt-112  6ccca76a0d3343c8f05be686837a5048cd60cc15a52eefe1d1f116a1aec63780
attempt-113  21ac552f49af2662cbaa97ec98a02ac2bc310d1fdd62a3cec4c703d2d61f2a5b
attempt-114  dc3ef8d4dec203a64f96d7de024f09b815db291f3bad93b3d3ac65a81da85050
attempt-115  7a35a42a2ba959273e5b0b8ac7b9cd660acc00ed322807d07bac29a73ef78684
attempt-116  9a020279661485627de3f0dc349fbcdabf276e83f23cc1a4185a66fce926a64d
attempt-117  d1b64ae72313866ba95b2da742d197c3a55f76a231ca802f235dfa3fe0babdcb
attempt-118  e4f81edfd3df312c8f9ba947719a6745aea77055d1a684b5cf8827105c0265bf
attempt-119  e888b826dc0c746d6b1234a1912d7345b976ad94da1eb399b7323c7176a5430a
attempt-120  fc03f392b11e64d0e54e3edd1da1e2e002ef33e0d1ca37adac6ad02d4fca4862
attempt-121  0cf0c3f5527983d81e3661cf28bf63cb419a66e4e41f08b3820a8d0976f94ecf
attempt-122  d8b3546d0d527b9a60bdd0933564feb71e3128a9c6f564296c5f490fa90083c4
attempt-123  d439a139fe5f7d49176a4c0071d24a2c51d1e9a01761090abe89b740d7e00bee
attempt-124  5c179801332f461ed710408eb35ac56ad16e2b79d15b0ba38c2bbaefd1078cd0
attempt-125  e2cbff86def50d43b8a44708c4db091def815bd5bd3960b31f6170184b00a857
attempt-126  a2e73990cdad00af672849ffa7cac431cc2c9a011688fd2a9af37a397b5d1d85
attempt-127  1b54b19856654494c16867d24665ad5b597e1a7e12b8369f3bbc85bf2fd7e0d9
attempt-128  503a1367aaf75c9f990f9bee26f6951bfc1b369fa2b3db05c38f4596dec88873
attempt-129  446f52b2d196662735f9d44ed01a451e95627fae2a22f8595bccf15f9b9dd60c
attempt-130  ae90aa1d54391eee08e2b7c2087ff81ed520ba65bc25ae0029246a4829adf5b1
attempt-131  896132cfcd26eebd12859b162b03881004803c1abd392009dc71c6f3f79b1398
attempt-132  9c3978230b92f3932dbbcfc0b3ba8308d180bf79aa9322ed3b7efbe85b82ec34
attempt-133  b9926050ab59f1ff1e6c1cf780e17e45d79a633493e3ba3c96b20c02f97f69bb
attempt-134  43183013f3cece96a45ab5bf5e3e96c4ed9aa37329d397021f1c47350bfca4ec
attempt-135  7f52694ca5eac3c9e2f8c33c9fc1c1ac000cb290388290d80ec228b713e93ed0
attempt-136  1c18c19ddaebf18d8efe39051ec548529aa6949153c46a73826dddfae5f2b302
attempt-137  28efde98c7fce34e9eae0477d74ed5ad4bec3872426680f402a090619995671d
attempt-138  6f7cfd76b989e15302b82e3a48708f939025abccee8ae26a800d3b4687f8a879
attempt-139  51cc0cf3bf1a2eee38d04ca34103f923986660bba3c70a0232ef08737882da13
attempt-140  0eb305fd7286f91a2e0ee2b79a05f665150ec814e914321ebceab4652a86894d
attempt-141  b093cdfea0c02b30b6b13827b69e841b4e287e0df5f7e5d0c6bb5daac66bcf96
attempt-142  5f1a76f03d14bc3ec7936471f126ae6cee38e1274c8fbc8a7e94bfafc33a152c
attempt-143  67ec183abce38be50972b5df54e9b4f047c893c0f1fdc6615ea55f3dc9779e3d
attempt-144  d149cf3f26b7502b7f3032d490ba2e06c5ce9dd0f8be155c6ecb68b839b11a5c
attempt-145  abb9b6204ef959734a3cf1ce6b174cb447ad08214ef450bb8ceaa2ebb2f0e226
attempt-146  81916bbef01496876e69ec9578d7b447626b47b9261767bdb60a83c135f2e9a0
attempt-147  214ffef0801fe45b11b4061f5d11a317621b42226edb71fe5193c4cdabc79a3c
attempt-148  b866f565a48c62faccba10c38eef81e7c1f4ae42f26e12b5f7e56e04a604ef96
attempt-149  404584e6537a38161aeb7e09a13d1130dc91c94e06d0975685a2507c0f016a93
attempt-150  0da1576d96a7fddd5b229b2364cc89365035118f3de0291a309ac08133f0c338
attempt-151  44d9bdfd952eededb26e3506115baa8e0e21e05b1009ada4cf11b1bb97035545
attempt-152  ddfb329e8904a4e4e26dd4295ec493899e5fd10a3d3f5a7b47c06e66150c0c9b
attempt-153  94ade259485e46894ab711488175480ebdde2c6cb4220205ec772bf17a025c79
attempt-154  a9766d3f0182189a4d43b2f58180a3fd364277b6be7560bd72ca565d157da8ba
attempt-155  5449c39e57ddbc8be0d340950c5cb9f21c805471fd823f7eb4e6d7a5e587e04b
attempt-156  f4bd39acc4b67c6c8f30dceae26850837ee847f4d5722af546d193d3c4b15269
attempt-157  20d44d22c2e9684b532bc658db1bbd102ec88340bb7004bc3b5d983557ccd12a
attempt-158  fadef934785838a9d72a8f738613a28885d6cfdac3a2495dc500a0ef7957668b
attempt-159  7b2d86aa1272e67b329e9288c0a4df30c0a3604f25ae8c8494bffacb5fd5b7f8
attempt-160  a4409da4829073cf0ee6822f7b9ef8407c045d811f468c8c5b93af863b6536b8
attempt-161  6bc53b99a6733b9fd6e9ee67aa5fb85b462c4cf2a56a0b11f1dca31d507fdf0e
attempt-162  c0a2e42327ce435fbc8da03a34cec6c2c8e8ed753588efab2b77b5e8d5fc6bf8
attempt-163  2c67d48da0c1e3676a9ac83c6ea4b5c8fe57d532b7980e979bc3b673292bfd83
attempt-164  9f5f95882f716a34453c0086e7e348411e606dd01ae10259c599278026d2e54d
attempt-165  41d31f90e8eceafcb2bfa2a23def21b53137943000bfd9a94bb918ea87518ca7
attempt-166  e345737905117b37883fc682bcf881e79687e59dfd1aebdc5fca3ff6698697a3
attempt-167  ba81fbb943de6a71d3e90ad570e7333893f5da3ecc3d59d376614a0131b4b3e4
attempt-168  5ff01bef9842034f2a3164b9871479c20e730d31797d72cab629bcbd1ee67629
attempt-169  20cd79aa2b5c15f857bce1605e49e0ed47c656a1a40b8e8e046aff8f8a04fbad
attempt-170  4f39bb9cc967903c7746fa05d09240d45382d4134d39c7092c40bebca0e7d0cd
```

## Safety, history, and export parity

The audit summed every training, checkpoint-gate, final-comparison, and replay safety field. All
were zero: authority rejects, hidden/state leaks, invalid action indices, replay mismatches, stale
actions, truncations, and wrong-seat actions. `post_ppo_correction=false` is present on all 170
intervals. No teacher rewrite occurred after any PPO update.

The append-only experiment ledger contains **357 verified records** and ends at record SHA-256
`f6cbb67c1ef8eea3449dabc659f31f6a522331aad653982bac069c6dc738d146`:

| Event | Count |
|:---|---:|
| campaign started/completed | 1 / 1 |
| experiment started | 170 |
| baseline interval completed | 64 |
| checkpoint high watermark | 46 |
| checkpoint below high watermark | 60 |
| new model family started | 15 |

The best checkpoint was exported as preserved evidence only; it was not deployed or substituted
for the global champion:

- Model ID: `v3-global-highwater-campaign-20260804-attempt-073`
- Export SHA-256: `b9e93e656bc3acbc5e269469191a2179b255cce54e9c7918d94fd56487345271`
- Source parameter SHA-256:
  `e8b8e9a1439f1aa5a47ce304daf5d6c8348deaa7cd16d231c0f6e98a9e5580c2`
- Python self-play seed: `5300730000`; 146 actions; seat-A winner; safety zero.
- Replay SHA-256: `96353c41d100fe4266568297764be21c040247d5810dc25da84fa50dee5f0615`
- Go replay verifier: PASS; identical 146 actions, policy hash, and terminal winner.

## Acceptance evidence

| Gate | Result | Final evidence |
|:---|:---:|:---|
| A - Champion control | PASS | Raw CP400 identity/hash frozen; five-checkpoint baseline, strict high-watermark tie behavior, three-miss failure, `>50%` 1K/3K promotion rule, no rollback, and deadline rules covered by tests and the real run. |
| B - History | PASS | 357-record append-only hash chain plus human Markdown rendering; every start, baseline, pass, miss, family transition, and completion recorded. |
| C - Pool integrity | PASS | Explicit CP400 curriculum and current-family-only history; ordinary files cannot silently enter the pool. |
| D - Observation completeness | PASS | Frozen V3 manifest and capacity audit cover statuses/stacks, forms, abilities, cards, dice, health, and viewer-visible state; overflow fails preflight. |
| E - Privacy/parity | PASS | Python/Go schema and replay parity; all hidden/state leak counters zero. |
| F - Model compatibility | PASS | V1/V2 remain loadable; V3 is separately versioned and exports complete hashes/manifests; Godot integration test lists all versions and raw CP400. |
| G - Teacher safety | PASS | Fresh Mechanics-V2 warm start only; zero-PPO starting models; no post-PPO correction; teacher cannot promote. |
| H - Learning telemetry | PASS | Every interval records PPO metrics, parameter movement, behavior, opponent/seat distribution, throughput, timing, and safety. |
| I - Campaign timer | PASS | Exact monotonic 28,800-second deadline; checkpoint 170 completed once under grace; no checkpoint 171. |
| J - Authority safety | PASS | All aggregate safety counters zero across training and 107,000 evaluations. |
| K - Outcome | PASS (global held) | Best campaign gate 460-56-484 (48.80%); final fresh bank 460-64-476 (49.20%); hashes, confidence, seeds, swaps, and fail decision preserved. |

## Final verification

All checks were rerun after the campaign and export:

```text
uv run ruff check .                              PASS
uv run ruff format --check .                     PASS (40 files)
uv run pytest -q                                 PASS (61 tests)
go test ./...                                    PASS
go vet ./...                                     PASS
go test -race ./internal/battle/...              PASS
dice-and-destiny-server/scripts/build_native.sh  PASS
./scripts/godot.sh --headless --script \
  res://scripts/verify_battle_authority.gd        PASS
./scripts/godot.sh --headless --script \
  res://tests/phase3/verify_learned_battle_v2.gd PASS (V1/V2/V3/raw CP400)
campaign artifact audit                          PASS
Go V3 replay parity (attempt 73)                 PASS (146 actions + winner)
```

The final artifact audit additionally proved: all 170 raw checkpoint file hashes match state;
every interval is exactly 52,224 steps; parent/optimizer progression is continuous on misses; all
32 fresh bases load at zero PPO timesteps; all 106 checkpoint episode files and the final episode
file contain complete 500-seed seat swaps; no undeclared 3K block exists; and the family-plan,
seed-manifest, and experiment-ledger canonical hashes verify.

## Artifact hashes

Raw evidence is intentionally ignored and preserved beneath
`dice-and-destiny-server/ml/runs/v3-global-highwater-campaign-20260804-8h/`.

| Artifact | SHA-256 kind | SHA-256 |
|:---|:---|:---|
| `campaign-state.json` | bytes | `94f92c874d1559b11ae9d47068056e0ddb48bc59e01399d6ae71ef337a26bf82` |
| `champion-registry.json` | bytes | `547a837a5bcfcd9bd8b5ec6ef36d516379c17b0592bcb477feb5a13a0a6df7df` |
| `history/experiments.jsonl` | bytes | `7e1d672ae753d8c94c69755b7c2f74cd871de8beeb19eeaa32367260d28c803c` |
| `history/experiments.md` | bytes | `3f88e96c203f204e430323f111f0c6ccfcec90fa48a3598d63a976651638474c` |
| `preflight/campaign-preflight.json` | bytes | `cfe49ea5c819a1f2fb96d82a5e763bac845dcef74abd2a4c281220ff14987140` |
| `seed-banks/manifest.json` | bytes | `75df220a596c6d7903f54204cfbdecbd4db9598cd0ef491b6c576b795959f49e` |
| seed-bank manifest | canonical | `cff51c09954ccbbe11489fca858a4167fb870575170aa3ef471e926ee1044d64` |
| `model-family-plan.json` | bytes | `0d60ecc285a845ff5f547530ec30772e4c05b03bd4223144af1fdcfeec1ad4d3` |
| family plan | canonical | `1017703d61dba0c54e9cbce97733fe93ff74493ba79a9e1fc595d5c8d59a8a0e` |
| `observation-manifest-v3.json` | bytes | `b820241a137f5ea3ffdc2ba42512e34dee8ac40b67bd916c00e3b3fe307097eb` |
| observation manifest | canonical | `5ede31f0126be159f4a7f3ff04a94973643a237d586f806818792c9ce29316e2` |
| `final-comparison/summary.json` | bytes | `dde01df8f0bb6a9c79a7114921343b74ef146d50a2d7ea2b7188052c85cee30a` |
| best evidence export | bytes | `b9e93e656bc3acbc5e269469191a2179b255cce54e9c7918d94fd56487345271` |
| export self-play replay | bytes | `96353c41d100fe4266568297764be21c040247d5810dc25da84fa50dee5f0615` |
| ledger tip | canonical record | `f6cbb67c1ef8eea3449dabc659f31f6a522331aad653982bac069c6dc738d146` |

## Files and migration

The implementation continues the Observation V3/champion infrastructure documented in
`three-hour-champion-campaign-report.md`. The main rule changes are in `campaign.py`, with CLI and
documentation wiring in `cli.py` and `ml/README.md`, and regression coverage in the ML test suite.
The existing V1/V2/V3 runtime, raw checkpoint-400 model, corrected deployable V3 model, and all
prior run artifacts remain preserved. No model was overwritten, deleted, or promoted.

## Recommended next experiment

The strongest evidence favors **replicating 128/2/GELU across several independently warm-started
system-random seeds** before continuing architecture traversal. Family 8 reached 48.80% on its
campaign bank and 49.20% on the independent final bank; the nearby 128/1/GELU family reached
46.55%. However, this campaign assigns one seed to each architecture, so architecture and seed
quality are confounded. A separate campaign that holds 128/2/GELU fixed and changes only the
replication protocol would test whether the near-50% result is reproducible and would give a much
better estimate of that architecture's true performance.
