package engine

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	battlerandom "diceanddestiny/server/internal/battle/random"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

func adventurerFixture(t *testing.T) (state.Battle, content.BattleLibrary) {
	t.Helper()
	lib, err := content.LoadBattleExtension(settledTestLibrary(t), filepath.Join("..", "..", "..", "content", "adventurer_v1"))
	if err != nil {
		t.Fatal(err)
	}
	b := settledStatusBattle(t, lib, "", 0)
	a := b.Actors["player"]
	a.DefinitionID = "adventurer"
	a.Resources.EnergyPoints = 10
	r := b.Settled.Actors["player"]
	r.OffensiveAbilityIDs = lib.Combatants["adventurer"].AbilityBoard.Offensive
	r.DefensiveAbilityIDs = lib.Combatants["adventurer"].AbilityBoard.Defensive
	r.UsedAbilities = map[string]int{}
	r.CardInstances = map[string]state.CardInstance{}
	for _, entry := range lib.Combatants["adventurer"].Decklist {
		for n := 0; n < entry.Count; n++ {
			id := fmt.Sprintf("%s-%d", entry.CardID, n)
			r.CardInstances[id] = state.CardInstance{InstanceID: id, DefinitionID: entry.CardID}
			a.Cards.Hand = append(a.Cards.Hand, id)
		}
	}
	b.Actors["player"] = a
	b.Settled.Actors["player"] = r
	b.Settled.Stage = stageOffensivePlan
	openSettledWindowForActors(&b, "test", stageOffensivePlan, "planning", []command.Type{command.TypePlanningCards}, []string{"player"}, true)
	return b, lib
}
func adventurerRoll(b *state.Battle, lib content.BattleLibrary, faces []int) {
	r := b.Settled.Actors["player"]
	r.FinalDice = rolledCombatFaces(lib, b.Actors["player"].DiceLoadout, faces)
	r.RollsUsed = 3
	r.QualifiedAbilityIDs = qualifiedAbilities(lib, r.OffensiveAbilityIDs, r.FinalDice, nil)
	b.Settled.Actors["player"] = r
}

func TestAdventurerEveryOffensiveRoll(t *testing.T) {
	b, lib := adventurerFixture(t)
	for code := 0; code < 7776; code++ {
		n := code
		faces := make([]int, 5)
		sword, shield, coin := 0, 0, 0
		mask := 0
		for i := range faces {
			faces[i] = n%6 + 1
			n /= 6
			mask |= 1 << (faces[i] - 1)
			if faces[i] <= 3 {
				sword++
			} else if faces[i] <= 5 {
				shield++
			} else {
				coin++
			}
		}
		adventurerRoll(&b, lib, faces)
		small := mask&15 == 15 || mask&30 == 30 || mask&60 == 60
		large := mask&31 == 31 || mask&62 == 62
		expected := map[string]int{"adventurer_strike": 0, "guarded_strike": 0, "measured_strike": 0, "adventurer_small_straight": 0, "adventurer_large_straight": 0, "decisive_blow": 0}
		if sword >= 3 {
			expected["adventurer_strike"] = sword + 1
		}
		if sword >= 2 && shield >= 3 {
			expected["guarded_strike"] = 5
		}
		if sword >= 2 && coin >= 2 {
			expected["measured_strike"] = 4
		}
		if small {
			expected["adventurer_small_straight"] = 6
		}
		if large {
			expected["adventurer_large_straight"] = 8
		}
		if coin >= 4 {
			expected["decisive_blow"] = 10
		}
		for id, want := range expected {
			tier, ok := qualifiedTier(lib.Abilities[id], b.Settled.Actors["player"].FinalDice)
			if ok != (want > 0) {
				t.Fatalf("%v %s qualification=%v want damage %d", faces, id, ok, want)
			}
			if ok {
				got := summarizeOffensiveOutcome(tier.Operations, []string{"enemy"})["base_damage"]
				if got != want {
					t.Fatalf("%s %v damage %v want %d", id, faces, got, want)
				}
			}
		}
	}
}

