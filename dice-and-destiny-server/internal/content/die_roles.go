package content

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Symbol roles name what a symbol does on its die from face counts alone, so
// board templates, ability costs and defenses written in roles work on any
// symbol order of a die family.
const (
	// RoleMain is the most common symbol: the 3-face symbol, or the first
	// 2-face symbol in face order when no symbol has 3 faces.
	RoleMain = "main"
	// RoleSide is the other 2-face symbol.
	RoleSide = "side"
	// RoleOdd is the first 1-face symbol that is not on the last face.
	RoleOdd = "odd"
	// RoleOddB is the second such symbol (only 3-1-1-1 dice have one).
	RoleOddB = "odd_b"
	// RoleApex is the 1-face symbol on the last face, the rarest.
	RoleApex = "apex"
)

// DieRoleNames lists every role in a fixed order.
var DieRoleNames = []string{RoleMain, RoleSide, RoleOdd, RoleOddB, RoleApex}

// Die families group face layouts that roll the same odds for symbol costs:
// only face counts matter, not where the faces sit.
const (
	FamilyThreeTwoOne    = "321"
	FamilyThreeOneOneOne = "3111"
	FamilyTwoTwoOneOne   = "2211"
)

// DieLayout is a die described by roles.
type DieLayout struct {
	// Family is the face counts sorted, e.g. "321".
	Family string
	// Order is the face counts in face order, e.g. "2-3-1".
	Order string
	// Roles maps each symbol to its role.
	Roles map[string]string
	// Symbols maps each role on this die to its symbol.
	Symbols map[string]string
	// Faces maps each role to its face numbers, ascending.
	Faces map[string][]int
}

// IsDieRole reports a known role name.
func IsDieRole(role string) bool {
	for _, known := range DieRoleNames {
		if role == known {
			return true
		}
	}
	return false
}

// DieRoles describes a six-sided die by roles. Only the {3,2,1}, {3,1,1,1}
// and {2,2,1,1} families with a 1-face symbol on face 6 have roles.
func DieRoles(die BattleDieDefinition) (DieLayout, error) {
	if die.SideCount != 6 || len(die.Faces) != 6 {
		return DieLayout{}, fmt.Errorf("die %q: roles need six faces", die.ID)
	}
	faces := append([]BattleDieFace(nil), die.Faces...)
	sort.Slice(faces, func(i, j int) bool { return faces[i].Number < faces[j].Number })
	var order []string
	counts := map[string]int{}
	numbers := map[string][]int{}
	for _, face := range faces {
		if counts[face.Symbol] == 0 {
			order = append(order, face.Symbol)
		}
		counts[face.Symbol]++
		numbers[face.Symbol] = append(numbers[face.Symbol], face.Number)
	}
	inOrder := make([]string, len(order))
	sorted := make([]int, len(order))
	for i, symbol := range order {
		inOrder[i] = strconv.Itoa(counts[symbol])
		sorted[i] = counts[symbol]
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sorted)))
	family := ""
	for _, n := range sorted {
		family += strconv.Itoa(n)
	}
	if family != FamilyThreeTwoOne && family != FamilyThreeOneOneOne && family != FamilyTwoTwoOneOne {
		return DieLayout{}, fmt.Errorf("die %q: face layout %s has no roles", die.ID, strings.Join(inOrder, "-"))
	}
	apex := faces[5].Symbol
	if counts[apex] != 1 {
		return DieLayout{}, fmt.Errorf("die %q: the symbol on face 6 must have one face", die.ID)
	}
	layout := DieLayout{Family: family, Order: strings.Join(inOrder, "-"), Roles: map[string]string{}, Symbols: map[string]string{}, Faces: map[string][]int{}}
	assign := func(symbol, role string) {
		layout.Roles[symbol] = role
		layout.Symbols[role] = symbol
		layout.Faces[role] = numbers[symbol]
	}
	assign(apex, RoleApex)
	for _, symbol := range order {
		if counts[symbol] == 3 {
			assign(symbol, RoleMain)
		}
	}
	for _, symbol := range order {
		if counts[symbol] != 2 {
			continue
		}
		if _, taken := layout.Symbols[RoleMain]; !taken {
			assign(symbol, RoleMain)
		} else {
			assign(symbol, RoleSide)
		}
	}
	for _, symbol := range order {
		if counts[symbol] != 1 || symbol == apex {
			continue
		}
		if _, taken := layout.Symbols[RoleOdd]; !taken {
			assign(symbol, RoleOdd)
		} else {
			assign(symbol, RoleOddB)
		}
	}
	return layout, nil
}

// annotateDieRoles publishes each die's roles in the catalog so viewers can
// label role-based outcomes without computing roles themselves. Dice without
// roles are left unchanged.
func annotateDieRoles(lib *BattleLibrary) {
	for id, die := range lib.Dice {
		layout, err := DieRoles(die)
		if err != nil {
			die.Roles = nil
		} else {
			die.Roles = layout.Roles
		}
		lib.Dice[id] = die
	}
}

// ValidateLibrary checks a whole catalog, for callers that add definitions to
// a loaded one (a player's generated abilities).
func ValidateLibrary(lib BattleLibrary) error {
	return validateBattleLibrary(lib)
}
