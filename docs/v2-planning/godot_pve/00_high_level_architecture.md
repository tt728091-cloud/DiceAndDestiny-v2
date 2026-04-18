# Godot-Only PvE High-Level Architecture

## Purpose

This document explores a V2 path where Dice and Destiny becomes a single-player PvE game built entirely in Godot, with no Go server.

The reference inventory is:

- `/Users/daddymere/games/Dice-and-Destiny/docs/v2-planning/v2_game_inventory_and_boundary_map.md`

The goal is to preserve the strongest parts of the existing game idea while changing the runtime shape from online PvP to local player-vs-environment.

This should still feel like the same game. The major change is opponent shape and runtime topology, not the identity of the card, dice, ability, and card-as-health systems.

## Core Direction

The PvE version should be structured like a local deterministic game engine inside Godot.

There is no external Go server, but the code should still behave like a client/server architecture inside one Godot project:

- no lobby service
- no matchmaker
- no game-session server
- no websocket protocol
- no remote authoritative state; authority lives in the local battle engine
- hard local split between presentation client code and authoritative battle engine code

Instead, Godot owns:

- run progression
- battle selection
- encounter setup
- combat rules
- enemy behavior
- content loading
- save/load
- presentation
- deterministic tests

Local rule:

- presentation code acts like the client
- battle engine code acts like the server authority
- presentation sends commands into the battle engine
- battle engine validates commands, mutates state, and returns events/snapshots
- presentation renders those events/snapshots
- presentation must not directly mutate authoritative battle state

The architecture should still keep the same discipline we wanted for V2:

- small feature folders
- deterministic test fixtures
- narrow command entry points
- domain rules separated from UI
- no global prompt sprawl
- one feature slice at a time

Another important goal is future portability. If the Godot-only PvE game works well and a later PvP server, PvE server, or MMO-style version is built, the combat rules, content schemas, tests, and many battle concepts should be reusable. That means local Godot code should still separate rules from presentation and avoid hard-coding combat logic into scenes.

## What Changes From Multiplayer V2

### Removed Boundaries

These multiplayer boundaries disappear:

- `lobby`
- `matchmaker`
- `game-session`
- server protocol messages
- player seat assignment
- reconnect logic
- websocket state sync

### New PvE Boundaries

These replace the networked server/client split:

- `app shell`
- `local client adapter`
- `battle engine boundary`
- `run engine`
- `encounter engine`
- `combat engine`
- `enemy AI / intent engine`
- `content runtime`
- `save system`
- `presentation`
- `test harness`

These boundaries are local to Godot, but they should be treated like real boundaries. The `combat engine` is the closest equivalent to the future server-authoritative rules engine if PvP returns later.

## Proposed Godot Boundary Map

### App Shell

Owns high-level screen and mode transitions.

Responsibilities:

- boot the game
- route between title, run setup, map, battle, rewards, and game over
- own top-level scene loading
- avoid owning combat rules

Example features:

- title screen
- new run
- continue run
- run completed
- run failed

### Local Client Adapter

Owns the Godot-side request boundary from presentation into the battle engine.

Responsibilities:

- convert UI actions into battle commands
- call the battle engine through a narrow interface
- receive battle events and state snapshots
- publish render-friendly view updates to presentation
- prevent UI nodes from directly mutating combat state

This is the local stand-in for what a network client would do in a future server version.

Example:

- UI selects an ability and target
- local client adapter creates `SelectOffensiveAbilityCommand`
- battle engine validates and records the selection
- battle engine returns `AbilitySelected` or a validation failure
- presentation updates from the returned event

### Battle Engine Boundary

Owns the local authoritative command API for battles.

Responsibilities:

- expose command entry points such as roll dice, keep dice, select ability, play card, pass, continue, and reveal
- validate commands against current battle state
- execute accepted commands
- return domain events and read-only state snapshots
- hide mutable combat state from presentation

This is the boundary that should be easiest to replace with a server call later.

Future server translation:

- local call: `battle_engine.submit(command)`
- server call: send command JSON to server, receive events/snapshot
- combat rules should remain behind the same conceptual boundary

### Authority-Side Enemy Controller

Owns enemy decisions from inside the authority side of the architecture.

The enemy is not a second presentation client. In a future server PvE or MMO-style game, the enemy controller would live on the server beside the battle engine. In the local Godot PvE version, it should live beside the battle engine as local authority code.

Responsibilities:

- observe the battle state through authority-approved read models
- choose hidden enemy dice results, abilities, cards, and reactions
- submit enemy commands into the battle engine boundary
- allow the battle engine to validate and record those commands
- keep enemy decisions hidden until the reveal point

Important rule:

- player presentation code sends player commands through the local client adapter
- enemy controller sends enemy commands from inside the authority side
- both command paths should meet at the battle engine boundary
- the battle engine remains the only place that validates and mutates authoritative battle state

Future server translation:

- player command path: remote client -> server battle API -> battle engine
- enemy command path: server enemy controller -> battle engine
- reveal path: battle engine -> events/snapshot -> remote clients

This preserves the PvP-like structure while avoiding fake networking for the enemy in the local PvE game.

### Run Engine

Owns the roguelike or campaign run state outside a single battle.

Responsibilities:

- selected character
- current deck
- persistent run resources
- current map node or battle selection
- rewards collected
- relics or upgrades if added later
- run-level random seed
- transition into encounters

This is where the Slay-the-Spire-like or Darkest-Dungeon-like structure could eventually live.

Possible first version:

- choose a character
- choose from a small list of battles
- after a battle, choose a reward
- proceed to the next battle

Things to avoid early:

- complex map generation
- permanent meta progression
- relic systems
- shops
- events

Current decision:

- do not design run progression yet
- focus purely on battles first
- after core battle gameplay works, decide whether encounter progression is map-based, campaign-node-based, dungeon-based, or another structure

### Encounter Engine

Owns the setup of one battle.

Responsibilities:

- choose enemies
- build initial player combat state from run state
- build initial enemy combat state
- define win/loss conditions
- provide encounter rewards after victory

This layer bridges run state into combat state.

Example:

- run says player has deck A
- encounter says enemy group is `slime_pair_1`
- encounter engine creates a combat setup with player deck, enemy data, seed, and battle context

### Combat Engine

Owns the deterministic rules for a battle.

Responsibilities:

- cards
- deck, hand, discard, removed pile
- card-as-health if preserved
- resources
- dice rolls
- abilities
- status effects
- damage resolution
- turn or segment progression
- combat events
- validation of combat commands

This should be the local replacement for the old server authority.

Important rule:

- UI can ask the combat engine what is legal, but UI should not own the rules.
- UI should not reach into combat state directly; it should go through the local client adapter and battle engine boundary.

### Enemy AI / Intent Engine

Owns enemy decisions.

Responsibilities:

- choose enemy intent
- expose enemy intent to the UI
- resolve enemy action during combat
- apply enemy abilities, attacks, blocks, statuses, and special actions

Recommended first version:

- enemies use the same kind of game pieces as the player, including cards and dice where useful
- enemies may "cheat" when choosing results instead of fully simulating player-style decisions
- enemy dice can be presented as a generated result, such as "this attack took 2 rolls," without implementing complex keep/reroll decision logic
- enemy cards can be selected by simple rules, weighted behavior, or scripted intent patterns
- enemies use a deterministic intent list or weighted pattern where that is enough
- the player should not see the enemy's selected roll or attack result until the reveal point
- enemy and player selections reveal through the same simultaneous flow

Avoid early:

- complex AI planners
- hidden simultaneous PvP-style action commitment

Design note:

- enemies should feel like they are using the same combat language as the player
- enemy implementation does not need to be fair, symmetrical, or fully simulated
- the test harness must be able to force hidden enemy intent, selected card, dice result, and final action output
- enemy shortcuts should live in the enemy controller and intent engine, not in shared player command rules

### Content Runtime

Owns loading authored content into runtime-safe data structures.

Responsibilities:

- cards
- abilities
- characters
- enemies
- enemy intents
- dice templates
- status definitions
- encounter definitions
- reward pools

Recommended direction:

- keep one authored content source of truth
- load content into typed runtime data
- validate content before use
- avoid multiple copied content folders

Clarification:

- "JSON reuse" means cards, dice, abilities, characters, enemies, and encounters should still be data-driven
- the current JSON files can be used as reference or imported if they fit cleanly
- if current JSON shapes are messy, normalize them before making them the new source of truth
- the goal is not to abandon JSON-driven content

### Save System

Owns persistence.

Responsibilities:

- save current run
- load current run
- store unlocked content if meta progression is added later
- store settings

Recommended first version:

- one run save slot
- no cloud sync
- no server persistence

### Presentation

Owns scenes, UI, animation, input, and audio.

Responsibilities:

- map screen
- battle screen
- card hand UI
- dice UI
- enemy intent UI
- resource UI
- status UI
- reward UI
- targeting UX

