package scenario_test

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle"
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/scenario"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func TestRoundTwoPoisonScenarioBuildsAtUnenteredBoundary(t *testing.T) {
	catalog, builder := fixtureCatalogAndBuilder(t)
	spec, err := catalog.Load("round-2-poisoned-player")
	if err != nil {
		t.Fatal(err)
	}
	built, err := builder.Build(spec)
	if err != nil {
		t.Fatal(err)
	}
	if built.Segment.Round != 2 || built.Segment.Current != segment.OngoingEffects ||
		built.Flow.Entered {
		t.Fatalf("entry = %#v / %#v, want round 2 ongoing_effects before OnEnter", built.Segment, built.Flow)
	}
	player := built.Actors["player"]
	if player.CurrentHealth() != 16 || len(player.Cards.Removed) != 4 {
		t.Fatalf("player health/zones = %d / %#v, want four removed health cards", player.CurrentHealth(), player.Cards)
	}
	if len(player.Statuses) != 1 || player.Statuses[0].DefinitionID != "poison" ||
		player.Statuses[0].Stacks != 2 {
		t.Fatalf("player statuses = %#v, want two Poison stacks", player.Statuses)
	}
	if built.Origin.Kind != state.BattleOriginScenario ||
		built.Origin.ScenarioFingerprint == "" ||
		built.Random.Mode != state.RandomModeReproducible {
		t.Fatalf("scenario metadata = %#v / %#v", built.Origin, built.Random)
	}
}

