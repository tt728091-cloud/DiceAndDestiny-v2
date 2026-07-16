# Dice and Destiny v2

This repository contains the V2 Godot/Go battle-authority spike and planning docs.

The current proven shape is:

```text
Godot
-> BattleAuthority boundary
-> thin C++ GDExtension JSON bridge
-> Go battle authority
-> JSON result back to Godot
```

The C++ bridge is intentionally thin. Gameplay rules belong in Go authority code, and presentation belongs in Godot.

## Repository Layout

```text
dice-and-destiny-client/
  Godot project, GDScript authority boundary, headless integration test, manual spike UI

dice-and-destiny-server/
  Go authority package, Go tests, C++ GDExtension bridge, native build script

docs/
  V2 planning, Godot/Go architecture, testing policy, and development rules
```

Current Go authority layout:

```text
dice-and-destiny-server/
  internal/
    battle/
      authority.go
      authority_test.go
      command/
      event/
      snapshot/
      state/
      dice/
      ability/
      card/
      segment/
      enemy/
  adapters/
    gdextension/
    httpserver/
    testdriver/
```

Current Godot client layout:

```text
dice-and-destiny-client/
  app/
  local_client/
  features/
  engine/
  content/
  presentation/
  save/
  tests/
```

The current spike scripts remain in `dice-and-destiny-client/scripts/` until production stories move them into the new layout.

## Required Tools

- Godot 4.6 or compatible 4.x version available as `godot`
- Go
- Python 3
- Xcode command line tools / C++ toolchain on macOS
- Git

The native build script creates a local Python virtualenv for SCons when needed and clones `godot-cpp` into `dice-and-destiny-server/third_party/` when missing.

## Fresh Clone Setup

From the repo root:

```bash
cd dice-and-destiny-server
./scripts/build_native.sh
```

This builds and copies the ignored native artifacts into the Godot project:

```text
dice-and-destiny-client/native/libbattle_go_authority.dylib
dice-and-destiny-client/native/libbattle_authority.macos.template_debug.framework/
```

These files are generated locally and intentionally not committed.

## Run Tests

Run Go tests:

```bash
cd dice-and-destiny-server
go test ./...
```

Run the Godot headless integration test:

```bash
./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd
```

## Collision-Free Workspace Runs

Always launch this project through `scripts/godot.sh`, including headless tests and editor sessions:

```bash
./scripts/godot.sh
./scripts/godot.sh --editor
./scripts/godot.sh --headless --script res://tests/test_client_logic.gd
```

The launcher derives every path from its own checkout. On a new worktree it builds missing native artifacts and performs the initial Godot import automatically. Normal game and editor runs use stable workspace-local state beneath `dice-and-destiny-client/.godot/runtime/`. Scripted runs automatically receive disposable per-process client and server state. Logs use unique files, the debug inspector asks the OS for an available port, and all Go content/save roots are explicitly scoped to the current checkout.

To run and inspect a game, no port or token setup is required:

```bash
DICE_AND_DESTINY_INSPECTOR=1 ./scripts/godot.sh
python3 dice-and-destiny-client/devtools/inspect_game.py health
```

The inspector client discovers the running instance from this workspace. Do not use a raw `godot --path ...` command for development automation because it bypasses per-process test isolation and unique log allocation.

Expected Godot output includes:

```json
{"accepted":true,"events":[{"type":"dice_rolled","actor_id":"player","values":["sword","shield","focus"]}],"snapshot":{"battle_id":"battle-1","segment":"offensive","round":1}}
```

## Testing Policy

Battle-authority stories use three automated layers:

1. Focused Go rule/module tests for meaningful behavior below the JSON boundary.
2. Go authority command tests for JSON command in and JSON result out.
3. Godot headless integration tests for the full `Godot -> C++ -> Go -> C++ -> Godot` loop.

Manual UI checks are useful but are not the primary proof that a story is complete.

See:

```text
docs/v2-planning/godot_pve/03_story_testing_policy.md
```

The handoff for the first real gameplay story is:

```text
docs/v2-planning/godot_pve/04_next_story_segment_turn_order_baseline.md
```

The follow-up battle engine/segment-flow design is:

```text
docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md
```

The dice-roll handoff is preserved as a future story:

```text
docs/v2-planning/godot_pve/05_future_story_roll_dice_baseline.md
```

## C++ Bridge Rule

The C++ GDExtension bridge must remain a thin JSON transport layer.

It may handle Godot registration, native loading, symbol binding, string conversion, memory cleanup, and transport/load errors.

It must not parse gameplay commands, validate gameplay, mutate battle state, create events/snapshots, or grow fine-grained gameplay methods.

If a story appears to require C++ bridge changes, stop and explain the reason before implementing so the change can be reviewed explicitly.

## Spike Checkpoint

The tag:

```text
spike-go-gdextension-authority
```

marks the commit where the Godot headless test successfully called the Go authority through the C++ GDExtension bridge and received the expected `dice_rolled` result.
