package engine

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	battlerandom "diceanddestiny/server/internal/battle/random"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

func venomFixture(t *testing.T) (state.Battle, content.BattleLibrary) {
	t.Helper()
	lib, err := content.LoadBattleExtension(settledTestLibrary(t), filepath.Join("..", "..", "..", "content", "venom_v1"))
	if err != nil {
		t.Fatal(err)
	}
	b := settledStatusBattle(t, lib, "", 0)
	r := b.Settled.Actors["player"]
	r.OffensiveAbilityIDs = lib.Combatants["venom"].AbilityBoard.Offensive
	r.DefensiveAbilityIDs = lib.Combatants["venom"].AbilityBoard.Defensive
	r.UsedAbilities = map[string]int{}
	r.CardInstances = map[string]state.CardInstance{}
	b.Settled.Actors["player"] = r
	return b, lib
}
func finishVenomApplications(t *testing.T, e Engine, b *state.Battle, lib content.BattleLibrary) []event.Event {
	t.Helper()
	var events []event.Event
	if b.Settled.Venom.Active == nil {
		started, err := e.startVenomWork(b, lib, false)
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, started...)
	}
	for i := 0; i < 20 && b.Settled.Venom.Active != nil; i++ {
		if b.Settled.Stage != stageVenomStatus {
			t.Fatalf("expected application, got %s", b.Settled.Stage)
		}
		id := b.Settled.Window.RequiredActorID
		result, err := e.handleVenomStatus(b, lib, command.Command{ActorID: id, Type: command.TypePass})
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, result...)
	}
	return events
}
func TestVenomOverflowAndNoRefresh(t *testing.T) {
	for _, tc := range []struct{ p, add, v, marker, wantP, wantQueue int }{{2, 1, 0, 0, 3, 0}, {2, 2, 0, 0, 3, 1}, {3, 3, 0, 0, 3, 1}, {3, 1, 3, 0, 3, 1}, {3, 2, 0, 1, 3, 0}} {
		b, lib := venomFixture(t)
		if tc.p > 0 {
			applyStatus(&b, lib, "enemy", "poison", tc.p)
		}
		if tc.v > 0 {
			applyStatus(&b, lib, "enemy", "volatile_poison", tc.v)
		}
		if tc.marker > 0 {
			applyStatus(&b, lib, "enemy", "incubation", 1)
		}
		applyVenomStatus(&b, lib, "player", state.SettledStatusApplication{TargetActorID: "enemy", StatusID: "poison", Stacks: tc.add})
		if stacks(&b, "enemy", "poison") != tc.wantP || len(venomRuntime(&b).Queue) != tc.wantQueue {
			t.Fatalf("case %+v: %+v", tc, b.Settled.Venom)
		}
		if tc.wantQueue > 0 {
			finishVenomApplications(t, NewEngine(), &b, lib)
			if stacks(&b, "enemy", "incubation") != 1 {
				t.Fatal("missing overflow incubation")
			}
		}
	}
}
func TestIncubationMaturesSurvivorAndExpires(t *testing.T) {
	for _, tc := range []struct {
		name                string
		poison, volatile    int
		cleanse, replace    bool
		wantP, wantV, wantI int
	}{{"survivor", 2, 1, false, false, 1, 2, 0}, {"no survivor", 0, 1, false, false, 0, 1, 0}, {"full", 1, 3, false, false, 1, 3, 0}, {"cleansed", 1, 0, true, false, 1, 0, 0}, {"replacement", 1, 0, true, true, 1, 0, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			b, lib := venomFixture(t)
			if tc.poison > 0 {
				applyStatus(&b, lib, "enemy", "poison", tc.poison)
			}
			if tc.volatile > 0 {
				applyStatus(&b, lib, "enemy", "volatile_poison", tc.volatile)
			}
			applyStatus(&b, lib, "enemy", "incubation", 1)
			captureIncubation(&b)
			queueMaturation(&b)
			if tc.cleanse {
				removeStatus(&b, "enemy", "incubation", 0)
			}
			if tc.replace {
				applyStatus(&b, lib, "enemy", "incubation", 1)
			}
			events := finishVenomApplications(t, NewEngine(), &b, lib)
			if stacks(&b, "enemy", "poison") != tc.wantP || stacks(&b, "enemy", "volatile_poison") != tc.wantV || stacks(&b, "enemy", "incubation") != tc.wantI {
				t.Fatalf("unexpected statuses: %+v", b.Actors["enemy"].Statuses)
			}
			found := false
			for _, emitted := range events {
				data, ok := emitted.Data["poison_conversion"].(map[string]any)
				if !ok {
					continue
				}
				found = true
				if data["target_actor_id"] != "enemy" || data["poison_before"] != tc.poison || data["volatile_before"] != tc.volatile || data["poison_after"] != tc.wantP || data["volatile_after"] != tc.wantV || data["converted"] != (tc.wantV > tc.volatile) {
					t.Fatalf("incorrect conversion presentation result: %+v", data)
				}
			}
			if !found {
				t.Fatal("missing conversion outcome event")
			}
		})
	}
}
func TestCatalystAutomaticPriorityAndOneRetry(t *testing.T) {
	for _, tc := range []struct {
		name             string
		vf, pf, expected int
	}{{"volatile clear first", 6, 5, 0}, {"volatile five ignored", 5, 6, 1}, {"no clear", 5, 4, -1}} {
		t.Run(tc.name, func(t *testing.T) {
			b, lib := venomFixture(t)
			applyStatus(&b, lib, "enemy", "poison", 1)
			applyStatus(&b, lib, "enemy", "volatile_poison", 1)
			applyStatus(&b, lib, "player", "catalyst", 3)
			rolls := captureToxins(&b, lib, "enemy", []string{"volatile_poison", "poison"}, 2, false)
			rolls[0].Die.Face = tc.vf
			rolls[1].Die.Face = tc.pf
			b.Settled.TriggerBatch = &state.SettledTriggerBatch{ID: "test", Rolls: rolls}
			script := &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "status_effect_dice", Bound: 6, Value: 5}}}
			e := Engine{namedRandom: script}
			_, opened, err := e.automaticCatalyst(&b, lib)
			if err != nil {
				t.Fatal(err)
			}
			if opened != (tc.expected >= 0) {
				t.Fatal("wrong automatic trigger")
			}
			for i, r := range b.Settled.TriggerBatch.Rolls {
				if r.Rerolled != (i == tc.expected) {
					t.Fatalf("wrong die retried: %+v", r)
				}
			}
			want := 3
			if tc.expected >= 0 {
				want = 2
			}
			if stacks(&b, "player", "catalyst") != want {
				t.Fatal("wrong cost")
			}
			_, again, err := e.automaticCatalyst(&b, lib)
			if err != nil || again {
				t.Fatal("Catalyst retried twice")
			}
		})
	}
}
func TestCatalystResolvesCollectedRollsAfterCleanse(t *testing.T) {
	for _, cleansed := range []bool{false, true} {
		for _, clearingFace := range []int{5, 6} {
			for _, retryFace := range []int{2, 5, 6} {
				b, lib := venomFixture(t)
				b.Segment.Current = segment.OngoingEffects
				applyStatus(&b, lib, "enemy", "poison", 3)
				applyStatus(&b, lib, "player", "catalyst", 2)
				rolls := captureToxins(&b, lib, "enemy", []string{"poison", "poison", "poison"}, 3, false)
				for i, face := range []int{clearingFace, 1, 6} {
					rolls[i].Die.Face = face
					rolls[i].Resolved = true
				}
				b.Settled.TriggerBatch = &state.SettledTriggerBatch{ID: "antidote-rolls", Rolls: rolls, Reactable: true}
				actor := b.Actors["enemy"]
				actor.Controller = state.ControllerHuman
				actor.Resources.EnergyPoints = 1
				actor.Cards.Hand = []string{"antidote-card"}
				actor.Cards.Deck = []string{"health-1", "health-2", "health-3"}
				b.Actors["enemy"] = actor
				runtime := b.Settled.Actors["enemy"]
				runtime.CardInstances = map[string]state.CardInstance{"antidote-card": {InstanceID: "antidote-card", DefinitionID: "antidote"}}
				b.Settled.Actors["enemy"] = runtime
				e := Engine{namedRandom: &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "status_effect_dice", Bound: 6, Value: retryFace - 1}, {Stream: "damage_selection", Bound: 3, Value: 0}, {Stream: "damage_selection", Bound: 2, Value: 0}}}}
				openStatusEffectReveal(&b, true)
				if cleansed {
					removeStatus(&b, "enemy", "poison", 0)
					actor := b.Actors["enemy"]
					moveCard(&actor.Cards, "antidote-card", "hand", "discard")
					b.Actors["enemy"] = actor
				}
				// Close ordinary card responses through the real reaction handler.
				windowID := b.Settled.Window.ID
				for i := 0; i < 3 && b.Settled.Window.ID == windowID; i++ {
					if _, err := e.handleStatusReactionCommand(&b, lib, command.Command{ActorID: b.Settled.Window.RequiredActorID, Type: command.TypePass}); err != nil {
						t.Fatal(err)
					}
				}
				if stacks(&b, "player", "catalyst") != 1 || !rolls[0].Rerolled || rolls[0].Die.Face != retryFace || rolls[2].Rerolled || b.Settled.Stage != stageOngoingReact {
					t.Fatalf("cleansed=%v face=%d: Catalyst must retry exactly the first clearing die and reveal it: %+v", cleansed, clearingFace, rolls)
				}
				windowID = b.Settled.Window.ID
				for i := 0; i < 3 && b.Settled.Window.ID == windowID; i++ {
					if _, err := e.handleStatusReactionCommand(&b, lib, command.Command{ActorID: b.Settled.Window.RequiredActorID, Type: command.TypePass}); err != nil {
						t.Fatal(err)
					}
				}
				if b.Settled.Stage != stageOngoingDamage || stacks(&b, "player", "catalyst") != 1 {
					t.Fatal("retry must resolve without spending a second Catalyst")
				}
				damage := 0
				for _, source := range b.Settled.PendingDamage.Sources {
					damage += source.BaseAmount
				}
				wantStacks, wantDamage := 2, 2
				if retryFace >= 5 {
					wantStacks, wantDamage = 1, 1
				}
				if cleansed {
					wantStacks = 0
				}
				if damage != wantDamage || stacks(&b, "enemy", "poison") != wantStacks {
					t.Fatalf("wrong final results: damage=%d poison=%d", damage, stacks(&b, "enemy", "poison"))
				}
			}
		}
	}
}

