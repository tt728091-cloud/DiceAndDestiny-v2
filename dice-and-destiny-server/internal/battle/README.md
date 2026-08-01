# Battle Authority Package Layout

This folder owns the portable Go battle authority.

Rules:

- no Godot imports
- no GDExtension types
- no UI state
- deterministic tests
- commands in, events/snapshots out

The current spike keeps the first command handler in `authority.go`. As battle logic grows, move meaningful behavior into the subpackages below instead of letting `authority.go` become a large rules file.

Planned package ownership:

```text
command/   command envelopes, parsing helpers, and command validation shapes
event/     domain events emitted by accepted commands
snapshot/  read-only state returned to Godot or future network clients
state/     authoritative mutable battle state
dice/      dice pools, rolling, symbols, and deterministic dice tests
ability/   ability definitions, selection rules, and resolution helpers
card/      card behavior, zones, card-as-health rules, and card movement
segment/   battle segment/phase progression
enemy/     authority-side enemy decisions and intent logic
```

`authority.go` should remain the coarse JSON command boundary:

```text
command JSON -> parse/validate -> call domain packages -> events/snapshot -> result JSON
```

## Local two-seat Blade Warden driver

Phase 1 includes a server-side console that starts two externally controlled
Blade Wardens with reproducible randomness, prints each seat's viewer-safe
snapshot and complete legal commands, and submits the selected command through
the normal authority boundary:

```bash
cd dice-and-destiny-server
go run ./cmd/two-seat-driver -battle-id local-blade-mirror -seed 1
```

Use `-resume` with the same battle ID to continue a persisted local battle.
The driver is local-only and does not open a network listener.

The underlying start command is also available directly to tests and other
local authority clients:

```json
{
  "battle_id": "blade-mirror-1",
  "actor_id": "seat-a",
  "type": "start_battle",
  "payload": {
    "seats": [
      {"instance_id": "seat-a", "definition_id": "blade_warden"},
      {"instance_id": "seat-b", "definition_id": "blade_warden"}
    ],
    "seed": 1
  }
}
```

When the viewer owns pending input, the result's `legal_actions` array contains
complete command envelopes that can be submitted unchanged. `open_battle`
returns the same seat-specific observation and current candidates without
advancing the battle.
