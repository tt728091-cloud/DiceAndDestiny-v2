# Phase 6: Damage Resolution and Card-as-Health

## Status

Implemented June 14, 2026.

## Completed Behavior

The placeholder damage-resolution segment was replaced with a persisted
production flow:

```text
collect finalized operations
-> calculate source and accumulated damage
-> select proposed health cards
-> reveal proposals
-> run the Phase 3 reaction chain
-> recalculate after accepted reactions
-> atomically commit damage cards
-> queue deferred operations
-> complete
```

Damage remains proposed until the reaction chain closes. Card zones do not
change during collection, calculation, selection, reveal, or reaction rounds.

The flow supports:

- simultaneous damage to multiple actors
- multiple attributed damage sources per target
- defensive prevention targeted at an incoming planning proposal or damage
  source
- independent counter-damage sources
- accumulated-damage prevention
- source-specific prevention and signed modification
- proposed-card replacement
- card release when damage decreases
- additional selection and reveal when damage increases
- multiple typed damage reactions in one actor commitment
- configurable reaction eligibility, defaulting to all battle actors
- shared-pass-round closure through the existing Phase 3 window system

Poison, Advanced Poison, Baryl, Blind, status triggers, and battle-result
evaluation remain unexecuted.

## Files Changed

### Damage Domain and State

- `dice-and-destiny-server/internal/battle/damage/damage.go`
- `dice-and-destiny-server/internal/battle/state/damage.go`
- `dice-and-destiny-server/internal/battle/state/battle.go`
- `dice-and-destiny-server/internal/battle/state/resolution.go`

### Operation Runtime

- `dice-and-destiny-server/internal/battle/operation/runtime.go`

### Flow and Commands

- `dice-and-destiny-server/internal/battle/engine/damage_resolution_flow.go`
- `dice-and-destiny-server/internal/battle/engine/placeholder_flows.go`
- `dice-and-destiny-server/internal/battle/engine/resolution.go`
- `dice-and-destiny-server/internal/battle/command/command.go`

### Events, Persistence, and Snapshots

- `dice-and-destiny-server/internal/battle/event/event.go`
- `dice-and-destiny-server/internal/battle/repository/repository.go`
- `dice-and-destiny-server/internal/battle/snapshot/snapshot.go`

### Tests

- `dice-and-destiny-server/internal/battle/damage/damage_test.go`
- `dice-and-destiny-server/internal/battle/engine/damage_resolution_flow_test.go`
- `dice-and-destiny-server/internal/battle/operation/runtime_test.go`

## Damage Proposal and Source Contracts

Phase 5 `FinalizedOperationProposal` values are the only content inputs.
Damage resolution does not reload YAML or switch on content IDs.

`operation.RuntimeRegistry` executes registered generic operation types.
Phase 6 registers runtime handlers for:

```text
deal_damage
prevent_damage
```

Compilation and execution remain separate. Runtime handlers convert immutable
compiled plans into typed runtime proposals without mutating battle state.

Each source damage proposal preserves:

- stable proposal ID
- originating planning proposal ID
- source actor ID
- source content type and content ID
- target actor ID
- base, prevented, modified, and final amounts
- the complete originating finalized operation

Accumulated proposals preserve the target, source proposal IDs, base total,
accumulated prevention, and final total. Broad prevention is allocated across
source proposals deterministically so source-level committed amounts sum to
the accumulated committed amount.

Defensive `prevent_damage` operations may target an offensive planning
proposal ID or a source damage proposal ID. Defensive `deal_damage` operations
become independent counter-damage sources.

Unsupported finalized operations are carried through the revealed batch and
queued on `Battle.PendingOperations` after commit. `noop` is treated as an
already-resolved operation. Status application remains queued for Phase 7.

## Card-Selection Algorithm

`ActorState.CurrentHealth()` is the reusable authoritative calculation:

```text
len(deck) + len(discard) + len(hand)
```

For each target's complete pending damage:

1. Build one candidate population from deck and discard card occurrences.
2. Exclude card occurrences already represented by accepted proposals.
3. Select without replacement using an injected `Intn` random source.
4. Use hand occurrences only after no unselected deck or discard occurrence
   remains.
5. Preserve each selected card's original zone.
6. Create stable typed proposed-card-removal entries before any mutation.

Duplicate card definition IDs are supported because selection and validation
count card occurrences by actor, zone, and card ID. Damage above remaining
health selects every remaining occurrence once and stops safely.

No discard reshuffle occurs. Production uses the existing crypto random source;
tests inject deterministic and recording sources.

## Reaction and Recalculation Behavior

The reveal opens one persisted Phase 3 reaction resolution containing:

- accumulated damage proposals
- source-specific damage proposals
- proposed damage-card removals
- finalized non-damage operations

Typed reactions are stored in normal interaction commitments:

```text
prevent_accumulated_damage
prevent_damage_source
modify_damage_source
replace_damage_card
```

