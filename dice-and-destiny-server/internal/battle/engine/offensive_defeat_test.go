package engine

import (
	"diceanddestiny/server/internal/battle/command"
	battlerandom "diceanddestiny/server/internal/battle/random"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
	"fmt"
	"path/filepath"
	"testing"
)

func offensiveDefeatFixture(t *testing.T) (state.Battle, content.BattleLibrary) {
	t.Helper()
	b, lib := pairFixture(t)
	var err error
	lib, err = content.LoadBattleExtension(lib, filepath.Join("..", "..", "..", "content", "minions_v1"))
	if err != nil {
		t.Fatal(err)
	}
	b.Segment.Current = segment.Offensive
	for id, a := range b.Actors {
		a.Cards.Deck = []string{id + "-one", id + "-two", id + "-three", id + "-four"}
		a.Health = state.HealthMetadata{Model: "card_zones", MaxHealth: 4}
		b.Actors[id] = a
	}
	for _, id := range []string{"enemy", "enemy2"} {
		r := b.Settled.Actors[id]
		r.SelectedAbilityID = "brine_lash"
		r.SelectedTargetIDs = []string{"player"}
		r.FinalDice = rolledFaces(lib, "brine_d6", []int{3, 3, 1, 1, 1})
		r.PlanningCommitted = true
		b.Settled.Actors[id] = r
	}
	return b, lib
}

func TestProvokedLethalDamageCancelsOffense(t *testing.T) {
	for _, stage := range []string{stageOffensivePlan, stageOffensiveReact} {
		for _, health := range []int{2, 3} {
			t.Run(fmt.Sprintf("%s/health-%d", stage, health), func(t *testing.T) {
				b, lib := offensiveDefeatFixture(t)
				b.Settled.Stage = stage
				a := b.Actors["enemy2"]
				a.Cards.Deck = []string{"last-1", "last-2"}
				a.Cards.Hand = nil
				a.Cards.Discard = nil
				if health == 3 {
					a.Cards.Deck = append(a.Cards.Deck, "last-3")
				}
				b.Actors["enemy2"] = a
				p := b.Actors["player"]
				p.Resources.EnergyPoints = 2
				p.Cards.Hand = []string{"agitate-card"}
				b.Actors["player"] = p
				applyStatus(&b, lib, "enemy2", "volatile_poison", 1)
				openSettledWindow(&b, "offense", stage, "planning", []command.Type{command.TypePass, command.TypePlanningCards})
				e := NewEngine()
				e.namedRandom = &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{
					{Stream: "status_effect_dice", Bound: 6, Value: 0},
					{Stream: "damage_selection", Bound: health, Value: 0},
					{Stream: "damage_selection", Bound: health - 1, Value: 0},
				}}
				if stage == stageOffensivePlan {
					if err := e.playVenomCard(&b, lib, "player", "agitate-card", lib.Cards["agitate"], []string{"enemy2"}, "volatile_poison"); err != nil {
						t.Fatal(err)
					}
				} else {
					// Ability-triggered Provoke can also suspend an already revealed attack.
					venomRuntime(&b).Queue = append(venomRuntime(&b).Queue, state.VenomWork{Kind: "provoke", Rolls: captureToxins(&b, lib, "enemy2", []string{"volatile_poison"}, 1, true)})
				}
				if _, err := e.startVenomWork(&b, lib, false); err != nil {
					t.Fatal(err)
				}
				for i := 0; i < 8 && b.Settled.Stage == stageOngoingReact; i++ {
					if _, err := e.handleStatusReactionCommand(&b, lib, command.Command{ActorID: b.Settled.Window.RequiredActorID, Type: command.TypePass}); err != nil {
						t.Fatal(err)
					}
				}
				if b.Settled.Stage != stageOngoingDamage {
					t.Fatalf("expected toxin damage, got %s", b.Settled.Stage)
				}
				for i := 0; i < 8 && b.Settled.Stage == stageOngoingDamage; i++ {
					if _, err := e.handleDamageReactionCommand(&b, lib, command.Command{ActorID: b.Settled.Window.RequiredActorID, Type: command.TypePass}); err != nil {
						t.Fatal(err)
					}
				}
				if b.Actors["enemy2"].CurrentHealth() != health-2 {
					t.Fatal("toxin did not deal two damage")
				}
				if b.Settled.Stage != stage || b.Status != state.BattleActive {
					t.Fatalf("surviving enemy must keep offense active: %s %s", b.Settled.Stage, b.Status)
				}
				if health == 2 {
					if b.Actors["enemy2"].DefeatState != state.ActorDefeated {
						t.Fatal("lethal Provoke resumed offense with pending defeat")
					}
					if _, ok := b.Flow.PendingInput["enemy2"]; ok {
						t.Fatal("dead enemy received restored input")
					}
					if b.Settled.Actors["enemy2"].SelectedAbilityID != "" {
						t.Fatal("dead enemy retained its attack selection")
					}
				}
				// Save/reload cannot restore the canceled attack.
				b = b.Clone()
				closeSettledWindow(&b)
				if _, err := e.finalizeOffensiveSources(&b, lib); err != nil {
					t.Fatal(err)
				}
				want := 1
				if health == 3 {
					want = 2
				}
				if len(b.Settled.OffensiveSources) != want {
					t.Fatalf("want %d living attackers, got %+v", want, b.Settled.OffensiveSources)
				}
				for _, s := range b.Settled.OffensiveSources {
					if health == 2 && s.SourceActorID == "enemy2" {
						t.Fatal("dead enemy carried damage into defense")
					}
				}
			})
		}
	}
}

