# Story 3: Synchronized Segment Flow Progression

## Purpose

Define how every segment flow progresses automatically until the battle reaches a point where human input is required.

All actors share the same current segment and the same internal flow stage. Actors may resolve their work at different speeds, but the flow cannot move to its next stage until every required actor has resolved the current stage.

The engine should continue through automatic work, internal flow stages, and complete segments. It returns control only when at least one human-controlled actor must provide input or when an error prevents safe progression.

## Core Rules

```text
Each flow owns its internal stages and completion rules.
The engine repeatedly asks the current flow to progress.
The flow advances as far as it can without human input.
The segment manager only calculates the next segment.
All required actors must resolve a stage before the flow advances.
```

No segment is inherently automatic or interactive.

- `ongoing_effects` usually resolves automatically, but an effect may require a choice.
- `income` usually draws cards and gains energy automatically, but an effect may require a choice.
- `offensive` usually requires actors to roll, select an ability, or pass.
- `defensive` may auto-complete when nobody can defend or may require defensive actions.
- `damage_resolution` may resolve automatically or may open a reaction or prevention opportunity.

## Before Coding, Read

- `docs/godot_pve_player_combat_flow/2_offensive_flow_dice_command.md`
- `docs/godot_pve_player_combat_flow/6_actor_segment_readiness_gates.md`
- `docs/godot_pve_player_combat_flow/7_advance_until_first_wait.md`
- `docs/godot_pve_player_combat_flow/8_passive_segment_trigger_hooks.md`
- `docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md`
- `dice-and-destiny-server/internal/battle/engine/flow.go`
- `dice-and-destiny-server/internal/battle/engine/engine.go`
- `dice-and-destiny-server/internal/battle/engine/command.go`
- `dice-and-destiny-server/internal/battle/segment/manager.go`
- Reference only:
  - `/Users/daddymere/games/Dice-and-Destiny/docs/deep-dive-2025-10-12/04_SEGMENT_LIFECYCLE.md`
  - `/Users/daddymere/games/Dice-and-Destiny/docs/deep-dive-2025-10-12/10_CALLBACKS_AND_HOOKS.md`

## Current V2 Problems This Story Must Fix

- `Engine.AdvanceSegment` does not consult `SegmentFlow.CanAdvance` before exiting and advancing.
- `OffensiveFlow.OnEnter` can return `wait_for_command` while `OffensiveFlow.CanAdvance` returns `ready_to_advance`.
- More than one method can currently claim authority over the flow decision.
- New battles start in `ongoing_effects`, but battle state does not record whether its entry work has run.
- There is no persisted internal stage within a segment.
- There is no generic way to represent one actor being locked in while another actor still needs input.
- A command result does not identify the input currently required from an actor.
- Repeated automatic work cannot yet accumulate events across multiple stages and segments.

## Global Progress Outcomes

Use two normal engine-level outcomes:

```text
continue
waiting_for_input
```

`continue` means the engine should keep progressing. It is normally internal to the engine loop and does not need to be returned as a stopping state.

`waiting_for_input` means at least one human-controlled actor must send a command before progression can continue.

Do not add a separate `blocked` outcome. A required trigger choice, reaction, card decision, required roll, or other nested resolution is still a wait for actor input. Its reason is represented by the current flow stage and pending input description.

Do not add both `complete` and `ready_to_advance` as equivalent decisions. Segment completion should be one explicit flow result:

```text
segment_complete
```

Errors are not flow outcomes. An error rejects the current command or progression attempt without pretending the flow is waiting normally.

Suggested status type:

```go
type ProgressStatus string

const (
	ProgressContinue        ProgressStatus = "continue"
	ProgressWaitingForInput ProgressStatus = "waiting_for_input"
	ProgressSegmentComplete ProgressStatus = "segment_complete"
)
```

## Actor Progress Is Separate From Global Progress

An actor being ready does not mean the segment can advance.

Suggested actor-stage states:

```text
resolving_automatic
needs_input
locked_in
resolved
not_participating
```

Meanings:

- `resolving_automatic`: engine-controlled work can still run for this actor.
- `needs_input`: this human-controlled actor must submit a command.
- `locked_in`: this actor has made a hidden commitment and is waiting for other actors.
- `resolved`: this actor has completed the current stage.
- `not_participating`: this stage does not apply to this actor.