func TestAgitateRevealsAllVolatileRollsDealsDamageAndResumesPlanning(t *testing.T) {
	b, lib := venomFixture(t)
	b.Settled.Stage = stageOffensivePlan
	for _, id := range []string{"player", "enemy"} {
		a := b.Actors[id]
		a.Controller = state.ControllerHuman
		a.Resources.EnergyPoints = 2
		a.Cards.Deck = []string{id + "-1", id + "-2", id + "-3", id + "-4", id + "-5", id + "-6"}
		if id == "player" {
			a.Cards.Hand = []string{"agitate-card"}
		}
		b.Actors[id] = a
	}
	r := b.Settled.Actors["player"]
	r.CardInstances["agitate-card"] = state.CardInstance{InstanceID: "agitate-card", DefinitionID: "agitate"}
	b.Settled.Actors["player"] = r
	applyStatus(&b, lib, "enemy", "volatile_poison", 2)
	openSettledWindow(&b, "planning", stageOffensivePlan, "planning", []command.Type{command.TypePlanningCards, command.TypePlanningRoll})
	planningWindow := b.Settled.Window.ID
	e := Engine{namedRandom: &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{
		{Stream: "status_effect_dice", Bound: 6, Value: 0},
		{Stream: "status_effect_dice", Bound: 6, Value: 0},
		{Stream: "damage_selection", Bound: 6, Value: 0},
		{Stream: "damage_selection", Bound: 5, Value: 0},
		{Stream: "damage_selection", Bound: 4, Value: 0},
		{Stream: "damage_selection", Bound: 3, Value: 0},
	}}}
	if err := e.playVenomCard(&b, lib, "player", "agitate-card", lib.Cards["agitate"], []string{"enemy"}, "volatile_poison"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.startVenomWork(&b, lib, false); err != nil {
		t.Fatal(err)
	}
	for _, viewer := range []string{"player", "enemy"} {
		view := snapshot.FromBattleForViewer(b, viewer)
		if view.Stage != stageOngoingReact || len(view.SettledEffectRolls) != 2 || view.SettledEffectRolls[0].Die.Face != 1 || view.SettledEffectRolls[0].SourceContentID != "volatile_poison" {
			t.Fatalf("%s cannot see Agitate's two Volatile rolls in Offensive: %+v", viewer, view.SettledEffectRolls)
		}
	}
	for i := 0; i < 3 && b.Settled.Stage == stageOngoingReact; i++ {
		if _, err := e.handleStatusReactionCommand(&b, lib, command.Command{ActorID: b.Settled.Window.RequiredActorID, Type: command.TypePass}); err != nil {
			t.Fatal(err)
		}
	}
	if b.Settled.Stage != stageOngoingDamage || b.Actors["enemy"].CurrentHealth() != 6 {
		t.Fatal("damage must await the damage response")
	}
	view := snapshot.FromBattleForViewer(b, "player")
	if view.SettledDamage == nil || len(view.SettledDamage.Sources) != 2 {
		t.Fatal("two Volatile rolls must reveal two damage sources")
	}
	for _, source := range view.SettledDamage.Sources {
		if source.FinalAmount != 2 {
			t.Fatal("each Volatile 1 must deal two pending damage")
		}
	}
	for i := 0; i < 3 && b.Settled.Stage == stageOngoingDamage; i++ {
		if _, err := e.handleDamageReactionCommand(&b, lib, command.Command{ActorID: b.Settled.Window.RequiredActorID, Type: command.TypePass}); err != nil {
			t.Fatal(err)
		}
	}
	if b.Actors["enemy"].CurrentHealth() != 2 || stacks(&b, "enemy", "volatile_poison") != 2 || b.Settled.Venom.Provoked["enemy"] != 0 {
		t.Fatal("Agitate must deal four damage, keep both stacks, and leave the Provoke allowance untouched")
	}
	if b.Settled.Stage != stageOffensivePlan || b.Settled.Window.ID != planningWindow || b.Settled.TriggerBatch != nil {
		t.Fatal("Agitate must resume the interrupted planning window")
	}
}

