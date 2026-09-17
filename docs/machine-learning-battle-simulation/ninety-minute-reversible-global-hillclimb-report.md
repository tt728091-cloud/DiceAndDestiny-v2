# 90-minute reversible global hill-climb report

## Outcome — superseded by direct confirmation

Starting from raw checkpoint 480, the campaign completed 43 independent
one-checkpoint attempts, temporarily promoted 10 under its 1,000-game local
gate, and rejected and fully rolled back 33. It ended with attempt 37
temporarily registered as champion. A later direct 9,000-game comparison did
not confirm that promotion: checkpoint 480 won 4,140 games to attempt 37's
4,018, with 842 draws. **Checkpoint 480 is therefore the current global
champion.** Attempt 37 is retained as a rejected historical candidate:

- Candidate: `v2-global-hillclimb-20260805-90m-attempt-037`
- PPO learner steps: 2,924,160
- Checkpoint: `dice-and-destiny-server/ml/runs/v2-global-hillclimb-20260805-90m/attempts/attempt-037/raw-ppo-challenger.zip`
- Checkpoint SHA-256: `2606bc46c3efbf05a9a158bb916487bb8f792f3bb4a9c957d9f036fb5331c8ef`
- Optimizer SHA-256: `4b67b905af3083906a1d2c1d2398ea1bc891970a63003d8b6c79631b56c3f8e3`
- Parameter SHA-256: `7b796a5b8755733ae4f4ecf6d68dee3846cd710ff83b293b5f88a05c6b3261b8`
- Promotion gate: 454 wins / 112 draws / 434 losses, 51.00% adjusted
- Promotion-gate 95% interval: 47.90%–54.09%
- Gameplay export SHA-256: `35a9c6142869f84f5b42d5f9f8c559dbfb00e93e8ebcee409e8f9f6756116f9d`

Attempt 37 and every intermediate temporary champion remain preserved in the
registry hall. Checkpoint 480 is restored as both checkpoint and global
champion. All rejected checkpoints and evaluations remain on disk; nothing was
deleted or overwritten.

### Post-campaign direct comparison

| Block | Attempt 37 W/D/L | Attempt 37 score | Checkpoint 480 score |
|---|---:|---:|---:|
| 3,000 games, seeds 6800000000–6800001499 | 1,386/250/1,364 | 50.3667% | 49.6333% |
| 3,000 games, seeds 6800001500–6800002999 | 1,299/301/1,400 | 48.3167% | 51.6833% |
| 3,000 games, seeds 6800003000–6800004499 | 1,333/291/1,376 | 49.2833% | 50.7167% |
| **Combined 9,000** | **4,018/842/4,140** | **49.3222%** | **50.6778%** |

Attempt 37's combined adjusted 95% interval was 48.2898%–50.3552%. All 4,500
base seeds were disjoint and played in both seat assignments, and all safety
counters were zero. The canonical registry was reverted to checkpoint 480 on
2026-08-05 after this evidence was audited.

## Rules implemented

The new `global-hillclimb-seed-banks`, `global-hillclimb-preflight`, and
`global-hillclimb-campaign` commands implement the requested campaign mode:

1. Load the current registered global with its optimizer.
2. Train one requested 50,000-step interval. The rollout-aligned actual count
   is exactly 52,224 PPO steps.
3. Save the challenger before evaluation.
4. Evaluate exactly 1,000 seat-swapped games against the current global using
   a fresh, disjoint seed bank.
5. Promote atomically only when the adjusted score is strictly above 50% and
   all safety counters are zero.
6. On failure, preserve the failed artifact, reload the last successful global
   and its optimizer, and retry with the next predeclared training RNG seed.
7. Append every result to a SHA-256 hash-chained history.
8. Never run automatic post-PPO correction.