An AI-controlled enemy should normally progress through `resolving_automatic` without stopping the engine. It may reach `locked_in` or `resolved` before the human player acts.

The global flow waits only when a human-controlled actor is `needs_input`.

The current internal stage advances only when every required actor is `locked_in`, `resolved`, or `not_participating`, according to that stage's rules.

## Persisted Flow State

Battle state needs persisted segment-flow state so entry work and internal stages execute exactly once.

Suggested shape:

```go
type SegmentFlowState struct {
	Segment      segment.Segment
	Round        int
	Entered      bool
	Stage        string
	Iteration    int
	Actors       map[string]ActorFlowState
	PendingInput map[string]PendingInput
}

type ActorFlowState struct {
	Status       ActorProgressStatus
	ReasonCode   string
	CommitmentID string
}

type PendingInput struct {
	ID              string
	ActorID         string
	Segment         segment.Segment
	Stage           string
	InputType       string
	SourceType      string
	SourceID        string
	AllowedCommands []command.Type
}
```

The exact field names may follow local conventions, but state must identify:

- current segment and round
- whether segment entry work has run
- current internal stage
- current stage iteration when a stage can repeat
- each actor's progress state
- pending human input
- hidden commitments needed by later stages

Constructing a battle must not silently execute segment work. The first engine progression call enters `ongoing_effects` exactly once.

## Flow Contract

There must be one authoritative progression method. `OnEnter`, `CanAdvance`, and command results must not return conflicting completion decisions.

Recommended interface direction:

```go
type SegmentFlow interface {
	ID() segment.Segment
	OnEnter(ctx *Context) ([]event.Event, error)
	Progress(ctx *Context) (ProgressResult, error)
	HandleCommand(ctx *Context, cmd command.Command) ([]event.Event, error)
	OnExit(ctx *Context) ([]event.Event, error)
}

type ProgressResult struct {
	Status       ProgressStatus
	Events       []event.Event
	PendingInput map[string]PendingInput
}
```

Responsibilities:

- `OnEnter` initializes the segment and its first internal stage exactly once.
- `Progress` performs all currently available automatic work and reports whether to continue, wait, or finish the segment.
- `HandleCommand` validates and applies one actor command for the current stage.
- after every accepted command, the engine calls `Progress` again
- `OnExit` performs segment cleanup only after `Progress` returns `segment_complete`

The implementation may retain `CanAdvance` temporarily, but only one method may be authoritative. The preferred end state is to replace `CanAdvance` with `Progress`.

## Engine Progression Loop

The engine owns the outer loop across stages and segments.

```text
ensure current segment OnEnter has run exactly once
repeat:
  ask current flow to Progress
  append emitted events

  if waiting_for_input:
    return accumulated viewer-safe events + pending input + snapshot

  if continue:
    remain in current segment and call Progress again

  if segment_complete:
    run current flow OnExit
    append exit events
    ask segment manager for next segment
    update segment state
    append segment_advanced event
    run next flow OnEnter exactly once
    append entry events
    continue loop
```

After an accepted gameplay command:

```text
validate command against current segment, stage, actor, and pending input
apply command
append command events
run the same progression loop
return at the next human input point
```

This means the client does not need a separate command for every automatic transition.

## Event Accumulation And Delivery

Automatic work should not pause merely to display an event.

The engine accumulates events in deterministic order until it reaches the next human input point.

Example at battle start:

```text
enter ongoing_effects
Artificer automatically gains synth
ongoing_effects completes
enter income
player draws a card
player gains 1 energy point
income completes
enter offensive planning
enemy AI rolls and locks in an action
player needs input
return all visible accumulated events + current snapshot + player's pending input
```

The returned result can contain:

```text
token_gained
segment_advanced: ongoing_effects -> income
cards_drawn
energy_points_gained
segment_advanced: income -> offensive
offensive planning state visible to the viewer
pending input: player must roll, select an ability, play an allowed card, or pass
```

When a player command is accepted, the engine again progresses until the next human decision. For example, after a dice roll it returns the roll result and waits for the player to reroll, select an ability, play an allowed card, or pass.

Events remain authoritative facts. Viewer filtering decides which facts are visible at each stage.

## Hidden Commitments And Reveal Gates

Actors may act simultaneously, but their choices are not necessarily immediately visible to opponents.

