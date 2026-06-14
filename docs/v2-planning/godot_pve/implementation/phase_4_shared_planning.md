# Phase 4: Shared Synchronized Planning

## Status

Implemented June 14, 2026.

## Completed Behavior

Offensive and defensive now use one reusable synchronized planning
implementation configured by segment.

The shared lifecycle is:

```text
initialize eligible and non-participating actors
-> collect private planning actions
-> automatically plan and lock AI/system actors
-> wait for required human actors
-> simultaneously reveal final commitments
-> open the generic Phase 3 reaction chain
-> revalidate reaction changes
-> reopen only materially affected actors
-> repeat reveal/reaction/revalidation as needed
-> persist finalized planning proposals
```

Supported private planning actions are:

- roll dice
- keep dice
- reroll selected dice
- commit one or more allowed cards
- select an allowed ability
- select one or more targets
- pass
- lock in

Each participating actor persists:

- final dice with face number, value, and symbols
- kept indices
- rolls used and maximum rolls
- selected ability
- committed cards
- selected targets
- pass state
- lock-in state
- action and reveal revisions
- paid-cost and resolved-card markers reserved for later operation rules

Multiple human and AI-controlled actors can plan in the same hidden window.
AI actors complete automatic work without causing an early engine return.

## Files Changed

### Shared Planning and Segment Configuration

- `dice-and-destiny-server/internal/battle/engine/planning.go`
- `dice-and-destiny-server/internal/battle/engine/offensive_flow.go`
- `dice-and-destiny-server/internal/battle/engine/defensive_flow.go`
- `dice-and-destiny-server/internal/battle/engine/resolution.go`
- `dice-and-destiny-server/internal/battle/engine/command.go`

### State, Commands, Persistence, and Visibility

- `dice-and-destiny-server/internal/battle/state/battle.go`
- `dice-and-destiny-server/internal/battle/state/resolution.go`
- `dice-and-destiny-server/internal/battle/command/command.go`
- `dice-and-destiny-server/internal/battle/event/event.go`
- `dice-and-destiny-server/internal/battle/snapshot/snapshot.go`
- `dice-and-destiny-server/internal/battle/repository/repository.go`

### Tests

- `dice-and-destiny-server/internal/battle/engine/planning_test.go`
- `dice-and-destiny-server/internal/battle/engine/engine_test.go`
- `dice-and-destiny-server/internal/battle/state/battle_test.go`

## Shared Configuration and State Contracts

`SharedPlanningFlow` is configured with:

```text
segment: offensive | defensive
default maximum rolls
eligible actors
non-participating actors and reason codes
eligible targets per actor
AI planning policy
```

`ResolutionState.Planning` persists the segment, cycle, actor planning states,
changed actors, applied reaction windows, and finalization state. Planning uses
`InteractionPurposePlanning`; revealed batches use the existing Phase 3
proposal and reaction-window contracts.

Every targeted re-entry creates a new planning window with a new window ID and
planning cycle. Only affected actors are required by that window. Unaffected
actors remain locked.

Final accepted output is stored in:

```text
Battle.OffensiveProposals
Battle.DefensiveProposals
```

Offensive proposals are data for later defense and damage phases. Defensive
proposals are persisted modifiers/counter-effect proposals for later
resolution. Phase 4 does not apply attack damage, statuses, counter-damage, or
permanent card removal.

## Commands Introduced

Typed planning commands are:

```text
planning_roll
planning_keep
planning_reroll
planning_commit_cards
planning_select_ability
planning_select_targets
planning_pass
planning_lock_in
```

Each payload carries:

```text
pending input ID
planning window ID
segment
stage
iteration
planning cycle
command-specific data
```

Commands validate battle, actor, controller, segment, stage, iteration,
planning cycle, active window, pending input, participation, lock state, and
the current allowed-command set.

Accepted typed commands rotate the pending input ID. Reusing a command is
therefore stale. Locked actors have no planning pending input and cannot act
unless revalidation explicitly reopens them.

`roll_dice` remains as a compatibility alias for existing Phase 1-3 callers.
New planning clients should use the typed planning commands.

## Reveal and Visibility Policy

Planning state is private until every required actor in the current planning
window is locked.

The simultaneous reveal contains:

- final die face number, value, and symbols
- rolls used and maximum rolls
- selected ability or pass
- committed cards
- selected targets

It does not expose:

- intermediate roll history
- reroll history
- another actor's unrevealed replacement commitment
- another actor's kept indices

The viewer snapshot exposes the viewer's current private planning state and
only the last revealed commitment for other actors. During targeted re-entry,
the previously revealed commitment remains public while the replacement stays
hidden until lock-in.

Subsequent planning cycles reveal only commitments from actors required by the
new targeted window.

## Revalidation and Material Change Rules

After a reaction chain closes, Phase 4 applies typed temporary planning
adjustments and compares their actual effects.

An actor is materially affected when an accepted adjustment:

- changes a final die face
- increases maximum rolls
- clears the selected ability
- removes a selected target
- explicitly requests actor re-entry

Only materially affected actors reopen.

Re-entry preserves:

- current final dice
- kept indices
- rolls already used
- maximum rolls
- committed cards
- selected ability and remaining targets unless specifically changed
- paid-cost markers
- resolved-card markers

