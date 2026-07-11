# Battle Turn Structure

> Status: Working design draft for discussion. This document describes the intended player experience and rules model. It is not a claim that every part is implemented.

## Purpose

Define the shape, timing, and responsibilities of a battle round in Dice and Destiny. This document should make it possible to answer:

- where the battle currently is
- what happens automatically
- what decisions an actor can make
- when hidden information is revealed
- when reactions are allowed
- when proposed consequences become permanent
- when a segment, round, or battle is complete

The design should support one human player fighting one or more AI enemies now without preventing future simultaneous multiplayer.

## YAML-Driven Content Creation

Dice and Destiny content is built from reusable YAML definitions. Go implements and validates generic mechanics; YAML composes those mechanics into game content.

```text
Symbol catalog
└──> Dice definitions
     ├──> Offensive ability qualification requirements
     └──> Dice rolls declared by statuses, cards, and Defensive abilities

Status definitions
├──> Card effects may apply or remove statuses
└──> Ability effects may apply or remove statuses

Card definitions ───────────────┐
Offensive ability definitions ──┼──> Player or Enemy definition
Defensive ability definitions ──┤
Dice definitions ────────────────┤
Starting status references ─────┘
```

The content hierarchy is:

1. Define reusable dice, statuses, cards, and abilities in separate YAML files.
2. Load the central symbol catalog, then load every definition into one validated content library keyed by stable definition ID.
3. Define a player character or enemy by referencing those IDs in its decklist, dice loadout, ability lists, and starting statuses.
4. At battle setup, resolve the references into an immutable runtime content catalog pinned to that battle.
5. Create mutable runtime instances—cards in zones, status stacks, dice results, energy, and used abilities—without modifying the reusable definitions.

### Content identity and file rules

- One YAML file defines one reusable content item.
- Every definition has a `schema_version`, stable `id`, and player-facing `name`.
- Cross-content references use the stable `id`, never the display name or filename.
- IDs are unique within their content type and should use lowercase snake case, such as `standard_d6`, `poison`, or `sword_cut`.
- YAML parsing rejects unknown fields.
- Validation rejects duplicate IDs, missing references, invalid targets, illegal timing, contradictory lifecycle flags, and unsupported operation types.
- Runtime gameplay never branches on a content ID such as `if status == poison`.
- New behavior should first be expressed by composing registered generic operations. New Go code is added only when a genuinely reusable mechanic is missing.

## Symbol Catalog

Symbols are stable content IDs shared by dice faces, ability requirements, presentation, and future rules. They are defined once in a central catalog so dice and abilities cannot silently disagree because of spelling or missing artwork.

The target catalog is conceptually:

```yaml
schema_version: 1

symbols:
  - id: sword
    name: Sword
    presentation_key: symbol_sword

  - id: shield
    name: Shield
    presentation_key: symbol_shield

  - id: gold_coin
    name: Gold Coin
    presentation_key: symbol_gold_coin
```

Symbol validation requires:

- unique lowercase snake-case IDs
- a non-empty display name
- a valid presentation key or other agreed UI asset reference
- every dice-face `symbol` to reference a catalog entry
- every symbol-based ability requirement to reference a catalog entry

Go mechanics compare symbol IDs. Godot maps the presentation key to the correct icon and display treatment. Neither side should infer a symbol from display text.

## Definition Versus Instance

A definition describes reusable rules:

```text
poison status definition
standard_d6 definition
sword_cut ability definition
tip_it card definition
```

An instance is mutable battle state:

```text
two Poison stacks on player-1
die-3 currently showing face 5, Shield
one Tip It card in the player's discard pile
Sword Cut used during Round 3 Offensive
```

Definitions are shared and immutable. Instances have unique IDs, ownership, zones, stacks, duration state, and per-round usage state.

## Dice Creation

### Default combat dice

The default actor combat loadout is five identical six-sided dice:

```text
default dice count: 5
default die: standard_d6
```

Every die face has exactly:

- one numeric face number
- one symbol ID

The target `standard_d6.yaml` is conceptually:

```yaml
schema_version: 1
id: standard_d6
name: Standard D6
die_type: d6
side_count: 6

faces:
  - number: 1
    symbol: sword
  - number: 2
    symbol: sword
  - number: 3
    symbol: sword
  - number: 4
    symbol: shield
  - number: 5
    symbol: shield
  - number: 6
    symbol: gold_coin
```

### Dice validation

A dice definition is valid only when:

- `side_count` is positive and equals the number of authored faces
- face numbers are unique and cover `1..side_count`
- every face contains exactly one non-empty symbol ID
- the definition ID is unique
- any referenced symbol ID is valid for presentation and requirement matching

The five dice in the default loadout all reference the same definition. Each runtime die still has its own instance/index so effects can keep, reroll, replace, or modify one specific die.

### Dice ownership and reuse

Actor definitions specify their resolved dice loadout:

```yaml
dice_loadout:
  - dice_id: standard_d6
    count: 5
```

The creation workflow starts new players and enemies with five `standard_d6` dice and writes that resolved loadout explicitly into the actor YAML. Runtime loading should not depend on an invisible fallback. Authors may then replace some or all dice with other definitions.

Combat dice and effect dice are different uses of the same generic roll system:

- Offensive uses the actor's five-die combat loadout for ability qualification.
- Defensive abilities may declare their own dice, such as Basic Defense rolling 1D6.
- Statuses may declare effect dice, such as one Poison D6 per stack.
- Cards may declare effect dice when their resolution requires a roll.
- Ability/status/card effect dice do not consume Offensive rolls unless their content explicitly says so.

Dice dependencies differ by content type:

- An Offensive ability reads the actor's final combat dice and requires authored symbol counts, face numbers, or numeric/symbol combinations before it can be selected.
- A Defensive ability is selected first, then may reference a dice definition for its internal resolution.
- A status or card may reference a dice definition for an automatic or player-activated roll.

Every revealed die result contains its die identity or index, numeric face, and symbol. Numeric-only effect operations may ignore the symbol, but the underlying die result remains well-defined.

### Dice creation workflow

```text
choose stable ID and display name
-> choose side count
-> author one number and symbol per face
-> validate face coverage and symbols
-> add the YAML to the dice content directory
-> reference the dice ID from player/enemy loadouts or effect operations
-> load and compile into the runtime content library
```

## Status Creation

A status definition describes something that can be attached to an actor and later trigger automatically, be activated by its owner, or support both behaviors.

### Status activation modes

Statuses support three content-driven modes:

| Mode | Meaning | Example |
|---|---|---|
| Automatic | The engine evaluates declared triggers without the owner choosing to play it | Poison, Blind, Entangle |
| Player activated | The status sits on its owner until used in a legal window | Cleanse, a stored Prevent effect |
| Hybrid | The status has automatic triggers and separately activatable behavior | Future advanced statuses |

Positive or negative presentation is metadata. It does not determine timing. Timing comes from triggers and playable-window declarations.

### Status definition responsibilities

A status YAML can declare:

- stable identity and presentation metadata
- activation mode
- stack limit and overflow behavior
- duration behavior when applicable
- automatic segment, phase, and stage triggers
- legal segments/windows for player activation
- costs and targeting
- whether its resolution opens a reaction window
- lifecycle/consumption flags
- reusable operations and nested outcomes

The target shape is conceptually:

```yaml
schema_version: 1
id: example_status
name: Example Status
activation_mode: automatic
polarity: negative

stacking:
  stack_limit: 3
  overflow_policy: reject_additional_stacks

lifecycle:
  persistent: true
  consume_on_trigger_checkpoint: false
  consume_on_play: false
  remove_after_resolution: false
  remove_on_duration_zero: false

triggers:
  - id: example_on_enter
    segment: ongoing_effects
    phase: entry
    stage: collect_statuses
    priority: 0
    reaction_window:
      opens: true
      pass_required: true
    operations: []

playable_during: []
```

Field names may evolve during implementation, but these behaviors must remain explicit and validated.

### Automatic status example: Poison

Poison is persistent, stacks, and triggers automatically during Ongoing Effects entry. Its YAML composes generic roll, outcome, damage, and stack-removal operations:

```yaml
schema_version: 1
id: poison
name: Poison
activation_mode: automatic
polarity: negative

stacking:
  stack_limit: 3
  overflow_policy: reject_additional_stacks

lifecycle:
  persistent: true
  consume_on_trigger_checkpoint: false
  consume_on_play: false
  remove_after_resolution: false
  remove_on_duration_zero: false

triggers:
  - id: poison_on_enter
    segment: ongoing_effects
    phase: entry
    stage: collect_statuses
    priority: 0

    operations:
      - id: poison_rolls
        type: roll_dice
        target: self
        one_per_status_stack: true
        dice_id: standard_d6
        reaction_window:
          opens: true
          pass_required: true

        outcomes:
          - faces: [1, 2, 3, 4]
            operations:
              - type: deal_damage
                target: self
                amount: 1

          - faces: [5, 6]
            operations:
              - type: remove_status_stack
                target: self
                status_id: poison
                stack_count: 1
```

The segment code only asks for matching `ongoing_effects/entry` triggers. It does not know that Poison exists.

### Automatic one-use status examples

Blind and Entangle use different YAML flags and timing without special engine branches:

```text
Entangle
-> trigger: Offensive entry
-> operation: reduce maximum Offensive rolls by 1
-> reaction window: none
-> consume whenever trigger checkpoint is reached

Blind
-> trigger: Offensive exit after ability revalidation
-> if no selected ability: consume without rolling
-> otherwise roll declared Blind D6 and open its reaction window
-> cancel or preserve selected ability from final face
-> consume after the resolution completes
```

### Player-activated status example: Cleanse

Cleanse sits on its owner until the player activates it in a legal window. Playing it immediately consumes the status and performs its declared removal operation:

```yaml
schema_version: 1
id: cleanse
name: Cleanse
activation_mode: player_activated
polarity: positive

lifecycle:
  persistent: false
  consume_on_play: true
  consume_on_trigger_checkpoint: false
  remove_after_resolution: false
  remove_on_duration_zero: false

playable_during:
  - segment: defensive
    window_purpose: reaction

targeting:
  selector: one_status_on_self

reaction_window:
  opens: false

operations:
  - type: remove_status
    target: selected_status
```

Cleanse may be played inside an existing window without opening another nested reaction window. Its exact legal windows and whether it removes one stack or a complete status are declared by its YAML.

### Status creation workflow