func TestAdventurerCardsAndSelectedTier(t *testing.T) {
	t.Run("nudge boundaries and ownership", func(t *testing.T) {
		for before := 1; before <= 6; before++ {
			for after := 0; after <= 7; after++ {
				b, lib := adventurerFixture(t)
				adventurerRoll(&b, lib, []int{before, 4, 4, 5, 6})
				health := b.Actors["player"].CurrentHealth()
				err := playProgramCard(NewEngine(), &b, lib, "player", "nudge-0", func(c programChoice) bool { return c.Die == 0 && c.Face == after })
				valid := after >= 1 && after <= 5 && (after == before-1 || after == before+1)
				if (err == nil) != valid {
					t.Fatalf("%d -> %d: %v", before, after, err)
				}
				if valid && (b.Settled.Actors["player"].FinalDice[0].Face != after || b.Settled.Actors["player"].RollsUsed != 3 || b.Actors["player"].Resources.EnergyPoints != 9 || b.Actors["player"].CurrentHealth() != health) {
					t.Fatal("nudge changed wrong resources or die")
				}
			}
		}
		b, lib := adventurerFixture(t)
		if err := playProgramCard(NewEngine(), &b, lib, "player", "nudge-0", nil); err == nil {
			t.Fatal("nudge before first roll")
		}
	})
	t.Run("try again does not spend rolls or change other dice", func(t *testing.T) {
		b, lib := adventurerFixture(t)
		adventurerRoll(&b, lib, []int{1, 2, 3, 4, 5})
		script := &ownedSelectionScript{Values: []battlerandom.ScriptedValue{{Stream: "combat_dice", Bound: 6, Value: 5}}}
		e := NewEngine()
		e.namedRandom = script
		if err := playProgramCard(e, &b, lib, "player", "try_again-0", func(c programChoice) bool { return c.Die == 0 }); err != nil {
			t.Fatal(err)
		}
		r := b.Settled.Actors["player"]
		if r.RollsUsed != 3 || r.FinalDice[0].Face != 6 || r.FinalDice[1].Face != 2 || !containsString(r.QualifiedAbilityIDs, "adventurer_large_straight") {
			t.Fatalf("reroll result %+v", r)
		}
	})
	t.Run("draw and energy and ordinary discard", func(t *testing.T) {
		b, lib := adventurerFixture(t)
		a := b.Actors["player"]
		a.Cards.Hand = []string{"take_stock-0", "second_wind-0"}
		a.Cards.Deck = []string{"brace-0", "brace-1"}
		b.Actors["player"] = a
		e := NewEngine()
		for _, id := range []string{"take_stock-0", "second_wind-0"} {
			if err := playProgramCard(e, &b, lib, "player", id, nil); err != nil {
				t.Fatal(err)
			}
		}
		a = b.Actors["player"]
		if a.Resources.EnergyPoints != 11 || len(a.Cards.Hand) != 2 || len(a.Cards.Discard) != 2 || a.CurrentHealth() != 4 {
			t.Fatalf("resources %+v", a)
		}
	})
	t.Run("strong swing stacks only on qualified chosen attack and expires", func(t *testing.T) {
		b, lib := adventurerFixture(t)
		e := NewEngine()
		if err := playProgramCard(e, &b, lib, "player", "strong_swing-0", func(c programChoice) bool { return c.Ability == "adventurer_guard" }); err == nil {
			t.Fatal("defensive card target")
		}
		for _, id := range []string{"strong_swing-0", "strong_swing-1"} {
			if err := playProgramCard(e, &b, lib, "player", id, func(c programChoice) bool { return c.Ability == "adventurer_strike" }); err != nil {
				t.Fatal(err)
			}
		}
		adventurerRoll(&b, lib, []int{1, 2, 3, 1, 2})
		r := b.Settled.Actors["player"]
		r.SelectedAbilityID = "adventurer_strike"
		r.SelectedTierID = "3_swords"
		r.SelectedTargetIDs = []string{"enemy"}
		b.Settled.Actors["player"] = r
		ops, ok := resolvedOffensiveOperations(&b, lib, "player")
		if !ok || summarizeOffensiveOutcome(ops, nil)["base_damage"] != 8 {
			t.Fatalf("lower tier plus two cards %v", ops)
		}
		// Expiry at Offensive Exit is covered by the real phase transition in
		// TestStrongSwingExpiresAtOffensiveExitUsedOrUnused.
		adventurerRoll(&b, lib, []int{4, 4, 5, 5, 6})
		if ops, ok := resolvedOffensiveOperations(&b, lib, "player"); ok || len(ops) > 0 {
			t.Fatal("bonus turned miss into hit")
		}
	})
	t.Run("cost and timing rejection", func(t *testing.T) {
		for _, entry := range []string{"brace", "nudge", "try_again", "strong_swing", "take_stock", "second_wind"} {
			b, lib := adventurerFixture(t)
			b.Segment.Current = segment.Income
			closeSettledWindow(&b)
			b.Settled.Stage = ""
			if err := playProgramCard(NewEngine(), &b, lib, "player", entry+"-0", nil); err == nil {
				t.Fatalf("%s wrong phase accepted", entry)
			}
		}
		b, lib := adventurerFixture(t)
		a := b.Actors["player"]
		a.Resources.EnergyPoints = 0
		b.Actors["player"] = a
		if err := playProgramCard(NewEngine(), &b, lib, "player", "take_stock-0", nil); err == nil {
			t.Fatal("unpaid draw")
		}
	})
}

