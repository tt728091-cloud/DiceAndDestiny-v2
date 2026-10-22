package boardprice

import (
	"fmt"
	"sort"
	"strings"

	"diceanddestiny/server/internal/content"
)

// GeneratedPrefix starts every generated ability ID. The rest is the role
// cost ("board_main3_main4_main5"), so an unchanged board always yields the
// same IDs and a player's sheet can only add or replace generated abilities.
const GeneratedPrefix = "board_"

// AbilityID is a cost's generated ability ID.
func AbilityID(cost Cost) string { return GeneratedPrefix + cost.Key() }

// Generate turns a priced choice into real ability definitions with the
// die's symbol IDs: one ability per slot, its tiers hardest first (a ladder
// becomes tiers of one ability, and the player chooses among qualified
// tiers), each tier dealing its whole damage plus the slot's rider. It also
// returns the board: the generated abilities in template order and the
// choice's defenses.
func (b *Builder) Generate(choice Choice, report Report) ([]content.BattleAbilityDefinition, content.AbilityBoard, error) {
	resolved, die, board, err := b.Resolve(choice)
	if err != nil {
		return nil, content.AbilityBoard{}, err
	}
	if report.Die != die.ID {
		return nil, content.AbilityBoard{}, fmt.Errorf("the report prices %s, not %s", report.Die, die.ID)
	}
	tiers := map[string]TierReport{}
	for _, t := range report.Tiers {
		tiers[t.Slot+"|"+t.Tier] = t
	}
	var out []content.BattleAbilityDefinition
	var abilityBoard content.AbilityBoard
	for _, slot := range board.Slots {
		priced := make([]TierReport, len(slot.Cost))
		for i, tier := range slot.Cost {
			t, ok := tiers[slot.ID+"|"+tier.String()]
			if !ok {
				return nil, content.AbilityBoard{}, fmt.Errorf("the report does not price %s in slot %s", tier, slot.ID)
			}
			priced[i] = t
		}
		ability := b.ability(die, slot, priced)
		out = append(out, ability)
		abilityBoard.Offensive = append(abilityBoard.Offensive, ability.ID)
	}
	abilityBoard.Defensive = append([]string(nil), resolved.Defenses...)
	return out, abilityBoard, nil
}

func (b *Builder) ability(die Die, slot Slot, priced []TierReport) content.BattleAbilityDefinition {
	order := make([]int, len(slot.Cost))
	for i := range order {
		order[i] = i
	}
	// Hardest first: the authority offers the first qualified tier by default.
	sort.SliceStable(order, func(x, y int) bool { return priced[order[x]].Chase < priced[order[y]].Chase })
	q := &content.AbilityQualification{ChooseTier: len(slot.Cost) > 1}
	for _, i := range order {
		tier := slot.Cost[i]
		ops := []content.BattleOperation{{Type: "deal_damage", Target: "selected_targets", Amount: priced[i].WholeDamage}}
		ops = append(ops, cloneOperations(slot.Rider)...)
		q.ActivationTiers = append(q.ActivationTiers, content.AbilityTier{ID: tier.Key(), Requirements: tier.Requirements(die.Layout), Operations: ops})
	}
	return content.BattleAbilityDefinition{
		SchemaVersion: 1, ConfigurationVersion: 1,
		ID: AbilityID(slot.Cost), Name: b.AbilityName(die, slot.Cost), Type: "offensive",
		Presentation:  content.Presentation{RulesText: b.rulesText(die, slot, priced, order)},
		Cost:          content.BattleCost{Energy: 0},
		Usage:         content.BattleUsage{MaximumPerSegment: 1},
		Targeting:     &content.TargetingDefinition{Selector: "one_enemy", Minimum: 1, Maximum: 1},
		Qualification: q,
	}
}

func cloneOperations(ops []content.BattleOperation) []content.BattleOperation {
	out := make([]content.BattleOperation, len(ops))
	copy(out, ops)
	return out
}

// AbilityName names a generated ability by its cost in the die's symbols
// ("Cinder ×3/4/5", "Cinder ×2 + Spark ×3"); straights, which name no symbol,
// are runs. Different costs on one die never share a name.
func (b *Builder) AbilityName(die Die, cost Cost) string {
	if straights(cost) {
		switch {
		case len(cost) == 2:
			return "Run"
		case cost[0].Straight == StraightSmall:
			return "Short Run"
		default:
			return "Long Run"
		}
	}
	return b.Label(die, cost)
}

