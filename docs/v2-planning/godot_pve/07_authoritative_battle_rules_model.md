# Authoritative Battle Rules Model

## Purpose

This document defines the intended high-level battle rules and runtime model for Dice and Destiny V2.

It is the target design used to evaluate the current implementation. It does not claim that the existing Go code already supports every rule described here.

The design priorities are:

- one authoritative Go battle engine
- a small, stable segment lifecycle
- simultaneous hidden decisions followed by shared reveals
- reusable reaction and interaction windows
- YAML-authored cards, abilities, dice, characters, enemies, and statuses
- reusable Go operations instead of one-off content code
- deterministic events, viewer-safe snapshots, save recovery, and battle replay
- one human-controlled player versus one or more AI-controlled enemies initially
- internal data structures that do not prevent future multiplayer

## Current Implementation Note

As of June 14, 2026, Phase 1 of the lifecycle described here is implemented:

- `start_battle` accepts one player descriptor and one or more enemy descriptors
- descriptors carry stable instance IDs and reusable definition IDs
- an injected participant assembler produces authoritative `BattleSetup`
- a battle repository owns checkpoints by battle ID
- the in-memory repository supports create, load, and save with defensive copies
- accepted commands load the existing checkpoint, apply the command, append authoritative events, and save the updated checkpoint
- engine responses distinguish `waiting_for_input` from `battle_complete` and include the terminal battle result

The default exported authority still needs the Phase 2 production participant assembler that loads mutable player state and fresh enemy definitions. The current repository is in-memory, so process-restart recovery, disk persistence, event sequencing, and replay remain deferred.

## Core Battle Loop

Battles use this segment order:

```text
ongoing_effects
-> income
-> offensive
-> defensive
-> damage_resolution
-> ongoing_effects
```

Round 1 begins in `ongoing_effects`.

The round increments only when `damage_resolution` completes and the battle advances back to `ongoing_effects`.

All participating actors share:

- the same current round
- the same current segment
- the same current segment phase
- the same active nested resolution or interaction window

Actors may finish their hidden decisions at different times, but shared progression waits until every required actor has committed, passed, resolved, or been marked not participating.

## Stable Segment Lifecycle

Every segment exposes only three engine-level gameplay checkpoints:

```text
on_enter
in_progress
on_exit
```

These checkpoints are data, not a growing list of functions.

Conceptually:

```text
TriggerCheckpoint {
    segment: ongoing_effects
    phase: on_enter
}
```

The same generic trigger/effect system receives the current segment and phase. Do not create a separate engine function for every possible timing such as `before_draw`, `after_roll`, or `before_damage`.

More detailed behavior occurs inside persisted nested resolutions while the segment remains at one of its three checkpoints.

Example:

```text
segment: ongoing_effects
phase: on_enter

nested poison resolution:
collect rolls
-> reveal rolls
-> reaction chain
-> finalize rolls
-> propose damage
-> reveal damage cards
-> reaction chain
-> commit damage
-> complete poison processing
```

## Responsibility Boundaries

### Segment Package

The segment package owns:

- valid segment IDs
- deterministic next-segment calculation
- round wrap behavior

It does not own:

- actors
- dice
- cards
- abilities
- effects
- reactions
- AI
- damage
- snapshots
- reasons why a segment may advance

### Engine

The engine owns orchestration:

```text
identify current segment and phase
-> enter exactly once
-> ask the current flow to progress
-> run automatic actors and rules
-> stop when human input is required
-> accept and validate commands
-> resume progression
-> exit a completed segment
-> evaluate battle completion
-> advance when allowed
```

The engine must not contain content-specific branches such as:

```text
if status == poison
if ability == poisoned_stab
if character == cursed_pirate
```

### Segment Flows

Each segment flow understands how its segment uses:

- `on_enter`
- `in_progress`
- `on_exit`
- nested resolutions
- actor synchronization
- pending input
- reveal and reaction cycles
- completion rules

A flow coordinates mechanics but delegates the mechanics themselves to rule packages.

### Rule Packages

Rule packages own reusable mechanics such as:

- dice rolling and manipulation
- card movement
- damage proposal and application
- status application and removal
- resource changes
- targeting
- ability requirement validation
- reaction conflict detection
- battle-result evaluation

### Content

YAML content defines how reusable operations are configured and composed.

