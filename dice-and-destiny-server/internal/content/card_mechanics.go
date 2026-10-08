package content

// Specialized mechanics describe rules that use persistent physical dice or
// suspend a play for an opponent's response. Identity never selects behavior.
import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

type CardMechanic struct {
	Kind          string         `json:"kind" yaml:"kind"`
	Params        map[string]any `json:"params" yaml:"params"`
	Windows       []string       `json:"windows" yaml:"windows"`
	UsesPerRound  int            `json:"uses_per_round" yaml:"uses_per_round"`
	UsesPerBattle int            `json:"uses_per_battle" yaml:"uses_per_battle"`
	Expiration    string         `json:"expiration,omitempty" yaml:"expiration,omitempty"`
	Rounds        int            `json:"rounds,omitempty" yaml:"rounds,omitempty"`
}
type CardMechanicSpec struct {
	Family     string                   `json:"family"`
	Label      string                   `json:"label"`
	Parameters map[string]CardParameter `json:"parameters"`
	Windows    []string                 `json:"windows"`
	Rules      string                   `json:"rules"`
}

var cardMechanicSpecs = buildCardMechanics()

// CardMechanics returns the read-only registry shared by validation and authoring.
func CardMechanics() map[string]CardMechanicSpec { return cardMechanicSpecs }

