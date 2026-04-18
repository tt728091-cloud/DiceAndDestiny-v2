# Local Client

Owns the Godot-side request boundary into the battle authority.

Responsibilities:

- convert presentation input into authority commands
- call `BattleAuthority`
- receive authority events/snapshots
- publish render-friendly view state
- prevent UI nodes from directly mutating combat state

This is the local stand-in for a future network client adapter.
