# Phase 3: Generic Nested Resolution and Interaction-Window State

## Status

Implemented June 14, 2026.

## Completed Behavior

The battle engine now persists generic nested resolution state that suspends a
segment checkpoint and resumes it after all interaction and reaction work
finishes.

Implemented behavior includes:

- stable resolution, proposal-batch, proposal, window, and commitment IDs
- proposal source, target, operation type, and typed payload data
- originating segment, lifecycle phase, flow stage, and iteration
- persisted suspended actor progress and pending input
- one interaction-window model for:
  - `required_roll`
  - `planning`
  - `reaction`
  - `choose_card`
  - `select_target`
- eligible and required actors
- per-actor interaction progress
- allowed commands and pass policy
- hidden commitments and explicit simultaneous reveal
- reaction round and chain depth
- automatic AI and system commitments
- human pending input
- reaction rounds that continue after any non-pass reaction commitment
- proposal validation followed by atomic batch application
- exact checkpoint restoration after the complete reaction chain
- `OnExit` suspension without repeated exit execution or early advancement
- explicit automatic-step, reaction-round, and chain-depth limit errors

The engine routes active interaction commands before the current segment flow's
`HandleCommand`. Active resolution progression similarly takes precedence over
the segment flow's `Progress`. No new segment lifecycle hook was added.

## State Contracts

### Resolution

`state.ResolutionState` persists:

```text
resolution ID
origin checkpoint
resolution stage
proposal batch
all interaction windows
active window ID
window sequence
reaction-window policy
suspended actor progress
suspended pending input
```

Completed resolutions remain in `Battle.Resolutions`; only
`Battle.ActiveResolutionID` is cleared. This preserves authoritative history
inside the checkpoint while allowing the originating flow to detect completion.

### Origin Checkpoint

`state.ResolutionCheckpoint` contains:

```text
segment
phase: on_enter | in_progress | on_exit
stage
iteration
```

The engine now supplies the actual lifecycle phase through `engine.Context`.
`BeginResolution` records that phase rather than accepting an unverified caller
value.

### Proposal Batch

`state.ProposalBatch` contains stable proposals with:

```text
proposal ID
source reference
target reference
operation type
typed payload
reveal status
commit status
```

Typed payload variants currently cover amounts, selections, and roll results.
The production engine contains no poison, damage, or complete card-effect rule.
Tests register a fake `adjust_value` rule to prove validation and atomic commit.

### Interaction Window

`state.InteractionWindow` persists:

```text
window ID and opened state
purpose and source
eligible and required actors
actor progress
allowed commands
hidden-commitment policy
reveal status
pass policy
commitments
reaction round
chain depth
suspended resolution checkpoint
```

`state.ReactionWindowPolicy` is stored with the resolution so the next reaction
round can be reconstructed after save/load without an in-memory continuation.

## Command Contracts

Phase 3 adds:

```text
commit_interaction
pass
```

Both commands carry:

```text
pending input ID
window ID
stage
iteration
reaction round
```

`commit_interaction` also carries typed commitment data:

```text
proposal IDs
card IDs
target IDs
choice ID
optional integer value
```

Commands are rejected when the battle, actor, pending input, window, stage,
iteration, reaction round, controller, actor progress, or allowed-command set
does not match current authoritative state.

## Events and Viewer Filtering

New authoritative events are:

```text
interaction_window_opened
interaction_committed
interaction_revealed
proposal_batch_revealed
proposal_batch_committed
resolution_completed
```

Hidden commitment events retain their authoritative payload in repository
history. Viewer filtering removes that payload for every other actor until the
explicit reveal event.

Viewer snapshots expose:

- active resolution metadata
- the revealed proposal batch
- active window policy and actor progress
- only the viewer's own hidden commitment while collecting
- all commitments after reveal

Pending input remains viewer-specific and contains no other actor's commitment.

## Limits

`engine.Config` now supports:

```text
MaxAutomaticSteps
MaxReactionChainDepth
MaxReactionRounds
```

Defaults are:

```text
automatic steps: 1000
reaction-chain depth: 16
reaction rounds: 32
```

Exceeding a limit returns an explicit wrapped engine error. The engine does not
insert a pass or otherwise resolve the window automatically.

## Files Changed

### State and Commands

- `dice-and-destiny-server/internal/battle/state/battle.go`
- `dice-and-destiny-server/internal/battle/state/resolution.go`
- `dice-and-destiny-server/internal/battle/command/command.go`

