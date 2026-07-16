package engine

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	battlerandom "diceanddestiny/server/internal/battle/random"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

func TestSettledQualificationKeepsEveryValidHumanChoiceAndIndependentBonuses(t *testing.T) {
	library := settledTestLibrary(t)
	dice := rolledFaces(library, "standard_d6", []int{1, 1, 2, 3, 6})
	ids := qualifiedAbilities(library, []string{"sword_cut", "golden_edge", "shield_bash", "perfect_form"}, dice, nil)
	if len(ids) != 2 || ids[0] != "golden_edge" || ids[1] != "sword_cut" {
		t.Fatalf("qualified abilities=%v", ids)
	}
	ability := library.Abilities["sword_cut"]
	tier, ok := qualifiedTier(ability, dice)
	if !ok || tier.ID != "four_swords" {
		t.Fatalf("tier=%#v ok=%v", tier, ok)
	}
	sharpenRequirement := library.Cards["sharpen_blade"].Operations[0].Modifier.AddConditionalBonus.Requirements
	if !requirementsMet(sharpenRequirement, dice) {
		t.Fatal("matching-pair modifier did not qualify")
	}
	if !requirementsMet(sharpenRequirement, rolledFaces(library, "standard_d6", []int{3, 2, 2, 4, 2})) {
		t.Fatal("matching-pair modifier did not qualify on three-of-a-kind")
	}
	if requirementsMet(sharpenRequirement, rolledFaces(library, "standard_d6", []int{1, 2, 3, 4, 5})) {
		t.Fatal("matching-pair modifier qualified without matching faces")
	}
}

func TestExactPairAndPairOrBetterRemainIndependentlyAuthorable(t *testing.T) {
	library := settledTestLibrary(t)
	exactPair := content.RequirementGroup{All: []content.BattleRequirement{{Type: "number_pattern", Pattern: "exact_pair"}}}
	pairOrBetter := content.RequirementGroup{All: []content.BattleRequirement{{Type: "number_pattern", Pattern: "pair_or_better"}}}
	pair := rolledFaces(library, "standard_d6", []int{1, 1, 2, 3, 4})
	triple := rolledFaces(library, "standard_d6", []int{2, 2, 2, 3, 4})
	unique := rolledFaces(library, "standard_d6", []int{1, 2, 3, 4, 5})

	if !requirementsMet(exactPair, pair) || !requirementsMet(pairOrBetter, pair) {
		t.Fatal("a two-die match must satisfy both pair patterns")
	}
	if requirementsMet(exactPair, triple) {
		t.Fatal("three-of-a-kind must not satisfy exact_pair")
	}
	if !requirementsMet(pairOrBetter, triple) {
		t.Fatal("three-of-a-kind must satisfy pair_or_better")
	}
	if requirementsMet(exactPair, unique) || requirementsMet(pairOrBetter, unique) {
		t.Fatal("unique faces must not satisfy either pair pattern")
	}
	sharpenPattern := library.Cards["sharpen_blade"].Operations[0].Modifier.AddConditionalBonus.Requirements.All[0].Pattern
	if sharpenPattern != "pair_or_better" {
		t.Fatalf("Sharpen Blade pattern=%q want pair_or_better", sharpenPattern)
	}
}

