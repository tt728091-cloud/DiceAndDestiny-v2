package content

import "fmt"

// CardTargetCapabilities is shared by validation and the guided authoring UI.
// Clients must not invent target combinations which the authority cannot use.
type CardTargetSpec struct {
	SelfCountMax int      `json:"self_count_max,omitempty"`
	Owners       []string `json:"owners"`
	Modes        []string `json:"modes"`
	Selections   []string `json:"selections"`
	Filters      []string `json:"filters"`
	Notes        string   `json:"notes"`
}

func CardTargetCapabilities(effect string) CardTargetSpec {
	spec := CardTargetSpec{Owners: []string{"self", "enemy", "any"}, Modes: []string{"one", "exact", "up_to", "all"}, Selections: []string{"choose", "random"}, Filters: []string{}}
	switch CardCapabilities()[effect].Target {
	case "actor":
		spec.SelfCountMax = 1
	case "card":
		spec.Owners = []string{"self"}
		spec.Filters = []string{"zones", "exclude_cards", "exclude_recovery", "drawn_this_play"}
		spec.Notes = "Only your live cards can be selected. Removed cards cannot be recovered. The played card is excluded."
	case "offensive_die":
		spec.Filters = []string{"faces"}
		spec.Notes = "Dice must have been rolled. Enemy dice can only be edited during offensive reaction, after they are revealed."
	case "defensive_die":
		spec.Owners = []string{"self"}
		spec.Filters = []string{"faces"}
		spec.Notes = "Only your current, unfinalized defensive roll can be rerolled."
	case "status":
		spec.Filters = []string{"polarity", "status_ids"}
	case "ability":
		spec.Filters = []string{"qualified"}
	case "none":
		spec.Owners = []string{}
		spec.Modes = []string{}
		spec.Selections = []string{}
	}
	if effect == "sacrifice" {
		spec.Modes = []string{"exact"}
		spec.Selections = []string{"choose"}
		spec.Notes += " Sacrifice must be the first step of its sequence, with no condition. It costs one health per card."
	}
	return spec
}
func CardStepWindows(s CardStep) []string {
	result := append([]string{}, CardCapabilities()[s.Effect].Windows...)
	if CardCapabilities()[s.Effect].Target == "offensive_die" && s.Target.Owner == "enemy" {
		result = []string{"offensive_reaction"}
	}
	for _, option := range s.Choices {
		for _, child := range option.Steps {
			result = intersectCardWindows(result, CardStepWindows(child))
		}
	}
	return result
}
func intersectCardWindows(a, b []string) []string {
	result := []string{}
	for _, w := range a {
		if ProgramContains(b, w) {
			result = append(result, w)
		}
	}
	return result
}
func CardCompatibleWindows(steps []CardStep) []string {
	result := append([]string{}, CardWindows...)
	for _, s := range steps {
		result = intersectCardWindows(result, CardStepWindows(s))
	}
	return result
}
func ValidateCardCondition(group *RequirementGroup, lib BattleLibrary) error {
	if group == nil || len(group.All) == 0 {
		return nil
	}
	if len(group.All) > 16 {
		return fmt.Errorf("conditions require 1–16 requirements")
	}
	if err := validateTier(AbilityTier{ID: "condition", Requirements: *group, Operations: []BattleOperation{{Type: "noop"}}}, lib); err != nil {
		return err
	}
	for _, r := range group.All {
		switch r.Type {
		case "symbol_count":
			if r.Minimum < 0 || r.Maximum < 0 || r.Minimum > 100 || r.Maximum > 100 || (r.Maximum > 0 && r.Minimum > r.Maximum) {
				return fmt.Errorf("symbol count bounds must be ordered within 0–100")
			}
			if r.Exact != nil && (*r.Exact < 0 || *r.Exact > 100 || r.Minimum != 0 || r.Maximum != 0) {
				return fmt.Errorf("use exact symbol count or minimum/maximum, not both")
			}
			if r.Pattern != "" || len(r.Faces) > 0 {
				return fmt.Errorf("symbol count does not accept pattern or faces")
			}
		case "number_pattern", "exact_faces":
			if r.SymbolID != "" || r.Minimum != 0 || r.Maximum != 0 || r.Exact != nil {
				return fmt.Errorf("pattern/face requirements do not accept symbol bounds")
			}
			if r.Type == "number_pattern" && len(r.Faces) > 0 || r.Type == "exact_faces" && r.Pattern != "" {
				return fmt.Errorf("use a pattern or exact faces, not both")
			}
			for _, face := range r.Faces {
				if face < 1 || face > 100 {
					return fmt.Errorf("condition faces must be within 1–100")
				}
			}
		}
	}
	return nil
}

func CardNeedsPriorRoll(steps []CardStep) bool {
	for _, s := range steps {
		if CardCapabilities()[s.Effect].Target == "offensive_die" && s.Target.Owner == "self" || s.Effect == "ability_bonus" && s.Target.Qualified {
			return true
		}
		for _, option := range s.Choices {
			if CardNeedsPriorRoll(option.Steps) {
				return true
			}
		}
	}
	return false
}

func CardMayDrawSelf(steps []CardStep) bool {
	for _, s := range steps {
		if s.Effect == "draw" && s.Target.Owner != "enemy" {
			return true
		}
		for _, o := range s.Choices {
			if CardMayDrawSelf(o.Steps) {
				return true
			}
		}
	}
	return false
}
