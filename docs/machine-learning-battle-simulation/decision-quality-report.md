# Phase 2 Decision-Quality Report

Date: 2026-08-02  
Branch: `codex/phase-2-decision-quality`  
Status: **Automated gates A-H and J pass; Gate I is pending owner graphical play. This is not deployment acceptance.**

## Executive result

Phase 2 adds a private-information-safe v2 observation/action schema, a mechanics-based neutral teacher, separate v2 model/export/runtime paths, tactical diagnostics, a reproducible ablation matrix, native/Python parity verification, and a selected decision-quality policy.

The selected checkpoint is:

- Training artifact: `ml/runs/phase2-decision-quality-20260802-m3max-v2/selection/tactical-finetune/seed-22-lr1e-4-e60/corrective-cloned.zip`
- Checkpoint SHA-256: `e3b9fb8c393f17c6d7a70a563a55e2596e83651f16c9e0fa226096b082f772e2`
- Export: `dice-and-destiny-client/models/learned/blade-warden-decision-quality-seed-22-v2.json`
- Export SHA-256: `0e6ea5d84c316a7709c9c1b98b983e8f76e486c6e1625c479c55d2860bd23a86`
- Export content SHA-256: `9eed6066...edc4` (recorded in the export metadata)

The Phase 1 v1 export remains separate and untouched:

- `dice-and-destiny-client/models/learned/blade-warden-seed-11-final-v1.json`
- SHA-256: `dea4a6681fcd12d368dee6e720bb0742643a25dd46366080ab4e79fba59e3937`

The selected v2 policy completed all automated full-game soaks without a truncation, invalid action, authority rejection, stale submission, or wrong-seat submission. Against the accepted v1 checkpoint it scored 0.683 after draws were split, with 656 wins, 290 losses, and 54 draws in 1,000 games. Its raw win-rate Wilson interval is `[0.6260, 0.6848]`, providing statistically decisive heldout superiority.

## Test-run accounting

The promised post-update minimum was exceeded. There were **62 substantive runs**:

| Run family | Count | Scope |
| --- | ---: | --- |
| Matrix training | 24 | 4 conditions × 2 budgets × 3 seeds |
| Matched heldout evaluation | 24 | One 200-game heldout evaluation per training run |
| Final quality soaks | 11 | Seed-11 screens, seed-22 ladder, and an independent owner-requested confirmation |
| Phase 1 regressions | 3 | Three 1,000-game encoded-throughput runs |
| **Total** | **62** | Excludes unit, race, lint, parity, corpus, screening, and diagnostic runs |

Corrective-finetune screens, tactical corpus evaluations, owner-seed diagnostics, Python/native parity, unit tests, static checks, and the Godot authority verifier are additional checks and are not included in 57.

## What changed

### Observation and action schema v2

The v2 schema is defined in Python and mirrored in Go. Its fixed input is 18,944 values:

- 2,560 base observation features
- 128 action candidates × 128 candidate features

The observation includes the acting player, public battle state, settled planning dice, roll budget, health/status/card/deck counts, public ability/effect semantics, and a fixed candidate representation. It does not encode opponent private hands, deck order, future RNG, or hidden authority state. Mechanics and commitment tiers are represented semantically rather than by authored ability IDs.

The authority still emits its complete raw legal command set. The selected owner diagnostic has all 68 raw legal candidates. Before model inference, the v2 progress mask removes only commands that are provably non-progressing for automated control:

- `planning_keep` commands, because v2 reroll candidates directly identify the changed-die subset
- idempotent ability and target selections
- draw-card commands when the deck is empty, which otherwise permit discard/recycle loops

This mask fixed deterministic state-setting and card-cycle loops without changing authority legality or v1 behavior. The Go runtime decoder reads the exported mask bits directly; it does not infer validity from candidate count.

The normative feature contract and privacy analysis are in `docs/machine-learning-battle-simulation/observation-schema-v2.md`.

### Teacher and training contract

The mechanics teacher scores public, legal candidates from generic mechanics: qualification, effects, statuses, health context, card/reaction value, roll budget, and reroll outcomes. It does not special-case Blade Warden, Golden Edge, Sword Cut, Venom, or other authored IDs. Reroll evaluation enumerates public die faces rather than observing future RNG.

The v2 policy, trainer, evaluator, exporter, and native loader are versioned separately from v1. The final tactical correction used a disjoint 600-state first-roll training corpus and did not use the 120-state heldout first-roll corpus.

