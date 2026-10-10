extends Node

const ACCEPTED_V1_MODEL_PATH := "res://models/learned/blade-warden-seed-11-final-v1.json"
const PHASE2_V2_MODEL_PATH := "res://models/learned/blade-warden-decision-quality-seed-22-v2.json"
const PHASE2_V2_MODEL_SHA256 := "0e6ea5d84c316a7709c9c1b98b983e8f76e486c6e1625c479c55d2860bd23a86"
const OPTIMIZED_V3_MODEL_PATH := "res://models/learned/blade-warden-optimized-5m-seed-22-v3.json"
const OPTIMIZED_V3_MODEL_SHA256 := "529a6b4d6ad347d5ba86b5e000cb5fceec306414cdf0af3405713a2bc5c32ebb"
const GLOBAL_CHAMPION_MODEL_PATH := "res://models/learned/blade-warden-global-champion-cp193-winner-health-v2.json"
const GLOBAL_CHAMPION_MODEL_SHA256 := "cd3d7451e91d071274c874b017e3c7e73358862090fd71954e282e5bf505b814"
const PRIOR_GLOBAL_CP38_MODEL_PATH := "res://models/learned/blade-warden-global-champion-cp38-winner-health-v2.json"
const PRIOR_GLOBAL_CP38_MODEL_SHA256 := "6aeb05e0657c8c221298c79780895bcc6aa527f1780c6c5eb66833293daf6693"
const PRIOR_GLOBAL_CP480_MODEL_PATH := "res://models/learned/blade-warden-global-champion-cp480-v3.json"
const PRIOR_GLOBAL_CP480_MODEL_SHA256 := "96755c199f4d93261695d4928a0f95e00858d3a9d6a5c429e46911fbd0a3cac6"
const PHASE2_OPT_IN_ENV := "DICE_AND_DESTINY_PHASE2_DECISION_MODEL"
const MODEL_ACCEPTED_V1 := "accepted-v1"
const MODEL_DECISION_V2 := "decision-v2"
const MODEL_OPTIMIZED_V3 := "optimized-v3"
const MODEL_GLOBAL_CHAMPION := "global-champion"
const MODEL_PRIOR_GLOBAL_CP38 := "prior-global-cp38"
const MODEL_PRIOR_GLOBAL_CP480 := "prior-global-cp480"
const MODEL_BRINE_MASK := "brine-mask"
const MODEL_BRINE_PAIR := "brine-mask-pair"
const MODEL_BELL_DIVER := "bell-diver"
const MODEL_RIBBON_EEL := "ribbon-eel"
## Scripted single-ability minions: content definition plus the summary shown in
## the setup menu and the battle badge. The server reads behavior from content.
const MINIONS := {
	MODEL_BRINE_MASK: {"definition": "drowned_oracle_brine_mask", "keeps": "Keeps every 3", "badge": "BRINE MASK · Keeps every 3",
		"rules": "Three rolls · 2 damage per 3 · Salt Veil rolls 1D6: half, rounded up · Brine Surge costs 5 energy for +1 damage"},
	MODEL_BRINE_PAIR: {"definition": "drowned_oracle_brine_mask", "keeps": "Keeps every 3", "badge": "BRINE MASK · Keeps every 3",
		"rules": "Three rolls · 2 damage per 3 · Salt Veil rolls 1D6: half, rounded up · Brine Surge costs 5 energy for +1 damage"},
	MODEL_BELL_DIVER: {"definition": "drowned_oracle_bell_diver", "keeps": "Keeps every 5 and 6", "badge": "BELL DIVER · Keeps every 5 and 6",
		"rules": "Three rolls · 3/4/5 Tolls deal 4/5/8; fewer is a miss · Brass Helm rolls 1D6: 1–3 prevent 2, 4–6 prevent 3 · 18 blank health cards"},
	MODEL_RIBBON_EEL: {"definition": "drowned_oracle_ribbon_eel", "keeps": "Keeps every even die", "badge": "RIBBON EEL · Keeps every even die",
		"rules": "Three rolls · 3/4/5 Snares deal 3/5/7; fewer is a miss · Slip the Current rolls 1D6: 1–3 prevent 1, 4–5 prevent 2, 6 prevents 4 · 15 blank health cards"},
}
## Campaign encounters name any scripted minion: "minion:<definition>:<count>".
const MODEL_MINION_PREFIX := "minion:"
const INFERENCE_TIMEOUT_MS := 2000

var selected_loadout_mode := "sandbox"
var _native_authority: Object
var _initialized := false
var _initialized_model_key := ""
var _selected_model_key := MODEL_ACCEPTED_V1
var _initialization_error := ""

