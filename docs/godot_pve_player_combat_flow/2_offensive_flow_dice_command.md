# Story 2: Dice Roll Domain And Command

## Purpose

Create the first real `roll_dice` command path for v2 without making the engine own dice behavior.

Dice should follow the same ownership pattern as cards:

- card package owns card mechanics such as drawing and returns card events
- dice package owns dice mechanics such as rolling, face lookup, symbol lookup, kept dice rules, roll counters, and returns dice events
- engine and segment flows only decide whether an actor currently has permission to roll
- authority only parses JSON, routes commands, and returns viewer-safe results

This story replaces the earlier "offensive-only" framing. Offensive is the most common place dice are rolled, but it is not the only legal dice context. Defensive abilities, cards, and future effect hooks can also require rolls.

## Old Repo Research Notes

Reference repo:

```text
/Users/daddymere/games/Dice-and-Destiny
```

Useful old files reviewed:

```text
dice-and-destiny-server/dice-and-destiny-game-instance/internal/commands/roll_dice_command.go
dice-and-destiny-server/dice-and-destiny-game-instance/internal/commands/use_ability_command.go
dice-and-destiny-server/dice-and-destiny-game-instance/internal/commands/play_card_command.go
dice-and-destiny-server/dice-and-destiny-game-instance/internal/commands/game_state_adapter.go
dice-and-destiny-server/dice-and-destiny-game-instance/internal/dice/symbols.go
dice-and-destiny-server/dice-and-destiny-game-instance/internal/dice/template_loader.go
dice-and-destiny-server/dice-and-destiny-game-instance/internal/cards/loader.go
dice-and-destiny-server/dice-and-destiny-game-instance/internal/effects/effect_interpreter.go
```

The old setup proves these rules should carry forward:

- offensive and defensive segments both allow dice rolling
- defensive abilities can create smaller roll pools such as `1d6`
- cards can require current dice values or symbols to be playable
- cards and effects can change the number of allowed rolls, such as additional defensive roll attempts
- dice faces may have zero, one, or many symbols
- ability validation depends on dice combinations and symbol counts
- dice state must track current values, kept dice, rolls used, max rolls, pool type, face symbols, symbol counts, and source context

The old setup also mixed too much into `roll_dice_command.go`: phase checks, status effects, character dice lookup, face symbol fallback, defensive special cases, roll bonus logic, and response shaping all lived in the command path. V2 should preserve the gameplay concepts but separate responsibilities more cleanly.

## Design Context

The target command path is:

```text
authority JSON boundary
-> command parser
-> engine
-> current segment flow / pending roll request validation
-> dice package
-> events + snapshot
```

`authority.go` should not know whether `roll_dice` belongs to offensive, defensive, a card effect, or a future hook.

The engine should know only enough to route the command and ask the current battle state whether this actor has an active roll opportunity. The engine should not know how many faces a die has, how symbols are mapped, how combinations are detected, how kept dice are applied, or how roll counts are incremented.

Reference:

```text
docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md
docs/godot_pve_player_combat_flow/1_character_combat_sheet_yaml.md
```

## Core Model

Add a battle-side dice state model that is mutable run combat state, not character creation data.

The character sheet from Story 1 defines the actor's dice loadout:

```yaml
dice_loadout:
  - dice_id: Mock D6
    count: 5
```

`dice_id` is the unique dice name from the dice content file, matching the card ID pattern where the unique card name is the ID.

The dice content file defines the die:

```yaml
id: Mock D6
name: Mock D6
faces:
  - face: 1
    value: 1
    symbols: []
  - face: 2
    value: 2
    symbols: []
  - face: 3
    value: 3
    symbols: []
  - face: 4
    value: 4
    symbols: []
  - face: 5
    value: 5
    symbols: []
  - face: 6
    value: 6
    symbols: []
```

Battle state stores the current dice roll state for an actor. Suggested shape:

```go
type DiceState struct {
	CurrentRoll *RollState
}

type RollState struct {
	RequestID    string
	ActorID      string
	Segment      segment.Segment
	Pool         RollPool
	SourceType   RollSourceType
	SourceID     string
	Dice         []RolledDie
	KeptIndices  []int
	RollsUsed    int
	MaxRolls     int
	Combinations []string
	SymbolCounts map[string]int
	Complete     bool
}

type RolledDie struct {
	Index   int
	DieID   string
	Face    int
	Value   int
	Symbols []string
}
```

The exact Go names can follow the codebase, but the concepts above need to exist so later stories do not have to redesign dice state.

## Pending Roll Requests

Do not validate `roll_dice` by saying "current segment must be offensive."

Validate it by saying "this actor has an active roll request that allows rolling right now."

Suggested shape:

```go
type RollRequest struct {
	ID         string
	ActorID    string
	Segment    segment.Segment
	Pool       RollPool
	SourceType RollSourceType
	SourceID   string
	DiceLoadout []DiceLoadoutEntry
	MaxRolls   int
	Required   bool
	Complete   bool
}
```

Initial roll pools:

```text
offensive
defensive
effect
card
```

Initial source types:

```text
segment
ability
card
status
system
```

Examples:

- offensive segment creates a default offensive roll opportunity for the active actor
- defensive segment creates a defensive roll opportunity when an actor has an incoming defendable attack and chooses a defensive ability that requires a roll
- a card can later create a card/effect roll opportunity
- a status effect can later create a required roll opportunity before a segment can complete

