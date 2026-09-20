package engine

import (
	"path/filepath"
	"testing"

	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

func TestMinionAttackTiersAndRoundBonus(t *testing.T) {
	b, lib := venomFixture(t)
	var err error
	lib, err = content.LoadBattleExtension(lib, filepath.Join("..", "..", "..", "content", "minions_v1"))
	if err != nil {
		t.Fatal(err)
	}
	actor := b.Actors["enemy"]
	actor.Controller = state.ControllerExternal
	actor.Resources.EnergyPoints = 1
	actor.Cards.Hand = []string{"surge"}
	actor.Cards.Discard = nil
	b.Actors["enemy"] = actor
	runtime := b.Settled.Actors["enemy"]
	runtime.OffensiveAbilityIDs = []string{"brine_lash"}
	runtime.CardInstances["surge"] = state.CardInstance{InstanceID: "surge", DefinitionID: "brine_surge"}
	runtime.SelectedAbilityID = "brine_lash"
	runtime.SelectedTargetIDs = []string{"player"}
	b.Settled.Actors["enemy"] = runtime
	b.Settled.Stage = stageOffensivePlan
	b.Settled.Window = &state.SettledWindow{Purpose: "planning"}
	e := NewEngine()
	if err := e.playSettledCard(&b, lib, "enemy", "surge", nil, "brine_lash", 0, ""); err != nil {
		t.Fatal(err)
	}
	if b.Actors["enemy"].Resources.EnergyPoints != 0 || len(b.Actors["enemy"].Cards.Hand) != 0 || len(b.Actors["enemy"].Cards.Discard) != 1 {
		t.Fatal("surge cost or card destination incorrect")
	}
	// A cloned/restored state retains a temporary modifier and its expiry.
	b = b.Clone()
	for n := 0; n <= 5; n++ {
		faces := []int{1, 1, 1, 1, 1}
		for i := 0; i < n; i++ {
			faces[i] = 3
		}
		runtime = b.Settled.Actors["enemy"]
		runtime.FinalDice = rolledFaces(lib, "brine_d6", faces)
		b.Settled.Actors["enemy"] = runtime
		ops, ok := resolvedOffensiveOperations(&b, lib, "enemy")
		if n == 0 {
			if ok || len(ops) > 0 {
				t.Fatal("zero-target miss must not deal bonus damage")
			}
			continue
		}
		if !ok {
			t.Fatalf("%d targets not qualified", n)
		}
		damage := 0
		for _, op := range ops {
			if op.Type == "deal_damage" {
				amount, _ := operationAmount(op, 0)
				damage += amount
			}
		}
		if damage != 2*n+1 {
			t.Fatalf("%d targets dealt %d", n, damage)
		}
	}
	// The same retained dice next round cannot reuse the old bonus.
	b.Segment.Round++
	ops, _ := resolvedOffensiveOperations(&b, lib, "enemy")
	damage := 0
	for _, op := range ops {
		if op.Type == "deal_damage" {
			amount, _ := operationAmount(op, 0)
			damage += amount
		}
	}
	if damage != 10 {
		t.Fatalf("round bonus persisted: %d", damage)
	}
}

func TestMinionLastTargetChangedAfterCommit(t *testing.T) {
	b, lib := venomFixture(t)
	var err error
	lib, err = content.LoadBattleExtension(lib, filepath.Join("..", "..", "..", "content", "minions_v1"))
	if err != nil {
		t.Fatal(err)
	}
	runtime := b.Settled.Actors["enemy"]
	runtime.OffensiveAbilityIDs = []string{"brine_lash"}
	runtime.FinalDice = rolledFaces(lib, "brine_d6", []int{2, 1, 1, 1, 1})
	runtime.SelectedAbilityID = "brine_lash"
	runtime.SelectedTierID = "brine_1"
	runtime.RollsUsed = 3
	runtime.MaxRolls = 3
	b.Settled.Actors["enemy"] = runtime
	if err := NewEngine().revalidateOffensiveSelection(&b, lib, "enemy"); err != nil {
		t.Fatal(err)
	}
	runtime = b.Settled.Actors["enemy"]
	if runtime.SelectedAbilityID != "" || runtime.RollsUsed != 3 {
		t.Fatal("removing final target must cancel attack without refunding rolls")
	}
}

func TestSaltVeilRollPrevention(t *testing.T) {
	b, lib := venomFixture(t)
	var err error
	lib, err = content.LoadBattleExtension(lib, filepath.Join("..", "..", "..", "content", "minions_v1"))
	if err != nil {
		t.Fatal(err)
	}
	ability := lib.Abilities["salt_veil"]
	roll := defenseRollOperation(ability)
	if roll == nil || roll.DiceCount != 1 || roll.DiceID != "standard_d6" || roll.ReactionWindow == nil || !roll.ReactionWindow.Opens {
		t.Fatal("Salt Veil must expose a single normal defense roll")
	}
	e := NewEngine()
	for face := 1; face <= 6; face++ {
		result, err := e.executeResolvedEffects(&b, lib, effectContext{SourceActorID: "enemy", SourceContentID: "salt_veil", SourceContentType: "ability", ProposalIDs: []string{"attack"}, RolledFace: face}, ability.Resolution.Operations)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Preventions) != 1 || result.Preventions[0].ProposalID != "attack" || result.Preventions[0].Amount != (face+1)/2 {
			t.Fatalf("face %d prevention: %+v", face, result.Preventions)
		}
		// Blocking more than the incoming damage never heals or removes health.
		for _, incoming := range []int{1, 5} {
			batch, err := e.buildDamageBatch(&b, []state.SettledDamageSource{{ID: "attack", SourceActorID: "player", TargetActorID: "enemy", BaseAmount: incoming, Prevention: result.Preventions[0].Amount}})
			if err != nil {
				t.Fatal(err)
			}
			expected := max(0, incoming-(face+1)/2)
			if batch.Sources[0].FinalAmount != expected {
				t.Fatalf("face %d incoming %d damage %+v", face, incoming, batch.Sources)
			}
		}
	}
}
