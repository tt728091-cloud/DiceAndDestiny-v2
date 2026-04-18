# GDExtension Adapter

This adapter lets Godot call the portable Go battle authority locally.

Current path:

```text
Godot GDScript
-> C++ NativeBattleAuthority
-> Go exported HandleCommandJSON
-> internal/battle authority
```

The C++ bridge must remain a thin JSON transport layer.

It may handle:

- Godot class registration
- native library loading
- Go symbol binding
- string conversion
- C string memory cleanup
- transport/load errors

It must not parse gameplay commands, validate gameplay, mutate battle state, create events, or create snapshots.