Presentation should consume events and state snapshots from the local engine. It should not mutate combat state directly.

Presentation is the "client side" of the local client/server split. It is allowed to:

- render state
- gather input
- show affordances
- ask for legal action previews
- send commands through the local client adapter

Presentation is not allowed to:

- directly change deck, hand, dice, status, health, resources, or segment state
- decide whether a command is authoritative
- apply damage or card movement directly

### Test Harness

Owns deterministic verification inside Godot.

Responsibilities:

- fixed decks
- fixed dice rolls
- fixed enemy intents
- fixed encounter setup
- command execution tests
- combat scenario tests
- UI fixture tests where practical

Recommended policy:

- test important state transitions and scenarios
- do not test trivial getters
- do not embed a test server into runtime
- keep tests Godot-local and deterministic

## Proposed Folder Shape

This is a first-pass shape for a new Godot-only V2 project.

```text
godot-pve/
  app/
    boot/
    screens/
      title/
      run_setup/
      map/
      battle/
      rewards/
      game_over/
  local_client/
    battle_gateway/
    command_builder/
    view_state/
  features/
    drawcard/
    playcard/
    rolldice/
    resolveintent/
    resolvedamage/
    applystatus/
    rewards/
  engine/
    boundary/
    run/
    encounter/
    combat/
      deck/
      hand/
      card/
      dice/
      ability/
      resource/
      status/
      damage/
      turn/
      command/
      event/
      snapshot/
    enemy/
      controller/
      intent/
      behavior/
  content/
    cards/
    abilities/
    characters/
    enemies/
    encounters/
    dice/
    rewards/
  presentation/
    battle/
    cards/
    dice/
    enemies/
    hud/
    rewards/
  save/
  tests/
    engine/
    scenarios/
    presentation/
    fixtures/
```

This folder shape is intentionally hybrid:

- feature folders exist for prompt-sized work
- shared engine concepts are centralized
- presentation stays separate from combat rules
- local client adapter stays separate from the battle engine authority
- tests have a first-class home

## Recommended PvE Combat Loop

The PvE combat loop should preserve the simultaneous PvP-style segment model.

The opponent is now computer-controlled, but the battle should still feel like the same game. Both sides roll/select behind the segment structure, then reveal and resolve actions together. The enemy can make hidden shortcut decisions immediately, but the game should present the flow as if both sides are participating in the same simultaneous battle rhythm.

Clarification on "preserve the current segment names":

The current PvP game uses segments such as:

- `ongoing_effects`
- `income`
- `offensive`
- `defensive`
- `damage_resolution`

The names can change later if a better vocabulary helps both PvE and future PvP reuse, but the structure should not change into a simple one-sided player-turn/enemy-turn loop.

Current default:

- keep the existing segment names for now
- only rename segments if the new names make the shared PvE/PvP architecture clearer
- avoid names that imply strict player-turn-then-enemy-turn behavior

Recommended first combat loop:

1. encounter starts
2. `ongoing_effects` resolves start-of-round or persistent effects
3. `income` gives card draw and resources where applicable
4. `offensive` begins
5. player rolls dice and chooses an offensive ability or action
6. enemy secretly chooses its dice result and offensive ability through simplified AI
7. both sides reveal offensive choices together
8. `defensive` begins if the revealed actions create a defense or disruption window
9. player can use cards or effects that modify, break, or respond to dice and results where legal
10. enemy may use simplified defensive or reaction behavior where the encounter supports it
11. `damage_resolution` applies final attacks, defense, statuses, and card-as-health damage
12. win/loss is checked
13. next round begins if combat continues

This still lets us preserve:

- cards
- dice
- abilities
- statuses
- damage resolution
- deck/hand/health ideas
- deterministic command execution
- simultaneous roll/select/reveal timing

The PvE version may simplify:

- multiplayer ready checks
- enemy-side decision intelligence
- enemy-side dice keep/reroll simulation
- enemy-side card choice complexity
- reaction windows that are not needed for the first milestone

Current decision:

- focus on the PvE battle loop first
- do not design broader run progression yet
- preserve simultaneous turns and simultaneous reveal
- keep the PvP segment model unless there is a clear shared PvE/PvP reason to rename it

## What To Preserve From Current Game

Strong candidates to preserve:

- card-as-health as a core identity of the game
- character-defined decks and abilities
- symbolic dice templates
- cards with costs, targets, and effects
- statuses such as blind and evasive if they still fit PvE
- ability upgrades from cards
- deterministic command validation
- event-based combat updates
- JSON-driven authored content as the source of truth after cleanup

## What To Rework For PvE

These need redesign instead of direct carryover:

- player identity becomes one local player profile or run player
- opponent becomes enemy or enemy group
- defensive segment remains part of the simultaneous battle model, but enemy-side choices can be simplified
- reaction windows can remain as a rules concept, but only implement the ones needed by the current battle slice
- network protocol becomes local command/event dispatch
- session startup becomes run and encounter startup
- reconnect becomes save/load or resume-run

Important future-PvP constraint:

- do not place combat rules directly in UI scene scripts
- do not let presentation directly mutate authoritative battle state
- do not let the C++ GDExtension bridge grow beyond thin JSON transport responsibilities
- do not make enemy-only shortcuts leak into player combat rules
- keep player command execution portable enough that a future server could run it
- keep combat content schemas and validation independent from a single UI implementation
- keep command/event tests focused on rules, not presentation

## Local Command/Event Pattern

Even without an external server, the game should keep a command/event style internally.

The local call should look structurally similar to a future server request:

```text
presentation -> local_client -> battle_engine_boundary -> combat_engine
```

Enemy command path:

```text
authority_enemy_controller -> battle_engine_boundary -> combat_engine
```

The return path should also look like a future server response:

```text
combat_engine -> events/snapshot -> local_client -> presentation
```

Example command:

```text
PlayCardCommand
```

Example output events:

```text
CardPlayed
DamageQueued
EnemyDamaged
CardMovedToDiscard
```

Why keep this:

- tests can call commands directly
- UI can consume stable events
- feature work stays narrow
- future replay/debug tooling becomes easier
- if multiplayer ever returns, the engine is less tangled
- if online PvE or MMO-style authority is added later, local calls can be replaced by server requests more cleanly

## Testing Strategy

The detailed Godot/Go story testing policy lives in:

```text
docs/v2-planning/godot_pve/03_story_testing_policy.md
```

For battle-authority work, the base expectation is three automated layers:

- focused Go rule/module tests for meaningful behavior below the JSON boundary
- Go authority command tests for command JSON in and result JSON out
- Godot headless integration tests for the full `Godot -> C++ -> Go -> C++ -> Godot` path

Manual UI spike checks are useful, but they are not the primary proof that a story is complete.

### Engine Tests

Use for pure combat and run logic:

- draw card
- play card
- roll dice
- resolve enemy intent
- apply damage
- apply status
- win/loss check

### Scenario Tests

Use for meaningful battle slices:

- start encounter with fixed deck and fixed enemy
- player draws known hand
- player plays known card
- enemy secretly selects a known result
- both sides reveal through the simultaneous flow
- expected health, zones, statuses, and events are produced

### Presentation Tests

Use where practical:

- incoming combat event updates hand UI
- enemy ready/reveal state is visible
- card can be selected and targeted
- reward choice updates run state

### Avoid

- testing passive getters
- testing Godot node wiring with no behavior
- requiring live networking for any local feature test

## First Milestone Proposal

The first PvE milestone should be a battle-only milestone, not a run-progression milestone.

Recommended first milestone:

- boot a battle screen directly from a fixed encounter
- load a basic fighter or warrior character
- load a goblin fighter or goblin warrior enemy
- create deterministic combat state
- implement dice rolls first
- show basic abilities that the player can try to roll for
- let the enemy choose a simplified hidden roll and ability result
- reveal both sides together
- then add cards and other battle details one piece at a time
- assert the flow with an engine scenario test

This proves the key local architecture without needing:

- map generation
- rewards
- save/load
- complex enemies
- full content migration

Milestone rule:

- load an enemy into a battle
- work through that battle with deterministic details
- build core battle logic one slice at a time, starting with dice rolls and basic abilities
- repeat this with many battles later after the battle core is reliable
- defer map, dungeon, campaign, and encounter-selection structure until after battle gameplay is stable

## Open Questions

These need user decisions before we drill down into detailed stories:

- exact segment vocabulary if we later decide the old names are not the best shared PvE/PvP names
- exact enemy model details: ability percentage tables, hidden dice result generation, and simplified card choice rules
- exact card-as-health damage rules for enemies and player
- cleaner V2 JSON schema design for cards, dice, abilities, characters, enemies, and encounters
- first battle milestone details: exact fighter ability list, goblin ability list, dice templates, and first win/loss condition