Godot owns presentation and the initial encounter request. It does not authoritatively roll dice, validate commands, apply damage, or mutate battle state.

## Battle Initialization

Godot requests a battle by identifying the participants.

Conceptually:

```yaml
player:
  instance_id: player_1
  run_state_id: current_run_player

enemies:
  - instance_id: goblin_1
    definition_id: goblin
  - instance_id: goblin_2
    definition_id: goblin
```

The distinction is:

```text
goblin       = reusable content definition
goblin_1     = mutable actor instance in this battle
goblin_2     = another mutable actor instance from the same definition
```

The client chooses who participates. Content and save data define their actual starting state.

### Player State

A new run begins from an authored starting character definition.

After the run begins, the player is loaded from mutable run state so changes carry between battles, including:

- current card zones and permanently removed cards
- acquired or removed cards
- current resources
- abilities and dice loadout
- statuses or injuries that persist between battles
- player preferences such as automatic status-effect rolls

### Enemy State

Enemies normally begin each battle as fresh instances loaded from immutable enemy definitions.

Two goblins in separate battles do not normally share mutable health or card state. A future encounter may explicitly opt into persistent enemy state.

## Authoritative Battle State

Battle state is the durable, authoritative condition required to resume the battle between commands or after a process restart.

It must include at least:

- battle ID
- round
- current segment and phase
- segment-flow stage and iteration
- participant actor IDs
- controller type per actor: `human`, `ai`, or `system`
- each actor's character/enemy definition ID
- card zones
- dice loadout and current dice state
- abilities
- resources
- statuses, tokens, and effect instances
- hidden planning commitments
- pending attacks and effect proposals
- active nested resolutions
- active interaction/reaction chain
- pending input by actor
- battle-result state
- deterministic ordering and random-result information needed for recovery

State must use actor collections keyed by stable actor ID. It must not hard-code one `player` field and one `enemy` field.

## Commands, Events, Snapshots, and Saves

### Commands

Commands are actor requests:

```text
roll dice
reroll selected dice
play card
select ability
select target
commit actions
pass
```

Every command is validated against:

- battle ID
- actor ID and controller
- current segment
- current phase
- nested resolution/window ID
- stage and iteration
- allowed command types
- whether the actor is still eligible to act

### Events

Events are authoritative facts:

```text
dice_rolled
commitment_revealed
card_played
damage_proposed
status_applied
cards_permanently_removed
segment_advanced
battle_completed
```

The battle keeps an append-only authoritative event history. Hidden facts may exist in the authoritative history before they become visible to a particular viewer.

Random outcomes must be recorded in events so replay does not reroll them.

Content schema/version information must eventually be recorded or pinned so later content edits do not change the meaning of an old replay.

### Snapshots

Snapshots describe current viewer-safe state. They do not replace events.

The snapshot builder uses authoritative state and event visibility rules to avoid exposing:

- enemy hand card IDs
- hidden deck order
- hidden planning dice
- unrevealed abilities, cards, and targets
- hidden reaction commitments

### Engine Response

The engine advances through automatic work until it reaches the next human decision point.

It returns:

```text
viewer-visible events since the previous command
+ viewer-safe current snapshot
+ current pending input
```

Godot uses the response events to animate everything that occurred since its previous command.

### Save and Recovery

The game should save after every committed resolution batch and accepted command boundary.

The durable save should contain:

- a battle-state checkpoint
- the authoritative event history or durable event-log reference
- enough random/content metadata to resume and replay deterministically

A crash must not require restarting the battle.

## YAML-Driven Reusable Mechanics

YAML should configure reusable Go operations rather than merely naming a hard-coded status handler.

Example poison:

```yaml
id: poison
stack_limit: 3

trigger:
  segment: ongoing_effects
  phase: on_enter
  priority: 0

resolution:
  type: roll_outcomes
  dice:
    count_per_stack: 1
    sides: 6

  outcomes:
    - faces: [1, 2, 3, 4]
      effects:
        - type: deal_damage
          amount: 1

    - faces: [5, 6]
      effects:
        - type: remove_status_stack
          amount: 1
```

Advanced poison can reuse the same operations:

```yaml
id: advanced_poison
stack_limit: 3

trigger:
  segment: ongoing_effects
  phase: on_enter
  priority: 0

resolution:
  type: roll_outcomes
  dice:
    count_per_stack: 1
    sides: 6

  outcomes:
    - faces: [1, 2, 3, 4, 5]
      effects:
        - type: deal_damage
          amount: 2

    - faces: [6]
      effects:
        - type: remove_status_stack
          amount: 1
```

Poison and advanced poison are separate status definitions, but their rolls participate in the same simultaneous ongoing-effects batch.

Reusable Go operations should include concepts such as:

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
```

When new content needs behavior that existing operations cannot express, first determine whether the missing behavior is a reusable operation. Use a specialized Go handler only when the mechanic is genuinely unique or a general operation would make the content language unsafe or excessively complex.

YAML is declarative content, not an unrestricted programming language.

## Interaction Windows

Use one generic persisted window system for all actor input during nested resolutions.

A window includes:

```text
window ID
source
purpose
segment and phase
suspended resolution checkpoint
eligible actors
required actors
allowed commands
hidden/public commitment policy
reveal policy
pass policy
actor commitments
reaction-chain depth and round
```

Useful purposes include:

```text
required_roll
planning
reaction
choose_card
select_target
```

These are purposes of one system, not separate engines.

The distinction matters because:

- a required roll may not be passed
- an optional reaction may be passed
- different windows allow different commands
- only selected actors may be eligible
- automatic preferences may satisfy some required-roll windows

## Simultaneous Hidden Commitment Model

The default for every segment phase and interaction window is:

```text
collect hidden commitments from all required actors
-> AI actors commit automatically
-> wait for required human actors
-> reveal the completed batch simultaneously
-> resolve compatible commitments
-> open another reaction round when required
```

Actors may commit multiple cards or actions during one window when the rules allow it.

An actor that passes one reaction round may react again in a later round after new actions are revealed.

The model may be overridden by explicit content, such as an `Anticipate Reaction` effect that delays an actor's opportunity until after the current commitments reveal.

## Resolution Batches

Gameplay consequences are proposed and committed in batches.

```text
collect proposals
-> reveal proposals
-> reaction chain
-> finalize proposals
-> commit batch atomically
-> discover immediate consequences
-> create the next batch if needed
```

Equivalent consequences accumulate.

Example:

```text
three poison rolls each produce 1 damage
-> proposed poison damage = 3
-> reveal three proposed damage cards together
```

Different proposal types can share one reaction window:

```text
- take 3 damage
- apply poison
- remove a resource
- remove another status
```

Reactions may target individual proposals even though the batch is revealed together.

Examples:

- change one of three poison dice
- cancel the proposed poison application
- prevent two points from three accumulated damage
- replace one of three proposed damage cards

## Reaction Chains

After a batch is revealed:

```text
everyone secretly commits reactions or passes
-> all commitments reveal together
-> compatible reactions resolve
-> another reaction round opens
-> repeat until everyone passes in the same round
-> commit the suspended batch
```

A reaction may itself be reacted to. The chain moves forward through new windows and does not reopen completed collection windows.

### Simultaneous Conflicts

The initial default for directly incompatible simultaneous reactions is mutual negation.

Example:

```text
reaction A: set die 2 to 1
reaction B: set die 2 to 6
result: both replacement effects are negated
```

Initial rules:

- incompatible replacements of the same value negate each other
- identical compatible replacements do not conflict by default
- additive effects stack by default
- unrelated effects resolve normally
- played cards remain spent when their effects are collision-negated
- paid costs are not refunded
- a negated effect does not create downstream triggers
- explicit content may override the default conflict behavior

The conflict matrix will expand as real card mechanics require it.

## Effect Ordering

When multiple effects trigger at the same checkpoint, the initial deterministic ordering is:

```text
1. explicit content priority, higher priority first
2. effect-instance creation order
3. stable effect-instance ID
```

Most content should use priority `0`.

Ordering controls deterministic proposal generation. It must not turn simultaneous commitment into first-actor-wins resolution.

This policy is intentionally adjustable as more content is created.

## Ongoing Effects Segment

`ongoing_effects` begins every round.

### `on_enter`

1. Gather every effect that triggers at `ongoing_effects/on_enter`.
2. Order triggers deterministically.
3. Group compatible simultaneous work.
4. Resolve required automatic or actor-driven rolls.
5. Reveal outcomes.
6. Run reaction chains.
7. Build and commit resulting consequence batches.
8. Continue until no `on_enter` effect remains unresolved.

### Poison Example

If an actor has three poison stacks:

1. Create three required D6 rolls, one per stack.
2. Roll all three simultaneously.
3. Reveal every final die face number and symbols.
4. Open one reaction chain in which any eligible actor can affect any individual poison roll.
5. Finalize all three rolls after everyone passes.
6. Evaluate each poison definition independently.
7. Accumulate resulting damage.
8. Select all proposed damage cards together.
9. Reveal the full proposed damage and all cards together.
10. Open a new reaction chain for the accumulated damage batch.
11. Commit the accepted damage and status removals together.

Poison and advanced poison use their own YAML parameters but participate in the same simultaneous roll batch.

### Immediate Status Consequences

Applying or removing a status may create another immediate resolution.

For Baryl:

```text
attempt to apply another Baryl while one is active
-> existing Baryl resolves completely
-> all of its reaction chains complete
-> new Baryl is applied
```

This is a Baryl-specific stack-overflow policy.

The default status policy is:

```text
when stack limit is reached, additional stacks cannot be applied
```

### `in_progress`

Continue any persisted ongoing-effect resolution that did not finish during the previous engine call because human input was required.

### `on_exit`

Run effects explicitly configured for `ongoing_effects/on_exit`, complete cleanup, and only then evaluate whether the battle has ended.

## Income Segment

### `on_enter`

Perform income work and triggers, including:

- draw cards
- gain energy or other resources
- resolve income-specific statuses and passives
- open interaction/reaction windows when required

Equivalent work for all actors should be calculated from the same pre-commit state and committed as a synchronized batch where applicable.

### `in_progress`

Resume unresolved income choices, reactions, or nested effects.

### `on_exit`

Run income exit effects and cleanup. Evaluate battle completion after all income work finishes.

## Offensive Segment

### `on_enter`

1. Initialize offensive planning state for every participating actor.
2. Determine available dice, rolls, abilities, cards, resources, and targets.
3. Create hidden planning state.
4. Progress AI-controlled actors automatically.
5. Request human offensive decisions.

### Hidden Planning

Each actor may privately:

- roll offensive dice
- keep dice
- use available rerolls
- play allowed cards
- select targets
- select a valid offensive ability
- pass
- commit multiple allowed actions
- lock in

Actors do not see another actor's:

- dice faces or symbols
- roll count
- kept dice
- cards
- ability
- targets

### Reveal

When all required actors lock in, reveal each actor's final offensive commitment.

Every final die must include:

```text
die ID or index
face number
symbols on that face
```

Example:

```text
Player:
- final dice:
  - die 1: face 4, symbols [Sword]
  - die 2: face 4, symbols [Sword]
  - die 3: face 2, symbols [Bow]