func TestAgitateCapturesAllOfOnlyChosenToxinOutsideProvokeLimit(t *testing.T) {
	for _, status := range []string{"poison", "volatile_poison"} {
		for count := 0; count <= 3; count++ {
			for used := 0; used <= 2; used++ {
				b, lib := venomFixture(t)
				b.Settled.Stage = stageOffensivePlan
				a := b.Actors["player"]
				a.Resources.EnergyPoints = 1
				a.Cards.Hand = []string{"agitate-card"}
				b.Actors["player"] = a
				other := "poison"
				if status == other {
					other = "volatile_poison"
				}
				applyStatus(&b, lib, "enemy", other, 3)
				if count > 0 {
					applyStatus(&b, lib, "enemy", status, count)
				}
				venomRuntime(&b).Provoked["enemy"] = used
				choices := venomCardChoices(&b, lib, "player", lib.Cards["agitate"])
				found := false
				for _, choice := range choices {
					if choice.Key != "poison" && choice.Key != "volatile_poison" {
						t.Fatal("Agitate offered a mixed or partial selection")
					}
					if choice.Key == status {
						found = true
					}
				}
				if found != (count > 0) {
					t.Fatalf("wrong eligibility for %s count=%d used=%d", status, count, used)
				}
				if count == 0 {
					continue
				}
				if err := NewEngine().playVenomCard(&b, lib, "player", "agitate-card", lib.Cards["agitate"], []string{"enemy"}, status); err != nil {
					t.Fatal(err)
				}
				work := b.Settled.Venom.Queue[0]
				if len(work.Rolls) != count || b.Settled.Venom.Provoked["enemy"] != used {
					t.Fatalf("Agitate was truncated or spent Provoke: %s count=%d used=%d", status, count, used)
				}
				for _, roll := range work.Rolls {
					if roll.SourceContentID != status || roll.ActorID != "enemy" {
						t.Fatal("Agitate captured the unchosen toxin")
					}
				}
			}
		}
	}
}

