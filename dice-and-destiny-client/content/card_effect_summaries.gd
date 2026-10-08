extends RefCounted

# Card faces for cards whose authority text has no short form: Venom mechanic
# cards, legacy battle_v1 cards and test mocks. Program cards (Adventurer,
# General, authored) get generated faces from the authority, and Curse cards
# author theirs, so neither belongs here. Faces are a keyword and a number;
# timing is the card's ribbon and the full rules stay in rules_text.
const TEXT := {
	"accelerant": "Poison → Volatile Poison\nProvoke it · Pay 1 Catalyst",
	"agitate": "Roll every stack of one enemy toxin type",
	"alchemists_gamble": "1–4: Volatile Poison\n5: Miss · 6: 3 damage",
	"antidote": "Clear 1 debuff",
	"antivenom_draught": "Prevent 2\nOptional: 1 Catalyst to cleanse 1 toxin",
	"battle_focus": "Draw 1 card\nGain 1 Energy",
	"bitter_reagent": "Gain 1 Catalyst",
	"coagulate": "Spend 1 enemy Poison\nPrevent 3",
	"culture_flask": "Gain 2 Catalyst",
	"deep_puncture": "Trade 1 attack damage for +1 Poison",
	"distill": "Poison → Volatile Poison\nSpend 1 Catalyst",
	"emergency_molt": "Prevent 2\nIf it helps, gain 1 Catalyst",
	"emergency_ward": "Prevent 3",
	"extract": "Spend 1 enemy Poison\nGain 2 Catalyst",
	"fever_cycle": "Provoke up to 2 enemy toxins",
	"forked_tongue": "Change a revealed die by ±1",
	"incubate": "Apply 1 Incubation\nSpend 1 Catalyst",
	"loaded_die": "Set 1 die to 6",
	"measured_dose": "Gain 2 Catalyst",
	"pinprick": "Apply 1 Poison",
	"repurpose": "Spend 1 Catalyst\nDraw 2 cards",
	"sharpen_blade": "Upgrade: matching pair applies +1 Bleed",
	"shock_dose": "Spend 1 Volatile Poison\nDeal 3 damage",
	"slow_release": "Apply 1 Incubation",
	"spined_rebuttal": "Prevent 1\nApply 1 Poison",
	"steady_hand": "Set 1 die to 1 or 4",
	"terminal_formula": "Terminal Bite: +1 damage per Provoke (max +2)",
	"tip_it": "Change a 6 to a 5",
	"twin_puncture": "Apply 2 Poison",
	"venom_lens": "Needlefang: +1 damage to every tier",
	"venom_reserve": "Spend 1 Catalyst\nGain 1 Energy",
	"Mock Focus": "No effect · test card",
	"Mock Guard": "No effect · test card",
	"Mock Strike": "No effect · test card"
}
