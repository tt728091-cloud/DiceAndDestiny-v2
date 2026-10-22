package content

import (
	"path/filepath"
	"reflect"
	"testing"
)

// The board-selection dice: one per family order, in the custom adventurer's
// pack, with the rarest symbol on face 6.
func TestBoardSelectionDiceLoadWithExpectedRoles(t *testing.T) {
	lib, err := LoadBattleLibrary(filepath.Join("..", "..", "content", "battle_v1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, pack := range []string{"venom_v1", "curse_v1", "adventurer_v1", "general_v1"} {
		if lib, err = LoadBattleExtension(lib, filepath.Join("..", "..", "content", pack)); err != nil {
			t.Fatalf("%s: %v", pack, err)
		}
	}
	cases := []struct {
		die, family, order string
		roles              map[string]string
	}{
		{"standard_d6", "321", "3-2-1", map[string]string{"sword": "main", "shield": "side", "gold_coin": "apex"}},
		{"hearth_d6", "321", "2-3-1", map[string]string{"hearth_spark": "side", "hearth_cinder": "main", "hearth_kiln": "apex"}},
		{"grove_d6", "3111", "3-1-1-1", map[string]string{"grove_bramble": "main", "grove_root": "odd", "grove_bloom": "odd_b", "grove_acorn": "apex"}},
		{"forge_d6", "2211", "2-2-1-1", map[string]string{"forge_anvil": "main", "forge_tongs": "side", "forge_rivet": "odd", "forge_ingot": "apex"}},
		{"loom_d6", "2211", "2-1-2-1", map[string]string{"loom_thread": "main", "loom_needle": "odd", "loom_spindle": "side", "loom_knot": "apex"}},
		// Character dice keep their authored symbols and get the same roles.
		{"venom_d6", "321", "3-2-1", map[string]string{"fang": "main", "gland": "side", "coil": "apex"}},
		{"curse_d6", "321", "3-2-1", map[string]string{"curse_skull": "main", "curse_shroud": "side", "curse_omen": "apex"}},
	}
	for _, c := range cases {
		die, ok := lib.Dice[c.die]
		if !ok {
			t.Fatalf("%s did not load", c.die)
		}
		for _, face := range die.Faces {
			if _, ok := lib.Symbols[face.Symbol]; !ok {
				t.Fatalf("%s face %d names unknown symbol %q", c.die, face.Number, face.Symbol)
			}
		}
		layout, err := DieRoles(die)
		if err != nil {
			t.Fatalf("%s: %v", c.die, err)
		}
		if layout.Family != c.family || layout.Order != c.order || !reflect.DeepEqual(layout.Roles, c.roles) {
			t.Fatalf("%s: got family %s order %s roles %v; want %s %s %v", c.die, layout.Family, layout.Order, layout.Roles, c.family, c.order, c.roles)
		}
		if !reflect.DeepEqual(die.Roles, c.roles) {
			t.Fatalf("%s: catalog roles %v, want %v", c.die, die.Roles, c.roles)
		}
		if layout.Roles[die.Faces[5].Symbol] != RoleApex {
			t.Fatalf("%s: face 6 is not the apex", c.die)
		}
		for role, faces := range layout.Faces {
			for _, face := range faces {
				if layout.Roles[die.Faces[face-1].Symbol] != role {
					t.Fatalf("%s: face %d is not %s", c.die, face, role)
				}
			}
		}
	}
}

func TestDieRolesRejectLayoutsWithoutRoles(t *testing.T) {
	face := func(symbols ...string) []BattleDieFace {
		out := make([]BattleDieFace, len(symbols))
		for i, s := range symbols {
			out[i] = BattleDieFace{Number: i + 1, Symbol: s}
		}
		return out
	}
	for name, faces := range map[string][]BattleDieFace{
		"three pairs":     face("a", "a", "b", "b", "c", "c"),
		"apex not rarest": face("a", "a", "a", "c", "b", "b"),
		"four of one":     face("a", "a", "a", "a", "b", "c"),
	} {
		if _, err := DieRoles(BattleDieDefinition{ID: "x", SideCount: 6, Faces: faces}); err == nil {
			t.Fatalf("%s: roles accepted", name)
		}
	}
}
