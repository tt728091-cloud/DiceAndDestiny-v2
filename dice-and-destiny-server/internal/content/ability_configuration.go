package content

import "fmt"

// Ability hooks schedule the same operations used by cards and statuses.
// Conditions apply once to the completed activation, not once per rolled die.
type AbilityHook struct {
	Timing             string            `json:"timing" yaml:"timing"`
	TierID             string            `json:"tier_id,omitempty" yaml:"tier_id,omitempty"`
	FacesAny           []int             `json:"faces_any,omitempty" yaml:"faces_any,omitempty"`
	RequiresPrevention bool              `json:"requires_prevention,omitempty" yaml:"requires_prevention,omitempty"`
	Operations         []BattleOperation `json:"operations" yaml:"operations"`
}
type AbilityPayment struct {
	StatusID   string `json:"status_id" yaml:"status_id"`
	Stacks     int    `json:"stacks" yaml:"stacks"`
	Prevention int    `json:"prevention" yaml:"prevention"`
}

// SharedSpecialEffect is an explicit parameter set for owned-die mechanics.
// It is source-independent: card, ability, and status operations use it alike.
type SharedSpecialEffect struct {
	FallbackStatusID string `json:"fallback_status_id,omitempty" yaml:"fallback_status_id,omitempty"`
	FallbackStacks   int    `json:"fallback_stacks,omitempty" yaml:"fallback_stacks,omitempty"`
	Kind             string `json:"kind" yaml:"kind"`
	Amount           int    `json:"amount,omitempty" yaml:"amount,omitempty"`
	Limit            int    `json:"limit,omitempty" yaml:"limit,omitempty"`
	Face             int    `json:"face,omitempty" yaml:"face,omitempty"`
	StatusID         string `json:"status_id,omitempty" yaml:"status_id,omitempty"`
	Threshold        int    `json:"threshold,omitempty" yaml:"threshold,omitempty"`
	ResultStatusID   string `json:"result_status_id,omitempty" yaml:"result_status_id,omitempty"`
	Stacks           int    `json:"stacks,omitempty" yaml:"stacks,omitempty"`
}

