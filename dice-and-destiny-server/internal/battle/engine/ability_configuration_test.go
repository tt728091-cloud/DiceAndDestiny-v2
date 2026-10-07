package engine

import (
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
	"encoding/json"
	"reflect"
	"testing"
)

func TestSharedDrawAndStatusOperationsAcrossActivationSources(t *testing.T) {
	var expected state.ActorState
	for i, kind := range []string{"card", "ability", "status"} {
		b, lib := curseFixture(t)
		before := b.Actors["player"].CurrentHealth()
		e := NewEngine()
		ops := []content.BattleOperation{{Type: "draw_cards", Target: "self", Amount: 2}, {Type: "gain_resource", Target: "self", Resource: "energy", Amount: 3}, {Type: "apply_status", Target: "self", StatusID: "catalyst", StackCount: 2}}
		r, err := e.executeEffects(&b, lib, effectContext{SourceActorID: "player", SourceContentID: "configured_effect", SourceContentType: kind}, ops)
		if err != nil {
			t.Fatal(err)
		}
		if err = e.applyEffectMutations(&b, lib, "", r); err != nil {
			t.Fatal(err)
		}
		if len(b.Actors["player"].Cards.Hand) != 2 || stacks(&b, "player", "catalyst") != 2 || b.Actors["player"].CurrentHealth() != before {
			t.Fatalf("%s: incorrect shared effect", kind)
		}
		if i == 0 {
			expected = b.Actors["player"]
		} else if !reflect.DeepEqual(expected, b.Actors["player"]) {
			t.Fatalf("%s diverged from card behavior", kind)
		}
	}
}
func TestConfiguredCurseHooksUseParametersAndSurviveChoiceSave(t *testing.T) {
	b, lib := curseFixture(t)
	e := NewEngine()
	a := lib.Abilities["grasp_of_the_sarcophagus"]
	a.ID = "custom_binding"
	a.Name = "Custom Binding"
	a.Hooks[0].Operations[0].Special.Amount = 3
	a.Hooks[0].Operations[0].Special.Limit = 1
	lib.Abilities[a.ID] = a
	if err := e.runAbilityHooks(&b, lib, a, "before_defense", "player", "enemy", "", "base", nil, false); err != nil {
		t.Fatal(err)
	}
	if _, err := e.startCurseWork(&b, lib, false); err != nil {
		t.Fatal(err)
	}
	if markedCount(&b, "enemy") != 3 || b.Settled.Curse.Active == nil {
		t.Fatal("configured curse count/choice missing")
	}
	raw, _ := json.Marshal(b)
	var restored state.Battle
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	w := restored.Settled.Curse.Active
	if !w.Configured || w.Limit != 1 || w.Amount != 3 {
		t.Fatal("choice parameters lost on save")
	}
	if err := e.resolveCurseChoice(&restored, lib, w.Options[0]); err != nil {
		t.Fatal(err)
	}
	if len(entombedIndices(&restored, "enemy")) != 1 {
		t.Fatal("choice did not entomb")
	}
	restored.Settled.Curse.Active = nil
	restored.Settled.Curse.Queue = nil
	if err := e.runAbilityHooks(&restored, lib, a, "before_defense", "player", "enemy", "", "base", nil, false); err != nil {
		t.Fatal(err)
	}
	if _, err := e.startCurseWork(&restored, lib, false); err != nil {
		t.Fatal(err)
	}
	if restored.Settled.Curse.Active != nil {
		t.Fatal("configured entomb limit ignored")
	}
}
func TestConfiguredDefenseHookRunsOnceForMultipleMatchingDice(t *testing.T) {
	b, lib := curseFixture(t)
	e := NewEngine()
	a := lib.Abilities["misfortune_repaid"]
	a.ID = "custom_repaid"
	a.Hooks[0].Operations[0].StackCount = 4
	lib.Abilities[a.ID] = a
	b.Settled.OffensiveSources = []state.SettledDamageSource{{ID: "incoming", SourceActorID: "enemy", TargetActorID: "player", BaseAmount: 6}}
	d := state.SettledDefense{ActorID: "player", AbilityID: a.ID, SourceID: "incoming", RolledFaces: []int{6, 6}}
	if err := e.curseDefenseCompleted(&b, lib, d, false); err != nil {
		t.Fatal(err)
	}
	if stacks(&b, "enemy", "curse_count") != 4 || len(b.Settled.Curse.Queue) != 0 {
		t.Fatal("once-per-roll condition or actual prevention gate incorrect")
	}
}
func TestConfiguredTerminalAndSelectableTierDoNotUseAbilityNames(t *testing.T) {
	b, lib := venomFixture(t)
	e := NewEngine()
	a := lib.Abilities["terminal_bite"]
	a.ID = "custom_toxin_attack"
	a.Hooks[0].Operations[0].StackCount = 2
	lib.Abilities[a.ID] = a
	applyStatus(&b, lib, "enemy", "poison", 3)
	rt := b.Settled.Actors["player"]
	rt.SelectedAbilityID = a.ID
	rt.SelectedTargetIDs = []string{"enemy"}
	rt.FinalDice = rolledFaces(lib, "venom_d6", []int{6, 6, 6, 1, 2})
	b.Settled.Actors["player"] = rt
	b.Settled.OffensiveSources = []state.SettledDamageSource{{ID: "custom", SourceActorID: "player", SourceContentID: a.ID, TargetActorID: "enemy", BaseAmount: 4}}
	if err := e.prepareVenomAttacks(&b, lib); err != nil {
		t.Fatal(err)
	}
	if stacks(&b, "player", "catalyst") != 2 || len(b.Settled.Venom.AttackChecks["custom"]) != 2 {
		t.Fatal("renamed provoke or configured reward ignored")
	}
	a = lib.Abilities["needlefang"]
	a.ID = "custom_fangs"
	lib.Abilities[a.ID] = a
	rt.SelectedAbilityID = a.ID
	rt.SelectedTierID = "fang_3"
	rt.FinalDice = rolledFaces(lib, "venom_d6", []int{1, 1, 1, 1, 1})
	b.Settled.Actors["player"] = rt
	ops, ok := resolvedOffensiveOperations(&b, lib, "player")
	if !ok || len(ops) == 0 {
		t.Fatal("renamed tiers unavailable")
	}
	n, _ := operationAmount(ops[0], 0)
	if n != 4 {
		t.Fatalf("selected lower tier lost: %d", n)
	}
}

