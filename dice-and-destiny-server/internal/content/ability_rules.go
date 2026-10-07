package content

import (
	"fmt"
	"strings"
)

func AbilityRules(a BattleAbilityDefinition, lib BattleLibrary) string {
	lines := []string{fmt.Sprintf("Cost: %d energy.", a.Cost.Energy)}
	if a.Qualification != nil {
		if a.Qualification.ChooseTier {
			lines = append(lines, "Choose one qualified tier.")
		}
		for i, t := range a.Qualification.ActivationTiers {
			lines = append(lines, tierLabel(t.ID, i)+": "+requirementWords(t.Requirements, lib)+" → "+OperationRules(t.Operations, lib))
		}
		lines = append(lines, "Misses if no activation tier qualifies.")
		for _, t := range a.Qualification.ConditionalBonuses {
			lines = append(lines, "Also, with "+requirementWords(t.Requirements, lib)+": "+OperationRules(t.Operations, lib))
		}
	}
	if a.Resolution != nil {
		r := a.Resolution
		if r.Roll != nil {
			lines = append(lines, fmt.Sprintf("Roll %d owned %s.", r.Roll.DiceCount, diceWords(r.Roll.DiceCount, r.Roll.DiceID, lib)))
		}
		lines = append(lines, OperationRules(r.Operations, lib))
		if r.EnergyGainLimit > 0 {
			lines = append(lines, fmt.Sprintf("Gain at most %d energy from this roll.", r.EnergyGainLimit))
		}
		switch d := a.SavedCardDestination; d {
		case "", "original":
			lines = append(lines, "Saved cards return to their piles.")
		default:
			lines = append(lines, "Saved cards go to "+strings.ReplaceAll(d, "_", " ")+".")
		}
		if a.Selection != nil && a.Selection.OffensiveAbilitiesOnly {
			lines = append(lines, "Only against an incoming offensive ability.")
		}
	}
	if p := a.OptionalPayment; p != nil {
		lines = append(lines, fmt.Sprintf("Optional before rolling: spend %d %s to prevent %d additional damage.", p.Stacks, lib.Statuses[p.StatusID].Name, p.Prevention))
	}
	for _, h := range a.Hooks {
		when := strings.ReplaceAll(h.Timing, "_", " ")
		when = strings.ToUpper(when[:1]) + when[1:]
		if h.TierID != "" {
			when += " (" + tierLabel(h.TierID, -1) + ")"
		}
		if len(h.FacesAny) > 0 {
			when += " if any die shows " + faceWords(h.FacesAny)
		}
		if h.RequiresPrevention {
			when += " if damage was prevented"
		}
		lines = append(lines, when+": "+OperationRules(h.Operations, lib))
	}
	switch n := a.Usage.MaximumPerSegment; {
	case n == 1:
		lines = append(lines, "Use at most once per segment.")
	case n > 1:
		lines = append(lines, fmt.Sprintf("Use at most %d times per segment.", n))
	}
	return strings.Join(lines, "\n")
}

// Tier IDs are stable keys such as "5_swords"; show them as words, and number
// generated IDs ("tier_1712345") by position instead.
func tierLabel(id string, index int) string {
	if strings.HasPrefix(id, "tier_") && index >= 0 {
		return fmt.Sprintf("Tier %d", index+1)
	}
	words := strings.ReplaceAll(id, "_", " ")
	if words == "" {
		return fmt.Sprintf("Tier %d", index+1)
	}
	return strings.ToUpper(words[:1]) + words[1:]
}
func symbolName(id string, lib BattleLibrary) string {
	if s, ok := lib.Symbols[id]; ok && s.Name != "" {
		return s.Name
	}
	return strings.ReplaceAll(id, "_", " ")
}
func diceWords(count int, id string, lib BattleLibrary) string {
	name := strings.ReplaceAll(id, "_", " ")
	if d, ok := lib.Dice[id]; ok && d.Name != "" {
		name = d.Name
	}
	if count == 1 {
		return name + " die"
	}
	return name + " dice"
}
func faceWords(faces []int) string {
	parts := make([]string, len(faces))
	for i, f := range faces {
		parts[i] = fmt.Sprint(f)
	}
	return strings.Join(parts, ", ")
}

