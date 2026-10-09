package content

import (
	"fmt"
	"sort"
	"strings"
)

// Card faces state the effect as a keyword and a number. Default behaviour,
// edge cases and timing stay out: the timing ribbon shows when a card plays,
// and the hover shows the full rules (CardProgramRulesWithStatuses).
// Only non-default parameters are mentioned, e.g. a reroll that uses a roll
// attempt or prevention that sends saved cards somewhere other than their pile.

// CardProgramFace is the short card-face text for a card program.
func CardProgramFace(p *CardProgram, statuses map[string]BattleStatusDefinition) string {
	if p == nil {
		return ""
	}
	return strings.Join(faceSteps(p.Steps, statuses), "\n")
}

func faceSteps(steps []CardStep, statuses map[string]BattleStatusDefinition) []string {
	var lines []string
	for _, s := range steps {
		if s.Effect == "choice" {
			for i, o := range s.Choices {
				text := strings.Join(faceSteps(o.Steps, statuses), " · ")
				if o.Energy > 0 {
					text = fmt.Sprintf("+%d Energy: %s", o.Energy, text)
				}
				if i > 0 {
					text = "or " + text
				}
				lines = append(lines, text)
			}
			continue
		}
		text := faceStep(s, statuses)
		if condition := ProgramConditionRules(s.Condition); condition != "" {
			text = "With " + condition + ": " + text
		}
		lines = append(lines, text)
	}
	return lines
}