func TestOffensiveExitCancelsZeroHealthAttack(t *testing.T) {
	b, lib := offensiveDefeatFixture(t)
	a := b.Actors["enemy2"]
	a.Cards.Deck = nil
	a.Cards.Hand = nil
	a.Cards.Discard = nil
	b.Actors["enemy2"] = a
	b.Settled.Stage = stageOffensiveReact
	b.Settled.OffensiveSources = []state.SettledDamageSource{{ID: "already-prepared", SourceActorID: "enemy2", TargetActorID: "player", BaseAmount: 4}}
	if _, err := NewEngine().finalizeOffensiveSources(&b, lib); err != nil {
		t.Fatal(err)
	}
	if len(b.Settled.OffensiveSources) != 1 || b.Settled.OffensiveSources[0].SourceActorID != "enemy" {
		t.Fatalf("zero-health attack survived offense exit: %+v", b.Settled.OffensiveSources)
	}
	if b.Segment.Current != segment.Defensive {
		t.Fatalf("did not advance to defense: %s", b.Segment.Current)
	}
}

func TestLethalToxinEndsBattleWhenLastEnemyDies(t *testing.T) {
	b, lib := offensiveDefeatFixture(t)
	a := b.Actors["enemy"]
	a.Cards.Deck = nil
	a.DefeatState = state.ActorDefeated
	b.Actors["enemy"] = a
	a = b.Actors["enemy2"]
	a.Cards.Deck = []string{"last-1", "last-2"}
	b.Actors["enemy2"] = a
	venomRuntime(&b).Active = &state.VenomWork{Kind: "provoke"}
	e := NewEngine()
	batch, err := e.buildDamageBatch(&b, []state.SettledDamageSource{{ID: "toxin", SourceActorID: "enemy2", TargetActorID: "enemy2", SourceContentID: "volatile_poison", BaseAmount: 2}})
	if err != nil {
		t.Fatal(err)
	}
	b.Settled.PendingDamage = batch
	if _, err = e.finishDamageBatch(&b, lib); err != nil {
		t.Fatal(err)
	}
	if b.Status != state.BattleVictory || b.WinnerActorID != "player" || b.Settled.Actors["enemy2"].SelectedAbilityID != "" {
		t.Fatalf("last enemy's lethal toxin did not end battle: %s %s", b.Status, b.WinnerActorID)
	}
}

func TestCombatDamageRemainsSimultaneousAfterOffense(t *testing.T) {
	b, lib := offensiveDefeatFixture(t)
	b.Segment.Current = segment.DamageResolution
	b.Settled.OffensiveSources = []state.SettledDamageSource{
		{ID: "incoming", SourceActorID: "enemy2", TargetActorID: "player", BaseAmount: 4},
		{ID: "outgoing", SourceActorID: "player", TargetActorID: "enemy2", BaseAmount: 4},
	}
	e := NewEngine()
	batch, err := e.buildDamageBatch(&b, b.Settled.OffensiveSources)
	if err != nil {
		t.Fatal(err)
	}
	b.Settled.PendingDamage = batch
	if _, err = e.finishDamageBatch(&b, lib); err != nil {
		t.Fatal(err)
	}
	if b.Actors["player"].CurrentHealth() != 0 || b.Actors["enemy2"].CurrentHealth() != 0 || len(b.Settled.OffensiveSources) != 2 {
		t.Fatal("late lethal damage incorrectly canceled a simultaneous attack")
	}
}
