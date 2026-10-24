package boardprice

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"diceanddestiny/server/internal/content"
)

// Cost is what an ability needs to activate, one tier per entry: "main3" is
// one tier, "main3/main4/main5" a ladder of three and "SS/LS" both straights
// in one ability. Costs are written in die roles (content.DieRoleNames), so
// they mean the same on any symbol order of a family.
type Cost []Tier

// Tier is one way to qualify: a straight, or a count of each role at once.
type Tier struct {
	// Straight is "small" or "large"; empty for role counts.
	Straight string
	// Counts are the roles needed, in content.DieRoleNames order.
	Counts []RoleCount
}

// RoleCount is "at least Count symbols with this role".
type RoleCount struct {
	Role  string
	Count int
}

const (
	StraightSmall = "small"
	StraightLarge = "large"
)

var rolePart = regexp.MustCompile(`^([a-z_]+)([1-9][0-9]*)$`)

// ParseCost reads cost notation: tiers split by "/", parts by "+", each part
// a role and a count ("odd_b2") or SS / LS.
func ParseCost(text string) (Cost, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("empty cost")
	}
	var cost Cost
	seen := map[string]bool{}
	for _, part := range strings.Split(text, "/") {
		tier, err := parseTier(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("cost %q: %w", text, err)
		}
		if seen[tier.String()] {
			return nil, fmt.Errorf("cost %q repeats tier %s", text, tier)
		}
		seen[tier.String()] = true
		cost = append(cost, tier)
	}
	return cost, nil
}

func parseTier(text string) (Tier, error) {
	switch text {
	case "SS":
		return Tier{Straight: StraightSmall}, nil
	case "LS":
		return Tier{Straight: StraightLarge}, nil
	}
	counts := map[string]int{}
	for _, part := range strings.Split(text, "+") {
		m := rolePart.FindStringSubmatch(strings.TrimSpace(part))
		if m == nil {
			return Tier{}, fmt.Errorf("unreadable part %q", part)
		}
		if !content.IsDieRole(m[1]) {
			return Tier{}, fmt.Errorf("unknown role %q", m[1])
		}
		if counts[m[1]] > 0 {
			return Tier{}, fmt.Errorf("role %s named twice", m[1])
		}
		n, _ := strconv.Atoi(m[2])
		counts[m[1]] = n
	}
	var tier Tier
	for _, role := range content.DieRoleNames {
		if counts[role] > 0 {
			tier.Counts = append(tier.Counts, RoleCount{Role: role, Count: counts[role]})
		}
	}
	return tier, nil
}

// Dice is how many dice the tier needs showing (a straight needs 4 or 5).
func (t Tier) Dice() int {
	switch t.Straight {
	case StraightSmall:
		return 4
	case StraightLarge:
		return 5
	}
	n := 0
	for _, c := range t.Counts {
		n += c.Count
	}
	return n
}

// String is the tier's notation, with roles in role order.
func (t Tier) String() string {
	switch t.Straight {
	case StraightSmall:
		return "SS"
	case StraightLarge:
		return "LS"
	}
	parts := make([]string, len(t.Counts))
	for i, c := range t.Counts {
		parts[i] = fmt.Sprintf("%s%d", c.Role, c.Count)
	}
	return strings.Join(parts, "+")
}

// Key is the tier's notation as an ID part: "main2_side3", "ss".
func (t Tier) Key() string {
	return strings.ToLower(strings.ReplaceAll(t.String(), "+", "_"))
}

// String is the cost's canonical notation.
func (c Cost) String() string {
	parts := make([]string, len(c))
	for i, t := range c {
		parts[i] = t.String()
	}
	return strings.Join(parts, "/")
}

// Key is the cost as an ID part: "main3_main4_main5", "ss_ls".
func (c Cost) Key() string {
	parts := make([]string, len(c))
	for i, t := range c {
		parts[i] = t.Key()
	}
	return strings.Join(parts, "_")
}

// Reachable reports whether every tier can be rolled on a die: each role it
// names is on the die and no tier needs more dice than are rolled.
func (c Cost) Reachable(layout content.DieLayout, dice int) error {
	for _, t := range c {
		if t.Dice() > dice {
			return fmt.Errorf("%s needs %d dice; %d are rolled", t, t.Dice(), dice)
		}
		for _, rc := range t.Counts {
			if _, ok := layout.Symbols[rc.Role]; !ok {
				return fmt.Errorf("%s needs the %s role; %s dice have no such symbol", t, rc.Role, layout.Order)
			}
		}
	}
	return nil
}

// Requirements writes the tier as engine requirements with the die's symbol
// IDs, so qualification is the authority's own.
func (t Tier) Requirements(layout content.DieLayout) content.RequirementGroup {
	switch t.Straight {
	case StraightSmall:
		return content.RequirementGroup{All: []content.BattleRequirement{{Type: "number_pattern", Pattern: "small_straight"}}}
	case StraightLarge:
		return content.RequirementGroup{All: []content.BattleRequirement{{Type: "number_pattern", Pattern: "large_straight"}}}
	}
	var group content.RequirementGroup
	for _, rc := range t.Counts {
		group.All = append(group.All, content.BattleRequirement{Type: "symbol_count", SymbolID: layout.Symbols[rc.Role], Minimum: rc.Count})
	}
	return group
}
