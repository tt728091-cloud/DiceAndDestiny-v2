# Pre-Phase 7: Two-Round Multi-Enemy Integration Test

## Purpose

Create one production-path integration test proving that the Phase 1 through
Phase 6 systems work together as a complete battle loop.

The test must start a battle containing:

```text
Player:
- instance ID: player
- definition ID: current_run_player
- controller: human

Enemy 1:
- instance ID: goblin-1
- definition ID: mock_goblin
- controller: AI

Enemy 2:
- instance ID: goblin-2
- definition ID: mock_goblin
- controller: AI
```

It must then complete all five segments in round 1 and round 2. The test
finishes when the battle reaches the first player input in the offensive
segment of round 3.

Reaching round 3 offensive planning proves that rounds 1 and 2 both completed:

```text
round 1 damage_resolution
-> round 2 ongoing_effects
-> ...
-> round 2 damage_resolution
-> round 3 ongoing_effects
-> round 3 income
-> round 3 offensive player input
```

This is a structural battle-loop test. It is not a test of finished combat
content, intelligent enemy actions, status execution, or battle completion.

## Required Result

After this story, the test suite must prove:

1. A production participant assembler can create one player and two independent
   enemy instances.
2. The public authority boundary can persist and reload the same battle for
   every player command.
3. Automatic segments progress without client commands.
4. Both AI enemies participate automatically wherever required.
5. The human player is asked for input only at valid planning or reaction
   checkpoints.
6. Hidden simultaneous offensive planning reveals only after all required
   actors lock in.
7. A shared reaction window closes when the player and both enemies pass.
8. Defensive planning skips when there are no incoming defensible proposals.
9. Damage resolution skips when there are no damage operations.
10. `damage_resolution -> ongoing_effects` increments the round exactly once.
11. The battle can repeat the complete loop without losing or duplicating
    actors, state, events, rewards, or checkpoints.

## Before Coding, Read

- `docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md`
- `docs/v2-planning/godot_pve/07_authoritative_battle_rules_model.md`
- `docs/v2-planning/godot_pve/implementation/phase_1_durable_battle_lifecycle.md`
- `docs/v2-planning/godot_pve/implementation/phase_2_complete_battle_setup.md`
- `docs/v2-planning/godot_pve/implementation/phase_3_nested_resolution_windows.md`
- `docs/v2-planning/godot_pve/implementation/phase_4_shared_planning.md`
- `docs/v2-planning/godot_pve/implementation/phase_5_typed_operations.md`
- `docs/v2-planning/godot_pve/implementation/phase_6_damage_card_health.md`
- `dice-and-destiny-server/internal/battle/authority.go`
- `dice-and-destiny-server/internal/battle/participant_assembler.go`
- `dice-and-destiny-server/internal/battle/engine/engine.go`
- `dice-and-destiny-server/internal/battle/engine/default_flows.go`
- `dice-and-destiny-server/internal/battle/engine/planning.go`
- `dice-and-destiny-server/internal/battle/engine/resolution.go`
- `dice-and-destiny-server/internal/battle/engine/damage_resolution_flow.go`
- `dice-and-destiny-server/internal/battle/command/command.go`
- `dice-and-destiny-server/internal/battle/repository/repository.go`

## Test Location

Add a focused integration test in:

```text
dice-and-destiny-server/internal/battle/multi_round_integration_test.go
```

Suggested test name:

```go
func TestAuthorityRunsPlayerAndMultipleEnemiesThroughTwoCompleteRounds(t *testing.T)
```

The test should remain in package `battle` so it can construct an `Authority`,
use the production file participant assembler, and inspect the repository
checkpoint without bypassing the public command route.

## Required Production Path

Construct the test authority with:

```text
engine.NewEngine()
repository.NewInMemory()
NewFileParticipantAssembler(real content root, real run-player root)
```

Send commands through:

```go
authority.HandleCommandJSON(...)
```

Do not construct a battle directly with `state.NewBattle`, call individual
flows manually, or mutate the battle between commands.

The test must exercise this route:

```text
JSON command
-> command parsing
-> Authority
-> repository load
-> Engine.ApplyBattleCommand
-> automatic progression
-> repository save
-> viewer-safe result
```

The in-memory repository is acceptable for this test. The purpose is to prove
the repository-backed authority lifecycle between commands, not process-restart
recovery.

## Starting Command

Start the battle through the JSON boundary:

```json
{
  "battle_id": "two-round-multi-enemy",
  "actor_id": "player",
  "type": "start_battle",
  "payload": {
    "player": {
      "instance_id": "player",
      "definition_id": "current_run_player"
    },
    "enemies": [
      {
        "instance_id": "goblin-1",
        "definition_id": "mock_goblin"
      },
      {
        "instance_id": "goblin-2",
        "definition_id": "mock_goblin"
      }
    ]
  }
}
```

The authority should automatically process round 1 `ongoing_effects` and
`income`, enter round 1 `offensive`, let both enemies prepare their automatic
commitments, and stop for the player's offensive planning input.

## Current Phase 1-6 Baseline

The test intentionally uses the current simplest valid combat path:

```text
player passes offense
goblin-1 passes offense automatically
goblin-2 passes offense automatically
everyone passes the offensive reaction window
no defensive action is eligible
no damage exists to resolve
advance to the next round
```

Do not add fake attacks, hard-coded dice results, test-only gameplay flows, or
status execution merely to make this test more eventful.

The current `OngoingEffectsFlow` is still automatic and does not execute
Poison, Advanced Poison, Baryl, Blind, or Injury. Existing participant statuses
must not cause this story to implement Phase 7 behavior.

The current default income configuration grants one card draw to actor
`player`. It grants no income reward to either goblin. The test should describe
and assert this current production behavior without treating it as the final
enemy-income design.

## Complete Per-Segment Run-Through

### Round 1: Ongoing Effects

Expected authority behavior:

- enter `ongoing_effects` at round 1
- mark `player`, `goblin-1`, and `goblin-2` as automatic participants
- resolve all three actors automatically
- request no human input
- run no status trigger behavior during this Phase 1-6 baseline
- complete the segment once
- advance to `income` without a client command

Player action:

```text
none
```

Enemy actions:

```text
goblin-1: automatic no-op
goblin-2: automatic no-op
```

### Round 1: Income

Expected authority behavior:

- enter `income` at round 1
- apply the default reward exactly once
- draw one card for `player`
- do not draw cards or grant resources to either goblin under the current
  default reward configuration
- request no human input
- complete the segment once
- advance to `offensive`

Player action:

```text
none; the draw is automatic
```

Enemy actions:

```text
none
```

### Round 1: Offensive Planning

Expected authority behavior before player input:

- enter `offensive` at round 1
- create one shared planning resolution
- include all three actors
- let `goblin-1` create an automatic empty/pass commitment and lock in
- let `goblin-2` create an automatic empty/pass commitment and lock in
- keep both enemy commitments hidden
- leave `player` in `needs_input`
- return exactly the player's pending planning input

The player uses two commands because passing and locking are separate actions.

#### Player Command 1: Pass Offensive Planning

Send:

```text
planning_pass
```

Build its payload from the current pending input:

```json
{
  "pending_input_id": "<current pending input ID>",
  "checkpoint": {
    "window_id": "<current window ID>",
    "segment": "offensive",
    "stage": "<current stage>",
    "iteration": "<current iteration>",
    "planning_cycle": "<current planning cycle>"
  }
}
```

Expected result:

- command is accepted
- player plan is marked as passed
- player is not locked in yet
- enemy commitments remain hidden
- offensive planning does not reveal
- authority returns a new current pending input for the player
- `planning_lock_in` is now allowed

#### Player Command 2: Lock In Offensive Planning

Send:

```text
planning_lock_in
```

Use the new pending input ID and its current planning checkpoint.

Expected result:

- command is accepted
- player becomes locked in
- all required offensive actors are now locked in
- player, `goblin-1`, and `goblin-2` commitments reveal together
- all three revealed commitments represent passes
- an offensive reaction window opens
- both AI enemies automatically pass that reaction window
- authority stops for the player's reaction input

The reveal must not contain an attack, selected target, or damage operation.

#### Player Command 3: Pass Offensive Reaction

Send:

```text
pass
```

Build its payload from the current reaction pending input:

```json
{
  "pending_input_id": "<current pending input ID>",
  "checkpoint": {
    "window_id": "<current window ID>",
    "stage": "<suspended offensive stage>",
    "iteration": "<suspended iteration>",
    "reaction_round": "<current reaction round>"
  }
}
```

Expected result:

- command is accepted
- the player passes
- `goblin-1` and `goblin-2` have already passed automatically
- the hidden simultaneous reaction round reveals
- the offensive planning proposal batch finalizes
- no second reaction round opens because every required actor passed
- offensive `OnExit` completes

This command should continue automatically through every remaining round 1
segment and stop only at round 2 offensive planning.

### Round 1: Defensive

Expected authority behavior:

- enter `defensive` at round 1
- inspect the finalized offensive proposals
- find no incoming defensible proposal because every actor passed offense
- mark all three actors `not_participating`
- create no human pending input
- complete defensive planning automatically
- advance to `damage_resolution`

Player action:

```text
none
```

Enemy actions:

```text
none
```

### Round 1: Damage Resolution

Expected authority behavior:

- enter `damage_resolution` at round 1
- find no damage operation and no deferred consequence to resolve
- reveal no damage cards
- open no damage reaction window
- remove no health cards
- complete the segment automatically
- advance to `ongoing_effects` at round 2
- set `CompletedTurn` on the wrap event
- increment the round exactly once

Player action:

```text
none
```

Enemy actions:

```text
none
```

### Round 2

Round 2 must repeat the same behavior:

```text
ongoing_effects: all three actors resolve automatically
income: player draws exactly one card
offensive: both goblins automatically pass and lock
offensive player input: planning_pass
offensive player input: planning_lock_in
offensive reaction input: pass
defensive: all three actors are not participating
damage_resolution: no damage exists
wrap: advance to round 3 ongoing_effects
```

The final round 2 reaction pass should automatically progress through:

```text
round 2 offensive exit
-> round 2 defensive
-> round 2 damage_resolution
-> round 3 ongoing_effects
-> round 3 income
-> round 3 offensive
```

The test stops when round 3 offensive planning asks the player for input.
Do not complete round 3.

## Human Command Count

After `start_battle`, the baseline should require exactly six accepted human
commands to complete two rounds:

```text
Round 1:
1. planning_pass
2. planning_lock_in
3. pass the offensive reaction

Round 2:
4. planning_pass
5. planning_lock_in
6. pass the offensive reaction
```

There should be no human command for:

- ongoing effects
- income
- defensive planning on the all-pass path
- damage resolution on the no-damage path
- segment advancement
- round advancement

Do not send `advance_segment`. Segment and round progression must remain
engine-owned.

## Command Driver Requirements

Use small test helpers to avoid duplicating JSON construction, but keep the
test sequence explicit and readable.

The driver must always build commands from the latest returned or persisted
pending input. It must not predict window IDs, pending input IDs, stages,
iterations, reaction rounds, or planning cycles.

For a planning pending input:

```text
planning_pass
-> reload
-> read new pending input
-> planning_lock_in
```

For a reaction pending input:

```text
pass
```

Every command must include the complete current checkpoint. Do not weaken
stale-command validation for this test.

## Persistence Requirements

After the start command and after every one of the six player commands:

1. Load the checkpoint from `repository.Repository`.
2. Verify the battle exists.
3. Verify the saved battle agrees with the returned snapshot for:
   - battle ID
   - current segment
   - current round
   - actor count
   - current player pending-input state
4. Use the newly loaded checkpoint as the source of authoritative assertions.

The authority itself must remain responsible for loading and saving around
commands. The test may inspect repository checkpoints, but it must not edit or
resave them.

Prove that:

- all three actor instance IDs survive every save/load
- both goblins remain independently identified actors
- the player remains human-controlled
- both goblins remain AI-controlled
- current segment and round survive every save/load
- active planning or reaction windows survive at waits
- no old pending input remains after progression creates a new checkpoint
- authoritative event history grows rather than being replaced

## Required State Assertions

### Participants

At every checkpoint:

```text
actor count = 3
actors contain player
actors contain goblin-1
actors contain goblin-2
```

Also verify:

- both goblins use definition `mock_goblin`
- each goblin retains its own actor state and uniquely scoped status instance IDs
- no actor disappears between rounds
- no duplicate actor instance is created

### Round and Segment

Required player-facing waits:

```text
after start:
  segment = offensive
  round = 1
  input = offensive planning for player

after round 1 planning_pass:
  segment = offensive
  round = 1
  input = offensive planning for player

after round 1 planning_lock_in:
  segment = offensive
  round = 1
  input = offensive reaction for player

after round 1 reaction pass:
  segment = offensive
  round = 2
  input = offensive planning for player

after round 2 planning_pass:
  segment = offensive
  round = 2
  input = offensive planning for player

after round 2 planning_lock_in:
  segment = offensive
  round = 2
  input = offensive reaction for player

after round 2 reaction pass:
  segment = offensive
  round = 3
  input = offensive planning for player
```

### Income

Because the engine stops at round 3 offensive planning, income has entered
three times:

```text
round 1 income: one player draw
round 2 income: one player draw
round 3 income: one player draw
```

Assert:

- exactly three player `cards_drawn` events exist in authoritative history
- each income entry applies its draw once
- the player's total health-card count is unchanged because cards only move
  from deck to hand
- the player's hand gains three cards compared with the pre-income loaded run
  state, subject to the current maximum-hand behavior
- the player's deck loses the same number of cards
- neither goblin receives a default income draw
- no income reward is duplicated by repository reload or repeated progression

Prefer assertions based on before/after counts and event attribution over
hard-coding the full contents of the run-player save file.

### Planning and Reactions

For each completed offensive segment:

- all three actors participate in one shared planning resolution
- both goblins lock without human input
- enemy commitments remain hidden before the player's lock-in
- reveal contains all three actors
- each actor's finalized commitment is a pass
- no selected ability exists
- no selected target exists
- no finalized damage operation exists
- the reaction window includes all required actors
- both AI enemies pass automatically
- the human pass closes the reaction chain
- no unnecessary second reaction round is created

The round 2 planning resolution and window IDs must differ from round 1 IDs.

### Defense and Damage

For each completed round:

- defensive entry occurs exactly once
- all actors are ineligible because there is no incoming defensible proposal
- no defensive player input is emitted
- damage-resolution entry occurs exactly once
- no damage proposal is created
- no damage card is revealed
- no card is moved to `removed`
- no damage reaction input is emitted
- all actors retain their initial health-card count

### Round Wrap

Authoritative segment events must prove exactly two completed turns:

```text
damage_resolution round 1 -> ongoing_effects round 2
damage_resolution round 2 -> ongoing_effects round 3
```

For each wrap:

- `CompletedTurn` is true
- the destination is `ongoing_effects`
- the round increases by one
- no segment is skipped
- no segment is entered twice for the same round

By the final wait, expected segment-entry counts are:

```text
ongoing_effects: 3
income: 3
offensive: 3
defensive: 2
damage_resolution: 2
```

The third ongoing, income, and offensive entries belong to the beginning of
round 3. Defensive and damage resolution have only completed for rounds 1 and
2.

## Event-Order Assertions

Do not assert only the final round number. Verify the authoritative event
history contains the ordered segment route:

```text
round 1:
ongoing_effects
-> income
-> offensive
-> defensive
-> damage_resolution

round 2:
ongoing_effects
-> income
-> offensive
-> defensive
-> damage_resolution

round 3:
ongoing_effects
-> income
-> offensive
```

Also verify:

- segment advancement always follows segment completion
- planning reveal occurs before offensive finalization
- offensive finalization occurs before defensive entry
- no damage event appears on this all-pass path
- response events are viewer-safe while stored repository events remain
  authoritative

Avoid asserting an entire event struct slice with `reflect.DeepEqual` if doing
so would make the test fail for unrelated additive event fields. Assert stable
event types, actor IDs, segment IDs, rounds, and ordering.

## Prohibited Shortcuts

The test must not:

- replace production flows with test-only waiting or automatic flows
- directly set the battle segment or round
- directly mark actors resolved, passed, or locked
- directly call `segment.Manager.Advance`
- manually clear a resolution
- manually create or commit planning proposals
- manually save mutated state between commands
- use `advance_segment`
- skip the public JSON command parser
- use a fake participant assembler
- remove either enemy to simplify synchronization
- disable checkpoint validation
- implement Phase 7 status behavior
- implement Phase 9 battle completion

## Handling a Discovered Defect

The expected result is that the current Phase 1-6 production code can satisfy
this test.

If the test exposes a production integration defect:

1. Confirm the failure is in existing Phase 1-6 behavior rather than a missing
   future gameplay feature.
2. Fix the smallest responsible production boundary.
3. Do not replace the failing path with test-only behavior.
4. Add a focused regression assertion for the defect.
5. Document the defect and fix in the completion report.

Do not broaden this story into status execution, enemy strategy, non-pass
abilities, battle outcomes, or disk-backed restart recovery.

## Completion Report

After implementation, create:

```text
docs/v2-planning/godot_pve/implementation/pre_phase_7_two_round_multi_enemy_test.md
```

The report must include:

- the test name and location
- whether production code required a fix
- the exact player command sequence
- what both enemies did in every segment
- the round and segment checkpoints observed after each command
- persistence assertions
- event-order assertions
- income counts
- participant-state assertions
- any behavior intentionally deferred to Phase 7 or later
- complete verification results

## Verification

Run:

```bash
cd dice-and-destiny-server
go test ./...
go vet ./...
git diff --check
```

The story is complete only when all three commands pass and the new integration
test proves the battle reaches round 3 offensive planning through the
production authority path.
