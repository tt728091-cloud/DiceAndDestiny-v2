# Story 13: Segment Flow Completion Policy

## Purpose

Formalize how segment flows decide whether to wait, advance, block, or remain active.

The segment manager should only advance when the engine asks it to. Segment flows should report their decision to the engine.

## Design Context

The key rule:

```text
Segment flow controls completion decisions.
Engine applies those decisions.
Segment manager calculates deterministic next segment.
```

Reference:

```text
docs/v2-planning/godot_pve/06_battle_engine_segment_flow_design.md
```

## Scope

- Finalize the `FlowDecision` shape.
- Define how the engine consumes flow decisions.
- Define whether decisions include values such as:
  - `wait_for_command`
  - `ready_to_advance`
  - `blocked`
  - `complete`
- Add engine safeguards against accidental infinite auto-advance loops.
- Add tests for flow-controlled advancement.

## Out Of Scope

- New card mechanics.
- New dice mechanics.
- Damage mechanics.
- Godot, C++, or UI changes.

## Tests

Run from `dice-and-destiny-server`:

```bash
go test ./...
```

Add engine/domain tests proving:

- a flow can choose to wait
- a flow can choose to become ready to advance
- engine does not advance unless commanded or policy says to continue
- segment manager does not know why advancement happened
- repeated advancement has an explicit guard against infinite loops
- invalid decisions are rejected or handled explicitly

## Definition Of Done

- Flow completion policy is explicit in code.
- Engine behavior for each decision is tested.
- Segment manager remains deterministic and narrow.
- Go tests pass.
- No Godot, C++, or UI files are changed.