During offensive planning:

- player and enemy work within the same flow stage
- enemy AI may roll and select `stab` before the player starts
- enemy commitment is stored authoritatively
- enemy commitment is hidden from the player while the player is still planning
- enemy is `locked_in`
- player continues rolling, playing allowed cards, selecting an ability, or passing
- when every required actor is locked in, offensive planning completes

Only then does the flow enter its reveal stage and emit viewer-visible commitment events.

Hidden information must not leak through:

- events
- snapshots
- pending input descriptions
- counts or identifiers that reveal a selected ability before the reveal gate

For enemy planning, all roll and commitment details remain hidden until every required actor has locked in. This includes:

- rolled face values
- rolled symbols
- kept dice and reroll choices
- number of rolls used
- selected ability
- selected cards
- selected targets
- reaction commitments

Internal AI work may finish before the human acts, but its results are authoritative private state until the reveal gate.

When planning reveals, viewers receive only:

- final dice faces and symbols
- total rolls used
- selected ability
- committed cards
- selected targets

Do not reveal the roll-by-roll history, intermediate dice states, kept-dice history, or individual reroll choices.

The event/snapshot filtering policy should support an event being authoritative but not yet visible to an opposing viewer.

## Reaction Windows Are A General Flow Mechanism

Reactions are not exclusive to offensive. Any segment, stage, command, trigger, effect, reveal, or resolved reaction may create a reaction opportunity.

Examples include:

- reacting to an opponent's revealed ability
- playing a card that changes an opponent's die
- reacting to that die-changing card
- reacting to damage before it resolves
- reacting to an ongoing or income effect when a rule permits it

A reaction opportunity is a synchronized response step that temporarily suspends the originating segment flow.

Suggested reaction state:

```go
type ReactionWindow struct {
	ID               string
	PreviousWindowID string
	Segment          segment.Segment
	Stage            string
	Depth            int
	TriggerType      string
	TriggerID        string
	TriggerActorID   string
	EligibleActorIDs []string
	Round            int
	Actors           map[string]ActorFlowState
	Commitments      map[string]ReactionCommitment
	Status           ReactionWindowStatus
}
```

The exact shape may differ, but the battle must persist:

- what opened the reaction window
- the segment checkpoint the reaction chain suspended
- the previous reaction window in the response chain
- which actors are eligible
- each eligible actor's hidden response
- the current reaction round and response-chain depth
- whether the window is collecting, revealing, resolving, or complete

### Default Reaction Policy

Unless a card, ability, status, or effect explicitly says otherwise:

- a reactable action opens a reaction opportunity for every eligible opposing actor
- eligible actors choose a reaction or pass
- all reaction choices are committed secretly
- AI actors decide automatically and commit secretly
- the engine waits only for eligible human actors
- no reaction commitment is revealed until all eligible actors have locked in
- the committed reaction batch is then revealed
- non-conflicting reactions resolve in deterministic stable order
- directly conflicting simultaneous reactions negate each other's conflicting actions
- a revealed or resolved reaction may open another reaction opportunity
- a new reaction window continues the response chain before pending actions finish resolving
- if every eligible actor passes, the reaction window closes
- passing applies only to the current reaction round; a later reactable action can make that actor eligible again

Special rules may later mark an action:

```text
not_reactable
limited_reactors
restricted_reaction_types
```

Those are exceptions. The default is that a qualifying action gives the other side an opportunity to react.

### Simultaneous Reaction Collisions

The first implementation does not assign one actor automatic priority when simultaneous hidden reactions collide.

If two or more reactions attempt incompatible changes to the same game value or resolution target, the conflicting actions negate each other.

Example:

```text
Player secretly commits a card that changes Goblin die 2 to a six.
Goblin secretly commits a card that changes Goblin die 2 to a one.
Both commitments reveal together.
Both actions target the same die with incompatible replacement values.
Neither dice-changing effect is applied.
```

Collision rules:

- only the directly conflicting actions are negated
- unrelated actions in the same reveal batch still resolve
- committed cards are still spent when their effects are negated by a collision
- all committed card costs are still paid, including energy and any other required resources
- collision-negated cards move to their normal post-play zone, such as discard, unless the card explicitly defines another destination
- collision negation cancels effect resolution; it does not undo the card play or refund its costs
- identical compatible effects are not automatically collisions unless their content rules say they cannot stack
- additive, replacement, prevention, redirection, cancellation, and target-changing effects need explicit conflict classification
- conflict detection belongs in a reaction/effect rule package, not in the segment manager
- collision results emit explicit events identifying the negated commitments without leaking information before reveal
- a negated effect cannot open a later response window from an effect that never occurred
- a card, ability, or effect may later define explicit priority or collision behavior that overrides this default

Suggested result concept:

```go
type ReactionResolution struct {
	CommitmentID string
	Status       ReactionResolutionStatus
	ConflictIDs  []string
}

const (
	ReactionApplied         ReactionResolutionStatus = "applied"
	ReactionPassed          ReactionResolutionStatus = "passed"
	ReactionNegatedCollision ReactionResolutionStatus = "negated_collision"
)
```

The initial rule is intentionally conservative. Future stories may add initiative, effect speed, layered resolution, explicit card priority, or more precise conflict matrices as real collision cases are introduced.

### Reaction Chains

Reaction chains must be first-class behavior rather than a special-case callback.

Example:

```text
1. Stab and Punch are revealed.
2. Player commits a hidden die-changing card; Goblin commits pass.
3. Both commitments reveal.
4. The die-changing card is reactable.
5. The next response window opens before the card finishes resolving.
6. Goblin commits a counter reaction; Player commits pass.
7. Both commitments reveal.
8. The counter reaction resolves.
9. The die-changing card then resolves or is cancelled as rules determine.
10. The reaction chain finishes when no pending action creates another reaction opportunity.
11. Control returns directly to the suspended offensive checkpoint.
```

The reaction system therefore needs a persisted response chain plus a reference to the suspended segment checkpoint. Do not implement reactions using blocking goroutines, sleeps, or in-memory callbacks that cannot survive command boundaries.

A completed reaction window is not reopened after a later response window finishes. Each new window moves the response chain forward. Once the chain is exhausted, control returns to the originating segment flow rather than returning through old reaction collection windows.

### Reaction Completion

A reaction window does not complete because one actor passes.

It completes when:

- every eligible actor has committed for the current reaction round
- the hidden commitments have been revealed
- all commitments for that round have resolved
- no revealed or resolved commitment opened the next reaction window
- no unresolved action remains in the response chain

The originating segment remains suspended while any reaction window or unresolved reaction commitment remains active. When the chain finishes, the segment resumes from its persisted checkpoint.

Add separate configurable guards for:

- maximum response-chain depth
- maximum reaction rounds per originating checkpoint
- maximum total automatic progression steps

Exceeding a guard is an explicit engine error. It must not silently pass, discard, or auto-resolve reactions.

## Offensive Flow Example

The offensive flow is not one wait followed by segment completion. It contains multiple internal stages.

Suggested initial stage model:

```text
planning
reveal
reaction
revalidation
complete
```

Example:

```text
Round 1 offensive: planning

1. Goblin AI rolls once and rerolls twice.
2. Goblin selects Stab and becomes locked_in.
3. Goblin's dice, rerolls, selected ability, cards, and target remain hidden from Player 1.
4. Player 1 rolls once and rerolls twice.
5. Player 1 selects Punch and becomes locked_in.
6. All required actors are locked in, so planning completes.

Round 1 offensive: reveal

7. The locked planning results are revealed to eligible viewers.
8. Player 1 can now see the Goblin's final dice state, rolls used, selected Stab ability, committed cards, and targets.
9. The Goblin controller can now inspect Player 1's revealed planning result.
10. The flow determines who may react.

Round 1 offensive: reaction

11. Player 1 secretly commits a card that changes one goblin die.
12. Goblin AI secretly commits pass.
13. Both reaction commitments reveal together.
14. The player's card creates a new reaction opportunity for the Goblin.
15. The response chain continues before the card finishes resolving.
16. When no further response is committed, the pending reaction actions resolve.
17. Control returns directly to the suspended offensive checkpoint.

Round 1 offensive: revalidation

18. Stab is no longer valid.
19. Goblin returns to offensive planning with its current dice state and one roll already used.
20. Goblin may use its remaining rolls, select another valid ability, play an allowed card, or pass.
21. If an effect increases Goblin's `max_rolls`, the newly available roll can be used.
22. The resulting commitment is revealed and may create another reaction chain.
23. When no actor has another required decision and no reaction chain is active, offensive completes.
24. Engine exits offensive and advances to defensive.
```

