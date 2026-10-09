package content

// Card programs are a versioned, bounded authoring language. The same field
// registry is returned to editors and used to reject ignored parameters.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

type CardProgram struct {
	Version         int        `json:"version" yaml:"version"`
	Windows         []string   `json:"windows" yaml:"windows"`
	RollRequirement string     `json:"roll_requirement" yaml:"roll_requirement"`
	UsesPerRound    int        `json:"uses_per_round" yaml:"uses_per_round"`
	UsesPerBattle   int        `json:"uses_per_battle" yaml:"uses_per_battle"`
	Steps           []CardStep `json:"steps" yaml:"steps"`
}
type CardStep struct {
	Effect    string            `json:"effect" yaml:"effect"`
	Target    CardTarget        `json:"target" yaml:"target"`
	Params    map[string]any    `json:"params" yaml:"params"`
	Condition *RequirementGroup `json:"condition,omitempty" yaml:"condition,omitempty"`
	Choices   []CardOption      `json:"choices,omitempty" yaml:"choices,omitempty"`
}
type CardOption struct {
	Name   string     `json:"name" yaml:"name"`
	Energy int        `json:"energy" yaml:"energy"`
	Steps  []CardStep `json:"steps" yaml:"steps"`
}
type CardTarget struct {
	Owner           string   `json:"owner" yaml:"owner"`
	Mode            string   `json:"mode" yaml:"mode"`
	Count           int      `json:"count" yaml:"count"`
	Selection       string   `json:"selection" yaml:"selection"`
	Zones           []string `json:"zones,omitempty" yaml:"zones,omitempty"`
	Faces           []int    `json:"faces,omitempty" yaml:"faces,omitempty"`
	Polarity        string   `json:"polarity,omitempty" yaml:"polarity,omitempty"`
	StatusIDs       []string `json:"status_ids,omitempty" yaml:"status_ids,omitempty"`
	ExcludeCards    []string `json:"exclude_cards,omitempty" yaml:"exclude_cards,omitempty"`
	ExcludeRecovery bool     `json:"exclude_recovery,omitempty" yaml:"exclude_recovery,omitempty"`
	DrawnThisPlay   bool     `json:"drawn_this_play,omitempty" yaml:"drawn_this_play,omitempty"`
	Qualified       bool     `json:"qualified,omitempty" yaml:"qualified,omitempty"`
}
type CardEconomy struct {
	Buy       int           `json:"buy" yaml:"buy"`
	Sell      int           `json:"sell" yaml:"sell"`
	CopyLimit int           `json:"copy_limit" yaml:"copy_limit"`
	Upgrades  []CardUpgrade `json:"upgrades" yaml:"upgrades"`
}
type CardUpgrade struct {
	To string `json:"to" yaml:"to"`
	XP int    `json:"xp" yaml:"xp"`
}
type CardParameter struct {
	Type    string   `json:"type"`
	Default any      `json:"default"`
	Minimum int      `json:"minimum,omitempty"`
	Maximum int      `json:"maximum,omitempty"`
	Options []string `json:"options,omitempty"`
}
type CardEffectSpec struct {
	Label      string                   `json:"label"`
	Target     string                   `json:"target"`
	Parameters map[string]CardParameter `json:"parameters"`
	Windows    []string                 `json:"windows"`
}

// CardTimingChoices (card_timing.go) maps the player-facing before/after/any
// choices onto these windows.
var CardWindows = []string{"offensive_before_roll", "offensive_planning", "offensive_after_roll", "offensive_reaction", "defense_before_roll", "defense_selection", "defense_after_roll", "defense_reaction", "damage_reaction"}

