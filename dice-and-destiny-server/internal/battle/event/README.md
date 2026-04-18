# Event

Owns battle domain events emitted by accepted commands.

Events describe what happened. They are intended for Godot presentation, future network clients, logs, replay/debug tooling, and tests.

Examples:

- `dice_rolled`
- `card_played`
- `damage_queued`
- `status_applied`
- `segment_advanced`

Events should describe authority-approved outcomes, not UI actions.
