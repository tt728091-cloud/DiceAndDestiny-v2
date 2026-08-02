# Phase 3 learned-opponent handoff report

Date: 2026-08-01

Status: **AUTOMATED GATES PASS — Phase 3 remains incomplete until owner UI
acceptance is confirmed.**

Branch: `codex/phase-3-learned-opponent`

Reviewed Phase 2 baseline: `fff39759360ce89041940cec99410082af0671dd`

The owner reviewed the implementation and authorized committing and pushing this
branch on 2026-08-01. The formal three-battle manual gate remains tracked below
until its complete-battles, rematch, and both-seat checklist is explicitly
confirmed; automated proxy play cannot satisfy that gate.

## Accepted frozen policy and compatibility contract

The reviewed Phase 2 evidence passes categories A–G and identifies seed 11's
final Maskable PPO checkpoint as the accepted policy. Its held-out, seat-swapped
evaluation completed 100/100 wins against random and scored 51 wins, 43 losses,
and 6 draws against the transparent heuristic. The accepted artifact is:

- Checkpoint: `dice-and-destiny-server/ml/runs/phase2-training/seed-11/checkpoints/final.zip`
- Model ID: `blade-warden-maskable-ppo-seed-11-final-v1`
- Checkpoint SHA-256: `e2e98c2ecadfe9e06892676962e751a116cb07183aa193882d73927866d760b8`
- Parameter SHA-256: `b53c6633893bbaca6dd429edb6f1aeb94585e6c24cf05c2dbc1451e2fd758f7a`
- Phase 1 training-engine revision: `5d81e8f6de350a3bdcc3f4ccf42a3447443f83fe`
- Phase 2 implementation revision: `fff39759360ce89041940cec99410082af0671dd`
- Content hash: `9eed6066ea8c95f8a60038647de935e88ed8d6e9cbc618229070a4d78945edc4`
- Environment schema: `dice-and-destiny-ml-env-v1`
- Observation schema: `dice-and-destiny-observation-v1`
- Action schema: `dice-and-destiny-action-candidates-v1`
- Observation: 4,224 `float32` values: 128 viewer-state features plus
  128 candidate rows of 32 features
- Maximum candidates: 128, with unused rows zero-padded and masked
- Training dependency lock: Python 3.12.7, Torch 2.13.0, Stable-Baselines3
  2.9.0, sb3-contrib 2.9.0, Gymnasium 1.3.0, and NumPy 2.5.1

The graphical runtime does not ship those Python dependencies. A deterministic
exporter extracts only the accepted actor's context `[64,64]`, candidate
`[64,64]`, and score `[64,1]` tensors. The tracked 562,579-byte export is
`dice-and-destiny-client/models/learned/blade-warden-seed-11-final-v1.json`
with export SHA-256
`dea4a6681fcd12d368dee6e720bb0742643a25dd46366080ab4e79fba59e3937`.
The Go loader pins that complete export SHA-256 before decoding and also rejects
any mismatch in the identifiers above, format, dimensions, tensor names/shapes,
or a freshly computed content hash. A weight-mutation test proves that accepted
metadata cannot conceal a changed actor. Golden tests reproduce
the original Python policy's decisions at multiple changing authority windows.

## Graphical start, play, result, and rematch flow

Run `./scripts/godot.sh` from the repository root. The first visible screen now
offers three modes:

1. **Classic Battle — Blade Warden vs Venom Goblin (D100)**, preserving the
   existing path and active-battle resume behavior.
2. **Learned Mirror — Human Seat A**.
3. **Learned Mirror — Human Seat B**.

The learned options start a Blade Warden mirror and label the opponent
**Learned Policy · Blade Warden**. During each learned decision the opponent
shows a visible thinking state, all player controls are locked against duplicate
submissions, and inference runs on a Godot `Thread` while rendered frames keep
advancing. A measured two-second limit produces a visible actionable error and
diagnostic record; there is no fallback controller. A failed decision exposes
**Retry Learned Decision** and **Return to Mode Menu**. Retry operates on the
same unchanged authority decision, while leaving the mode submits no substitute
action.

The completion screen derives victory, defeat, or draw from the authority's
winner and shows final health. **Rematch · Same Seats** creates a fresh seeded
battle while retaining the same in-memory policy. **New Battle · Change Seat or
Mode** returns to the graphical mode menu. Neither action restarts Godot or the
inference runtime.