### Engine

- `dice-and-destiny-server/internal/battle/engine/flow.go`
- `dice-and-destiny-server/internal/battle/engine/engine.go`
- `dice-and-destiny-server/internal/battle/engine/command.go`
- `dice-and-destiny-server/internal/battle/engine/resolution.go`

### Persistence and Viewer Boundary

- `dice-and-destiny-server/internal/battle/repository/repository.go`
- `dice-and-destiny-server/internal/battle/event/event.go`
- `dice-and-destiny-server/internal/battle/snapshot/snapshot.go`

### Tests

- `dice-and-destiny-server/internal/battle/engine/resolution_test.go`
- `dice-and-destiny-server/internal/battle/state/battle_test.go`

## Tests and Results

Focused tests prove:

1. AI commits automatically while a human receives pending input.
2. Hidden commitments do not leak through events, snapshots, or pending input.
3. Resolution and hidden event state survive repository save/load.
4. Repository and battle clones do not alias nested maps, slices, or pointers.
5. Human and AI commitments reveal simultaneously.
6. Proposal batches reveal before reaction collection.
7. A non-pass reaction opens the next reaction round.
8. A shared pass round closes the chain.
9. The fake typed proposal validates and commits atomically.
10. The originating stage and iteration resume only after the full chain.
11. `OnExit` can suspend and resume without executing twice.
12. Stale battle, actor, window, stage, iteration, and reaction-round commands
    are rejected without mutation.
13. Reaction-round, chain-depth, and automatic-step limits return explicit
    errors without auto-pass.
14. All five initial interaction purposes use the same window contract.

Verification:

```bash
cd dice-and-destiny-server
go test ./...
```

Result: pass.

```bash
git diff --check
```

Result: pass.

## Architectural Decisions

### Resolution Progression Belongs to the Existing Engine Loop

The engine checks serialized active resolution state before calling
`SegmentFlow.Progress`. This keeps orchestration authoritative without adding
content-specific hooks, goroutines, callbacks, or in-memory continuations.

### Segment Lifecycle Remains Small

Segment flows still expose only:

```text
OnEnter
Progress
HandleCommand
OnExit
```

`SegmentFlowState.ExitStarted` provides exact-once exit behavior when `OnExit`
opens a nested resolution.

### Rules Are Registered by Operation Type

`ProposalRule` separates orchestration from consequence application. The engine
validates every proposal in the batch before applying any proposal. Resolution
progress runs on a battle clone, so an apply failure does not commit a partial
batch.

### Reaction Continuation Is Data

Reaction policy and every completed window remain in the resolution state.
Opening the next round uses only persisted data and configured limits.

### Hidden Facts Stay Authoritative

The repository stores complete events and battle state. Filtering occurs only
when producing a viewer response.

## Deviations and Remaining Gaps

- Phase 3 supports one active proposal batch per resolution. Immediate
  consequence scheduling and multi-batch content pipelines remain for later
  phases.
- Reaction commitments currently determine whether another round opens, but no
  production conflict matrix or proposal mutation rule exists.
- Commitment payloads provide typed common fields; purpose-specific validation
  will be added with shared planning and real operations.
- The repository remains in-memory and does not survive process restart.
- Existing offensive commitments and dice requests have not yet been migrated
  onto the generic interaction-window model.
- No poison, damage, health-card removal, complete card effects, offensive
  planning, or defensive planning was implemented.

## Phase 4 Prerequisites and Recommendations

1. Build shared offensive and defensive planning as configuration over
   `InteractionPurposePlanning`, not as separate reaction systems.
2. Migrate offensive roll, reroll, card, ability, target, pass, and lock-in
   commands into typed interaction commitments.
3. Add purpose-specific commitment validators while preserving the generic
   window state and command checkpoint.
4. Represent final planning output as proposal batches with stable source and
   target references.
5. Reuse the existing reveal and reaction-round machinery for offensive and
   defensive planning.
6. Add material-change detection and reopen only affected actors after reaction
   revalidation.
7. Keep AI planning behind the generic `InteractionAI` boundary and require it
   to submit the same authoritative commitment shape as humans.
8. Do not add offensive-only or defensive-only engine hooks.
9. Decide whether Phase 4 needs multiple active batches inside one resolution;
   if so, extend `ResolutionState` with a batch collection and active batch ID
   without changing the window or segment lifecycle contracts.
