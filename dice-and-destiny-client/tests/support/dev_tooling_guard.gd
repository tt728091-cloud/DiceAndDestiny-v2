extends RefCounted

## Skips script tests whose developer tooling is a launch-time opt-in.
##
## The native history, snapshot, and scenario authorities read their
## DICE_AND_DESTINY_ENABLE_* flags once when the process starts, so a test
## cannot enable them for itself. When a required flag is missing the test
## prints one SKIP line and exits with code 0 instead of failing obscurely.
##
## Usage at the very start of a SceneTree test:
##     if DevToolingGuard.skip_unless_enabled(self, "verify_example", ["DICE_AND_DESTINY_ENABLE_HISTORY"]): return

const HISTORY := "DICE_AND_DESTINY_ENABLE_HISTORY"
const SNAPSHOTS := "DICE_AND_DESTINY_ENABLE_SNAPSHOTS"
const SCENARIOS := "DICE_AND_DESTINY_ENABLE_SCENARIOS"

## Returns true (after printing SKIP and quitting with code 0) when any required
## flag is not set to "1"; the caller must return immediately in that case.
static func skip_unless_enabled(tree: SceneTree, test_name: String, required_flags: Array) -> bool:
	var missing: Array[String] = []
	for flag in required_flags:
		if OS.get_environment(str(flag)) != "1": missing.append(str(flag))
	if missing.is_empty(): return false
	var required: Array[String] = []
	for flag in required_flags: required.append("%s=1" % flag)
	print("SKIP: %s requires %s (set before launching ./scripts/godot.sh; missing: %s)" % [test_name, " ".join(required), ", ".join(missing)])
	tree.quit(0)
	return true
