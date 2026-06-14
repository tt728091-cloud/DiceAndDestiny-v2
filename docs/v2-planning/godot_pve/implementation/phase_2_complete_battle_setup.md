# Phase 2: Complete Battle Setup

## Status

Implemented June 14, 2026.

## Completed Behavior

Battle setup now preserves complete participant combat state:

- participant instance ID and reusable definition ID
- controller type
- character or enemy ID, display name, and class
- starting, maximum, and current resource values
- health model and maximum health
- decklist composition
- ordered deck, hand, discard, and permanently removed card zones
- dice loadout and referenced dice definitions
- ability loadout
- status instances and stacks
- tokens and values
- roll automation preferences

Mutable run-player JSON was expanded to carry the same data. Schema-version-1
saves validate complete character, resource, health, decklist, card-zone, dice,
ability, status, token, and preference state. The narrow schema-version-0 shape
remains loadable for compatibility.

Run-player setup now accepts an empty draw deck when at least one health card
remains in hand or discard. It rejects only states with no remaining health
cards across deck, hand, and discard.

Enemy definitions are loaded from strict YAML. Each enemy descriptor creates a
fresh mutable actor from its immutable definition. Repeated definitions in one
battle receive independent card, status, token, dice, and loadout slices.
Starting enemy status instance IDs are scoped by actor instance ID.

`FileParticipantAssembler` loads:

```text
player definition ID -> mutable run-player JSON
enemy definition ID  -> immutable enemy YAML
```

It merges one human player and one or more AI enemies into the existing
`BattleSetup`, validates referenced cards, abilities, and dice, and deduplicates
identical shared dice definitions.

The default exported authority now uses this assembler and the Phase 1
repository-backed lifecycle. Content and run-state roots may be overridden with:

```text
DICE_AND_DESTINY_CONTENT_ROOT
DICE_AND_DESTINY_RUN_STATE_ROOT
```

Viewer snapshots remain filtered:

- all viewers receive public metadata, resource/health summaries, controller,
  statuses, tokens, and loadout counts
- only the owning viewer receives hand card IDs, decklist composition, dice
  loadout IDs, ability IDs, and roll preferences
- opposing hand IDs, deck order, and hidden planning dice remain hidden

## Files Changed

### Authoritative State and Setup

- `dice-and-destiny-server/internal/battle/state/battle.go`
- `dice-and-destiny-server/internal/battle/setup/character_combat_sheet.go`
- `dice-and-destiny-server/internal/battle/setup/run_player.go`
- `dice-and-destiny-server/internal/battle/setup/enemy_definition.go`
- `dice-and-destiny-server/internal/battle/resource/energy.go`

### Content and Save Loading

- `dice-and-destiny-server/internal/content/character_combat_sheet.go`
- `dice-and-destiny-server/internal/content/enemy_definition.go`
- `dice-and-destiny-server/internal/save/run_player.go`
- `dice-and-destiny-server/content/characters/mock_paladin.yaml`
- `dice-and-destiny-server/content/enemies/mock_goblin.yaml`
- `dice-and-destiny-server/save/run_players/current_run_player.json`

### Lifecycle and Viewer Boundary

- `dice-and-destiny-server/internal/battle/participant_assembler.go`
- `dice-and-destiny-server/internal/battle/authority.go`
- `dice-and-destiny-server/internal/battle/snapshot/snapshot.go`

### Tests

- `dice-and-destiny-server/internal/battle/participant_assembler_test.go`
- `dice-and-destiny-server/internal/battle/setup/character_combat_sheet_test.go`
- `dice-and-destiny-server/internal/battle/setup/run_player_test.go`
- `dice-and-destiny-server/internal/battle/state/battle_test.go`
- `dice-and-destiny-server/internal/battle/income/reward_test.go`
- `dice-and-destiny-server/internal/content/character_combat_sheet_test.go`
- `dice-and-destiny-server/internal/content/enemy_definition_test.go`
- `dice-and-destiny-server/internal/save/run_player_test.go`

## Tests and Results

Focused tests cover:

- complete character-sheet conversion
- complete mutable run-state loading
- empty draw deck with health cards in hand or discard
- strict enemy YAML loading
- deep copying from setup into battle state
- one player plus two instances of the same enemy definition
- independent mutable state and status instance IDs for repeated enemies
- viewer-safe loadout visibility
- default exported authority production assembly
- Phase 1 checkpoint persistence and progression to human input

Commands:

```bash
cd dice-and-destiny-server
go test ./...
```

Result: pass.

```bash
git diff --check
```

Result: pass.

## Architectural Decisions

### Run State Is Mutable; Enemy Content Is Immutable

The player is loaded from mutable run state so acquired cards, card damage,
resources, loadouts, statuses, tokens, and preferences carry between battles.
Enemies are reconstructed from YAML for every battle.

### Instance IDs and Definition IDs Remain Separate

Participant descriptors continue to own battle instance IDs and reusable
definition IDs. The authority applies those descriptors after assembly, so
loaded content cannot replace encounter-selected identity or controller type.

### Decklist and Card Zones Are Separate

Decklist composition records the complete owned card population. Ordered card
zones record current mutable health and draw state. Legacy saves without a
decklist derive one from all four zones.

### State Shapes Do Not Implement Behavior

Statuses, tokens, abilities, and roll preferences are preserved but do not
resolve effects. This keeps Phase 2 limited to loading, setup, persistence, and
viewer-safe representation.

### Existing Energy Access Remains Compatible

`ActorState.EnergyPoints` remains as a synchronized compatibility alias for
existing income and engine helpers. `ActorState.Resources.EnergyPoints` is the
complete Phase 2 resource location. Phase 3 or a dedicated cleanup should move
remaining helpers to `Resources` and remove the alias once no callers depend on
it.

## Deviations and Remaining Gaps

- Mutable run state remains JSON rather than YAML. The required data is
  preserved; format migration was not necessary for Phase 2.
- Default content paths fall back to repository-relative paths derived from the
  compiled source location. Packaged builds should set the two root environment
  variables or replace this with adapter-provided configuration.
- The repository remains in-memory and does not survive process restart.
- Enemy definitions use existing placeholder card, ability, and dice content.
- Status and token definitions are not loaded or behaviorally validated yet.
- The existing default income reward still targets the `player` actor ID.
  Encounter callers should continue using `player` until income rewards are
  made participant-aware.
- No reaction windows, shared planning, damage resolution, or status behavior
  were added.

## Phase 3 Prerequisites and Recommendations

Before implementing gameplay reactions:

1. Treat the expanded actor state as the persisted source for all nested
   resolutions; do not reload participant files during a battle.
2. Add generic persisted resolution, proposal-batch, and interaction-window
   state keyed by stable IDs.
3. Include source actor ID, source definition ID, segment, phase, stage,
   iteration, eligible actors, required actors, and suspended checkpoint in
   every window.
4. Keep hidden commitments authoritative and build viewer filtering from
   visibility policy rather than exposing commitment structs directly.
5. Add deep-copy tests for every new map, slice, and nested pointer because the
   repository depends on `Battle.Clone` for checkpoint isolation.
6. Make default income participant-aware before supporting arbitrary human
   instance IDs beyond `player`.
7. Migrate resource mutation callers to `ActorState.Resources` before adding
   costs or resource proposals, then remove the compatibility energy alias.
8. Prove pause, save, load, and resume behavior with fake nested resolutions
   before adding poison, offensive planning, or defensive planning.
