# Next Story Baseline: Segment Turn Order

## Purpose

This document is the handoff for the first real gameplay story after the Godot/Go authority spike.

The spike proved:

```text
Godot
-> BattleAuthority
-> thin C++ GDExtension bridge
-> Go exported function
-> portable Go battle authority
-> result JSON back to Godot
```

The first gameplay story should keep that architecture and introduce the battle segment manager in Go.

This story is scaffolding. There are no real actions to execute yet. The goal is to establish deterministic segment order and the authority-side state transition shape that later commands will use.

## Current Baseline

Current successful spike tag:

```text
spike-go-gdextension-authority
```

Current key files:

```text
dice-and-destiny-server/internal/battle/authority.go
dice-and-destiny-server/internal/battle/authority_test.go
dice-and-destiny-server/internal/battle/segment/README.md
dice-and-destiny-server/internal/battle/state/README.md
dice-and-destiny-server/internal/battle/event/README.md
dice-and-destiny-server/internal/battle/snapshot/README.md
dice-and-destiny-client/scripts/verify_battle_authority.gd
dice-and-destiny-client/scripts/battle_authority.gd
dice-and-destiny-client/scripts/go_gdextension_battle_authority.gd
dice-and-destiny-server/adapters/gdextension/
```

The current Go authority only supports a spike `roll_dice` command and returns a hard-coded snapshot:

```json
{
  "battle_id": "battle-1",
  "segment": "offensive",
  "round": 1
}
```

The segment story should make segment order a real Go domain concept instead of a hard-coded string.

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

## Recommended First Story

Story:

```text
Implement deterministic segment turn order in Go.
```

Initial scope:

- add a focused Go segment package implementation under `internal/battle/segment/`
- define the ordered battle segments
- define deterministic advancement from one segment to the next
- define round advancement when the segment cycle wraps
- add focused segment manager tests
- update authority behavior only enough to expose/prove the segment state through JSON
- keep the Godot headless integration test passing

Out of scope:

- dice rolling implementation
- real player actions
- cards
- damage
- enemy AI
- UI segment rendering
- C++ bridge changes
- broad command schema redesign

## Initial Segment Order

Use the existing PvP-style segment names for now:

```text
ongoing_effects
income
offensive
defensive
damage_resolution
```

Segment order:

```text
ongoing_effects -> income -> offensive -> defensive -> damage_resolution
```

Round behavior:

```text
after damage_resolution, the current turn is complete
the next turn starts again at ongoing_effects
round increments when the next turn starts
```

If a better segment name or order appears necessary, stop and review before changing the vocabulary.

## Suggested Files

Create:

```text
dice-and-destiny-server/internal/battle/segment/manager.go
dice-and-destiny-server/internal/battle/segment/manager_test.go
```

Possible later files, only if needed:

```text
dice-and-destiny-server/internal/battle/state/state.go
dice-and-destiny-server/internal/battle/event/event.go
dice-and-destiny-server/internal/battle/snapshot/snapshot.go
```

Update:

```text
dice-and-destiny-server/internal/battle/authority.go
dice-and-destiny-server/internal/battle/authority_test.go
dice-and-destiny-client/scripts/verify_battle_authority.gd
```

Only update Godot UI files if the story explicitly includes manual UI presentation work.

## Possible Command Shape

Because this story is scaffolding and has no real battle actions yet, the authority may need a simple test command.

Recommended command:

```json
{
  "battle_id": "battle-1",
  "actor_id": "system",
  "type": "advance_segment",
  "payload": {}
}
```

Possible accepted result:

```json
{
  "accepted": true,
  "events": [
    {
      "type": "segment_advanced",
      "from": "ongoing_effects",
      "to": "income",
      "round": 1
    }
  ],
  "snapshot": {
    "battle_id": "battle-1",
    "segment": "income",
    "round": 1
  }
}
```

This command is only a scaffold until real commands advance segments through battle rules.

If the implementation can prove segment behavior through Go focused tests and Go authority tests without adding this command yet, that is acceptable. The Godot headless integration test still needs to prove the full bridge works.

## Test Expectations

### 1. Focused Go Segment Tests

Add tests under:

```text
dice-and-destiny-server/internal/battle/segment/manager_test.go
```

Tests should prove:

- initial segment is deterministic
- segment order is deterministic
- advancing from each segment returns the expected next segment
- advancing from `damage_resolution` completes the current turn
- the next turn starts at `ongoing_effects`
- round increments only when the next turn starts
- invalid/unknown segments are rejected or handled explicitly

These tests should not depend on Godot, C++, JSON, or UI.

### 2. Go Authority Command Tests

Update:

```text
dice-and-destiny-server/internal/battle/authority_test.go
```

Tests should prove whichever authority command/result shape is chosen:

- valid segment advancement is accepted
- returned event type is `segment_advanced` if the command is added
- returned snapshot includes expected `segment` and `round`
- unsupported command types are rejected
- invalid segment state cannot silently produce incorrect results

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
- assert important segment/snapshot fields once the segment command contract is stable

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

Future dice handoff:

```text
docs/v2-planning/godot_pve/05_future_story_roll_dice_baseline.md
```

For interactive code walkthroughs in Codex, use the local skill:

```text
$code-walkthrough-review
```