The checkpoint-480 controls were frozen for the entire campaign. There were no
within-campaign recipe changes and no model-family changes. Observation V2,
action V2, the 96-wide sparse actor/dense critic, terminal reward, learning
rate, PPO epochs, clip range, target KL, entropy, batch and rollout sizes,
gamma, GAE lambda, value-loss coefficient, opponent categories, the original
1,002-checkpoint historical selection, curriculum, teacher frequency, and
alternating seat schedule all remained unchanged. The only varying inputs were
the predeclared training RNG seed and the disjoint evaluation seed bank.

## Preflight and timer

- Campaign ID: `v2-global-hillclimb-20260805-90m`
- Start: 2026-08-05 2:35:56 PM CDT / 19:35:56 UTC
- Nominal deadline: 2026-08-05 4:05:56 PM CDT / 21:05:56 UTC
- Finish: 2026-08-05 4:06:47 PM CDT / 21:06:47 UTC
- Nominal budget: 5,400 seconds
- Actual elapsed: 5,451.18 seconds
- Required deadline grace: 51.18 seconds
- Frozen-controls canonical SHA-256: `f029492c0b80d5a079299839e4c891035a6c18eebe353e715140cd6589c29f23`
- Source config SHA-256: `7e7095010cd91a9b0d3017d09bc951c89bdba6678ea5a154ec73c411ad83d35c`
- Seed-manifest canonical SHA-256: `d47213e1f8415cc64471df019ed3159c532a6d9d5d08636a27946acb021aca3f`

Attempt 43 started with 76.16 seconds remaining. The deadline passed during its
training/checkpoint-save phase. As required, the controller finished the save,
ran all 1,000 testing battles, recorded its 454/81/465 (49.45%) failure, rolled
back to attempt 37, and stopped. Attempt 44 was never started.

## Aggregate campaign statistics

| Measure | Result |
|---|---:|
| Attempts trained and tested | 43 |
| Promotions | 10 |
| Rejections with rollback | 33 |
| Promotion rate | 23.26% |
| Actual PPO work performed | 2,245,632 steps |
| Accepted PPO advancement | 522,240 steps |
| Final accepted PPO steps | 2,924,160 |
| Evaluation games | 43,000 |
| Unique base seeds | 21,500 |
| Seat placements | 43,000: 21,500 per challenger seat |
| Mean adjusted score | 48.52% |
| Best attempt score | 52.25% (attempt 14) |
| Worst attempt score | 42.45% (attempt 29) |
| Safety violations | 0 |
| Post-PPO corrective updates | 0 |

Training used 3,827.53 seconds total, 89.01 seconds per attempt on average
(range 87.57–90.62). The 1,000-game evaluations used 1,536.50 seconds total,
35.73 seconds per attempt on average (range 33.67–36.98). Combined measured
training and evaluation time was 5,364.03 seconds; checkpoint loading/saving,
registry/history writes, process setup, and other controller overhead used
87.15 seconds.

## Complete checkpoint progression

Draws count as one-half. “Pass” means strictly greater than 50.00%, not equal.
Every row used exactly 1,000 games and a unique 500-seed bank with both seat
assignments.