## Complete human UI input coverage

The learned mode reuses the normal responsive battle screen. Actor aliases are
presentation-only: visible `blade` commands map to the selected human seat and
visible `goblin` targets map to the model seat before ordinary authority
validation. The reachable input surface is covered as follows:

| Reachable human decision | Visible Godot control |
| --- | --- |
| Start/mode/seat | Three mode-menu buttons, including learned Seat A and Seat B |
| Planning roll | **Roll 5 Dice** |
| Keep and reroll | Click dice to select keeps, then **Reroll Unkept**; the keep and reroll are each authority submissions |
| Planning pass | **Pass Planning** or **Pass Defense** |
| Offensive ability and target | Ability tiles; mirror target is the visible learned Blade Warden |
| Defensive ability and source target | Select an incoming-source tile, then a defensive ability tile |
| Defense roll | Click the visible pending player defense die |
| Status-effect roll | Click each visible pending status die |
| Cards | Click a legal hand card; supported selectors expose self, enemy, owned die, selected die, owned offensive ability, incoming source, or negative status controls |
| Reactions and confirmations | Legal card controls plus **Pass / Acknowledge** in offensive, defense, damage, status-roll, and status-damage windows |
| Die adjustment | Visible rolled-die target buttons, including blind/status die adjustment |
| Hand limit | Toggle the exact cards, then **Discard selected cards (n/n)** |
| Status choice | A visible button for each legal negative status target |
| Presentation acknowledgment | Visible continuation controls on queued result beats |
| Result/rematch/new battle | Authority-reported result, **Rematch · Same Seats**, and **New Battle · Change Seat or Mode** |

The focused Godot test clicks the graphical classic mode and verifies the Venom
Goblin D100 screen, clicks graphical learned Seat A, submits a real planning roll
through its rendered button, and activates the graphical rematch. It also
exercises a failed asynchronous learned decision, its visible diagnostics and
recovery buttons, a successful retry, a complete learned battle, stale
rejection, hidden state during unrevealed planning, both human seats, responsive
frames during thinking, and persistent model load count one. The existing
real-playthrough test also reaches planning, defensive selection/roll/reaction,
offensive/damage/status reactions, status roll, hand limit, and card flows.

## Inference and authority architecture

`LearnedBattleRuntime` is an application autoload owning one native authority
object. Its first learned battle validates and loads the frozen export once.
Every battle and rematch reuses that object and resets only the in-memory battle
environment and per-battle telemetry.

For a model turn, the persistent Go session asks the real authority for the
model seat's viewer-filtered result and complete current legal commands. The Go
encoder produces the exact Phase 2 observation and action mask. The pure-Go
actor scores valid candidate rows and returns one index. The original command at
that index is passed through the ordinary authority step; no rule or state
mutation is implemented in the policy layer. The result is filtered for the
human viewer before crossing the native bridge back to Godot.

Human commands follow the inverse alias mapping and the same ordinary authority
path. Submitted actor, battle, pending-input, window checkpoint, seat, and exact
candidate membership are all checked. Stale, fabricated, wrong-seat, invalid,
or rejected commands fail visibly. No training, optimizer, Python process,
per-decision model load, heuristic, random substitute, or silent fallback exists
in this path.

Per-battle JSON-lines diagnostics record model/checkpoint/schema/revisions,
seed, actual seats, every human and model authority command, selected candidate
index, result, per-decision inference latency, errors, timeouts, fallback use,
and safety counters at the launcher-isolated workspace path
`dice-and-destiny-client/.godot/runtime/workspace/user/phase3/learned-battles.jsonl`.

## Automated both-seat acceptance

The Phase 3 runner used one loaded instance of the actual tracked accepted actor
for 100 consecutive real-authority battles, alternating the proxy human between
Seat A and Seat B. Seeds were `30000000` through `30000099`. It records every
actual command and selected index in ignored run artifacts at
`dice-and-destiny-server/ml/runs/phase3-acceptance`.

| Measure | Result |
| --- | ---: |
| Battles | 100 (50 human Seat A, 50 human Seat B) |
| Human-perspective results | 10 victories, 86 defeats, 4 draws |
| Actual learned-policy decisions | 7,781 |
| Model loads / process restarts | 1 / 0 |
| Mean / p95 inference latency | 0.181 ms / 1.145 ms |
| Mean / p95 battle duration | 390.051 ms / 498.426 ms |
| Crashes / errors / timeouts | 0 / 0 / 0 |
| Authority rejects / invalid actions | 0 / 0 |
| Stale / wrong-seat actions | 0 / 0 |
| Hidden-information leaks | 0 |
| Unexplained truncations | 0 |
| Fallbacks | 0 |