func _ready() -> void:
	if OS.get_environment(PHASE2_OPT_IN_ENV) == "1":
		_selected_model_key = MODEL_DECISION_V2
	if ClassDB.can_instantiate("NativeBattleAuthority"):
		_native_authority = ClassDB.instantiate("NativeBattleAuthority")
	else:
		_initialization_error = "NativeBattleAuthority GDExtension class is unavailable."

func select_model(model_key: String) -> Dictionary:
	if model_key.begins_with(MODEL_MINION_PREFIX) and _minion_opponent(model_key).is_empty():
		return {"ok": false, "error": "Invalid minion opponent selection: %s" % model_key}
	if not model_key.begins_with(MODEL_MINION_PREFIX) and model_key not in [MODEL_ACCEPTED_V1, MODEL_DECISION_V2, MODEL_OPTIMIZED_V3, MODEL_GLOBAL_CHAMPION, MODEL_PRIOR_GLOBAL_CP38, MODEL_PRIOR_GLOBAL_CP480] and not MINIONS.has(model_key):
		return {"ok": false, "error": "Unknown learned model selection: %s" % model_key}
	_selected_model_key = model_key
	_initialized = _initialized_model_key == model_key
	_initialization_error = ""
	return {"ok": true, "model_key": model_key}

func selected_model_key() -> String:
	return _selected_model_key

## The menu/badge summary for a scripted minion's content definition.
static func minion_summary(definition_id: String) -> Dictionary:
	for minion in MINIONS.values():
		if minion.definition == definition_id: return minion
	return {}

## The model key for a campaign encounter's scripted minion opponent.
static func encounter_model_key(encounter: Dictionary) -> String:
	return "%s%s:%d" % [MODEL_MINION_PREFIX, str(encounter.get("opponent", "")), maxi(1, int(encounter.get("opponent_count", 1)))]

func _minion_opponent(model_key: String) -> Dictionary:
	if MINIONS.has(model_key):
		return {"definition": MINIONS[model_key].definition, "count": 2 if model_key == MODEL_BRINE_PAIR else 1}
	if not model_key.begins_with(MODEL_MINION_PREFIX): return {}
	var parts := model_key.trim_prefix(MODEL_MINION_PREFIX).split(":")
	if parts.size() != 2 or parts[0].is_empty() or not parts[1].is_valid_int() or int(parts[1]) not in [1, 2]: return {}
	return {"definition": parts[0], "count": int(parts[1])}

func ensure_initialized() -> Dictionary:
	if _initialized:
		return {"ok": true}
	if _native_authority == null:
		return {"ok": false, "error": _initialization_error}
	var minion := _minion_opponent(_selected_model_key)
	var result := _request({
		"op": "initialize",
		"loadout_root": WorkspacePaths.runtime_dir("user/character_loadouts"),
		"replace_session": not _initialized_model_key.is_empty() and _initialized_model_key != _selected_model_key,
		"model_path": "" if not minion.is_empty() else ProjectSettings.globalize_path(_selected_model_path()),
		"opponent_count": int(minion.get("count", 1)),
		"opponent_definition": str(minion.get("definition", "")),
		"model_sha256": _selected_model_sha256(),
		"content_root": ProjectSettings.globalize_path("res://../dice-and-destiny-server/content"),
		"run_state_root": ProjectSettings.globalize_path("res://../dice-and-destiny-server/save/run_players"),
		"diagnostics_path": WorkspacePaths.persistent_file("phase3/learned-battles.jsonl"),
		"timeout_ms": INFERENCE_TIMEOUT_MS,
	})
	_initialized = result.get("ok") == true
	if _initialized:
		_initialized_model_key = _selected_model_key
	if not _initialized:
		_initialization_error = str(result.get("error", "The learned policy could not be loaded."))
	return result

func _selected_model_path() -> String:
	match _selected_model_key:
		MODEL_DECISION_V2:
			return PHASE2_V2_MODEL_PATH
		MODEL_OPTIMIZED_V3:
			return OPTIMIZED_V3_MODEL_PATH
		MODEL_GLOBAL_CHAMPION:
			return GLOBAL_CHAMPION_MODEL_PATH
		MODEL_PRIOR_GLOBAL_CP38:
			return PRIOR_GLOBAL_CP38_MODEL_PATH
		MODEL_PRIOR_GLOBAL_CP480:
			return PRIOR_GLOBAL_CP480_MODEL_PATH
		_:
			return ACCEPTED_V1_MODEL_PATH

