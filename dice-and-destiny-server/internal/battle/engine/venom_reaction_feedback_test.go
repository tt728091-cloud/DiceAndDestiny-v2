package engine

import (
	"encoding/json"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
)

func TestAntidoteResponseToPinprickPublishesCauseBeforeApplication(t *testing.T) {
	b, lib := venomFixture(t)
	b.Segment.Current = segment.Offensive
	a := b.Actors["enemy"]
	a.Controller = state.ControllerExternal
	a.Resources.EnergyPoints = 4
	a.Cards.Hand = []string{"antidote", "hidden"}
	a.Cards.Discard = nil
	b.Actors["enemy"] = a
	r := b.Settled.Actors["enemy"]
	r.CardInstances = map[string]state.CardInstance{"antidote": {InstanceID: "antidote", DefinitionID: "antidote"}, "hidden": {InstanceID: "hidden", DefinitionID: "tip_it"}}
	b.Settled.Actors["enemy"] = r
	b.Settled.PlanningPublic = map[string]state.SettledPlanningPublicState{"enemy": {EnergyPoints: 4, HandCount: 2}}
	applyStatus(&b, lib, "enemy", "poison", 2)
	openSettledWindow(&b, "planning", stageOffensivePlan, "planning", []command.Type{command.TypePlanningCards, command.TypePlanningRoll})
	venomRuntime(&b).Queue = append(venomRuntime(&b).Queue, state.VenomWork{Kind: "application", SourceActorID: "player", SourceContentID: "pinprick", TargetActorID: "enemy", StatusID: "poison", Stacks: 1})
	e := NewEngine()
	if _, err := e.startVenomWork(&b, lib, false); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(command.CommitInteractionPayload{Commitment: command.InteractionCommitmentData{CardIDs: []string{"antidote"}, ChoiceID: "poison"}})
	events, err := e.handleVenomStatus(&b, lib, command.Command{ActorID: "enemy", Type: command.TypeCommitInteraction, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != event.TypeCardPlayed {
		t.Fatalf("reaction lacks public card cause: %+v", events)
	}
	data := events[0].Data
	if data["card_definition_id"] != "antidote" || data["operation"] != "remove_status" || data["stacks_before"] != 2 || data["stacks_after"] != 0 || data["stacks_removed"] != 2 {
		t.Fatalf("incorrect cleanse feedback: %+v", data)
	}
	if stacks(&b, "enemy", "poison") != 0 || b.Settled.Stage != stageVenomStatus {
		t.Fatal("Pinprick applied before its reaction finished")
	}
	resolved := finishVenomApplications(t, e, &b, lib)
	if stacks(&b, "enemy", "poison") != 1 {
		t.Fatal("Pinprick should add one after Antidote")
	}
	found := false
	for _, ev := range resolved {
		if application, ok := ev.Data["status_application"].(map[string]any); ok {
			found = application["before"] == 0 && application["after"] == 1
		}
	}
	if !found {
		t.Fatal("missing separate Pinprick application outcome")
	}
	view := snapshot.FromBattleForViewer(b, "player").Actors["enemy"]
	if view.EnergyPoints != 3 || view.HandCount != 1 || view.DiscardCount != 1 {
		t.Fatalf("reaction public costs went stale after returning to planning: %+v", view)
	}
	if len(view.Hand) != 0 || len(view.CardInstances) != 0 {
		t.Fatal("hidden hand leaked")
	}
}