```text
choose ID, name, polarity, and activation mode
-> define stacking and duration behavior
-> define automatic triggers and/or playable windows
-> define costs and legal targets
-> compose generic operations and outcomes
-> declare reaction-window behavior
-> declare lifecycle/consumption flags
-> validate all status and operation references
-> add the status ID to abilities, starting actor state, or other legal content
```

## Card, Ability, Player, And Enemy Composition

The same creation approach extends to the remaining content types.

### Cards

Every card instance is always exactly one point of health. This is a core game invariant, not an optional field on individual card definitions.

```text
current health = cards in deck + hand + discard
lost health = cards in permanently removed
maximum health = total cards in the actor's starting decklist
```

Moving a card between deck, hand, and discard does not change health. Playing a card normally moves it from hand to discard, so it remains health. Damage moves a card from an active zone into the permanently removed zone, reducing health by one.

All card types, effects, costs, rarities, and owners follow the same one-card/one-health rule. Players and enemies use the same card definitions, instances, zones, and damage-removal rules.

Each card has its own YAML definition containing:

- stable identity and presentation metadata
- card type/tags
- immediate play costs
- legal source zones
- legal segment, phase, stage, and interaction-window timing
- target rules
- default destination when played
- whether its own resolution opens a reaction window
- generic operations, outcomes, and reusable content references

The target card shape is conceptually:

```yaml
schema_version: 1
id: example_card
name: Example Card
type: reaction
tags: []

presentation:
  rules_text: Example rules text.
  art_key: card_example

cost:
  energy: 1

play:
  source_zones: [hand]
  destination: discard
  playable_during:
    - segment: offensive
      phase: main
      window_purpose: reaction

targeting:
  selector: selected_die
  minimum: 1
  maximum: 1

reaction_window:
  opens: false

operations:
  - type: modify_die
    target: selected_die
    modification: set_face
    face: 5
```

### Card payment and movement

When a play command is accepted:

```text
validate timing, owner, target, and cost
-> spend energy and other costs immediately
-> move the card to its declared destination immediately
-> record its hidden commitment/effect
-> reveal it at the current window's shared reveal point
```

The default destination is discard. Content may explicitly override it with return to hand, permanent removal, or another supported destination. Collision-negated, canceled, and prevented card effects do not return the card or refund its costs unless content explicitly creates that refund.

### Card timing

A card is legal only when at least one `playable_during` rule matches the authoritative checkpoint and active window. Timing rules can restrict:

- segment
- Entry, Main, or Exit phase
- segment-specific stage
- window purpose, such as planning, reaction, damage response, or status response
- source or proposal types
- owner/target conditions

Cards played inside an existing interaction window do not automatically open another reaction window. The card's own `reaction_window` declaration controls that behavior.

### Card content references

Cards may reference:

- status IDs to apply, remove, or modify statuses
- dice IDs for card-owned rolls
- symbol IDs for die/symbol interactions
- generic target and proposal IDs supplied by the active window
- registered operation types

These references are resolved and validated when the content library loads. A card never duplicates the referenced status or dice definition.

Cards may apply, remove, or otherwise interact with reusable status definitions. These references use status IDs and are validated when the content library loads. A card never copies the status's trigger logic into itself.

```yaml
id: poison_dart
name: Poison Dart

effects:
  - type: apply_status
    target: selected_targets
    status_id: poison
    stack_count: 1
```

```yaml
id: antidote
name: Antidote

effects:
  - type: remove_status
    target: selected_status
```

```yaml
decklist:
  - card_id: tip_it
    count: 2
  - card_id: loaded_die
    count: 2
```

Each decklist count creates that many distinct runtime card instances. Four copies of one definition contribute four maximum health and can be independently drawn, discarded, played, or permanently removed.

### Card validation

A card definition is valid only when:

- its stable ID is unique
- costs are non-negative
- at least one legal play timing exists
- source and destination zones are supported and compatible
- target selectors match the operations that use them
- all referenced statuses, dice, symbols, and operations exist
- nested roll outcomes are complete and non-overlapping where required
- no field attempts to change the one-card/one-health invariant

### Card creation workflow

```text
choose ID, name, type, and presentation
-> define costs and default destination
-> define legal play timings/windows
-> define targeting
-> compose operations and reusable references
-> declare reaction behavior
-> validate the YAML against the complete content library
-> add card ID/count entries to player or enemy decklists
```

### Abilities

Abilities are separate board content and never count as health. An actor definition references its Offensive and Defensive abilities, and the UI presents the appropriate board during that segment.

```text
Offensive segment -> show actor's Offensive ability board
Defensive segment -> show actor's Defensive ability board
```

Available abilities are public board information. Hidden planning conceals the selected ability until its reveal; it does not conceal which abilities the actor owns.

Every ability YAML declares:

- stable identity, type, and presentation
- costs and usage limits
- targeting
- qualification or selection rules
- complete operations and conditional bonuses
- reaction behavior
- any nested dice/choice resolutions
- supported battle-duration upgrade behavior

### Offensive ability creation

Offensive abilities are earned by the actor's final accepted combat dice. A definition may inspect:

- symbol counts
- exact or ranged face counts
- pairs, three-of-a-kind, four-of-a-kind, and five-of-a-kind
- straights and other numeric patterns
- combinations of symbol and number requirements
- future registered requirement types

An Offensive ability separates activation tiers from independent bonus conditions:

- **Activation tiers:** determine whether that individual ability is selectable and which base results it can produce.
- **Conditional bonuses:** add operations when their own conditions match. Every compatible matching bonus applies unless an exclusive group says otherwise.

There is no ranking across different player abilities. The engine evaluates the final dice against every Offensive ability on the player's board and presents every qualified ability. The player may select any one of them, including a lower-damage ability whose status, resource, draw, targeting, or other effects better fit the situation.

```text
evaluate final dice against every board ability
-> collect all qualified abilities
-> show all legal choices to the player
-> player chooses one ability and required targets
-> validate and lock in that exact choice
```

Enemy selection remains controller-driven through its authored D100 chart and automatic post-reaction validity rules.

The target Sword Cut shape is conceptually:

```yaml
schema_version: 1
id: sword_cut
name: Sword Cut
type: offensive

presentation:
  rules_text: Deal damage based on Swords. Three-of-a-kind also applies Bleed.
  art_key: ability_sword_cut

cost:
  energy: 0

usage:
  maximum_per_segment: 1

targeting:
  selector: one_enemy
  minimum: 1
  maximum: 1

qualification:
  activation_tiers:
    - id: three_swords
      requirements:
        all:
          - type: symbol_count
            symbol_id: sword
            exact: 3
      operations:
        - type: deal_damage
          target: selected_targets
          amount: 5

    - id: four_swords
      requirements:
        all:
          - type: symbol_count
            symbol_id: sword
            exact: 4
      operations:
        - type: deal_damage
          target: selected_targets
          amount: 6

    - id: five_swords
      requirements:
        all:
          - type: symbol_count
            symbol_id: sword
            exact: 5
      operations:
        - type: deal_damage
          target: selected_targets
          amount: 7

  conditional_bonuses:
    - id: three_of_a_kind_bleed
      requirements:
        all:
          - type: number_pattern
            pattern: three_of_a_kind
      operations:
        - type: apply_status
          target: selected_targets
          status_id: bleed
          stack_count: 1
```

The same final dice are evaluated against the chosen ability's activation tiers and every conditional bonus. Selecting Sword Cut does not prevent the player from having selected another separately qualified board ability instead.

Example without Bleed:

```text
final dice: 1, 1, 2, 3, 3
symbols: Sword, Sword, Sword, Sword, Sword

five-Sword tier: yes -> deal 7 damage
three-of-a-kind: no -> do not apply Bleed
```

Example with Bleed:

```text
final dice: 1, 1, 1, 2, 3
symbols: Sword, Sword, Sword, Sword, Sword

five-Sword tier: yes -> deal 7 damage
three-of-a-kind: yes -> also apply 1 Bleed
```

An ability's tiers and bonuses may perform any registered legal operations, including:

- deal, prevent, or modify damage
- apply, remove, or modify statuses
- gain, spend, or remove energy/resources
- draw or move cards
- select, reveal, replace, or remove cards when the timing permits
- change targets
- roll or modify dice
- create counters or other new proposal sources

Tiers are not limited to scaling one number. Each tier owns its complete operation list.

### Offensive ability status references

Abilities reference status definitions by stable ID:

```yaml
id: venom_strike
type: offensive

qualification:
  activation_tiers:
    - id: venom_strike_base
      requirements:
        all:
          - type: symbol_count
            symbol_id: sword
            minimum: 2
          - type: symbol_count
            symbol_id: gold_coin
            minimum: 1
      operations:
        - type: deal_damage
          target: selected_targets
          amount: 3
        - type: apply_status
          target: selected_targets
          status_id: poison
          stack_count: 2
```

Venom Strike does not reimplement Poison. It proposes attaching the reusable `poison` definition to its target.

### Defensive ability creation

Defensive abilities are available from the actor's Defensive board without combat-dice qualification. Their YAML declares what they may target and what happens after selection.

Basic Defense is conceptually:

```yaml
schema_version: 1
id: basic_defense
name: Basic Defense
type: defensive

cost:
  energy: 0

usage:
  maximum_per_segment: 1

selection:
  requires_incoming_proposal: true
  allowed_proposal_types: [damage_source]
  target_count: 1

resolution:
  roll:
    dice_id: standard_d6
    dice_count: 1

  reaction_window:
    opens: true
    pass_required: true

  operations:
    - type: prevent_damage
      target: selected_proposal
      amount: rolled_face
```

The ability is selected first. Its internal 1D6 resolution then determines prevention. Another Defensive ability may have no roll, may target every incoming source, may redirect an attack, or may create a separate counterattack proposal.

Defensive usage rules can declare:

- once per incoming proposal
- once per Defensive segment
- multiple uses while costs can be paid
- one selection affecting multiple sources
- mutually exclusive ability groups

### Ability payment and usage

- An ability does not leave the board when used.
- When a complete legal selection is accepted, its costs and limited uses are consumed immediately.
- The runtime ability instance records segment/round/battle usage markers.
- The default actor selects one Offensive ability per Offensive segment.
- Defensive abilities follow their own authored usage limits and may allow multiple selections.
- Canceling an ability does not refund its costs or use unless content explicitly says so.

### Battle-duration ability upgrades