func _selected_model_sha256() -> String:
	match _selected_model_key:
		MODEL_DECISION_V2:
			return PHASE2_V2_MODEL_SHA256
		MODEL_OPTIMIZED_V3:
			return OPTIMIZED_V3_MODEL_SHA256
		MODEL_GLOBAL_CHAMPION:
			return GLOBAL_CHAMPION_MODEL_SHA256
		MODEL_PRIOR_GLOBAL_CP38:
			return PRIOR_GLOBAL_CP38_MODEL_SHA256
		MODEL_PRIOR_GLOBAL_CP480:
			return PRIOR_GLOBAL_CP480_MODEL_SHA256
		_:
			return ""

## A non-empty `encounter` starts that encounter of the `campaign_save`, with
## the save's own deck and abilities; the authority records the result there.
func start_battle(battle_id: String, human_seat: String, seed: int, rematch: bool = false, character: String = "blade_warden", unified_defense: bool = false, loadout_mode: String = "sandbox", encounter: String = "", campaign_save: String = "") -> Dictionary:
	var initialized := ensure_initialized()
	if initialized.get("ok") != true:
		return {"accepted": false, "error": initialized.get("error", _initialization_error)}
	return _request({
		"op": "reset",
		"encounter": encounter,
		"campaign_save": campaign_save,
		"loadout_mode": loadout_mode,
		"battle_id": battle_id,
		"human_seat": human_seat,
		"character": character,
		"unified_defense": unified_defense,
		"seed": seed,
		"rematch": rematch,
	})

func submit_human(command_json: String) -> Dictionary:
	if not _initialized:
		return {"accepted": false, "error": "The learned battle runtime is not initialized."}
	return _request({"op": "submit_human", "command_json": command_json})

func advance_model() -> Dictionary:
	if not _initialized:
		return {"accepted": false, "error": "The learned battle runtime is not initialized."}
	return _request({"op": "advance_model"})

func telemetry() -> Dictionary:
	if not _initialized:
		return {"ok": false, "error": "The learned battle runtime is not initialized."}
	return _request({"op": "telemetry"})

func initialization_error() -> String:
	return _initialization_error

func _request(payload: Dictionary) -> Dictionary:
	var raw := str(_native_authority.learned_battle_request(JSON.stringify(payload)))
	var parsed = JSON.parse_string(raw)
	if parsed is Dictionary:
		return parsed
	return {"accepted": false, "ok": false, "error": "Learned runtime returned invalid JSON.", "raw_result": raw}

func _exit_tree() -> void:
	if is_instance_valid(_native_authority):
		_native_authority.free()
		_native_authority = null

# Read-only: does not initialize, replace, or advance the active battle session.
func character_catalogs(mode: String = "sandbox") -> Dictionary:
	if _native_authority == null: return {"ok": false, "error": _initialization_error}
	return _request({"op": "progression_catalogs" if mode == "progression" else "character_catalogs", "loadout_root": WorkspacePaths.runtime_dir("user/character_loadouts"), "content_root": ProjectSettings.globalize_path("res://../dice-and-destiny-server/content")})

## The campaign menu: encounters, starting sheets, and every campaign save with
## its XP, health and next encounter. Converts pre-save campaign progress once.
func campaign_status() -> Dictionary:
	if _native_authority == null: return {"ok": false, "error": _initialization_error}
	return _request({"op": "campaign_status", "loadout_root": WorkspacePaths.runtime_dir("user/character_loadouts"), "content_root": ProjectSettings.globalize_path("res://../dice-and-destiny-server/content")})

## Starts a campaign: a new named save copying the starting sheet `character`.
func campaign_new(character: String, campaign_name: String) -> Dictionary:
	if _native_authority == null: return {"ok": false, "error": _initialization_error}
	return _request({"op": "campaign_new", "character": character, "name": campaign_name, "loadout_root": WorkspacePaths.runtime_dir("user/character_loadouts"), "content_root": ProjectSettings.globalize_path("res://../dice-and-destiny-server/content")})

## Permanently deletes one campaign save.
func campaign_delete(save_id: String) -> Dictionary:
	if _native_authority == null: return {"ok": false, "error": _initialization_error}
	return _request({"op": "campaign_delete", "campaign_save": save_id, "loadout_root": WorkspacePaths.runtime_dir("user/character_loadouts"), "content_root": ProjectSettings.globalize_path("res://../dice-and-destiny-server/content")})

