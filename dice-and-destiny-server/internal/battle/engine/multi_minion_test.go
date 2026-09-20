package engine

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
	"encoding/json"
	"testing"
)

func pairFixture(t *testing.T) (state.Battle, content.BattleLibrary) {
	b, lib := venomFixture(t)
	clone := b.Clone()
	b.Actors["enemy2"] = clone.Actors["enemy"]
	b.Settled.Actors["enemy2"] = clone.Settled.Actors["enemy"]
	for id, a := range b.Actors {
		a.Controller = state.ControllerExternal
		a.TeamID = "minions"
		if id == "player" {
			a.TeamID = "hero"
		}
		b.Actors[id] = a
	}
	return b, lib
}
func TestTeamCompletionAndTargetValidation(t *testing.T) {
	for _, tc := range []struct {
		dead   []string
		want   state.BattleStatus
		winner string
	}{
		{nil, state.BattleActive, ""},
		{[]string{"enemy"}, state.BattleActive, ""},
		{[]string{"enemy", "enemy2"}, state.BattleVictory, "player"},
		{[]string{"player"}, state.BattleVictory, "enemy"},
		{[]string{"player", "enemy", "enemy2"}, state.BattleDraw, ""},
	} {
		b, lib := pairFixture(t)
		for _, id := range tc.dead {
			a := b.Actors[id]
			a.DefeatState = state.ActorPendingDefeat
			b.Actors[id] = a
		}
		_, err := evaluateBattleCompletion(&b)
		if err != nil {
			t.Fatal(err)
		}
		if b.Status != tc.want || b.WinnerActorID != tc.winner {
			t.Fatalf("dead %v: %s winner %s", tc.dead, b.Status, b.WinnerActorID)
		}
		if err := validateActorTargeting(&b, "enemy", lib.Abilities["needlefang"].Targeting, []string{"enemy2"}); err == nil {
			t.Fatal("friendly fire accepted")
		}
		if len(tc.dead) > 0 && tc.dead[0] == "enemy" {
			if err := validateActorTargeting(&b, "player", lib.Abilities["needlefang"].Targeting, []string{"enemy"}); err == nil {
				t.Fatal("defeated target accepted")
			}
		}
	}
}
func TestSeparateDefensesPreservePreventionAndCounterTarget(t *testing.T) {
	b, lib := pairFixture(t)
	e := NewEngine()
	b.Segment.Current = segment.Defensive
	b.Settled.OffensiveSources = []state.SettledDamageSource{{ID: "first", SourceActorID: "enemy", SourceContentID: "slash", TargetActorID: "player", BaseAmount: 6}, {ID: "second", SourceActorID: "enemy2", SourceContentID: "slash", TargetActorID: "player", BaseAmount: 4}}
	b.Settled.DefenseHistory = map[string]state.SettledDefense{}
	b.Settled.DefenseSelections = map[string]state.SettledDefense{"player": {ActorID: "player", AbilityID: "shedskin", SourceID: "second", RolledFace: 1, RolledFaces: []int{1, 1}}}
	r := b.Settled.Actors["player"]
	r.UsedAbilities["shedskin"] = 1
	b.Settled.Actors["player"] = r
	if _, err := e.finalizeDefenses(&b, lib); err != nil {
		t.Fatal(err)
	}
	if b.Settled.Stage != stageDefenseSelect || b.Settled.OffensiveSources[0].Prevention != 0 || b.Settled.OffensiveSources[1].Prevention != 2 {
		t.Fatalf("wrong isolated defense: %+v", b.Settled)
	}
	if defenseUses(&b, "player", "shedskin") != 0 {
		t.Fatal("second use unavailable")
	}
	payload, _ := json.Marshal(command.PlanningAbilityPayload{AbilityID: "shedskin", TargetIDs: []string{"second"}})
	if _, err := e.handleDefenseSelectionCommand(&b, lib, command.Command{ActorID: "player", Type: command.TypePlanningAbility, Payload: payload}); err == nil {
		t.Fatal("duplicate defense accepted")
	}
	b = b.Clone()
	b.Settled.DefenseSelections = map[string]state.SettledDefense{"player": {ActorID: "player", AbilityID: "shedskin", SourceID: "first", RolledFace: 1, RolledFaces: []int{1, 1}}}
	if _, err := e.finalizeDefenses(&b, lib); err != nil {
		t.Fatal(err)
	}
	if b.Settled.OffensiveSources[0].Prevention != 2 || b.Settled.OffensiveSources[1].Prevention != 2 {
		t.Fatal("first defense reapplied or second lost")
	}
	batch, err := e.buildDamageBatch(&b, b.Settled.OffensiveSources)
	if err != nil {
		t.Fatal(err)
	}
	if damageForTarget(batch, "player") != 6 {
		t.Fatalf("combined damage: %+v", batch)
	}
	targets, err := effectTargets(&b, effectContext{SourceActorID: "player", ProposalIDs: []string{"second"}}, "enemy")
	if err != nil || len(targets) != 1 || targets[0] != "enemy2" {
		t.Fatal("defensive effects hit wrong attacker")
	}
}
func TestVenomCardsSelectEachEnemy(t *testing.T) {
	for _, id := range []string{"pinprick", "twin_puncture", "slow_release", "incubate", "distill", "accelerant", "agitate", "fever_cycle", "extract", "shock_dose"} {
		b, lib := pairFixture(t)
		e := NewEngine()
		b.Settled.Stage = stageOffensivePlan
		a := b.Actors["player"]
		a.Resources.EnergyPoints = 10
		a.Cards.Hand = []string{"card"}
		b.Actors["player"] = a
		applyStatus(&b, lib, "player", "catalyst", 1)
		for _, enemy := range []string{"enemy", "enemy2"} {
			applyStatus(&b, lib, enemy, "poison", 2)
			applyStatus(&b, lib, enemy, "volatile_poison", 1)
		}
		var choice *venomCardChoice
		choices := venomCardChoices(&b, lib, "player", lib.Cards[id])
		for _, c := range choices {
			if len(c.Targets) == 1 && c.Targets[0] == "enemy2" {
				copy := c
				choice = &copy
				break
			}
		}
		if choice == nil {
			t.Fatalf("%s cannot target second enemy", id)
		}
		if err := e.playVenomCard(&b, lib, "player", "card", lib.Cards[id], choice.Targets, choice.Key); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if stacks(&b, "enemy", "poison") != 2 || stacks(&b, "enemy", "volatile_poison") != 1 {
			t.Fatalf("%s spent status from wrong enemy", id)
		}
		for _, work := range venomRuntime(&b).Queue {
			if work.TargetActorID != "" && work.TargetActorID != "enemy2" {
				t.Fatalf("%s queued wrong target: %+v", id, work)
			}
		}
	}
}

