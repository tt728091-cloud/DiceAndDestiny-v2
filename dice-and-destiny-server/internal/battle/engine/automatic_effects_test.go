package engine

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type effectsSequence struct{ faces []int }

func (r *effectsSequence) IntnNamed(stream string, bound int) (int, error) {
	if stream != "status_effect_dice" {
		return 0, nil
	}
	if len(r.faces) == 0 {
		return 0, fmt.Errorf("unexpected extra Effects roll")
	}
	n := r.faces[0]
	r.faces = r.faces[1:]
	return n - 1, nil
}

func TestAutomaticEffectsBleedCatalystDamageAndConversion(t *testing.T) {
	b, lib := venomFixture(t)
	for _, id := range []string{"player", "enemy"} {
		actor := b.Actors[id]
		actor.Controller = state.ControllerHuman
		if id == "enemy" {
			actor.Controller = state.ControllerAI
		}
		actor.Cards.Deck = nil
		actor.Cards.Hand = nil
		actor.Cards.Discard = nil
		runtime := b.Settled.Actors[id]
		runtime.CardInstances = map[string]state.CardInstance{}
		for i := 0; i < 20; i++ {
			card := fmt.Sprintf("%s-card-%d", id, i)
			actor.Cards.Deck = append(actor.Cards.Deck, card)
			runtime.CardInstances[card] = state.CardInstance{InstanceID: card, DefinitionID: "tip_it"}
		}
		b.Actors[id] = actor
		b.Settled.Actors[id] = runtime
	}
	applyStatus(&b, lib, "player", "bleed", 3)
	applyStatus(&b, lib, "player", "catalyst", 2)
	applyStatus(&b, lib, "enemy", "poison", 3)
	applyStatus(&b, lib, "enemy", "volatile_poison", 1)
	applyStatus(&b, lib, "enemy", "incubation", 1)
	b.Segment.Current = segment.OngoingEffects
	b.Settled.Stage = ""
	b.Settled.Initialized = true
	b.Flow = state.NewSegmentFlowState(b.Segment)
	random := &effectsSequence{faces: []int{5, 1, 6, 6, 2}}
	engine := Engine{namedRandom: random}
	events, err := engine.progressAutomaticEffects(&b, lib)
	if err != nil {
		t.Fatal(err)
	}
	if b.Segment.Current != segment.Income || b.Settled.Window != nil {
		t.Fatalf("Effects interrupted: %s %#v", b.Segment.Current, b.Settled.Window)
	}
	if len(random.faces) != 0 {
		t.Fatal("missing automatic roll")
	}
	if stacks(&b, "player", "bleed") != 2 || stacks(&b, "player", "catalyst") != 1 {
		t.Fatal("wrong Bleed/Catalyst closeout")
	}
	if stacks(&b, "enemy", "poison") != 0 || stacks(&b, "enemy", "volatile_poison") != 2 || stacks(&b, "enemy", "incubation") != 0 {
		t.Fatalf("wrong toxin closeout: %+v", b.Actors["enemy"].Statuses)
	}
	for _, id := range []string{"player", "enemy"} {
		if len(b.Actors[id].Cards.Removed) != 3 {
			t.Fatalf("%s damage=%v", id, b.Actors[id].Cards.Removed)
		}
	}
	var summary event.Event
	for _, ev := range events {
		if ev.Type == event.Type("effects_resolved") {
			summary = ev
		}
	}
	if summary.Data == nil {
		t.Fatal("missing animation summary")
	}
	raw, _ := json.Marshal(summary)
	var data struct {
		Data struct {
			Steps []event.Event `json:"steps"`
		} `json:"data"`
	}
	json.Unmarshal(raw, &data)
	forced := 0
	for _, step := range data.Data.Steps {
		if step.Data["source_type"] == "catalyst" {
			forced++
			if step.Data["status_id"] != "volatile_poison" || step.Data["face_before"] != float64(6) {
				t.Fatal("Catalyst priority changed")
			}
		}
	}
	if forced != 1 {
		t.Fatalf("Catalyst rerolls=%d", forced)
	}
	if path := os.Getenv("DICE_AND_DESTINY_EFFECTS_FIXTURE"); path != "" {
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// A forged old response cannot reintroduce a pause or mutate a status.
	b.Segment.Current = segment.OngoingEffects
	if len(engine.LegalActions(&b, "player")) != 0 {
		t.Fatal("Effects exposes participant choices")
	}
	if _, err := engine.handleSettledCommand(&b, command.Command{ActorID: "player", Type: command.TypeCommitInteraction}); err == nil {
		t.Fatal("Effects accepted manual response")
	}
	for _, card := range lib.Cards {
		if cardPlayableDuring(card, &b, "reaction") || cardPlayableDuring(card, &b, "planning") {
			t.Fatalf("card %s remains playable during Effects", card.ID)
		}
	}
}

func TestCatalystSetupCardsReplaceToxinEdits(t *testing.T) {
	for _, tc := range []struct {
		id         string
		gain, cost int
	}{{"bitter_reagent", 1, 1}, {"measured_dose", 2, 2}} {
		t.Run(tc.id, func(t *testing.T) {
			b, lib := venomFixture(t)
			if lib.Cards[tc.id].Cost.Energy != tc.cost {
				t.Fatal("wrong setup card cost")
			}
			b.Segment.Current = segment.Offensive
			b.Settled.Stage = stageOffensivePlan
			a := b.Actors["player"]
			a.Resources.EnergyPoints = 5
			b.Actors["player"] = a
			choices := venomCardChoices(&b, lib, "player", lib.Cards[tc.id])
			if len(choices) != 1 {
				t.Fatalf("setup choices=%v", choices)
			}
			if choices[0].Key != "gain" {
				t.Fatalf("still offers dice edits: %+v", choices)
			}
			a = b.Actors["player"]
			a.Cards.Hand = []string{"setup"}
			beforeEnergy := a.Resources.EnergyPoints
			b.Actors["player"] = a
			if err := (Engine{}).playVenomCard(&b, lib, "player", "setup", lib.Cards[tc.id], choices[0].Targets, choices[0].Key); err != nil {
				t.Fatal(err)
			}
			if stacks(&b, "player", "catalyst") != tc.gain || b.Actors["player"].Resources.EnergyPoints != beforeEnergy-tc.cost {
				t.Fatal("wrong Catalyst grant or payment")
			}
			applyStatus(&b, lib, "player", "catalyst", 3)
			if len(venomCardChoices(&b, lib, "player", lib.Cards[tc.id])) != 0 {
				t.Fatal("setup card playable at Catalyst cap")
			}
		})
	}
}
