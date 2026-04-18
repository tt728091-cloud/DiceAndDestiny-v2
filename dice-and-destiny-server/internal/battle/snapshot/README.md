# Snapshot

Owns read-only state returned after authority commands.

Snapshots describe the current state after events have been applied. Godot can render from snapshots, and future network clients can receive the same shape over HTTP/WebSocket.

Examples:

- battle ID
- current round
- current segment
- actors
- visible dice
- visible hand/state summaries
- health/resources/statuses

Snapshots should not expose mutable state directly.
