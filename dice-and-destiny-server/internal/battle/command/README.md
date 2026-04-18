# Command

Owns battle command shapes below the JSON authority boundary.

This package should eventually contain command envelopes, command-specific payload types, parsing helpers, and command validation helpers that are not tied to Godot or transport code.

Examples:

- `roll_dice`
- `play_card`
- `select_ability`
- `pass_segment`

Do not put command execution side effects here. Execution belongs in the authority/domain packages that own the relevant rules.
