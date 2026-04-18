# Portable Go Battle Authority Architecture

## Purpose

This document defines the long-term architecture target for a Godot-first PvE game that can later grow into a server-authoritative PvE/PvP game.

The first game is:

- single-player
- local PvE
- Godot UI and world
- Go combat brain if the GDExtension spike proves viable

The long-term goal is:

- top-down 2D world similar in perspective to Pokemon Red/Blue
- player walks around a world
- player fights PvE monsters
- later support PvP fights
- eventually support online PvE/PvP or MMO-style server authority

The architecture should make that future easier, not harder.

## Core Principle

The Go code must be the portable battle authority, not Godot extension gameplay code.

That means:

- Go owns combat rules
- Go owns battle state mutation
- Go owns command validation
- Go owns deterministic battle events
- Go owns snapshots
- Go does not know about Godot scenes
- Go does not import Godot APIs
- Go does not call Godot nodes
- Go does not render UI
- Go does not play audio
- Go does not know about Godot input events

Godot owns:

- UI
- animation
- input
- top-down world exploration
- battle presentation
- local adapter calls into the authority
- save UI and world UI

## The Shape We Want

First local PvE game:

```text
Godot presentation
-> Godot battle gateway
-> GDExtension / native adapter
-> portable Go battle authority
-> events and snapshots
-> Godot battle gateway
-> Godot presentation
```

Future server-authoritative game:

```text
Godot presentation
-> Godot network battle gateway
-> HTTP/WebSocket
-> Go server battle authority
-> events and snapshots
-> HTTP/WebSocket
-> Godot network battle gateway
-> Godot presentation
```

The goal is for the Godot presentation layer to not care which authority transport is currently being used.

## Adapter Rule

The authority boundary should be represented by an interface-like concept:

```text
BattleAuthority
```

The Godot side should only know this boundary:

```text
submit_command(command) -> battle_result
get_snapshot() -> battle_snapshot
```

Initial boundary rule:

- Godot sends command JSON to the authority boundary.
- Go returns result JSON with events and snapshots.
- Godot renders from the returned JSON data.
- Go does not render or call Godot.
- Godot does not run combat logic for commands that belong to the authority.

Local GDExtension implementation:

```text
BattleAuthority
-> GoGDExtensionBattleAuthority
-> portable Go battle authority
```

Future server implementation:

```text
BattleAuthority
-> NetworkBattleAuthority
-> Go server API
```

Test implementation:

```text
BattleAuthority
-> FakeBattleAuthority
```

The rest of the Godot UI should not directly know whether the battle authority is local Go, local GDScript, or a remote server.

## C++ Bridge Boundary Rule

The C++ GDExtension bridge is only a native transport adapter.

It should remain thin and stable:

```text
Godot String command_json
-> C++ bridge
-> Go exported function
-> Go result JSON
-> C++ bridge
-> Godot String result_json
```

The bridge may know how to:

- register `NativeBattleAuthority` with Godot
- expose coarse methods such as `submit_command`
- load the Go shared library
- bind Go symbols such as `HandleCommandJSON` and `FreeCString`
- convert strings across the Godot/C/Go boundary
- free returned C strings
- report load/transport failures as JSON errors

The bridge must not know gameplay meaning.

It must not parse commands to branch on `roll_dice`, `play_card`, damage, status, cards, enemies, abilities, or segments. It must not validate gameplay, mutate battle state, create events, create snapshots, or duplicate Go rule structs.

Normal gameplay stories should not modify C++ bridge code. If a C++ bridge change appears necessary, the implementation must stop and explain the reason for review before making the change. Acceptable reasons should be limited to bridge mechanics such as platform loading, packaging, symbol binding, memory ownership, diagnostics, threading/runtime concerns, or adding a rare coarse transport entry point.

## Go Package Boundary

The Go code should be split into two layers.

### Portable Engine

This is the real game logic.

Example path:

```text
go-authority/
  internal/
    battle/
      command/
      event/
      snapshot/
      state/
      dice/
      ability/
      card/
      segment/
      enemy/
```

