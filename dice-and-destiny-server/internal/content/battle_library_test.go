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
	chart.Abilities[0].ActivationRanges.FirstRoll.End = 99
	combatant.AI.OffensivePlanning.Charts["3_rolls"] = chart
	library.Combatants[combatant.ID] = combatant
	if err := validateBattleLibrary(library); err == nil || !strings.Contains(err.Error(), "cover 100") {
		t.Fatalf("D100 gap error=%v", err)
	}
}

func TestVenomGoblinOffensiveChartsAlwaysSelectVenomStrike(t *testing.T) {
	library, err := LoadBattleLibrary(filepath.Join("..", "..", "content", "battle_v1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, chartID := range []string{"1_roll", "2_rolls", "3_rolls"} {
		chart := library.Combatants["venom_goblin"].AI.OffensivePlanning.Charts[chartID]
		if len(chart.Abilities) != 1 || chart.Abilities[0].AbilityID != "venom_strike" {
			t.Errorf("venom goblin %s abilities = %#v, want only venom_strike", chartID, chart.Abilities)
			continue
		}
		r := chart.Abilities[0].ActivationRanges.FirstRoll
		if r == nil || r.Start != 1 || r.End != 100 || len(chart.NoAbilityRanges) != 0 {
			t.Errorf("venom goblin %s Venom Strike weighting = %#v with no-ability ranges %#v, want 1..100 only", chartID, r, chart.NoAbilityRanges)
		}
	}
}