For this story, implement the minimum needed to support offensive rolling and one test-only non-offensive pending roll. The API and state should not prevent defensive/card/effect rolls.

## Roll Command Contract

The command should identify the actor and, when needed, the active roll request.

Suggested JSON:

```json
{
  "action": "roll_dice",
  "actor_id": "player",
  "request_id": "roll-player-offensive-1",
  "reroll_indices": [0, 2, 4]
}
```

Rules:

- `request_id` may be optional only if the actor has exactly one active roll request
- reject if the actor has no active roll request
- reject if the request belongs to another actor
- reject if the request is complete
- reject if `rolls_used >= max_rolls`
- reject invalid reroll indices
- reject rerolling kept dice once keep support exists
- roll all dice on the first roll when `reroll_indices` is empty or omitted
- on later rolls, empty `reroll_indices` means reroll all non-kept dice

## Dice Package Responsibilities

The dice package owns:

- loading or receiving dice definitions from `content/dice/<id>.yaml`
- expanding actor dice loadout entries into concrete dice instances
- validating dice definitions
- validating roll requests from the dice perspective
- random face selection through an injectable random source
- deterministic roll source for tests
- mapping rolled faces to values and symbols
- calculating symbol counts
- calculating combinations such as pairs, straights, five of a kind
- updating dice roll state
- returning `dice_rolled` domain events

The dice package should expose a small command-facing function, similar in spirit to `card.DrawCards`:

```go
events, err := dice.Roll(battle, requestID, actorID, rerollIndices, dice.WithRandomSource(source))
```

The exact signature can differ, but the ownership should not: engine calls dice; dice mutates dice state and returns events.

## Engine And Flow Responsibilities

Engine and flows own:

- routing `roll_dice` from parsed command to current battle command handling
- creating the default offensive roll request when the offensive segment starts or when the actor first becomes eligible to roll
- allowing non-offensive roll requests when state says they exist
- rejecting command attempts that do not match an active roll request
- appending returned dice events to the command result
- producing the viewer-safe snapshot/result

Engine and flows do not own:

- random number generation details
- die face lookup
- symbol counting
- combination detection
- dice content parsing
- hard-coded dice counts
- offensive vs defensive dice pool internals

## Event Contract

Add a domain event for rolled dice.

Suggested event fields:

```go
Type:       "dice_rolled"
ActorID:    actorID
Segment:    currentSegment
RequestID:  requestID
Pool:       "offensive"
SourceType: "segment"
SourceID:   "offensive"
Dice:       []RolledDie
RolledIndices: []int
RollsUsed:  1
MaxRolls:   3
RollsRemaining: 2
Combinations: []string
SymbolCounts: map[string]int
```

The event package can define `event.NewDiceRolled(...)`, but the dice package should call it and return the event, the same way the card package calls `event.NewCardsDrawn(...)`.

Viewer filtering can initially expose player and enemy dice publicly for the simple PvE test path unless existing snapshot policy requires hidden enemy dice. If hidden dice are needed, add the fields now but cover hidden/reveal behavior in a later story.

## Scope

- Add or flesh out battle dice domain models.
- Add dice content model support for simple YAML dice definitions.
- Add a default `Mock D6` dice definition if Story 1 has not already created it.
- Add battle state support for actor dice state and active roll requests.
- Add command parsing for `roll_dice`.
- Route `roll_dice` through engine/flow/domain packages.
- Implement deterministic dice rolling for tests.
- Emit `dice_rolled` events from the dice/domain path.
- Remove any remaining spike dice behavior from `authority.go` if it still exists.
- Update snapshots so the current dice roll state is visible to tests.

## Out Of Scope

- UI dice rendering.
- Godot input changes.
- C++ bridge changes.
- keep dice command.
- ability selection and ability resolution.
- card play resolution.
- defensive ability resolution.
- damage from dice results.
- hidden enemy dice and reveal timing.
- status effects that create roll requests.
- full symbol-heavy character dice.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add focused dice tests proving:

- dice rolling can be deterministic in tests
- dice results stay within allowed face values
- dice results include symbols from the dice definition, even when the initial mock symbols are empty arrays
- symbol counts are calculated from rolled faces
- combinations are calculated from rolled values
- roll counters update correctly
- max rolls are enforced

Add engine/domain tests proving:

- `roll_dice` is accepted for an active offensive roll request
- `roll_dice` is not rejected merely because the current segment is not offensive
- `roll_dice` is accepted for a test defensive/effect roll request outside offensive
- `roll_dice` is rejected when the actor has no active roll request
- `roll_dice` is rejected when the request belongs to another actor
- `roll_dice` is rejected when max rolls are exhausted
- `dice_rolled` is emitted through the engine result
- the dice package creates the dice event instead of engine constructing dice event details
- authority JSON test proves the command/result contract without putting gameplay validation in `authority.go`

## Definition Of Done

- `roll_dice` is handled by engine/flow/domain packages.
- Dice mechanics live in the dice package.
- Engine remains a thin orchestrator and does not know dice face/symbol/combo rules.
- `authority.go` does not contain dice gameplay logic.
- The story supports offensive rolling now and does not block defensive/card/effect roll requests later.
- Go tests pass.
- No C++ bridge changes are made.
- No UI changes are made.