- rolls used: 2
- uses Slash
- targets Goblin 1
- proposes 3 damage

Goblin 1:
- final dice:
  - die 1: face 5, symbols [Dagger]
  - die 2: face 3, symbols [Poison]
  - die 3: face 5, symbols [Dagger]
- rolls used: 3
- uses Poisoned Stab
- targets Player
- proposes 2 damage
- proposes 1 poison

Goblin 2:
- final dice:
  - die 1: face 2, symbols [Shield]
  - die 2: face 1, symbols []
  - die 3: face 1, symbols []
- rolls used: 2
- passes
```

Do not reveal:

- intermediate roll history
- keep/reroll history
- when an actor considered a card or ability

### Offensive Reaction and Revalidation

1. Open a reaction chain for the revealed offensive batch.
2. Resolve reactions.
3. Revalidate every commitment.
4. Reopen hidden planning only for materially affected actors.
5. Keep unaffected actors locked in.
6. Reveal only changed commitments when reopened actors lock again.
7. Repeat until every offensive commitment is valid and accepted.

An actor is materially affected when its:

- dice
- selected ability
- target
- cost
- available rolls
- available legal actions
- controlling effects

change enough to require a new decision.

### Offensive `on_exit`: Blind

Blind is an offensive exit effect.

It occurs only after normal offensive planning, reveal, reactions, and revalidation have completed.

For every actor with Blind:

```text
if actor passed or selected no offensive ability:
    expire Blind without rolling

if actor selected an offensive ability:
    create required Blind roll
