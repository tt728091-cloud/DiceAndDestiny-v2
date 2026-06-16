package contentpin_test

import (
	"testing"

	"diceanddestiny/server/internal/battle/contentpin"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/state"
)

func TestContentFingerprintIsStableAcrossMapInsertionOrderAndDetectsChanges(t *testing.T) {
	first := contentBattle([]string{"card-b", "card-a"})
	second := contentBattle([]string{"card-a", "card-b"})
	firstPin, err := contentpin.Compute(first)
	if err != nil {
		t.Fatalf("Compute(first) returned error: %v", err)
	}
	secondPin, err := contentpin.Compute(second)
	if err != nil {
		t.Fatalf("Compute(second) returned error: %v", err)
	}
	if firstPin != secondPin {
		t.Fatalf("map insertion order changed fingerprint: %#v != %#v", firstPin, secondPin)
	}

	changed := second.Clone()
	card := changed.Content.Cards["card-a"]
	amount := 2
	card.Operations[0].Amount = &amount
	changed.Content.Cards["card-a"] = card
	changedPin, err := contentpin.Compute(changed)
	if err != nil {
		t.Fatalf("Compute(changed) returned error: %v", err)
	}
	if changedPin.Fingerprint == firstPin.Fingerprint {
		t.Fatal("compiled content change did not change fingerprint")
	}
}

func contentBattle(order []string) state.Battle {
	battle, _ := state.NewBattle("content-pin")
	battle.Content.Cards = make(map[string]state.RuntimeContentDefinition)
	for _, id := range order {
		amount := 1
		battle.Content.Cards[id] = state.RuntimeContentDefinition{
			ID: id,
			Operations: []operation.Plan{{
				ID: id + "-damage", Type: operation.TypeDealDamage, Amount: &amount,
			}},
		}
	}
	battle.DiceDefinitions = map[string]state.DiceDefinition{
		"d6": {
			ID: "d6", SideCount: 6,
			Faces: []state.DiceFace{{Face: 1, Value: 1, Symbols: []string{"sun"}}},
		},
	}
	return battle
}
