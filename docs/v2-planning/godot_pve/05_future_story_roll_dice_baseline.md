# Future Story Baseline: Real Dice Roll

## Purpose

This document is the handoff for a future dice-roll gameplay story after the segment turn-order baseline exists.

The spike proved:

```text
Godot
-> BattleAuthority
-> thin C++ GDExtension bridge
-> Go exported function
-> portable Go battle authority
-> result JSON back to Godot
```

The dice story should keep that architecture and replace the hard-coded dice result with tested dice behavior in Go.

## Current Baseline

Current successful spike tag:

```text
spike-go-gdextension-authority
```

Current key files:

```text
dice-and-destiny-server/internal/battle/authority.go
dice-and-destiny-server/internal/battle/authority_test.go
dice-and-destiny-server/internal/battle/dice/README.md
dice-and-destiny-client/scripts/verify_battle_authority.gd
dice-and-destiny-client/scripts/battle_authority.gd
dice-and-destiny-client/scripts/go_gdextension_battle_authority.gd
dice-and-destiny-server/adapters/gdextension/
```

The current Go authority returns hard-coded symbolic dice values:

```text
sword, shield, focus
```

That is acceptable for the spike, but a future dice story should introduce real dice behavior behind the authority boundary.

## Hard Rules

Testing is priority 1.

Every battle-authority story must use the relevant layers:

1. focused Go rule/module tests
2. Go authority command tests
3. Godot headless integration tests

The C++ GDExtension bridge must remain a thin JSON transport layer.

Do not modify C++ bridge code for gameplay changes. If C++ changes appear necessary, stop and explain the reason for review before implementing.

Godot presentation/client code may build commands and render results. It must not validate authoritative gameplay or mutate battle state directly.

Go owns authority behavior.

## Recommended Dice Story

Story:

```text
Implement deterministic dice rolling in Go for the `roll_dice` command.
```

Initial scope:

- add a focused Go dice package implementation under `internal/battle/dice/`
- support deterministic tests through injected RNG or fake roller
- update `authority.go` so `roll_dice` calls the dice package instead of returning hard-coded values directly
- keep JSON command/result shape stable unless there is a reviewed reason to change it
- keep the Godot headless integration test passing

Out of scope:

- real UI dice animation
- card rules
- damage rules
- enemy AI
- C++ bridge changes
- broad command schema redesign

## Suggested Files

Create:

```text
dice-and-destiny-server/internal/battle/dice/roller.go
dice-and-destiny-server/internal/battle/dice/roller_test.go
```

Update:

```text
dice-and-destiny-server/internal/battle/authority.go
dice-and-destiny-server/internal/battle/authority_test.go
dice-and-destiny-client/scripts/verify_battle_authority.gd
```

Only update Godot UI files if the story explicitly includes manual UI presentation work.

## Test Expectations

### 1. Focused Go Dice Tests

Add tests under:

```text
dice-and-destiny-server/internal/battle/dice/roller_test.go
```

Examples:

```text
Roll 5D6 returns exactly 5 values.
Each value is within 1..6.
With fake RNG, returned values are deterministic.
Symbolic dice rolling returns expected symbols from deterministic input.
```

Use deterministic dependencies. Do not write tests that depend on uncontrolled randomness.

### 2. Go Authority Command Tests

Update:

```text
dice-and-destiny-server/internal/battle/authority_test.go
```

Tests should prove:

- valid `roll_dice` command is accepted
- returned event type is `dice_rolled`
- returned dice values match the deterministic dice behavior
- snapshot is correct
- invalid command type is rejected
- invalid dice pool is rejected

### 3. Godot Headless Integration Test

Update only if needed:

```text
dice-and-destiny-client/scripts/verify_battle_authority.gd
```

This test should keep proving:

```text
Godot -> C++ -> Go -> C++ -> Godot
```

It should not use UI button clicks.

It should:

- build command dictionary directly
- call `BattleAuthority.submit_command`
- parse returned JSON
- assert `accepted == true`
- assert first event type is `dice_rolled`
- assert important dice result fields once the dice result contract is stable

## Commands To Run Before Story Completion

If Go export or C++ bridge files changed, rebuild native artifacts first:

```bash
cd dice-and-destiny-server
./scripts/build_native.sh
```

Run Go tests:

```bash
cd dice-and-destiny-server
go test ./...
```

Run Godot headless integration:

```bash
godot --headless --path dice-and-destiny-client --script res://scripts/verify_battle_authority.gd
```

The story is not complete until the relevant tests pass after the final change.

## Useful Docs

Read these before starting:

```text
docs/v2-planning/godot_pve/03_story_testing_policy.md
docs/v2-planning/godot_pve/01_portable_go_authority_architecture.md
docs/v2-planning/godot_pve/00_high_level_architecture.md
README.md
```

For interactive code walkthroughs in Codex, use the local skill:

```text
$code-walkthrough-review
```