A card may upgrade an ability on its owner's board for the remainder of the current battle. The card remains a normal one-health card and follows normal payment/destination rules. The upgrade changes only the runtime ability instance.

```text
play upgrade card
-> pay and move card immediately
-> select a legal board ability
-> apply a persisted battle-duration modifier
-> events/snapshots show the upgraded board ability
-> base ability YAML remains unchanged
-> clear modifier when battle ends
```

Conceptually:

```yaml
id: sharpen_blade
name: Sharpen Blade
type: ability_upgrade

play:
  source_zones: [hand]
  destination: discard

targeting:
  selector: one_owned_offensive_ability

operations:
  - type: apply_ability_modifier
    target: selected_ability
    duration: battle
    modifier:
      add_conditional_bonus:
        id: sharpened_bleed
        requirements:
          all:
            - type: number_pattern
              pattern: exact_pair
        operations:
          - type: apply_status
            target: selected_targets
            status_id: bleed
            stack_count: 1
```

Ability modifiers may be designed to add or replace operations, adjust costs or usage, alter requirements, or add bonus conditions. Every supported modifier type must be explicit and validated; cards cannot inject unrestricted scripts.

### Ability validation

All abilities require:

- a unique ID and valid `offensive` or `defensive` type
- valid costs, usage rules, and target selectors
- references to existing symbols, dice, statuses, and operations
- deterministic, non-ambiguous activation tier matching or an explicit authored tier-selection policy
- valid requirement combinations
- complete nested roll outcomes where applicable
- compatible upgrade modifier types
- no health value or card-zone behavior

### Ability creation workflow

```text
choose ID, name, type, and presentation
-> define costs, usage, and targeting
-> for Offensive: define non-ambiguous activation tiers and independent bonuses
-> for Defensive: define selection rules and optional nested resolution
-> compose operations and reusable status/dice references
-> declare reaction behavior
-> declare supported upgrade modifiers when needed
-> validate against the complete content library
-> add the ability ID to a player or enemy board list
```

### Players and enemies

A player character and an enemy share one base combatant definition schema. The enemy adds an AI block; it does not use a separate representation for cards, dice, abilities, statuses, resources, or health.

The shared base declares:

- metadata and class
- starting resources and Income behavior
- decklist of card IDs and counts
- dice loadout of dice IDs and counts
- Offensive ability IDs
- Defensive ability IDs
- starting status instances referencing status IDs
- tokens and other actor-specific state
- roll/automation preferences

The target shared base is conceptually:

```yaml
schema_version: 1
id: example_combatant
name: Example Combatant
class: fighter

resources:
  starting_hand_size: 4
  starting_energy: 2
  hand_limit: 6

income:
  cards: 1
  energy: 1

decklist:
  - card_id: tip_it
    count: 2
  - card_id: basic_strike
    count: 8

dice_loadout:
  - dice_id: standard_d6
    count: 5

ability_board:
  offensive:
    - sword_cut
    - golden_edge

  defensive:
    - basic_defense
    - protect

starting_statuses: []
starting_tokens: []

roll_preferences:
  status_effects: automatic
  offensive: manual
```

Maximum health is not independently authored. It is derived from the number of card instances created by the resolved decklist.

### Enemy definition

An enemy definition embeds the same base fields and adds controller behavior:

```yaml
schema_version: 1
id: goblin_raider
name: Goblin Raider
class: goblin

# Shared combatant fields: resources, income, decklist, dice_loadout,
# ability_board, starting_statuses, starting_tokens, roll_preferences.

ai:
  offensive_planning:
    charts:
      3_rolls:
        abilities:
          - ability_id: jagged_slash
            activation_ranges:
              first_roll: [1, 5]
              second_roll: [6, 10]
              third_roll: [11, 15]
        no_ability_ranges:
          - [78, 100]

  defensive_selection:
    controller: basic
```

Enemy creation:

```text
load enemy definition
-> resolve shared content references
-> create a unique enemy actor instance
-> create distinct runtime card/status/dice/ability instances
-> apply the AI block
-> enter battle setup
```

Multiple enemies may use the same definition, but their instance IDs, card zones, statuses, random state, and decisions remain independent. Enemies start fresh from their definition for each encounter unless a future encounter explicitly persists them.

### Player definition and saved state layers

Player creation has three distinct layers:

```text
1. Character definition YAML
   reusable starting template

2. Player creation record
   immutable saved record of how this player instance began

3. Current run player state
   mutable saved record of how the player exists now
```

#### Character definition YAML

The character definition uses the shared combatant schema and describes the default starting deck, dice, Offensive board, Defensive board, resources, statuses, and tokens.

It is reusable content, not the mutable player save.

#### Player creation record

When a new player is created, the game resolves the character definition and saves an immutable creation record:

```yaml
schema_version: 1
player_id: player_001
character_definition_id: example_combatant
content_fingerprint: pinned-content-version

initial_state:
  decklist:
    - card_id: tip_it
      count: 2
    - card_id: basic_strike
      count: 8

  dice_loadout:
    - dice_id: standard_d6
      count: 5

  ability_board:
    offensive: [sword_cut, golden_edge]
    defensive: [basic_defense, protect]
```

This record preserves origin/provenance and makes it possible to compare the current player with how that player began. Ordinary run progression does not rewrite it.

#### Current run player state

The mutable current state records the player's actual present condition:

- current energy and other resources
- every card instance and its current zone
- acquired cards and permanently removed/missing-health cards
- current dice loadout and persistent dice changes
- current Offensive and Defensive board abilities
- persistent run-level ability upgrades
- current statuses, stacks, durations, and tokens
- roll/automation preferences
- progression metadata needed by the run

Conceptually:

```yaml
schema_version: 1
player_id: player_001
creation_record_id: player_001_origin
character_definition_id: example_combatant

resources:
  energy: 3

cards:
  deck:
    - instance_id: card_001
      definition_id: basic_strike
  hand:
    - instance_id: card_002
      definition_id: tip_it
  discard: []
  removed:
    - instance_id: card_003
      definition_id: basic_strike

dice_loadout:
  - dice_id: standard_d6
    count: 5

ability_board:
  offensive: [sword_cut, golden_edge]
  defensive: [basic_defense, protect]

persistent_ability_modifiers: []
statuses: []
tokens: []
```

Every card instance appears in exactly one zone. Because every card is one health, missing health is represented directly by instances in `removed`.

Acquiring a card creates another card instance, adds it to the current player state, and increases both current and maximum possible health by one. Permanently removing a card moves that instance to `removed`; it is not deleted from the save.

### Battle setup from current player state

```text
load immutable player creation record
-> load mutable current run player state
-> load referenced current content definitions
-> validate and migrate saved references when required
-> create battle actor state from the current player
-> instantiate fresh enemies from enemy definitions
-> pin the resolved content catalog to the battle
```

Battle-duration ability upgrades, temporary roll state, interaction windows, pending proposals, and segment usage markers belong to the battle checkpoint. They clear when the battle ends unless an explicit run-level reward or effect converts them into persistent state.

After battle, authorized persistent results update the current run player state. The immutable creation record and reusable character definition remain unchanged.

### Shared combatant validation

Player characters and enemies share validation for:

- resources and Income values
- non-empty one-card/one-health decklists
- unique resolved card instances
- valid dice definitions and positive counts
- default five-`standard_d6` creation output
- valid and non-duplicated Offensive/Defensive ability references
- valid starting status definitions and stack limits
- valid tokens and preferences
- total card-zone instances matching the saved current collection

Enemy-only validation additionally verifies complete non-overlapping D100 ranges, legal reveal profiles, and valid ability references. Player-save validation additionally verifies creation-record provenance, current zones, acquired content, missing health, and persistent run upgrades.

### Complete example content library

A self-contained target-schema example is stored at `docs/game-design/examples/content-library/`.

It includes:

- one symbol catalog
- one reusable Standard D6
- Poison, Bleed, and Entangle statuses
- six reusable cards
- ten Offensive/Defensive abilities
- the complete Blade Warden player template
- the complete Venom Goblin enemy definition
- complete one-, two-, and three-roll enemy D100 charts

These examples are design targets and are intentionally outside the production content directory until the current loaders support the target schema.

### Full-battle templates

The first deterministic full-battle template is stored at
`docs/game-design/examples/full-battles/blade_warden_vs_venom_goblin.md`.

It runs Blade Warden and Venom Goblin through four complete rounds and exercises
Income, Offensive qualification, enemy D100 planning, reaction-driven die
changes, selection-first defenses, status triggers, damage across all card
zones, overage damage, pending defeat, and battle completion at segment exit.

All random outcomes and scenario decisions are scripted so the walkthrough can
later become a deterministic integration fixture. Enemy Defensive choices are
also scripted for this rules fixture; choosing those defenses from an AI policy
is a separate behavior to specify and test.

### Current implementation versus target

The current Go content layer already:

- loads separate YAML files for dice, statuses, cards, abilities, characters, and enemies
- rejects unknown YAML fields
- validates unique IDs and cross-content references
- compiles reusable operation definitions
- validates character/enemy decklists, dice loadouts, abilities, and starting state
- derives card-based health from deck size

Important target gaps include:

- a central validated symbol catalog does not yet exist
- the current standard D6 has empty symbol lists instead of one required symbol per face
- the current dice schema allows symbol arrays rather than enforcing one symbol
- current effect-roll operations use raw side counts rather than referencing reusable dice definitions by `dice_id`
- the current mock actors author their loadouts manually; the target creation workflow should prepopulate and explicitly save the five-`standard_d6` default
- status activation modes, playable windows, lifecycle flags, stages, and trigger-level reaction policies are incomplete
- cards and abilities are mostly mock/no-op content
- card/status and ability/status reference validation currently covers only the operation forms already implemented and must expand with the target operations
- Offensive requirements do not yet express general symbol counts and tiered results
- Offensive requirements do not yet support independent numeric-pattern bonuses such as three-of-a-kind Bleed
- the current Defensive implementation still mirrors Offensive planning rather than selection-first ability resolution
- player/enemy YAML does not yet separate Offensive and Defensive ability lists
- battle-duration runtime ability upgrades and validated `apply_ability_modifier` operations do not yet exist
- enemy Offensive D100 charts are not yet part of enemy content
- character and enemy loaders duplicate overlapping fields instead of using one explicit shared combatant schema
- the current run-player save does not link to a separate immutable player-creation record
- saved card zones currently store card definition IDs without unique runtime card instance IDs

## Proposed Vocabulary