func TestAdventurerProtectionAndBrace(t *testing.T) {
	b, lib := adventurerFixture(t)
	e := NewEngine()
	for _, id := range []string{"guarded_strike", "measured_strike"} {
		tier := lib.Abilities[id].Qualification.ActivationTiers[0]
		res, err := e.executeEffects(&b, lib, effectContext{SourceActorID: "player", TargetActorIDs: []string{"enemy"}}, tier.Operations)
		if err != nil {
			t.Fatal(err)
		}
		if err = e.applyEffectMutations(&b, lib, "", res); err != nil {
			t.Fatal(err)
		}
	}
	if stacks(&b, "player", "protect") != 2 || b.Actors["player"].Resources.EnergyPoints != 11 {
		t.Fatal("ability benefit not granted")
	}
	b.Segment.Current = segment.DamageResolution
	b.Settled.Stage = stageDamageReact
	b.Settled.PendingDamage = &state.SettledDamageBatch{Sources: []state.SettledDamageSource{{ID: "a", SourceActorID: "enemy", SourceContentID: "sword_cut", TargetActorID: "player", BaseAmount: 7}, {ID: "b", SourceActorID: "enemy", SourceContentID: "sword_cut", TargetActorID: "player", BaseAmount: 3}}}
	openSettledWindow(&b, "damage", stageDamageReact, "damage_response", []command.Type{command.TypeCommitInteraction, command.TypePass})
	// Round grant is persisted and independent of Guard or another attack source.
	raw, _ := json.Marshal(b)
	var restored state.Battle
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	b = restored
	choice := command.InteractionCommitmentData{ChoiceID: "spend_round_prevention", ProposalIDs: []string{"b"}}
	if _, err := e.spendRoundPrevention(&b, lib, "player", choice); err != nil {
		t.Fatal(err)
	}
	if got := settledSourceAmount(b.Settled.PendingDamage.Sources[1]); got != 1 {
		t.Fatalf("protection result %d", got)
	}
	if _, err := e.spendRoundPrevention(&b, lib, "player", choice); err == nil {
		t.Fatal("spent twice")
	}
	if err := playProgramCard(e, &b, lib, "player", "brace-0", func(c programChoice) bool { return c.Source == "a" }); err != nil {
		t.Fatal(err)
	}
	if got := settledSourceAmount(b.Settled.PendingDamage.Sources[0]); got != 4 {
		t.Fatalf("brace result %d", got)
	}
	if stacks(&b, "player", "protect") != 0 {
		t.Fatal("spent status remains on actor")
	}
	applyStatus(&b, lib, "player", "protect", 9)
	if stacks(&b, "player", "protect") != 2 {
		t.Fatal("Protect exceeds cap")
	}
	b.Segment.Round++
	b.Segment.Current = segment.OngoingEffects
	b.Settled.Stage = ""
	b.Settled.PendingDamage = nil
	closeSettledWindow(&b)
	if _, err := e.progressAutomaticEffects(&b, lib); err != nil {
		t.Fatal(err)
	}
	if stacks(&b, "player", "protect") != 0 || b.Segment.Current != segment.Income {
		t.Fatal("Protect did not expire during automatic ongoing effects")
	}
	applyStatus(&b, lib, "player", "protect", 2)
	removeSourcesByActor(&b, "player")
	if stacks(&b, "player", "protect") != 0 {
		t.Fatal("cancelled attack retained protection")
	}
}

