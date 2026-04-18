# Presentation

Owns scenes, controls, animation, input, audio, and rendering.

Presentation may:

- render authority snapshots
- animate authority events
- collect user input
- ask local client code to build commands

Presentation must not:

- validate authority commands
- mutate authoritative battle state
- apply combat rules directly