func TestSharpenBladeStacksWithSwordCutThreeOfAKind(t *testing.T) {
	library := settledTestLibrary(t)
	battle := settledStatusBattle(t, library, "", 0)
	runtime := battle.Settled.Actors["player"]
	runtime.OffensiveAbilityIDs = []string{"sword_cut"}
	runtime.FinalDice = rolledFaces(library, "standard_d6", []int{3, 2, 2, 4, 2})
	runtime.SelectedAbilityID = "sword_cut"
	runtime.SelectedTierID = "four_swords"
	runtime.SelectedTargetIDs = []string{"enemy"}
	runtime.CardInstances = map[string]state.CardInstance{
		"sharpen-instance": {InstanceID: "sharpen-instance", DefinitionID: "sharpen_blade"},
	}
	runtime.AbilityModifiers = []state.RuntimeAbilityModifier{{
		SourceCardInstanceID: "sharpen-instance",
		AbilityID:            "sword_cut",
		BonusID:              "sharpened_pair_bleed",
	}}
	battle.Settled.Actors["player"] = runtime

	ops, ok := resolvedOffensiveOperations(&battle, library, "player")
	if !ok {
		t.Fatal("Sword Cut outcome did not resolve")
	}
	outcome := summarizeOffensiveOutcome(ops, runtime.SelectedTargetIDs)
	if outcome["base_damage"] != 6 {
		t.Fatalf("revealed damage=%#v want 6", outcome["base_damage"])
	}
	revealedApplications, ok := outcome["status_applications"].([]state.SettledStatusApplication)
	if !ok || len(revealedApplications) != 2 {
		t.Fatalf("revealed Bleed applications=%#v, want 2", outcome["status_applications"])
	}
	battle.Settled.Stage = stageOffensiveReact
	opened := (Engine{}).OpenResult(&battle, "player")
	if opened.Snapshot == nil || opened.Snapshot.Actors["player"].SelectedTier != "four_swords" || opened.Snapshot.Actors["player"].OffensiveOutcome["base_damage"] != 6 {
		t.Fatalf("reopened offensive outcome=%#v", opened.Snapshot)
	}

	if _, err := (Engine{}).finalizeOffensiveSources(&battle, library); err != nil {
		t.Fatal(err)
	}
	if len(battle.Settled.OffensiveSources) != 1 {
		t.Fatalf("offensive sources=%#v", battle.Settled.OffensiveSources)
	}
	applications := battle.Settled.OffensiveSources[0].StatusApplications
	if len(applications) != 2 {
		t.Fatalf("Bleed applications=%#v, want Sword Cut plus Sharpen Blade", applications)
	}
	for _, application := range applications {
		if application.TargetActorID != "enemy" || application.StatusID != "bleed" || application.Stacks != 1 {
			t.Fatalf("unexpected Bleed application=%#v", application)
		}
	}
}

func TestEntangleIsContentDrivenConsumedAtOffensiveEntryAndReducesRolls(t *testing.T) {
	library := settledTestLibrary(t)
	battle := settledStatusBattle(t, library, "entangle", 1)
	eng := NewEngine()
	if err := eng.applyOffensiveEntryTriggers(&battle, library); err != nil {
		t.Fatal(err)
	}
	if got := battle.Settled.Actors["player"].MaxRolls; got != 2 {
		t.Fatalf("max rolls=%d want 2", got)
	}
	if len(battle.Actors["player"].Statuses) != 0 {
		t.Fatal("Entangle was not consumed")
	}
	if battle.Settled.Window != nil {
		t.Fatal("Entangle opened a reaction window")
	}
}

func TestBlindConsumesWithoutAbilityAndCompletesReactableRollWithAbility(t *testing.T) {
	library := settledTestLibrary(t)
	t.Run("no ability", func(t *testing.T) {
		battle := settledStatusBattle(t, library, "blind", 1)
		eng := NewEngine()
		events, opened, err := eng.resolveBlindCheckpoint(&battle, library)
		if err != nil || opened || len(events) != 0 {
			t.Fatalf("result events=%v opened=%v err=%v", events, opened, err)
		}
		if len(battle.Actors["player"].Statuses) != 0 {
			t.Fatal("Blind remained after its checkpoint")
		}
	})
	t.Run("selected ability", func(t *testing.T) {
		battle := settledStatusBattle(t, library, "blind", 1)
		runtime := battle.Settled.Actors["player"]
		runtime.SelectedAbilityID = "sword_cut"
		battle.Settled.Actors["player"] = runtime
		battle.Settled.OffensiveSources = []state.SettledDamageSource{{ID: "source", SourceActorID: "player"}}
		script := &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "status_effect_dice", Bound: 6, Value: 0}}}
		eng, err := NewEngineWithConfig(Config{NamedRandom: script}, DefaultFlows()...)
		if err != nil {
			t.Fatal(err)
		}
		_, opened, err := eng.resolveBlindCheckpoint(&battle, library)
		if err != nil || !opened {
			t.Fatalf("opened=%v err=%v", opened, err)
		}
		if battle.Settled.Actors["player"].SelectedAbilityID == "" {
			t.Fatal("Blind committed before its reaction window closed")
		}
		if _, err := eng.handleBlindReactionCommand(&battle, library, command.Command{Type: command.TypePass}); err != nil {
			t.Fatal(err)
		}
		if battle.Settled.Actors["player"].SelectedAbilityID != "" || len(battle.Settled.OffensiveSources) != 0 {
			t.Fatal("Blind face 1 did not cancel the selected source")
		}
		if len(battle.Actors["player"].Statuses) != 0 {
			t.Fatal("Blind was not removed after its roll")
		}
		if err := script.AssertExhausted(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestAIDamageResponsePassesWhenEmergencyWardIsUnavailable(t *testing.T) {
	library := settledTestLibrary(t)
	battle := settledStatusBattle(t, library, "", 0)
	battle.Segment.Current = segment.DamageResolution
	batch := &state.SettledDamageBatch{
		Sources: []state.SettledDamageSource{{
			ID: "source-1", SourceActorID: "player", TargetActorID: "enemy", FinalAmount: 2,
		}},
		Removals: []state.ProposedCardRemoval{{
			ID: "removal-1", TargetActorID: "enemy", CardID: "enemy-card", Accepted: true,
		}},
	}
	script := &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{
		Stream: "ai_damage_response", Bound: 2, Value: 1,
	}}}
	eng, err := NewEngineWithConfig(Config{NamedRandom: script}, DefaultFlows()...)
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.autoAIDamageResponse(&battle, library, batch); err != nil {
		t.Fatalf("AI should pass without Emergency Ward: %v", err)
	}
	if batch.Sources[0].ReactionPrevention != 0 || !batch.Removals[0].Accepted {
		t.Fatalf("unavailable response changed damage: %#v", batch)
	}
	if err := script.AssertExhausted(); err != nil {
		t.Fatal(err)
	}
}