func buildCardMechanics() map[string]CardMechanicSpec {
	out := map[string]CardMechanicSpec{}
	add := func(family, id, rules string, windows []string, params map[string]CardParameter) {
		out[id] = CardMechanicSpec{family, strings.ReplaceAll(id, "_", " "), params, windows, rules}
	}
	// Each list starts with its any-time window; before/after-only variants follow.
	plan := []string{"offensive_planning", "offensive_before_roll", "offensive_after_roll"}
	react := []string{"offensive_reaction"}
	defense := []string{"defense_selection", "defense_before_roll", "defense_after_roll", "damage_reaction", "ongoing_damage"}
	n := func(v int) CardParameter { return number(v, 1, 100) }
	z := func(v int) CardParameter { return number(v, 0, 100) }
	status := func(v string) CardParameter { return CardParameter{Type: "status", Default: v} }
	for _, id := range []string{"pinprick", "twin_puncture"} {
		amount := 1
		if id == "twin_puncture" {
			amount = 2
		}
		add("venom", id, "Apply {stacks} stacks of {status_id} to an enemy through its status-response window.", plan, map[string]CardParameter{"stacks": n(amount), "status_id": status("poison")})
	}
	for _, id := range []string{"culture_flask", "bitter_reagent", "measured_dose", "extract"} {
		amount := 2
		if id == "bitter_reagent" {
			amount = 1
		}
		p := map[string]CardParameter{"stacks": n(amount), "status_id": status("catalyst")}
		if id == "extract" {
			p["cost_stacks"] = n(1)
			p["cost_status"] = status("poison")
		}
		rules := "Gain {stacks} {status_id}."
		if id == "extract" {
			rules = "Spend {cost_stacks} enemy {cost_status}; " + rules
		}
		add("venom", id, rules, plan, p)
	}
	for _, id := range []string{"slow_release", "incubate"} {
		cost := 0
		if id == "incubate" {
			cost = 1
		}
		add("venom", id, "Spend {cost_stacks} {cost_status}; apply {stacks} Incubation to a poisoned enemy without Incubation.", plan, map[string]CardParameter{"cost_stacks": z(cost), "cost_status": status("catalyst"), "stacks": n(1)})
	}
	for _, id := range []string{"distill", "accelerant"} {
		add("venom", id, "Spend {cost_stacks} {cost_status}; convert {convert_stacks} Poison to {gain_stacks} Volatile Poison. Accelerant immediately checks the converted toxins within the round's Provoke allowance.", plan, map[string]CardParameter{"cost_stacks": z(1), "cost_status": status("catalyst"), "convert_stacks": n(1), "gain_stacks": n(1)})
	}
	add("venom", "agitate", "Check every stack of the chosen toxin, up to {checks} (0 means all); does not spend the Provoke allowance.", plan, map[string]CardParameter{"checks": z(0)})
	add("venom", "fever_cycle", "Choose up to {checks} toxins to check, within the round's shared Provoke allowance.", plan, map[string]CardParameter{"checks": n(2)})
	add("venom", "repurpose", "Spend {cost_stacks} {cost_status}; draw {amount} cards without recycling discard.", plan, map[string]CardParameter{"cost_stacks": z(1), "cost_status": status("catalyst"), "amount": n(2)})
	add("venom", "venom_reserve", "Spend {cost_stacks} {cost_status}; gain {amount} energy.", plan, map[string]CardParameter{"cost_stacks": z(1), "cost_status": status("catalyst"), "amount": n(1)})
	add("venom", "shock_dose", "Spend {cost_stacks} enemy {cost_status}; deal {damage} damage in a new response window.", plan, map[string]CardParameter{"cost_stacks": n(1), "cost_status": status("volatile_poison"), "damage": n(3)})
	add("venom", "venom_lens", "Give {ability_id} +{damage} direct damage while this preparation is active. Does not stack.", plan, map[string]CardParameter{"ability_id": {Type: "ability", Default: "needlefang"}, "damage": n(1)})
	add("venom", "deep_puncture", "For the selected attack with at least {minimum_damage} damage and {status_id}: trade {damage_cost} damage for {stacks} additional status stacks.", react, map[string]CardParameter{"minimum_damage": n(2), "damage_cost": z(1), "stacks": n(1), "status_id": status("poison")})
	add("venom", "terminal_formula", "For {ability_id}, gain {damage_per_check} damage per captured toxin check this attack.", react, map[string]CardParameter{"ability_id": {Type: "ability", Default: "terminal_bite", Options: []string{"terminal_bite", "fever_spike"}}, "damage_per_check": n(1)})
	add("venom", "steady_hand", "After rolling, set one of your offensive dice to a face in {faces}.", plan, map[string]CardParameter{"faces": {Type: "integers", Default: []int{1, 4}, Minimum: 1, Maximum: 6}})
	add("venom", "forked_tongue", "Change a revealed offensive die by one of {deltas}, staying within {minimum}–{maximum}.", react, map[string]CardParameter{"deltas": {Type: "integers", Default: []int{-1, 1}, Minimum: -5, Maximum: 5}, "minimum": number(1, 1, 6), "maximum": number(6, 1, 6)})
	for _, id := range []string{"coagulate", "emergency_molt", "antivenom_draught", "spined_rebuttal"} {
		amount := 2
		p := map[string]CardParameter{}
		switch id {
		case "coagulate":
			amount = 3
			p["cost_stacks"] = n(1)
			p["cost_status"] = status("poison")
		case "emergency_molt":
			p["reward_stacks"] = n(1)
			p["reward_status"] = status("catalyst")
		case "antivenom_draught":
			p["cost_stacks"] = z(1)
			p["cost_status"] = status("catalyst")
			p["cleanse_stacks"] = n(1)
		case "spined_rebuttal":
			amount = 1
			p["stacks"] = n(1)
			p["status_id"] = status("poison")
		}
		p["prevent"] = n(amount)
		w := defense
		if id == "spined_rebuttal" {
			w = []string{"defense_selection", "defense_before_roll", "defense_after_roll", "defense_reaction"}
		}
		rules := "Prevent {prevent} damage from one incoming source."
		switch id {
		case "coagulate":
			rules = "Spend {cost_stacks} enemy {cost_status}; " + rules
		case "emergency_molt":
			rules += " Gain {reward_stacks} {reward_status} if this prevents damage."
		case "antivenom_draught":
			rules += " Optionally spend {cost_stacks} {cost_status} to cleanse {cleanse_stacks} of a chosen toxin or Incubation."
		case "spined_rebuttal":
			rules += " Apply {stacks} {status_id} to the attacker."
		}
		add("venom", id, rules, w, p)
	}
	for _, id := range []string{"mark_the_number", "hexward_retort", "maledictions_refusal"} {
		w := plan
		if id == "hexward_retort" {
			w = []string{"defense_reaction"}
		}
		rules := "Apply {curses} Curse to an enemy."
		if id == "maledictions_refusal" {
			rules += " Guard the next Curse Count cleanse: roll an owned die; a cursed result blocks the cleanse."
		}
		add("curse", id, rules, w, map[string]CardParameter{"curses": n(1)})
	}
	for _, id := range []string{"shared_misfortune", "rotten_numeral"} {
		count := 3
		clean := false
		if id == "shared_misfortune" {
			count = 2
			clean = true
		}
		add("curse", id, "Choose a face from {faces}; mark it on {dice} distinct eligible dice. Clean-only selection falls back to Lesser Curse for missing dice.", plan, map[string]CardParameter{"dice": n(count), "clean_only": {Type: "boolean", Default: clean}, "faces": {Type: "integers", Default: []int{1, 2, 3, 4, 5, 6}, Minimum: 1, Maximum: 6}})
	}
	add("curse", "widen_the_crack", "Roll a partially cursed die, then mark an adjacent uncursed face using {deltas} without wrapping.", plan, map[string]CardParameter{"deltas": {Type: "integers", Default: []int{-1, 1}, Minimum: -5, Maximum: 5}})
	add("curse", "unquiet_hands", "Roll a chosen cursed physical die {rolls} times; each result can trigger its curses.", plan, map[string]CardParameter{"rolls": n(1)})
	add("curse", "tombs_choice", "The enemy chooses: gain {count} Curse Count or roll {rolls} owned dice.", plan, map[string]CardParameter{"count": n(3), "rolls": n(5)})
	add("curse", "misfortunes_choice", "Spend {cost_count} enemy Curse Count. They choose {damage} damage or {stacks} {status_id}.", plan, map[string]CardParameter{"cost_count": n(3), "damage": n(2), "status_id": status("cursed_entangle"), "stacks": n(1)})
	add("curse", "curse_eater", "Spend {cost_count} enemy Curse Count; choose {draw} cards or {energy} energy.", plan, map[string]CardParameter{"cost_count": n(3), "draw": n(2), "energy": n(2)})
	add("curse", "ruin_made_flesh", "Spend a multiple of {cost_count} Curse Count up to {maximum_cost}; add {damage} damage per group to your selected qualified attack.", react, map[string]CardParameter{"cost_count": n(3), "maximum_cost": n(6), "damage": n(2)})
	add("curse", "blind_omen", "Requires {required_count} enemy Curse Count and a selected attack. Spend {cost_count} Count and apply {stacks} {status_id}.", react, map[string]CardParameter{"required_count": n(6), "cost_count": n(3), "status_id": status("blind"), "stacks": n(1)})
	add("curse", "spiteful_ward", "Prevent {prevent} incoming attack damage. If it prevents damage, apply {curses} Curse to the attacker.", defense, map[string]CardParameter{"prevent": n(2), "curses": n(1)})
	add("curse", "black_fingerprint", "After your next attack deals 1–6 damage, mark that numbered face on {dice} enemy dice; roll {fallback_rolls} owned dice if no eligible die remains.", plan, map[string]CardParameter{"dice": n(1), "fallback_rolls": n(1)})
	add("curse", "black_dividend", "While this preparation is active, gain {energy} energy on an enemy cursed result, once per round, up to {rewards} times.", plan, map[string]CardParameter{"energy": n(1), "rewards": n(2)})
	add("curse", "curse_bloom", "After the next Curse conversion deals damage, apply {curses} Lesser Curse.", plan, map[string]CardParameter{"curses": n(3)})
	add("curse", "three_knocks", "Multiply the next Curse conversion damage by {multiplier}; consumed at conversion.", plan, map[string]CardParameter{"multiplier": n(2)})
	for _, id := range []string{"grave_interest", "black_tax"} {
		add("curse", id, "Replace {groups} groups of next Curse conversion damage with {penalty} per group of next Income loss (energy for Grave Interest; draws for Black Tax).", plan, map[string]CardParameter{"groups": n(1), "penalty": n(1)})
	}
	add("curse", "stored_calamity", "Skip the next Curse conversion, retaining Count. Cannot be played again in the round that it delayed conversion.", plan, map[string]CardParameter{})
	add("curse", "chosen_instrument", "Choose a cursed physical die. The next random owned-die selection uses it, then consumes this preparation.", plan, map[string]CardParameter{})
	add("curse", "second_knell", "Reroll the next cursed physical-die result {retries} times. Consume before rerolling.", plan, map[string]CardParameter{"retries": number(1, 1, 10)})
	add("curse", "call_the_mark", "Set one revealed offensive die to one of its own cursed faces.", react, map[string]CardParameter{})
	add("curse", "no_safe_keep", "Reroll one enemy kept, cursed offensive die which has not already been effect-rerolled.", react, map[string]CardParameter{})
	add("general", "roll_table", "Roll using a configured outcome table.", plan, map[string]CardParameter{})
	return out
}
func MechanicKind(c BattleCardDefinition) string {
	if c.Mechanic != nil {
		return c.Mechanic.Kind
	}
	for _, op := range c.Operations {
		if op.Type == "venom_card" || op.Type == "curse_card" {
			if op.ID != "" {
				return op.ID
			}
			return c.ID
		}
	}
	return ""
}
func MechanicStep(c BattleCardDefinition) CardStep {
	s := CardStep{Params: map[string]any{}}
	for k, p := range CardMechanics()[MechanicKind(c)].Parameters {
		s.Params[k] = p.Default
	}
	if c.Mechanic != nil {
		for k, v := range c.Mechanic.Params {
			s.Params[k] = v
		}
	}
	return s
}
func MechanicInt(c BattleCardDefinition, key string) int { return ProgramInt(MechanicStep(c), key) }
func MechanicString(c BattleCardDefinition, key string) string {
	return ProgramString(MechanicStep(c), key)
}
func MechanicInts(c BattleCardDefinition, key string) []int { return ProgramInts(MechanicStep(c), key) }
func MechanicBool(c BattleCardDefinition, key string) bool  { return ProgramBool(MechanicStep(c), key) }
func EditableCard(c BattleCardDefinition) (BattleCardDefinition, error) {
	raw, err := json.Marshal(c)
	if err != nil {
		return c, err
	}
	var copy BattleCardDefinition
	if err = json.Unmarshal(raw, &copy); err != nil {
		return c, err
	}
	c = copy
	if c.Mechanic != nil {
		return c, nil
	}
	kind := MechanicKind(c)
	if kind == "" && len(c.Operations) == 1 && c.Operations[0].Type == "roll_dice" {
		kind = "roll_table"
	}
	spec, ok := CardMechanics()[kind]
	if !ok {
		return EditableGeneralCard(c)
	}
	if kind != "roll_table" {
		c.Operations[0].ID = kind
	}
	c.Mechanic = &CardMechanic{Kind: kind, Params: MechanicStep(c).Params, Windows: defaultCardWindows(spec.Windows)}
	if spec.Family == "curse" || kind == "venom_reserve" || kind == "deep_puncture" || kind == "terminal_formula" {
		c.Mechanic.UsesPerRound = 1
	}
	if kind == "venom_lens" {
		c.Mechanic.UsesPerBattle = 1
	}
	if MechanicHasStatus(kind) {
		c.Mechanic.Expiration = DefaultMechanicExpiration(kind)
		c.Mechanic.Rounds = 1
	}
	c.AccessType = spec.Family
	return c, nil
}
func MechanicRules(c BattleCardDefinition) string {
	m := c.Mechanic
	spec := CardMechanics()[m.Kind]
	text := spec.Rules
	if m.Kind == "roll_table" {
		return RollTableRules(c)
	}
	for k, v := range MechanicStep(c).Params {
		text = strings.ReplaceAll(text, "{"+k+"}", strings.ReplaceAll(fmt.Sprint(v), "_", " "))
	}
	if MechanicHasStatus(m.Kind) {
		expiry := map[string]string{"battle": "battle end", "offensive_exit": "the end of Offense", "damage_exit": "the end of damage resolution", "next_income": "Income", "next_ongoing": "ongoing effects"}[m.Expiration]
		if m.Expiration != "battle" {
			expiry += fmt.Sprintf(" (in %d round(s))", m.Rounds)
		}
		text += "\nUnused preparation expires at " + expiry + "."
	}
	if strings.Contains(text, "{") {
		return "Invalid mechanic parameters"
	}
	if m.UsesPerRound > 0 {
		text += fmt.Sprintf("\nUp to %d plays per round.", m.UsesPerRound)
	}
	if m.UsesPerBattle > 0 {
		text += fmt.Sprintf("\nUp to %d plays per battle.", m.UsesPerBattle)
	}
	if c.SavedCardDestination != "" {
		text += "\nSaved cards go to " + c.SavedCardDestination + "."
	}
	return text
}
func ValidateCardMechanic(c BattleCardDefinition, lib BattleLibrary) error {
	m := c.Mechanic
	spec, ok := CardMechanics()[m.Kind]
	if !ok {
		return fmt.Errorf("unknown mechanic %q", m.Kind)
	}
	if err := validateMechanicCommon(c, lib); err != nil {
		return err
	}
	if m.Kind == "roll_table" {
		return ValidateRollTable(c, lib)
	}
	if c.Program != nil || len(c.Operations) != 1 || c.Operations[0].Type != spec.Family+"_card" || c.Operations[0].ID != m.Kind || c.Targeting.Selector != spec.Family+"_choice" {
		return fmt.Errorf("mechanic must have its matching operation and target selector, and no program")
	}
	legacy := c.Operations[0]
	if legacy.Target == "self" && spec.Family == "curse" {
		legacy.Target = ""
	}
	if !reflect.DeepEqual(legacy, BattleOperation{Type: spec.Family + "_card", ID: m.Kind}) {
		return fmt.Errorf("configure mechanic parameters rather than legacy operation fields")
	}
	if c.Targeting.Maximum != 1 || c.Targeting.Minimum != 0 || c.Targeting.RequiresQualified || c.Targeting.RequiredFace != 0 {
		return fmt.Errorf("specialized target constraints are defined by the mechanic")
	}
	if m.UsesPerRound < 0 || m.UsesPerRound > 100 || m.UsesPerBattle < 0 || m.UsesPerBattle > 100 || len(m.Windows) == 0 {
		return fmt.Errorf("invalid usage or windows")
	}
	for _, w := range m.Windows {
		if !ProgramContains(spec.Windows, w) {
			return fmt.Errorf("%s cannot run in %s", m.Kind, w)
		}
	}
	for k, v := range m.Params {
		p, ok := spec.Parameters[k]
		if !ok {
			return fmt.Errorf("unsupported parameter %s", k)
		}
		raw, _ := json.Marshal(v)
		switch p.Type {
		case "integer":
			var n int
			if json.Unmarshal(raw, &n) != nil || n < p.Minimum || n > p.Maximum {
				return fmt.Errorf("%s must be an integer from %d to %d", k, p.Minimum, p.Maximum)
			}
		case "integers":
			var ns []int
			if json.Unmarshal(raw, &ns) != nil || len(ns) == 0 || len(ns) > 12 {
				return fmt.Errorf("%s requires 1–12 integers", k)
			}
			seen := map[int]bool{}
			for _, n := range ns {
				if n < p.Minimum || n > p.Maximum || seen[n] {
					return fmt.Errorf("invalid or duplicate %s value", k)
				}
				seen[n] = true
			}
		case "boolean":
			if _, ok := v.(bool); !ok {
				return fmt.Errorf("%s requires boolean", k)
			}
		case "status", "ability":
			id, ok := v.(string)
			if !ok {
				return fmt.Errorf("%s requires reference", k)
			}
			if p.Type == "status" {
				if _, ok = lib.Statuses[id]; !ok {
					return fmt.Errorf("unknown status %s", id)
				}
			} else if ability, exists := lib.Abilities[id]; !exists || ability.Type != "offensive" || len(p.Options) > 0 && !ProgramContains(p.Options, id) {
				return fmt.Errorf("unknown ability %s", id)
			}
		}
	}
	if m.Kind == "forked_tongue" && MechanicInt(c, "minimum") > MechanicInt(c, "maximum") {
		return fmt.Errorf("minimum exceeds maximum")
	}
	if m.Kind == "ruin_made_flesh" && MechanicInt(c, "maximum_cost") < MechanicInt(c, "cost_count") {
		return fmt.Errorf("maximum cost is less than one group")
	}
	if m.Kind == "blind_omen" && MechanicInt(c, "required_count") < MechanicInt(c, "cost_count") {
		return fmt.Errorf("required Count must cover the cost")
	}
	if c.Play.Destination != "discard" && c.Play.Destination != "hand" && c.Play.Destination != "deck" && c.Play.Destination != "removed" {
		return fmt.Errorf("invalid play destination")
	}
	for _, z := range c.Play.SourceZones {
		if z != "hand" && z != "deck" && z != "discard" {
			return fmt.Errorf("invalid source pile")
		}
	}
	if e := c.Economy; e != nil && (e.Buy < 0 || e.Sell < 0 || e.Sell > e.Buy || e.CopyLimit < 1 || e.CopyLimit > 100) {
		return fmt.Errorf("invalid economy")
	}
	return nil
}

