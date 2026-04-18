# Command Builder

Owns conversion from presentation state and user actions into battle command dictionaries/JSON.

Examples:

- build `roll_dice`
- build `play_card`
- build `select_ability`
- build `pass_segment`

This code may understand client-side input and selected UI state, but it must not decide whether a command is authoritative. Go authority owns validation.
