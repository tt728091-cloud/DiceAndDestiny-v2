package content

// IsVenomIdentifier identifies the explicitly supported additive character pack.
// Frozen policy encoders retain zero for these new categorical IDs; existing
// vocabulary indices and weights never move. Other unknown content still fails.
func IsVenomIdentifier(id string) bool {
	switch id {
	case "accelerant", "agitate", "antivenom_draught", "apply_incubation", "barbed_mantle", "bitter_reagent", "catalyst", "coagulate", "coil", "culture_flask", "deep_puncture", "distill", "emergency_molt", "extract", "fang", "fever_cycle", "fever_spike", "forked_tongue", "gland", "incubate", "incubation", "incubation_or_poison", "measured_dose", "needlefang", "pinprick", "provoke", "repurpose", "shedskin", "shock_dose", "slow_release", "spined_rebuttal", "steady_hand", "terminal_bite", "terminal_formula", "twin_puncture", "venom", "venom_card", "venom_d6", "venom_gland", "venom_lens", "venom_reserve":
		return true
	}
	return false
}