func validateSpecialEffect(s *SharedSpecialEffect, lib BattleLibrary) error {
	if s == nil {
		return fmt.Errorf("special_effect requires special parameters")
	}
	for _, n := range []int{s.Amount, s.Limit, s.Threshold, s.Stacks, s.FallbackStacks} {
		if n < 0 || n > 100 {
			return fmt.Errorf("special effect values must be 0–100")
		}
	}
	switch s.Kind {
	case "curse", "roll_cursed", "curse_face_choice":
		if s.Amount < 1 {
			return fmt.Errorf("%s requires positive amount", s.Kind)
		}
	case "entomb_choice":
		if s.Amount < 1 || s.Limit < 1 || s.Limit > 5 {
			return fmt.Errorf("entomb_choice requires curse amount and entomb limit 1–5")
		}
	case "curse_die_choice":
		if s.Face < 1 || s.Face > 6 {
			return fmt.Errorf("curse_die_choice requires face 1–6")
		}
	case "conditional_status":
		if s.Threshold < 1 || s.Limit < 1 || s.Stacks < 1 || s.FallbackStacks < 1 || lib.Statuses[s.StatusID].ID == "" || lib.Statuses[s.ResultStatusID].ID == "" || lib.Statuses[s.FallbackStatusID].ID == "" {
			return fmt.Errorf("conditional_status requires threshold, result cap condition, positive stacks, and existing statuses")
		}
	case "status_threshold":
		if s.Threshold < 1 || s.Stacks < 1 || lib.Statuses[s.StatusID].ID == "" || lib.Statuses[s.ResultStatusID].ID == "" {
			return fmt.Errorf("status_threshold requires existing statuses and positive threshold/stacks")
		}
	default:
		return fmt.Errorf("unknown shared special effect %q", s.Kind)
	}
	return nil
}
func validateAbilityConfiguration(a BattleAbilityDefinition, lib BattleLibrary) error {
	if a.ConfigurationVersion < 0 || a.ConfigurationVersion > 1 {
		return fmt.Errorf("unsupported configuration version")
	}
	if a.ConfigurationVersion > 0 && a.Targeting != nil && (a.Type != "offensive" || (a.Targeting.Selector != "self" && a.Targeting.Selector != "one_enemy") || a.Targeting.Minimum != 1 || a.Targeting.Maximum != 1) {
		return fmt.Errorf("abilities target exactly one enemy or self")
	}
	if a.Cost.Energy > 100 || a.Usage.MaximumPerSegment < 0 || a.Usage.MaximumPerSegment > 100 {
		return fmt.Errorf("cost and usage must be 0–100")
	}
	if a.OptionalPayment != nil {
		p := a.OptionalPayment
		if a.Type != "defensive" || lib.Statuses[p.StatusID].ID == "" || p.Stacks < 1 || p.Stacks > 100 || p.Prevention < 1 || p.Prevention > 100 {
			return fmt.Errorf("optional payment requires defense, known status, and positive stacks/prevention (1–100)")
		}
	}
	if a.ConfigurationVersion > 0 {
		if a.Type == "offensive" && a.Targeting == nil {
			return fmt.Errorf("offensive ability requires explicit actor targeting")
		}
		if a.Selection != nil && (len(a.Selection.AllowedProposalTypes) != 1 || a.Selection.AllowedProposalTypes[0] != "damage_source") {
			return fmt.Errorf("defense must target damage_source proposals")
		}
		if a.Qualification != nil {
			for _, t := range append(append([]AbilityTier{}, a.Qualification.ActivationTiers...), a.Qualification.ConditionalBonuses...) {
				if err := ValidateCardCondition(&t.Requirements, lib); err != nil {
					return err
				}
				if err := validateAbilityOperations(t.Operations, a.Type, 0); err != nil {
					return err
				}
			}
		}
		if a.Resolution != nil {
			if len(a.Resolution.Operations) == 0 {
				return fmt.Errorf("add at least one defensive effect")
			}
			if err := validateAbilityOperations(a.Resolution.Operations, a.Type, 0); err != nil {
				return err
			}
		}
	}
	tiers := map[string]bool{}
	if a.Qualification != nil {
		if len(a.Qualification.ActivationTiers) == 0 {
			return fmt.Errorf("at least one activation tier required")
		}
		for _, t := range a.Qualification.ActivationTiers {
			if tiers[t.ID] {
				return fmt.Errorf("duplicate tier %s", t.ID)
			}
			tiers[t.ID] = true
		}
	}
	if a.Resolution != nil {
		r := a.Resolution
		if r.EnergyGainLimit < 0 || r.EnergyGainLimit > 100 {
			return fmt.Errorf("invalid energy gain limit")
		}
		if r.Roll != nil && (r.Roll.DiceCount < 1 || r.Roll.DiceCount > 5) {
			return fmt.Errorf("defense rolls require 1–5 dice")
		}
	}
	if a.Selection != nil && (!a.Selection.RequiresIncomingProposal || a.Selection.TargetCount != 1) {
		return fmt.Errorf("defenses must select one incoming source")
	}
	if len(a.Hooks) > 32 {
		return fmt.Errorf("too many ability hooks")
	}
	for _, h := range a.Hooks {
		if (a.Type == "offensive" && h.Timing != "before_defense" && h.Timing != "after_damage" && h.Timing != "after_provoke") || (a.Type == "defensive" && h.Timing != "after_defense") {
			return fmt.Errorf("hook timing %s conflicts with %s ability", h.Timing, a.Type)
		}
		if a.ConfigurationVersion > 0 && (h.Timing == "after_damage" || h.Timing == "after_provoke") && a.Qualification != nil {
			for _, tier := range a.Qualification.ActivationTiers {
				if h.TierID != "" && tier.ID != h.TierID {
					continue
				}
				hasDamage, hasProvoke := false, false
				for _, op := range tier.Operations {
					hasDamage = hasDamage || op.Type == "deal_damage"
					hasProvoke = hasProvoke || op.Type == "provoke"
				}
				if !hasDamage || (h.Timing == "after_provoke" && !hasProvoke) {
					return fmt.Errorf("%s hook needs damage%s in tier %s", h.Timing, map[bool]string{true: " and provoke", false: ""}[h.Timing == "after_provoke"], tier.ID)
				}
			}
		}
		if h.TierID != "" && !tiers[h.TierID] {
			return fmt.Errorf("unknown hook tier %s", h.TierID)
		}
		if a.Type != "defensive" && (len(h.FacesAny) > 0 || h.RequiresPrevention) {
			return fmt.Errorf("face/prevention hook conditions require defense")
		}
		for _, f := range h.FacesAny {
			if f < 1 || f > 6 {
				return fmt.Errorf("hook face must be 1–6")
			}
		}
		if len(h.Operations) == 0 || len(h.Operations) > 32 {
			return fmt.Errorf("hook needs operations")
		}
		for _, op := range h.Operations {
			switch op.Type {
			case "draw_cards", "gain_resource", "apply_status", "remove_status_stack", "special_effect", "noop":
			default:
				return fmt.Errorf("%s cannot run at an ability hook; use activation operations", op.Type)
			}
		}
		if err := validateAbilityOperations(h.Operations, a.Type, 0); err != nil {
			return err
		}
		if err := validateBattleOperations(h.Operations, lib); err != nil {
			return err
		}
	}
	return nil
}