func TestAIDamageResponsePassesWhenEmergencyWardIsUnaffordable(t *testing.T) {
	library := settledTestLibrary(t)
	battle := settledStatusBattle(t, library, "", 0)
	battle.Segment.Current = segment.DamageResolution
	enemy := battle.Actors["enemy"]
	enemy.Cards.Hand = []string{"enemy-ward"}
	enemy.Resources.EnergyPoints = 0
	battle.Actors["enemy"] = enemy
	runtime := battle.Settled.Actors["enemy"]
	runtime.CardInstances = map[string]state.CardInstance{
		"enemy-ward": {InstanceID: "enemy-ward", DefinitionID: "emergency_ward"},
	}
	battle.Settled.Actors["enemy"] = runtime
	batch := &state.SettledDamageBatch{
		Sources: []state.SettledDamageSource{{
			ID: "source-1", SourceActorID: "player", TargetActorID: "enemy", FinalAmount: 2,
		}},
		Removals: []state.ProposedCardRemoval{{
			ID: "removal-1", TargetActorID: "enemy", CardID: "enemy-ward", Accepted: true,
		}},
	}
	script := &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{
		Stream: "ai_damage_response", Bound: 2, Value: 1,
	}}}
	eng, err := NewEngineWithConfig(Config{NamedRandom: script}, DefaultFlows()...)
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.autoAIDamageResponse(&battle, library, batch); err != nil {
		t.Fatalf("AI should pass without enough energy: %v", err)
	}
	if batch.Sources[0].ReactionPrevention != 0 || battle.Actors["enemy"].Resources.EnergyPoints != 0 || len(battle.Actors["enemy"].Cards.Hand) != 1 {
		t.Fatalf("unaffordable response changed state: battle=%#v batch=%#v", battle.Actors["enemy"], batch)
	}
	if err := script.AssertExhausted(); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileSettledDamageInitializesOverageAfterCheckpointReload(t *testing.T) {
	library := settledTestLibrary(t)
	battle := settledStatusBattle(t, library, "", 0)
	enemy := battle.Actors["enemy"]
	enemy.Health = state.HealthMetadata{Model: "card_zones", MaxHealth: 1}
	enemy.Cards.Hand = []string{"enemy-card"}
	battle.Actors["enemy"] = enemy
	batch := &state.SettledDamageBatch{
		Sources: []state.SettledDamageSource{{
			ID: "source-1", TargetActorID: "enemy", BaseAmount: 2, ReactionPrevention: 1,
		}},
		Removals: []state.ProposedCardRemoval{{
			ID: "removal-1", TargetActorID: "enemy", CardID: "enemy-card", Accepted: true,
		}},
		Overage: nil, // Empty maps are omitted from JSON and reload as nil.
	}
	reconcileSettledDamage(batch, &battle)
	if batch.Overage == nil || batch.Overage["enemy"] != 0 {
		t.Fatalf("overage was not restored: %#v", batch.Overage)
	}
	if batch.Sources[0].FinalAmount != 1 || !batch.Removals[0].Accepted {
		t.Fatalf("reconciled damage=%#v", batch)
	}
}

