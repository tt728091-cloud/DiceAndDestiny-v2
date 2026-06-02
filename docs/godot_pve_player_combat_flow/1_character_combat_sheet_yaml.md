# Story 1: Character Combat Sheet YAML

## Purpose

Introduce a character combat sheet loaded from YAML so battle setup can start from authored character loadout data instead of only a hardcoded or card-zone-only save shape.

The sheet is a character creation/loadout document. It is not mutated during combat. It references canonical card, ability, and dice content files by ID/name, then records the character's class, deck composition, dice loadout, ability loadout, and starting combat values.

## Design Context

The current setup path already supports loading minimal run player state and converting it into battle setup:

```text
save file
-> loaded run/player state
-> setup.BattleSetupFromRunPlayer
-> state.NewBattleFromSetup
```

That path currently focuses on card zones. The next step is to make the source data look like an actual character combat sheet:

```text
character combat sheet YAML
-> content references by ID/name
-> loaded character combat sheet
-> battle setup
-> mutable battle state
```

This story owns the authored sheet and referenced content. Later setup stories own expanding the sheet into mutable battle state:

```text
decklist
-> deterministic ordered deck / shuffled deck
-> hand, discard, removed zones
-> current energy, health, statuses, tokens
```

The old repo had a useful content idea but a problematic runtime shape: cards, abilities, dice, characters, and validation data were split across several JSON files and copied between locations. This story should keep the useful idea and avoid the split-brain behavior.

Rules for this v2 story:

- Cards, abilities, and dice are separate content types.
- Each content item lives in its own YAML file.
- Content ID is the unique display name for that item.
- Card names must be unique.
- Ability names must be unique.
- Dice names must be unique.
- The character combat sheet references content IDs/names; it does not embed full card, ability, or dice definitions.
- Use v2 segment names in new content: `ongoing_effects`, `income`, `offensive`, `defensive`, `damage_resolution`.
- Do not use old phase names like `offensive_roll` or `defensive_roll` in new v2 content.

## Cards And Decklist

The character sheet should store deckbuilding composition only:

```text
decklist = card IDs and counts that define the run deck composition
```

Do not include ordered mutable combat zones in the character sheet. Battle setup should expand the decklist into mutable zones later:

```text
cards.deck
cards.hand
cards.discard
cards.removed
```

Card health depends on card zones:

```text
max health = starting deck size from decklist total
current health = deck + hand + discard card count
removed cards = damage / missing health, recoverable by healing rules later
```

For the character sheet, only max health is relevant. Max health is derived from the decklist total. Current health belongs to battle/run state and should not be stored in the character creation sheet.

## Abilities

Abilities are not cards. Cards are deck objects that can be drawn and played. Abilities are character actions available to the actor in combat.

The player sheet should list the ability IDs/names available to the actor for this combat loadout. Each ability content file owns its own phase/segment restrictions, such as `offensive` or `defensive`.

Do not duplicate ability phase grouping in the player sheet for this story. Later stories may add a separate larger inventory/equipped-board distinction if needed.

## Dice

Dice follow the same reference pattern as cards:

```text
content/dice/<dice name>.yaml = canonical die definition
character sheet dice_loadout = dice ID/name + count
```

The die content file owns:

- unique die ID/name
- die type, such as `d6`
- side count
- face values
- symbols on each face

For the first mocked player, the sheet should reference one blank `Standard D6` die definition with count `5`. Its faces are values 1 through 6 and each face has `symbols: []`.

## Before Coding, Read

- `README.md`
- `docs/v2-planning/godot_pve/00_high_level_architecture.md`
- `docs/v2-planning/godot_pve/01_portable_go_authority_architecture.md`
- `docs/v2-planning/godot_pve/03_story_testing_policy.md`
- `docs/story-11-income-flow-card-draw-hook/todo.md`
- `stories/godot_pve/11g_build_battle_setup_from_run_player_state.md`
- `stories/godot_pve/11h_load_player_run_state_from_save_for_battle_setup.md`
- `dice-and-destiny-server/internal/battle/setup/run_player.go`
- `dice-and-destiny-server/internal/save/run_player.go`
- `dice-and-destiny-server/internal/battle/state/battle.go`
- Reference only, do not copy wholesale:
  - `/Users/daddymere/games/Dice-and-Destiny/docs/architecture/complete_data_flow_analysis.md`
  - `/Users/daddymere/games/Dice-and-Destiny/content/characters/default.json`
  - `/Users/daddymere/games/Dice-and-Destiny/content/characters/moon-elf-test-better-d.json`
  - `/Users/daddymere/games/Dice-and-Destiny/content/abilities/basic_attack.json`
  - `/Users/daddymere/games/Dice-and-Destiny/content/abilities/defensive_stance.json`
  - `/Users/daddymere/games/Dice-and-Destiny/content/dice_symbol/templates/standard_d6.json`
  - `/Users/daddymere/games/Dice-and-Destiny/content/dice_symbol/symbols/symbols.json`