func MechanicHasStatus(kind string) bool {
	return ProgramContains([]string{"venom_lens", "deep_puncture", "terminal_formula", "black_fingerprint", "chosen_instrument", "black_tax", "stored_calamity", "grave_interest", "second_knell", "maledictions_refusal", "black_dividend", "curse_bloom", "three_knocks", "spiteful_ward", "ruin_made_flesh"}, kind)
}
func PrepareMechanicCard(c BattleCardDefinition, lib *BattleLibrary) BattleCardDefinition {
	if c.Mechanic == nil {
		return c
	}
	c.Presentation.RulesText = MechanicRules(c)
	// Keep an authored short face summary; generated text is the fallback.
	if c.Presentation.EffectSummary == "" {
		c.Presentation.EffectSummary = strings.Split(c.Presentation.RulesText, "\n")[0]
	}
	timing := CardTimingRules(c.Mechanic.Windows, "any", false)
	if MechanicHasStatus(c.Mechanic.Kind) {
		id := MechanicStatusDefinitionID(c)
		polarity := "negative"
		if ProgramContains([]string{"venom_lens", "deep_puncture", "terminal_formula", "ruin_made_flesh"}, c.Mechanic.Kind) {
			polarity = "positive"
		}
		lib.Statuses[id] = BattleStatusDefinition{SchemaVersion: 1, ID: id, Name: c.Name + " preparation", Presentation: Presentation{RulesText: c.Presentation.RulesText, Glyph: "◆"}, ActivationMode: "automatic", Polarity: polarity, Stacking: StatusStacking{StackLimit: 1, OverflowPolicy: "reject_additional_stacks"}}
	}
	if timing != "" {
		c.Presentation.RulesText += "\n" + timing
	}
	return c
}