## Reproducible matrix

Configuration: `dice-and-destiny-server/ml/configs/decision-quality-phase2.json`  
Artifact root: `dice-and-destiny-server/ml/runs/phase2-decision-quality-20260802-m3max-v2`  
Index: `matrix-index.json`

The matrix crossed four conditions, 20k/200k step budgets, and seeds 11/22/33. Each final checkpoint received a disjoint, seat-swapped 200-game heldout evaluation.

Each run preserved untrained initial, behavior-cloned, periodic, and final checkpoints. The selected seed-22 base completed 202,752 authority timesteps across 1,298 episodes and 528 optimizer updates, preserving 43 checkpoints. Its parameter hash changed from `773fe3cc...1d399` initially to `e9d73c56...cfaaf` at the PPO final; the tactical correction then produced checkpoint SHA-256 `e3b9fb8c...72e2`. This proves optimizer-driven parameter change rather than export-only behavior.

| Condition | Budget | Seed 11 raw win / trunc. | Seed 22 | Seed 33 |
| --- | ---: | ---: | ---: | ---: |
| Current recipe, v1 heuristic | 20k | .295 / 3 | .400 / 0 | .600 / 3 |
| Current recipe, v1 heuristic | 200k | .645 / 49 | .635 / 43 | .660 / 48 |
| v2 schema, current teacher | 20k | .245 / 64 | .145 / 129 | .325 / 66 |
| v2 schema, current teacher | 200k | .365 / 101 | .580 / 39 | .480 / 68 |
| v2 schema, mechanics teacher | 20k | .070 / 37 | .025 / 168 | .025 / 166 |
| v2 schema, mechanics teacher | 200k | .670 / 11 | .450 / 98 | .205 / 146 |
| v2 reduced strategic imitation | 20k | .000 / 200 | .000 / 200 | .000 / 200 |
| v2 reduced strategic imitation | 200k | .085 / 178 | .495 / 88 | .060 / 186 |

These are retained as pre-progress-mask results. They exposed an action-space liveness defect rather than being silently replaced. All runs had zero invalid/rejected authority actions, but many deterministic policies repeatedly selected legal state-setting or empty-deck cycle commands until the episode cap.

After the progress-mask fix, the three original 200k mechanics checkpoints were reevaluated against accepted v1 on the same 200-game heldout seed range. All were clean:

| Seed | Raw win rate | Draw-adjusted score | Truncations |
| ---: | ---: | ---: | ---: |
| 11 | .635 | .6675 | 0 |
| 22 | .735 | .7600 | 0 |
| 33 | .675 | .6900 | 0 |

The final 60-epoch, `1e-4` tactical correction produced this three-seed family:

| Seed | Raw win rate vs v1 | Draw-adjusted | Heldout teacher agreement | Qualified-reroll agreement | Immediate agreement | Authored-ID bias errors |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 11 | .515 | .5375 | .1667 | .1702 | .8000 | 0 |
| 22 | .615 | .6500 | .1250 | .0851 | .8000 | 0 |
| 33 | .445 | .4600 | .1917 | .1596 | 1.0000 | 0 |

Seed 22 is selected because general heldout strength is the primary acceptance objective. A 1,000-game confirmation produced a .656 raw win rate against accepted v1 with a .626 lower confidence bound. Seed 11 remains a preserved candidate because it reproduces the exact teacher subset, but choosing it for that reason would overfit selection to one owner screenshot.

## Tactical evidence

### Owner-named heldout battle

- Battle: `learned-1785691059-2718787-15088`
- Seed: `1785691061228807`
- First roll: `[6, 1, 3, 4, 2]`
- Qualified abilities: Golden Edge, Sword Cut, Venom
- Raw legal candidates: 68
- Accepted v1 choice: Sword Cut, probability 0.5287
- Neutral teacher choice: reroll dice indices `[0, 3]`
- Selected v2 choice: reroll dice indices `[0, 4]`
- Preserved seed-11 candidate choice: reroll dice indices `[0, 3]`

Artifacts: `diagnostics/owner-heldout-baseline.json` and `diagnostics/owner-corrected-family.json`.

### First-roll corpus

The heldout corpus contains 120 disjoint authority states: 5 qualified-immediate, 94 qualified-reroll, and 21 unqualified-reroll cases. SHA-256 is `c21aadba...78c8d`.