Assume Stories 11A through 11H are implemented. If the current code differs, inspect what exists locally and adapt to the repo's actual state.

## Scope

- Add a YAML character combat sheet format.
- Add a loader that reads the YAML character combat sheet from disk.
- Keep the current JSON loader if it already exists; do not break existing Story 11H tests.
- Add a minimal content directory structure for referenced content IDs:
  - `dice-and-destiny-server/content/cards/`
  - `dice-and-destiny-server/content/abilities/`
  - `dice-and-destiny-server/content/dice/`
- Create placeholder content files for the mocked character sheet:
  - a few placeholder card files whose counts total 20 cards
  - 3 offensive ability files
  - 1 defensive ability file
  - 1 blank `Standard D6` die file
- The loader should validate that card IDs referenced by the sheet exist in the loaded card content set.
- The loader should validate that ability IDs referenced by the sheet exist in the loaded ability content set.
- The loader should validate that dice IDs referenced by the sheet exist in the loaded dice content set.
- Add in-memory Go shapes for the character combat sheet.
- Preserve decklist composition.
- Include actor/character metadata, class, starting resources, derived max health, dice loadout, and ability loadout in the loaded sheet.
- Explicitly exclude mutable combat zones from the character sheet.
- Do not make the engine resolve cards, abilities, dice requirements, damage, healing, or statuses yet.

## Proposed Character Sheet YAML Shape

Names may change to better match the codebase, but keep the concepts intact.

```yaml
schema_version: 1
actor_id: player

character:
  id: Mock Paladin
  name: Mock Paladin
  class: paladin

resources:
  starting_hand_size: 4
  max_hand_size: 7
  starting_energy_points: 2
  max_energy_points: 10

health:
  model: card_zones
  max_health: 20

decklist:
  - card_id: Mock Strike
    count: 8
  - card_id: Mock Guard
    count: 6
  - card_id: Mock Focus
    count: 6

dice_loadout:
  - dice_id: Standard D6
    count: 5

abilities:
  - Mock Smite
  - Mock Radiant Chain
  - Mock Final Judgment
  - Mock Guarding Light
```

## Proposed Placeholder Card Shape

Each card should live in its own YAML file under `dice-and-destiny-server/content/cards/`.

The card `id` is the unique card name.

```yaml
schema_version: 1
id: Mock Strike
name: Mock Strike
type: placeholder
cost:
  energy_points: 0
phase_restrictions: []
effects:
  - type: noop
```

## Proposed Placeholder Ability Shape

Each ability should live in its own YAML file under `dice-and-destiny-server/content/abilities/`.

The ability `id` is the unique ability name. The ability content file owns its phase/segment restrictions.

```yaml
schema_version: 1
id: Mock Smite
name: Mock Smite
type: offensive
phase_restrictions:
  - offensive
dice_requirement:
  kind: small_straight
cost:
  energy_points: 0
requires_target: true
effects:
  - type: noop
```

Example starting abilities:

- `Mock Smite`: offensive, small straight
- `Mock Radiant Chain`: offensive, large straight
- `Mock Final Judgment`: offensive, five sixes
- `Mock Guarding Light`: defensive, no requirement or defensive placeholder requirement

## Proposed Placeholder Dice Shape

Each die should live in its own YAML file under `dice-and-destiny-server/content/dice/`.

The dice `id` is the unique dice name.

```yaml
schema_version: 1
id: Standard D6
name: Standard D6
type: d6
sides: 6
faces:
  - value: 1
    symbols: []
  - value: 2
    symbols: []
  - value: 3
    symbols: []
  - value: 4
    symbols: []
  - value: 5
    symbols: []
  - value: 6
    symbols: []
```

## Possible Go Shape

Exact package and field names may change to match local code.

```go
type CharacterCombatSheet struct {
	SchemaVersion int
	ActorID       string
	Character     CharacterState
	Resources     ResourceState
	Health        HealthState
	Decklist      []DecklistEntry
	DiceLoadout   []DiceLoadoutEntry
	Abilities     []string
}

type CharacterState struct {
	ID    string
	Name  string
	Class string
}

type ResourceState struct {
	StartingHandSize     int
	MaxHandSize          int
	StartingEnergyPoints int
	MaxEnergyPoints      int
}

type HealthState struct {
	Model     string
	MaxHealth int
}

type DecklistEntry struct {
	CardID string
	Count  int
}

type DiceLoadoutEntry struct {
	DiceID string
	Count  int
}

type CardDefinition struct {
	ID                string
	Name              string
	Type              string
	Cost              Cost
	PhaseRestrictions []string
	Effects           []EffectDefinition
}

type AbilityDefinition struct {
	ID                string
	Name              string
	Type              string
	PhaseRestrictions []string
	DiceRequirement   DiceRequirement
	Cost              Cost
	RequiresTarget    bool
	Effects           []EffectDefinition
}

type DiceDefinition struct {
	ID    string
	Name  string
	Type  string
	Sides int
	Faces []DieFace
}

type DieFace struct {
	Value   int
	Symbols []string
}
```