The exact invalidated-action behavior is a later offensive-flow story. Story 3 must provide the stage, synchronization, waiting, iteration, and hidden-information contracts needed to support it.

## Offensive Checkpoints And Re-entry

Offensive must persist a checkpoint that can be resumed after a reaction chain.

Suggested checkpoint data includes:

```go
type OffensiveCheckpoint struct {
	Stage       string
	Iteration   int
	Actors      map[string]OffensiveActorState
	Commitments map[string]OffensiveCommitment
}

type OffensiveActorState struct {
	RollsUsed     int
	MaxRolls      int
	Dice          []state.RolledDie
	KeptIndices   []int
	SelectedActionID string
	Status        ActorProgressStatus
}
```

When reactions finish:

1. return directly to the suspended offensive checkpoint
2. revalidate all affected offensive commitments against authoritative state
3. preserve each actor's final dice, kept dice, `rolls_used`, and `max_rolls`
4. preserve spent cards, paid costs, applied effects, and other resolved state changes
5. reopen offensive planning only for an actor whose commitment or available action state was materially changed
6. continue through lock-in, reveal, reactions, and revalidation again as needed

Unaffected actors remain locked in.

For example:

```text
Player 1 committed Punch.
Goblin committed Stab.
A reaction changes a Goblin die and invalidates Stab.
Goblin returns to planning.
Player 1 remains locked into Punch because Punch and Player 1's action state were not changed.
```

Targeted re-entry rules:

- revalidate actors whose dice, commitment, target, costs, available rolls, legal actions, or controlling effects changed
- unlock only actors who need another decision because of those changes
- unaffected actors retain their existing locked commitment
- unaffected actors do not reroll, select another ability, replay cards, or pass again
- already revealed unaffected commitments remain known; they are not treated as newly hidden
- the affected actor's replacement rolls, cards, targets, and commitment remain hidden until that actor locks in again
- after all reopened actors lock in, reveal only the new or changed commitment data needed for the next cycle
- the previously locked commitments still participate in later revalidation
- if a later reaction materially changes an unaffected actor, that actor can then be reopened

An actor can be materially changed even when its currently selected ability remains valid. Examples include:

- the actor gains another offensive roll and must be allowed to decide whether to use it
- the actor's target becomes invalid
- the actor's action cost changes
- the actor gains or loses access to a legal card or ability that requires a decision

Purely informational changes do not unlock an actor.

An invalidated actor does not receive a fresh default roll pool. For example:

```text
Goblin used 1 of 3 rolls and selected Stab.
A reaction changes a die and invalidates Stab.
Goblin resumes with rolls_used=1 and max_rolls=3.
Goblin still has 2 normal rolls available.
```

Using all original rolls does not necessarily resolve the actor:

```text
Goblin used 3 of 3 rolls and selected Stab.
A reaction invalidates Stab.
Goblin has a card or effect that grants +1 offensive roll.
The effect changes max_rolls from 3 to 4.
Goblin resumes planning with rolls_used=3 and one roll available.
```

After revalidation, an actor may still:

- select another ability already satisfied by the current dice
- use any remaining roll
- gain and use an additional roll through a legal card or effect
- play another card allowed during offensive planning
- pass

The actor becomes resolved or auto-passed only when the rules determine that no further legal decision remains. Reaching the original default roll count alone is not sufficient.

Offensive can repeat this checkpoint cycle:

```text
planning
-> reveal
-> reaction chain
-> revalidation
-> affected actor planning
-> reveal
-> another reaction chain
-> revalidation
-> complete
```

Reaction collection windows do not restart during this cycle. The offensive checkpoint is the reusable originating state; each reaction chain is consumed once and then discarded after its results are applied.

## Flow Examples

### Automatic Ongoing Effects

```text
enter ongoing_effects
run automatic triggers
Artificer gains synth
no actor needs input
segment_complete
```

### Ongoing Effect Requiring Input

```text
enter ongoing_effects
run automatic triggers
effect requires Player 1 to choose a discarded card
Player 1 = needs_input
return waiting_for_input
```

After the choice command:

```text
apply choice
Player 1 = resolved
continue remaining automatic effects
segment_complete
```