The game needs names for several nested levels of time. The proposed hierarchy is:

```text
battle
└── round
    └── segment
        └── phase/checkpoint
            └── stage
                └── interaction window
                    └── reaction round, when applicable
```

| Level | Meaning | Example |
|---|---|---|
| Battle | The complete encounter | Player versus two goblins |
| Round | One complete pass through all battle segments | Round 3 |
| Segment | A major rules purpose shared by all actors | Offensive |
| Phase/checkpoint | One of the three universal lifecycle positions inside a segment | Entry, main, exit |
| Stage | A segment-specific step that can pause and resume | Hidden planning, reveal, card selection |
| Interaction window | A bounded opportunity for eligible actors to act or pass | React to revealed damage cards |
| Reaction round | One simultaneous set of hidden reactions inside a reaction chain | Both sides react, reveal, then react again |

### Settled phase names

The current code calls the three universal checkpoints `on_enter`, `in_progress`, and `on_exit`. The older game called similar steps `pre_segment`, `segment_execution`, and `post_segment`.

The settled game-design names are:

| Design name | Code name | Purpose |
|---|---|---|
| Entry phase | `on_enter` | Establish the segment, gather triggers, and create its first work |
| Main phase | `in_progress` | Perform automatic work and player decisions until the segment is complete |
| Exit phase | `on_exit` | Resolve exit triggers, finalize outputs, and clean up |

The complete shared cycle is called a Round. "Turn structure" may still be used informally for the overall design, but the authoritative rules use Battle, Round, Segment, and Phase.

## Core Round Loop

The current V2 model contains five separate segments:

```text
Ongoing Effects
-> Income
-> Offensive
-> Defensive
-> Damage Resolution
-> next round's Ongoing Effects
```

Round 1 starts at Ongoing Effects. The round number increases only after Damage Resolution finishes.

All actors share the same round, segment, phase, and active nested resolution. Individual actors can be waiting, deciding, locked in, resolved, or not participating, but one actor does not independently advance to the next segment.

## Dice, Symbols, And Ability Qualification

Every rolled die preserves both its numeric face and every symbol printed on that face. Ability requirements may inspect face numbers, symbols, combinations of the two, or other declared roll properties.

The initial standard combat loadout uses five identical D6 dice with this definition:

| Face | Symbol |
|---:|---|
| 1 | Sword |
| 2 | Sword |
| 3 | Sword |
| 4 | Shield |
| 5 | Shield |
| 6 | Gold Coin |

For now, all five dice in an actor's standard loadout use the same face map. The model still preserves a die ID so future characters may replace individual dice with different definitions.

Only the final accepted roll qualifies the selected ability unless content explicitly refers to earlier rolls. A final roll may qualify multiple abilities, but the default actor selects only one.

Tiered requirements use their authored matching rules. Sword Cut uses mutually exclusive exact Sword counts:

```text
Sword Cut
3+ Swords -> deal 5 damage
4+ Swords -> deal 6 damage
5+ Swords -> deal 7 damage
```

The final roll:

```text
1 Sword, 1 Sword, 3 Sword, 3 Sword, 6 Gold Coin
```

contains four Sword symbols, so Sword Cut qualifies at its four-Sword tier and proposes six damage. The duplicate face numbers remain available to any ability that also checks for pairs or exact numeric combinations.

The five-die qualification pool belongs to Offensive. Defensive abilities are selected from the actor's available list without first being earned by a combat-dice result. A selected Defensive ability may create its own nested dice roll, choice, or other resolution as declared by that ability.

### Enemy Offensive D100 planning tables

AI-controlled enemies do not perform the player's literal roll, keep, and reroll loop. Each enemy definition instead provides prebaked D100 outcome charts keyed by segment and maximum available rolls.

When AI planning begins:

```text
apply roll-count modifiers
-> select the chart for the resulting maximum rolls
-> make one authoritative D100 roll
-> read the selected ability, simulated roll count, and reveal dice from that row
-> validate the resulting commitment
-> lock in or pass
```

A three-roll chart can express cumulative success by simulated roll:

| D100 | Result | Simulated state |
|---:|---|---|
| 1-5 | Ability 1 | Activated on first roll; 2 rerolls remained |
| 6-10 | Ability 1 | Activated on second roll; 1 reroll remained |
| 11-15 | Ability 1 | Activated on third roll; 0 rerolls remained |
| 16-18 | Ability 2 | Activated on first roll; 2 rerolls remained |
| 19-23 | Ability 2 | Activated on second roll; 1 reroll remained |
| remaining authored ranges | Abilities 2-4 | Definition-specific outcomes |
| 78-100 in the example shape | No ability | Enemy passes |

This means Ability 1 has a 5% cumulative chance after one roll, 10% after two, and 15% after three. The exact ranges are authored per enemy and difficulty.

Enemy YAML should state those ranges directly instead of requiring authors to calculate weights or utility scores. The intended shape resembles:

```yaml
ai_planning:
  offensive:
    charts:
      3_rolls:
        abilities:
          - ability_id: ability_1
            activation_ranges:
              first_roll: [1, 5]
              second_roll: [6, 10]
              third_roll: [11, 15]

          - ability_id: ability_2
            activation_ranges:
              first_roll: [16, 18]
              second_roll: [19, 23]
              third_roll: [24, 30]

          # Ability 3 and Ability 4 author the ranges from 31 through 77.

        no_ability_ranges:
          - [78, 100]
```

The table validator flattens these authored ranges, rejects overlaps and out-of-range values, and identifies any accidental uncovered values. Separately authored one-, two-, and three-roll charts make the effect of Entangle or other roll-count changes explicit.

Each successful row also contains or references a valid five-die reveal profile so the public commitment still shows concrete face numbers and symbols. The selected ability must validate against those revealed dice. The AI's internal D100 roll is authoritative and replayable but is not presented as a human-style dice sequence.

An Entangled enemy uses its separately authored two-roll chart, which omits third-roll success ranges and therefore has a larger miss range. Other roll-count changes select their matching prebaked chart. These activation charts are for enemy Offensive planning, where abilities are earned through simulated dice qualification. Enemy Defensive behavior is selection-first just like the player's and does not use an ability-activation D100 chart.

### Enemy ability recheck after dice changes

When a reaction changes any revealed enemy die, the engine re-evaluates every enemy ability against the complete modified dice pool and all other validity rules.

```text
re-evaluate all abilities
-> if original ability remains the only valid ability: keep it
-> if exactly one ability is valid: select it
-> if multiple abilities are valid: select one with an authoritative uniform random choice
-> if no ability is valid: enemy uses no ability
```

This is an automatic validity recheck, not another D100 planning roll and not a human-style replanning loop. Any random fallback selection is persisted and replayable.

Example:

```text
Original final dice: 6, 6, 6, 6, 6
Ultimate requires: five face-6 dice
Small Ultimate requires: four face-6 dice

Tip It changes one die to face 5.
Modified dice: 5, 6, 6, 6, 6

Ultimate becomes invalid.
Small Ultimate remains valid and is selected automatically.
```

If a reroll or die modification leaves both abilities valid, the engine randomly selects between the valid abilities. The enemy does nothing only when the modified state qualifies for no legal ability.

## Universal Segment Lifecycle

Every segment uses the same outer lifecycle:

```text
ENTRY
1. Mark the segment entered exactly once.
2. Gather entry triggers and determine participants.
3. Initialize persisted segment stages and automatic work.

MAIN
4. Perform all work that does not need human input.
5. If human input is needed, open a precise interaction window and wait.
6. After input, resume from the saved stage.
7. Continue through reveals, reactions, revalidation, and commits as required.
8. Declare the segment complete only when no required work remains.

EXIT
9. Resolve exit triggers and nested consequences.
10. Finalize the segment's outputs and clear temporary state.
11. Evaluate battle completion.
12. If the battle continues, advance to the next segment.
```

Entry and exit are not assumed to be instantaneous. Either may open nested resolutions and wait for input.

```mermaid
sequenceDiagram
    participant Engine as Battle Engine
    participant Flow as Current Segment
    participant Resolution as Nested Resolution
    participant Actor as Human or AI Actor
    participant State as Battle State

    Engine->>Flow: Enter segment once
    Flow->>State: Initialize phase and stages
    loop Until segment work is complete
        Engine->>Flow: Progress automatic work
        Flow->>Resolution: Create or resume nested work
        alt Human decision required
            Resolution-->>Actor: Open allowed-action window
            Actor->>Resolution: Commit action or pass
        else AI or automatic work
            Resolution->>Resolution: Resolve automatically
        end
        Resolution->>State: Reveal, revalidate, or commit
    end
    Engine->>Flow: Run exit work
    Flow->>State: Finalize outputs and cleanup
    Engine->>State: Evaluate battle and advance
```

## Shared Resolution Pattern

Many mechanics should reuse a common proposal pipeline:

```text
collect hidden commitments or automatic proposals
-> reveal the completed batch
-> open reactions when eligible reactions exist
-> revalidate anything changed by reactions
-> commit the accepted batch atomically
-> discover and resolve immediate consequences
```

A proposal is an intended change that has not happened permanently yet. Examples include damage, card removal, status application, resource change, die modification, or target change.

A commit is the moment the accepted proposal changes authoritative battle state.

Reaction chains use simultaneous hidden commitments:

```text
eligible actors secretly react or pass
-> reveal together
-> apply compatible reactions
-> open another reaction round if anyone acted
-> close only when everyone passes in the same reaction round
```

Not every automatic animation needs a rules window. Presentation can play events in order without stopping authoritative progression unless a real decision is required.

### Settled simultaneous-batch rules

All work sharing the same timing point follows this order:

```text
gather from the same pre-batch state
-> collect required automatic results, rolls, or choices
-> reveal together
-> run the declared reaction window
-> resolve conflicts and revalidate
-> commit accepted proposals together
-> create child consequence batches afterward
```

An immediate consequence caused by a commit is not inserted into the middle of its parent batch. It creates a new child batch after the parent commits. Deterministic ordering controls collection, reproducibility, and presentation; it does not grant one simultaneous effect first-actor advantage.

### Settled reaction-window rules

- Every batch declared reactable opens a reaction window.
- Every required human actor must explicitly react or pass, even when they have no useful response.
- Window creation depends on the batch and content rules, not on whether an actor secretly possesses a playable response.
- Non-reactable automatic batches do not open a window.
- A reaction chain closes only when all required actors pass in the same reaction round.
- A played reaction does not automatically open another nested reaction window. Its own content must explicitly declare one.
- Damage-card reveal is always reactable.