|Attempt|Parent PPO|Candidate PPO|W/D/L|Adjusted|95% CI|Decision|Checkpoint SHA prefix|
|-:|-:|-:|:-:|-:|:-:|:-:|:-|
|1|2,401,920|2,454,144|436/103/461|48.75%|45.66–51.85%|fail|`9fe66406da35`|
|2|2,401,920|2,454,144|432/89/479|47.65%|44.57–50.75%|fail|`791023db6d03`|
|3|2,401,920|2,454,144|419/101/480|46.95%|43.87–50.05%|fail|`6fc587bb2912`|
|4|2,401,920|2,454,144|451/97/452|49.95%|46.86–53.04%|fail|`7431198c9878`|
|5|2,401,920|2,454,144|437/81/482|47.75%|44.67–50.85%|fail|`7c37fb97a61a`|
|6|2,401,920|2,454,144|430/85/485|47.25%|44.17–50.35%|fail|`4d224e9c638f`|
|7|2,401,920|2,454,144|457/112/431|51.30%|48.20–54.39%|pass|`0e02efad32d8`|
|8|2,454,144|2,506,368|441/102/457|49.20%|46.11–52.30%|fail|`5589eb8f7910`|
|9|2,454,144|2,506,368|432/96/472|48.00%|44.92–51.10%|fail|`7ecb97448a92`|
|10|2,454,144|2,506,368|460/93/447|50.65%|47.55–53.74%|pass|`d7068b171fd5`|
|11|2,506,368|2,558,592|441/108/451|49.50%|46.41–52.59%|fail|`1dc0498d4053`|
|12|2,506,368|2,558,592|473/98/429|52.20%|49.10–55.28%|pass|`edbd4ca5361e`|
|13|2,558,592|2,610,816|434/83/483|47.55%|44.47–50.65%|fail|`e463b4435a30`|
|14|2,558,592|2,610,816|471/103/426|52.25%|49.15–55.33%|pass|`f9decfd8b25f`|
|15|2,610,816|2,663,040|432/100/468|48.20%|45.12–51.30%|fail|`7c65cc5107b5`|
|16|2,610,816|2,663,040|467/109/424|52.15%|49.05–55.23%|pass|`4cdfcc0bfdc4`|
|17|2,663,040|2,715,264|452/105/443|50.45%|47.36–53.54%|pass|`78d50c4302bd`|
|18|2,715,264|2,767,488|449/111/440|50.45%|47.36–53.54%|pass|`3c9946f2f744`|
|19|2,767,488|2,819,712|408/96/496|45.60%|42.54–48.70%|fail|`de723f3be05e`|
|20|2,767,488|2,819,712|435/108/457|48.90%|45.81–52.00%|fail|`3724592032fb`|
|21|2,767,488|2,819,712|459/102/439|51.00%|47.90–54.09%|pass|`d0e70ba874c8`|
|22|2,819,712|2,871,936|413/103/484|46.45%|43.38–49.55%|fail|`505de83dae65`|
|23|2,819,712|2,871,936|409/110/481|46.40%|43.33–49.50%|fail|`06ebba2d3926`|
|24|2,819,712|2,871,936|408/88/504|45.20%|42.14–48.30%|fail|`b4089a1f17d4`|
|25|2,819,712|2,871,936|444/91/465|48.95%|45.86–52.05%|fail|`1aa21b303a79`|
|26|2,819,712|2,871,936|408/91/501|45.35%|42.29–48.45%|fail|`7ffc0bde810c`|
|27|2,819,712|2,871,936|415/101/484|46.55%|43.48–49.65%|fail|`c5fe9c29334f`|
|28|2,819,712|2,871,936|407/103/490|45.85%|42.78–48.95%|fail|`5837c9e9140a`|
|29|2,819,712|2,871,936|380/89/531|42.45%|39.42–45.54%|fail|`9a6016f95430`|
|30|2,819,712|2,871,936|443/105/452|49.55%|46.46–52.64%|fail|`55fa5839c2a0`|
|31|2,819,712|2,871,936|446/112/442|50.20%|47.11–53.29%|pass|`5a465501a4e7`|
|32|2,871,936|2,924,160|438/99/463|48.75%|45.66–51.85%|fail|`9fae4d7ee59e`|
|33|2,871,936|2,924,160|420/107/473|47.35%|44.27–50.45%|fail|`7a8c24b13c9f`|
|34|2,871,936|2,924,160|436/107/457|48.95%|45.86–52.05%|fail|`c6c9f0fede2b`|
|35|2,871,936|2,924,160|418/100/482|46.80%|43.73–49.90%|fail|`e5d32405220a`|
|36|2,871,936|2,924,160|424/102/474|47.50%|44.42–50.60%|fail|`213a0cf7a713`|
|37|2,871,936|2,924,160|454/112/434|51.00%|47.90–54.09%|pass|`2606bc46c3ef`|
|38|2,924,160|2,976,384|431/111/458|48.65%|45.56–51.75%|fail|`332d295e5a79`|
|39|2,924,160|2,976,384|445/93/462|49.15%|46.06–52.25%|fail|`d62e21b56d9d`|
|40|2,924,160|2,976,384|443/93/464|48.95%|45.86–52.05%|fail|`69df7fcfefc3`|
|41|2,924,160|2,976,384|429/112/459|48.50%|45.41–51.60%|fail|`0993852d7724`|
|42|2,924,160|2,976,384|440/93/467|48.65%|45.56–51.75%|fail|`5d5ddfdfa3ae`|
|43|2,924,160|2,976,384|454/81/465|49.45%|46.36–52.54%|fail|`61f40988fe57`|