func number(defaultValue, minimum, maximum int) CardParameter {
	return CardParameter{Type: "integer", Default: defaultValue, Minimum: minimum, Maximum: maximum}
}
func choice(defaultValue string, values ...string) CardParameter {
	return CardParameter{Type: "enum", Default: defaultValue, Options: values}
}
func CardCapabilities() map[string]CardEffectSpec {
	all := CardWindows
	// Die effects need rolled dice, so they never play before the first roll.
	offense := []string{"offensive_planning", "offensive_after_roll", "offensive_reaction"}
	defense := []string{"defense_before_roll", "defense_selection", "defense_after_roll", "defense_reaction", "damage_reaction"}
	dest := choice("original", "original", "discard", "hand", "deck", "removed")
	return map[string]CardEffectSpec{
		"curse":              {"Apply ordinary Curse", "actor", map[string]CardParameter{"amount": number(1, 1, 100)}, all},
		"roll_cursed":        {"Roll cursed owned dice", "actor", map[string]CardParameter{"amount": number(1, 1, 5)}, all},
		"conditional_status": {"Apply a conditional status", "actor", map[string]CardParameter{"status_id": {Type: "status", Default: "poison"}, "threshold": number(1, 1, 100), "result_status_id": {Type: "status", Default: "incubation"}, "limit": number(1, 1, 100), "stacks": number(1, 1, 100), "fallback_status_id": {Type: "status", Default: "poison"}, "fallback_stacks": number(1, 1, 100)}, all},
		"status_threshold":   {"Apply a status at a stack threshold", "actor", map[string]CardParameter{"status_id": {Type: "status", Default: "curse_count"}, "threshold": number(6, 1, 100), "result_status_id": {Type: "status", Default: "blind"}, "stacks": number(1, 1, 100)}, all},
		"draw":               {"Draw cards", "actor", map[string]CardParameter{"amount": number(1, 1, 100)}, all},
		"energy":             {"Gain energy", "actor", map[string]CardParameter{"amount": number(1, 1, 100)}, all},
		"prevent":            {"Prevent damage", "source", map[string]CardParameter{"amount": number(3, 1, 100), "destination": dest}, defense},
		"save_cards":         {"Save chosen threatened cards", "threatened_card", map[string]CardParameter{"destination": dest}, defense},
		"move_cards":         {"Move cards", "card", map[string]CardParameter{"destination": choice("hand", "hand", "deck", "discard", "removed")}, all},
		"sacrifice":          {"Sacrifice cards as a cost", "card", map[string]CardParameter{}, all},
		"set_die":            {"Set die face", "offensive_die", map[string]CardParameter{"faces": {Type: "integers", Default: []int{6}, Minimum: 1, Maximum: 100}}, offense},
		"adjust_die":         {"Adjust die face", "offensive_die", map[string]CardParameter{"deltas": {Type: "integers", Default: []int{-1, 1}, Minimum: -100, Maximum: 100}, "minimum": number(1, 1, 100), "maximum": number(5, 1, 100), "wrap": {Type: "boolean", Default: false}}, offense},
		"flip_die":           {"Flip die face", "offensive_die", map[string]CardParameter{"sum": number(7, 2, 200)}, offense},
		"copy_die":           {"Copy another die face", "offensive_die", map[string]CardParameter{}, offense},
		"reroll":             {"Reroll offensive dice", "offensive_die", map[string]CardParameter{"result": choice("replace", "replace", "higher", "lower"), "consume_roll": {Type: "boolean", Default: false}}, offense},
		"reroll_defense":     {"Reroll defensive dice", "defensive_die", map[string]CardParameter{"result": choice("replace", "replace", "higher", "lower")}, []string{"defense_reaction"}},
		"remove_status":      {"Remove statuses", "status", map[string]CardParameter{"stacks": number(0, 0, 100)}, all},
		"apply_status":       {"Apply status", "actor", map[string]CardParameter{"status_id": {Type: "status", Default: ""}, "stacks": number(1, 1, 100)}, all},
		"ability_bonus":      {"Add an ability bonus", "ability", map[string]CardParameter{"damage": number(2, 0, 100), "status_id": {Type: "status_optional", Default: ""}, "stacks": number(1, 1, 100), "duration": choice("offensive", "offensive", "round", "next_use", "rounds", "battle"), "rounds": number(1, 1, 100), "stack_limit": number(1, 1, 100), "stacking": choice("stack", "stack", "replace", "refresh"), "preparation_status": {Type: "status_optional", Default: ""}}, []string{"offensive_before_roll", "offensive_planning", "offensive_after_roll"}},
		"choice":             {"Choose an option", "none", map[string]CardParameter{}, all},
	}
}
func ProgramInt(s CardStep, key string) int {
	v, ok := s.Params[key]
	if !ok {
		v = CardCapabilities()[s.Effect].Parameters[key].Default
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return 0
}
func ProgramString(s CardStep, key string) string {
	v, ok := s.Params[key]
	if !ok {
		v = CardCapabilities()[s.Effect].Parameters[key].Default
	}
	s1, _ := v.(string)
	return s1
}
func ProgramBool(s CardStep, key string) bool { b, _ := s.Params[key].(bool); return b }
func ProgramInts(s CardStep, key string) []int {
	v, ok := s.Params[key]
	if !ok {
		v = CardCapabilities()[s.Effect].Parameters[key].Default
	}
	raw, _ := json.Marshal(v)
	var out []int
	_ = json.Unmarshal(raw, &out)
	return out
}
func ProgramContains(values []string, v string) bool {
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}
func ValidateCardProgram(card BattleCardDefinition, lib BattleLibrary) error {
	p := card.Program
	if p == nil || p.Version != 1 || len(p.Windows) == 0 || len(p.Steps) == 0 || len(p.Steps) > 32 {
		return fmt.Errorf("card program requires version 1, windows and 1–32 steps")
	}
	if !ProgramContains([]string{"any", "before_first", "after_first"}, p.RollRequirement) || p.UsesPerRound < 0 || p.UsesPerBattle < 0 {
		return fmt.Errorf("invalid roll requirement or usage limits")
	}
	if (p.RollRequirement == "before_first" || ProgramContains(p.Windows, "offensive_before_roll")) && CardNeedsPriorRoll(p.Steps) {
		return fmt.Errorf("these effects require a prior offensive roll; before-first-roll timing is impossible")
	}
	for _, w := range p.Windows {
		if !ProgramContains(CardWindows, w) {
			return fmt.Errorf("unsupported card window %q", w)
		}
	}
	for _, z := range card.Play.SourceZones {
		if !ProgramContains([]string{"hand", "discard", "deck"}, z) {
			return fmt.Errorf("unsupported playable source %q", z)
		}
	}
	if !ProgramContains([]string{"hand", "discard", "deck", "removed"}, card.Play.Destination) {
		return fmt.Errorf("unsupported play destination")
	}
	if len(card.Operations) > 0 || card.Targeting.Selector != "card_program" {
		return fmt.Errorf("program cards cannot mix legacy operations or targeting")
	}
	if card.SavedCardDestination != "" {
		return fmt.Errorf("program prevention destinations belong on each effect")
	}
	if card.ReactionWindow.Opens || card.ReactionWindow.PassRequired || card.Play.BeforeFirstRoll || len(card.Play.PlayableDuring) > 0 {
		return fmt.Errorf("program timing belongs in program.windows and roll_requirement")
	}
	if err := validateProgramSteps(p.Steps, p.Windows, lib, 0, false); err != nil {
		return err
	}
	if revive, cost := ProgramReviveCount(p.Steps), ProgramRemovalCost(card); revive > cost {
		return fmt.Errorf("reviving %d removed cards is a trade: remove at least %d cards as its cost (this card's removed play destination plus sacrifices), found %d", revive, revive, cost)
	}
	if c := card.Economy; c != nil {
		if c.Buy < 0 || c.Buy > 1000000 || c.Sell < 0 || c.Sell > c.Buy || c.CopyLimit < 1 || c.CopyLimit > 100 {
			return fmt.Errorf("invalid card economy (sale price must not exceed buy price)")
		}
		seen := map[string]bool{}
		for _, u := range c.Upgrades {
			if u.To == card.ID || seen[u.To] || u.XP < 0 {
				return fmt.Errorf("invalid upgrade branch")
			}
			if _, ok := lib.Cards[u.To]; !ok {
				return fmt.Errorf("unknown upgrade %s", u.To)
			}
			seen[u.To] = true
		}
	}
	return nil
}
func validateProgramSteps(steps []CardStep, windows []string, lib BattleLibrary, depth int, priorDraw bool) error {
	if depth > 4 || len(steps) == 0 || len(steps) > 32 {
		return fmt.Errorf("effect sequence is empty or exceeds nesting/step limit")
	}
	effectsStarted := false
	sacrificeSeen := false
	for _, s := range steps {
		spec, ok := CardCapabilities()[s.Effect]
		if !ok {
			return fmt.Errorf("unknown effect %q", s.Effect)
		}
		for _, w := range windows {
			if !ProgramContains(CardStepWindows(s), w) {
				return fmt.Errorf("%s cannot run in %s", s.Effect, w)
			}
		}
		if s.Effect == "sacrifice" {
			if sacrificeSeen || effectsStarted || s.Condition != nil || s.Target.Owner != "self" || s.Target.Mode != "exact" || s.Target.Selection != "choose" {
				return fmt.Errorf("sacrifice must be an unconditional exact self cost before effects")
			}
			sacrificeSeen = true
		} else {
			effectsStarted = true
		}
		if err := ValidateCardCondition(s.Condition, lib); err != nil {
			return err
		}

		for key, value := range s.Params {
			field, ok := spec.Parameters[key]
			if !ok {
				return fmt.Errorf("%s does not support parameter %q", s.Effect, key)
			}
			raw, _ := json.Marshal(value)
			switch field.Type {
			case "integer":
				var n int
				if json.Unmarshal(raw, &n) != nil || n < field.Minimum || n > field.Maximum {
					return fmt.Errorf("%s.%s must be an integer in %d–%d", s.Effect, key, field.Minimum, field.Maximum)
				}
			case "integers":
				var ns []int
				if json.Unmarshal(raw, &ns) != nil || len(ns) == 0 || len(ns) > 100 {
					return fmt.Errorf("%s must be a nonempty integer list", key)
				}
				for _, n := range ns {
					if n < field.Minimum || n > field.Maximum {
						return fmt.Errorf("%s value outside range", key)
					}
				}
			case "enum":
				v, ok := value.(string)
				if !ok || !ProgramContains(field.Options, v) {
					return fmt.Errorf("invalid %s option", key)
				}
			case "boolean":
				if _, ok := value.(bool); !ok {
					return fmt.Errorf("%s must be boolean", key)
				}
			case "status", "status_optional":
				v, ok := value.(string)
				if !ok {
					return fmt.Errorf("status must be an id")
				}
				if v != "" || field.Type == "status" {
					if _, ok := lib.Statuses[v]; !ok {
						return fmt.Errorf("unknown status %q", v)
					}
				}
			}
		}
		if s.Effect == "apply_status" && ProgramString(s, "status_id") == "" {
			return fmt.Errorf("apply_status requires a status")
		}
		if s.Effect == "ability_bonus" && ProgramInt(s, "damage") == 0 && ProgramString(s, "status_id") == "" {
			return fmt.Errorf("ability bonus needs damage or status")
		}
		if s.Effect == "adjust_die" && ProgramInt(s, "maximum") < ProgramInt(s, "minimum") {
			return fmt.Errorf("die range is reversed")
		}
		if s.Effect == "choice" {
			if len(s.Choices) < 1 || len(s.Choices) > 16 {
				return fmt.Errorf("choice requires 1–16 options")
			}
			for _, o := range s.Choices {
				if strings.TrimSpace(o.Name) == "" || o.Energy < 0 || o.Energy > 100 {
					return fmt.Errorf("invalid option")
				}
				if err := validateProgramSteps(o.Steps, windows, lib, depth+1, priorDraw); err != nil {
					return err
				}
			}
		} else if len(s.Choices) > 0 {
			return fmt.Errorf("only choice accepts options")
		}
		if spec.Target == "none" {
			priorDraw = priorDraw || CardMayDrawSelf([]CardStep{s})
			continue
		}
		t := s.Target
		if t.DrawnThisPlay && !priorDraw {
			return fmt.Errorf("drawn-this-play targeting needs an earlier draw for yourself")
		}
		priorDraw = priorDraw || CardMayDrawSelf([]CardStep{s})
		targetSpec := CardTargetCapabilities(s.Effect)
		if t.Owner == "self" && t.Mode == "exact" && targetSpec.SelfCountMax > 0 && t.Count > targetSpec.SelfCountMax {
			return fmt.Errorf("self targeting for %s has only one character", s.Effect)
		}
		if !ProgramContains(targetSpec.Owners, t.Owner) || !ProgramContains(targetSpec.Modes, t.Mode) || !ProgramContains(targetSpec.Selections, t.Selection) {
			return fmt.Errorf("invalid %s targeting", s.Effect)
		}
		if (t.Mode == "exact" || t.Mode == "up_to") && (t.Count < 1 || t.Count > 100) {
			return fmt.Errorf("target count must be 1–100")
		}
		if spec.Target != "card" && (len(t.Zones) > 0 || len(t.ExcludeCards) > 0 || t.ExcludeRecovery || t.DrawnThisPlay) {
			return fmt.Errorf("card filters require card targets")
		}
		if spec.Target == "card" {
			if t.Owner != "self" {
				return fmt.Errorf("card selection currently uses owned cards to preserve private piles")
			}
			if len(t.Zones) == 0 {
				return fmt.Errorf("card targets require source piles")
			}
			for _, z := range t.Zones {
				if z == "removed" {
					// Reviving is an authored trade (see ProgramReviveCount).
					if s.Effect != "move_cards" || ProgramString(s, "destination") == "removed" || len(t.Zones) != 1 || t.Mode == "all" {
						return fmt.Errorf("removed cards can only be revived on their own, by a fixed number of cards, to a live pile")
					}
					continue
				}
				if !ProgramContains([]string{"hand", "discard", "deck"}, z) {
					return fmt.Errorf("unsupported card pile %q", z)
				}
			}
		}
		if len(t.Faces) > 0 && spec.Target != "offensive_die" && spec.Target != "defensive_die" {
			return fmt.Errorf("face filters require dice")
		}
		if (t.Polarity != "" || len(t.StatusIDs) > 0) && spec.Target != "status" {
			return fmt.Errorf("status filters require statuses")
		}
		if t.Polarity != "" && !ProgramContains([]string{"positive", "negative", "any"}, t.Polarity) {
			return fmt.Errorf("invalid polarity")
		}
		for _, id := range t.StatusIDs {
			def, ok := lib.Statuses[id]
			if !ok {
				return fmt.Errorf("unknown status filter %s", id)
			}
			// A filter outside the chosen polarity could never be targeted.
			if (t.Polarity == "positive" || t.Polarity == "negative") && def.Polarity != t.Polarity {
				return fmt.Errorf("status filter %s is %s, but this effect only targets %s statuses", def.Name, def.Polarity, t.Polarity)
			}
		}
		for _, id := range t.ExcludeCards {
			if _, ok := lib.Cards[id]; !ok {
				return fmt.Errorf("unknown excluded card %s", id)
			}
		}
		for _, face := range t.Faces {
			if face < 1 || face > 100 {
				return fmt.Errorf("face filter outside 1–100")
			}
		}
		if t.Qualified && spec.Target != "ability" {
			return fmt.Errorf("qualification filter requires abilities")
		}

	}
	return nil
}
func ProgramDurationRules(s CardStep) string {
	switch ProgramString(s, "duration") {
	case "offensive":
		return "Expires at the end of the offensive segment, even if unused."
	case "round":
		return "Expires after this round's damage resolves."
	case "rounds":
		return fmt.Sprintf("Expires after %d rounds, including this round, when damage resolves.", ProgramInt(s, "rounds"))
	case "next_use":
		return "Consumed when the selected ability next qualifies and resolves; a miss does not consume it."
	case "battle":
		return "Lasts until the battle ends."
	}
	return ""
}
func programTargetWords(t CardTarget, noun string, statuses map[string]BattleStatusDefinition) string {
	count := "one"
	switch t.Mode {
	case "exact":
		count = fmt.Sprintf("%d", t.Count)
	case "up_to":
		count = fmt.Sprintf("up to %d", t.Count)
		// The editor's maximum count reads as "any number".
		if t.Count >= 100 {
			count = "any"
		}
	case "all":
		count = "all eligible"
	}
	owner := "your"
	switch t.Owner {
	case "enemy":
		owner = "an enemy's"
	case "any":
		owner = "any participant's"
	}
	if noun == "statuses" {
		switch t.Polarity {
		case "negative":
			noun = "debuffs (negative statuses)"
		case "positive":
			noun = "buffs (positive statuses)"
		}
	}
	if t.Selection == "random" {
		count += " random"
	}
	text := count + " of " + owner + " " + noun
	if len(t.Zones) > 0 {
		text += " in " + strings.Join(t.Zones, "/")
	}
	if t.DrawnThisPlay {
		text += " drawn by this card"
	}
	if t.ExcludeRecovery {
		text += " (excluding recovery cards)"
	}
	if len(t.ExcludeCards) > 0 {
		text += " (excluding " + strings.Join(t.ExcludeCards, ", ") + ")"
	}
	if len(t.Faces) > 0 {
		text += " showing " + orList(t.Faces)
	}
	if len(t.StatusIDs) > 0 {
		names := make([]string, 0, len(t.StatusIDs))
		for _, id := range t.StatusIDs {
			if def, ok := statuses[id]; ok && def.Name != "" {
				names = append(names, def.Name)
			} else {
				names = append(names, id)
			}
		}
		text += " matching " + strings.Join(names, ", ")
	}
	if t.Qualified {
		text += " that currently qualify"
	}
	return text
}
func programDestinationWords(s CardStep) string {
	d := ProgramString(s, "destination")
	if d == "original" {
		return "Saved cards stay in their current piles."
	}
	if d == "removed" {
		return "Saved cards are permanently removed instead; each still costs one health."
	}
	return "Saved cards go to " + d + "."
}
func ProgramConditionRules(g *RequirementGroup) string {
	if g == nil {
		return ""
	}
	parts := []string{}
	for _, r := range g.All {
		count := fmt.Sprintf("at least %d", r.Minimum)
		if r.Exact != nil {
			count = fmt.Sprintf("exactly %d", *r.Exact)
		} else if r.Maximum > 0 {
			count += fmt.Sprintf(" and at most %d", r.Maximum)
		}
		switch r.Type {
		case "symbol_count":
			parts = append(parts, count+" "+r.SymbolID+" symbols")
		case "number_pattern":
			parts = append(parts, strings.ReplaceAll(r.Pattern, "_", " "))
		case "exact_faces":
			parts = append(parts, fmt.Sprintf("dice showing %v", r.Faces))
		default:
			parts = append(parts, strings.ReplaceAll(r.Type, "_", " ")+" "+count)
		}
	}
	return strings.Join(parts, " and ")
}
func CardProgramRules(p *CardProgram) string {
	return CardProgramRulesWithStatuses(p, nil)
}

// CardProgramRulesWithStatuses names status filters from the library; without
// one, filters fall back to their IDs.
func CardProgramRulesWithStatuses(p *CardProgram, statuses map[string]BattleStatusDefinition) string {
	if p == nil {
		return ""
	}
	var lines []string
	var describe func([]CardStep, string)
	describe = func(steps []CardStep, prefix string) {
		for _, s := range steps {
			target := func(noun string) string { return programTargetWords(s.Target, noun, statuses) }
			text := ""
			switch s.Effect {
			case "curse":
				text = fmt.Sprintf("Apply %d ordinary Curse to %s.", ProgramInt(s, "amount"), target("characters"))
			case "roll_cursed":
				text = fmt.Sprintf("Roll up to %d distinct cursed dice owned by %s.", ProgramInt(s, "amount"), target("characters"))
			case "conditional_status":
				text = fmt.Sprintf("For %s: at %d %s and below %d %s, apply %d %s; otherwise apply %d %s.", target("characters"), ProgramInt(s, "threshold"), ProgramString(s, "status_id"), ProgramInt(s, "limit"), ProgramString(s, "result_status_id"), ProgramInt(s, "stacks"), ProgramString(s, "result_status_id"), ProgramInt(s, "fallback_stacks"), ProgramString(s, "fallback_status_id"))
			case "status_threshold":
				text = fmt.Sprintf("If %s has at least %d %s, apply %d %s.", target("characters"), ProgramInt(s, "threshold"), ProgramString(s, "status_id"), ProgramInt(s, "stacks"), ProgramString(s, "result_status_id"))
			case "draw":
				text = fmt.Sprintf("Draw %s for %s. If the deck runs short, draw what is left; discard is never reshuffled.", counted(ProgramInt(s, "amount"), "card", "cards"), target("characters"))
			case "energy":
				text = fmt.Sprintf("Give %d energy to %s.", ProgramInt(s, "amount"), target("characters"))
			case "prevent":
				text = fmt.Sprintf("Prevent %d damage from %s. %s", ProgramInt(s, "amount"), target("incoming attacks"), programDestinationWords(s))
			case "save_cards":
				text = "Save " + target("threatened cards") + ". " + programDestinationWords(s)
			case "move_cards":
				text = "Move " + target("cards") + " to " + ProgramString(s, "destination") + "."
				if ProgramContains(s.Target.Zones, "removed") {
					revived := s.Target
					revived.Zones = nil
					text = "Revive " + programTargetWords(revived, "permanently removed cards", statuses) + " to " + ProgramString(s, "destination") + ". Each revived card heals 1 health of the wound that removed it."
				}
				if ProgramString(s, "destination") == "removed" {
					text += " Each permanently removed card costs one health."
				}
			case "sacrifice":
				text = "First sacrifice " + target("other cards") + ". Permanently remove them and lose one health per card before gaining the effects below."
			case "set_die":
				text = fmt.Sprintf("Set %s to %s.", target("offensive dice"), orList(ProgramInts(s, "faces")))
			case "adjust_die":
				text = fmt.Sprintf("Change %s by %s.", target("offensive dice"), deltaWords(ProgramInts(s, "deltas")))
				if ProgramBool(s, "wrap") {
					text += fmt.Sprintf(" Results wrap around within %d–%d.", ProgramInt(s, "minimum"), ProgramInt(s, "maximum"))
				} else {
					text += fmt.Sprintf(" The result must stay within %d–%d.", ProgramInt(s, "minimum"), ProgramInt(s, "maximum"))
				}
			case "copy_die":
				text = "Set " + target("offensive dice") + " to another die's current face on the same character."
			case "flip_die":
				text = fmt.Sprintf("Flip %s: the old and new faces must total %d.", target("offensive dice"), ProgramInt(s, "sum"))
			case "reroll", "reroll_defense":
				noun := "offensive dice"
				if s.Effect == "reroll_defense" {
					noun = "defensive dice"
				}
				text = "Reroll " + target(noun) + "."
				switch ProgramString(s, "result") {
				case "higher":
					text += " Keep the higher result."
				case "lower":
					text += " Keep the lower result."
				}
				if s.Effect == "reroll" {
					if ProgramBool(s, "consume_roll") {
						text += " Uses one normal roll attempt per affected character."
					} else {
						text += " Does not use a normal roll attempt."
					}
				}
			case "remove_status":
				count := counted(ProgramInt(s, "stacks"), "stack", "stacks")
				if ProgramInt(s, "stacks") == 0 {
					count = "all stacks"
				}
				text = "Remove " + count + " from " + target("statuses") + "."
			case "apply_status":
				text = fmt.Sprintf("Apply %d %s to %s.", ProgramInt(s, "stacks"), statusName(ProgramString(s, "status_id"), statuses), target("characters"))
			case "ability_bonus":
				text = fmt.Sprintf("Give %s +%d damage", target("offensive abilities"), ProgramInt(s, "damage"))
				if ProgramString(s, "status_id") != "" {
					text += fmt.Sprintf(" and %d %s on a successful attack", ProgramInt(s, "stacks"), statusName(ProgramString(s, "status_id"), statuses))
				}
				text += ". " + ProgramDurationRules(s)
				switch ProgramString(s, "stacking") {
				case "stack":
					// The editor's maximum limit means copies stack freely.
					if ProgramInt(s, "stack_limit") >= 100 {
						text += " Copies stack on the same ability."
					} else {
						text += fmt.Sprintf(" Stacks up to %d times per ability.", ProgramInt(s, "stack_limit"))
					}
				case "replace":
					text += " Replaces the previous bonus on that ability."
				case "refresh":
					text += " Refreshes the existing bonus duration without adding a stack."
				}
			case "choice":
				text = "Choose one option:"
			}
			if condition := ProgramConditionRules(s.Condition); condition != "" {
				text = "Only with " + condition + ": " + text
			}
			lines = append(lines, prefix+text)
			for _, o := range s.Choices {
				if o.Energy > 0 {
					lines = append(lines, prefix+fmt.Sprintf("%s (costs %d more energy):", o.Name, o.Energy))
				} else {
					lines = append(lines, prefix+o.Name+":")
				}
				describe(o.Steps, prefix+"  ")
			}
		}
	}
	describe(p.Steps, "")
	if limit := CardPlayLimitRules(p.UsesPerRound, p.UsesPerBattle); limit != "" {
		lines = append(lines, limit)
	}
	if timing := CardTimingRules(p.Windows, p.RollRequirement, CardNeedsPriorRoll(p.Steps)); timing != "" {
		lines = append(lines, timing)
	}
	return strings.Join(lines, "\n")
}

// ProgramPreparationStatus is the visible status an ability bonus applies: an
// authored status when the effect names one (with its own rules and expiry
// text), otherwise a generated per-card preparation status.
// ProgramReviveCount is the most removed cards one play can return: each
// revive step's fixed count, taking the largest option of a choice.
func ProgramReviveCount(steps []CardStep) int {
	total := 0
	for _, s := range steps {
		if s.Effect == "move_cards" && ProgramContains(s.Target.Zones, "removed") {
			if s.Target.Mode == "exact" || s.Target.Mode == "up_to" {
				total += s.Target.Count
			} else {
				total++
			}
		}
		best := 0
		for _, o := range s.Choices {
			best = max(best, ProgramReviveCount(o.Steps))
		}
		total += best
	}
	return total
}

// ProgramRemovalCost counts the cards a play permanently removes as its cost:
// the card itself when its play destination is removed, plus sacrifices.
func ProgramRemovalCost(card BattleCardDefinition) int {
	cost := 0
	if card.Play.Destination == "removed" {
		cost++
	}
	if card.Program != nil {
		for _, s := range card.Program.Steps {
			if s.Effect == "sacrifice" {
				cost += s.Target.Count
			}
		}
	}
	return cost
}

func ProgramPreparationStatus(card string, step CardStep) string {
	if id := ProgramString(step, "preparation_status"); id != "" {
		return id
	}
	return ProgramStatusID(card, step)
}

// ProgramStatusID includes the effect parameters, avoiding collisions when a
// card has several independently configured preparations.
func ProgramStatusID(card string, step CardStep) string {
	raw, _ := json.Marshal(step)
	h := sha256.Sum256(raw)
	return card + "_preparation_" + hex.EncodeToString(h[:6])
}

// PresentProgramCard generates a program card's player-facing text: the short
// face, the timing ribbon and the full rules shown on hover.
func PresentProgramCard(card *BattleCardDefinition, statuses map[string]BattleStatusDefinition) {
	p := card.Program
	card.Presentation.RulesText = CardProgramRulesWithStatuses(p, statuses)
	card.Presentation.EffectSummary = CardProgramFace(p, statuses)
	if card.Play.Destination == "removed" {
		// Removing itself is part of the card's cost, so the rules state it
		// before the timing line and the face shows it.
		self := "When played, this card is permanently removed instead of discarded (−1 health)."
		lines := strings.Split(card.Presentation.RulesText, "\n")
		limits := strings.Split(CardPlayLimitRules(p.UsesPerRound, p.UsesPerBattle), "\n")
		at := len(lines)
		for i, line := range lines {
			if strings.HasPrefix(line, "Play:") || slices.Contains(limits, line) && line != "" {
				at = i
				break
			}
		}
		card.Presentation.RulesText = strings.Join(slices.Insert(lines, at, self), "\n")
		card.Presentation.EffectSummary += "\nRemoves itself (−1 health)"
	}
	card.Presentation.Timing = CardTimingTags(p.Windows, p.RollRequirement, CardNeedsPriorRoll(p.Steps))
	card.Presentation.PlayLimit = CardPlayLimit(p.UsesPerRound, p.UsesPerBattle)
	card.Presentation.Target = CardStatusTarget(p.Steps)
}

func PrepareProgramCard(card BattleCardDefinition, lib *BattleLibrary) BattleCardDefinition {
	if card.Program == nil {
		return card
	}
	if card.Operations == nil {
		card.Operations = []BattleOperation{}
	}
	if card.Play.PlayableDuring == nil {
		card.Play.PlayableDuring = []PlayTiming{}
	}
	// Steps without parameters (e.g. a sacrifice cost) store {} rather than
	// null, so every reader can treat params as a dictionary.
	var normalize func([]CardStep)
	normalize = func(steps []CardStep) {
		for i := range steps {
			if steps[i].Params == nil {
				steps[i].Params = map[string]any{}
			}
			for j := range steps[i].Choices {
				normalize(steps[i].Choices[j].Steps)
			}
		}
	}
	normalize(card.Program.Steps)
	PresentProgramCard(&card, lib.Statuses)
	var walk func([]CardStep)
	walk = func(steps []CardStep) {
		for _, s := range steps {
			if s.Effect == "ability_bonus" && ProgramString(s, "preparation_status") == "" {
				id := ProgramStatusID(card.ID, s)
				lib.Statuses[id] = BattleStatusDefinition{SchemaVersion: 1, ID: id, Name: card.Name + " preparation", Presentation: Presentation{RulesText: CardProgramRules(&CardProgram{Steps: []CardStep{s}}) + " Requires a qualified, selected ability.", Glyph: "⚔"}, ActivationMode: "automatic", Polarity: "positive", Stacking: StatusStacking{Uncapped: true, OverflowPolicy: "reject_additional_stacks"}}
			}
			for _, o := range s.Choices {
				walk(o.Steps)
			}
		}
	}
	walk(card.Program.Steps)
	return card
}