## Triggered And Played Effects

Effects are data-driven and declare when they are allowed or triggered. Segment code asks the generic effect system what must happen at the current checkpoint; it must not contain branches for specific content such as Poison, Blind, or Entangle.

The timing identity of triggered work includes at least:

```text
segment + phase/checkpoint + stage/timing point
```

Effects at the same timing point are collected into one global simultaneous batch. Effects at different timing points resolve separately, even when they belong to the same segment.

Examples:

- Three Poison stacks trigger at `ongoing_effects/on_enter` and join one simultaneous Poison roll batch.
- Entangle triggers when Offensive is entered, reduces the affected actor's maximum rolls from three to two for that planning window, and is then consumed without opening a reaction window.
- Blind triggers after an Offensive ability has been selected and planning has otherwise completed. The affected actor rolls 1D6; on 1-2 the selected ability does not happen.

Some effects are not automatic triggers. They behave more like playable cards and may be used only in declared windows. Cleanse may remove a status from its user when Cleanse is legal. A Prevent effect may declare that it can be played only during Defensive and may reduce only a qualifying incoming damage source.

Content definitions therefore need to distinguish:

- automatic trigger timing
- segments, phases, stages, or window purposes in which an effect may be played
- legal targets
- whether the effect applies to one source, multiple sources, or an accumulated total
- the operation proposals produced when the effect resolves
- whether resolving the effect opens a reaction window

The exact YAML field names remain an implementation detail, but the intended content semantics resemble:

```yaml
# Triggered Poison resolution
trigger:
  segment: ongoing_effects
  phase: entry
reaction_window:
  opens: true
  pass_required: true
```

```yaml
# Playable Cleanse resolution
playable_during:
  - allowed window or timing
reaction_window:
  opens: false
```

```yaml
# One-use Entangle resolution
trigger:
  segment: offensive
  phase: entry
operations:
  - type: adjust_max_rolls
    amount: -1
reaction_window:
  opens: false
lifecycle:
  persistent: false
  remove_after_resolution: true
```

Cleanse may be played inside an already-open eligible window without its resolution recursively opening another reaction window. Poison explicitly opens one for its revealed roll or consequence batch.

## Resource Consumption, Reveal, And Cancellation

The authoritative payment moment is when an action is played and accepted, not when the hidden action is later revealed.

```text
validate play
-> immediately consume its costs and limited uses
-> apply or record its private effect
-> persist the hidden commitment
-> reveal the play at the shared reveal point
```

Default consumption rules:

- A played card immediately leaves its current zone and moves to discard.
- Energy is spent immediately when the card, ability, status, or other action that costs it is played.
- A consumable status or token is removed or decremented immediately when played.
- A dice roll or reroll use is consumed immediately when the roll command is accepted.
- Selecting an ability counts as playing it once all required selections are valid; its costs are paid at that time.
- An invalid, rejected, or stale command consumes nothing.
- Content may replace the default destination or payment rule, such as returning a card to hand instead of discarding it.

Immediate consumption does not mean immediate public disclosure. During hidden planning or hidden reactions, the acting player can see the updated private state, but opponents learn which card, status, ability, or resource was used only at the shared reveal. Viewer-safe snapshots and events must not leak the hidden play before that reveal.

Action costs are committed costs, not proposals waiting for the outcome batch. Once an action is accepted, its costs remain spent even if its effect is later blocked, canceled, prevented, or collision-negated. A refund happens only when content explicitly creates one.

Cancellation must name its scope:

- **Cancel source:** cancel the ability, card, status, or other source and every proposal it created.
- **Cancel proposal:** cancel one specific result while preserving the source's other proposals.
- **Prevent or modify damage:** change only the selected damage source or accumulated total.
- **Cancel status application:** stop only the selected status proposal.
- **Modify proposal:** change a declared amount, target, die, or other field.

Example:

```text
Cinder Strike proposes 3 damage and 2 Poison.

Cancel source    -> no damage and no Poison
Prevent 2 damage -> 1 damage and 2 Poison
Cancel Poison    -> 3 damage and no Poison
```

Canceling a selected ability does not normally reopen planning. Planning reopens only when a reaction materially changes a decision input, such as dice, available rolls, or target legality, or when the reacting content explicitly instructs the engine to reopen affected actors. Blind therefore cancels a failed selected ability without granting a replacement selection.

Abilities are not discarded. By default, a selected ability gains a used marker for its segment, and the normal actor may use one Offensive ability per Offensive segment. Cards default to discard immediately when played; explicit return, exhaust, or permanent-removal rules override that destination.

## Status Stacking, Duration, And Removal

Stacks and duration are separate concepts:

- Stacks represent intensity, quantity, or repeated applications.
- Duration represents how long a status remains.
- A status may use stacks, duration, both, or neither.
- A playable status may instead behave as a consumable resource.

Applying the same status definition to the same actor adds stacks to the existing instance up to its declared stack limit. Different definitions, such as Poison and Advanced Poison, do not combine. Applications created by the same simultaneous batch accumulate together before the limit is enforced.

The default overflow policy rejects additional stacks above the limit. Content may override this with a declared behavior such as replacing stacks, converting the status, or creating an overflow resolution.

Reapplying a duration status does not add the two durations together by default. It keeps the longer remaining duration. Content may explicitly refresh, extend, replace, or preserve the existing duration instead.

```text
Weakened with 1 round remaining
+ apply Weakened for 3 rounds
= Weakened with 3 rounds remaining
```

Ordinary round-based durations tick during Ongoing Effects exit. Only statuses present when Ongoing Effects began are eligible for that exit tick, so a status applied during the segment does not immediately lose a round.

Status removal follows the lifecycle declared by that status rather than one universal expiration rule:

- **Duration expiration:** remove after the status's current segment trigger work has completed and its duration reaches zero.
- **Consume after resolution:** remove after its declared resolution and reaction window completely finish. Blind follows this rule in Offensive.
- **Outcome-based removal:** remove stacks only when a declared outcome occurs. Poison persists until a qualifying roll removes stacks.
- **External removal:** another effect such as Cleanse removes the specified stacks or status.
- **Played consumption:** a playable status is removed or decremented immediately when used.

These behaviors are controlled by validated content flags, not by status IDs or engine branches. The exact YAML names may evolve, but the intended configuration resembles:

```yaml
# Poison-like lifecycle
lifecycle:
  persistent: true
  consume_on_play: false
  remove_after_resolution: false
  remove_on_duration_zero: false
```

```yaml
# Blind-like lifecycle
lifecycle:
  persistent: false
  consume_on_trigger_checkpoint: true
  consume_on_play: false
  remove_after_resolution: true
  remove_on_duration_zero: false
```

`consume_on_trigger_checkpoint` means Blind is spent whenever its Offensive trigger point is reached, even when its target selected no ability and no roll is needed. When Blind does create work, `remove_after_resolution` defers the actual removal until the status's complete use—including its roll, reveal, and reaction window—rather than removing it as soon as the trigger starts. A persistent status may still lose stacks through explicit outcome operations or external effects. Content validation must reject contradictory lifecycle flag combinations.

Applying a status does not activate it unless it declares an immediate or on-apply trigger. Otherwise it waits for its declared segment, phase, and stage. The stack count captured when a trigger batch is collected determines the work already in that batch; changes during the resolution affect future batches, not rolls already collected.

For example, three Poison stacks create three Poison rolls. If two completed outcomes remove stacks, all three rolls still finish and the actor ends with one Poison stack.

Reusable removal operations must support:

- remove a specified number of stacks
- remove all stacks of one status
- remove one chosen status
- remove every status matching a category
- consume the status being played

## Segment Designs

### 1. Ongoing Effects

Purpose: resolve statuses, passives, durations, and other scheduled work that belongs at the start of a round.

Entry phase:

1. Gather all `ongoing_effects/on_enter` triggers.
2. Order them deterministically.
3. Group work that should resolve simultaneously.
4. Determine which work is automatic and which work needs a roll, choice, target, or reaction.

Main phase:

```text
collect trigger batch
-> resolve required rolls or choices
-> reveal outcomes
-> reactions
-> generate consequence proposals
-> reveal/react/commit consequences
-> repeat until the trigger queue is empty
```

Example: three Poison stacks can create three simultaneous rolls, one reaction chain over the revealed rolls, and then one accumulated damage proposal.

Exit phase:

- resolve explicit effects-exit triggers
- tick ordinary duration counters and apply expiration rules
- clear temporary effect-resolution state
- as the final exit action, evaluate battle completion before Income

Settled defaults:

- Ordinary duration counters tick during Ongoing Effects exit.
- Effects sharing the same timing point resolve as one global simultaneous batch. Different timing points within the same segment create separate batches.
- The last operation in every segment's exit phase is the battle-completion check. An actor defeated by Ongoing Effects does not receive Income if that check ends the battle.

### 2. Income

Purpose: grant the round's normal cards, energy, and other replenishing resources.

The default Income package is:

```text
draw 1 card
gain 1 energy
```

Energy accumulates between rounds and currently has no hard cap.

Entry phase:

1. Determine each actor's base income and modifiers from the same starting state.
2. Gather income-entry triggers.
3. Build the reward batch.

Main phase:

```text
calculate rewards
-> resolve replacement effects or choices
-> commit draws and resources
-> resolve consequences caused by those gains
```

The normal case should be automatic. An interaction window opens only when a rule creates a meaningful choice, such as choosing one of several cards or replacing a draw.

Exit phase:

- resolve income-exit triggers
- clear temporary reward state
- as the final exit action, evaluate battle completion

Settled defaults:

- Each actor normally draws one card and gains one energy.
- Energy accumulates and has no default hard cap for now.
- The hand cap is six cards, but it is not enforced during Income. Actors may spend cards during the round to return to the cap naturally. Any actor still above six must discard down to six near the end of Damage Resolution.

### 3. Offensive

Purpose: let every participating actor create an attack or other offensive proposal through hidden simultaneous planning.

Entry phase:

1. Determine available offensive dice, rolls, abilities, cards, resources, and targets.
2. Initialize hidden planning for every participating actor.
3. Let AI actors plan automatically.
4. Request decisions from human actors.

Main phase:

```text
hidden planning
-> lock in
-> simultaneous reveal
-> reaction chain
-> revalidation
-> reopen only materially affected actors
-> reveal changed commitments
-> repeat until all commitments are valid
-> finalize offensive proposals
```

