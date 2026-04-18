# Spike Story: Go Battle Authority Through Godot GDExtension

## Purpose

Prove whether the first PvE game can use a portable Go battle authority called from Godot through a thin native adapter.

This is a technical spike, not a gameplay feature.

## Core Question

Can we make this work cleanly?

```text
Godot UI
-> BattleAuthority boundary
-> GDExtension / native adapter
-> portable Go battle authority
-> JSON events/snapshot
-> Godot UI
```

And can that later become this without rewriting the combat rules?

```text
Godot UI
-> BattleAuthority boundary
-> HTTP/WebSocket adapter
-> Go server battle authority
-> JSON events/snapshot
-> Godot UI
```

## Hard Rules

- Go battle authority must not import Godot APIs.
- Go battle authority must not know about Godot scenes, nodes, UI, animation, or input.
- GDExtension/native adapter must stay thin.
- The spike must use JSON strings across the boundary in both directions.
- Godot sends command JSON to Go.
- Go returns result JSON to Godot.
- The spike should prove one command and one result only.
- Do not build real battle gameplay in this spike.

## Suggested Workspace Shape

Create this as a disposable or experimental project area first.

Example:

```text
spikes/
  godot_go_battle_authority/
    godot/
    go-authority/
      internal/
        battle/
      adapters/
        gdextension/
```

If creating inside the repo, keep it separate from production V2 folders until the spike result is known.

## Minimal Command

Use one command:

```json
{
  "battle_id": "battle-1",
  "actor_id": "player",
  "type": "roll_dice",
  "payload": {
    "pool": "offensive"
  }
}
```

## Minimal Result

Return one result:

```json
{
  "accepted": true,
  "events": [
    {
      "type": "dice_rolled",
      "actor_id": "player",
      "values": ["sword", "shield", "focus"]
    }
  ],
  "snapshot": {
    "battle_id": "battle-1",
    "segment": "offensive",
    "round": 1
  }
}
```

## Spike Story

### Goal

Create a minimal Godot screen with one button:

- button sends `RollDiceCommand`
- Go authority returns `DiceRolled`
- Godot displays the returned values and snapshot

## Acceptance Criteria

- Godot can call the native adapter from a button press.
- Native adapter can call portable Go authority code.
- Go authority returns JSON result.
- Godot displays the returned event/snapshot.
- Go authority has no Godot imports.
- Combat rule logic is not placed in the Godot UI.
- The spike documents whether the approach feels clean enough to continue.

## Tests / Verification

Required verification:

- Go unit test for the portable authority command handler.
- Godot headless integration test that calls the `BattleAuthority` boundary with a canned command and verifies the returned result.
- Manual Godot UI spike pressing the button and seeing the returned result.
- Export check if practical for the first target desktop platform.

Optional verification:

- Build script that compiles the Go shared library and places it where Godot can load it.

## Implementation Notes

### Recommended First Boundary Shape

Use a coarse JSON function boundary.

Example conceptual function:

```text
SubmitCommand(command_json: String) -> String
```

Meaning:

- input string is command JSON from Godot
- output string is result JSON from Go
- Godot parses the returned JSON and renders it
- Go never renders anything and never calls Godot UI

Reason:

- keeps the spike small
- mirrors future HTTP/WebSocket payloads
- avoids early typed binding complexity
- makes debugging easier

### Go Authority Package

The Go authority should expose a pure function internally:

```text
HandleCommand(commandJSON string) string
```

Rules:

- parse command JSON
- if command type is `roll_dice`, return deterministic `dice_rolled`
- no Godot imports
- no UI code
- no randomness unless injected

### Adapter

The adapter can be ugly during the spike, but must stay thin.

Allowed:

- exported native function
- string conversion
- call to `HandleCommand`
- return string to Godot

Not allowed:

- combat rule logic
- dice rule logic
- UI knowledge
- Godot scene knowledge in Go battle packages

## Expected Unknowns

The spike should answer:

- Is Go/GDExtension practical enough for this project?
- Does the build/export pipeline feel manageable?
- Are there platform issues for the first target platform?
- Is the boundary clean enough that a future HTTP/WebSocket adapter could replace it?
- Is the community Go/Godot binding path acceptable, or would a C/C++ shim be needed?

## Possible Outcomes

### Outcome A: Continue With Go GDExtension

Use this if:

- Godot call boundary is clean
- export path works
- Go code stays portable
- the adapter remains thin

Next step:

- create the real `BattleAuthority` boundary in the Godot PvE project
- start battle milestone stories with dice rolls and basic abilities

### Outcome B: Use Local Godot Authority First

Use this if:

- Go/GDExtension feels too fragile
- export path is painful
- bindings are not reliable enough

Next step:

- keep the same `BattleAuthority` boundary
- implement authority locally in Godot first
- keep command/event JSON compatible with a future Go server

### Outcome C: Use Go Sidecar Instead

Use this if:

- GDExtension is painful
- but real Go authority is important immediately
- desktop-only local shipping is acceptable

Shape:

```text
Godot UI -> localhost HTTP/WebSocket -> Go sidecar server
```

This is closer to a real future server, but has process lifecycle and packaging costs.

## Out Of Scope

- full combat engine
- enemy controller
- cards
- damage resolution
- save/load
- top-down world
- PvP networking

## Completion Proof

The spike is complete when a fresh context can report one of these:

- "Godot successfully called Go authority through native adapter and displayed `dice_rolled`."
- "The approach failed for these concrete reasons."
- "The approach works only with these constraints."

The result should be used to decide whether the first PvE battle authority is implemented in Go through GDExtension, local Godot code, or a Go sidecar.