```

All applicable Blind rolls happen simultaneously.

1. Roll each required Blind die.
2. Reveal final face numbers and symbols.
3. Open a reaction chain for the Blind results.
4. Finalize the rolls after everyone passes.
5. On the normal Blind result of 1-2, cancel that actor's selected offensive ability.
6. Otherwise preserve the ability.
7. Expire Blind regardless of result.

When Blind cancels an offensive ability, its complete proposal is cancelled, including damage, status applications, and targeted effects. The actor does not return to offensive planning unless a future Blind definition explicitly says otherwise.

Whether all paid ability costs remain spent should be defined by the Blind/content rules; the default should be explicit rather than inferred.

After exit-triggered effects finish, persist the finalized offensive attacks and effect proposals for defense and damage resolution.

Do not apply attack damage or attack-delivered statuses during offensive.

## Defensive Segment

Defensive uses the same hidden planning, reveal, reaction, and revalidation machinery as offensive.

It runs when at least one finalized offensive proposal creates something defensible.

### `on_enter`

1. Load finalized incoming attacks and effects.
2. Show each actor the incoming public information relevant to them.
3. Determine eligible defenders.
4. Mark actors with no defensive participation as `not_participating`.
5. Initialize hidden defensive planning.

### Hidden Planning

Eligible actors may privately:

- roll defensive dice
- keep dice and reroll
- play allowed cards
- select defensive abilities
- select incoming attacks/effects as targets
- reduce or prevent damage
- redirect attacks or effects
- create counter-damage or counter-effects
- pass
- commit multiple actions
- lock in

### Reveal, Reactions, and Revalidation

Reveal:

- final defensive die face numbers and symbols
- rolls used
- selected defensive ability or pass
- selected targets
- committed cards
- proposed defensive effects

Then:

```text
reaction chain
-> revalidation
-> reopen only materially affected defenders
-> reveal changed commitments
-> repeat until valid
```

Defensive reaction windows are as extensive as offensive reaction windows. Dice manipulation, card play, target changes, cancellation, and nested reactions all use the same generic systems.

### `on_exit`

1. Attach finalized defensive modifiers to their targeted proposals.
2. Add counterattacks and counter-effects to pending resolution.
3. Preserve source attribution.
4. Do not remove health cards.
5. Do not apply final attack consequences.
6. Evaluate battle completion only after every defensive exit effect finishes.

## Damage Resolution Segment

Damage resolution applies the accepted consequences created by offensive and defensive.

### `on_enter`: Calculate Final Proposals

For every pending source:

1. Start with its base effects.
2. Apply accepted defensive modifiers.
3. Apply increases, reductions, prevention, redirection, and target changes.
4. Calculate final damage per target.
5. Preserve source attribution for rules that care about a particular source.
6. Accumulate equivalent consequences for presentation and broad prevention.

Example:

```text
Player:
- take 3 total damage from two accepted sources
- gain 1 poison

Goblin 1:
- take 4 total damage
```

### Select and Reveal Damage Cards

For each actor taking damage:

1. Select every proposed damage card together.
2. Reveal all selected cards together.
3. Do not permanently remove them yet.

The reaction window includes all proposals currently present:

```text
damage totals
revealed damage cards
status applications
status removals
resource changes
counter-effects
other accepted ability effects
```

Actors may target individual proposals while also using effects that operate on accumulated totals.

Examples:

- prevent two of three total damage
- replace one selected damage card
- cancel poison without changing damage
- redirect one source

### Reaction Chain

Run the standard hidden simultaneous reaction chain until everyone passes in the same round.

### Commit

After the chain closes:

1. Apply final damage.
2. Move accepted damage cards to the permanently removed zone.
3. Apply statuses.
4. Apply resource and other state changes.
5. Resolve all immediate consequences caused by the commit.
6. Create and finish additional batches when required.

All nested effects must finish even when an actor has reached zero health during the segment.

### `in_progress`

Resume any damage proposal, reaction chain, or immediate consequence that paused for human input.

### `on_exit`

Only after every pending attack, status application, counter-effect, immediate trigger, and nested reaction chain has completed:

1. Run damage-resolution exit effects.
2. Clear temporary attack and resolution state.
3. Evaluate battle completion.
4. If battle continues, advance to the next round's `ongoing_effects`.

## Card-as-Health Rules

One card equals one health by default.

Current health is:

```text
draw pile count
+ discard pile count
+ hand count
```

Permanently removed cards are missing health.

### Default Damage Selection

For the full pending damage amount:

1. Select randomly from the combined draw and discard population.
2. Each individual card has equal probability regardless of which of those two zones contains it.
3. Use hand cards only after both draw and discard are empty.
4. Reveal all selected cards together.
5. Move accepted cards to permanently removed only after reactions finish.

Example:

```text
draw: 20 cards
discard: 10 cards