During hidden planning an actor may roll, keep dice, reroll, play allowed cards, select an ability, select targets, pass, and lock in. Other actors should not see intermediate rolls or decisions.

The default actor may select one Offensive ability per segment. Selecting a valid ability auto-locks the actor once all required targets have also been selected. If the ability has only itself or no selectable target, selecting the ability immediately locks it in.

Future character mechanics may support multiple ability selections. One anticipated pattern reserves the dice used for a chosen ability, then lets the actor spend remaining rolls and unreserved dice attempting to qualify for another ability. This is an advanced exception, not part of the initial default flow.

Exit phase:

- resolve offensive-exit effects such as the current Blind concept
- cancel or modify proposals as those exit effects require
- persist finalized attacks and other consequences for Defense and Damage Resolution
- do not apply attack damage yet

Reveal information:

- Reveal final dice, selected ability, committed cards, selected targets, and proposals already determined by that commitment.
- Do not perform or reveal conditional follow-up calculations prematurely. For example, an ability that deals the total of a later 3D6 roll reveals that it was selected, but rolls its damage only after the ability survives its reaction and cancellation window.

### 4. Defensive

Purpose: let targeted actors answer finalized defensible offensive proposals.

Entry phase:

1. Load finalized incoming attacks and defensible effects.
2. Determine eligible defenders and the proposals each may target.
3. Mark actors without incoming defensible work as not participating.
4. Present each defender's legal Defensive abilities and usage limits.
5. Initialize hidden Defensive selections.

Main phase:

```text
review incoming sources
-> select Defensive ability and legal target or pass
-> immediately spend its costs and limited uses
-> lock in hidden selections
-> reveal selected defenses
-> create each selected ability's declared nested resolution
-> resolve ability-specific dice, choices, reveals, and reactions
-> finalize defensive proposals
-> allow another selection only when content permits it
-> finish Defensive
```

Defensive abilities are not qualified or unlocked by a shared 5D6 roll. Any dice belong to the selected ability itself. For example, Basic Defense is always selectable against a qualifying incoming source, then rolls 1D6 and prevents damage equal to its finalized face.

An ability-specific die does not use the Offensive symbol pool, keep/reroll history, or maximum Offensive roll count unless that ability explicitly says otherwise.

Participation defaults:

- An actor with no incoming defensible proposal auto-completes and has nothing to select.
- If no actor has an incoming defensible proposal, the entire Defensive segment automatically completes.
- Defensive content declares whether it targets one incoming source, multiple sources, or an accumulated total.
- Content may also limit a defense to once per attack, once per segment, or another explicit usage rule. This allows multiple defensive abilities to be used when their rules permit it.
- Enemy AI selects from its currently legal Defensive abilities directly. It does not roll a D100 to determine whether those abilities were activated.

Exit phase:

- attach accepted defensive modifiers to the offensive proposals they affect
- add counterattacks and counter-effects to the current round's pending Damage Resolution
- preserve source attribution
- do not remove health cards or commit final attack consequences yet

Counterattack rule:

- Every counterattack remains a separate damage source. An attack for eight and a counterattack for two are not automatically combined into ten source damage.
- Source identity matters for effects such as Protect, which reduces one incoming source by half. A future stronger effect may instead reduce every source or an accumulated damage total if its content definition says so.

### 5. Damage Resolution

Purpose: turn accepted offensive and defensive proposals into final, permanent consequences.

Entry phase:

1. Collect all pending sources and modifiers.
2. Apply prevention, increases, reductions, redirection, and target changes.
3. Calculate both per-source and accumulated totals.
4. Select proposed health-card removals randomly without replacement using the default zone order: deck, then discard pile, then hand.
5. If damage exceeds all remaining cards, reveal every card and preserve the remaining amount as overage damage.

Main phase:

```text
reveal final proposals and selected damage cards
-> reaction chain
-> recalculate and reconcile changed cards/totals
-> commit accepted damage and card removals
-> apply statuses, resources, and other same-batch consequences
-> resolve immediate consequence batches
-> enforce the six-card hand cap
-> repeat until no pending consequence remains
```

Damage cards are always revealed before commitment. The human player always receives a reaction window after the reveal and must explicitly play an allowed response or pass. This is a rules-level pause, not merely animation pacing, even when the player ultimately has no useful response.

Damage prevention removes overage damage first. Only prevention remaining after overage reaches zero releases proposed card removals. For example, if damage exceeds the target's remaining cards and the target prevents six damage, the six points reduce overage before saving any cards.

Within each zone, cards are randomly selected without replacement. The normal zone priority may be overridden by content. An ability may select from hand first, from discard first, allow a player choice, or use another declared rule.

Damage and same-batch status applications commit together. Applying a status does not necessarily activate its triggered behavior immediately. Poison may be attached during Damage Resolution but waits until its declared Ongoing Effects trigger. A status with an explicit immediate trigger may instead create a new consequence batch.

After all damage and immediate consequences have resolved, each actor above the six-card hand cap receives a discard choice and must discard down to six before Damage Resolution may exit.

Exit phase:

- resolve damage-resolution exit triggers
- clear temporary attacks, defenses, proposals, and resolution state
- mark pending defeats as final defeats
- as the final exit action, evaluate victory, defeat, draw, or escape
- if active, advance to the next round

Settled defaults:

- Default damage-card selection is random without replacement within the zone priority deck, discard, then hand; content may override it.
- Damage reveal always opens a mandatory response-or-pass window for the human player.
- Damage and status application can belong to the same commit, but scheduled status activation follows the status's declared trigger timing.

### Overage damage and defeat

Overage is provisional damage used during reveal and reactions. It is not persistent negative health.

For an actor with `C` remaining health cards:

```text
final damage = max(original damage - prevention, 0)
cards removed = min(final damage, C)
overage = max(final damage - C, 0)
```

Prevention therefore removes overage before it can release a proposed card removal. An actor is pending defeat whenever the accepted result would leave it with zero cards, even when final overage is zero.

Source-specific prevention causes source totals, accumulated totals, selected cards, and overage to be recalculated. Overage remains visible during the reaction window so actors can understand how much prevention is needed to save a card.

After commit, overage has no additional default effect. It may be referenced by future content explicitly. An actor at zero cards remains pending defeat while all immediate consequences and nested resolutions belonging to the current segment finish. The actor becomes finally defeated during that segment's exit, immediately before the battle-completion check.

## Actor Progress States

Within a shared stage, each actor can be:

| State | Meaning |
|---|---|
| Resolving automatically | The engine can still act for this actor without human input |
| Needs input | A human actor has a current allowed-command prompt |
| Locked in | The actor made a hidden commitment and is waiting for others |
| Resolved | The actor completed the current stage |
| Not participating | The current stage does not apply to this actor |

The battle returns control to the client only for meaningful human input or terminal completion. AI work and ordinary animations should not create authoritative pauses.

## Completion Rules

- A segment cannot exit while a required actor, nested resolution, or immediate consequence is unresolved.
- Reaching zero health does not interrupt an active segment or reaction chain.
- The battle-completion check is the final action of every segment's exit phase.
- If both the player and all enemies are defeated by the completed work, the authoritative battle result is a draw.
- The next round begins only after Damage Resolution fully exits.

## Current Implementation Snapshot

As of this draft, the Go authority already implements:

- the five-segment order and round wrap
- `on_enter`, `in_progress`, and `on_exit` flow checkpoints
- automatic progression until human input
- persisted actor progress and pending input
- hidden shared planning for Offensive and Defensive
- planning rolls, keeps, rerolls, cards, abilities, targets, pass, and lock-in commands
- simultaneous reveal, reaction windows, reaction rounds, and selective replanning
- damage collection, calculation, proposed health-card removal, reveal, reactions, and commit
- battle completion at segment boundaries
- viewer-filtered snapshots, persistence, recovery, and replay foundations

Important gaps between code and the target design include:

- Ongoing Effects is currently a placeholder and does not execute status triggers.
- Income currently defaults to drawing one card for the actor ID `player`; the target default of granting one energy is not implemented in that default flow.
- Status applications and immediate status-triggered work after damage are not yet executed by the Damage Resolution flow.
- The six-card hand cap and end-of-Damage-Resolution discard choice are not yet implemented.
- Content-level declarations for whether a resolution opens its own reaction window are not yet implemented.
- Immediate payment, card destination, consumable-use, and refund rules are not fully implemented in planning and reaction commands.
- The complete status lifecycle, including generic stacking, duration, trigger-relative expiration, and removal modes, is not yet implemented.
- Overage calculation, presentation, and content hooks are not yet complete as target rules.
- Prebaked enemy D100 planning charts keyed by available roll count are not yet implemented.
- The target selection-first Defensive flow differs from the current shared Offensive/Defensive qualification implementation and requires a dedicated Defensive orchestration path.
- Several reaction conflict and content-specific policies remain intentionally undefined.
- Presentation pacing and the final client experience are not authoritative engine rules yet.

## Design Principles To Preserve

- Segment order is separate from segment mechanics.
- The Go battle engine is authoritative; Godot presents events and submits allowed commands.
- Hidden decisions remain hidden until their shared reveal.
- Consequences are proposed before they are committed.
- Offensive and Defensive reuse interaction windows, proposals, reactions, and synchronization, but they do not share an ability-qualification flow.
- Rolls, choices, reactions, and targets reuse one generic interaction-window model.
- Content configures reusable operations instead of adding content-name branches to the engine.
- Automatic progression is safe, finite, deterministic, and resumable.

## Settled Structural Decisions

- The complete shared cycle is a Round.
- Every segment uses Entry, Main, and Exit phases.
- Ongoing Effects and Income remain separate, producing five segments total.
- Work at the same timing point uses one simultaneous batch derived from the same pre-batch state.
- Child consequences begin only after their parent batch commits.
- Reactable batches always require eligible human actors to react or explicitly pass.
- Content declares whether its resolution opens a reaction window.
- Default damage selection is random within the zone order deck, discard, then hand.
- Resources and limited uses are consumed as soon as a play is accepted, while public disclosure waits for reveal.
- Played cards default to immediate discard, and spent costs are not refunded when their effects are canceled or collision-negated.
- Cancellation has an explicit source, proposal, damage, status, or modification scope.
- Replanning happens only after a material input change or an explicit reopen instruction.
- Status stacks and durations are separate, and each status declares whether it expires by duration, after resolution, by outcome, through external removal, or when played.
- Status lifecycle behavior is controlled by validated YAML flags rather than hard-coded status identities.
- Overage is provisional reaction-window information; zero remaining cards determines pending defeat, and final defeat waits for the current segment's exit.
- Offensive abilities are earned through the 5D6 qualification roll; Defensive abilities are selected directly, and any dice are part of the selected ability's own resolution.
- Every card instance is exactly one point of health; deck, hand, and discard are active health zones, while permanent removal reduces health by one.
- Abilities are separate board content and never count as health.
- Every qualified Offensive board ability remains a player choice; the chosen ability resolves its matching tier plus every compatible matching bonus condition from the same final dice.
- Cards may apply battle-duration runtime upgrades to board abilities without mutating their reusable YAML definitions.

