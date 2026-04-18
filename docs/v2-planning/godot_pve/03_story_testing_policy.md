# Godot/Go Story Testing Policy

## Purpose

This document defines the required testing shape for Godot/Go battle-authority stories.

Testing is priority 1 for this project. A story is not complete until its relevant focused tests are added or updated and the full automated test suite passes.

The goal is not to test every tiny function. The goal is to prove behavior at the right layers:

- focused Go rule/module behavior
- Go authority command behavior
- Godot headless integration behavior

Manual UI checks are useful during spikes, but they are not the base proof that a story is done.

## C++ Bridge Boundary Rule

The C++ GDExtension bridge must remain a thin JSON transport layer.

Its allowed responsibilities are:

- register Godot-visible native bridge classes
- expose coarse Godot-callable methods such as `submit_command(command_json) -> result_json`
- load the Go shared library
- bind exported Go symbols
- convert Godot strings to C strings and returned C strings back to Godot strings
- free memory that crosses the C/Go boundary
- return transport/load errors as JSON result strings

The C++ bridge must not own gameplay semantics.

It must not:

- parse command JSON to branch on gameplay command types
- know what `roll_dice`, `play_card`, damage, status, segments, enemies, cards, or abilities mean
- validate gameplay commands
- mutate battle state
- create battle events or snapshots
- duplicate Go command/result structs in C++
- grow new fine-grained gameplay methods such as `roll_die`, `apply_damage`, or `resolve_ability`

Normal story work should not require C++ changes.

If a story appears to require C++ bridge changes, stop and explain why before implementing them. The reason must be reviewed explicitly. Acceptable reasons are rare and should be limited to bridge mechanics, such as platform loading, symbol binding, memory handling, coarse transport entry points, packaging, diagnostics, or threading/runtime concerns.

Gameplay changes belong in Go authority code and Godot presentation/client code, not in the C++ bridge.

## Required Test Layers

### 1. Focused Go Rule Or Module Tests

Use these for meaningful Go functions or modules below the JSON authority boundary.

These are smaller than a full command loop, but still test real behavior. They should not target trivial getters, setters, or formatting helpers.

Examples:

- roll a dice pool and get the expected number of dice
- validate dice values are inside the allowed range
- apply damage and verify health/card movement
- resolve an ability against a target
- advance a segment from offensive to defensive
- apply a status effect and verify its duration or modifier

Example shape:

```text
rollDice(count=5, sides=6, rng=fake_rng) -> 5 values, each in 1..6
```

Random behavior must be testable deterministically. Prefer injected RNG, fixed seeds, or fake dice rollers. Tests should not depend on uncontrolled randomness.

These tests are the right place to use test-first development when the outcome is clear. For example, if the next story is "roll 5D6", write the focused Go test first, then implement until it passes.

### 2. Go Authority Command Tests

Use these for the portable Go authority command boundary.

These tests call the Go authority directly, without Godot and without C++.

Current example:

```text
internal/battle/authority_test.go
```

Current shape:

```text
command JSON string
-> battle.HandleCommand(commandJSON)
-> result JSON string
-> parse and assert result
```

These tests prove:

- command JSON parses correctly
- unsupported commands are rejected
- invalid payloads are rejected
- accepted commands emit the expected events
- accepted commands return the expected snapshot
- the Go result JSON contract stays stable

When a story adds or changes a command, this layer must be updated.

### 3. Godot Headless Integration Tests

Use these for the full local authority loop from Godot through native code into Go and back.

Current example:

```text
dice-and-destiny-client/scripts/verify_battle_authority.gd
```

Current shape:

```text
Godot headless test
-> BattleAuthority
-> GoGDExtensionBattleAuthority
-> NativeBattleAuthority C++ bridge
-> Go exported HandleCommandJSON
-> battle.HandleCommand
-> result JSON back to Godot
-> parse and assert
-> quit(0) or quit(1)
```

These tests must not depend on UI button clicks or visual nodes. They should build commands directly, call the authority boundary, parse the returned JSON, and assert important fields.

Use this layer to prove:

- Godot can load the GDExtension
- Godot can instantiate the native authority
- C++ can load the Go shared library
- command JSON reaches Go
- result JSON returns to Godot
- Godot can parse and validate the result

When a story changes the authority boundary, native bridge, exported Go entry point, or command/result contract, this layer must be updated.

## Manual UI Spike Checks

Manual UI checks are allowed, but they are not a replacement for automated tests.

Current example:

```text
dice-and-destiny-client/scripts/battle_spike.gd
```

This screen is useful for proving:

- a user can press a button
- the UI can call the authority boundary
- the result can be displayed

It should not be treated as the story's primary correctness proof. Automated tests should cover behavior first. UI checks can be added when the story specifically changes presentation, input, or rendering.

## Story Definition Of Done

Every battle-authority story must answer these before it is complete:

- Which focused Go rule/module tests prove the new behavior?
- Which Go authority command tests prove the JSON command/result behavior?
- Which Godot headless integration tests prove the full Godot-to-Go loop still works?
- Were all automated tests run after the change?

A story is complete only when:

- relevant new or changed tests are committed with the story
- focused Go tests pass
- Go authority command tests pass
- Godot headless integration tests pass
- any required manual UI check is documented separately from automated test proof

## Current Commands

Run Go tests:

```bash
cd /Users/daddymere/games/Dice-and-Destiny-v2/dice-and-destiny-server
go test ./...
```

Run the Godot headless integration test:

```bash
godot --headless --path /Users/daddymere/games/Dice-and-Destiny-v2/dice-and-destiny-client --script res://scripts/verify_battle_authority.gd
```

Rebuild native artifacts before the Godot integration test when Go export or C++ bridge code changes:

```bash
cd /Users/daddymere/games/Dice-and-Destiny-v2/dice-and-destiny-server
./scripts/build_native.sh
```

Then rerun the Godot headless integration test.