func TestProvokeSharedBudgetAndTerminalOrder(t *testing.T) {
	b, lib := venomFixture(t)
	applyStatus(&b, lib, "enemy", "poison", 3)
	applyStatus(&b, lib, "enemy", "volatile_poison", 2)
	if len(captureToxins(&b, lib, "enemy", []string{"poison"}, 1, true)) != 1 {
		t.Fatal("first check missing")
	}
	r := b.Settled.Actors["player"]
	r.SelectedAbilityID = "terminal_bite"
	r.SelectedToxins = []string{"volatile_poison", "poison"}
	r.FinalDice = rolledFaces(lib, "venom_d6", []int{6, 6, 6, 1, 2})
	b.Settled.Actors["player"] = r
	b.Settled.OffensiveSources = []state.SettledDamageSource{{ID: "bite", SourceActorID: "player", SourceContentID: "terminal_bite", TargetActorID: "enemy", BaseAmount: 4}}
	e := NewEngine()
	e.prepareVenomAttacks(&b, lib)
	if stacks(&b, "player", "catalyst") != 1 || len(b.Settled.Venom.AttackChecks["bite"]) != 1 || b.Settled.Venom.Provoked["enemy"] != 2 {
		t.Fatal("Terminal gain/budget incorrect")
	}
	if len(captureToxins(&b, lib, "enemy", []string{"poison"}, 1, true)) != 0 {
		t.Fatal("exceeded round budget")
	}
	e.prepareVenomAttacks(&b, lib)
	if stacks(&b, "player", "catalyst") != 1 {
		t.Fatal("gain repeated")
	}
}
func TestShedskinTwoCoilsAndBarbedSwap(t *testing.T) {
	b, lib := venomFixture(t)
	e := NewEngine()
	b.Settled.OffensiveSources = []state.SettledDamageSource{{ID: "hit", SourceActorID: "enemy", TargetActorID: "player", BaseAmount: 7}}
	ctx := effectContext{SourceActorID: "player", SourceContentID: "shedskin", SourceContentType: "ability", ProposalIDs: []string{"hit"}, RolledFace: 6}
	for i := 0; i < 2; i++ {
		result, err := e.executeResolvedEffects(&b, lib, ctx, lib.Abilities["shedskin"].Resolution.Operations)
		if err != nil {
			t.Fatal(err)
		}
		if err = e.applyEffectMutations(&b, lib, "", result); err != nil {
			t.Fatal(err)
		}
	}
	if len(b.Settled.Venom.Queue) != 1 {
		t.Fatal("two Coils must propose only one marker")
	}
	finishVenomApplications(t, e, &b, lib)
	if stacks(&b, "enemy", "incubation") != 1 {
		t.Fatal("Shedskin must seed marker without Poison")
	}
	ctx.SourceContentID = "barbed_mantle"
	ctx.RolledFace = 4
	result, err := e.executeResolvedEffects(&b, lib, ctx, lib.Abilities["barbed_mantle"].Resolution.Operations)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.applyEffectMutations(&b, lib, "", result); err != nil {
		t.Fatal(err)
	}
	if stacks(&b, "player", "catalyst") != 1 || b.Settled.OffensiveSources[0].Prevention != 3 {
		t.Fatal("Gland must prevent 3 + gain Catalyst")
	}
}
func TestVenomCostsAndCloneDoNotLeak(t *testing.T) {
	b, lib := venomFixture(t)
	applyStatus(&b, lib, "player", "catalyst", 2)
	applyStatus(&b, lib, "enemy", "poison", 1)
	a := b.Actors["player"]
	a.Resources.EnergyPoints = 3
	a.Cards.Hand = []string{"distill-card"}
	b.Actors["player"] = a
	r := b.Settled.Actors["player"]
	r.CardInstances["distill-card"] = state.CardInstance{InstanceID: "distill-card", DefinitionID: "distill"}
	b.Settled.Actors["player"] = r
	b.Settled.Stage = stageOffensivePlan
	e := NewEngine()
	if err := e.playVenomCard(&b, lib, "player", "distill-card", lib.Cards["distill"], []string{"enemy"}, "convert"); err != nil {
		t.Fatal(err)
	}
	if stacks(&b, "player", "catalyst") != 1 || b.Actors["player"].Resources.EnergyPoints != 2 {
		t.Fatal("cost not paid at acceptance")
	}
	before, _ := json.Marshal(b)
	clone := b.Clone()
	clone.Settled.Venom.Queue[0].TargetActorID = "player"
	clone.Settled.Venom.Used["mutated"] = true
	after, _ := json.Marshal(b)
	if string(before) != string(after) {
		t.Fatal("clone shares mutable Venom state")
	}
	removeStatus(&b, "enemy", "poison", 1)
	finishVenomApplications(t, e, &b, lib)
	if stacks(&b, "enemy", "volatile_poison") != 0 || stacks(&b, "player", "catalyst") != 1 {
		t.Fatal("failed conversion must not refund or mint toxin")
	}
}

