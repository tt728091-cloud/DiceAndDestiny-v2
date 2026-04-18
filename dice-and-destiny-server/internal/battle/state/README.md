# State

Owns authoritative mutable battle state.

This package should hold the data structures that command execution mutates after validation.

Examples:

- current round and segment
- player/enemy combat state
- decks, hands, discard piles, removed piles
- dice state
- resources
- statuses

Godot must never mutate this state directly. Presentation receives events and snapshots only.
