# `battle_v1` content contract

Cards, abilities, and statuses have different activation envelopes, but their
effects use the same recursive `operations` language. Adding a definition must
not require a Go or Godot ID branch.

- Cards define cost, `play.playable_during`, targeting, destination, and
  `operations`.
- Offensive ability tiers and defensive `resolution.operations` use that same
  operation list.
- Status triggers define their checkpoint and use that same operation list.
- Dice and symbols are referenced by ID and are published to the client with
  the pinned battle catalog.

The shared operation kinds are `noop`, `deal_damage`, `prevent_damage`,
`scale_damage`, `apply_status`, `remove_status`, `remove_status_stack`,
`gain_resource`, `draw_cards`, `modify_die`, `apply_ability_modifier`,
`adjust_max_rolls`, `cancel_source`, and `roll_dice`.

`roll_dice` has the same shape everywhere:

```yaml
- id: example_roll
  type: roll_dice
  target: selected_targets
  dice_id: standard_d6
  dice_count: 1
  reaction_window: {opens: false}
  outcomes:
    - {faces: [1, 2, 3, 4], operations: [{type: deal_damage, target: selected_targets, amount: 2}]}
    - {faces: [5], operations: [{type: noop}]}
    - {faces: [6], operations: [{type: apply_status, target: selected_targets, status_id: bleed, stack_count: 1}]}
```

Outcome faces must be non-overlapping and cover every face of the referenced
die. Nested outcome operations are validated and interpreted recursively.
Definition IDs are lowercase snake case. Card, ability, and status display
names are unique across those categories. Unknown YAML fields and references
are rejected when the library loads.

The YAML-only proof definitions are `alchemists_gamble`, `fateful_strike`, and
`volatile_poison`. The authority integration test composes all three through a
real battle without definition-specific implementation code.