func TestEveryVenomCardOffersOnlyExecutableChoices(t *testing.T) {
	_, library := venomFixture(t)
	for _, entry := range library.Combatants["venom"].Decklist {
		t.Run(entry.CardID, func(t *testing.T) {
			battle, lib := venomFixture(t)
			actor := battle.Actors["player"]
			actor.Resources.EnergyPoints = 10
			actor.Cards.Hand = []string{"card"}
			actor.Cards.Deck = []string{"draw-1", "draw-2", "draw-3"}
			battle.Actors["player"] = actor
			runtime := battle.Settled.Actors["player"]
			runtime.CardInstances["card"] = state.CardInstance{InstanceID: "card", DefinitionID: entry.CardID}
			runtime.SelectedAbilityID = "needlefang"
			runtime.SelectedTierID = "fang_3"
			runtime.SelectedTargetIDs = []string{"enemy"}
			runtime.RollsUsed = 1
			runtime.FinalDice = rolledFaces(lib, "venom_d6", []int{1, 1, 1, 4, 6})
			if entry.CardID == "terminal_formula" {
				runtime.SelectedAbilityID = "terminal_bite"
				runtime.FinalDice = rolledFaces(lib, "venom_d6", []int{1, 1, 6, 6, 6})
			}
			battle.Settled.Actors["player"] = runtime
			for _, id := range []string{"player", "enemy"} {
				applyStatus(&battle, lib, id, "poison", 2)
				applyStatus(&battle, lib, id, "volatile_poison", 1)
			}
			applyStatus(&battle, lib, "player", "incubation", 1)
			applyStatus(&battle, lib, "player", "catalyst", 2)
			battle.Settled.Stage = stageOffensivePlan
			switch entry.CardID {
			case "deep_puncture", "terminal_formula", "forked_tongue":
				battle.Settled.Stage = stageOffensiveReact
			case "spined_rebuttal":
				battle.Settled.Stage = stageDefenseReact
			case "coagulate", "emergency_molt", "antivenom_draught":
				battle.Settled.Stage = stageDamageReact
			}
			source := state.SettledDamageSource{ID: "incoming", SourceActorID: "enemy", SourceContentID: "sword_cut", TargetActorID: "player", BaseAmount: 5, FinalAmount: 5}
			battle.Settled.OffensiveSources = []state.SettledDamageSource{source}
			if battle.Settled.Stage == stageDamageReact {
				battle.Settled.PendingDamage = &state.SettledDamageBatch{ID: "damage", Sources: []state.SettledDamageSource{source}}
			}
			def := lib.Cards[entry.CardID]
			choices := venomCardChoices(&battle, lib, "player", def)
			if len(choices) == 0 {
				t.Fatal("reviewed card has no legal choice in its intended window")
			}
			for _, choice := range choices {
				branch := battle.Clone()
				engine := NewEngine()
				if err := engine.playVenomCard(&branch, lib, "player", "card", def, choice.Targets, choice.Key); err != nil {
					t.Fatalf("enumerated choice %+v failed: %v", choice, err)
				}
				if containsString(branch.Actors["player"].Cards.Hand, "card") || !containsString(branch.Actors["player"].Cards.Discard, "card") {
					t.Fatal("card did not move to discard")
				}
				if branch.Actors["player"].Resources.EnergyPoints < 0 {
					t.Fatal("overspent energy")
				}
				for _, id := range []string{"player", "enemy"} {
					for _, status := range branch.Actors[id].Statuses {
						if status.Stacks < 1 || status.Stacks > lib.Statuses[status.DefinitionID].Stacking.StackLimit {
							t.Fatal("status cap violated")
						}
					}
				}
			}
		})
	}
}