func faceStep(s CardStep, statuses map[string]BattleStatusDefinition) string {
	t := s.Target
	switch s.Effect {
	case "curse":
		return fmt.Sprintf("Apply %d Curse", ProgramInt(s, "amount")) + faceActorSuffix(t)
	case "roll_cursed":
		return fmt.Sprintf("Roll up to %d cursed dice", ProgramInt(s, "amount"))
	case "conditional_status":
		return fmt.Sprintf("At %d+ %s: apply %d %s, else %d %s", ProgramInt(s, "threshold"), statusName(ProgramString(s, "status_id"), statuses), ProgramInt(s, "stacks"), statusName(ProgramString(s, "result_status_id"), statuses), ProgramInt(s, "fallback_stacks"), statusName(ProgramString(s, "fallback_status_id"), statuses))
	case "status_threshold":
		return fmt.Sprintf("At %d+ %s: apply %d %s", ProgramInt(s, "threshold"), statusName(ProgramString(s, "status_id"), statuses), ProgramInt(s, "stacks"), statusName(ProgramString(s, "result_status_id"), statuses))
	case "draw":
		return faceActorVerb(t, "Draw", "draws") + " " + counted(ProgramInt(s, "amount"), "card", "cards")
	case "energy":
		return faceActorVerb(t, "Gain", "gains") + fmt.Sprintf(" %d Energy", ProgramInt(s, "amount"))
	case "prevent":
		text := fmt.Sprintf("Prevent %d", ProgramInt(s, "amount"))
		if !faceSingle(t) {
			text += " on " + faceTarget(t, "attack", "attacks")
		}
		return text + faceDestination(s)
	case "save_cards":
		return "Save " + faceTarget(t, "threatened card", "threatened cards") + faceDestination(s)
	case "move_cards":
		if ProgramContains(t.Zones, "removed") {
			revived := t
			revived.Zones = nil
			text := "Revive " + faceTarget(revived, "removed card", "removed cards")
			if d := ProgramString(s, "destination"); d != "hand" {
				text += " to " + d
			}
			return text
		}
		cards := faceTarget(t, "card", "cards")
		if len(t.Zones) > 0 {
			cards += " from " + strings.Join(t.Zones, "/")
		}
		if t.DrawnThisPlay {
			cards += " drawn by this card"
		}
		if ProgramString(s, "destination") == "removed" {
			return "Remove " + cards + " (−1 health each)"
		}
		return "Move " + cards + " to " + ProgramString(s, "destination")
	case "sacrifice":
		return "Sacrifice " + faceTarget(t, "other card", "other cards") + " (−1 health each)"
	case "set_die":
		return "Set " + faceTarget(t, "die", "dice") + " to " + orList(ProgramInts(s, "faces"))
	case "adjust_die":
		text := "Change " + faceTarget(t, "die", "dice") + " by " + deltaWords(ProgramInts(s, "deltas"))
		if ProgramBool(s, "wrap") {
			text += " (wraps)"
		}
		return text
	case "copy_die":
		return "Set " + faceTarget(t, "die", "dice") + " to match another"
	case "flip_die":
		if ProgramInt(s, "sum") == 7 {
			return "Flip " + faceTarget(t, "die", "dice") + "\n(1↔6 · 2↔5 · 3↔4)"
		}
		return "Flip " + faceTarget(t, "die", "dice") + fmt.Sprintf(" (faces total %d)", ProgramInt(s, "sum"))
	case "reroll", "reroll_defense":
		text := "Reroll " + faceTarget(t, "die", "dice")
		if s.Effect == "reroll_defense" {
			text = "Reroll " + faceTarget(t, "defense die", "defense dice")
		}
		switch ProgramString(s, "result") {
		case "higher":
			text += ", keep the higher"
		case "lower":
			text += ", keep the lower"
		}
		if s.Effect == "reroll" && ProgramBool(s, "consume_roll") {
			text += " (uses a roll)"
		}
		return text
	case "remove_status":
		singular, pluralNoun := statusNouns(t, statuses)
		stacks := ProgramInt(s, "stacks")
		if stacks == 0 {
			return "Clear " + faceTarget(t, singular, pluralNoun)
		}
		if faceSingle(t) {
			return fmt.Sprintf("Remove %d %s %s", stacks, faceOwner(t)+singular, plural(stacks, "stack", "stacks"))
		}
		return fmt.Sprintf("Remove %s from %s", counted(stacks, "stack", "stacks"), faceTarget(t, singular, pluralNoun))
	case "apply_status":
		name := statusName(ProgramString(s, "status_id"), statuses)
		if t.Owner == "self" {
			return fmt.Sprintf("Gain %d %s", ProgramInt(s, "stacks"), name)
		}
		return fmt.Sprintf("Apply %d %s", ProgramInt(s, "stacks"), name) + faceActorSuffix(t)
	case "ability_bonus":
		var gains []string
		if d := ProgramInt(s, "damage"); d > 0 {
			gains = append(gains, fmt.Sprintf("+%d damage", d))
		}
		if id := ProgramString(s, "status_id"); id != "" {
			gains = append(gains, fmt.Sprintf("+%d %s on hit", ProgramInt(s, "stacks"), statusName(id, statuses)))
		}
		abilities := faceTarget(t, "ability", "abilities")
		if t.Qualified {
			abilities = faceTarget(t, "qualifying ability", "qualifying abilities")
		}
		return strings.Join(gains, " and ") + " to " + abilities + faceDuration(s)
	}
	return ""
}

// faceSingle reports a one-object target.
func faceSingle(t CardTarget) bool {
	return t.Mode == "one" || t.Mode == "" || (t.Mode == "exact" && t.Count == 1)
}

// faceTarget is a short counted noun: "1 die", "up to 2 enemy dice", "any dice".
func faceTarget(t CardTarget, singular, pluralNoun string) string {
	count := "1"
	noun := singular
	switch t.Mode {
	case "exact":
		count = fmt.Sprint(t.Count)
		if t.Count != 1 {
			noun = pluralNoun
		}
	case "up_to":
		// The editor's maximum count reads as "any number".
		count, noun = fmt.Sprintf("up to %d", t.Count), pluralNoun
		if t.Count >= 100 {
			count = "any"
		}
	case "all":
		count, noun = "all", pluralNoun
	}
	if t.Selection == "random" {
		count += " random"
	}
	text := count + " " + faceOwner(t) + noun
	if len(t.Faces) > 0 {
		text += " showing " + orList(t.Faces)
	}
	return text
}

