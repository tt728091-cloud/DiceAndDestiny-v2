package operation_test

import (
	"reflect"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/segment"
)

func TestDefaultRegistryCompilesNestedOutcomesDeterministically(t *testing.T) {
	registry := operation.DefaultRegistry()
	definitions := []operation.Definition{
		{
			Type:              operation.TypeRollDice,
			Target:            operation.TargetSelf,
			OnePerStatusStack: true,
			SideCount:         intPointer(6),
			Operations: []operation.Definition{
				{
					Type: operation.TypeEvaluateRollOutcome,
					Outcomes: []operation.OutcomeDefinition{
						{
							ID:    "damage",
							Faces: []int{1, 2, 3, 4},
							Operations: []operation.Definition{
								{
									Type:   operation.TypeDealDamage,
									Target: operation.TargetSelf,
									Amount: intPointer(1),
								},
							},
						},
					},
				},
			},
		},
	}

	first, err := registry.Compile("statuses.poison.triggers[0]", definitions)
	if err != nil {
		t.Fatalf("Compile() returned error: %v", err)
	}
	second, err := registry.Compile("statuses.poison.triggers[0]", definitions)
	if err != nil {
		t.Fatalf("second Compile() returned error: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("compiled plans differ:\n%#v\n%#v", first, second)
	}
	if first[0].ID != "statuses.poison.triggers[0].operations[0]" ||
		first[0].Operations[0].ID != "statuses.poison.triggers[0].operations[0].operations[0]" ||
		first[0].Operations[0].Outcomes[0].Operations[0].ID !=
			"statuses.poison.triggers[0].operations[0].operations[0].outcomes[0].operations[0]" {
		t.Fatalf("deterministic path IDs = %#v", first)
	}
}

func TestRegistryRejectsUnknownTypeAndInvalidCombinations(t *testing.T) {
	registry := operation.DefaultRegistry()
	tests := []struct {
		name string
		def  operation.Definition
		want string
	}{
		{
			name: "unknown type",
			def:  operation.Definition{Type: "script"},
			want: "no registered handler",
		},
		{
			name: "missing damage amount",
			def:  operation.Definition{Type: operation.TypeDealDamage, Target: operation.TargetSelf},
			want: "amount must be positive",
		},
		{
			name: "invalid noop parameter",
			def:  operation.Definition{Type: operation.TypeNoop, Amount: intPointer(1)},
			want: "noop does not accept parameters",
		},
		{
			name: "unknown target",
			def:  operation.Definition{Type: operation.TypeDealDamage, Target: "everyone", Amount: intPointer(1)},
			want: "unknown target selector",
		},
		{
			name: "unknown zone",
			def: operation.Definition{
				Type:            operation.TypeMoveCards,
				Target:          operation.TargetSelf,
				Amount:          intPointer(1),
				SourceZone:      "limbo",
				DestinationZone: operation.ZoneHand,
			},
			want: "unknown source zone",
		},
		{
			name: "unknown resource",
			def: operation.Definition{
				Type:     operation.TypeGainResource,
				Target:   operation.TargetSelf,
				Amount:   intPointer(1),
				Resource: "mana",
			},
			want: "unknown resource",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := registry.Compile("test", []operation.Definition{test.def})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Compile() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestDefaultRegistrySupportsEveryReusableOperationType(t *testing.T) {
	registry := operation.DefaultRegistry()
	definitions := []operation.Definition{
		{ID: "roll", Type: operation.TypeRollDice, Target: operation.TargetSelf, DiceCount: intPointer(2), SideCount: intPointer(6)},
		{
			ID:   "evaluate",
			Type: operation.TypeEvaluateRollOutcome,
			Outcomes: []operation.OutcomeDefinition{
				{ID: "low", MinFace: intPointer(1), MaxFace: intPointer(3), Operations: []operation.Definition{{ID: "nested_noop", Type: operation.TypeNoop}}},
			},
		},
		{ID: "modify", Type: operation.TypeModifyDie, Target: operation.TargetSelectedDie, Modification: operation.DieModificationSetFace, Face: intPointer(6)},
		{ID: "reroll", Type: operation.TypeRerollDie, Target: operation.TargetSelectedDie},
		{ID: "damage", Type: operation.TypeDealDamage, Target: operation.TargetSelectedTargets, Amount: intPointer(2)},
		{ID: "prevent", Type: operation.TypePreventDamage, Target: operation.TargetSelectedProposal, Amount: intPointer(1)},
		{ID: "apply", Type: operation.TypeApplyStatus, Target: operation.TargetSelectedTargets, StatusID: "poison", StackCount: intPointer(1)},
		{ID: "remove", Type: operation.TypeRemoveStatusStack, Target: operation.TargetSelectedStatus, StackCount: intPointer(1)},
		{ID: "move", Type: operation.TypeMoveCards, Target: operation.TargetSelf, Amount: intPointer(1), SourceZone: operation.ZoneHand, DestinationZone: operation.ZoneDiscard},
		{ID: "draw", Type: operation.TypeDrawCards, Target: operation.TargetSelf, Amount: intPointer(2), SourceZone: operation.ZoneDeck},
		{ID: "resource", Type: operation.TypeGainResource, Target: operation.TargetSelf, Amount: intPointer(1), Resource: operation.ResourceEnergyPoints},
		{ID: "target", Type: operation.TypeChangeTarget, Source: operation.TargetSelectedProposal, Target: operation.TargetSelectedTargets},
		{ID: "noop", Type: operation.TypeNoop},
	}
	plans, err := registry.Compile("all", definitions)
	if err != nil {
		t.Fatalf("Compile() returned error: %v", err)
	}
	if len(plans) != len(definitions) {
		t.Fatalf("plan count = %d, want %d", len(plans), len(definitions))
	}
}

func TestTriggerValidationAndOrdering(t *testing.T) {
	if err := operation.ValidateTrigger(operation.Trigger{Segment: "before_draw", Phase: "on_enter"}); err == nil {
		t.Fatal("unknown segment was accepted")
	}
	if err := operation.ValidateTrigger(operation.Trigger{Segment: segment.Income, Phase: "after_roll"}); err == nil {
		t.Fatal("unknown phase was accepted")
	}

	values := []operation.EffectInstanceTrigger{
		{Trigger: operation.Trigger{Priority: 0}, CreationOrder: 2, InstanceID: "b"},
		{Trigger: operation.Trigger{Priority: 5}, CreationOrder: 9, InstanceID: "z"},
		{Trigger: operation.Trigger{Priority: 0}, CreationOrder: 1, InstanceID: "c"},
		{Trigger: operation.Trigger{Priority: 0}, CreationOrder: 1, InstanceID: "a"},
	}
	operation.SortTriggers(values)
	got := []string{values[0].InstanceID, values[1].InstanceID, values[2].InstanceID, values[3].InstanceID}
	want := []string{"z", "a", "c", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("trigger order = %v, want %v", got, want)
	}
	values[1].Segment = segment.Income
	values[1].Phase = "on_enter"
	matches := operation.MatchingTriggers(values, segment.Income, "on_enter")
	if len(matches) != 1 || matches[0].InstanceID != values[1].InstanceID {
		t.Fatalf("matching triggers = %#v", matches)
	}
}

func intPointer(value int) *int {
	return &value
}