func MechanicStatusDefinitionID(c BattleCardDefinition) string {
	kind := MechanicKind(c)
	if c.ID == kind {
		if kind == "three_knocks" {
			return "three_knocks_status"
		}
		if ProgramContains([]string{"grave_interest", "second_knell", "maledictions_refusal", "black_dividend", "curse_bloom"}, kind) {
			return kind
		}
	}
	return c.ID + "_card_effect"
}

func RollTableRules(c BattleCardDefinition) string {
	if len(c.Operations) != 1 {
		return "Configure one roll table."
	}
	op := c.Operations[0]
	text := fmt.Sprintf("Roll %d of the target's owned dice. Each die resolves its matching numbered outcome:", op.DiceCount)
	for _, row := range op.Outcomes {
		parts := []string{}
		for _, effect := range row.Operations {
			switch effect.Type {
			case "noop":
				parts = append(parts, "no effect")
			case "deal_damage":
				parts = append(parts, fmt.Sprintf("deal %v damage", effect.Amount))
			case "draw_cards":
				parts = append(parts, fmt.Sprintf("draw %v cards", effect.Amount))
			case "gain_resource":
				parts = append(parts, fmt.Sprintf("gain %v energy", effect.Amount))
			case "apply_status":
				parts = append(parts, fmt.Sprintf("apply %d %s", effect.StackCount, effect.StatusID))
			}
		}
		text += fmt.Sprintf("\n%v: %s.", row.Faces, strings.Join(parts, ", "))
	}
	return text
}
func ValidateRollTable(c BattleCardDefinition, lib BattleLibrary) error {
	if c.Program != nil || len(c.Operations) != 1 || c.Operations[0].Type != "roll_dice" || len(c.Mechanic.Params) != 0 {
		return fmt.Errorf("roll table requires one roll_dice operation and no program or parameters")
	}
	if c.Targeting.Selector != "one_enemy" && c.Targeting.Selector != "self" {
		return fmt.Errorf("roll table must target self or one enemy")
	}
	m := c.Mechanic
	if len(m.Windows) != 1 || m.Windows[0] != "offensive_planning" {
		return fmt.Errorf("roll table requires offensive planning")
	}
	op := c.Operations[0]
	if op.DiceID != "standard_d6" || op.Repeat != "" || op.DiceCount < 1 || op.DiceCount > 100 || op.Target != "selected_targets" || op.OnePerStatusStack || (op.ReactionWindow != nil && op.ReactionWindow.Opens) || len(op.Outcomes) < 1 || len(op.Outcomes) > 100 {
		return fmt.Errorf("invalid roll table")
	}
	for _, row := range op.Outcomes {
		if len(row.Operations) < 1 || len(row.Operations) > 32 || len(row.Faces) == 0 {
			return fmt.Errorf("empty or oversized outcome")
		}
		for _, e := range row.Operations {
			if !ProgramContains([]string{"noop", "deal_damage", "apply_status", "gain_resource", "draw_cards"}, e.Type) {
				return fmt.Errorf("unsupported outcome effect %s", e.Type)
			}
			if e.Type != "noop" && e.Target != "selected_targets" && e.Target != "self" {
				return fmt.Errorf("unsupported outcome target")
			}
			if e.Type == "gain_resource" && e.Resource != "energy" {
				return fmt.Errorf("outcomes support energy resource")
			}
			if e.Type == "apply_status" && e.StackCount > 100 {
				return fmt.Errorf("outcome stacks exceed 100")
			}
			if e.Amount != nil {
				raw, _ := json.Marshal(e.Amount)
				var n int
				if json.Unmarshal(raw, &n) != nil || n < 0 || n > 100 {
					return fmt.Errorf("outcome amount must be an integer from 0 to 100")
				}
			}
		}
	}
	return validateBattleOperations(c.Operations, lib)
}

