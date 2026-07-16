class_name BattlePresentationCatalog
extends RefCounted

const CARDS := {
	"tip_it": {"name": "Tip It", "cost": 1, "text": "Change one revealed enemy face 6 to face 5.", "accent": "violet", "art": "res://assets/battle/cards/tip_it.png"},
	"loaded_die": {"name": "Loaded Die", "cost": 1, "text": "Change one owned combat die to face 6.", "accent": "gold", "art": "res://assets/battle/cards/loaded_die.png"},
	"antidote": {"name": "Antidote", "cost": 1, "text": "Remove one negative status from yourself.", "accent": "green", "art": "res://assets/battle/cards/antidote.png"},
	"sharpen_blade": {"name": "Sharpen Blade", "cost": 1, "text": "Give an Offensive ability a matching-pair Bleed bonus. Three-of-a-kind also counts.", "accent": "red", "art": "res://assets/battle/cards/sharpen_blade.png"},
	"emergency_ward": {"name": "Emergency Ward", "cost": 1, "text": "Prevent 3 damage from one revealed source.", "accent": "blue", "art": "res://assets/battle/cards/emergency_ward.png"},
	"battle_focus": {"name": "Battle Focus", "cost": 0, "text": "Draw 1 card, then gain 1 energy.", "accent": "cyan", "art": "res://assets/battle/cards/battle_focus.png"},
}

const ABILITIES := {
	"sword_cut": {"name": "Sword Cut", "recipe": "3 / 4 / 5 Swords", "text": "Deal 5 / 6 / 7 damage. Three-of-a-kind applies Bleed."},
	"shield_bash": {"name": "Shield Bash", "recipe": "2 Swords + 2 Shields", "text": "Deal 4 damage and apply Entangle."},
	"golden_edge": {"name": "Golden Edge", "recipe": "2 Swords + 1 Gold", "text": "Deal 5 damage and gain 1 energy."},
	"perfect_form": {"name": "Perfect Form", "recipe": "Faces 1, 2, 3, 4, 5", "text": "Deal 8 damage."},
	"jagged_slash": {"name": "Jagged Slash", "recipe": "3 / 4 / 5 Swords", "text": "Deal 4 / 5 / 6 damage."},
	"venom_strike": {"name": "Venom Strike", "recipe": "2 Swords + 1 Gold", "text": "Deal 3 damage and apply 2 Poison."},
	"crushing_advance": {"name": "Crushing Advance", "recipe": "2 Swords + 2 Shields", "text": "Deal 5 damage."},
	"greedy_blow": {"name": "Greedy Blow", "recipe": "2 Gold", "text": "Deal 7 damage."},
	"basic_defense": {"name": "Basic Defense", "recipe": "Select 1 source; roll 1D6", "text": "Prevent damage equal to the defense die."},
	"protect": {"name": "Protect", "recipe": "1 Energy; select 1 source", "text": "Halve that source's damage, rounded down."},
}

const STATUSES := {
	"poison": {"name": "Poison", "text": "Roll per stack: 1-4 damage, 5-6 removes a stack.", "glyph": "☠"},
	"bleed": {"name": "Bleed", "text": "Deals 1 damage per stack during Effects, then loses a stack.", "glyph": "◆"},
	"entangle": {"name": "Entangle", "text": "Reduces the next Offensive maximum rolls by one.", "glyph": "⌁"},
	"blind": {"name": "Blind", "text": "An exit roll can cancel the selected Offensive ability.", "glyph": "◉"},
}

static func card(id: String) -> Dictionary:
	return CARDS.get(id, {"name": id.replace("_", " ").capitalize(), "cost": 0, "text": "", "art": ""})

static func ability(id: String) -> Dictionary:
	return ABILITIES.get(id, {"name": id.replace("_", " ").capitalize(), "recipe": "", "text": ""})

static func status(id: String) -> Dictionary:
	return STATUSES.get(id, {"name": id.replace("_", " ").capitalize(), "text": "", "glyph": "•"})

static func symbol_for_face(face: int) -> String:
	if face <= 3: return "⚔"
	if face <= 5: return "◆"
	return "●"

static func symbol_name(face: int) -> String:
	if face <= 3: return "Sword"
	if face <= 5: return "Shield"
	return "Gold Coin"