func TestVenomChildWindowPreservesPlanningPrivacy(t *testing.T) {
	b, lib := venomFixture(t)
	r := b.Settled.Actors["enemy"]
	r.FinalDice = rolledFaces(lib, "standard_d6", []int{1, 2, 3, 4, 5})
	r.SelectedAbilityID = "sword_cut"
	r.RollHistory = []state.RollBatch{{Number: 1, Dice: r.FinalDice}}
	b.Settled.Actors["enemy"] = r
	b.Settled.Stage = stageOffensivePlan
	v := venomRuntime(&b)
	v.Queue = []state.VenomWork{{Kind: "application", SourceActorID: "player", TargetActorID: "enemy", StatusID: "poison", Stacks: 1}}
	if _, err := NewEngine().startVenomWork(&b, lib, false); err != nil {
		t.Fatal(err)
	}
	view := snapshot.FromBattleForViewer(b, "player")
	enemy := view.Actors["enemy"]
	if enemy.Dice != nil || enemy.SelectedAbility != "" || len(enemy.RollHistory) > 0 || len(enemy.Hand) > 0 || len(enemy.CardInstances) > 0 {
		t.Fatal("child status response exposed private planning")
	}
}
func TestIncubationApplicationRechecksPoison(t *testing.T) {
	b, lib := venomFixture(t)
	applyStatus(&b, lib, "enemy", "poison", 1)
	v := venomRuntime(&b)
	v.Queue = []state.VenomWork{{Kind: "application", SourceActorID: "player", TargetActorID: "enemy", StatusID: "incubation", Stacks: 1, RequirePoison: true}}
	removeStatus(&b, "enemy", "poison", 0)
	e := NewEngine()
	if _, err := e.startVenomWork(&b, lib, false); err != nil {
		t.Fatal(err)
	}
	finishVenomApplications(t, e, &b, lib)
	if stacks(&b, "enemy", "incubation") != 0 {
		t.Fatal("normal Incubation applied after Poison was cleansed")
	}
}

