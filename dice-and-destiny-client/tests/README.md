# Tests

Owns deterministic Godot-side tests.

Current spike test:

```text
scripts/verify_battle_authority.gd
```

Future tests should move under this folder when we add a real Godot test harness structure.

Required style for battle-authority integration tests:

- no UI button clicks
- build command dictionaries directly
- call `BattleAuthority`
- parse returned JSON
- assert important result fields
- exit with `quit(0)` or `quit(1)`
