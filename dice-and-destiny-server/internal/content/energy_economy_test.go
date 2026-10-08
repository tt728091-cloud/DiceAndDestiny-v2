package content

import (
	"errors"
	"testing"
)

// Every shipped combatant uses the ×5 energy economy: 1 card and 5 energy per
// Income under the 75 energy cap.
func TestShippedCombatantsUseBaseIncomeAndEnergyCap(t *testing.T) {
	lib := shippedCards(t, "adventurer_v1", "venom_v1", "curse_v1", "minions_v1")
	for _, id := range []string{"blade_warden", "venom_goblin", "adventurer", "venom", "curse", "drowned_oracle_brine_mask"} {
		c, ok := lib.Combatants[id]
		if !ok {
			t.Fatalf("missing combatant %q", id)
		}
		if c.Income.Cards != 1 || c.Income.Energy != 5 || c.Resources.EnergyCap != DefaultEnergyCap || c.Resources.EffectiveEnergyCap() != 75 {
			t.Fatalf("%s income %+v resources %+v", id, c.Income, c.Resources)
		}
	}
}

func TestCombatantEnergyCapValidation(t *testing.T) {
	if (CombatantResources{}).EffectiveEnergyCap() != DefaultEnergyCap {
		t.Fatal("an omitted energy cap must use the game default")
	}
	for name, resources := range map[string]CombatantResources{
		"negative cap":            {HandLimit: 6, StartingEnergy: 10, EnergyCap: -1},
		"start above cap":         {HandLimit: 6, StartingEnergy: 11, EnergyCap: 10},
		"start above default cap": {HandLimit: 6, StartingEnergy: DefaultEnergyCap + 1},
	} {
		t.Run(name, func(t *testing.T) {
			lib := shippedCards(t)
			c := lib.Combatants["blade_warden"]
			c.Resources = resources
			lib.Combatants["blade_warden"] = c
			if err := validateBattleLibrary(lib); !errors.Is(err, ErrInvalidContent) {
				t.Fatalf("validation error = %v", err)
			}
		})
	}
}