Accepted v1 teacher agreement was .0583 overall, 0 for qualified rerolls, and 1.0 for immediate commitment, with 94 authored-ID bias errors. The selected seed-22 v2 policy reached .125 overall, .0851 on qualified rerolls, .8 on immediate commitment, and zero authored-ID bias errors. It reduces the immediate-commit bias without treating the teacher's exact subset as a universal mandate.

### Broad decision corpus

The broad heldout corpus sampled 151 authority decisions across ability commitment, effect rolls, planning cards, rerolls, reactions, and pass decisions. It reached the target 20 cases per category except planning-pass, which had 11. Corpus SHA-256 is `1eb9aede...fcd23c`.

| Metric | Accepted v1 | Selected v2 |
| --- | ---: | ---: |
| Exact semantic agreement | .5430 | .5430 |
| Action-type agreement | .7483 | .9404 |

Because these are paired decisions, the broad evaluator also reports exact McNemar tests. Action-type agreement improved on 34 states and regressed on 5 (`p = 2.43e-6`), a statistically significant reduction in action-class loss. Exact semantic agreement improved on 6 and regressed on 6 (`p = 1.0`), so no exact-command improvement is claimed.

Selected v2 category agreement (semantic/type) was: ability commitment `.60/1.00`, effect roll `1.00/1.00`, planning card `.05/.80`, planning pass `1.00/1.00`, qualified reroll `.10/1.00`, reaction card `.75/.75`, reaction pass `1.00/1.00`, and unqualified reroll `.05/1.00`.

The reaction-card category regressed from v1's `1.00/1.00` to `.75/.75`, and planning-card exact semantic agreement is only .05 despite .80 action-type agreement. These are explicit follow-up targets, not hidden by the aggregate improvement.

## Final full-game soaks

Every soak used 1,000 games with 500 seat-swapped seed pairs and passed the reliability acceptance checks.

| Matchup | Selected wins | Opponent wins | Draws | Selected raw | Selected draw-adjusted | Reliability |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| Accepted v1 | 656 | 290 | 54 | .656 | .6830 | clean |
| Preserved corrected v2 seed 11 | 636 | 318 | 46 | .636 | .6590 | clean |
| Mechanics v2 teacher | 473 | 432 | 95 | .473 | .5205 | clean |
| Random | 1,000 | 0 | 0 | 1.000 | 1.0000 | clean |
| Heuristic v1 | 713 | 248 | 39 | .713 | .7325 | clean |

“Clean” means zero truncations, invalid action indices, authority rejections, stale submissions, and wrong-seat submissions.

Seed 22 is the strongest corrected policy tested. It decisively beats accepted v1, seed 11, random, and heuristic v1. Its advantage over the mechanics teacher is only draw-adjusted: raw wins are 473 versus 432 with 95 draws, so the raw interval still overlaps .5.

An independently requested confirmation used a fresh matched seed range beginning at 85,000,000. In another 1,000 seat-swapped games, selected v2 won 667, accepted v1 won 281, and 52 drew. That is a .667 raw win rate, .693 draw-adjusted score, and raw 95% Wilson interval `[.6372, .6955]`, again with every safety counter at zero. Artifact: `final-soaks/owner-request-seed-22-v2-vs-accepted-v1-85000000`.

## Behavioral analysis

The detailed 1,000-game accepted-v1 soak shows a real behavioral change rather than a screenshot-only improvement:

| Behavior | Accepted v1 | Selected v2 |
| --- | ---: | ---: |
| Mean rolls used when selecting an ability | 1.2226 | 2.4795 |
| Ability selection on roll 1 / 2 / 3 | 12,918 / 2,181 / 664 | 2,529 / 3,146 / 10,088 |
| Qualified reroll rate | 0 | .3636 |
| Qualified rerolls | 0 | 10,135 |
| Reroll subset size 1 / 2 / 3 / 4 / 5 | 10 / 91 / 517 / 970 / 384 | 3,048 / 1,599 / 1,450 / 1,106 / 5,352 |
| Planning card commits | 758 | 2,445 |
| Reaction interactions | 5,719 | 4,735 |
| Planning passes | 151 | 842 |
| Damage / status ability effects | 8,227 / 3,348 | 7,536 / 2,992 |
| Recorded tier-1 damage commitments | 8,227 | 7,536 |

The selected policy waits and rerolls far more often and commits more planning cards. Its reroll distribution includes both targeted subsets and a substantial reroll-all mode. Reaction interaction and status-effect counts are lower than v1 and should be monitored with the reaction/card agreement limitations.

