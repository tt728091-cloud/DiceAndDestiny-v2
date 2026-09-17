package engine

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"encoding/json"
	"testing"
)

func TestOffensiveDiceEditReturnsAffectedPlan(t *testing.T) {
	for _, caseID := range []struct {
		used int
		card string
	}{{2, "tip_it"}, {3, "tip_it"}, {2, "forked_tongue"}, {3, "forked_tongue"}} {
		used := caseID.used
		b, lib := venomFixture(t)
		b.Segment.Current = segment.Offensive
		for _, id := range []string{"player", "enemy"} {
			a := b.Actors[id]
			a.Controller = state.ControllerExternal
			a.Resources.EnergyPoints = 3
			b.Actors[id] = a
			r := b.Settled.Actors[id]
			r.PlanningCommitted = true
			b.Settled.Actors[id] = r
		}
		r := b.Settled.Actors["player"]
		r.RollsUsed = used
		r.MaxRolls = 3
		r.FinalDice = rolledFaces(lib, "venom_d6", []int{6, 3, 1, 1, 1})
		r.SelectedAbilityID = "needlefang"
		r.SelectedTierID = "fang_4"
		r.SelectedTargetIDs = []string{"enemy"}
		r.KeptIndices = []int{0}
		b.Settled.Actors["player"] = r
		a := b.Actors["enemy"]
		a.Cards.Hand = []string{"tip"}
		b.Actors["enemy"] = a
		r = b.Settled.Actors["enemy"]
		if r.CardInstances == nil {
			r.CardInstances = map[string]state.CardInstance{}
		}
		r.CardInstances["tip"] = state.CardInstance{InstanceID: "tip", DefinitionID: caseID.card}
		b.Settled.Actors["enemy"] = r
		openSettledWindow(&b, "reaction", stageOffensiveReact, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePass})
		commitment := command.InteractionCommitmentData{CardIDs: []string{"tip"}, PlanningAdjustments: []command.PlanningAdjustment{{ActorID: "player", DieIndex: 0, Face: 5}}}
		if caseID.card == "forked_tongue" {
			commitment.PlanningAdjustments = nil
			commitment.ChoiceID = "player:1:4"
			commitment.ProposalIDs = []string{"player"}
		}
		payload, _ := json.Marshal(command.CommitInteractionPayload{Commitment: commitment})
		e := NewEngine()
		if _, err := e.handleOffensiveReactionCommand(&b, lib, command.Command{ActorID: "enemy", Type: command.TypeCommitInteraction, Payload: payload}); err != nil {
			t.Fatal(err)
		}
		b = b.Clone()
		if b.Settled.Stage != stageOffensivePlan || !b.Settled.ReactionReplanning || b.Settled.Window.RequiredActorID != "player" {
			t.Fatal("affected player must choose their plan")
		}
		if state.SettledPlanningPrivate(b) || !b.Settled.Actors["enemy"].PlanningCommitted || b.Settled.Actors["player"].RollsUsed != used || len(b.Settled.Actors["player"].KeptIndices) != 1 {
			t.Fatal("replanning lost revealed/retained state")
		}
		actions := settledLegalActions(&b, lib, "player", b.Flow.PendingInput["player"])
		hasChoice, hasReroll := false, false
		for _, action := range actions {
			if action.Type == command.TypePlanningAbility {
				hasChoice = true
			}
			if action.Type == command.TypePlanningReroll {
				hasReroll = true
			}
			if action.Type == command.TypePlanningCards {
				t.Fatal("cannot replay planning cards after reveal")
			}
		}
		if !hasChoice || hasReroll != (used < 3) {
			t.Fatal("reselection and remaining rolls must be available correctly")
		}
		choose, _ := json.Marshal(command.PlanningAbilityPayload{AbilityID: "needlefang", TierID: "fang_3", TargetIDs: []string{"enemy"}})
		if _, err := e.handleOffensivePlanningCommand(&b, lib, command.Command{ActorID: "player", Type: command.TypePlanningAbility, Payload: choose}); err != nil {
			t.Fatal(err)
		}
		if b.Settled.Stage != stageOffensiveReact || b.Settled.ReactionReplanning || b.Settled.Actors["player"].SelectedTierID != "fang_3" {
			t.Fatal("owner's replacement selection was not retained")
		}
	}
}
