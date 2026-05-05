# Story 11E: Income Reward Model

## Purpose

Replace the Story 11 hardcoded income draw count with an explicit income reward model that can later support card draw and action points.

Story 11 currently has:

```go
incomeDrawActorID = "player"
incomeDrawCount   = 1
```

That proves the hook, but real income may grant different combinations of cards and action points.

## Design Context

Income flow owns when income rewards happen. Card package owns card draw. Future resource package or actor state owns action point changes.

Near-term direction:

```text
IncomeFlow.OnEnter
-> resolve income reward/config for actor
-> card.DrawCards(...)
-> resource/action point rule later
-> return events
```

Do not put income reward decisions in `segment`.

## Before Coding, Read

- `README.md`
- `docs/story-11-income-flow-card-draw-hook/todo.md`
- `docs/v2-planning/godot_pve/03_story_testing_policy.md`
- `docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md`
- `stories/godot_pve/11a_battle_setup_player_combat_state.md`
- `stories/godot_pve/11d_draw_with_discard_reshuffle.md`
- `dice-and-destiny-server/internal/battle/engine/default_flows.go`
- `dice-and-destiny-server/internal/battle/engine/engine_test.go`
- `dice-and-destiny-server/internal/battle/state/battle.go`
- `dice-and-destiny-server/internal/battle/card/draw.go`
- `dice-and-destiny-server/internal/battle/event/event.go`

Assume the prior deck setup and draw behavior stories are implemented. If current code differs, inspect locally and adapt.

## Scope

- Add an explicit income reward/config shape. Possible starting shape:

```go
type IncomeReward struct {
	ActorID      string
	DrawCards    int
	ActionPoints int
}
```

- Wire `IncomeFlow.OnEnter` to use an income reward/config rather than hardcoded constants.
- For this story, action points may be added as minimal actor state if needed, but do not build a full resource system unless the code shape requires a tiny one.
- Keep card draw behavior delegated to the card package.

## Out Of Scope

- Full player choice UI.
- Godot commands for selecting income choices.
- Full resource economy.
- Balancing values.
- Enemy income.
- PvP simultaneous income choice flow.
- Godot, C++, or UI changes.

## Requirements

- Income behavior is configured through explicit data, not hardcoded draw constants.
- A reward with `DrawCards: 0` is valid.
- A reward with action points but no draw is valid if action point state is introduced.
- Negative draw/resource values are rejected at the income reward/config boundary, not hidden inside card draw as a gameplay choice.
- Existing Story 11 income draw behavior can still be represented by config.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add or update focused engine/domain tests proving:

- income config of draw 1 draws one card
- income config of draw 0 does not call or perform card draw
- income config with action points updates minimal resource state if implemented
- invalid negative income reward is rejected clearly
- segment package remains free of income/card/resource imports

## Definition Of Done

- Income reward behavior is explicit and configurable.
- Hardcoded Story 11 income draw constants are removed or reduced to test/default setup only.
- Draw-zero income is valid.
- `go test ./...` passes.