Every revealed reaction round is applied once, tracked by window ID. The
domain recalculation path is the same path used for initial preview and final
commit.

After recalculation:

- excess accepted card proposals are marked released without moving cards
- increased damage selects additional cards
- replacement proposals release the original and preserve original-zone data
  for the replacement
- newly selected or replaced cards reveal before the next reaction round
- any non-pass round opens the next Phase 3 reaction window
- a round where every required actor passes commits the batch

## Atomic Commit

Before mutation, commit validates every accepted card proposal:

- proposal IDs are unique
- target actors exist
- every required card occurrence still exists in its recorded original zone

Commit operates on a cloned battle. A failed validation or removal rejects the
entire commit without partial mutation.

On success:

- accepted cards move from their original zones to `removed`
- source and accumulated damage results are retained for events
- deferred finalized operations move to `Battle.PendingOperations`
- actors at zero health cards become `pending_defeat`
- battle status remains active
- temporary source, modifier, accumulated, card, and pending proposal slices
  are cleared

Completed segment work continues after an actor reaches zero. Phase 6 does not
evaluate victory, defeat, draw, escape, or interrupt the segment.

## Events and Visibility Policy

New authoritative events are:

```text
damage_proposed
damage_cards_revealed
damage_prevented_or_modified
damage_committed
cards_permanently_removed
```

Events preserve proposal IDs, source actor/content IDs, target actor IDs,
amounts, original card zones, and damage/source attribution on card removals.
Commit emits both source-level and accumulated damage events.

Before reveal, snapshots expose public pending totals but no proposed card
IDs. After reveal, accepted proposed damage cards are public to every viewer
under the current default rule. The active generic interaction window remains
viewer-filtered by the existing Phase 3 commitment policy.

Actor snapshots expose:

- current card-zone counts
- current health
- explicit health-card count for card-as-health actors, including zero
- pending defeat state

Damage snapshots expose:

- public pending totals
- revealed accepted card proposals
- damage stage and revision
- active damage interaction window ID

All arithmetic remains in the damage domain package.

## Persistence Behavior

`Battle.Clone` and repository event cloning now deep-copy:

- damage source, modifier, accumulated, and card proposals
- originating operation plans
- applied reaction-window IDs
- typed damage reactions
- deferred finalized operations
- detailed damage-card event payloads

Tests save and load at the initial damage reaction wait and again after
recalculation at reaction round two. The resumed checkpoint preserves selected
cards, reveal state, active window, reaction round, and pending input without
rerolling or reselecting existing cards.

## Tests and Results

Coverage includes:

- proposal-before-mutation behavior
- multiple sources with retained attribution
- accumulated totals and independent counter-damage
- defensive prevention in preview and commit
- one equal-probability deck/discard population
- exhaustive combined-index selection with one deck card and three discard
  cards, proving selection is per card occurrence rather than 50/50 by zone
- hand-only fallback after deck and discard exhaustion
- combined card reveal per target
- damage above remaining health
- repository save/load and clone isolation
- accumulated and source-specific prevention
- source modification
- proposed-card replacement
- released excess cards after reduction
- additional selection and reveal after increase
- exact-once permanent removal
- atomic failed validation
- simultaneous multi-actor damage
- simultaneous zero health and pending defeat
- continued queued consequence work at zero health
- pre-reveal and post-reveal viewer policy
- operation runtime registry behavior
- production `DamageResolutionFlow` end to end
- all existing Phase 1-5 tests

Verification:

```bash
cd dice-and-destiny-server
go test ./...
go vet ./...
git diff --check
```

All commands pass.

## Deferred Work

Phase 6 does not:

- execute status triggers
- execute Poison or Advanced Poison
- implement Baryl stack overflow
- implement Blind
- apply queued status operations
- evaluate battle completion
- implement healing from `removed`
- implement replay or disk-backed checkpoint recovery
- add Godot or presentation behavior

## Phase 7 Prerequisites and Recommendations

1. Consume `Battle.PendingOperations` and compiled status trigger plans without
   re-reading YAML.
2. Execute Poison and Advanced Poison through the existing generic
   `roll_dice`, `evaluate_roll_outcome`, `deal_damage`, and
   `remove_status_stack` plans.
3. Create one simultaneous roll batch across compatible status instances, then
   feed resulting damage into the Phase 6 proposal path.
4. Preserve status instance ID, stack index, roll proposal ID, source actor,
   and originating operation through damage attribution.
5. Use the existing Phase 3 reaction chain for status rolls and the existing
   Phase 6 reaction chain for resulting damage cards.
6. Keep `apply_status` and `remove_status_stack` execution generic. Do not add
   Poison or Advanced Poison operation handlers.
7. Add immediate-consequence scheduling before implementing Baryl's custom
   stack-overflow policy.
8. Leave battle-result evaluation at segment exit for Phase 9; pending defeat
   must not terminate Phase 7 nested work.
