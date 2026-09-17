package engine

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
	"encoding/json"
	"testing"
)

func TestAntidotePublicOutcomeInPlanningAndDefense(t *testing.T) {
	for _, stage := range []string{stageOffensivePlan, stageDefenseReact} {
		t.Run(stage, func(t *testing.T) {
			b, lib := venomFixture(t)
			for _, id := range []string{"player", "enemy"} {
				a := b.Actors[id]
				a.Controller = state.ControllerExternal
				a.Resources.EnergyPoints = 2
				b.Actors[id] = a
			}
			a := b.Actors["enemy"]
			a.Cards.Hand = []string{"antidote"}
			b.Actors["enemy"] = a
			r := b.Settled.Actors["enemy"]
			if r.CardInstances == nil {
				r.CardInstances = map[string]state.CardInstance{}
			}
			r.CardInstances["antidote"] = state.CardInstance{InstanceID: "antidote", DefinitionID: "antidote"}
			b.Settled.Actors["enemy"] = r
			applyStatus(&b, lib, "enemy", "poison", 3)
			applyStatus(&b, lib, "enemy", "bleed", 1)
			b.Segment.Current = segment.Defensive
			openSettledWindowForActors(&b, "defense", stageDefenseReact, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePass}, []string{"player", "enemy"}, false)
			moveSettledWindowToActor(&b, "player")
			e := NewEngine()
			if !snapshot.FromBattleForViewer(b, "player").PassHandsOffPriority || snapshot.FromBattleForViewer(b, "enemy").PassHandsOffPriority {
				t.Fatal("handoff hint must apply only to current priority holder")
			}
			if _, err := e.handleDefenseReactionCommand(&b, lib, command.Command{Type: command.TypePass, ActorID: "player"}); err != nil {
				t.Fatal(err)
			}
			if snapshot.FromBattleForViewer(b, "enemy").PassHandsOffPriority {
				t.Fatal("last pass closes responses, not a handoff")
			}
			var data map[string]any
			if stage == stageDefenseReact {
				payload, _ := json.Marshal(command.CommitInteractionPayload{Commitment: command.InteractionCommitmentData{CardIDs: []string{"antidote"}, ChoiceID: "poison"}})
				events, err := e.handleDefenseReactionCommand(&b, lib, command.Command{Type: command.TypeCommitInteraction, ActorID: "enemy", Payload: payload})
				if err != nil {
					t.Fatal(err)
				}
				data = events[0].Data
				if !snapshot.FromBattleForViewer(b, "player").PassHandsOffPriority {
					t.Fatal("response resets priority; no-choice player hands back automatically")
				}
			} else {
				b.Segment.Current = segment.Offensive
				openSettledWindow(&b, "planning", stageOffensivePlan, "planning", []command.Type{command.TypePlanningCards, command.TypePlanningRoll})
				payload, _ := json.Marshal(command.PlanningCardsPayload{CardIDs: []string{"antidote"}, StatusID: "poison"})
				events, err := e.handleOffensivePlanningCommand(&b, lib, command.Command{Type: command.TypePlanningCards, ActorID: "enemy", Payload: payload})
				if err != nil {
					t.Fatal(err)
				}
				data = events[0].Data
				if snapshot.FromBattleForViewer(b, "player").PassHandsOffPriority {
					t.Fatal("planning is a real choice, never a response handoff")
				}
			}
			if data["operation"] != "remove_status" || data["choice_id"] != "poison" || data["stacks_before"] != 3 || data["stacks_after"] != 0 || data["stacks_removed"] != 3 || data["card_definition_id"] != "antidote" {
				t.Fatalf("missing cleanse outcome: %+v", data)
			}
			if stacks(&b, "enemy", "bleed") != 1 || stacks(&b, "enemy", "poison") != 0 {
				t.Fatal("cleanse changed unrelated status")
			}
		})
	}
}