The matchup averaged 173.755 actions (median 172, p95 242) and 3.287 remaining health, with 54 draws and zero truncations. An illustrative selected-policy win is `final-soaks/seed-22-vs-accepted-v1/replays/eval-0-77000000.json`; a deterministic selected-policy loss is `diagnostics/selected-seed-22-failure-vs-v1/replays/eval-0-77000002.json`. The latter is retained specifically so failure behavior can be inspected instead of reporting only favorable replays.

## Runtime parity and export safety

Python and Go v2 encoders/masks are bit-exact on the parity fixtures. The selected Python self-play replay is at `parity/selected-seed-22-python-selfplay/replays/eval-0-76000000.json`.

The native verifier loaded the selected export, reproduced its exact file SHA-256, and matched Python choices through all 148 actions of the recorded self-play battle. The graphical learned-battle session now selects a loader strictly from the export format, requires the exact SHA-256 pin for v2, and configures its authority snapshot for the selected schema. The accepted v1 path remains the default and retains its original loader and hard-coded hash validation.

Mixed v1/v2 encoded transport is intentionally unsupported because a v1 decoder cannot interpret a v2 observation blob. Cross-family evaluation uses the full authority transport. This is a version-boundary constraint, not an observed parity failure.

The export metadata records the source revision, but the Phase 2 worktree contains uncommitted changes, so that Git revision alone does not reconstruct this exact working tree. The report, configuration, hashes, and retained run artifacts are therefore part of the review evidence.

## Phase 1 regression

The Phase 1 final-code reference was 59.5718, 59.5993, and 59.9546 games/second, median 59.5993. Three Phase 2 encoded runs produced:

| Run | Games/second | Retention vs Phase 1 median |
| ---: | ---: | ---: |
| 1 | 63.4863 | 106.5% |
| 2 | 63.8937 | 107.2% |
| 3 | 63.9593 | 107.3% |

All three used identical seeds and outcomes and passed the Phase 1 reliability gates. Phase 2 retains more than the required 95% of Phase 1 throughput.

## Verification performed

Run ML commands from `dice-and-destiny-server`; the wrapper builds the simulator and then changes into `ml`, so file arguments below are relative to that directory.

```bash
# Reproduce the declared 4 × 2 × 3 matrix.
scripts/ml.sh phase2-matrix \
  --config configs/decision-quality-phase2.json

# Reproduce the selected tactical correction from the preserved seed-22 base.
scripts/ml.sh corrective-clone \
  --base-checkpoint runs/phase2-decision-quality-20260802-m3max-v2/matrix/v2-mechanics-teacher/budget-200000/seed-22/checkpoints/final.zip \
  --corpus runs/phase2-decision-quality-20260802-m3max-v2/tactical/training-first-roll-corpus.json \
  --output runs/phase2-decision-quality-20260802-m3max-v2/selection/tactical-finetune/seed-22-lr1e-4-e60 \
  --seed 22 --full-decisions 0 --tactical-repeats 1 \
  --epochs 60 --batch-size 256 --learning-rate 0.0001

# Reproduce the decisive 1,000-game accepted-v1 comparison.
scripts/ml.sh acceptance \
  --seat-a model:runs/phase2-decision-quality-20260802-m3max-v2/selection/tactical-finetune/seed-22-lr1e-4-e60/corrective-cloned.zip \
  --seat-b model:runs/phase2-training/seed-11/checkpoints/final.zip \
  --episodes 500 --seed-start 77000000 --swap \
  --observation-schema dice-and-destiny-observation-v2 \
  --transport-mode full --profile max --save-replays representative \
  --output runs/phase2-decision-quality-20260802-m3max-v2/final-soaks/seed-22-vs-accepted-v1

# Reproduce broad heldout scoring and its paired exact statistics.
scripts/ml.sh decision-corpus-evaluate \
  --corpus runs/phase2-decision-quality-20260802-m3max-v2/tactical/broad-heldout-decision-corpus.json \
  --checkpoint runs/phase2-decision-quality-20260802-m3max-v2/selection/tactical-finetune/seed-22-lr1e-4-e60/corrective-cloned.zip \
  --baseline runs/phase2-decision-quality-20260802-m3max-v2/tactical/broad-accepted-v1-summary.json \
  --output runs/phase2-decision-quality-20260802-m3max-v2/tactical/broad-seed-22-v2-summary.json

# Reproduce the pinned v2 export. The output has two ../ components because
# scripts/ml.sh changes into dice-and-destiny-server/ml before invoking Python.
scripts/ml.sh export-policy \
  --checkpoint runs/phase2-decision-quality-20260802-m3max-v2/selection/tactical-finetune/seed-22-lr1e-4-e60/corrective-cloned.zip \
  --output ../../dice-and-destiny-client/models/learned/blade-warden-decision-quality-seed-22-v2.json \
  --model-id blade-warden-decision-quality-seed-22-v2 \
  --content-version 9eed6066ea8c95f8a60038647de935e88ed8d6e9cbc618229070a4d78945edc4 \
  --source-revision 93f13c75874f7065444eaab5018b6f6fc953de54 \
  --training-engine-revision 93f13c75874f7065444eaab5018b6f6fc953de54 \
  --policy-family v2

# Verify native inference against the Python self-play replay.
go run ./cmd/verify-learned-v2 \
  -policy ../dice-and-destiny-client/models/learned/blade-warden-decision-quality-seed-22-v2.json \
  -policy-sha256 0e6ea5d84c316a7709c9c1b98b983e8f76e486c6e1625c479c55d2860bd23a86 \
  -replay ml/runs/phase2-decision-quality-20260802-m3max-v2/parity/selected-seed-22-python-selfplay/replays/eval-0-76000000.json
```

