# Phase 9: Battle Completion, Replay, and Recovery

## Status

Implemented June 15, 2026.

Phase 9 was implemented before Phases 7 and 8. The completion gate therefore
uses the existing generic segment and nested-resolution contracts and does not
add Poison, Advanced Poison, Baryl, Blind, enemy strategy, Run, or other
Phase 7/8 gameplay.

## Battle Completion

Completion is evaluated only from the engine's existing
`completeAndAdvance` boundary:

```text
segment reports complete
-> run OnExit exactly once
-> finish every nested resolution opened by OnExit
-> convert pending_defeat actors to defeated
-> evaluate battle result
-> emit battle_completed or advance to the next segment
```

An actor becoming `pending_defeat` never stops damage resolution, immediate
consequences, segment exit, or a nested interaction/reaction chain.

The implemented outcomes are:

```text
player alive + all AI enemies defeated = victory
player defeated + enemies remain       = defeat
player defeated + all enemies defeated = draw
otherwise                               = active
```

`Battle.EscapeRequested` is the minimal generic authoritative hook for the
`escaped` result. No Run command or escape mechanic was added.

Terminal evaluation emits exactly one authoritative `battle_completed` event.
Subsequent progression returns `battle_complete` without another event.
Subsequent gameplay commands are rejected as `battle is complete`; authority
does not save the rejected working state, so the terminal checkpoint is
unchanged.

## Event Sequencing

Persisted events use event schema version 1 and contain:

- battle ID
- deterministic contiguous sequence number starting at 1
- stable event ID derived from battle ID and sequence
- event schema version
- the complete authoritative event payload

Event IDs use:

```text
<battle-id>:event:<20-digit-sequence>
```

No timestamp or random identifier participates in ordering. The checkpoint
stores `next_event_sequence`, so sequencing continues contiguously after a
process restart.

Random results are event facts. Dice faces, values, symbols, selected damage
cards, and related result data are stored in the event/checkpoint and are not
reevaluated by replay.

Hidden commitment payloads and their authoritative visibility metadata are
persisted. `event.ForViewer` filters copies returned to a viewer and never
changes stored events.

## Persistence Format

Checkpoint schema version 1 is a JSON envelope containing:

```text
schema_version
event_schema_version
battle_id
content_pin
next_event_sequence
battle
events
```

`battle` is the complete authoritative state, including:

- segment, round, phase, stage, and iteration
- actors, card zones, resources, statuses, tokens, and defeat state
- pending input
- hidden planning commitments
- roll requests and recorded rolls
- nested resolutions and interaction windows
- offensive and defensive proposals
- damage sources, totals, selected cards, and reaction state
- pending operations
- compiled runtime cards, abilities, statuses, operations, and dice

Missing checkpoints return `ErrBattleNotFound`. Invalid battle IDs return
`ErrInvalidBattleID`. Malformed, inconsistent, non-contiguous, or
fingerprint-mismatched checkpoints return `ErrCorruptCheckpoint`. Unsupported
checkpoint, event, content-pin, or compiled-content versions return
`ErrUnsupportedCheckpoint`.

Battle IDs are restricted to a bounded filename-safe character set before a
path is constructed.

## Atomic Writes

The disk repository:

1. serializes the complete replacement checkpoint
2. creates a temporary file in the destination directory
3. applies owner-only permissions
4. writes and syncs the temporary file
5. closes it
6. atomically renames it over the destination
7. removes the temporary file on every pre-rename failure

A rename failure leaves the previous valid checkpoint in place. Tests inject a
rename failure and verify that the old checkpoint still loads and no temporary
file remains.

The default authority now uses disk storage. The root is configured by:

```text
DICE_AND_DESTINY_BATTLE_STATE_ROOT
```

The local default is:

```text
dice-and-destiny-server/save/battles
```

That runtime directory is gitignored. The in-memory repository remains
available for focused tests.

## Recovery

Recovery tests reconstruct both the repository and `Authority` instance and
continue:

- offensive planning input
- an offensive reaction window
- a damage reaction checkpoint containing selected cards and damage proposals
- a terminal checkpoint