The learned action counts were 3,315 passes, 1,478 rolls, 1,434 ability
selections, 732 planning rolls, 542 interactions, 173 planning rerolls, 86 card
commits, and 21 planning passes. The machine-readable checker returned
`acceptance_passed: true` with an empty failure list. It additionally asserts a
contiguous per-battle command sequence, the current battle ID on every command,
controller/seat agreement, decision trace/count agreement, unique reset
identity, and fresh seed metadata.

## Owner-reported defense-roll regression

The first owner review found a learned-mirror battle stopped after selecting
Basic Defense. The authority had correctly advanced to `defense_roll`, whose
legal `roll_dice` candidate omitted the optional `request_id`; the graphical
builder incorrectly added `"request_id":""`, so exact candidate validation
rejected the otherwise valid blank-die click as stale/fabricated. The builder
now omits that optional field when the pending input has no source ID.

The same screen also labeled an unresolved three-damage Venom Strike as
`Final 0`. That value was the source structure's pre-resolution default, not an
authority damage result. Unsettled sources now display `Pending 3` (less any
prevention), while `Final` is reserved for a settled damage batch.

The focused Godot integration test replays the reported seed
`1785623447766714` through the real frozen-policy session, selects Basic Defense,
clicks the visible blank defense die, and asserts one accepted human authority
decision with zero stale actions and no UI error. Presentation coverage also
asserts the exact payload omission and pending-damage label. Learned diagnostics
now use the launcher-isolated workspace runtime path instead of the global
Godot `user://` location.

## Owner-reported offensive-reaction regression

The next owner review exposed a separate offensive-reaction presentation flaw.
The learned opponent legally played Tip It after the human passed, changing its
revealed die and therefore changing Venom Strike into Shield Bash. The client
continued to render the pre-reaction roll-history dice, logged only the generic
text `card played`. This made the authority-approved second response
opportunity look like an unexplained second attack selection.

Played offensive reaction events now publish the public card definition. Godot
shows an explicit explanation such as `Learned Blade Warden played Tip It: die
1 changed to face 5; Venom Strike → Shield Bash`, and the dice tray prefers the
latest authority reveal over stale roll history. The second response screen is
retained because the opponent's reaction changed the revealed state and the
human must be allowed to reconsider before passing again. The exact owner seed
`1785634159965507` and its full keep/reroll/Sharpen Blade/Sword Cut trace are now
replayed through graphical integration coverage, which asserts two visible,
graphically actionable reaction passes, the explained die/ability change, and
the corrected defense-die flow.

## Verification completed

All commands below passed on 2026-08-01 unless noted as a diagnostic-only
warning:

```bash
cd dice-and-destiny-server && go test ./...
cd dice-and-destiny-server && go vet ./...
cd dice-and-destiny-server/ml && uv run --python 3.12 pytest
cd dice-and-destiny-server/ml && uv run --python 3.12 ruff check .
cd dice-and-destiny-server && ./scripts/ml.sh smoke --seed 20260801
cd dice-and-destiny-server && ./scripts/ml.sh evaluate --seat-a model:runs/phase2-training/seed-11/checkpoints/final.zip --seat-b heuristic --episodes 1 --seed-start 20260802 --swap --output runs/phase3-checkpoint-smoke
cd dice-and-destiny-server && go run ./cmd/phase3-acceptance --battles 100 --seed-start 30000000 --output ml/runs/phase3-acceptance
dice-and-destiny-server/scripts/build_native.sh
./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd
./scripts/godot.sh --headless --script res://tests/phase3/verify_learned_battle.gd
./scripts/godot.sh --headless --script res://tests/test_client_logic.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_battle_scene.gd
./scripts/godot.sh --headless --script res://tests/scenarios/verify_real_playthrough.gd
git diff --check
```

Python reported 7/7 tests passing. The Godot presentation-layout test passed at
1920x1080 and 1280x720 but emitted its pre-existing process-exit RID leak
warnings; no changed functional assertion failed. The accepted Phase 2 smoke and
two-game swapped model-versus-heuristic probe completed with the model winning
both games, mean inference 0.243 ms, and zero reject/invalid/stale/wrong-seat or
truncation counts.

