# Phase 1: Durable Battle Lifecycle

## Status

Implemented June 14, 2026.

This phase establishes an in-process, repository-backed battle lifecycle through the portable JSON authority boundary. It removes the previous behavior where each public command operated on a newly created empty battle.

## Completed Behavior

### Battle Start

The authority accepts a `start_battle` command containing:

- one player descriptor
- one or more enemy descriptors
- a stable instance ID for every participant
- a reusable definition ID for every participant

The command actor must match the player instance ID. Participant instance IDs must be unique.

The authority:

1. validates the request
2. assigns the player a `human` controller and enemies `ai` controllers
3. delegates state construction to an injected `ParticipantAssembler`
4. validates that the assembler returned exactly the requested actors
5. creates authoritative battle state
6. progresses through automatic work to the first human input
7. stores the resulting battle checkpoint and authoritative events
8. returns viewer-filtered events, pending input, and a snapshot

### Continuing Commands

Commands after battle start:

1. load the checkpoint by battle ID
2. apply the command to a cloned authoritative battle
3. progress to the next human wait or terminal result
4. append unfiltered authoritative events to checkpoint history
5. save the updated checkpoint
6. return viewer-safe events and state

Rejected commands do not save partial mutations.

### Repository

`repository.Repository` exposes:

```go
Create(checkpoint Checkpoint) error
Load(battleID string) (Checkpoint, error)
Save(checkpoint Checkpoint) error
```

The in-memory implementation:

- rejects duplicate battle creation
- rejects loading or saving unknown battles
- returns defensive copies
- stores defensive copies
- preserves authoritative event history across commands

This provides durability across authority calls in one process. It does not provide process-restart recovery.

### State and Results

Actor state now preserves:

- stable actor instance ID as the actor map key
- reusable definition ID
- controller type

Battle state now includes:

```text
active
victory
defeat
draw
escaped
```

Engine responses distinguish:

```text
waiting_for_input
battle_complete
```

A terminal response also includes its authoritative battle result. Phase 1 represents terminal state but does not calculate victory or defeat.

### Event Boundary

Engine execution and viewer delivery are separate:

- command execution returns authoritative events
- the repository stores those authoritative events
- viewer filtering occurs only while building the response

This prevents private authoritative information from being discarded before future replay or alternate-viewer delivery.

## Files Changed

### Lifecycle Boundary

- `dice-and-destiny-server/internal/battle/authority.go`
  - added the `Authority` service
  - added `start_battle`
  - added participant validation and assembler integration
  - added repository load, append, and save behavior
  - added instance-level JSON handling
- `dice-and-destiny-server/internal/battle/authority_test.go`
  - added start, persist, reload, command continuation, and duplicate-ID tests

### Commands

- `dice-and-destiny-server/internal/battle/command/command.go`
  - added `start_battle`
  - added player and enemy participant descriptor payloads

### Repository

- `dice-and-destiny-server/internal/battle/repository/repository.go`
  - added the repository contract and in-memory implementation
- `dice-and-destiny-server/internal/battle/repository/repository_test.go`
  - added create/load/save, error, and defensive-copy tests

### Engine and Results

- `dice-and-destiny-server/internal/battle/engine/command.go`
  - added authoritative command application separate from response filtering
  - removed stateless empty-battle command execution
  - added terminal result output
- `dice-and-destiny-server/internal/battle/engine/engine.go`
  - added terminal-state stopping behavior
- `dice-and-destiny-server/internal/battle/engine/flow.go`
  - added the `battle_complete` result status
- `dice-and-destiny-server/internal/battle/engine/engine_test.go`
  - added terminal-result verification

The pre-existing uncommitted synchronized-flow changes in the remaining engine and event files were preserved and used by the lifecycle implementation.

### State and Snapshots

- `dice-and-destiny-server/internal/battle/state/battle.go`
  - added battle status
  - added actor definition IDs
  - added duplicate actor validation
- `dice-and-destiny-server/internal/battle/state/battle_test.go`
  - added definition/controller and duplicate actor tests
- `dice-and-destiny-server/internal/battle/snapshot/snapshot.go`
  - exposes definition IDs and controllers in viewer snapshots

### Planning Documentation

- `docs/v2-planning/godot_pve/07_authoritative_battle_rules_model.md`
  - records the implemented Phase 1 boundary and deferred durability work
- `docs/v2-planning/godot_pve/08_current_implementation_gap_analysis.md`
  - marks Phase 1 implemented and updates the current runtime route and gaps

## Tests

Focused tests prove:

1. One player and two enemies can be requested through JSON.
2. The assembler receives each instance and definition ID.
3. Participant controllers are assigned correctly.
4. The battle progresses to the first human wait.
5. Initial authoritative events and state are stored.
6. A later roll command loads the same battle.
7. The roll mutates the existing actor state.
8. The new authoritative event is appended and saved.
9. Duplicate battle IDs do not replace checkpoints.
10. Repository inputs and outputs do not alias stored mutable state.
11. Terminal battle state returns `battle_complete` with the battle result.

Verification command:

```bash
cd dice-and-destiny-server
go test ./...
```

Result: pass.

## Decisions

### Participant Loading Is Injected

Phase 1 defines and persists the lifecycle without inventing incomplete player or enemy loading. `ParticipantAssembler` is the boundary Phase 2 will implement using run state and enemy content.

### In-Memory Storage Is the Phase 1 Repository

The repository contract is storage-independent, but Phase 1 implements only in-memory storage. Disk serialization, atomic file writes, checkpoint versions, and restart recovery remain later work.

### The Repository Stores Checkpoints and Authoritative Events

Checkpoint state supports immediate command continuation. Authoritative events are retained alongside it so the lifecycle does not need another breaking repository shape before event sequencing and replay are implemented.

### Duplicate Starts Are Rejected

`start_battle` never silently replaces an existing battle ID. Explicit replacement or abandonment semantics can be added later if required.

### Terminal State Is Modeled Before It Is Calculated

The result contract can now report terminal battles without hard-coding all accepted results as waits. Battle-result evaluation remains with the future damage and completion rules.

## Remaining Work for Phase 2

Phase 2 must provide the production participant assembler and complete actor setup.

Required work:

- load the player from mutable run state
- load fresh enemy instances from enemy YAML definitions
- merge one player and multiple enemies into one `BattleSetup`
- preserve character/enemy definition metadata
- preserve full card zones and acquired or removed cards
- preserve dice loadouts and definitions
- preserve abilities
- preserve starting and current resources
- preserve statuses, tokens, and persistent injuries
- preserve roll automation preferences
- ensure repeated enemy definitions create independent mutable actor instances
- configure the default exported authority with this production assembler

Phase 2 should not replace the repository-backed lifecycle. It should implement the participant construction behind the existing `ParticipantAssembler` boundary.

## Deferred Beyond Phase 2

- disk-backed atomic checkpoints
- process-restart resume tests
- event IDs and deterministic sequence numbers
- command and causal event metadata
- content/schema version pinning
- replay reconstruction and validation
- battle-result evaluation at segment exit
