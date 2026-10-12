package content

import (
	"strings"
	"testing"
)

func TestValidateCardMapEffect(t *testing.T) {
	good := &CardMapEffect{Kind: "move", Amount: 2, Text: "Move 2"}
	if err := ValidateCardMapEffect(good); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCardMapEffect(nil); err != nil {
		t.Fatal("no map effect is fine")
	}
	cases := []struct {
		name   string
		effect CardMapEffect
		want   string
	}{
		{"unknown kind", CardMapEffect{Kind: "teleport", Amount: 1, Text: "x"}, "kind \"teleport\" is unknown"},
		{"zero amount", CardMapEffect{Kind: "draw", Amount: 0, Text: "x"}, "amount must be 1–100"},
		{"negative energy", CardMapEffect{Kind: "move", Amount: 4, EnergyCost: -1, Text: "x"}, "energy_cost must be 0–100"},
		{"terrain flag on draw", CardMapEffect{Kind: "draw", Amount: 2, IgnoreTerrain: true, Text: "x"}, "terrain options belong to move"},
		{"no text", CardMapEffect{Kind: "energy", Amount: 10}, "text is required"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateCardMapEffect(&c.effect)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("expected %q, got %v", c.want, err)
			}
		})
	}
}

func TestMapOnlyCardsAreBlankBattleCards(t *testing.T) {
	mapCard := BattleCardDefinition{SchemaVersion: 1, ID: "trailblaze", Name: "Trailblaze", Type: "map",
		Presentation: Presentation{RulesText: "Map only."}, Play: CardPlayDefinition{SourceZones: []string{"hand"}, Destination: "discard"},
		Targeting: TargetingDefinition{Selector: "self", Maximum: 1},
		MapEffect: &CardMapEffect{Kind: "move", Amount: 2, Text: "Move 2"}}
	if !IsMapOnlyCard(mapCard) || !IsBlankCard(mapCard) {
		t.Fatal("a card with only a map effect is a map-only (blank) battle card")
	}
	both := mapCard
	both.Program = &CardProgram{Version: 1, Windows: []string{"offensive_planning"}, RollRequirement: "any", Steps: []CardStep{{Effect: "draw", Target: CardTarget{Owner: "self", Mode: "one", Count: 1, Selection: "choose"}, Params: map[string]any{"amount": 2}}}}
	if IsMapOnlyCard(both) || IsBlankCard(both) {
		t.Fatal("a card with a program and a map effect is a 'both' card")
	}
	// The library validator accepts a blank card with a map effect and rejects a bad one.
	lib := BattleLibrary{Symbols: map[string]SymbolDefinition{}, Dice: map[string]BattleDieDefinition{}, Statuses: map[string]BattleStatusDefinition{}, Abilities: map[string]BattleAbilityDefinition{}, Combatants: map[string]CombatantDefinition{}, Cards: map[string]BattleCardDefinition{"trailblaze": mapCard}}
	if err := validateBattleLibrary(lib); err != nil {
		t.Fatalf("a map-only card failed validation: %v", err)
	}
	bad := mapCard
	bad.MapEffect = &CardMapEffect{Kind: "move", Amount: 0, Text: "Move 0"}
	lib.Cards["trailblaze"] = bad
	if err := validateBattleLibrary(lib); err == nil || !strings.Contains(err.Error(), "map_effect amount") {
		t.Fatalf("a bad map effect passed library validation: %v", err)
	}
}

func TestStatusModifiersValidation(t *testing.T) {
	status := BattleStatusDefinition{SchemaVersion: 1, ID: "fortified", Name: "Fortified", Presentation: Presentation{RulesText: "x"}, ActivationMode: "automatic", Polarity: "positive",
		Stacking: StatusStacking{StackLimit: 1, OverflowPolicy: "reject_additional_stacks"}, Lifecycle: StatusLifecycle{Persistent: true}, Modifiers: &StatusModifiers{DefenseDiceBonus: 1}}
	lib := BattleLibrary{Symbols: map[string]SymbolDefinition{}, Dice: map[string]BattleDieDefinition{}, Statuses: map[string]BattleStatusDefinition{"fortified": status}, Abilities: map[string]BattleAbilityDefinition{}, Combatants: map[string]CombatantDefinition{}, Cards: map[string]BattleCardDefinition{}}
	if err := validateBattleLibrary(lib); err != nil {
		t.Fatal(err)
	}
	empty := status
	empty.Modifiers = &StatusModifiers{}
	lib.Statuses["fortified"] = empty
	if err := validateBattleLibrary(lib); err == nil || !strings.Contains(err.Error(), "empty modifiers") {
		t.Fatalf("empty modifiers accepted: %v", err)
	}
	out := status
	out.Modifiers = &StatusModifiers{DefenseDiceBonus: 11}
	lib.Statuses["fortified"] = out
	if err := validateBattleLibrary(lib); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("out-of-range modifiers accepted: %v", err)
	}
}