func faceOwner(t CardTarget) string {
	switch t.Owner {
	case "enemy":
		return "enemy "
	case "any":
		return "any "
	}
	return ""
}

// faceActorVerb picks "Draw" for yourself and "Enemy draws" otherwise.
func faceActorVerb(t CardTarget, self, other string) string {
	switch t.Owner {
	case "enemy":
		if t.Mode == "all" {
			return "Each enemy " + other
		}
		return "Enemy " + other
	case "any":
		return "Target " + other
	}
	return self
}

func faceActorSuffix(t CardTarget) string {
	if t.Mode == "all" {
		switch t.Owner {
		case "enemy":
			return " to each enemy"
		case "self":
			return " to each ally"
		}
		return " to everyone"
	}
	return ""
}

// faceDestination names a saved-card destination other than the default
// ("original": saved cards stay in their piles).
func faceDestination(s CardStep) string {
	switch d := ProgramString(s, "destination"); d {
	case "", "original":
		return ""
	case "removed":
		return "\nSaved cards are removed"
	default:
		return "\nSaved cards → " + d
	}
}

func faceDuration(s CardStep) string {
	switch ProgramString(s, "duration") {
	case "offensive":
		return " this Offense"
	case "round":
		return " this round"
	case "rounds":
		return fmt.Sprintf(" for %s", counted(ProgramInt(s, "rounds"), "round", "rounds"))
	case "next_use":
		return " on its next use"
	case "battle":
		return " this battle"
	}
	return ""
}

// statusNouns names the statuses a target filters to: "Poison or Bleed",
// otherwise "debuff"/"buff"/"status" by polarity.
func statusNouns(t CardTarget, statuses map[string]BattleStatusDefinition) (string, string) {
	if len(t.StatusIDs) > 0 {
		names := make([]string, 0, len(t.StatusIDs))
		for _, id := range t.StatusIDs {
			names = append(names, statusName(id, statuses))
		}
		joined := joinOr(names)
		return joined, joined
	}
	switch t.Polarity {
	case "negative":
		return "debuff", "debuffs"
	case "positive":
		return "buff", "buffs"
	}
	return "status", "statuses"
}

func statusName(id string, statuses map[string]BattleStatusDefinition) string {
	if def, ok := statuses[id]; ok && def.Name != "" {
		return def.Name
	}
	return strings.ReplaceAll(id, "_", " ")
}

// counted counts a noun: "1 card", "2 cards".
func counted(n int, singular, pluralNoun string) string {
	return fmt.Sprintf("%d %s", n, plural(n, singular, pluralNoun))
}

// orList reads integers as "1 or 4", "1, 2 or 3".
func orList(values []int) string {
	words := make([]string, 0, len(values))
	for _, v := range values {
		words = append(words, fmt.Sprint(v))
	}
	return joinOr(words)
}

func joinOr(words []string) string {
	if len(words) <= 1 {
		return strings.Join(words, "")
	}
	return strings.Join(words[:len(words)-1], ", ") + " or " + words[len(words)-1]
}

// deltaWords reads die adjustments as "±1", "+1 or +2", "±1 or ±2".
func deltaWords(deltas []int) string {
	has := map[int]bool{}
	for _, d := range deltas {
		has[d] = true
	}
	var sizes []int
	seen := map[int]bool{}
	for _, d := range deltas {
		a := d
		if a < 0 {
			a = -a
		}
		if !seen[a] {
			seen[a] = true
			sizes = append(sizes, a)
		}
	}
	sort.Ints(sizes)
	var words []string
	for _, a := range sizes {
		switch {
		case has[a] && has[-a] && a != 0:
			words = append(words, fmt.Sprintf("±%d", a))
		case has[a]:
			words = append(words, fmt.Sprintf("+%d", a))
		default:
			words = append(words, fmt.Sprintf("−%d", a))
		}
	}
	return joinOr(words)
}
