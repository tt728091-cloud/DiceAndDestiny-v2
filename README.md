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
godot --headless --path dice-and-destiny-client --script res://scripts/verify_battle_authority.gd
```

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