func DefaultMechanicExpiration(kind string) string {
	switch kind {
	case "venom_lens":
		return "battle"
	case "deep_puncture", "terminal_formula", "ruin_made_flesh":
		return "offensive_exit"
	case "black_fingerprint", "spiteful_ward":
		return "damage_exit"
	case "maledictions_refusal":
		return "next_income"
	}
	return "next_ongoing"
}
func validateMechanicCommon(c BattleCardDefinition, lib BattleLibrary) error {
	m := c.Mechanic
	if c.Cost.Energy < 0 || c.Cost.Energy > 100 || m.UsesPerRound < 0 || m.UsesPerRound > 100 || m.UsesPerBattle < 0 || m.UsesPerBattle > 100 {
		return fmt.Errorf("invalid cost or usage limit")
	}
	if c.AccessType != "" && !ProgramContains([]string{"general", "venom", "curse", "blade_warden"}, c.AccessType) {
		return fmt.Errorf("unknown card family")
	}
	if len(c.Play.SourceZones) == 0 || !ProgramContains([]string{"hand", "discard", "deck", "removed"}, c.Play.Destination) {
		return fmt.Errorf("invalid play piles")
	}
	for _, z := range c.Play.SourceZones {
		if !ProgramContains([]string{"hand", "deck", "discard"}, z) {
			return fmt.Errorf("invalid source pile")
		}
	}
	if c.Play.BeforeFirstRoll || c.ReactionWindow.Opens || c.ReactionWindow.PassRequired {
		return fmt.Errorf("mechanic timing is configured through windows")
	}
	if MechanicHasStatus(m.Kind) {
		if !ProgramContains([]string{"offensive_exit", "damage_exit", "next_income", "next_ongoing", "battle"}, m.Expiration) || m.Rounds < 1 || m.Rounds > 100 {
			return fmt.Errorf("choose an expiration and 1–100 rounds")
		}
	} else if m.Expiration != "" || m.Rounds != 0 {
		return fmt.Errorf("immediate effects do not have an expiration")
	}
	if c.Economy != nil {
		e := c.Economy
		if e.Buy < 0 || e.Buy > 1000000 || e.Sell < 0 || e.Sell > e.Buy || e.CopyLimit < 1 || e.CopyLimit > 100 {
			return fmt.Errorf("invalid economy")
		}
		seen := map[string]bool{}
		for _, u := range e.Upgrades {
			if u.To == c.ID || seen[u.To] || u.XP < 0 {
				return fmt.Errorf("invalid upgrade")
			}
			if _, ok := lib.Cards[u.To]; !ok {
				return fmt.Errorf("unknown upgrade %s", u.To)
			}
			seen[u.To] = true
		}
	}
	return nil
}
