# V2 Game Inventory And Boundary Map

## Purpose

This document captures two things:

1. A durable inventory of what the current game is trying to do.
2. A first-pass V2 boundary map that groups those features into:
   - domain engine
   - session server
   - client
   - content pipeline
   - test harness

The goal is to preserve the game understanding from the deep-dive work, then use that understanding to drive a cleaner V2 architecture.

## Code Map

The main places this inventory was pulled from are:

- `dice-and-destiny-server/dice-and-destiny-game-instance/cmd/gameinstance/main.go`
- `dice-and-destiny-server/dice-and-destiny-game-instance/internal/segment/segment_manager.go`
- `dice-and-destiny-server/dice-and-destiny-game-instance/internal/commands/game_state_adapter.go`
- `dice-and-destiny-client/core/NetworkManager.gd`
- `dice-and-destiny-client/core/PhaseManager.gd`
- `dice-and-destiny-client/UI/Phases/SinglePhase/PersistentGameScreen.gd`
- `content/`

## Current Authored Content Count

Approximate current authored content count:

- 38 abilities
- 25 cards
- 26 characters

## Fundamental Game Inventory

### Match / Session Layer

- The game has a lobby service that accepts client websocket connections and forwards matchmaking requests.
- The game has a matchmaker service that queues players and pairs them into 2-player matches.
- The game assigns positional player identities like `player-1` and `player-2` for actual combat logic.
- The game redirects clients from the lobby websocket to a dedicated game-instance websocket.
- The game-instance currently assumes exactly 2 players in a session.
- The game tracks chosen character IDs during matchmaking and carries them into the live game.
- The game sends a per-player startup payload with player index, player ID, turn context, and resources.

### Core Combat Structure

- The active combat model is a segment-based round loop, not a classic phase loop.
- The current segment set is `ongoing_effects`, `income`, `offensive`, `defensive`, and `damage_resolution`.
- Each segment has internal lifecycle steps: `pre_segment`, `segment_execution`, and `post_segment`.
- Segments can be automatic, player-action driven, or conditional.
- Segments can require all players to be ready before advancing.
- Segments can expose different allowed actions and allowed card types.
- The system supports timers, timeouts, and auto-advance behavior.
- The offensive segment supports iterative looping if revealed actions get invalidated.
- The flow supports reaction windows inside segments, not just between rounds.

### Player Model

- A player has a socket connection, player ID, display name, and selected character.
- A player has per-turn and per-segment readiness state.
- A player has resources, dice state, hand/deck state, status effects, and available abilities.
- A player can pass, continue, roll, keep dice, use abilities, play cards, react, and use evasive effects.
- The game tracks per-player rolls used and per-player dice pools rather than only a global roll count.
- The game tracks per-player defense bonuses and attack state.

### Characters

- Characters are content-driven JSON definitions loaded from disk.
- Characters define class, name, abilities, deck contents, resource configuration, and optional dice configuration.
- Characters can define custom dice templates or dice face configs.
- Characters may be premade or custom.
- Character loading contains conversion logic for multiple historical JSON shapes, which means character data evolved over time.
- The server falls back to default decks and hardcoded default abilities if character loading fails.
- Some classes currently receive default dice configurations if the authored character lacks one.

### Abilities

- Abilities are authored content and also partially duplicated in `game_rules.json`.
- Abilities are split by offensive and defensive use.
- Abilities have segment restrictions, target requirements, cooldowns, costs, and damage/effect data.
- Abilities can create attacks, apply statuses, roll defensive effects, dodge, counterattack, or modify resources.
- The system supports ability upgrades from cards.
- The system supports cards that grant temporary replacement or additional abilities.
- Ability availability can change by segment and by upgrade state.
- The client displays abilities for both player and opponent.

### Cards