// rulesText is the generated ability's full rules for the hover: the dice
// requirement with each symbol's faces, the damage of every tier, the rider
// and when the ability misses.
func (b *Builder) rulesText(die Die, slot Slot, priced []TierReport, order []int) string {
	rider := b.riderClause(slot.Rider)
	var sb strings.Builder
	switch {
	case straights(slot.Cost) && len(slot.Cost) == 1:
		sb.WriteString(straightRule(slot.Cost[0].Straight, true))
		fmt.Fprintf(&sb, " Deal %d damage%s. Misses without the required straight.", priced[0].WholeDamage, rider)
		return sb.String()
	case len(slot.Cost) == 1:
		tier := slot.Cost[0]
		fmt.Fprintf(&sb, "%s: deal %d damage%s. %s.", b.countText(die, tier), priced[0].WholeDamage, rider, b.facesText(die, slot.Cost))
		switch rest := b.Menus.Pricing.Dice - tier.Dice(); {
		case rest == 1:
			sb.WriteString(" The fifth die may be anything.")
		case rest > 1:
			sb.WriteString(fmt.Sprintf(" The other %s dice may be anything.", diceWord(rest)))
		}
		sb.WriteString(" " + b.missText(die, slot.Cost))
		return sb.String()
	}
	parts := make([]string, len(order))
	for k, i := range order {
		tier := slot.Cost[i]
		label := b.countText(die, tier)
		if tier.Straight != "" {
			label = straightRule(tier.Straight, false)
		}
		parts[k] = fmt.Sprintf("%s: %d damage", label, priced[i].WholeDamage)
	}
	fmt.Fprintf(&sb, "Choose a qualified tier: %s.", strings.Join(parts, "; "))
	if rider != "" {
		fmt.Fprintf(&sb, " With every tier,%s.", strings.TrimPrefix(rider, " and"))
	}
	if !straights(slot.Cost) {
		fmt.Fprintf(&sb, " %s; numbers need not match.", b.facesText(die, slot.Cost))
	}
	sb.WriteString(" " + b.missText(die, slot.Cost))
	return sb.String()
}

func straightRule(kind string, sentence bool) string {
	if kind == StraightSmall {
		if sentence {
			return "Small straight: any four consecutive numbers (1-4, 2-5 or 3-6); the fifth die may be anything."
		}
		return "small straight (any four consecutive numbers, 1-4, 2-5 or 3-6)"
	}
	if sentence {
		return "Large straight: five consecutive numbers (1-5 or 2-6)."
	}
	return "large straight (five consecutive numbers, 1-5 or 2-6)"
}

// countText writes a tier's symbol counts: "2 Cinder + 3 Spark".
func (b *Builder) countText(die Die, tier Tier) string {
	parts := make([]string, len(tier.Counts))
	for i, c := range tier.Counts {
		parts[i] = fmt.Sprintf("%d %s", c.Count, b.SymbolName(die, c.Role))
	}
	return strings.Join(parts, " + ")
}

// facesText says which faces show each symbol the cost names.
func (b *Builder) facesText(die Die, cost Cost) string {
	var parts []string
	seen := map[string]bool{}
	for _, role := range content.DieRoleNames {
		for _, tier := range cost {
			for _, c := range tier.Counts {
				if c.Role == role && !seen[role] {
					seen[role] = true
					parts = append(parts, fmt.Sprintf("%s is on %s", b.SymbolName(die, role), faceNumbers(die, role)))
				}
			}
		}
	}
	text := strings.Join(parts, "; ")
	return strings.ToUpper(text[:1]) + text[1:]
}

// missText says when the ability misses.
func (b *Builder) missText(die Die, cost Cost) string {
	if straights(cost) {
		return "Misses without a straight."
	}
	if ladder, role := ladderRole(cost); ladder {
		least := cost[0].Counts[0].Count
		for _, t := range cost {
			least = min(least, t.Counts[0].Count)
		}
		return fmt.Sprintf("Misses with fewer than %d %s.", least, b.SymbolName(die, role))
	}
	if len(cost) == 1 && len(cost[0].Counts) == 1 {
		c := cost[0].Counts[0]
		return fmt.Sprintf("Misses with fewer than %d %s.", c.Count, b.SymbolName(die, c.Role))
	}
	return "Misses without the required symbols."
}

// riderClause continues "deal N damage": " and gain 2 Protect (…)".
func (b *Builder) riderClause(ops []content.BattleOperation) string {
	var parts []string
	for _, op := range ops {
		amount := 0
		switch v := op.Amount.(type) {
		case int:
			amount = v
		case float64:
			amount = int(v)
		}
		switch op.Type {
		case "apply_status":
			status := b.Lib.Statuses[op.StatusID]
			if op.StatusID == "protect" {
				parts = append(parts, fmt.Sprintf("gain %d Protect (positive status) before defense; during damage reaction, spend all Protect to prevent that much damage from one incoming attack, and unused Protect expires during the next round's Ongoing Effects (cap %d)", op.StackCount, status.Stacking.StackLimit))
			} else {
				parts = append(parts, fmt.Sprintf("gain %d %s (%s)", op.StackCount, status.Name, strings.TrimSuffix(status.Presentation.RulesText, ".")))
			}
		case "gain_resource":
			parts = append(parts, fmt.Sprintf("gain %d energy", amount))
		case "draw_cards":
			parts = append(parts, fmt.Sprintf("draw %d %s (none from an empty draw pile)", amount, plural(amount, "card")))
		case "heal_cards":
			what := "your most recently removed card returns to your discard pile, leaving the wound that removed it"
			if amount > 1 {
				what = fmt.Sprintf("your %s most recently removed cards return to your discard pile, each leaving the wound that removed it", diceWord(amount))
			}
			parts = append(parts, fmt.Sprintf("heal %d: %s (nothing happens without removed cards)", amount, what))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " and " + strings.Join(parts, " and ")
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

func diceWord(n int) string {
	words := []string{"zero", "one", "two", "three", "four", "five"}
	if n >= 0 && n < len(words) {
		return words[n]
	}
	return fmt.Sprint(n)
}