The accepted attempts were 7, 10, 12, 14, 16, 17, 18, 21, 31, and 37.
The accepted lineage therefore moved from 2,401,920 to 2,924,160 PPO steps.
Every failed row's `next_parent_sha256` equals its row's parent SHA; every
passing row's `next_parent_sha256` equals its challenger SHA.

## Artifact and integrity evidence

The final audit reopened and hashed all 43 checkpoint ZIPs, 43 summaries, and
43 seed banks; recomputed all adjusted scores and decisions; verified the
complete parent chain; and recomputed every record in the 43-line hash chain.
It reported `AUDIT PASS []`.

- Campaign state SHA-256: `6c5d63f57a1440a6d4253f84f9d53e3f142c82b64efaef69ae58b1cb797f88d9`
- Attempt-history file SHA-256: `df49cc54f338de23ba173837d18d8620b1b7f58b934970920239e2d64757d28c`
- Attempt-history tip SHA-256: `302cfc8f82b32b76c69f208908be67df925952b464f6ee327937c50cf9d9ef05`
- Frozen-controls file SHA-256: `fe9f62025ab1e942e4dd68527babf32f431a32f66615f91f39bb06c1a99d3992`
- Preflight file SHA-256: `81cddc2967a873fcc892f520c8f58fbdbd8f16a0ca40a9d6d8037a0f70a0aeba`
- Seed-manifest file SHA-256: `f572ce1022f2a0e8c54d02e6cf0c72ce31a9d43d252a83b6cdce262990019eed`
- Campaign console SHA-256: `ee51b2541ea57eb556ca86910f8c18158b95ec95377e7ebd9d3bfe018724cde9`
- Campaign stderr SHA-256: `d964bf32b082fc6b6bfa82b8460b9f493bb07bb4938662abf80cd2ca8fa654e1`

The canonical registry is
`dice-and-destiny-server/ml/champions/current-global.json`. The standalone
promotion record is
`dice-and-destiny-server/ml/champions/promotion-20260805-hillclimb-2924160.json`.

## Verification after integration

- Python ML suite: 65 passed in 2.43 seconds.
- Go server suite: `go test ./...` passed for all packages.
- Godot authority smoke: battle start and 5D6 planning roll passed.
- Godot learned-battle integration at campaign close: the menu listed all
  preserved opponents and loaded the then-selected attempt-37 export by exact
  model ID and SHA-256. A later verification after the direct comparison
  confirmed that the menu/runtime again load checkpoint 480.
- `git diff --check`: passed.

## Interpretation

This campaign demonstrates the intended operational property: stochastic PPO
updates frequently regressed, but none of the 33 failing branches contaminated
the accepted lineage. It also demonstrates that brute-force retries can find a
chain of locally winning 1,000-game checkpoints without tuning controls.

The promotion rule used a point estimate above 50%, not a confidence-bound
requirement. Several temporary promotions had 95% intervals spanning 50%, and
the attempt-37 interval against its immediate predecessor also spanned 50%.
The later 9,000-game direct test supplied the missing comparison and favored
checkpoint 480. Attempt 37 is therefore not the global champion; the experiment
shows a chain of noisy local 1,000-game wins, not a durable improvement over
checkpoint 480.
