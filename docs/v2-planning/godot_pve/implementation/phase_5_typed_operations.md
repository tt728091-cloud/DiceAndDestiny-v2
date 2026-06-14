# Phase 5: Typed YAML Operations and Trigger Registry Foundation

## Status

Implemented June 14, 2026.

## Completed Behavior

Content loading now compiles strict YAML card, ability, and status operations
into immutable runtime plans before battle state is created.

Implemented behavior includes:

- the closed engine timing model:
  - `on_enter`
  - `in_progress`
  - `on_exit`
- triggers represented by segment, phase, and priority
- deterministic trigger matching and ordering by:
  1. higher priority
  2. lower effect-instance creation order
  3. stable effect-instance ID
- strict YAML decoding for cards, abilities, dice, characters, enemies, and
  statuses
- deterministic operation path IDs when YAML omits an explicit operation ID
- typed targets, card zones, resources, reactions, die changes, roll outcomes,
  and nested operation lists
- an injectable operation registry with per-type validation and compilation
- content-reference validation for cards, abilities, dice, statuses, stack
  limits, and named stack-overflow policies
- one compiled content catalog per battle rather than full content copies on
  every actor
- repository cloning and save/load preservation for compiled content and
  finalized operation proposals

No operation mutates battle consequences yet. Phase 5 creates validated plans
and planning proposals only.

## YAML Schemas Introduced

Reusable operation types:

```text
roll_dice
evaluate_roll_outcome
modify_die
reroll_die
deal_damage
prevent_damage
apply_status
remove_status_stack
move_cards
draw_cards
gain_resource
change_target
noop
```

Typed fields cover:

- source and target selectors
- amounts
- status IDs and stack counts
- dice counts and side counts
- one roll per status stack
- explicit faces or inclusive face ranges
- nested operations per outcome
- reaction eligibility and policy
- die index, face, and modification mode
- card source and destination zones
- resource type

Unknown fields, operation types, selectors, zones, resources, reaction
policies, and invalid parameter combinations fail content loading.

Status YAML contains:

```text
schema version
stable ID
display name
stack limit
optional named stack-overflow policy
trigger definitions
typed resolution operations
```

Authored examples were added for:

- `poison`: one D6 per stack; faces 1-4 propose one damage; faces 5-6 remove
  one stack
- `advanced_poison`: one D6 per stack; faces 1-5 propose two damage; face 6
  removes one stack
- `injury`: a non-executing placeholder definition for the existing run-state
  reference

Poison and Advanced Poison compile through the same operation handlers and
differ only by YAML parameters.

### Data-Driven Poison Proof

Poison and Advanced Poison are not represented by separate Go handlers,
status-ID switches, or poison-specific orchestration.

Both YAML files compile through the same registered generic operation types:

```text
roll_dice
-> evaluate_roll_outcome
-> deal_damage | remove_status_stack
```

Their behavior differs only in authored data:

```text
Poison:
- damage faces: 1-4
- damage amount: 1
- remove-stack faces: 5-6

Advanced Poison:
- damage faces: 1-5
- damage amount: 2
- remove-stack face: 6
```

`typed_operations_test.go` loads both definitions through
`LoadContentLibrary`, verifies that both produce nested compiled operation
plans, verifies that their plans differ, and checks the compiled damage amounts
of one and two respectively.

`operation_test.go` separately proves that the reusable registry compiles the
generic `roll_dice`, `evaluate_roll_outcome`, `deal_damage`, and
`remove_status_stack` operation types. There is no registered `poison` or
`advanced_poison` operation type.

Runtime poison execution remains deferred to Phase 7. Phase 7 must consume
these compiled generic plans rather than introduce status-ID-specific handlers.

## Operation and Trigger Contracts

`operation.Definition` is the strict authored union. `operation.Plan` is the
compiled runtime form.

Every compiled operation has either:

- an explicit YAML `id`, or
- a deterministic identity based on its content path and nested index

`operation.Registry` maps each operation type to a `Handler` contract:

```text
Type
Validate
Compile
```

The registry is immutable after construction and injectable through
`LoadContentLibraryWithRegistry`.

Trigger validation accepts only known battle segments and the three lifecycle
phases. No timing-specific engine functions such as `before_draw`,
`after_roll`, or `before_damage` were added.

## Registry and Compiler Architecture

Content parsing remains in `internal/content`.

Typed operation definitions and compilation live in
`internal/battle/operation`. The package has no Godot, JSON transport, or
battle-state mutation concerns.

The file participant assembler converts loaded content into a runtime catalog:

```text
cards      -> segment restrictions, cost, target requirement, operations
abilities  -> segment restrictions, cost, dice requirement, target requirement, operations
statuses   -> stack policy, trigger checkpoints, operations
```