func TestVenomRevalidationPreservesNeedleTierAndUpdatesFever(t *testing.T) {
	b, lib := venomFixture(t)
	e := NewEngine()
	r := b.Settled.Actors["player"]
	r.SelectedAbilityID, r.SelectedTierID = "needlefang", "fang_3"
	r.FinalDice = rolledFaces(lib, "venom_d6", []int{1, 1, 1, 1, 1})
	b.Settled.Actors["player"] = r
	if err := e.revalidateOffensiveSelection(&b, lib, "player"); err != nil {
		t.Fatal(err)
	}
	if b.Settled.Actors["player"].SelectedTierID != "fang_3" {
		t.Fatal("lower Needlefang tier was lost")
	}
	r = b.Settled.Actors["player"]
	r.SelectedAbilityID, r.SelectedTierID = "fever_spike", "greater"
	r.FinalDice = rolledFaces(lib, "venom_d6", []int{1, 1, 1, 4, 6})
	b.Settled.Actors["player"] = r
	if err := e.revalidateOffensiveSelection(&b, lib, "player"); err != nil {
		t.Fatal(err)
	}
	if b.Settled.Actors["player"].SelectedTierID == "greater" {
		t.Fatal("Fever retained a tier that no longer qualifies")
	}
}

func TestEmergencyMoltUsesFinalDamage(t *testing.T) {
	for _, zeroed := range []bool{false, true} {
		b, lib := venomFixture(t)
		v := venomRuntime(&b)
		v.MoltRewards = []state.VenomMoltReward{{ActorID: "player", SourceID: "hit", PriorPrevention: 1}}
		source := state.SettledDamageSource{ID: "hit", BaseAmount: 5, ReactionPrevention: 3}
		if zeroed {
			source.ScaleDenominator = 2
			source.ScaleNumerator = 0
		}
		batch := &state.SettledDamageBatch{Sources: []state.SettledDamageSource{source}}
		commitMoltRewards(&b, lib, batch)
		want := 1
		if zeroed {
			want = 0
		}
		if stacks(&b, "player", "catalyst") != want {
			t.Fatal("Molt reward did not reflect final damage")
		}
		commitMoltRewards(&b, lib, batch)
		if stacks(&b, "player", "catalyst") != want {
			t.Fatal("Molt reward repeated")
		}
	}
}

