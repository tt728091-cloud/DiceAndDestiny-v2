package content_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"diceanddestiny/server/internal/content"
)

func TestLoadCharacterCombatSheetLoadsReferencesAndDerivesMaxHealth(t *testing.T) {
	root := writeContentLibrary(t)
	sheetPath := writeCharacterSheet(t, root, `schema_version: 1
actor_id: player
character:
  id: Mock Paladin
  name: Mock Paladin
  class: paladin
resources:
  starting_hand_size: 4
  max_hand_size: 7
  starting_energy_points: 2
  max_energy_points: 10
health:
  model: card_zones
  max_health: 20
decklist:
  - card_id: Mock Strike
    count: 8
  - card_id: Mock Guard
    count: 6
  - card_id: Mock Focus
    count: 6
dice_loadout:
  - dice_id: Standard D6
    count: 5
abilities:
  - Mock Smite
  - Mock Guarding Light
`)

	got, err := content.LoadCharacterCombatSheet(sheetPath, root)
	if err != nil {
		t.Fatalf("LoadCharacterCombatSheet() returned error: %v", err)
	}

	want := content.CharacterCombatSheet{
		SchemaVersion: 1,
		ActorID:       "player",
		Character: content.CharacterMetadata{
			ID:    "Mock Paladin",
			Name:  "Mock Paladin",
			Class: "paladin",
		},
		Resources: content.StartingResources{
			StartingHandSize:     4,
			MaxHandSize:          7,
			StartingEnergyPoints: 2,
			MaxEnergyPoints:      10,
		},
		Health: content.CharacterHealth{
			Model:     "card_zones",
			MaxHealth: 20,
		},
		Decklist: []content.DecklistEntry{
			{CardID: "Mock Strike", Count: 8},
			{CardID: "Mock Guard", Count: 6},
			{CardID: "Mock Focus", Count: 6},
		},
		DiceLoadout: []content.DiceLoadoutEntry{
			{DiceID: "Standard D6", Count: 5},
		},
		AbilityIDs: []string{"Mock Smite", "Mock Guarding Light"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCharacterCombatSheet() = %#v, want %#v", got, want)
	}
}

func TestLoadCharacterCombatSheetRejectsMissingCardReference(t *testing.T) {
	root := writeContentLibrary(t)
	sheetPath := writeCharacterSheet(t, root, validCharacterSheetWith(`  - card_id: Missing Card
    count: 1
`))

	got, err := content.LoadCharacterCombatSheet(sheetPath, root)
	if err == nil {
		t.Fatalf("LoadCharacterCombatSheet() succeeded with sheet %#v", got)
	}
	if !errors.Is(err, content.ErrInvalidCharacterCombatSheet) {
		t.Fatalf("LoadCharacterCombatSheet() error = %v, want ErrInvalidCharacterCombatSheet", err)
	}
	if !strings.Contains(err.Error(), `referenced card "Missing Card" was not found`) {
		t.Fatalf("LoadCharacterCombatSheet() error = %q, want missing card context", err.Error())
	}
}

func TestLoadCharacterCombatSheetRejectsMissingAbilityReference(t *testing.T) {
	root := writeContentLibrary(t)
	sheetPath := writeCharacterSheet(t, root, strings.Replace(validCharacterSheet(), "  - Mock Smite\n", "  - Missing Ability\n", 1))

	got, err := content.LoadCharacterCombatSheet(sheetPath, root)
	if err == nil {
		t.Fatalf("LoadCharacterCombatSheet() succeeded with sheet %#v", got)
	}
	if !errors.Is(err, content.ErrInvalidCharacterCombatSheet) {
		t.Fatalf("LoadCharacterCombatSheet() error = %v, want ErrInvalidCharacterCombatSheet", err)
	}
	if !strings.Contains(err.Error(), `referenced ability "Missing Ability" was not found`) {
		t.Fatalf("LoadCharacterCombatSheet() error = %q, want missing ability context", err.Error())
	}
}

func TestLoadCharacterCombatSheetRejectsMissingDiceReference(t *testing.T) {
	root := writeContentLibrary(t)
	sheetPath := writeCharacterSheet(t, root, strings.Replace(validCharacterSheet(), "dice_id: Standard D6", "dice_id: Missing Dice", 1))

	got, err := content.LoadCharacterCombatSheet(sheetPath, root)
	if err == nil {
		t.Fatalf("LoadCharacterCombatSheet() succeeded with sheet %#v", got)
	}
	if !errors.Is(err, content.ErrInvalidCharacterCombatSheet) {
		t.Fatalf("LoadCharacterCombatSheet() error = %v, want ErrInvalidCharacterCombatSheet", err)
	}
	if !strings.Contains(err.Error(), `referenced dice "Missing Dice" was not found`) {
		t.Fatalf("LoadCharacterCombatSheet() error = %q, want missing dice context", err.Error())
	}
}

func TestLoadCharacterCombatSheetRejectsMutableCombatZones(t *testing.T) {
	root := writeContentLibrary(t)
	sheetPath := writeCharacterSheet(t, root, validCharacterSheet()+`cards:
  deck: [Mock Strike]
  hand: []
  discard: []
  removed: []
`)

	got, err := content.LoadCharacterCombatSheet(sheetPath, root)
	if err == nil {
		t.Fatalf("LoadCharacterCombatSheet() succeeded with sheet %#v", got)
	}
	if !strings.Contains(err.Error(), "field cards not found") {
		t.Fatalf("LoadCharacterCombatSheet() error = %q, want unknown cards field rejection", err.Error())
	}
}

func TestLoadCharacterCombatSheetRejectsAuthoredMaxHealthMismatch(t *testing.T) {
	root := writeContentLibrary(t)
	sheetPath := writeCharacterSheet(t, root, strings.Replace(validCharacterSheet(), "max_health: 20", "max_health: 19", 1))

	got, err := content.LoadCharacterCombatSheet(sheetPath, root)
	if err == nil {
		t.Fatalf("LoadCharacterCombatSheet() succeeded with sheet %#v", got)
	}
	if !errors.Is(err, content.ErrInvalidCharacterCombatSheet) {
		t.Fatalf("LoadCharacterCombatSheet() error = %v, want ErrInvalidCharacterCombatSheet", err)
	}
	if !strings.Contains(err.Error(), "must match decklist total 20") {
		t.Fatalf("LoadCharacterCombatSheet() error = %q, want derived max health context", err.Error())
	}
}

func TestLoadContentLibraryRejectsDuplicateCardIDs(t *testing.T) {
	root := writeContentLibrary(t)
	writeFile(t, filepath.Join(root, "cards", "duplicate.yaml"), placeholderCard("Mock Strike"))

	got, err := content.LoadContentLibrary(root)
	if err == nil {
		t.Fatalf("LoadContentLibrary() succeeded with library %#v", got)
	}
	if !errors.Is(err, content.ErrInvalidContent) {
		t.Fatalf("LoadContentLibrary() error = %v, want ErrInvalidContent", err)
	}
	if !strings.Contains(err.Error(), `duplicate card id "Mock Strike"`) {
		t.Fatalf("LoadContentLibrary() error = %q, want duplicate card context", err.Error())
	}
}

func TestLoadContentLibraryRejectsOldPhaseNames(t *testing.T) {
	root := writeContentLibrary(t)
	writeFile(t, filepath.Join(root, "abilities", "old-phase.yaml"), `schema_version: 1
id: Old Phase Ability
name: Old Phase Ability
type: offensive
phase_restrictions:
  - offensive_roll
dice_requirement:
  kind: none
cost:
  energy_points: 0
requires_target: true
effects:
  - type: noop
`)

	got, err := content.LoadContentLibrary(root)
	if err == nil {
		t.Fatalf("LoadContentLibrary() succeeded with library %#v", got)
	}
	if !errors.Is(err, content.ErrInvalidContent) {
		t.Fatalf("LoadContentLibrary() error = %v, want ErrInvalidContent", err)
	}
	if !strings.Contains(err.Error(), `unknown segment "offensive_roll"`) {
		t.Fatalf("LoadContentLibrary() error = %q, want old phase rejection", err.Error())
	}
}

func TestRepositoryMockPaladinContentLoads(t *testing.T) {
	repoRoot := serverRoot(t)

	got, err := content.LoadCharacterCombatSheet(
		filepath.Join(repoRoot, "content", "characters", "mock_paladin.yaml"),
		filepath.Join(repoRoot, "content"),
	)
	if err != nil {
		t.Fatalf("LoadCharacterCombatSheet() returned error: %v", err)
	}

	if got.ActorID != "player" {
		t.Fatalf("actor ID = %q, want player", got.ActorID)
	}
	if got.Health.MaxHealth != 20 {
		t.Fatalf("derived max health = %d, want 20", got.Health.MaxHealth)
	}
	if len(got.Decklist) != 3 {
		t.Fatalf("decklist length = %d, want 3", len(got.Decklist))
	}
	if len(got.AbilityIDs) != 4 {
		t.Fatalf("ability count = %d, want 4", len(got.AbilityIDs))
	}
	if !reflect.DeepEqual(got.DiceLoadout, []content.DiceLoadoutEntry{{DiceID: "Standard D6", Count: 5}}) {
		t.Fatalf("dice loadout = %#v, want Standard D6 x5", got.DiceLoadout)
	}
}

func writeContentLibrary(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cards"), 0o755); err != nil {
		t.Fatalf("MkdirAll(cards) returned error: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "abilities"), 0o755); err != nil {
		t.Fatalf("MkdirAll(abilities) returned error: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "dice"), 0o755); err != nil {
		t.Fatalf("MkdirAll(dice) returned error: %v", err)
	}

	writeFile(t, filepath.Join(root, "cards", "mock-strike.yaml"), placeholderCard("Mock Strike"))
	writeFile(t, filepath.Join(root, "cards", "mock-guard.yaml"), placeholderCard("Mock Guard"))
	writeFile(t, filepath.Join(root, "cards", "mock-focus.yaml"), placeholderCard("Mock Focus"))
	writeFile(t, filepath.Join(root, "abilities", "mock-smite.yaml"), placeholderAbility("Mock Smite", "offensive", "small_straight", true))
	writeFile(t, filepath.Join(root, "abilities", "mock-guarding-light.yaml"), placeholderAbility("Mock Guarding Light", "defensive", "none", false))
	writeFile(t, filepath.Join(root, "dice", "standard-d6.yaml"), standardD6())

	return root
}

func writeCharacterSheet(t *testing.T, root, contents string) string {
	t.Helper()

	path := filepath.Join(root, "mock-paladin.yaml")
	writeFile(t, path, contents)
	return path
}

func validCharacterSheet() string {
	return validCharacterSheetWith(`  - card_id: Mock Strike
    count: 8
  - card_id: Mock Guard
    count: 6
  - card_id: Mock Focus
    count: 6
`)
}

func validCharacterSheetWith(decklist string) string {
	return `schema_version: 1
actor_id: player
character:
  id: Mock Paladin
  name: Mock Paladin
  class: paladin
resources:
  starting_hand_size: 4
  max_hand_size: 7
  starting_energy_points: 2
  max_energy_points: 10
health:
  model: card_zones
  max_health: 20
decklist:
` + decklist + `dice_loadout:
  - dice_id: Standard D6
    count: 5
abilities:
  - Mock Smite
  - Mock Guarding Light
`
}

func placeholderCard(id string) string {
	return `schema_version: 1
id: ` + id + `
name: ` + id + `
type: placeholder
cost:
  energy_points: 0
phase_restrictions: []
effects:
  - type: noop
`
}

func placeholderAbility(id, segment, requirement string, requiresTarget bool) string {
	return `schema_version: 1
id: ` + id + `
name: ` + id + `
type: ` + segment + `
phase_restrictions:
  - ` + segment + `
dice_requirement:
  kind: ` + requirement + `
cost:
  energy_points: 0
requires_target: ` + boolString(requiresTarget) + `
effects:
  - type: noop
`
}

func standardD6() string {
	return `schema_version: 1
id: Standard D6
name: Standard D6
die_type: d6
side_count: 6
faces:
  - face: 1
    value: 1
    symbols: []
  - face: 2
    value: 2
    symbols: []
  - face: 3
    value: 3
    symbols: []
  - face: 4
    value: 4
    symbols: []
  - face: 5
    value: 5
    symbols: []
  - face: 6
    value: 6
    symbols: []
`
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) returned error: %v", path, err)
	}
}

func serverRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}
