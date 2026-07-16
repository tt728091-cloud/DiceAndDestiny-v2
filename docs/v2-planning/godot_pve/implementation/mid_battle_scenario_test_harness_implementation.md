# Mid-Battle Scenario Harness Implementation

## Status

Implemented June 15, 2026.

The implementation adds a shared Go scenario builder and a development-only
Godot launcher without adding arbitrary state import to `start_battle` or
implementing Phase 7/8 gameplay.

## Scenario Schema And Catalog

Scenario schema version 1 supports:

- authored `run_player`, `character_definition`, and `enemy_definition`
  participant baselines
- round and unentered segment entry
- complete card-zone replacement
- energy, status, token, and defeat-state overrides
- finalized offensive and defensive prerequisites
- normal or reproducible randomness
- bounded setup scripts using production commands
- metadata and a canonical SHA-256 scenario fingerprint

Named YAML scenarios are loaded only from `DICE_AND_DESTINY_SCENARIO_ROOT`.
Godot supplies a validated scenario ID, never a filesystem path. The included
`round-2-poisoned-player` fixture starts before round 2
`ongoing_effects/OnEnter`, with four removed player health cards and two Poison
stacks.

The scenario fingerprint excludes the generated battle ID and is independent
of YAML formatting and map insertion order.

## Authority And Persistence

`ScenarioAuthority` handles:

- `list_scenarios`
- `validate_scenario`
- `start_scenario`

Ordinary `Authority` handles read-only `open_battle` and all later gameplay
commands. `open_battle` loads the checkpoint and returns a viewer-safe result
without progression, events, rerolls, or writes.

Synthetic battles use generated `scenario-*` IDs. A repository router sends
that namespace to `DICE_AND_DESTINY_SCENARIO_STATE_ROOT`; normal IDs continue
to use `DICE_AND_DESTINY_BATTLE_STATE_ROOT`. Route and persisted origin must
agree.

Checkpoint battle state records:

- normal or scenario origin
- scenario ID and schema version
- stable scenario fingerprint
- creator classification
- random mode, algorithm, seed, and draw cursor

Scenario completion remains in the scenario repository. No reward,
achievement, unlock, or run-progression integration is called.

## Randomness

Production defaults use cryptographic randomness. Each draw increments the
persisted per-battle cursor.

Reproducible scenarios use `sha256-counter-v1` over the scenario seed and
persisted cursor. Planning dice and damage-card selection consume the
battle-bound source, so concurrent battles do not share mutable random state
and restart resumes at the same draw position.

Engine config still accepts injected dice and damage sources for focused tests.

## Validation

The builder rejects unsupported schema versions, unsafe IDs, invalid
participants, unknown actor overrides, invalid entry points, malformed card
conservation, unknown content, invalid energy, status stack violations,
duplicate status/token IDs, inconsistent defeat state, and malformed segment
prerequisites.

Segment-entry construction always resets to a new unentered flow with no
pending input or active resolution. Production `OnEnter` and automatic
progression create flow internals.

Setup scripts are limited to 32 commands. Payload placeholders resolve the
current pending input/window/checkpoint fields before commands are submitted
through `Engine.ApplyBattleCommand`. Expected wait assertions fail the launch
when production flow no longer reaches the named planning or reaction wait.

## Security Gating

Scenario requests require both:

1. a Go build compiled with the `scenario_tools` build tag
2. `DICE_AND_DESTINY_ENABLE_SCENARIOS=1` in the process environment

The Godot launcher additionally requires a debug build and the project setting
`dice_and_destiny/development/enable_scenarios`.

The environment must be set before Godot starts because the Go c-shared runtime
captures process configuration when loaded. Release builds must omit the
`scenario_tools` tag. The native development build script includes it
explicitly.

## Godot Flow

Godot now has:

- scenario command builders
- a JSON battle gateway
- viewer-safe battle view state
- active battle-ID persistence
- a development scenario-selection screen
- a snapshot-driven battle screen
- startup reopen through `open_battle`

The launcher hands both normal and scenario results to the same battle screen.
The C++ GDExtension remains a JSON/string transport and contains no gameplay
logic.

## Tests

Go coverage includes schema loading, fixture construction, round/segment
entry, actor overrides, prerequisite validation, stable fingerprints, normal
and reproducible randomness, bounded scripted planning waits, both player
baselines, disabled tooling, catalog safety, creation, duplicate protection,
repository routing, provenance, hidden information, restart, terminal reopen,
random cursor continuation, and non-mutating `open_battle`.

The Godot headless test runs the complete
Godot -> C++ -> Go -> repository -> Go -> C++ -> Godot path. It lists and
starts the Poison fixture, verifies displayed damage/provenance and hidden
enemy information, persists the active battle ID, reopens the battle, and
compares checkpoint bytes before and after reopen.

Run it after rebuilding native authority:

```bash
cd dice-and-destiny-server
./scripts/build_native.sh

cd ..
DICE_AND_DESTINY_ENABLE_SCENARIOS=1 \
./scripts/godot.sh --headless \
  --script res://tests/scenarios/verify_scenario_launcher.gd
```

## Deliberate Scope

Raw exact-checkpoint import remains test-only through the existing repository
checkpoint APIs. Interactive exact waits use bounded production-command setup
scripts.

Poison content is loaded, validated, persisted, opened, and displayed. Its
Phase 7 execution is intentionally not implemented, so automatic progression
continues beyond `ongoing_effects` to the next real human planning wait.