func TestCompletedHumanDefenseRollOpensReactionWhenAIDefenseDoesNotRoll(t *testing.T) {
	library := settledTestLibrary(t)
	battle := settledStatusBattle(t, library, "", 0)
	battle.Segment.Current = segment.Defensive
	battle.Settled.DefenseSelections = map[string]state.SettledDefense{
		"player": {ActorID: "player", AbilityID: "basic_defense", SourceID: "enemy-source", RolledFace: 4},
		"enemy":  {ActorID: "enemy", AbilityID: "protect", SourceID: "player-source"},
	}
	events, err := (Engine{}).resolveDefenseRollsAndOpenReaction(&battle, library)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("already resolved/non-rolling defenses emitted dice events: %#v", events)
	}
	if battle.Settled.Stage != stageDefenseReact || battle.Settled.Window.Stage != stageDefenseReact {
		t.Fatalf("completed human defense roll skipped reaction: stage=%q window=%#v", battle.Settled.Stage, battle.Settled.Window)
	}
}

func TestDefensiveRevealOpensWhenOnlyDefenseDoesNotRoll(t *testing.T) {
	library := settledTestLibrary(t)
	battle := settledStatusBattle(t, library, "", 0)
	battle.Segment.Current = segment.Defensive
	battle.Settled.OffensiveSources = []state.SettledDamageSource{{ID: "player-source", SourceActorID: "player", TargetActorID: "enemy", BaseAmount: 5}}
	battle.Settled.DefenseSelections = map[string]state.SettledDefense{
		"enemy": {ActorID: "enemy", AbilityID: "protect", SourceID: "player-source"},
	}

	events, err := (Engine{}).resolveDefenseRollsAndOpenReaction(&battle, library)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 || battle.Settled.Stage != stageDefenseReact || battle.Settled.Window == nil {
		t.Fatalf("non-rolling defense skipped reveal: events=%#v stage=%q window=%#v", events, battle.Settled.Stage, battle.Settled.Window)
	}
}

func TestAIDefenseFallsBackToAffordableAbility(t *testing.T) {
	library := settledTestLibrary(t)
	battle := settledStatusBattle(t, library, "", 0)
	battle.Segment.Current = segment.Defensive
	enemyRuntime := battle.Settled.Actors["enemy"]
	enemyRuntime.DefensiveAbilityIDs = []string{"basic_defense", "protect"}
	battle.Settled.Actors["enemy"] = enemyRuntime
	battle.Settled.OffensiveSources = []state.SettledDamageSource{{ID: "player-source", SourceActorID: "player", TargetActorID: "enemy", BaseAmount: 5}}
	script := &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "ai_defense", Bound: 2, Value: 1}}}
	engine := Engine{namedRandom: script}

	if err := engine.selectAIDefenses(&battle, library); err != nil {
		t.Fatal(err)
	}
	selection, ok := battle.Settled.DefenseSelections["enemy"]
	if !ok || selection.AbilityID != "basic_defense" || selection.SourceID != "player-source" {
		t.Fatalf("AI affordable defense = %#v, want Basic Defense against player-source", selection)
	}
	if err := script.AssertExhausted(); err != nil {
		t.Fatal(err)
	}
}

func settledTestLibrary(t *testing.T) content.BattleLibrary {
	t.Helper()
	library, err := content.LoadBattleLibrary(filepath.Join("..", "..", "..", "content", "battle_v1"))
	if err != nil {
		t.Fatal(err)
	}
	return library
}
func settledStatusBattle(t *testing.T, library content.BattleLibrary, statusID string, stacks int) state.Battle {
	t.Helper()
	catalog, err := json.Marshal(library)
	if err != nil {
		t.Fatal(err)
	}
	statuses := []state.StatusState(nil)
	if statusID != "" {
		statuses = []state.StatusState{{InstanceID: "status-1", DefinitionID: statusID, Stacks: stacks}}
	}
	battle, err := state.NewBattleFromSetup("status-test", state.BattleSetup{Actors: []state.ActorSetup{{ID: "player", ControllerType: state.ControllerHuman, Statuses: statuses}, {ID: "enemy", ControllerType: state.ControllerAI}}, SettledCatalog: catalog, SettledActors: map[string]state.SettledActorRuntime{"player": {MaxRolls: 3}, "enemy": {MaxRolls: 3}}})
	if err != nil {
		t.Fatal(err)
	}
	battle.Segment.Current = segment.Offensive
	battle.Flow = state.NewSegmentFlowState(battle.Segment)
	return battle
}
