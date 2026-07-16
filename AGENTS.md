# Dice and Destiny Agent Instructions

## Godot workspace isolation

- Always run Godot from the repository root through `./scripts/godot.sh`.
- Never invoke this project with a raw `godot --path ...` command. Doing so bypasses per-workspace logs and per-process test-state isolation.
- Do not add fixed inspector ports, shared `/tmp` paths, or shared Godot `user://` development-save paths.
- Do not point `DICE_AND_DESTINY_*_ROOT` variables at another checkout. The launcher assigns roots for the current worktree automatically.
- A new worktree needs no manual setup. The launcher builds missing native artifacts and performs the initial Godot import automatically.

Standard commands:

```bash
# Run the game
./scripts/godot.sh

# Open the editor
./scripts/godot.sh --editor

# Run a Godot script test with disposable process-local state
./scripts/godot.sh --headless --script res://path/to/test.gd

# Run with the debug inspector
DICE_AND_DESTINY_INSPECTOR=1 ./scripts/godot.sh

# Connect to the inspector for this worktree; no port or token is required
python3 dice-and-destiny-client/devtools/inspect_game.py health
```

Normal development state belongs beneath `dice-and-destiny-client/.godot/runtime/` and must remain uncommitted. Scripted Godot tests receive temporary state that the launcher removes when the process exits.

## Native builds

- Build native Go/C++ artifacts with `dice-and-destiny-server/scripts/build_native.sh`.
- Do not copy native artifacts from another worktree. Each worktree builds and loads its own ignored artifacts.
- The native build script serializes duplicate builds inside one worktree; builds in separate worktrees may run concurrently.

## Verification

Run checks relevant to the change. The standard authority checks are:

```bash
cd dice-and-destiny-server && go test ./...
cd .. && ./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd
```

Use `./scripts/godot.sh` for every additional Godot test.

## Git worktrees

- Start parallel Codex tasks in separate Worktree environments based on the intended baseline branch.
- Keep each task on its own branch before committing or pushing; Git cannot check out the same branch in multiple worktrees simultaneously.
- Do not alter or clean another worktree's `.godot`, build, save, or runtime directories.
