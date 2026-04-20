# Story 14: Godot Headless Engine Contract

## Purpose

Update the Godot headless integration proof after the Go engine contract is stable.

This story proves the full local authority loop still works after command handling moves from spike authority logic into the engine/flow architecture.

## Design Context

The target full loop remains:

```text
Godot
-> BattleAuthority
-> thin C++ GDExtension bridge
-> Go authority JSON boundary
-> Go engine/flow
-> events and snapshot
-> Godot
```

The C++ bridge must stay a thin transport layer.

Reference:

```text
docs/v2-planning/godot_pve/01_portable_go_authority_architecture.md
docs/v2-planning/godot_pve/03_story_testing_policy.md
docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md
```

## Scope

- Update the Godot headless test to send the stable engine-backed command shape.
- Assert important result fields from the engine-backed response.
- Rebuild native artifacts if Go exported behavior or linked Go code requires it.
- Keep the C++ bridge unchanged unless a transport-level issue is discovered.

## Out Of Scope

- Manual UI.
- Presentation polish.
- New battle screens.
- C++ gameplay handling.
- Godot-side authoritative gameplay validation.

## Tests

Run Go tests:

```bash
cd dice-and-destiny-server
go test ./...
```

Rebuild native artifacts if needed:

```bash
cd dice-and-destiny-server
./scripts/build_native.sh
```

Run Godot headless integration:

```bash
godot --headless --path dice-and-destiny-client --script res://scripts/verify_battle_authority.gd
```

The Godot test should prove:

- Godot can call the authority boundary
- C++ bridge transports the command and result
- Go authority reaches the engine
- returned JSON can be parsed by Godot
- important segment/snapshot/event fields are asserted

## Definition Of Done

- Go tests pass.
- Native artifacts are rebuilt if required.
- Godot headless integration test passes.
- C++ bridge remains gameplay-free.
- No manual UI work is required.