### Automatic Income

```text
enter income
draw cards
gain energy
run income triggers
no actor needs input
segment_complete
```

### Synchronized Offensive Planning

```text
Player 1 = needs_input
Goblin AI = resolving_automatic
engine resolves Goblin AI
Goblin AI = locked_in
Player 1 still needs input
global result = waiting_for_input
```

After Player 1 commits:

```text
Player 1 = locked_in
Goblin AI = locked_in
planning stage advances to reveal
flow continues automatically until a reaction input is needed
```

## Command Validation

Every gameplay command must be validated against:

- battle ID
- actor ID
- actor controller type
- current segment
- current internal stage
- pending input ID when applicable
- allowed command types for that pending input
- whether the actor has already locked in or resolved the stage

Reject stale commands from an earlier stage or iteration.

Commands from an actor who is already `locked_in`, `resolved`, or `not_participating` are rejected unless the current flow explicitly allows changing or withdrawing a commitment.

## Controller Types

Actor setup/state should distinguish at least:

```text
human
ai
system
```

- human actors can cause `waiting_for_input`
- AI actors are progressed by an injected controller/strategy
- system actors/effects resolve through deterministic domain behavior

Do not place enemy decision logic in the segment manager.

For Story 3 tests, fake flows and fake controllers are enough. Full enemy AI is out of scope.

## Initial Runtime Topology And Server-Ready Shape

The initial supported runtime is:

```text
one human-controlled player
versus
one or more AI-controlled enemies
```

This story does not implement networking, remote players, matchmaking, or a multiplayer server.

The domain model must still avoid single-player assumptions:

- store actors by stable actor ID
- store controller type per actor
- represent pending input as a collection keyed by actor ID
- represent readiness and commitments as actor collections
- pass viewer actor ID into event and snapshot filtering
- do not hard-code `player` and `enemy` fields into flow state
- do not put local UI callbacks into engine/domain packages
- do not assume only one actor can be eligible for a stage or reaction

The local command boundary may return only the one human viewer's filtered result for now. Internally, the engine should produce authoritative state and events that can later be filtered independently for multiple connected human viewers.

## Transition Safety

- `OnExit` only runs after the flow reports `segment_complete`.
- Segment state changes only after successful exit work.
- `OnEnter` runs exactly once for each segment/round entry.
- A failed progression step must not silently advance to another stage or segment.
- A failed segment entry must not leave the battle pretending the new segment was successfully entered.
- Duplicate engine calls while waiting must not repeat automatic rewards, triggers, or entry events.
- The engine must reject invalid progress statuses.

Where practical, calculate mutations/events before committing them. If full transactional rollback is not practical yet, structure each progression step so validation occurs before mutation and add tests against duplicate entry effects.

## Loop Guard

Automatic progression needs a maximum number of steps per engine call.

The guard counts:

- internal flow-stage progress steps
- automatic actor controller steps
- segment transitions

If the guard is exceeded:

- return an explicit engine error
- include enough segment/stage context for diagnosis
- do not force actors to pass
- do not silently skip work

The default limit should be high enough for normal automatic triggers but finite and configurable in tests.

## Scope

- Replace the ambiguous flow-decision contract with the synchronized progression contract.
- Add persisted segment-flow state.
- Add exact-once segment entry tracking.
- Add current internal stage tracking.
- Add actor progress states.
- Add pending input descriptions.
- Add actor controller type needed to distinguish human and automatic actors.
- Add generic persisted reaction-chain state linked to its originating segment checkpoint.
- Add hidden simultaneous reaction commitments and reveal gates.
- Add default simultaneous reaction collision detection and mutual negation results.
- Add one authoritative flow progression method.
- Add an engine loop that progresses until human input is required or an error occurs.
- Accumulate events across automatic stages and segment transitions.
- Return pending input and the final viewer-safe snapshot.
- Validate commands against segment, stage, actor state, and pending input.
- Add a configurable automatic-step guard.
- Keep the segment manager limited to deterministic next-segment calculation.

## Minimum Implementation For This Story

Use simple/fake flows to establish the contract before implementing the full offensive lifecycle.

The minimum real behavior should prove:

- initial `ongoing_effects` entry runs exactly once
- default ongoing effects complete automatically
- income entry performs its current automatic rewards
- income completes automatically
- offensive entry creates its current roll opportunity
- progression stops in offensive because the human player needs input
- an AI actor can resolve and lock in without causing the engine to return early
- accumulated events are returned in deterministic order

Do not implement the full planning/reveal/reaction/revalidation rules in this story.

## Out Of Scope

- Complete enemy AI.
- Full offensive ability selection.
- Complete card/ability-specific reaction effects.
- Invalidated offensive action rules.
- Full trigger registry.
- Full status effect system.
- Card effect resolution.
- Damage mechanics.
- Godot, C++, or UI changes.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add focused tests proving:

- a new battle enters `ongoing_effects` exactly once
- repeated progression while waiting does not repeat entry work
- automatic ongoing effects advance into income
- automatic income rewards are applied exactly once
- automatic income advances into offensive
- offensive waits when a human actor needs input
- accumulated ongoing, income, segment transition, and offensive events are returned in order
- a flow can require human input in any segment
- an ongoing-effect choice can stop progression before income
- an AI actor resolves automatically without returning `waiting_for_input`
- an AI actor may become locked in while a human actor still needs input
- one locked-in actor does not advance the shared stage while another required actor still needs input
- all required actors locked in allows the flow to continue
- hidden AI commitments are not visible before the reveal stage
- enemy dice values, symbols, rerolls, roll count, cards, targets, and selected ability remain hidden before reveal
- reveal exposes final dice, rolls used, selected ability/cards, and targets without roll history
- reaction commitments remain hidden until every eligible actor has locked in
- directly conflicting simultaneous reactions negate each other
- non-conflicting reactions in the same batch still resolve
- collision results identify all negated commitment IDs
- collision-negated cards still pay costs and move to their normal post-play zone
- collision negation does not refund energy or other resources
- negated collision effects do not mutate their targets
- negated collision effects do not create later response windows
- a reaction can create the next response window in its chain
- originating segment progression pauses while a reaction chain is active
- a reaction chain returns directly to its originating segment checkpoint
- all-pass closes a reaction window
- one actor passing does not close a reaction window while another actor has committed a reaction
- response-chain depth and reaction-round guards reject runaway chains explicitly
- reaction completion does not reopen an earlier reaction collection window
- offensive revalidation preserves final dice, kept dice, rolls used, and max rolls
- an actor invalidated after using 1 of 3 rolls resumes with 2 rolls available
- an actor invalidated after using 3 of 3 rolls can use a newly granted fourth roll
- reaching the original default roll count does not auto-pass an actor who still has a legal card, ability, added roll, or pass decision
- an invalidated offensive actor can recommit and create a new reveal/reaction cycle
- only materially affected actors reopen offensive planning
- unaffected actors retain their locked commitments
- unaffected actors do not repeat rolls, card plays, ability selection, or pass decisions
- replacement commitments remain hidden until reopened actors lock in
- subsequent reveal exposes only newly changed commitment information
- a command is rejected for the wrong segment, stage, actor, or pending input
- stale commands from an earlier iteration are rejected
- `OnExit` does not run before `segment_complete`
- segment manager does not know why advancement occurred
- invalid progress statuses are rejected
- the automatic-step guard rejects an infinite automatic flow
- failures do not duplicate entry rewards or silently advance the segment

## Definition Of Done

- Every segment can resolve automatically or wait for human input.
- The engine progresses across internal stages and segments until human input is required.
- Actor readiness is synchronized without treating one actor's readiness as global readiness.
- Offensive re-entry is targeted to materially affected actors while unaffected actors remain locked.
- AI-controlled actors can resolve automatically and lock in while humans are still acting.
- Hidden commitments remain hidden until the flow's reveal gate.
- Reveal exposes final planning results without exposing roll-by-roll history.
- Reactions are a reusable nested flow mechanism available throughout the game.
- Reactions to reactions are persisted as a forward-moving chain before the originating segment checkpoint resumes.
- Direct simultaneous reaction collisions use mutual negation until richer priority rules are introduced.
- The supported runtime is one human versus AI actors without hard-coding that topology into domain state.
- Automatic events are accumulated and returned at the next human input point.
- Segment entry work is exact-once.
- The segment manager remains deterministic and unaware of actors, input, hooks, AI, cards, dice, or abilities.
- Go tests pass.
- No Godot, C++, or UI files are changed.