Increasing `max_rolls` updates both the planning state and the authoritative
roll request. Newly available rolls become legal immediately. Reaching the
original default roll count does not pass or lock the actor while another
legal decision remains.

## Offensive Configuration

Every battle actor participates in offensive planning. Each may use offensive
dice, mock offensive abilities, cards, actor targets, pass, and lock in.

Accepted offensive commitments are persisted as finalized proposals.
Defensible proposals are those that did not pass and have at least one target.

Phase 4 intentionally does not:

- apply damage
- apply attack-delivered statuses
- run Blind
- run other offensive `OnExit` effects

Blind remains a Phase 8 prerequisite.

## Defensive Configuration

Defensive planning starts only when a finalized offensive proposal is marked
defensible.

Actors with no incoming defensible proposal are marked:

```text
not_participating
reason: no_incoming_defensible_proposal
```

Eligible defenders use the same planning commands, hidden commitment state,
reveal, reaction chain, revalidation, and targeted re-entry code as offensive.
Their eligible targets are the incoming finalized offensive proposal IDs.

Final defensive commitments are persisted without applying damage, resolving
counter-effects, or permanently removing cards.

## Temporary Phase 4 Adapters and Limitations

Phase 4 deliberately does not implement the Phase 5 YAML operation registry.

Two focused adapters prove the lifecycle:

1. Mock ability classification uses ability ID text. IDs containing `guard` or
   `defen` are temporarily defensive; other IDs are temporarily offensive.
2. Reaction commitments may carry typed planning adjustments:
   `set_die_face`, `increase_max_rolls`, `clear_ability`, `remove_target`, and
   `reopen_actor`.

Phase 5 must replace ability-name classification and reaction-adjustment
interpretation with validated content definitions and registered operations.

Other current limitations:

- card costs and effects are recorded but not executed
- paid costs are not yet created by production rules
- defensive modifiers and counter-effects are proposals only
- the complete reaction conflict matrix remains deferred
- the repository remains in-memory and does not survive process restart

## Tests and Results

Phase 4 tests cover:

- offensive and defensive using the same shared implementation
- multiple human and AI actors
- AI lock-in while humans still need input
- hidden planning state and viewer-safe pending input
- simultaneous reveal with final face numbers and symbols
- no intermediate roll or opponent keep-history leakage
- multiple committed cards and multiple targets
- reveal only after every required actor locks
- reaction-chain suspension
- targeted re-entry after a material reaction
- unaffected actors remaining locked
- preserved dice, keeps, rolls, cards, targets, paid costs, and resolved markers
- remaining rolls and reaction-granted rolls
- legal decisions at the original default roll count
- repeated planning/reveal/reaction cycles
- delta-only replacement reveal
- defensive participation and defensive skip behavior
- finalized offensive and defensive proposals
- no permanent card removal
- stale, duplicate, and locked-actor command rejection
- repository save/load during planning, reaction, and re-entry
- deep-copy isolation for planning state

### Explicit Defensive Test Evidence

Defensive behavior is not inferred only from an offensive test parameterized
with `segment.Defensive`.

`TestDefensiveFlowRunsActualPlanningRevealReactionAndFinalization` starts a
battle directly in the defensive segment with a persisted incoming defensible
offensive proposal and registers the production `DefensiveFlow`. It verifies:

- `DefensiveFlow.OnEnter` derives the eligible defender from the incoming
  proposal
- actors without incoming proposals are marked `not_participating`
- defensive pending input allows roll, cards, defensive ability, incoming
  proposal targets, and pass
- the human performs a defensive roll and commits two cards
- `Hero Guard` is selected as a defensive ability
- the incoming offensive proposal ID is selected as the defensive target
- defensive lock-in produces a planning reveal
- the reveal opens the generic reaction window
- a shared reaction pass finalizes the defensive batch
- a persisted `Battle.DefensiveProposals` entry contains the defensive dice,
  cards, ability, and target
- the engine advances from defensive to damage resolution

`TestSharedPlanningOffensiveDefensiveReactionReentryAndPersistence` separately
proves the natural offensive-to-defensive transition with both a human
defender and an automatically planned AI defender.

`TestPlanningSupportsMultipleTargetsAndDefensiveSkipsWithoutIncomingProposals`
proves that defensive planning is skipped when there is no incoming defensible
proposal.

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

## Phase 5 Prerequisites and Recommendations

1. Replace the temporary ability-name adapter with loaded typed ability
   definitions, segment restrictions, dice requirements, costs, and targets.
2. Register typed operations for card play, costs, dice changes, targeting,
   damage proposals, statuses, and resources.
3. Convert Phase 4 planning adjustments into normal registered reaction
   operations with conflict classification.
4. Preserve `PlanningState`, planning window IDs, cycles, revisions, and
   targeted re-entry semantics; content operations should mutate those
   contracts rather than replace the lifecycle.
5. Generate offensive and defensive proposal payloads from validated ability
   and card operations while retaining stable source and target attribution.
6. Add real cost payment and resolved-card effects without refunding or
   replaying them during targeted re-entry.
7. Keep attack consequences deferred until the damage-resolution phase.
