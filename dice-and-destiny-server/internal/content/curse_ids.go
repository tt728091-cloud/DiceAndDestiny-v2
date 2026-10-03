package content

// IsCurseIdentifier recognizes the additive Curse vocabulary without changing trained feature positions.
func IsCurseIdentifier(id string) bool {
	switch id {
	case "mark_the_number", "black_fingerprint", "shared_misfortune", "widen_the_crack", "rotten_numeral", "maledictions_refusal", "unquiet_hands", "no_safe_keep", "chosen_instrument", "call_the_mark", "tombs_choice", "second_knell", "three_knocks", "grave_interest", "curse_eater", "black_dividend", "curse_bloom", "ruin_made_flesh", "black_tax", "stored_calamity", "misfortunes_choice", "spiteful_ward", "hexward_retort", "blind_omen", "hexbrand", "grasp_of_the_sarcophagus", "funeral_rattle", "eclipse_of_the_black_star", "curse", "curse_d6", "curse_skull", "curse_shroud", "curse_omen", "curse_count", "grave_debt", "three_knocks_status", "cursed_entangle", "hexward_rebuttal", "misfortune_repaid", "curse_card", "curse_action":
		return true
	}
	return false
}
