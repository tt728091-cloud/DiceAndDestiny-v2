# Dice and Destiny Godot Client Layout

This Godot project owns presentation, input, scenes, local client adapters, and headless Godot integration tests.

The current spike files still live in `scripts/` and `scenes/`:

```text
scripts/battle_authority.gd
scripts/go_gdextension_battle_authority.gd
scripts/verify_battle_authority.gd
scripts/battle_spike.gd
scenes/battle_spike.tscn
```

As production gameplay grows, new work should move into the project structure below instead of adding more unrelated files to the flat `scripts/` folder.

Planned ownership:

```text
app/           top-level boot and screen routing
local_client/  Godot-side authority gateways, command builders, and render-ready view state
features/      prompt/story-sized client features
engine/        local Godot engine code only if we choose a Godot authority path
content/       client-side authored content or imported content references
presentation/  scenes, controls, animation, input, and UI rendering
save/          local save/load client code
tests/         deterministic Godot tests, scenarios, presentation tests, and fixtures
```

Godot presentation must not directly mutate authoritative battle state. It should send commands through the local client boundary and render returned events/snapshots.