## Requirement-by-requirement audit

| Requirement | Current evidence | Status |
| --- | --- | --- |
| Accepted competent Phase 2 checkpoint | Phase 2 A–G report, held-out swapped evaluations, exact archive/parameter hashes, reproducible Python-to-Go golden decisions | **PASS** |
| Frozen inference only | Tracked actor-only export; pure-Go scorer has no optimizer, training API, or Python process | **PASS** |
| Human/model split and authority-only mutation | UI alias commands and indexed model candidates both enter ordinary Go authority methods; other layers only consume returned snapshots | **PASS** |
| Graphical selection and preserved D100 path | Focused Godot test clicks classic D100 and learned Seat A; the menu exposes learned Seat B | **PASS automated** |
| Complete graphical human decisions | Normal screen covers every current Blade Warden selector/input; presentation tests click planning/reroll/card/status/defense/effect controls and a real learned roll reaches authority | **PASS automated readiness; owner play pending** |
| Viewer-safe model input and complete mask | Phase 1 snapshot/event tests, learned encoder tests, and Python golden equivalence; mask population equals candidate count | **PASS** |
| One persistent model load | Go two-battle test, graphical rematch, and 100-battle lifetime count all equal one | **PASS** |
| Off-loop inference, duplicate guard, timeout, diagnostics, recovery | Godot worker and locked controls; wrong-seat guard; measured no-action timeout; visible error, retry, and mode-exit test | **PASS** |
| No concealed fallback | No fallback implementation; telemetry records its counter; timeout and soak assert zero | **PASS** |
| Authority result and graphical rematch/new battle | Terminal result mapping, actionable rematch test, visible new-battle control | **PASS automated** |
| Versioned diagnostic record | JSONL contains model/export/checkpoint/schema/revisions, seed, seats, command trace, result, latencies, errors/timeouts/fallback | **PASS** |
| Both human seats | Godot Seat B opening model turn plus the 50/50 alternating soak | **PASS automated** |
| 100 consecutive actual-checkpoint battles | Strict checker passes 100/100 with 7,781 accepted-model decisions and all prohibited counts zero | **PASS** |
| Three owner battles, one rematch, both seats, Godot-only input | Requires owner observation and confirmation | **WAITING — Phase 3 incomplete** |

## Remaining risks and incomplete acceptance

- Owner UI acceptance has not occurred. Phase 3 is therefore **incomplete** even
  though every automated implementation and acceptance gate currently passes.
- This proves one accepted Blade Warden policy in one mirror matchup. It is not
  roster-wide behavior, a difficulty system, or consumer packaging.
- The export is a deliberately strict, architecture-specific inference format.
  A retrained model, content change, schema change, or architecture change must
  be reviewed and re-exported rather than accepted implicitly.
- The two-second timeout is measured around pure actor scoring. Failure is
  surfaced with no gameplay fallback; the player may retry that unchanged
  authority decision or return graphically to the mode menu.
- Automated layout coverage cannot replace subjective review of pacing,
  readability, thinking-state duration, and the clarity of every unusual card
  or status selector.

## Owner manual-UI acceptance checklist

Start the app once with `./scripts/godot.sh`; terminal use ends after launch.
During all battles, use only visible Godot controls.

1. Choose **Learned Mirror · Human Seat A** and complete a full battle. Exercise
   every choice the battle presents through the UI; confirm the opponent is
   labeled learned, thinking is visible, input locks while it acts, and the
   authority result appears.
2. Click **Rematch · Same Seats** and complete a second battle without restarting
   Godot. Confirm the matchup resets cleanly and no terminal input is requested.
3. Click **New Battle · Change Seat or Mode**, choose **Learned Mirror · Human
   Seat B**, and complete a third battle. Confirm Seat B is human-controlled and
   the learned policy controls Seat A.
4. Across the three battles, explicitly confirm that planning rolls,
   keep/reroll, abilities and targets, defense, reactions, passes, cards,
   hand-limit/status choices when reached, result, and rematch were all graphical
   and that no CLI, raw JSON, or terminal battle input was needed.
5. Report any missing control, misleading label, unreadable state, hang, model
   error, result mismatch, or rematch/state carryover. Phase 3 may be marked
   complete only after these checks pass.