func TestConfiguredOffensiveEnergyReservationSurvivesReloadAndReselection(t *testing.T) {
	b, lib := venomFixture(t)
	a := lib.Abilities["needlefang"]
	a.ID = "costly_fangs"
	a.Cost.Energy = 3
	lib.Abilities[a.ID] = a
	actor := b.Actors["player"]
	actor.Resources.EnergyPoints = 5
	b.Actors["player"] = actor
	rt := b.Settled.Actors["player"]
	rt.SelectedAbilityID = a.ID
	b.Settled.Actors["player"] = rt
	if !reserveOffensiveCost(&b, lib, "player") || b.Actors["player"].Resources.EnergyPoints != 2 {
		t.Fatal("cost not paid")
	}
	raw, _ := json.Marshal(b)
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatal(err)
	}
	if !reserveOffensiveCost(&b, lib, "player") || b.Actors["player"].Resources.EnergyPoints != 2 {
		t.Fatal("repeat charged after reload")
	}
	a.Cost.Energy = 6
	lib.Abilities[a.ID] = a
	if reserveOffensiveCost(&b, lib, "player") || b.Actors["player"].Resources.EnergyPoints != 2 {
		t.Fatal("unaffordable replan altered energy")
	}
	rt = b.Settled.Actors["player"]
	rt.SelectedAbilityID = ""
	b.Settled.Actors["player"] = rt
	if !reserveOffensiveCost(&b, lib, "player") || b.Actors["player"].Resources.EnergyPoints != 5 {
		t.Fatal("cancellation did not refund")
	}
}

func TestSharedConditionalStatusUsesConfiguredBranchForEveryActivationSource(t *testing.T) {
	for _, kind := range []string{"card", "ability", "status"} {
		for _, condition := range []bool{false, true} {
			b, lib := venomFixture(t)
			e := NewEngine()
			if condition {
				applyStatus(&b, lib, "enemy", "poison", 2)
			}
			effect := &content.SharedSpecialEffect{Kind: "conditional_status", StatusID: "poison", Threshold: 2, ResultStatusID: "blind", Stacks: 1, Limit: 1, FallbackStatusID: "catalyst", FallbackStacks: 2}
			ctx := effectContext{SourceActorID: "player", SourceContentID: "custom_branch", SourceContentType: kind, TargetActorIDs: []string{"enemy"}}
			r, err := e.executeEffects(&b, lib, ctx, []content.BattleOperation{{Type: "special_effect", Target: "selected_targets", Special: effect}})
			if err != nil {
				t.Fatal(err)
			}
			if err = e.applyEffectMutations(&b, lib, "", r); err != nil {
				t.Fatal(err)
			}
			if condition && stacks(&b, "enemy", "blind") != 1 {
				t.Fatalf("%s positive branch ignored", kind)
			}
			if !condition && stacks(&b, "enemy", "catalyst") != 2 {
				t.Fatalf("%s fallback ignored", kind)
			}
		}
	}
}

func TestConfiguredConditionalStatusThroughCardCommand(t *testing.T) {
	for _, condition := range []bool{false, true} {
		step := selfStep("conditional_status", map[string]any{
			"status_id": "poison", "threshold": 2,
			"result_status_id": "protect", "stacks": 1, "limit": 2,
			"fallback_status_id": "poison", "fallback_stacks": 1,
		})
		b, lib := programFixture(t, step)
		if condition {
			applyStatus(&b, lib, "player", "poison", 2)
		}
		energy := b.Actors["player"].Resources.EnergyPoints
		health := b.Actors["player"].CurrentHealth()
		runProgramAction(t, &b, lib, "start")
		if condition && (stacks(&b, "player", "protect") != 1 || stacks(&b, "player", "poison") != 2) {
			t.Fatal("card command did not use the shared positive branch")
		}
		if !condition && (stacks(&b, "player", "protect") != 0 || stacks(&b, "player", "poison") != 1) {
			t.Fatal("card command did not use the shared fallback branch")
		}
		if b.Settled.Actors["player"].CardExecution != nil || b.Actors["player"].Resources.EnergyPoints != energy-2 || b.Actors["player"].CurrentHealth() != health {
			t.Fatal("shared status effect broke card completion, cost or health")
		}
	}
}
