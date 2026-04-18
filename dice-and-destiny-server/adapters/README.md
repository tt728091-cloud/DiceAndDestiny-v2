# Adapters

Adapters expose the portable Go battle authority to different runtimes.

Rules:

- adapters may know transport/runtime mechanics
- adapters may serialize or deserialize command/result JSON
- adapters must not contain combat rules
- adapters must not duplicate authority-side gameplay decisions

Current and planned adapters:

```text
gdextension/  current Godot local native bridge
httpserver/   future server-authoritative transport option
testdriver/   future CLI or test harness entry point
```
