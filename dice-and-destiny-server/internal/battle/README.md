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
