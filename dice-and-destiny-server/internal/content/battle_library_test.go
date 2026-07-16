package content

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryBattleLibraryLoadsAndValidatesAllReferences(t *testing.T) {
	root := filepath.Join("..", "..", "content", "battle_v1")
	library, err := LoadBattleLibrary(root)
	if err != nil {
		t.Fatalf("LoadBattleLibrary() error = %v", err)
	}
	if len(library.Symbols) != 3 || len(library.Dice) != 1 || len(library.Cards) != 7 || len(library.Abilities) != 11 || len(library.Statuses) != 5 || len(library.Combatants) != 2 {
		t.Fatalf("unexpected catalog composition: symbols=%d dice=%d cards=%d abilities=%d statuses=%d combatants=%d", len(library.Symbols), len(library.Dice), len(library.Cards), len(library.Abilities), len(library.Statuses), len(library.Combatants))
	}
	die := library.Dice["standard_d6"]
	for index, face := range die.Faces {
		if face.Number != index+1 || face.Symbol == "" {
			t.Fatalf("face %d = %#v", index+1, face)
		}
	}
}

func TestBattleLibraryStrictnessAndCrossReferences(t *testing.T) {
	var die BattleDieDefinition
	if err := decodeKnownYAML([]byte("schema_version: 1\nid: test\nname: Test\ndie_type: d6\nside_count: 1\nfaces: [{number: 1, symbol: sword}]\nunknown: true\n"), &die); err == nil {
		t.Fatal("unknown YAML field was accepted")
	}
	library, err := LoadBattleLibrary(filepath.Join("..", "..", "content", "battle_v1"))
	if err != nil {
		t.Fatal(err)
	}
	broken := library.Dice["standard_d6"]
	broken.Faces[0].Symbol = "missing"
	library.Dice["standard_d6"] = broken
	if err := validateBattleLibrary(library); err == nil || !strings.Contains(err.Error(), "unknown symbol") {
		t.Fatalf("missing symbol error=%v", err)
	}
}

func TestD100ChartsRejectGapsAndOverlaps(t *testing.T) {
	library, err := LoadBattleLibrary(filepath.Join("..", "..", "content", "battle_v1"))
	if err != nil {
		t.Fatal(err)
	}
	combatant := library.Combatants["venom_goblin"]
	chart := combatant.AI.OffensivePlanning.Charts["3_rolls"]
	chart.NoAbilityRanges = []D100Range{{Start: 91, End: 94}}
	combatant.AI.OffensivePlanning.Charts["3_rolls"] = chart
	library.Combatants[combatant.ID] = combatant
	if err := validateBattleLibrary(library); err == nil || !strings.Contains(err.Error(), "cover 90") {
		t.Fatalf("D100 gap error=%v", err)
	}
}

func TestVenomGoblinOffensiveChartsStayAggressive(t *testing.T) {
	library, err := LoadBattleLibrary(filepath.Join("..", "..", "content", "battle_v1"))
	if err != nil {
		t.Fatal(err)
	}
	wantAttackChance := map[string]int{"1_roll": 50, "2_rolls": 80, "3_rolls": 95}
	for chartID, want := range wantAttackChance {
		chart := library.Combatants["venom_goblin"].AI.OffensivePlanning.Charts[chartID]
		noAbility := 0
		for _, r := range chart.NoAbilityRanges {
			noAbility += r.End - r.Start + 1
		}
		if got := 100 - noAbility; got != want {
			t.Errorf("venom goblin %s attack chance = %d%%, want %d%%", chartID, got, want)
		}
	}
}