Current repository path:

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
```

Rules:

- no Godot imports
- no GDExtension types
- no UI state
- deterministic tests
- commands in, events/snapshots out

### Adapter Layer

This is the thin layer that lets another runtime call the engine.

Example path:

```text
go-authority/
  adapters/
    gdextension/
    httpserver/
    testdriver/
```

Current repository path:

```text
dice-and-destiny-server/
  adapters/
    gdextension/
    httpserver/
    testdriver/
```

Rules:

- adapter may know how to serialize/deserialize commands
- adapter may know about function-call boundary details
- adapter may expose exported functions for a shared library
- adapter must not contain combat rules

If a later server is built, the `httpserver` adapter should call the same portable engine that the GDExtension adapter called.

## Command And Result Contract

For the first PvE architecture and the GDExtension spike, the Godot-to-Go boundary must be JSON in both directions.

Required shape:

```text
Godot -> command JSON -> Go authority
Go authority -> result JSON -> Godot
```

This intentionally mirrors the future server shape:

```text
Godot -> command JSON over HTTP/WebSocket -> Go server
Go server -> result JSON over HTTP/WebSocket -> Godot
```

Keep the boundary coarse and stable.

Example command envelope:

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

Example result envelope:

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

Why JSON at the boundary:

- easy to debug
- easy to test
- easy to mirror over HTTP/WebSocket later
- avoids Godot-specific type coupling
- keeps GDExtension spike simple

Typed native calls can be considered later only if the JSON boundary becomes a proven performance problem. The default architectural contract is JSON commands in and JSON results out.

## Enemy Controller Placement

The enemy controller belongs on the authority side.

Local PvE:

```text
Godot presentation -> BattleAuthority -> Go battle authority
Go enemy controller -> Go battle authority
```

Future server PvE:

```text
Godot client -> network -> Go server battle authority
Go server enemy controller -> Go server battle authority
```

Future PvP:

```text
Godot client A -> network -> Go server battle authority
Godot client B -> network -> Go server battle authority
```

The player and enemy should both produce commands, but they do not need to come from the same place.

Player command source:

- Godot presentation/client

Enemy command source:

- authority-side enemy controller

Shared command destination:

- battle authority boundary

## Why This Helps The Future PvP Game

If the first PvE game is built this way, the future PvP game can reuse:

- combat command types
- combat validation
- dice rules
- ability rules
- card-as-health rules
- segment progression
- damage resolution
- status effects
- snapshots
- event stream concepts
- deterministic tests
- JSON content schemas

What would change later:

- GDExtension/local adapter becomes HTTP/WebSocket adapter
- local authority becomes server authority
- authority-side enemy controller may coexist with remote player clients
- Godot top-down world can call into either local or remote battle authority

## Risks

### Go GDExtension Integration Risk

Godot officially documents GDExtension around native shared libraries and C++ bindings. Go can build C shared libraries, but Go/Godot binding support is not the official primary path.

Mitigation:

- run a spike before committing
- keep the Go engine independent from the GDExtension adapter
- use JSON strings across the boundary first

### Boundary Leakage Risk

Risk:

- Godot UI starts depending on Go implementation details
- Go battle engine starts depending on Godot details

Mitigation:

- Godot talks only to `BattleAuthority`
- Go portable engine has no Godot imports
- adapter code stays thin

### Overengineering Risk

Risk:

- too much architecture before the first battle works

Mitigation:

- first spike only proves one command and one event
- first milestone starts with dice rolls and basic abilities
- do not build the whole future MMO architecture now

## Recommended Decision

Use this as the target architecture:

```text
Godot UI
-> BattleAuthority boundary
-> thin adapter
-> portable Go battle authority
```

But only commit to Go/GDExtension after a spike proves:

- Godot can call the Go authority cleanly
- Go can return events/snapshots cleanly
- the exported build can include the native library cleanly
- the Go authority remains free of Godot dependencies

If the spike fails, keep the same architecture but implement the first local authority in Godot. The boundary still protects the future server path.
