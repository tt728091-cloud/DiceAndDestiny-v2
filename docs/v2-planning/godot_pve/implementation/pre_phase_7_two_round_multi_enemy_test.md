# Pre-Phase 7: Two-Round Multi-Enemy Integration Test

## Status

Implemented June 15, 2026.

## Test

Test:

```go
func TestAuthorityRunsPlayerAndMultipleEnemiesThroughTwoCompleteRounds(t *testing.T)
```

Location:

```text
dice-and-destiny-server/internal/battle/multi_round_integration_test.go
```

The test uses:

```text
engine.NewEngine()
repository.NewInMemory()
NewFileParticipantAssembler(real content root, real run-player root)
authority.HandleCommandJSON(...)
```

Every command follows the production route from JSON parsing through authority,
repository load/save, engine command application and automatic progression,
then viewer-safe result construction.

## Production Fix

No production-code fix was required. The existing Phase 1 through Phase 6
production flows completed both rounds and reached round 3 offensive planning.

The accepted `planning_pass` action rotates and persists the pending input but
does not emit a standalone event under the current Phase 4 event contract. The
test therefore proves that repository history retains its exact prior prefix,
never shrinks or is replaced, and grows by the exact authoritative event delta
when a command emits events.

## Player Command Sequence

After `start_battle`, exactly six human commands are accepted:

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

Every payload is built from the latest player pending input loaded from the
repository, including its current window ID, stage, iteration, planning cycle,
or reaction round. No segment-advance command is sent.

## Enemy Behavior

Both `goblin-1` and `goblin-2`:

- resolve ongoing effects automatically as no-ops
- receive no default income reward
- automatically complete offensive planning with hidden locked pass
  commitments
- automatically pass the shared offensive reaction window
- become ineligible for defensive planning because all offensive proposals are
  passes
- require no damage-resolution input because no damage operation exists

The two goblins remain separate AI-controlled `mock_goblin` actor instances
with independently scoped status instance IDs at every checkpoint.

## Observed Checkpoints

```text
after start:
  offensive, round 1, player offensive planning input

after round 1 planning_pass:
  offensive, round 1, new player offensive planning input

after round 1 planning_lock_in:
  offensive, round 1, player offensive reaction input

after round 1 reaction pass:
  offensive, round 2, player offensive planning input

after round 2 planning_pass:
  offensive, round 2, new player offensive planning input

after round 2 planning_lock_in:
  offensive, round 2, player offensive reaction input

after round 2 reaction pass:
  offensive, round 3, player offensive planning input
```

## Persistence Assertions

After the start command and every player command, the test reloads the
repository checkpoint and verifies:

- battle ID, segment, round, actor count, and player pending input match the
  returned snapshot/result
- all three actor IDs survive save/load without duplication
- controllers and definition IDs remain correct
- active planning and reaction windows survive waits
- accepted commands rotate the pending input ID
- old pending input does not remain current
- event history retains its prior prefix and appends command event deltas
- stored hidden enemy commitments remain authoritative while viewer results
  hide them before reveal

The test only inspects repository checkpoints. It never edits or resaves them.

## Planning and Reaction Assertions

For rounds 1 and 2, the test verifies:

- one shared offensive planning resolution contains all three actors
- both enemies lock in automatically before player input
- enemy commitments remain hidden until player lock-in
- the planning reveal contains player, `goblin-1`, and `goblin-2`
- all three revealed commitments are locked passes
- no ability, target, committed card, or finalized operation is present
- the reaction window requires all three actors
- both enemies have already passed at the player reaction wait
- the player's pass reveals one shared pass round and closes the chain
- no second reaction round opens
- round 1 and round 2 resolution/window IDs differ

## Income and Participant State

Income enters in rounds 1, 2, and 3 before the final wait.

The test verifies:

- exactly three authoritative `cards_drawn` events exist
- each event attributes exactly one drawn card to `player`
- the player hand gains three cards from the loaded run-state baseline
- the player deck loses the same three cards
- total player health-card count remains unchanged
- neither goblin changes any card zone or receives an income draw
- every actor retains its initial health-card count
- no card enters any actor's permanently removed zone

## Defense, Damage, and Event Order

The authoritative segment-entry route is asserted as:

```text
round 1:
ongoing_effects -> income -> offensive -> defensive -> damage_resolution

round 2:
ongoing_effects -> income -> offensive -> defensive -> damage_resolution

round 3:
ongoing_effects -> income -> offensive
```

Expected entry counts are verified:

```text
ongoing_effects: 3
income: 3
offensive: 3
defensive: 2
damage_resolution: 2
```

The test also verifies:

- no segment is entered twice for the same round
- every advancement matches the next segment entry
- exactly two `damage_resolution -> ongoing_effects` advances set
  `CompletedTurn`
- planning reveal precedes offensive proposal finalization
- offensive finalization precedes defensive entry
- defensive planning opens no window and produces no proposal
- damage resolution emits no damage or permanent-removal event
- no pending operation or damage state remains at the final wait

## Deferred Behavior

This story intentionally does not implement:

- Phase 7 status-trigger execution
- Poison or Advanced Poison rolls
- Baryl immediate consequences
- enemy strategy beyond the current automatic planning policy
- Blind offensive-exit behavior
- battle-result evaluation or battle completion
- disk-backed process-restart recovery

## Verification

From `dice-and-destiny-server`:

```text
go test ./...
PASS

go vet ./...
PASS

git diff --check
PASS
```

The focused test also passes independently and reaches round 3 offensive
planning through the repository-backed authority JSON path.