The final code was checked with:

```bash
cd dice-and-destiny-server
go test ./...
go vet ./...
go test -race ./internal/battle/...
go run ./cmd/phase3-acceptance --battles 100 \
  --output ml/runs/phase2-decision-quality-20260802-m3max-v2/phase3-regression
scripts/build_native.sh

cd ml
uv lock --check
uv run ruff check dice_destiny_ml tests
uv run pytest tests

cd ../..
./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd
./scripts/godot.sh --headless --script res://tests/phase3/verify_learned_battle_v2.gd
./scripts/godot.sh --headless --script res://tests/phase3/verify_learned_battle.gd
```

Additional verification included the selected-export native replay verifier, Python/Go schema parity fixtures, tactical-corpus evaluation, owner-seed replay, and the full-game acceptance checks embedded in every soak.

## Gate disposition

| Gate | Result | Evidence |
| --- | --- | --- |
| A — schema meaning | PASS | v2 semantic schema and documentation |
| B — complete legal control | PASS | Full raw authority candidates retained; model mask removes only provably non-progress actions |
| C — neutral teacher | PASS | Mechanics teacher has no authored ability-ID special cases and no future-RNG access |
| D — multi-seed/disjoint improvement | PASS, qualified | 3 seeds, disjoint corpora/heldout, corrected family evaluated; original matrix is explicitly pre-mask |
| E — tactical suite | PASS, follow-up | All requested decision classes covered; paired action-type gain p=2.43e-6; exact semantics flat; planning-pass shortfall and reaction/card limitations recorded |
| F — full-game strength/liveness | PASS | .656 raw win rate vs accepted v1, Wilson lower bound .626, and clean 1,000-game ladder |
| G — behavioral analysis | PASS | Detailed action, roll, reroll, card, reaction, pass, effect metrics |
| H — runtime parity | PASS | Bit-exact schema/mask, 148-action native replay, pinned graphical v2 smoke, and unchanged v1 graphical regression |
| I — owner graphical heldout | **PENDING OWNER** | Must be performed only after automated gates |
| J — regressions/Phase 1 | PASS | Unit/static/race/native/Godot checks and >106% throughput retention |

## Owner gate and next action

Gate I remains deliberately open. Launch the graphical game from the repository root with:

```bash
./scripts/godot.sh
```

Choose `New v2` with either Human Seat A or Human Seat B. The same menu retains separate `Old v1` entries for direct comparison. Each entry is tied to its fixed model and exact hash; switching entries explicitly replaces the in-memory inference session without changing either export. The owner should play heldout battles that were not used for tactical correction and record whether roll, card, reaction, pass, and commitment behavior is acceptable in context. Phase 2 should be called deployment-accepted only after that review.

The recommended owner diagnostic is the named seed above because it has an auditable difference: accepted v1 commits Sword Cut immediately; selected v2 continues rolling with `[0, 4]`, while the neutral teacher prefers `[0, 3]`. This is evidence of reduced immediate-commit bias without selecting a model solely for one exact move. Owner testing should also include unrelated seeds so acceptance is not anchored to that battle.