func TestIncubationAndConversionDoNotInterruptDefense(t *testing.T) {
	for _, kind := range []string{"application", "conversion"} {
		t.Run(kind, func(t *testing.T) {
			b, lib := venomFixture(t)
			b.Segment.Current = segment.Defensive
			openSettledWindow(&b, "defense", stageDefenseReact, "reaction", []command.Type{command.TypePass})
			window := b.Settled.Window
			applyStatus(&b, lib, "enemy", "poison", 1)
			venomRuntime(&b).Queue = []state.VenomWork{{Kind: kind, TargetActorID: "enemy", StatusID: "incubation", Stacks: 1}}
			events, err := NewEngine().startVenomWork(&b, lib, false)
			if err != nil {
				t.Fatal(err)
			}
			if b.Settled.Window != window || b.Settled.Stage != stageDefenseReact || b.Settled.Venom.Active != nil {
				t.Fatal("automatic status interrupted defense")
			}
			for _, ev := range events {
				if ev.Type == event.TypeInteractionWindowOpened {
					t.Fatal("unnecessary status reaction window")
				}
			}
			if kind == "application" && stacks(&b, "enemy", "incubation") != 1 {
				t.Fatal("Incubation not applied")
			}
			if kind == "conversion" && (stacks(&b, "enemy", "poison") != 0 || stacks(&b, "enemy", "volatile_poison") != 1) {
				t.Fatal("Poison not upgraded")
			}
		})
	}
}

func TestSpinedPoisonResponseKeepsPublicDefenseBoard(t *testing.T) {
	b, lib := venomFixture(t)
	b.Segment.Current = segment.Defensive
	openSettledWindow(&b, "defense", stageDefenseReact, "reaction", []command.Type{command.TypePass, command.TypeCommitInteraction})
	originalWindow := b.Settled.Window
	b.Settled.OffensiveSources = []state.SettledDamageSource{{ID: "hit", SourceActorID: "enemy", SourceContentID: "sword_cut", TargetActorID: "player", BaseAmount: 5}}
	b.Settled.DefenseSelections = map[string]state.SettledDefense{"player": {ActorID: "player", AbilityID: "shedskin", SourceID: "hit", RolledFace: 1, RolledFaces: []int{1, 2}}, "enemy": {ActorID: "enemy", AbilityID: "basic_defense", RolledFace: 3}}
	a := b.Actors["player"]
	a.Resources.EnergyPoints = 3
	a.Cards.Hand = []string{"spined"}
	b.Actors["player"] = a
	e := NewEngine()
	if err := e.playVenomCard(&b, lib, "player", "spined", lib.Cards["spined_rebuttal"], []string{"hit"}, "prevent"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.startVenomWork(&b, lib, false); err != nil {
		t.Fatal(err)
	}
	if b.Settled.Stage != stageVenomStatus {
		t.Fatal("Poison response contract was removed")
	}
	view := snapshot.FromBattleForViewer(b, "player")
	if view.PresentationStage != stageDefenseReact || len(view.SettledDefenses) != 2 {
		t.Fatal("public defense results lost in child response")
	}
	if b.Settled.OffensiveSources[0].ReactionPrevention != 1 {
		t.Fatal("Spined Rebuttal prevention lost")
	}
	events := finishVenomApplications(t, e, &b, lib)
	if b.Settled.Window != originalWindow || stacks(&b, "enemy", "poison") != 1 {
		t.Fatal("application failed to restore original defense decision")
	}
	found := false
	for _, ev := range events {
		if data, ok := ev.Data["status_application"].(map[string]any); ok {
			found = data["status_id"] == "poison" && data["before"] == 0 && data["after"] == 1
		}
	}
	if !found {
		t.Fatal("missing committed status animation counts")
	}
}
