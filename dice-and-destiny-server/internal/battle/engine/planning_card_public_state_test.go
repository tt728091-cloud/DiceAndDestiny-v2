package engine

import (
	"encoding/json"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
)

func TestPublicPlanningCardPublishesResourcesWithoutHiddenChoices(t *testing.T) {
	b, lib := venomFixture(t)
	a := b.Actors["enemy"]
	a.Controller = state.ControllerExternal
	a.Resources.EnergyPoints = 3
	a.Cards.Hand = []string{"focus", "hidden-hand"}
	a.Cards.Deck = []string{"hidden-draw"}
	a.Cards.Discard = nil
	b.Actors["enemy"] = a
	r := b.Settled.Actors["enemy"]
	r.CardInstances = map[string]state.CardInstance{
		"focus":       {InstanceID: "focus", DefinitionID: "battle_focus"},
		"hidden-hand": {InstanceID: "hidden-hand", DefinitionID: "tip_it"},
		"hidden-draw": {InstanceID: "hidden-draw", DefinitionID: "loaded_die"},
	}
	r.FinalDice = rolledFaces(lib, "standard_d6", []int{1, 1, 1, 4, 4})
	r.SelectedAbilityID = "sword_cut"
	b.Settled.Actors["enemy"] = r
	b.Settled.PlanningPublic = map[string]state.SettledPlanningPublicState{
		"enemy": {EnergyPoints: 3, HandCount: 2, DeckCount: 1},
	}
	b.Segment.Current = segment.Offensive
	openSettledWindow(&b, "planning", stageOffensivePlan, "planning", []command.Type{command.TypePlanningCards, command.TypePlanningRoll})
	payload, _ := json.Marshal(command.PlanningCardsPayload{CardIDs: []string{"focus"}, TargetIDs: []string{"enemy"}})
	events, err := NewEngine().handleOffensivePlanningCommand(&b, lib, command.Command{Type: command.TypePlanningCards, ActorID: "enemy", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Data["card_definition_id"] != "battle_focus" {
		t.Fatalf("missing public play: %#v", events)
	}
	owner := snapshot.FromBattleForViewer(b, "enemy").Actors["enemy"]
	other := snapshot.FromBattleForViewer(b, "player").Actors["enemy"]
	if owner.EnergyPoints != 4 || other.EnergyPoints != 4 || other.HandCount != 2 || other.DeckCount != 0 || other.DiscardCount != 1 {
		t.Fatalf("Battle Focus public rewards stale: owner=%+v other=%+v", owner, other)
	}
	if len(other.Hand) != 0 || len(other.CardInstances) != 0 || other.Dice != nil || other.SelectedAbility != "" {
		t.Fatalf("private planning information leaked: %+v", other)
	}
}
