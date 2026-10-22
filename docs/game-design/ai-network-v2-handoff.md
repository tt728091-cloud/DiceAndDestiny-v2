# Handoff: AI network v2

**Status:** in progress (written 2026-10-22, AI thread, branch
`ai-value-network`).

**Plan of record:** [`ai-value-network-plan.md`](ai-value-network-plan.md),
section "Network v2", steps N1–N6. Results go in
[`balance-lab.md`](balance-lab.md).

**Why:** the first value network (T1–T6 of
[`ai-value-network-handoff.md`](ai-value-network-handoff.md)) judged
positions well but changed no moves. The user wants the network to be the AI,
not an add-on: "it was the whole point".

**The user's instructions (2026-10-22):** proceed with the plan, make the two
open decisions (in the plan), report back when done. The standing rules of
the first handoff hold:

- Run Godot only through `./scripts/godot.sh`; keep the Go server the
  authority; the AI never reads hidden state (the human's hand or future
  dice). Every encoder reads the viewer-safe view, and every future goes
  through `lookahead.Fork`.
- Never weaken a test or cherry-pick seeds to pass.
- Don't touch the board work (`internal/battle/boardprice`, `cmd/board-lab`,
  `content/progression_v1/board_templates.yaml`, `riders.yaml`, the custom
  adventurer, the Board Builder screen) or the gear work. Don't change cards,
  abilities or other content to improve results.
- Don't push or merge unless the user asks.

**Ownership:** as in the first handoff (`lookahead`, `mlsim`, `valuenet`,
`learned/lookahead_policy.go`, `cmd/balance-lab`, `cmd/value-probe`, `ml/`,
the two plan docs and `balance-lab.md`), plus the new
`internal/battle/decisions` package.

## Tasks

### V3 — Ship Rounds 1 with the health race (decision 1)

- Lab at in-game settings, budget mode: v2 (Rounds 0, budget 160) against
  Rounds 1 at budget 160 and 400, Starter mirror and Brine Mask, seeds 1–400
  from both seats.
- Ship the best Rounds 1 setting that beats v2 in both matchups (lower 95%
  bound ≥ −0.01 against Brine Mask) and takes no more time per decision.
- `LookaheadArchitecture` becomes `flat-monte-carlo-health-race-v3`;
  `balance-lab -tally` keeps v1, v2 and v3 apart. Godot authority check and
  the look-ahead mirror script test.

### N1 — Decision benchmark and headroom

- `lookahead`: `SearchPolicy.Options` (the search's options, with
  `TopAbilities`) and `SearchPolicy.OptionValues` (every option on the same
  futures).
- `decisions`: the record format (gzip JSON lines; the pinned catalog once
  per file), `Recorder` (a lab observer: hero seat only, sampled, its own
  randomness so the battle is unchanged), `Read`.
- `balance-lab -label FILE` and `-score FILES`.
- Benchmark v1 under `ml/runs/bench/v1/` (git-ignored), commands in
  `balance-lab.md`.
- Headroom lab: Rounds 1 at budget 250 and 1,000, Rounds 0 at 250, Starter
  mirror.

### N2 — Rider values by paired futures

`cmd/value-probe -paired` (or a new command): positions from recorded
battles, each rider added on one of two identically seeded forks at its
natural moment, both played to the end; ratio to the unit with a 95%
interval (bootstrap over positions).

### N3 — Training data

`balance-lab -label` at 128–256 futures, sample 1, look-ahead heroes, all
non-held-out pairings, seeds from 400,001. `valuenet` gets the `value-v2`
state encoder and the option encoder, with viewer-safety, determinism and
width tests, and a converter from decision records to float32 shards.

### N4 — Network v2

`ml/dice_destiny_ml/policy_train.py`: state ⊕ option → advantage over
greedy's pick, plus the result head. Export `dice-and-destiny-policy-v2`,
loaded in Go with check rows. `balance-lab` player `network` (its own pick,
no search) for `-score`. Gate on benchmark v1.

### N5 — Network in the search

`SearchPolicy` options: the network's pick as the anchor, its top options
as the candidates; the network as the rollout policy; the value head at the
cut. Equal-time lab tests against Rounds 1 with the health race. Ship as
version 4 only if the plan's exit passes.

### N6 — Self-play loop

Re-label with the N5 player, retrain, gate, test; stop when the gain is
inside the noise.

## Verification

```bash
cd dice-and-destiny-server && go test ./...
cd .. && ./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd
cd dice-and-destiny-server/ml && uv run pytest
```

If the in-game opponent changes: `./scripts/godot.sh --headless --script
res://tests/phase3/verify_lookahead_mirror.gd` and the timing from a lab run
at in-game settings.

## Final report

For each step: what was built (commits), the results tables, the tests run
with pass/fail, what shipped in-game, and where it stopped and why.