func validateAbilityOperations(ops []BattleOperation, kind string, depth int) error {
	if depth > 4 || len(ops) > 32 {
		return fmt.Errorf("ability effects exceed nesting/count limits")
	}
	rolls := 0
	for _, op := range ops {
		if op.OnePerStatusStack || op.Repeat != "" {
			return fmt.Errorf("ability operations have no status-stack repeat context")
		}
		if op.Amount != nil {
			switch n := op.Amount.(type) {
			case int:
				if n > 100 {
					return fmt.Errorf("ability amount exceeds 100")
				}
			case float64:
				if n > 100 {
					return fmt.Errorf("ability amount exceeds 100")
				}
			}
		}
		if op.Type == "provoke" {
			hasDamage := false
			for _, sibling := range ops {
				hasDamage = hasDamage || sibling.Type == "deal_damage"
			}
			if depth > 0 || !hasDamage || (op.Target != "selected_targets" && op.Target != "enemy") {
				return fmt.Errorf("provoke needs a top-level attack damage operation and enemy target")
			}
		}
		if op.StackCount > 100 {
			return fmt.Errorf("ability stack count exceeds 100")
		}
		if op.Type == "remove_status_stack" && op.StatusID == "" {
			return fmt.Errorf("ability status removal needs a status_id")
		}
		if op.Target == "selected_proposal" && op.Type != "prevent_damage" && op.Type != "scale_damage" {
			return fmt.Errorf("%s needs an actor target", op.Type)
		}
		if kind == "defensive" && (op.Target == "selected_targets" || op.Target == "target_actor") {
			return fmt.Errorf("defense effects use self or enemy (attacker), not offensive target selection")
		}
		if kind == "offensive" && op.Target == "selected_proposal" {
			return fmt.Errorf("offensive abilities have no incoming proposal")
		}
		switch op.Type {
		case "noop", "apply_status", "draw_cards", "gain_resource", "remove_status_stack", "special_effect", "apply_incubation", "incubation_or_poison":
		case "deal_damage", "provoke":
			if kind != "offensive" {
				return fmt.Errorf("%s requires an offensive activation", op.Type)
			}
		case "prevent_damage", "scale_damage":
			if op.Type == "scale_damage" && op.Numerator > op.Denominator {
				return fmt.Errorf("defense scaling cannot increase incoming damage")
			}
			if kind != "defensive" || op.Target != "selected_proposal" {
				return fmt.Errorf("%s requires a defensive incoming-source target", op.Type)
			}
		case "roll_dice":
			rolls++
			if kind == "defensive" && (depth > 0 || rolls > 1) {
				return fmt.Errorf("defense supports one top-level roll with per-face outcomes")
			}
			if op.DiceCount < 1 || op.DiceCount > 5 || op.OnePerStatusStack {
				return fmt.Errorf("ability roll needs 1–5 dice")
			}
		default:
			return fmt.Errorf("%s requires card/status-specific input and cannot be authored on an ability", op.Type)
		}
		if op.Target == "selected_die" || op.Target == "selected_status" || op.Target == "selected_ability" || op.Target == "selected_offensive_ability" {
			return fmt.Errorf("ability does not supply %s input", op.Target)
		}
		if op.Type == "provoke" {
			if err := validateOperationAmount(op.Amount, false); err != nil {
				return err
			}
			if _, ok := op.Amount.(string); ok {
				return fmt.Errorf("provoke requires a fixed count")
			}
		}
		for _, outcome := range op.Outcomes {
			if err := validateAbilityOperations(outcome.Operations, kind, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}
