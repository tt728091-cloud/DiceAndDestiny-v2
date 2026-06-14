package content_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/content"
)

func TestTypedCardAbilityAndStatusesLoadAndCompile(t *testing.T) {
	root := typedContentRoot(t)
	library, err := content.LoadContentLibrary(root)
	if err != nil {
		t.Fatalf("LoadContentLibrary() returned error: %v", err)
	}
	if len(library.Cards["Test Card"].Operations) != 1 ||
		len(library.Abilities["Test Ability"].Operations) != 1 {
		t.Fatalf("card/ability operations were not compiled: %#v", library)
	}
	poison := library.Statuses["poison"].Triggers[0].Operations
	advanced := library.Statuses["advanced_poison"].Triggers[0].Operations
	if reflect.DeepEqual(poison, advanced) {
		t.Fatal("Poison and Advanced Poison compiled to identical plans")
	}
	if poison[0].Type != operation.TypeRollDice ||
		poison[0].Operations[0].Type != operation.TypeEvaluateRollOutcome ||
		*poison[0].Operations[0].Outcomes[0].Operations[0].Amount != 1 ||
		*advanced[0].Operations[0].Outcomes[0].Operations[0].Amount != 2 {
		t.Fatalf("compiled poison plans = %#v / %#v", poison, advanced)
	}
}

func TestContentLoadingUsesInjectedOperationRegistry(t *testing.T) {
	registry, err := operation.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() returned error: %v", err)
	}
	_, err = content.LoadContentLibraryWithRegistry(typedContentRoot(t), registry)
	if err == nil || !strings.Contains(err.Error(), "no registered handler") {
		t.Fatalf("LoadContentLibraryWithRegistry() error = %v, want missing handler", err)
	}
}

func TestNamedStackOverflowPolicyReferenceLoads(t *testing.T) {
	root := typedContentRoot(t)
	status := strings.Replace(
		validPoison("poison", "Poison", 1, "[1, 2, 3, 4]"),
		"reject_additional_stacks",
		"resolve_existing_then_apply",
		1,
	)
	writeTypedFile(t, filepath.Join(root, "statuses", "poison.yaml"), status)
	library, err := content.LoadContentLibrary(root)
	if err != nil {
		t.Fatalf("LoadContentLibrary() returned error: %v", err)
	}
	if library.Statuses["poison"].StackOverflowPolicy != "resolve_existing_then_apply" {
		t.Fatalf("stack overflow policy = %q", library.Statuses["poison"].StackOverflowPolicy)
	}
}

func TestTypedContentRejectsUnknownFieldsAndInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		status string
		card   string
		want   string
	}{
		{
			name: "unknown operation field",
			card: strings.Replace(validTypedCard(), "    type: noop", "    type: noop\n    script: unsafe", 1),
			want: "field script not found",
		},
		{
			name: "unknown operation type",
			card: strings.Replace(validTypedCard(), "type: noop", "type: script", 1),
			want: "no registered handler",
		},
		{
			name: "unknown status reference",
			card: strings.Replace(
				validTypedCard(),
				"id: card_noop\n    type: noop",
				"id: apply_missing\n    type: apply_status\n    target: self\n    status_id: missing\n    stack_count: 1",
				1,
			),
			want: "references unknown status",
		},
		{
			name:   "unknown trigger segment",
			status: strings.Replace(validPoison("poison", "Poison", 1, "[1, 2, 3, 4]"), "segment: ongoing_effects", "segment: before_draw", 1),
			want:   "unknown trigger segment",
		},
		{
			name:   "unknown trigger phase",
			status: strings.Replace(validPoison("poison", "Poison", 1, "[1, 2, 3, 4]"), "phase: on_enter", "phase: after_roll", 1),
			want:   "unknown trigger phase",
		},
		{
			name:   "invalid stack limit",
			status: strings.Replace(validPoison("poison", "Poison", 1, "[1, 2, 3, 4]"), "stack_limit: 3", "stack_limit: 0", 1),
			want:   "stack_limit must be positive",
		},
		{
			name:   "unknown overflow policy",
			status: strings.Replace(validPoison("poison", "Poison", 1, "[1, 2, 3, 4]"), "reject_additional_stacks", "baryl_magic", 1),
			want:   "unknown stack_overflow_policy",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := typedContentRoot(t)
			if test.card != "" {
				writeTypedFile(t, filepath.Join(root, "cards", "test.yaml"), test.card)
			}
			if test.status != "" {
				writeTypedFile(t, filepath.Join(root, "statuses", "poison.yaml"), test.status)
			}
			_, err := content.LoadContentLibrary(root)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("LoadContentLibrary() error = %v, want %q", err, test.want)
			}
		})
	}
}

func typedContentRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"cards", "abilities", "dice", "statuses"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("MkdirAll() returned error: %v", err)
		}
	}
	writeTypedFile(t, filepath.Join(root, "cards", "test.yaml"), validTypedCard())
	writeTypedFile(t, filepath.Join(root, "abilities", "test.yaml"), `schema_version: 1
id: Test Ability
name: Test Ability
type: offensive
phase_restrictions: [offensive]
dice_requirement:
  kind: none
cost:
  energy_points: 1
requires_target: true
effects:
  - id: ability_damage
    type: deal_damage
    target: selected_targets
    amount: 2
`)
	writeTypedFile(t, filepath.Join(root, "dice", "test.yaml"), `schema_version: 1
id: Test D6
name: Test D6
die_type: d6
side_count: 6
faces:
  - {face: 1, value: 1, symbols: []}
  - {face: 2, value: 2, symbols: []}
  - {face: 3, value: 3, symbols: []}
  - {face: 4, value: 4, symbols: []}
  - {face: 5, value: 5, symbols: []}
  - {face: 6, value: 6, symbols: []}
`)
	writeTypedFile(t, filepath.Join(root, "statuses", "poison.yaml"), validPoison("poison", "Poison", 1, "[1, 2, 3, 4]"))
	writeTypedFile(t, filepath.Join(root, "statuses", "advanced.yaml"), validPoison("advanced_poison", "Advanced Poison", 2, "[1, 2, 3, 4, 5]"))
	return root
}

func validTypedCard() string {
	return `schema_version: 1
id: Test Card
name: Test Card
type: action
cost:
  energy_points: 0
phase_restrictions: [offensive]
effects:
  - id: card_noop
    type: noop
`
}

func validPoison(id, name string, amount int, faces string) string {
	return `schema_version: 1
id: ` + id + `
name: ` + name + `
stack_limit: 3
stack_overflow_policy: reject_additional_stacks
triggers:
  - id: on_enter
    segment: ongoing_effects
    phase: on_enter
    priority: 0
    resolution:
      - id: rolls
        type: roll_dice
        target: self
        one_per_status_stack: true
        side_count: 6
        operations:
          - id: outcomes
            type: evaluate_roll_outcome
            outcomes:
              - id: damage
                faces: ` + faces + `
                operations:
                  - id: damage_op
                    type: deal_damage
                    target: self
                    amount: ` + string(rune('0'+amount)) + `
              - id: remove
                faces: [6]
                operations:
                  - id: remove_op
                    type: remove_status_stack
                    target: selected_status
                    stack_count: 1
`
}

func writeTypedFile(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatalf("WriteFile() returned error: %v", err)
	}
}