## Read-only: one campaign save's deck, stored cards and abilities with every
## trade open to them, each pre-checked by the authority.
func campaign_loadout(save_id: String) -> Dictionary:
	if _native_authority == null: return {"ok": false, "error": _initialization_error}
	return _request({"op": "campaign_loadout", "campaign_save": save_id, "loadout_root": WorkspacePaths.runtime_dir("user/character_loadouts"), "content_root": ProjectSettings.globalize_path("res://../dice-and-destiny-server/content")})

## One Prepare transaction on one campaign save (never the editors' saves).
func campaign_purchase(save_id: String, kind: String, id: String, revision: int, cost: int, target_id: String = "", tree: String = "") -> Dictionary:
	if _native_authority == null: return {"ok": false, "error": _initialization_error}
	return _request({"op": "campaign_purchase", "campaign_save": save_id, "purchase": {"kind": kind, "id": id, "revision": revision, "expected_cost": cost, "target_id": target_id, "tree": tree}, "loadout_root": WorkspacePaths.runtime_dir("user/character_loadouts"), "content_root": ProjectSettings.globalize_path("res://../dice-and-destiny-server/content")})

func save_character_deck(character: String, decklist: Array) -> Dictionary:
	if _native_authority == null: return {"ok": false, "error": _initialization_error}
	var entries: Array = []
	# Shared cards' copies keep the tree they came through; never strip or merge them.
	for entry in decklist:
		var line := {"card_id": str(entry.card_id), "count": int(entry.count)}
		if not str(entry.get("tree", "")).is_empty(): line.tree = str(entry.tree)
		entries.append(line)
	return _request({"op": "save_character_deck", "content_root": ProjectSettings.globalize_path("res://../dice-and-destiny-server/content"), "loadout_root": WorkspacePaths.runtime_dir("user/character_loadouts"), "character": character, "decklist": entries})

## `tree` names the card tree for tree trades and for copies of shared cards;
## exclusive and ordinary cards ignore it.
## `tree_cards_only` (campaign editing) refuses adding cards outside every card tree.
func purchase_progression(character: String, kind: String, id: String, revision: int, cost: int, target_id: String = "", tree: String = "", tree_cards_only: bool = false) -> Dictionary:
	if _native_authority == null: return {"ok": false, "error": _initialization_error}
	return _request({"op": "progression_purchase", "content_root": ProjectSettings.globalize_path("res://../dice-and-destiny-server/content"), "loadout_root": WorkspacePaths.runtime_dir("user/character_loadouts"), "character": character, "purchase": {"kind": kind, "id": id, "revision": revision, "expected_cost": cost, "target_id": target_id, "tree": tree, "tree_cards_only": tree_cards_only}})

func save_economy_admin(settings: Dictionary) -> Dictionary:
	return _request({"op": "save_economy_admin", "admin_settings": settings, "loadout_root": WorkspacePaths.runtime_dir("user/character_loadouts"), "content_root": ProjectSettings.globalize_path("res://../dice-and-destiny-server/content")})

func card_admin(op: String, token: String = "", card_id: String = "", revision: int = 0) -> Dictionary:
	return _request({"op": op, "admin_token": token, "card_id": card_id, "catalog_revision": revision, "loadout_root": WorkspacePaths.runtime_dir("user/character_loadouts"), "content_root": ProjectSettings.globalize_path("res://../dice-and-destiny-server/content")})

func card_authoring(op: String = "card_authoring", card: Dictionary = {}, revision: int = 0) -> Dictionary:
	if _native_authority == null: return {"ok": false, "error": _initialization_error}
	return _request({"op": op, "card": card, "catalog_revision": revision, "loadout_root": WorkspacePaths.runtime_dir("user/character_loadouts"), "content_root": ProjectSettings.globalize_path("res://../dice-and-destiny-server/content")})

func ability_authoring(op: String = "ability_authoring", ability: Dictionary = {}, revision: int = 0, character: String = "", board: Dictionary = {}) -> Dictionary:
	if _native_authority == null: return {"ok": false, "error": _initialization_error}
	return _request({"op": op, "ability": ability, "catalog_revision": revision, "character": character, "ability_board": board, "loadout_root": WorkspacePaths.runtime_dir("user/character_loadouts"), "content_root": ProjectSettings.globalize_path("res://../dice-and-destiny-server/content")})

func card_trees(op: String = "card_trees", tree: Dictionary = {}, revision: int = 0, character: String = "adventurer", token: String = "") -> Dictionary:
	return _request({"op": op, "card_tree": tree, "catalog_revision": revision, "character": character, "admin_token": token, "loadout_root": WorkspacePaths.runtime_dir("user/character_loadouts"), "content_root": ProjectSettings.globalize_path("res://../dice-and-destiny-server/content")})
