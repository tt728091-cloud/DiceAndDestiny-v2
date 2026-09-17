package engine

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	battlerandom "diceanddestiny/server/internal/battle/random"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"encoding/json"
	"testing"
)

func TestShockDoseResolvesDuringPlanning(t *testing.T) {
	for _, mode := range []string{"damage", "prevention", "lethal"} {
		t.Run(mode, func(t *testing.T) {
			prevent := mode == "prevention"
			b, lib := venomFixture(t)
			b.Segment.Current = segment.Offensive
			for _, id := range []string{"player", "enemy"} {
				a := b.Actors[id]
				a.Controller = state.ControllerExternal
				a.Resources.EnergyPoints = 4
				a.Cards.Deck = []string{id + "1", id + "2", id + "3", id + "4", id + "5", id + "6"}
				if mode == "lethal" && id == "enemy" {
					a.Cards.Deck = a.Cards.Deck[:3]
				}
				a.Cards.Hand = nil
				a.Cards.Discard = nil
				b.Actors[id] = a
			}
			a := b.Actors["player"]
			a.Cards.Hand = []string{"dose"}
			b.Actors["player"] = a
			r := b.Settled.Actors["player"]
			r.CardInstances["dose"] = state.CardInstance{InstanceID: "dose", DefinitionID: "shock_dose"}
			b.Settled.Actors["player"] = r
			if prevent {
				a = b.Actors["enemy"]
				a.Cards.Hand = []string{"ward"}
				b.Actors["enemy"] = a
				r = b.Settled.Actors["enemy"]
				if r.CardInstances == nil {
					r.CardInstances = map[string]state.CardInstance{}
				}
				r.CardInstances["ward"] = state.CardInstance{InstanceID: "ward", DefinitionID: "emergency_ward"}
				b.Settled.Actors["enemy"] = r
			}
			applyStatus(&b, lib, "enemy", "volatile_poison", 2)
			openSettledWindow(&b, "planning", stageOffensivePlan, "planning", []command.Type{command.TypePlanningCards, command.TypePlanningRoll})
			window := b.Settled.Window.ID
			health := b.Actors["enemy"].CurrentHealth()
			bound := len(b.Actors["enemy"].Cards.Deck)
			e := Engine{namedRandom: &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "damage_selection", Bound: bound, Value: 0}, {Stream: "damage_selection", Bound: bound - 1, Value: 0}, {Stream: "damage_selection", Bound: bound - 2, Value: 0}}}}
			if err := e.playVenomCard(&b, lib, "player", "dose", lib.Cards["shock_dose"], []string{"enemy"}, "spend"); err != nil {
				t.Fatal(err)
			}
			if _, err := e.startVenomWork(&b, lib, false); err != nil {
				t.Fatal(err)
			}
			if b.Settled.Stage != stageOngoingDamage || len(b.Settled.OffensiveSources) != 0 || len(b.Settled.PendingDamage.Removals) != 3 {
				t.Fatal("Shock Dose must reveal immediate damage, not queue an offensive attack")
			}
			if stacks(&b, "enemy", "volatile_poison") != 1 || b.Actors["player"].Resources.EnergyPoints != 2 {
				t.Fatal("costs must be paid once")
			}
			b = b.Clone() // Pending child damage and planning continuation survive saves.
			if prevent {
				source := b.Settled.PendingDamage.Sources[0]
				payload, _ := json.Marshal(command.CommitInteractionPayload{Commitment: command.InteractionCommitmentData{CardIDs: []string{"ward"}, ProposalIDs: []string{source.ID}}})
				events, err := e.handleDamageReactionCommand(&b, lib, command.Command{ActorID: "enemy", Type: command.TypeCommitInteraction, Payload: payload})
				if err != nil {
					t.Fatal(err)
				}
				if len(events) != 1 || events[0].Type != event.TypeDamageModified || events[0].Data["card_definition_id"] != "emergency_ward" || events[0].Data["damage_before"] != 3 || events[0].Data["damage_after"] != 0 {
					t.Fatalf("missing visible prevention outcome: %+v", events)
				}
			}
			for i := 0; i < 4 && b.Settled.Stage == stageOngoingDamage && !state.IsTerminalBattleStatus(b.Status); i++ {
				if _, err := e.handleDamageReactionCommand(&b, lib, command.Command{ActorID: b.Settled.Window.RequiredActorID, Type: command.TypePass}); err != nil {
					t.Fatal(err)
				}
			}
			want := health - 3
			if prevent {
				want = health
			}
			if b.Actors["enemy"].CurrentHealth() != want {
				t.Fatalf("health=%d want %d", b.Actors["enemy"].CurrentHealth(), want)
			}
			if mode == "lethal" {
				if !state.IsTerminalBattleStatus(b.Status) {
					t.Fatal("lethal Shock Dose must end the battle immediately")
				}
				return
			}
			if b.Settled.Stage != stageOffensivePlan || b.Settled.Window.ID != window || b.Settled.PendingDamage != nil || b.Settled.Venom.Active != nil {
				t.Fatal("did not restore exact planning continuation")
			}
		})
	}
}
