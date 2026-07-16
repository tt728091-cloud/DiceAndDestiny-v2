# Project TODOs

## Per-battle diagnostic bundle

- [ ] Persist an exportable diagnostic bundle for every played battle.

The bundle should be keyed by battle ID and contain:

- submitted player commands, including rejected commands;
- authoritative events in sequence order;
- client and inspector errors;
- presentation beats and player acknowledgements;
- final authority checkpoint and relevant random-state metadata;
- screenshots captured at important segment transitions and errors;
- a concise human-readable battle summary.

Suggested layout:

```text
battle-id/
  commands.jsonl
  authority-events.jsonl
  client-errors.jsonl
  presentation.jsonl
  final-checkpoint.json
  screenshots/
  summary.txt
```

Acceptance criteria:

- The bundle is written incrementally so a crash does not erase earlier evidence.
- Private authority data remains separate from viewer-safe client data.
- A battle ID is sufficient to locate and export the bundle.
- Normal gameplay does not require the inspector to be enabled.
- Diagnostics can be disabled or retention-limited for release builds.
