# Example Target Content Library

This directory is a complete design example for the target YAML-driven content model.

It is intentionally stored under `docs/` because the target schemas include features that the current Go loaders do not yet implement, including:

- a central symbol catalog
- one required symbol per die face
- dice references inside effect rolls
- status activation/lifecycle flags
- general Offensive requirement tiers and bonuses
- selection-first Defensive abilities
- battle-duration ability modifiers
- separate Offensive and Defensive boards
- enemy Offensive D100 charts

Every reference used by the two combatants resolves to another file in this example library.

The example combatants are:

- `combatants/blade_warden.yaml`: human player starting template
- `combatants/venom_goblin.yaml`: AI enemy definition

The first deterministic full-battle template using this library is:

- `../full-battles/blade_warden_vs_venom_goblin.md`