The resumed authority is given an assembler that always fails. Continuing an
existing battle still succeeds, proving that resume uses the compiled content
inside the checkpoint and does not reload live YAML.

Tests compare checkpoints before and after repository reconstruction to verify
that hidden commitments, resolutions, damage proposals, events, segment,
round, pending input, actor state, and content pin survive exactly.

## Replay Reader

`internal/battle/replay.Reader` loads a checkpoint and validates:

- requested battle ID
- checkpoint and event schema versions
- content-pin versions and fingerprint
- contiguous event sequence
- stable event IDs
- per-event battle ID
- next event sequence

It returns events in persisted sequence order. An optional expected content
fingerprint detects changed content. An optional viewer actor ID applies the
normal event visibility filter to copies.

The reader never runs the engine, content operations, or random sources.

### Replay Limitation

This is an authoritative event-history reader, not full event-sourced state
reconstruction. Existing events do not describe every state mutation required
to rebuild a battle from an empty state. Immediate recovery uses the complete
checkpoint; replay provides validated recorded facts and viewer-safe history.

## Content Pinning

Battle creation computes a canonical SHA-256 fingerprint over the compiled
runtime:

- card definitions and operation plans
- ability definitions and operation plans
- status definitions, triggers, and operation plans
- dice definitions, faces, values, and symbols

Map-backed collections are converted to key-sorted arrays before hashing, so
the fingerprint is independent of Go map insertion or iteration order.

The pin records:

```text
content pin schema version
compiled content version
hash algorithm
fingerprint
```

The pin and compiled authoritative content are stored together. A changed live
YAML tree does not prevent an existing checkpoint from resuming. A changed
compiled checkpoint payload fails fingerprint validation.

## Phase 7/8 Compatibility

No new segment lifecycle phase was added. Future status triggers and Blind
work can continue to open generic nested resolutions from `OnExit`. Completion
will wait until `ActiveResolutionID` clears and the originating flow reports
segment completion again.

This preserves:

- segment package isolation
- generic nested resolutions
- shared offensive and defensive planning
- repository-backed authority commands
- viewer-safe snapshots and events
- automatic progression until human input

## Files Changed

Core implementation:

- `internal/battle/authority.go`
- `internal/battle/contentpin/content_pin.go`
- `internal/battle/engine/engine.go`
- `internal/battle/event/event.go`
- `internal/battle/replay/replay.go`
- `internal/battle/repository/repository.go`
- `internal/battle/snapshot/snapshot.go`
- `internal/battle/state/battle.go`
- `.gitignore`

Tests:

- `internal/battle/contentpin/content_pin_test.go`
- `internal/battle/engine/completion_test.go`
- `internal/battle/engine/damage_resolution_flow_test.go`
- `internal/battle/recovery_test.go`
- `internal/battle/replay/replay_test.go`
- `internal/battle/repository/disk_test.go`
- existing authority, repository, and default-authority tests updated for the
  versioned durable lifecycle

## Test Coverage

Phase 9 tests cover:

- active battle remains active
- victory, defeat, simultaneous draw, and escaped hook
- completion after segment exit and nested resolution completion
- exactly one `battle_completed` event
- terminal command rejection without mutation
- disk create, load, save, duplicate protection, and safe IDs
- corrupt and unsupported checkpoint rejection
- atomic replacement failure behavior
- restart during planning
- restart during offensive reaction
- restart during damage reaction
- terminal checkpoint restart
- contiguous sequence numbers across restart
- recorded dice faces and symbols without replay rerolls
- viewer-filtered replay hiding commitments
- stable canonical content fingerprints
- changed-content detection
- the existing two-round multi-enemy integration route

Verification:

```bash
cd dice-and-destiny-server
go test ./...
go vet ./...
git diff --check
```

All commands pass.

## Deferred Migration Work

No migration is attempted for genuinely older checkpoint, event, content-pin,
or compiled-content versions. They are rejected explicitly. A future migration
tool must transform the complete envelope, recompute the content pin, and
preserve or intentionally remap event identity and sequence.