func TestDefensePlansReserveCostsAndPassAllPreservesChosenDefenses(t *testing.T) {
	b, lib := pairFixture(t)
	e := NewEngine()
	b.Segment.Current = segment.Defensive
	b.Settled.OffensiveSources = []state.SettledDamageSource{{ID: "first", SourceActorID: "enemy", SourceContentID: "needlefang", TargetActorID: "player", BaseAmount: 6}, {ID: "second", SourceActorID: "enemy2", SourceContentID: "needlefang", TargetActorID: "player", BaseAmount: 4}}
	a := b.Actors["player"]
	a.Resources.EnergyPoints = 1
	b.Actors["player"] = a
	if _, err := e.progressSettledDefensive(&b, lib); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(command.PlanningAbilityPayload{AbilityID: "barbed_mantle", TargetIDs: []string{"second"}})
	if _, err := e.handleDefenseSelectionCommand(&b, lib, command.Command{ActorID: "player", Type: command.TypePlanningAbility, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if b.Settled.Stage != stageDefenseSelect || len(b.Settled.DefenseSelections) != 0 || len(b.Settled.DefensePlans) != 1 {
		t.Fatal("first choice must wait for remaining defenses before rolling")
	}
	copy := b.Clone()
	delete(copy.Settled.DefensePlans, "second")
	if len(b.Settled.DefensePlans) != 1 {
		t.Fatal("cloned plans alias original")
	}
	payload, _ = json.Marshal(command.PlanningAbilityPayload{AbilityID: "barbed_mantle", TargetIDs: []string{"first"}})
	if _, err := e.handleDefenseSelectionCommand(&b, lib, command.Command{ActorID: "player", Type: command.TypePlanningAbility, Payload: payload}); err == nil {
		t.Fatal("paid defense without energy")
	}
	payload, _ = json.Marshal(command.PlanningPassPayload{})
	if _, err := e.handleDefenseSelectionCommand(&b, lib, command.Command{ActorID: "player", Type: command.TypePlanningPass, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	selection := b.Settled.DefenseSelections["player"]
	if selection.SourceID != "second" || selection.AbilityID != "barbed_mantle" {
		t.Fatal("pass all discarded queued defense")
	}
	selection.RolledFace = 1
	selection.RolledFaces = []int{1}
	b.Settled.DefenseSelections["player"] = selection
	if _, err := e.finalizeDefenses(&b, lib); err != nil {
		t.Fatal(err)
	}
	if stacks(&b, "enemy2", "poison") != 1 || stacks(&b, "enemy", "poison") != 0 {
		t.Fatal("counter hit wrong attacker")
	}
	if b.Settled.DefenseHistory["first"].AbilityID != "" || b.Settled.DefenseHistory["second"].AbilityID != "barbed_mantle" {
		t.Fatal("skip overwrote another defense")
	}
	if b.Settled.OffensiveSources[0].Prevention != 0 || b.Settled.OffensiveSources[1].Prevention != 2 {
		t.Fatal("skip changed damage prevention")
	}
}

func TestCoagulateChoosesPoisonCostIndependentlyOfIncomingSource(t *testing.T) {
	b, lib := pairFixture(t)
	e := NewEngine()
	b.Settled.Stage = stageDamageReact
	a := b.Actors["player"]
	a.Resources.EnergyPoints = 1
	a.Cards.Hand = []string{"coagulate-card"}
	b.Actors["player"] = a
	applyStatus(&b, lib, "enemy2", "poison", 1)
	b.Settled.PendingDamage = &state.SettledDamageBatch{Sources: []state.SettledDamageSource{{ID: "incoming", SourceActorID: "enemy", TargetActorID: "player", BaseAmount: 5}}}
	var chosen *venomCardChoice
	for _, c := range venomCardChoices(&b, lib, "player", lib.Cards["coagulate"]) {
		if c.Key == "prevent|enemy2" {
			copy := c
			chosen = &copy
		}
	}
	if chosen == nil {
		t.Fatal("cannot spend other enemy Poison")
	}
	if err := e.playVenomCard(&b, lib, "player", "coagulate-card", lib.Cards["coagulate"], chosen.Targets, chosen.Key); err != nil {
		t.Fatal(err)
	}
	if stacks(&b, "enemy2", "poison") != 0 || b.Settled.PendingDamage.Sources[0].ReactionPrevention != 3 {
		t.Fatalf("cost/prevention mismatch: %+v", b.Settled.PendingDamage.Sources)
	}
}

func TestDefeatedEnemyRemovedFromSuspendedPlanning(t *testing.T) {
	b, lib := pairFixture(t)
	e := NewEngine()
	b.Segment.Current = segment.Offensive
	b.Settled.Stage = ""
	if _, err := e.progressSettledOffensive(&b, lib); err != nil {
		t.Fatal(err)
	}
	saved := &state.VenomResume{Stage: b.Settled.Stage, Window: b.Settled.Window, Flow: b.Flow}
	a := b.Actors["enemy2"]
	a.DefeatState = state.ActorDefeated
	b.Actors["enemy2"] = a
	venomRuntime(&b).Resume = saved
	if _, err := e.nextVenomWork(&b, lib); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.Flow.PendingInput["enemy2"]; ok {
		t.Fatal("restored dead enemy input")
	}
	if containsString(b.Settled.Window.RequiredActorIDs, "enemy2") || containsString(b.Settled.Window.PriorityActorIDs, "enemy2") {
		t.Fatal("dead enemy blocks continuation")
	}
	if _, ok := b.Flow.PendingInput["player"]; !ok {
		t.Fatal("lost player's suspended input")
	}
}

func TestAllDefenseChoicesPrecedeRolls(t *testing.T) {
	b, lib := pairFixture(t)
	e := NewEngine()
	b.Segment.Current = segment.Defensive
	b.Settled.OffensiveSources = []state.SettledDamageSource{
		{ID: "first", SourceActorID: "enemy", SourceContentID: "needlefang", TargetActorID: "player", BaseAmount: 6},
		{ID: "second", SourceActorID: "enemy2", SourceContentID: "needlefang", TargetActorID: "player", BaseAmount: 4},
	}
	if _, err := e.progressSettledDefensive(&b, lib); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"second", "first"} {
		payload, _ := json.Marshal(command.PlanningAbilityPayload{AbilityID: "shedskin", TargetIDs: []string{source}})
		if _, err := e.handleDefenseSelectionCommand(&b, lib, command.Command{ActorID: "player", Type: command.TypePlanningAbility, Payload: payload}); err != nil {
			t.Fatal(err)
		}
		if source == "second" {
			if b.Settled.Stage != stageDefenseSelect {
				t.Fatal("rolled before all choices")
			}
			if _, err := e.handleDefenseSelectionCommand(&b, lib, command.Command{ActorID: "player", Type: command.TypePlanningAbility, Payload: payload}); err == nil {
				t.Fatal("duplicate queued defense accepted")
			}
		}
	}
	if b.Settled.Stage != stageDefenseRoll || b.Settled.DefenseSelections["player"].SourceID != "first" {
		t.Fatal("first wave not ready")
	}
	selection := b.Settled.DefenseSelections["player"]
	selection.RolledFace = 1
	selection.RolledFaces = []int{1, 1}
	b.Settled.DefenseSelections["player"] = selection
	if _, err := e.finalizeDefenses(&b, lib); err != nil {
		t.Fatal(err)
	}
	if b.Settled.Stage != stageDefenseRoll || b.Settled.DefenseSelections["player"].SourceID != "second" || len(b.Settled.DefensePlans) != 0 {
		t.Fatal("queued second defense did not start automatically")
	}
	if b.Settled.OffensiveSources[0].Prevention != 2 || b.Settled.OffensiveSources[1].Prevention != 0 {
		t.Fatal("defenses mixed sources")
	}
}

func TestQueuedDefensesRollTogetherAndResolveSeparately(t *testing.T) {
	b, lib := pairFixture(t)
	b.Random = state.RandomState{Mode: state.RandomModeReproducible, Algorithm: state.RandomAlgorithmSHA256, Seed: 43}
	e := NewEngine()
	b.Segment.Current = segment.Defensive
	b.Settled.OffensiveSources = []state.SettledDamageSource{
		{ID: "first", SourceActorID: "enemy", TargetActorID: "player", BaseAmount: 6},
		{ID: "second", SourceActorID: "enemy2", TargetActorID: "player", BaseAmount: 6},
	}
	if _, err := e.progressSettledDefensive(&b, lib); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"first", "second"} {
		payload, _ := json.Marshal(command.PlanningAbilityPayload{AbilityID: "shedskin", TargetIDs: []string{source}})
		if _, err := e.handleDefenseSelectionCommand(&b, lib, command.Command{ActorID: "player", Type: command.TypePlanningAbility, Payload: payload}); err != nil {
			t.Fatal(err)
		}
	}
	before := b.Clone()
	events, err := e.handleDefenseRollCommand(&b, lib, command.Command{ActorID: "player", Type: command.TypeRollDice})
	if err != nil {
		t.Fatal(err)
	}
	queued := b.Settled.DefensePlans["second"]
	if len(events) != 2 || len(queued.RolledFaces) != 2 || queued.RolledFace == 0 || len(b.Settled.DefenseSelections["player"].RolledFaces) != 2 {
		t.Fatal("both independent two-die defenses must roll at the first checkpoint")
	}
	if b.Settled.Stage != stageDefenseReact || b.Settled.OffensiveSources[0].Prevention != 0 || b.Settled.OffensiveSources[1].Prevention != 0 {
		t.Fatal("rolling must not prematurely commit either defense")
	}
	// Replay consumes the same deterministic draws, including the queued set.
	if _, err := e.handleDefenseRollCommand(&before, lib, command.Command{ActorID: "player", Type: command.TypeRollDice}); err != nil {
		t.Fatal(err)
	}
	want, _ := json.Marshal(queued)
	replayed, _ := json.Marshal(before.Settled.DefensePlans["second"])
	if string(want) != string(replayed) {
		t.Fatal("queued dice are not deterministic")
	}
	// Clone models save/resume: later effect waves reuse the already rolled dice.
	b = b.Clone()
	if _, err := e.finalizeDefenses(&b, lib); err != nil {
		t.Fatal(err)
	}
	actual, _ := json.Marshal(b.Settled.DefenseSelections["player"])
	if b.Settled.Stage != stageDefenseReact || string(actual) != string(want) || b.Settled.OffensiveSources[1].Prevention != 0 {
		t.Fatal("second defense must reuse its dice and open effects/reactions without another roll checkpoint")
	}
}