func TestAdventurerAllGuardRolls(t *testing.T) {
	for a := 1; a <= 6; a++ {
		for c := 1; c <= 6; c++ {
			for d := 1; d <= 6; d++ {
				b, lib := adventurerFixture(t)
				faces := []int{a, c, d}
				prevention, energy := 0, 0
				for _, f := range faces {
					if f <= 3 {
						prevention++
					} else if f <= 5 {
						prevention += 2
					} else {
						energy = 1
					}
				}
				b.Segment.Current = segment.Defensive
				b.Settled.Stage = stageDefenseReact
				b.Settled.OffensiveSources = []state.SettledDamageSource{{ID: "incoming", SourceActorID: "enemy", SourceContentID: "sword_cut", TargetActorID: "player", BaseAmount: 10}}
				b.Settled.DefenseSelections = map[string]state.SettledDefense{"player": {ActorID: "player", AbilityID: "adventurer_guard", SourceID: "incoming", RolledFace: a, RolledFaces: faces}}
				if _, err := NewEngine().finalizeDefenses(&b, lib); err != nil {
					t.Fatal(err)
				}
				if b.Settled.OffensiveSources[0].Prevention != prevention || b.Actors["player"].Resources.EnergyPoints != 10+energy {
					t.Fatalf("guard %v: %+v energy %d", faces, b.Settled.OffensiveSources, b.Actors["player"].Resources.EnergyPoints)
				}
			}
		}
	}
}

func TestAdventurerProtectIsGrantedBeforeDefense(t *testing.T) {
	b, lib := adventurerFixture(t)
	enemy := b.Actors["enemy"]
	enemy.Cards.Deck = []string{"enemy-health"}
	b.Actors["enemy"] = enemy
	b.Segment.Current = segment.Offensive
	adventurerRoll(&b, lib, []int{1, 2, 4, 4, 5})
	rt := b.Settled.Actors["player"]
	rt.SelectedAbilityID = "guarded_strike"
	rt.SelectedTargetIDs = []string{"enemy"}
	b.Settled.Actors["player"] = rt
	closeSettledWindow(&b)
	events, err := NewEngine().finalizeOffensiveSources(&b, lib)
	if err != nil {
		t.Fatal(err)
	}
	if stacks(&b, "player", "protect") != 2 || b.Segment.Current != segment.Defensive {
		t.Fatalf("Protect must exist on the actor before defense: stacks=%d segment=%+v status=%s", stacks(&b, "player", "protect"), b.Segment, b.Status)
	}
	if lib.Statuses["protect"].Polarity != "positive" {
		t.Fatal("Protect must be a positive status")
	}
	found := false
	for _, ev := range events {
		if ev.Data["status_application"] != nil {
			found = true
		}
	}
	if !found {
		t.Fatal("missing public status acquisition event")
	}
	for _, source := range b.Settled.OffensiveSources {
		for _, app := range source.StatusApplications {
			if app.StatusID == "protect" {
				t.Fatal("Protect must not be deferred until after damage")
			}
		}
	}
}