## Example Round Combat Loadouts

The first walkthrough uses mirrored standard 5D6 face maps for the player and enemy. The following abilities are illustrative rules-test content, not final balance commitments.

### Starting state

The walkthrough begins at Round 3 Ongoing Effects entry.

| State | Player | Enemy |
|---|---|---|
| Deck | 4 cards | 5 cards |
| Discard | 2 cards | 2 cards |
| Hand | 6 cards | 3 cards |
| Energy | 2 | 2 |
| Combat dice | 5 standard D6 | 5 standard D6 |
| Normal Offensive rolls | 3 | 3 |
| Statuses | Poison 3, Entangle, Blind | None |

Entangle is a one-use status waiting for Offensive entry. It has no duration: it reduces the player's upcoming maximum rolls from three to two, then removes itself without a reaction window. Blind removes itself only after its complete Offensive resolution. Poison is persistent and rolls once per stack.

The player's initial hand contains these rules-test cards:

- `Loaded Die`: spend one energy and discard the card immediately to change one selected combat die to face 6, Gold Coin.
- `Tip It`: spend one energy and discard the card immediately during a qualifying reaction window to change one selected revealed face-6 die to face 5, Shield.

Each play remains hidden until its relevant shared reveal.

### Player Offensive abilities

| Ability | Requirement | Proposed result |
|---|---|---|
| Sword Cut | 3/4/5+ Swords | Deal 5/6/7 damage to one target |
| Shield Bash | 2+ Swords and 2+ Shields | Deal 4 damage and apply Entangle to one target |
| Golden Edge | 2+ Swords and 1+ Gold Coin | Deal 5 damage and gain 1 energy |
| Perfect Form | Final faces contain 1, 2, 3, 4, and 5 | Deal 8 damage to one target |

### Player Defensive abilities

| Ability | Selection rule | Resolution and proposed result |
|---|---|---|
| Basic Defense | Select one incoming damage source | Roll 1D6; prevent damage from that source equal to the finalized face |
| Protect | Select one incoming damage source | Reduce that source's final damage by half; no qualification roll |

### Enemy Offensive abilities

| Ability | Requirement | Proposed result |
|---|---|---|
| Jagged Slash | 3/4/5+ Swords | Deal 4/5/6 damage to one target |
| Venom Strike | 2+ Swords and 1+ Gold Coin | Deal 3 damage and apply 2 Poison to one target |
| Crushing Advance | 2+ Swords and 2+ Shields | Deal 5 damage to one target |
| Greedy Blow | 2+ Gold Coins | Deal 7 damage to one target |

### Enemy Defensive abilities

| Ability | Selection rule | Resolution and proposed result |
|---|---|---|
| Basic Defense | Select one incoming damage source | Roll 1D6; prevent damage from that source equal to the finalized face |
| Protect | Select one incoming damage source | Reduce that source's final damage by half; no qualification roll |

The enemy uses the same ability qualification, payment, targeting, validation, lock-in, and reveal contracts. Instead of literally rolling and keeping combat dice, it uses its D100 planning table to produce a valid hidden commitment automatically. The shared reveal still shows final face numbers, symbols, simulated rolls used, selected ability, cards, target, and proposals.

## Example Round Walkthrough

### Segment 1: Ongoing Effects

#### Entry phase

The engine gathers every Round 3 `ongoing_effects/entry` trigger. The player has three Poison stacks, so Poison creates three simultaneous status D6 rolls. These effect dice are declared by Poison and do not consume the actor's five combat dice or combat roll uses.

The deterministic walkthrough results are:

| Poison die | Face | Initial outcome |
|---:|---:|---|
| 1 | 2 | Propose 1 damage |
| 2 | 5 | Propose removing 1 Poison stack |
| 3 | 6 | Propose removing 1 Poison stack |

#### Main phase: roll batch

Poison declares its roll batch reactable:

```text
reveal all three Poison dice
-> open Poison-roll reaction window
-> required actors react or explicitly pass
-> finalize all three dice
```

The player passes and the enemy AI passes. The finalized outcomes create a child consequence batch containing one damage and two Poison-stack-removal proposals.

#### Main phase: damage child batch

Damage reveal always creates its own response window. The engine randomly selects one card from the player's deck without removing it permanently yet.

```text
reveal the proposed card removal
-> open mandatory damage response-or-pass window
-> player passes
-> enemy AI passes
-> commit the batch
```

The selected card moves to the player's permanently removed zone, and the two accepted status operations reduce Poison from three stacks to one. The two reaction windows remain distinct because roll-changing effects require finalized dice before damage cards can be calculated, while damage reactions target the later damage and card proposals.

#### Exit phase

- Ordinary eligible durations tick.
- Entangle remains queued for its one-time Offensive-entry trigger; it has no duration to tick.
- Blind remains because its Offensive use has not occurred.
- Poison remains with one persistent stack.
- Temporary Ongoing Effects resolution state is cleared.
- The final exit action checks battle completion; both actors remain active.

State carried into Income:

| State | Player | Enemy |
|---|---|---|
| Deck | 3 cards | 5 cards |
| Discard | 2 cards | 2 cards |
| Hand | 6 cards | 3 cards |
| Removed | 1 card | 0 cards |
| Energy | 2 | 2 |
| Statuses | Poison 1, Entangle, Blind | None |

### Segment 2: Income

#### Entry and main phases

No content modifies the ordinary Income package, so both actors receive the default simultaneous reward calculated from the same pre-Income state:

```text
draw 1 card
gain 1 energy
```

The reward batch is not reactable in this example. Each actor sees the identity of its own drawn card. The opponent may see public hand and deck counts but not the hidden card identity.

#### Exit phase

- The player is allowed to remain at seven hand cards because the six-card cap is enforced only near the end of Damage Resolution.
- Temporary Income state is cleared.
- The final exit action checks battle completion; both actors remain active.

State carried into Offensive:

| State | Player | Enemy |
|---|---|---|
| Deck | 2 cards | 4 cards |
| Discard | 2 cards | 2 cards |
| Hand | 7 cards | 4 cards |
| Removed | 1 card | 0 cards |
| Energy | 3 | 3 |
| Statuses | Poison 1, Entangle, Blind | None |

### Segment 3: Offensive

#### Entry phase: Entangle

The engine gathers `offensive/entry` triggers before opening planning. Entangle resolves automatically:

```text
player normal maximum rolls: 3
-> Entangle reduces this planning window to 2
-> persist max_rolls = 2 in the player's roll request
-> remove Entangle
```

Entangle is non-reactable. It changes no dice and proposes no damage, so it opens no reaction window.

The enemy has no roll-count modifier and selects its three-roll D100 Offensive chart.

#### Main phase: player roll one

The player rolls all five standard combat dice:

| Die | Face | Symbol | Decision |
|---:|---:|---|---|
| 1 | 1 | Sword | Keep |
| 2 | 1 | Sword | Keep |
| 3 | 4 | Shield | Reroll |
| 4 | 5 | Shield | Reroll |
| 5 | 6 | Gold Coin | Keep |

State after the first roll:

```text
rolls used: 1 of 2
rerolls remaining: 1
kept dice: [1 Sword, 1 Sword, 6 Gold Coin]
```

#### Main phase: player final reroll

The player rerolls dice 3 and 4. Because Entangle reduced the maximum to two total rolls, this is the final roll.

| Die | Final face | Final symbol |
|---:|---:|---|
| 1 | 1 | Sword |
| 2 | 1 | Sword |
| 3 | 3 | Sword |
| 4 | 3 | Sword |
| 5 | 6 | Gold Coin |

The final pool contains four Swords and one Gold Coin. It qualifies:

- Sword Cut at its four-Sword tier: propose 6 damage.
- Golden Edge: propose 5 damage and gain 1 energy.

The player selects Sword Cut and targets the enemy. Because the ability and required target are valid, selection immediately locks in the player. The Loaded Die card is not played in this example and remains in hand.

#### Main phase: enemy D100 plan

The enemy makes one hidden D100 roll against its three-roll Offensive chart. For the walkthrough, the result is `19`, selecting the authored Ability 2 row with one simulated reroll remaining.

That row produces the following valid hidden reveal profile:

```text
2 Sword, 2 Sword, 3 Sword, 5 Shield, 6 Gold Coin
```

The enemy selects Venom Strike, targets the player, records two simulated rolls used, and locks in. No human-style enemy roll or reroll sequence occurs.

#### Main phase: initial reveal

After both actors lock in, the engine reveals the completed Offensive commitments together.

Player reveal:

```text
final dice: 1 Sword, 1 Sword, 3 Sword, 3 Sword, 6 Gold Coin
rolls used: 2 of 2
selected ability: Sword Cut, four-Sword tier
target: enemy
proposals: deal 6 damage
```

Enemy reveal:

```text
final dice: 2 Sword, 2 Sword, 3 Sword, 5 Shield, 6 Gold Coin
simulated rolls used: 2 of 3
selected ability: Venom Strike
target: player
proposals: deal 3 damage and apply 2 Poison
```

#### Main phase: Offensive reaction chain

The initial reveal is reactable. In reaction round one, the player secretly plays `Tip It` on the enemy's face-6 Gold Coin die. The accepted play immediately spends one energy and moves `Tip It` from hand to discard, but the enemy does not learn what was played until the round reveals. The enemy AI passes.

Reaction round one reveals:

```text
Tip It: enemy die 5 changes from face 6, Gold Coin to face 5, Shield
```

The modified enemy pool is:

```text
2 Sword, 2 Sword, 3 Sword, 5 Shield, 5 Shield
```

Venom Strike is no longer valid because the pool has no Gold Coin. The engine checks every enemy Offensive ability against the complete modified state:

- Jagged Slash is valid with three Swords and would deal 4 damage.
- Crushing Advance is valid with at least two Swords and two Shields and would deal 5 damage.
- Venom Strike is invalid.
- Greedy Blow is invalid.

Two abilities remain valid, so the engine makes a persisted uniform random selection. The walkthrough result selects Crushing Advance.

The changed enemy commitment is revealed:

```text
selected ability changed: Venom Strike -> Crushing Advance
updated proposals: deal 5 damage
removed proposal: apply 2 Poison
```

Because materially changed commitment information was revealed, another Offensive reaction round opens. The player passes and the enemy AI passes in the same round, closing the reaction chain.

#### Exit phase: Blind

Normal Offensive planning, reveal, reaction, and revalidation are complete. Blind now triggers for the player because the player selected an Offensive ability.

The intended Blind content semantics resemble:

```yaml
trigger:
  segment: offensive
  phase: exit
  stage: after_offensive_revalidation

resolution:
  roll:
    dice_count: 1
    dice_id: standard_d6
  outcomes:
    - faces: [1, 2]
      operations:
        - type: cancel_source
          target: selected_offensive_ability
    - faces: [3, 4, 5, 6]
      operations:
        - type: noop

reaction_window:
  opens: true
  pass_required: true

lifecycle:
  persistent: false
  consume_on_trigger_checkpoint: true
  remove_after_resolution: true
```

##### Blind trigger eligibility

The engine evaluates Blind only after normal Offensive commitments are accepted:

```text
player selected Sword Cut: yes
Sword Cut has not already been canceled: yes
Blind therefore requires one D6 roll
```

If the player had passed or had no selected ability, Blind would remove itself without rolling because there would be no ability to test.

Blind is therefore consumed in every case when its Offensive checkpoint is reached:

```text
no selected ability -> no roll -> remove Blind
selected ability -> complete roll and reaction resolution -> remove Blind
selected ability already canceled -> no roll -> remove Blind
```

##### Blind effect roll

Blind creates a separate one-die effect roll. It does not reuse a combat die, consume one of the player's two Offensive rolls, or change the finalized five-die combat pool.

The player's default status-roll preference is automatic, so the engine rolls the die. The walkthrough result is:

| Effect | Die | Face | Provisional result |
|---|---:|---:|---|
| Blind | 1D6 | 4 | Preserve Sword Cut |

The face is provisional until Blind's reaction window finishes.

##### Blind reaction window

The engine reveals face 4 and opens the Blind roll's declared reaction window:

```text
eligible actors secretly commit a Blind-roll reaction or pass
-> player explicitly passes
-> enemy AI passes
-> reveal both passes
-> close the reaction round
```

If an eligible effect had changed or rerolled the Blind die, the engine would evaluate the final modified face rather than the original face 4. A final face of 1-2 would cancel the complete Sword Cut source; a final face of 3-6 preserves it.

##### Blind outcome and removal

Both actors passed, so face 4 becomes final. Blind's no-op success outcome preserves Sword Cut and its six-damage proposal.

Only now—after the roll, reveal, reaction window, and outcome are complete—does `remove_after_resolution` remove Blind. Blind itself is spent regardless of whether the actor ultimately has an ability. Blind does not reopen Offensive planning in either outcome. If it had canceled Sword Cut, the player would finish Offensive with no selected attack and any previously spent resources would remain spent.

The finalized Offensive outputs are:

```text
Player -> Enemy: Sword Cut, 6 damage source
Enemy -> Player: Crushing Advance, 5 damage source
```

No attack damage is committed yet. Entangle and Blind have both been removed, while the player's one persistent Poison stack remains.

The final Offensive exit action checks battle completion; both actors remain active.

State carried into Defensive:

| State | Player | Enemy |
|---|---|---|
| Deck | 2 cards | 4 cards |
| Discard | 3 cards | 2 cards |
| Hand | 6 cards | 4 cards |
| Removed | 1 card | 0 cards |
| Energy | 2 | 3 |
| Statuses | Poison 1 | None |
| Incoming source | Crushing Advance: 5 | Sword Cut: 6 |

### Segment 4: Defensive

#### Entry phase

Both actors have an incoming defensible source, so both participate:

```text
Player defends against: Crushing Advance, 5 damage
Enemy defends against: Sword Cut, 6 damage
```

Defensive does not create a shared 5D6 qualification roll. Each actor receives the public incoming-source list and its currently legal selection-first Defensive abilities.

#### Main phase: hidden Defensive selections

The player selects Basic Defense and targets Crushing Advance. The enemy AI directly selects Basic Defense and targets Sword Cut. Neither actor needs to earn Basic Defense with symbols or face combinations.

After both required actors lock in, the selections reveal together:

```text
Player Basic Defense -> target Crushing Advance
Enemy Basic Defense -> target Sword Cut
```

Basic Defense declares its own nested roll resolution:

```yaml
selection:
  target: one_incoming_damage_source

resolution:
  roll:
    dice_count: 1
    dice_id: standard_d6
  operations:
    - type: prevent_damage
      amount: rolled_face
      target: selected_source

reaction_window:
  opens: true
  pass_required: true
```

#### Main phase: Basic Defense roll batch

The two selected Basic Defense abilities roll their separate 1D6 effect dice as one simultaneous timing batch:

| Defender | Targeted source | 1D6 result | Provisional prevention |
|---|---|---:|---:|
| Player | Crushing Advance: 5 | 3 | Prevent 3 |
| Enemy | Sword Cut: 6 | 2 | Prevent 2 |

These are ability-specific effect dice referencing the reusable `standard_d6` definition. Each result still has one numeric face and one symbol, but Basic Defense reads only the number and ignores the Sword, Shield, or Gold Coin for qualification. There is no keep/reroll loop unless Basic Defense itself is later authored to provide one.

The results reveal together and open Basic Defense's declared reaction window. The player passes and the enemy AI passes in the same reaction round, so both D6 faces become final.

The selected defenses produce these finalized modifiers:

```text
Player Basic Defense: prevent 3 from Crushing Advance
Enemy Basic Defense: prevent 2 from Sword Cut
```

#### Exit phase

Defensive modifiers attach to their selected source proposals:

```text
Crushing Advance: 5 base - 3 Basic Defense = 2 damage to Player
Sword Cut: 6 base - 2 Basic Defense = 4 damage to Enemy
```

No health cards are selected or removed during Defensive. The final exit action checks battle completion; both actors remain active.

State carried into Damage Resolution:

| Target | Finalized incoming sources |
|---|---|
| Player | Crushing Advance: 2 |
| Enemy | Sword Cut: 4 |

### Segment 5: Damage Resolution

#### Entry phase: collect and calculate

The engine collects the two finalized sources and calculates per-source and accumulated totals:

| Target | Source | Base | Defensive prevention | Final |
|---|---|---:|---:|---:|
| Player | Crushing Advance | 5 | 3 | 2 |
| Enemy | Sword Cut | 6 | 2 | 4 |

Accumulated target totals:

```text
Player: 2 damage
Enemy: 4 damage
```

Neither target has overage because each has more remaining health cards than incoming damage.

#### Main phase: propose health-card removals

The pre-selection card zones are:

| Zone | Player | Enemy |
|---|---:|---:|
| Deck | 2 | 4 |
| Discard | 3 | 2 |
| Hand | 6 | 4 |
| Already removed | 1 | 0 |

Default damage selection uses deck, then discard, then hand, with random selection without replacement inside each zone. In this case, both damage totals exactly consume the targets' remaining decks:

```text
Player: randomly order/select both deck cards for 2 proposed removals
Enemy: randomly order/select all four deck cards for 4 proposed removals
```

The selected cards remain proposals at this point. They have not moved to the permanently removed zone.

Each proposed removal preserves its source attribution:

```text
2 Player card proposals <- Crushing Advance
4 Enemy card proposals <- Sword Cut
```

#### Main phase: simultaneous reveal

The engine reveals the complete damage batch together:

```text
Player
- final incoming damage: 2
- source: Crushing Advance
- proposed cards: both remaining deck cards
- overage: 0

Enemy
- final incoming damage: 4
- source: Sword Cut
- proposed cards: all four remaining deck cards
- overage: 0
```

The UI can display the exact selected card identities in each target's pending-removal row. The reveal does not permanently remove them yet.

#### Main phase: damage reaction window

Damage reveal is always reactable. The engine opens one shared hidden response window containing all revealed source, total, and card-removal proposals.

For this walkthrough:

```text
player has no response it chooses to use -> explicitly passes
enemy AI has no response it chooses to use -> passes
```

The two passes reveal together. Because every required actor passed in the same reaction round, the chain closes. No recalculation or replacement selection is required.

#### Main phase: commit

The accepted batch commits atomically:

- Both proposed player deck cards move to the permanently removed zone.
- All four proposed enemy deck cards move to the permanently removed zone.
- No cards move from discard or hand.
- No overage persists.
- Crushing Advance and Sword Cut have no additional same-batch status or resource operations in this walkthrough.

State immediately after commit:

| State | Player | Enemy |
|---|---:|---:|
| Deck | 0 | 0 |
| Discard | 3 | 2 |
| Hand | 6 | 4 |
| Removed | 3 | 4 |
| Remaining health cards | 9 | 6 |
| Energy | 2 | 3 |
| Statuses | Poison 1 | None |

Neither actor is pending defeat because both retain cards in discard and hand.

#### Main phase: immediate consequences and hand cap

No committed operation creates an immediate child resolution in this example.

The engine then enforces the six-card hand cap:

```text
Player hand: 6 -> legal, no discard choice
Enemy hand: 4 -> legal, no discard choice
```

#### Exit phase

- Resolve any declared Damage Resolution exit triggers; none exist in this walkthrough.
- Clear temporary attack, defense, damage-proposal, selected-card, and reaction state.
- Neither actor is pending defeat, so neither becomes defeated.
- Run the final battle-completion check; the battle remains active.
- Advance from Round 3 Damage Resolution to Round 4 Ongoing Effects.

State carried into Round 4:

| State | Player | Enemy |
|---|---:|---:|
| Deck | 0 | 0 |
| Discard | 3 | 2 |
| Hand | 6 | 4 |
| Removed | 3 | 4 |
| Energy | 2 | 3 |
| Statuses | Poison 1 | None |

## Remaining Decisions To Make Together

Suggested discussion order:

1. Walk through one complete example round and revise every unclear transition.