func TestScenarioFingerprintIsCanonicalAndDetectsChanges(t *testing.T) {
	catalog, _ := fixtureCatalogAndBuilder(t)
	spec, err := catalog.Load("round-2-poisoned-player")
	if err != nil {
		t.Fatal(err)
	}
	first, err := scenario.Fingerprint(spec)
	if err != nil {
		t.Fatal(err)
	}
	reordered := spec
	reordered.Actors = map[string]scenario.ActorOverride{"player": spec.Actors["player"]}
	second, err := scenario.Fingerprint(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("map insertion changed fingerprint: %q != %q", first, second)
	}
	reordered.BattleID = "scenario-generated-id"
	withBattleID, err := scenario.Fingerprint(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if first != withBattleID {
		t.Fatalf("generated battle id changed fingerprint: %q != %q", first, withBattleID)
	}
	reordered.BattleID = ""
	reordered.Entry.Round++
	changed, err := scenario.Fingerprint(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if changed == first {
		t.Fatal("changed scenario retained fingerprint")
	}
}

func TestScenarioValidationRejectsInvalidActorOverrides(t *testing.T) {
	catalog, builder := fixtureCatalogAndBuilder(t)
	base, err := catalog.Load("round-2-poisoned-player")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		edit func(*scenario.Spec)
		want string
	}{
		{
			name: "card conservation",
			edit: func(spec *scenario.Spec) {
				override := spec.Actors["player"]
				override.CardZones.Deck = override.CardZones.Deck[:15]
				spec.Actors["player"] = override
			},
			want: "copies",
		},
		{
			name: "status stack limit",
			edit: func(spec *scenario.Spec) {
				override := spec.Actors["player"]
				(*override.Statuses)[0].Stacks = 4
				spec.Actors["player"] = override
			},
			want: "outside 1..3",
		},
		{
			name: "resource limit",
			edit: func(spec *scenario.Spec) {
				override := spec.Actors["player"].SetEnergy(11)
				spec.Actors["player"] = override
			},
			want: "energy",
		},
		{
			name: "duplicate tokens",
			edit: func(spec *scenario.Spec) {
				override := spec.Actors["player"].SetTokens([]state.TokenState{
					{ID: "ward", Value: 1},
					{ID: "ward", Value: 2},
				})
				spec.Actors["player"] = override
			},
			want: "duplicate id",
		},
		{
			name: "inconsistent defeat",
			edit: func(spec *scenario.Spec) {
				override := spec.Actors["player"].SetDefeatState(state.ActorDefeated)
				spec.Actors["player"] = override
			},
			want: "remaining health",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := cloneSpec(t, base)
			test.edit(&spec)
			_, err := builder.Build(spec)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Build() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestScenarioPrerequisiteValidation(t *testing.T) {
	catalog, builder := fixtureCatalogAndBuilder(t)
	spec, err := catalog.Load("round-2-poisoned-player")
	if err != nil {
		t.Fatal(err)
	}
	spec.Entry.Segment = segment.Defensive
	if _, err := builder.Build(spec); err == nil || !strings.Contains(err.Error(), "requires a finalized defensible") {
		t.Fatalf("defensive Build() error = %v", err)
	}
	spec.Prerequisites.OffensiveProposals = []state.PlanningProposal{{
		ID:         "player-offense",
		ActorID:    "player",
		Segment:    segment.Offensive,
		Defensible: true,
		Commitment: state.PlanningCommitmentData{LockedIn: true},
	}}
	if _, err := builder.Build(spec); err != nil {
		t.Fatalf("defensive Build() with proposal: %v", err)
	}
	spec.Entry.Segment = segment.DamageResolution
	if _, err := builder.Build(spec); err != nil {
		t.Fatalf("damage Build() with proposal: %v", err)
	}
}

func TestReproducibleRandomnessAndScriptedExactWait(t *testing.T) {
	catalog, builder := fixtureCatalogAndBuilder(t)
	spec, err := catalog.Load("round-2-poisoned-player")
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{"pending_input_id": "$pending_input_id"})
	spec.SetupScript = []scenario.ScriptStep{{
		ActorID: "player",
		Type:    command.TypeRollDice,
		Payload: payload,
		Expect: scenario.WaitExpectation{
			Segment:       segment.Offensive,
			InputType:     "planning",
			WindowPurpose: state.InteractionPurposePlanning,
		},
	}}
	firstSpec := spec
	firstSpec.BattleID = "scenario-random-one"
	secondSpec := spec
	secondSpec.BattleID = "scenario-random-two"
	first, firstProgress, err := builder.BuildAndProgress(firstSpec, engine.NewEngine())
	if err != nil {
		t.Fatal(err)
	}
	second, secondProgress, err := builder.BuildAndProgress(secondSpec, engine.NewEngine())
	if err != nil {
		t.Fatal(err)
	}
	if firstProgress.Status != engine.ProgressWaitingForInput ||
		secondProgress.Status != engine.ProgressWaitingForInput {
		t.Fatalf("progress statuses = %q / %q", firstProgress.Status, secondProgress.Status)
	}
	if !reflect.DeepEqual(first.Actors["player"].Dice, second.Actors["player"].Dice) ||
		!reflect.DeepEqual(first.Actors["goblin-1"].Dice, second.Actors["goblin-1"].Dice) ||
		first.Random.Cursor != second.Random.Cursor ||
		first.Random.Cursor == 0 {
		t.Fatalf("reproducible states differ:\n%#v\n%#v", first.Random, second.Random)
	}
}

func TestNormalRandomnessUsesPersistedPerBattleCursor(t *testing.T) {
	catalog, builder := fixtureCatalogAndBuilder(t)
	spec, err := catalog.Load("round-2-poisoned-player")
	if err != nil {
		t.Fatal(err)
	}
	spec.BattleID = "scenario-normal-random"
	spec.Random = scenario.RandomPolicy{Mode: state.RandomModeNormal}
	built, _, err := builder.BuildAndProgress(spec, engine.NewEngine())
	if err != nil {
		t.Fatal(err)
	}
	if built.Random.Mode != state.RandomModeNormal ||
		built.Random.Algorithm != state.RandomAlgorithmCrypto ||
		built.Random.Cursor == 0 {
		t.Fatalf("normal random state = %#v", built.Random)
	}
}

func TestRunPlayerAndCharacterDefinitionBaselines(t *testing.T) {
	catalog, builder := fixtureCatalogAndBuilder(t)
	spec, err := catalog.Load("round-2-poisoned-player")
	if err != nil {
		t.Fatal(err)
	}
	spec.Actors = nil
	character, err := builder.Build(spec)
	if err != nil {
		t.Fatal(err)
	}
	if character.Actors["player"].DefinitionID != "mock_paladin" {
		t.Fatalf("character definition = %q", character.Actors["player"].DefinitionID)
	}
	spec.Player.DefinitionID = "current_run_player"
	spec.Player.Source = "run_player"
	runPlayer, err := builder.Build(spec)
	if err != nil {
		t.Fatal(err)
	}
	if runPlayer.Actors["player"].DefinitionID != "current_run_player" {
		t.Fatalf("run player definition = %q", runPlayer.Actors["player"].DefinitionID)
	}
}

func TestCatalogRejectsUnsafeIDs(t *testing.T) {
	for _, id := range []string{"../secret", "/tmp/file", `bad\path`, ""} {
		if err := scenario.ValidateScenarioID(id); err == nil {
			t.Fatalf("ValidateScenarioID(%q) succeeded", id)
		}
	}
}

func fixtureCatalogAndBuilder(t *testing.T) (scenario.Catalog, scenario.Builder) {
	t.Helper()
	root := serverRoot(t)
	return scenario.Catalog{Root: filepath.Join(root, "scenarios")}, scenario.Builder{
		Assembler: battle.NewFileParticipantAssembler(
			filepath.Join(root, "content"),
			filepath.Join(root, "save", "run_players"),
		),
	}
}

func serverRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
}

func cloneSpec(t *testing.T, value scenario.Spec) scenario.Spec {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var cloned scenario.Spec
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}
