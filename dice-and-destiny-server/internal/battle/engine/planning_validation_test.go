package engine

import (
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func TestFinalPlanningSelectionValidatesTargetsDiceAndResources(t *testing.T) {
	battle := &state.Battle{
		Content: state.ContentCatalog{
			Abilities: map[string]state.RuntimeContentDefinition{
				"Targeted": {
					ID:              "Targeted",
					Segments:        []segment.Segment{segment.Offensive},
					RequiresTarget:  true,
					DiceRequirement: "none",
				},
				"Five Sixes": {
					ID:              "Five Sixes",
					Segments:        []segment.Segment{segment.Offensive},
					DiceRequirement: "five_sixes",
				},
				"Expensive": {
					ID:              "Expensive",
					Segments:        []segment.Segment{segment.Offensive},
					DiceRequirement: "none",
					EnergyCost:      3,
				},
			},
		},
	}
	actor := state.ActorState{Resources: state.ResourceState{EnergyPoints: 2}}
	tests := []struct {
		name string
		plan state.PlanningActorState
		want string
	}{
		{
			name: "target required",
			plan: state.PlanningActorState{SelectedAbility: "Targeted"},
			want: "requires a target",
		},
		{
			name: "dice requirement",
			plan: state.PlanningActorState{
				SelectedAbility: "Five Sixes",
				FinalDice:       []state.RolledDie{{Face: 6}, {Face: 6}, {Face: 6}, {Face: 6}, {Face: 5}},
			},
			want: "dice requirement",
		},
		{
			name: "resource cost",
			plan: state.PlanningActorState{SelectedAbility: "Expensive"},
			want: "costs 3 energy points",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateFinalPlanningSelection(battle, actor, test.plan, segment.Offensive)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateFinalPlanningSelection() error = %v, want %q", err, test.want)
			}
		})
	}
}