- Cards are authored JSON definitions loaded individually from `content/cards`.
- Cards have type, rarity, cost, targeting rules, keywords, art metadata, and one or more effects.
- Cards can cost energy, action points, dice values, or symbol requirements.
- Cards can be offensive, defensive, utility, reaction-like, upgrade-oriented, or targeted upgrade cards.
- Cards can deal damage, heal, draw, discard, block, modify dice, buff, debuff, grant extra rolls, or perform special behavior.
- Cards can target self, opponent, specific targets, or broader target classes.
- Cards can be phase/segment restricted.
- Cards can embed full upgraded ability definitions, not just simple numeric modifiers.
- The client checks card playability based on available resources and dice/symbol state.
- The server validates card play against hand ownership and cost rules before execution.

### Deck / Hand / Health System

- The game uses a card-as-health model.
- A player’s effective health is based on card zones managed by the deck system.
- Damage removes cards from deck, hand, or discard into a removed pile.
- Healing recovers cards back from removed or discard paths.
- The deck manager tracks deck, hand, discard, removed cards, health, and max health.
- The income segment or phase draws cards for both players.
- Round 1 can use a larger starting hand than later rounds.
- The game tracks opponent hand count separately from full opponent hand contents.
- The client attempts to keep the hand synchronized with server truth after damage and income updates.

### Resources

- The game has at least energy and action points as explicit resources.
- Resource values are loaded from rules and or character definitions.
- Turn start resets or regenerates some resources.
- Resource costs gate card plays and some actions.
- The game tracks combo-card history for combo requirements.
- Resource state is broadcast as part of full game-state updates.

### Dice System

- The game has a dice configuration loaded from rules.
- The game supports different dice pools, especially offensive and defensive pools.
- The game supports custom dice counts per character.
- The game supports custom dice templates and symbolic faces, not just numeric faces.
- A dice state includes current values, kept indices, rolls used, max rolls, combinations, face symbols, total symbol counts, pool type, and template ID.
- Players can reroll subsets of dice using kept indices.
- The game detects combinations such as pair and straight-style patterns.
- The game supports symbol-based requirements in addition to numeric and combination requirements.
- The client renders symbolic dice faces from server-provided face and symbol data.
- The game supports temporary extra-roll effects like `grant additional roll`.
- The game tracks roll bonuses and defensive reroll behavior separately from base dice rules.

### Attack / Defense Model

- Offensive actions generate pending attacks with attacker, defender, damage, ability source, flags, and pending effects.
- Defensive play is conditional on whether there are incoming attacks.
- Defensive actions can add defense bonuses, dodges, or counter-damage.
- Some attacks can become undefendable.
- The system tracks multiple damage sources for a single resolution window.
- Damage sources are exposed to clients during damage resolution so the UI can preview pending card loss.
- The game supports counterattacks as first-class attack records, not just side effects.

### Status Effects

- There is a centralized status effect manager on the server.
- Status effects include at least blind and evasive, and likely more.
- Status effects can be queued for application after a segment instead of applying instantly.
- Status effects track duration, value, source ability, and special properties.
- Status effects interact with round advancement and segment hooks.
- Blind has a special check flow during offensive resolution.
- Evasive has token-style behavior during damage response.
- Status effect updates are broadcast to both clients for UI display.

### Reaction System

- The game supports explicit reaction windows.
- Reaction windows have type, timeout, eligible players, per-player responses, continue signals, and contextual event data.
- There is a pre-resolution reaction window after offensive reveal.
- There is a post-resolution reaction window during damage resolution.
- There is a special blind-related reaction window path.
- Players can pass or continue during these windows.
- Reaction results can invalidate previously committed actions and force offensive replanning loops.

### Damage Resolution

- Damage resolution is its own segment.
- The server aggregates pending attacks into defender-targeted damage maps.
- Defensive reactions can still occur after damage is calculated but before final application.
- The game tracks detailed damage sources for evasive selection and pending card preview.
- Final damage application updates health and card zones and then broadcasts health and state changes.
- Damage resolution is guarded against duplicate execution with explicit flags.

### Turn / Round Progression

