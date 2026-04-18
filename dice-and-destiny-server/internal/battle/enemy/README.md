# Enemy

Owns authority-side enemy decisions.

The enemy controller belongs on the authority side, not in Godot presentation.

Responsibilities may include:

- enemy intent selection
- hidden enemy dice decisions
- simplified enemy ability/card choices
- deterministic enemy behavior for tests

Enemy decisions should submit or produce authority-side commands/results that meet the same battle boundary as player commands.