// Operation targets as readers see them.
func targetWords(target string) string {
	switch target {
	case "self":
		return "you"
	case "selected_targets", "target_actor":
		return "the target"
	case "enemy":
		return "the enemy"
	case "selected_proposal":
		return "the incoming attack"
	case "source_actor":
		return "the attacker"
	}
	return strings.ReplaceAll(target, "_", " ")
}
func requirementWords(g RequirementGroup, lib BattleLibrary) string {
	out := []string{}
	for _, r := range g.All {
		switch r.Type {
		case "symbol_count":
			n := fmt.Sprintf("at least %d", r.Minimum)
			if r.Exact != nil {
				n = fmt.Sprintf("exactly %d", *r.Exact)
			} else if r.Maximum > 0 {
				n = fmt.Sprintf("%d–%d", r.Minimum, r.Maximum)
			}
			out = append(out, n+" × "+symbolName(r.SymbolID, lib))
		case "exact_faces":
			out = append(out, "faces "+faceWords(r.Faces))
		default:
			out = append(out, strings.ReplaceAll(r.Pattern, "_", " "))
		}
	}
	return strings.Join(out, " + ")
}
func OperationRules(ops []BattleOperation, lib BattleLibrary) string {
	out := []string{}
	for _, o := range ops {
		target := targetWords(o.Target)
		var text string
		switch o.Type {
		case "deal_damage":
			text = fmt.Sprintf("Deal %v damage to %s.", o.Amount, target)
		case "prevent_damage":
			text = fmt.Sprintf("Prevent %v damage.", o.Amount)
		case "scale_damage":
			text = fmt.Sprintf("Keep %d/%d of incoming damage, rounded down.", o.Numerator, o.Denominator)
		case "draw_cards":
			subject, verb := "The target", "draws"
			if o.Target == "self" {
				subject, verb = "You", "draw"
			}
			text = fmt.Sprintf("%s %s %v %s from the deck (no discard recycling).", subject, verb, o.Amount, plural(o.Amount, "card", "cards"))
		case "gain_resource":
			if o.Target == "self" {
				text = fmt.Sprintf("You gain %v energy.", o.Amount)
			} else {
				text = fmt.Sprintf("%s gains %v energy.", strings.ToUpper(target[:1])+target[1:], o.Amount)
			}
		case "apply_status":
			text = fmt.Sprintf("Apply %d %s to %s.", o.StackCount, lib.Statuses[o.StatusID].Name, target)
		case "remove_status_stack":
			text = fmt.Sprintf("Remove %d %s from %s.", o.StackCount, lib.Statuses[o.StatusID].Name, target)
		case "provoke":
			text = fmt.Sprintf("Provoke %v toxin checks within the shared round limit.", o.Amount)
		case "apply_incubation":
			text = "Apply 1 Incubation to the attacker."
		case "incubation_or_poison":
			text = "Apply 1 Incubation if poisoned with no Incubation; otherwise apply 1 Poison."
		case "roll_dice":
			parts := []string{}
			for _, v := range o.Outcomes {
				parts = append(parts, fmt.Sprintf("on %s, %s", faceWords(v.Faces), OperationRules(v.Operations, lib)))
			}
			text = fmt.Sprintf("Roll %d %s. Each die: %s", o.DiceCount, diceWords(o.DiceCount, o.DiceID, lib), strings.Join(parts, " "))
		case "special_effect":
			s := o.Special
			if s == nil {
				continue
			}
			switch s.Kind {
			case "curse":
				text = fmt.Sprintf("Apply %d ordinary Curse to %s. Each application marks face 1 on a random clean die; once all dice are cursed, roll an eligible owned die and mark the result. An already cursed result adds Count without another mark.", s.Amount, target)
			case "roll_cursed":
				text = fmt.Sprintf("Roll up to %d distinct cursed dice owned by %s.", s.Amount, target)
			case "entomb_choice":
				text = fmt.Sprintf("Apply %d Curse; choose an unbound cursed die to Entomb (limit %d). Entombed dice have subset priority and must join ordinary rerolls until they roll a cursed face.", s.Amount, s.Limit)
			case "curse_face_choice":
				text = fmt.Sprintf("Choose a number to curse on every enemy die; if every face is cursed, roll up to %d owned dice instead.", s.Amount)
			case "curse_die_choice":
				text = fmt.Sprintf("Choose an enemy die: seed face %d if clean, otherwise expand its Curse (or Surge if full).", s.Face)
			case "conditional_status":
				text = fmt.Sprintf("If target has at least %d %s and fewer than %d %s, apply %d %s; otherwise apply %d %s.", s.Threshold, lib.Statuses[s.StatusID].Name, s.Limit, lib.Statuses[s.ResultStatusID].Name, s.Stacks, lib.Statuses[s.ResultStatusID].Name, s.FallbackStacks, lib.Statuses[s.FallbackStatusID].Name)
			case "status_threshold":
				text = fmt.Sprintf("At %d %s, apply %d %s.", s.Threshold, lib.Statuses[s.StatusID].Name, s.Stacks, lib.Statuses[s.ResultStatusID].Name)
			}
		case "noop":
			text = "No additional effect."
		default:
			text = strings.ReplaceAll(o.Type, "_", " ") + "."
		}
		out = append(out, text)
	}
	return strings.Join(out, " ")
}

func plural(amount any, one, many string) string {
	if fmt.Sprint(amount) == "1" {
		return one
	}
	return many
}