each draw card has a 1/30 chance
each discard card has a 1/30 chance
```

Special damage effects may override selection, such as selecting from hand or allowing an actor to choose.

When draw, discard, and hand are all empty, the actor has zero health and is defeated when battle completion is evaluated.

## Battle Completion

Battle completion does not interrupt a segment immediately when an actor reaches zero health.

Every status, damage proposal, counter-effect, immediate trigger, and reaction chain already belonging to the current segment must finish. Battle completion is evaluated only after the segment reaches `on_exit` and completes its exit work.

This permits simultaneous or delayed consequences such as:

- both sides reaching zero health
- all enemies reaching zero while a pending Baryl resolution can still defeat the player
- a counter-effect completing after its source actor reached zero

Authoritative results include:

```text
victory
defeat
draw
escaped
```

Initial single-player interpretation:

```text
player alive and all enemies defeated -> victory
player defeated and enemies remain -> defeat
player and all enemies defeated -> authoritative draw, run defeat
successful player escape -> escaped
```

Future run rules may define enemy escape or partial encounter outcomes.

## Roll Automation Preferences

Go remains authoritative for every roll.

Default player behavior:

```text
status/effect-required rolls: automatic
offensive rolls: manual
defensive strategic rolls: manual
```

Preferences live in mutable player/run state and can be scoped by roll purpose or effect type.

Example:

```yaml
roll_preferences:
  status_effects: automatic
  offensive: manual
  defensive: manual
```

Automation submits the same authoritative action an actor would otherwise request manually. It does not bypass reveal, reaction, or validation rules.

## Shared Planning Machinery

Offensive and defensive should reuse a generic synchronized planning mechanism configured with:

```text
segment
eligible actors
available dice
available abilities
allowed cards
target rules
commitment rules
validation rules
exit triggers
```

Do not build unrelated offensive-only and defensive-only reaction systems.

The same lower-level mechanisms should also support:

- poison rolls
- Blind rolls
- status choices
- damage-card reactions
- future multiplayer participants

## Transition and Safety Rules

- `on_enter` executes exactly once per segment entry.
- Repeated engine calls while waiting do not duplicate automatic work.
- Nested resolution checkpoints are persisted.
- `on_exit` runs only after all required in-progress work is complete.
- Segment state advances only after successful exit work.
- Invalid or stale commands are rejected.
- AI work never causes an early return merely because the AI made a decision.
- Human input is the normal reason progression returns control.
- Automatic progression has configurable finite guards.
- Exceeding a guard is an explicit engine error, never an automatic pass.
- Resolution batches validate before commit where practical.
- Save checkpoints do not expose hidden information to snapshots.

## Intentionally Deferred Policies

These do not block the architecture and should be refined as real content requires them:

- the complete reaction conflict matrix
- content-specific cost refund behavior when abilities are cancelled
- special priority rules beyond the initial deterministic ordering
- all escape/run rules
- unusual status stack-overflow policies beyond Baryl
- multiplayer timeout and disconnection behavior
- content-version migration for old saves and replays

## Architecture Review Checklist

When reviewing the current code, determine whether:

- segment ordering is isolated from gameplay rules
- the engine owns one authoritative progression loop
- segment flows use only `on_enter`, `in_progress`, and `on_exit`
- nested resolutions can pause and resume
- all actors share synchronized segment/phase state
- hidden commitments and reveal gates are generic
- reaction chains are persisted and reusable
- offensive and defensive share planning/reaction machinery
- battle setup loads mutable player state and fresh enemy definitions
- YAML can configure reusable operations parametrically
- events can support both viewer delivery and full replay
- snapshots filter private information
- battles can save and resume between commands
- damage cards are proposed before permanent removal
- battle completion waits for segment completion
- current code contains content-specific or single-player assumptions that violate this model