Actors continue to store stable card, ability, and status IDs. The compiled
catalog is stored once on the battle checkpoint and cloned defensively by the
repository boundary.

## Phase 4 Adapters

Replaced:

- ability classification by ID substrings such as `guard` and `defen`
- card acceptance based only on presence in hand
- planning finalization without content operation proposals

Planning now uses loaded definitions for segment restrictions, content
existence, target requirements, dice requirements, and represented energy
costs.

Retained:

- `PlanningAdjustment` reaction payloads for the existing Phase 4 targeted
  re-entry tests and compatibility route

Those adjustments still support Phase 4 revalidation behavior such as granting
rolls or explicitly reopening an actor. They are not authored YAML content and
do not execute the new operation plans. Replacing them requires reaction card
selection and operation execution, which is outside this phase.

## Planning Integration

Planning selection now validates:

- the actor owns the selected ability
- the ability or card exists in the battle content catalog
- the current segment is allowed
- required targets are selected
- supported dice requirements are met
- combined represented energy cost is affordable
- committed cards are present in the actor hand

Finalization creates stable `FinalizedOperationProposal` values containing:

- proposal ID
- content type and content ID
- compiled operation plan
- source actor ID
- selected target IDs

The plans are persisted on offensive or defensive planning proposals. They do
not apply damage, statuses, resources, card movement, or permanent card
removal.

Planning visibility remains unchanged. Compiled content is not added to viewer
snapshots or hidden commitment events.

## Files Changed

Content and schemas:

- `dice-and-destiny-server/internal/content/character_combat_sheet.go`
- `dice-and-destiny-server/internal/content/status.go`
- `dice-and-destiny-server/content/statuses/poison.yaml`
- `dice-and-destiny-server/content/statuses/advanced_poison.yaml`
- `dice-and-destiny-server/content/statuses/injury.yaml`

Registry and operation plans:

- `dice-and-destiny-server/internal/battle/operation/operation.go`

Battle setup, state, persistence, and planning:

- `dice-and-destiny-server/internal/battle/participant_assembler.go`
- `dice-and-destiny-server/internal/battle/state/battle.go`
- `dice-and-destiny-server/internal/battle/state/resolution.go`
- `dice-and-destiny-server/internal/battle/engine/planning.go`

Tests:

- `dice-and-destiny-server/internal/battle/operation/operation_test.go`
- `dice-and-destiny-server/internal/content/typed_operations_test.go`
- `dice-and-destiny-server/internal/battle/phase5_integration_test.go`
- `dice-and-destiny-server/internal/battle/engine/planning_validation_test.go`
- `dice-and-destiny-server/internal/battle/engine/planning_test.go`

## Tests and Results

Coverage includes:

- every reusable operation type
- valid typed card, ability, and status loading
- strict unknown-field rejection
- unknown segment, phase, operation, target, zone, resource, status reference,
  and overflow-policy rejection
- missing and incompatible operation parameters
- deterministic nested outcome compilation and operation IDs
- deterministic trigger matching and ordering
- distinct Poison and Advanced Poison plans using shared handlers
- injected registry failure behavior
- stack limits and named overflow policies
- loaded ability segment validation
- loaded card planning and typed finalized proposals
- target, dice, and resource planning validation
- no effect execution or permanent card removal during planning
- repository preservation of content and operation references
- all existing Phase 1-4 tests

Verification:

```bash
cd dice-and-destiny-server
go test ./...
go vet ./...
```

Both commands pass.

## Deferred Execution Behavior

This phase does not:

- execute status triggers
- roll poison dice
- apply or prevent damage
- apply or remove status stacks
- move, draw, or permanently remove cards through operation plans
- mutate resources through operation plans
- execute reaction cards through the new registry
- implement Baryl, Blind, battle completion, or replay

The operation registry is a content validation and compilation boundary. Phase
6 must add execution contracts without turning YAML into unrestricted
scripting.

## Phase 6 Prerequisites and Recommendations

1. Consume `FinalizedOperationProposal` values as inputs to damage-resolution
   proposal construction; do not re-read card or ability YAML during battle.
2. Add execution handlers beside the existing validator/compiler contracts,
   keyed by operation type and injected for tests.
3. Keep execution separated into proposal creation, validation, reveal,
   reaction, and atomic commit.
4. Implement `deal_damage` and `prevent_damage` as proposal mechanics before
   permanent card removal.
5. Preserve per-source attribution while also supporting accumulated damage
   totals.
6. Select and reveal damage cards before moving any card to `removed`.
7. Do not execute status triggers in Phase 6. Poison remains the Phase 7 proof
   that the same compiled plans can drive status resolution.
8. Replace legacy planning reaction adjustments only when reaction-selected
   cards can resolve registered typed operations and retain targeted re-entry.
9. Add durable content-version pinning before disk recovery or replay relies on
   compiled plans across application versions.