- The game tracks rounds separately from segment progression.
- The game increments rounds when the full segment cycle completes.
- Turn order and `whose turn` logic still partially exist as legacy concepts.
- Some actions still refer to older turn and phase semantics even though segment mode is now the real flow.
- Passing can be used as a progression mechanism.
- Continue actions are used where all players must acknowledge before moving on.

### Validation / Rules Enforcement

- There is contract and schema validation using `action_contracts.json`.
- There is command-level validation before execution.
- There is content validation for cards, abilities, and characters.
- There is local client-side validation for UX before sending actions.
- The server is still authoritative and performs the real validation.
- The client validation layer knows about phases, segments, and allowed actions.
- The current code contains many compatibility checks because server and client models drifted over time.

### Networking / Protocol

- The client runs two websocket flows: lobby and game-instance.
- The server sends typed messages like `match_found`, `player_info`, `segment_change`, `segment_step_update`, `game_state`, `dice_rolled`, `status_effects_update`, and action-specific result messages.
- The client converts server messages into Godot signals and UI updates.
- Some server responses are general command results, and some are hand-crafted one-off messages extracted from command result payloads.
- The protocol includes full-state broadcasts and event-style delta messages at the same time.
- Message handling currently mixes old phase messages and new segment messages.

### Client UI Systems

- The client has a lobby UI, character selection UI, main UI, persistent HUD, modular phase UIs, and a newer persistent single-phase combat screen.
- The client still contains both modular phase-swapping UI and a more persistent all-in-one game screen architecture.
- The client displays health, resources, turn and round, current segment, hand, opponent hand count, abilities, dice, incoming attacks, and status effects.
- The client supports targeting for cards and clickable target selection.
- The client has separate offensive, defensive, damage-resolution, and income presentation logic.
- The client contains compatibility code to keep old `PersistentUI` elements hidden when `PersistentGameScreen` is active.
- The client uses a large signal-driven architecture with many autoload singletons and `/root` lookups.

### Content / Admin / Tooling

- The server exposes authenticated content and admin APIs for cards, abilities, characters, dice symbols, dice faces, and dice templates.
- The client has in-engine admin interfaces for creating and editing abilities, cards, and dice.
- The system creates backups of edited content.
- There are validators and migration tools for content IDs and structure.
- There are multiple copies of similar content across `config`, `content`, and client-local data and content folders.
- The repo contains many backup, migrated, and test-specific content files.

### Testing / Automation

- There is a very large Godot testing framework embedded into the client runtime.
- The client can auto-start an internal testing server in debug mode.
- The repo contains many shell-based scenario tests and screenshot-driven tests.
- There are Go unit and integration tests, but the game-instance suite is currently broken.
- There are many worktree and test artifacts and investigation docs that capture feature-specific bug hunts.

## Existing Capability Buckets

The current game can be decomposed into these planning buckets:

1. Match and session lifecycle
2. Player identity and seat assignment
3. Character system
4. Ability system
5. Card system
6. Deck, hand, and health system
7. Resource economy
8. Dice engine
9. Attack and defense model
10. Status effects
11. Reaction windows
12. Damage resolution
13. Round and segment progression
14. Validation and contracts
15. Protocol, events, and state sync
16. Client presentation
17. Content authoring pipeline
18. Testing and automation

## V2 Boundary Map

This is the first-pass mapping of those 18 buckets into five V2 boundaries.

### Domain Engine

The domain engine should own all pure game rules and state transitions.

- Character system
- Ability system
- Card system
- Deck, hand, and health system
- Resource economy
- Dice engine
- Attack and defense model
- Status effects
- Reaction windows
- Damage resolution
- Round and segment progression

The domain engine should also own:

- deterministic command execution
- domain events
- rule validation that depends on game state
- no websocket code
- no client code
- no JSON transport formatting

### Session Server

The session server should own all runtime session coordination around the domain engine.

- Match and session lifecycle
- Player identity and seat assignment
- Protocol, events, and state sync

The session server should also own:

- lobby and match flow
- room lifecycle
- connection management
- authentication if used
- player join and reconnect behavior
- translating transport messages into engine commands
- translating domain events into outbound messages

The session server should not own:

- authored game rules
- client rendering logic
- content editing tools

### Client

The client should own all player-facing presentation and local interaction.

- Client presentation

The client should also own:

- screen and component state
- UI rendering of cards, dice, statuses, resources, and previews
- input handling and targeting UX
- local optimistic affordances where useful
- interpreting stable protocol contracts from the session server

The client should not own:

- authoritative game rules
- duplicated engine logic
- content authoring

### Content Pipeline

The content pipeline should own all authored game data and content tooling.

- Content authoring pipeline

The content pipeline should also own:

- abilities, cards, characters, dice templates, symbols
- schema validation and linting for authored data
- migration tools
- import and export tools
- build or publish steps that generate read-only runtime assets

The content pipeline should define one source of truth for authored content.

### Test Harness

The test harness should own deterministic verification across all other boundaries.

- Testing and automation
- validation and contracts

The test harness should include:

- domain scenario tests for rule execution
- server scenario tests for player connections and protocol behavior
- protocol contract tests for exact JSON message shapes
- client fixture-based tests for UI and state updates
- testkit utilities such as fixed decks, fake clocks, fake RNG, and fake clients

The test harness should not be embedded into the default runtime architecture of the shipped client.

## Feature-To-Boundary Matrix

### Match and session lifecycle

- Primary boundary: session server
- Supporting boundary: test harness

### Player identity and seat assignment

- Primary boundary: session server
- Supporting boundary: test harness

### Character system

- Primary boundary: domain engine
- Supporting boundaries: content pipeline, test harness

### Ability system

- Primary boundary: domain engine
- Supporting boundaries: content pipeline, test harness

### Card system

- Primary boundary: domain engine
- Supporting boundaries: content pipeline, test harness

### Deck, hand, and health system

- Primary boundary: domain engine
- Supporting boundary: test harness

### Resource economy

- Primary boundary: domain engine
- Supporting boundary: test harness

### Dice engine

- Primary boundary: domain engine
- Supporting boundaries: content pipeline, test harness

### Attack and defense model

- Primary boundary: domain engine
- Supporting boundary: test harness

### Status effects

- Primary boundary: domain engine
- Supporting boundary: test harness

### Reaction windows

- Primary boundary: domain engine
- Supporting boundary: test harness

### Damage resolution

- Primary boundary: domain engine
- Supporting boundary: test harness

### Round and segment progression

- Primary boundary: domain engine
- Supporting boundary: test harness

### Validation and contracts

- Primary boundaries: domain engine, content pipeline, test harness
- Supporting boundary: session server

### Protocol, events, and state sync

- Primary boundary: session server
- Supporting boundaries: client, test harness

### Client presentation

- Primary boundary: client
- Supporting boundary: test harness

### Content authoring pipeline

- Primary boundary: content pipeline
- Supporting boundary: test harness

### Testing and automation

- Primary boundary: test harness
- Supporting boundaries: domain engine, session server, client, content pipeline

## V2 Boundary Responsibilities Summary

### Domain Engine Owns

- gameplay truth
- rules
- state transitions
- deterministic outcomes
- domain events

### Session Server Owns

- players
- sessions
- transport
- command ingress
- event egress

### Client Owns

- rendering
- interaction
- local UX state

### Content Pipeline Owns

- authored content
- schemas
- publishing runtime content artifacts

### Test Harness Owns

- deterministic test fixtures
- scenario orchestration
- contract verification
- regression protection

## Suggested Next Planning Step

The next useful document should turn this boundary map into a concrete V2 development model:

- how features are sliced into small prompts
- how server-first stories are built before client work
- how deterministic tests are structured
- how each boundary exposes narrow funnel points

Recommended next doc:

- `docs/architecture/v2_development_workflow.md`

That document should define:

- module boundaries
- prompt format
- story slicing rules
- test layering
- definition of done
- a worked example such as `draw_card`