## Out Of Scope

- Card effect resolution.
- Ability selection.
- Ability requirement matching.
- Dice rolling commands.
- Damage and healing behavior.
- Mutable battle card zones.
- Current combat resources.
- Current combat statuses and tokens.
- Defense resolution.
- Enemy setup.
- Encounter setup.
- Godot UI.
- C++ bridge changes.
- Full content authoring tools.
- Deckbuilding UI.
- Save-slot UI.
- Backfilling all old repo cards, abilities, dice, or symbols.
- Dice symbol content loading, because the first mocked die has empty symbols.

## Requirements

- A YAML file can define a character combat sheet.
- The character combat sheet is loaded from disk into an in-memory character combat sheet state.
- The loaded state contains:
  - actor ID
  - character ID, display name, and class
  - starting hand size, default fixture value `4`
  - max hand size
  - starting energy points, default fixture value `2`
  - max energy points
  - card-health model
  - max health equal to decklist total
  - decklist card ID/count entries
  - dice loadout entries by dice ID/count
  - 4 ability IDs
- Cards, abilities, and dice are separate systems.
- Card content files are one file per card ID/name.
- Ability content files are one file per ability ID/name.
- Dice content files are one file per dice ID/name.
- Content IDs are unique names.
- Card names must be unique.
- Ability names must be unique.
- Dice names must be unique.
- Missing card content for a referenced card ID is rejected.
- Missing ability content for a referenced ability ID is rejected.
- Missing dice content for a referenced dice ID is rejected.
- Duplicate content IDs are rejected.
- Decklist entries require positive counts.
- Dice loadout entries require positive counts.
- Ability loadout must refer to known ability IDs.
- Dice loadout must refer to known dice IDs.
- Ability phase restrictions use v2 segment names, starting with `offensive` and `defensive`.
- Card phase restrictions use v2 segment names if present.
- No content file uses old phase names such as `offensive_roll` or `defensive_roll`.
- No action points are present in the combat sheet or new content contracts; energy points are the only first-pass playable resource.
- Missing actor ID is rejected.
- Missing character ID or class is rejected.
- Missing or empty decklist behavior is explicit; prefer rejecting empty decklist for now.
- Invalid dice content is rejected:
  - missing dice ID/name
  - unsupported die type
  - unsupported side count
  - `d6` does not have exactly 6 faces
  - face values outside 1 through side count
  - duplicate face values on one die
  - missing `symbols` arrays
- For Story 1, dice symbols may only be validated as arrays. Symbol definition loading can come later.
- Loader returns copied slices/maps so later mutation of loaded state cannot alias parser internals.
- Existing JSON run-player save behavior continues to pass.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add focused tests proving:

- valid mocked YAML loads actor, character, starting resources, health, decklist, dice loadout, and abilities
- starting hand size loads as `4`
- starting energy points loads as `2`
- decklist count total is `20`
- max health is `20`
- dice loadout loads `Standard D6` count `5`
- `Standard D6` content loads with 6 numeric faces
- all loaded die face symbol lists are empty
- ability IDs load as a flat combat ability loadout
- referenced card IDs must exist in `content/cards`
- referenced ability IDs must exist in `content/abilities`
- referenced dice IDs must exist in `content/dice`
- duplicate card content IDs are rejected
- duplicate ability content IDs are rejected
- duplicate dice content IDs are rejected
- missing actor ID is rejected
- missing character class is rejected
- empty decklist behavior is explicit
- invalid dice content shape is rejected
- old phase names are rejected in new content
- action point fields are rejected in new content
- existing JSON loader tests still pass

Use temporary directories for loader tests when practical. If fixture files are checked in, tests should not depend on absolute user-local paths.

## Definition Of Done

- The repo has an implementation-ready character combat sheet YAML contract.
- The repo has a checked-in mocked character combat sheet fixture.
- The repo has minimal per-card, per-ability, and per-dice content fixtures referenced by that sheet.
- Loader code can parse and validate the YAML and referenced content IDs.
- Loaded state preserves decklist composition.
- Loaded state preserves dice references by ID/count instead of embedding dice definitions in the player sheet.
- Loaded state treats cards, abilities, and dice as separate content systems.
- This story does not implement gameplay resolution for cards, abilities, dice, statuses, tokens, damage, or healing.
- `go test ./...` passes.
- No Godot, C++, or UI files are changed.
