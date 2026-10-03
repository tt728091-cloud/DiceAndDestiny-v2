package engine

import (
	"diceanddestiny/server/internal/battle/command"
	battlerandom "diceanddestiny/server/internal/battle/random"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
)

func curseFixture(t *testing.T) (state.Battle, content.BattleLibrary) {
	t.Helper()
	b, lib := venomFixture(t)
	var err error
	lib, err = content.LoadBattleExtension(lib, filepath.Join("..", "..", "..", "content", "curse_v1"))
	if err != nil {
		t.Fatal(err)
	}
	b.SettledCatalog, _ = json.Marshal(lib)
	for _, id := range []string{"player", "enemy"} {
		a := b.Actors[id]
		a.Controller = state.ControllerExternal
		a.Resources.EnergyPoints = 20
		a.DiceLoadout = []state.DiceLoadoutEntry{{DiceID: "curse_d6", Count: 5}}
		a.DefinitionID = "curse"
		a.Health = state.HealthMetadata{Model: "card_zones", MaxHealth: 24}
		a.Cards = state.CardZones{}
		r := b.Settled.Actors[id]
		r.CardInstances = map[string]state.CardInstance{}
		r.UsedAbilities = map[string]int{}
		r.OffensiveAbilityIDs = lib.Combatants["curse"].AbilityBoard.Offensive
		r.DefensiveAbilityIDs = lib.Combatants["curse"].AbilityBoard.Defensive
		r.HandLimit = 6
		for _, entry := range lib.Combatants["curse"].Decklist {
			instance := id + ":" + entry.CardID
			a.Cards.Deck = append(a.Cards.Deck, instance)
			r.CardInstances[instance] = state.CardInstance{InstanceID: instance, DefinitionID: entry.CardID}
		}
		b.Actors[id] = a
		b.Settled.Actors[id] = r
	}
	curseRuntime(&b)
	b.Random = state.RandomState{Mode: state.RandomModeReproducible, Algorithm: state.RandomAlgorithmSHA256, Seed: 91}
	return b, lib
}
func curseHand(b *state.Battle, actor, id string) {
	a := b.Actors[actor]
	instance := actor + ":" + id
	for i, c := range a.Cards.Deck {
		if c == instance {
			a.Cards.Deck = append(a.Cards.Deck[:i], a.Cards.Deck[i+1:]...)
			break
		}
	}
	a.Cards.Hand = append(a.Cards.Hand, instance)
	b.Actors[actor] = a
}
func cursePlan(b *state.Battle) {
	b.Segment.Current = segment.Offensive
	openSettledWindow(b, "plan", stageOffensivePlan, "planning", []command.Type{command.TypePlanningCards, command.TypePlanningRoll, command.TypePlanningKeep, command.TypePlanningReroll, command.TypePlanningAbility, command.TypePlanningPass})
}
func cursePlay(t *testing.T, e Engine, b *state.Battle, lib content.BattleLibrary, id, key, target string) {
	t.Helper()
	curseHand(b, "player", id)
	if err := e.playCurseCard(b, lib, "player", "player:"+id, lib.Cards[id], []string{target}, key); err != nil {
		t.Fatal(err)
	}
}
func markedCount(b *state.Battle, actor string) int {
	n := 0
	for _, d := range curseRuntime(b).Dice[actor] {
		n += len(d.CursedFaces)
	}
	return n
}
func finishCurseChoice(t *testing.T, e Engine, b *state.Battle, lib content.BattleLibrary, key string) {
	t.Helper()
	if _, err := e.startCurseWork(b, lib, false); err != nil {
		t.Fatal(err)
	}
	if b.Settled.Curse.Active == nil {
		t.Fatal("missing choice")
	}
	if !containsString(b.Settled.Curse.Active.Options, key) {
		t.Fatalf("choice %s not in %+v", key, b.Settled.Curse.Active)
	}
	actor := b.Settled.Window.RequiredActorID
	for _, cmd := range e.LegalActions(b, actor) {
		var p command.CommitInteractionPayload
		_ = command.DecodePayload(cmd, &p)
		if p.Commitment.ChoiceID == key {
			if _, err := e.handleSettledCommand(b, cmd); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatal("no authoritative choice action")
}
func TestCurseOwnedDicePriorityReuseAndResume(t *testing.T) {
	b, lib := curseFixture(t)
	e := NewEngine()
	b.Segment.Current = segment.OngoingEffects
	markCurse(&b, "enemy", 3, 6)
	entomb(&b, "enemy", 3)
	var sequence []int
	for n := 0; n < 12; n++ {
		i, err := e.selectOwned(&b, "enemy", fmt.Sprint(n), allOwned(&b, "enemy"))
		if err != nil {
			t.Fatal(err)
		}
		sequence = append(sequence, i)
		if n == 6 {
			clone := b.Clone()
			data, _ := json.Marshal(clone)
			if err = json.Unmarshal(data, &b); err != nil {
				t.Fatal(err)
			}
		}
	}
	if sequence[0] != 3 || sequence[5] != 3 || sequence[10] != 3 {
		t.Fatalf("Entomb priority: %v", sequence)
	}
	for _, start := range []int{0, 5} {
		seen := map[int]bool{}
		for _, v := range sequence[start : start+5] {
			seen[v] = true
		}
		if len(seen) != 5 {
			t.Fatalf("replacement before exhaustion: %v", sequence)
		}
	}
	r := b.Settled.Actors["enemy"]
	r.FinalDice = rolledFaces(lib, "curse_d6", []int{1, 2, 3, 4, 5})
	b.Settled.Actors["enemy"] = r
	before := cloneDice(r.FinalDice)
	_, err := e.rollOwnedSubset(&b, lib, "enemy", "card", allOwned(&b, "enemy"), 2)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, b.Settled.Actors["enemy"].FinalDice) {
		t.Fatal("extra dice changed offense")
	}
	clone := b.Clone()
	clone.Settled.Curse.Dice["enemy"][3].CursedFaces[0] = 1
	if b.Settled.Curse.Dice["enemy"][3].CursedFaces[0] != 6 {
		t.Fatal("clone shares marks")
	}
	view := snapshot.FromBattleForViewer(b, "player")
	if len(view.Actors["enemy"].OwnedDice) != 5 {
		t.Fatal("public physical dice missing")
	}
}
func TestCursePhysicalTriggersAndOneRetry(t *testing.T) {
	b, lib := curseFixture(t)
	markCurse(&b, "enemy", 0, 1)
	markCurse(&b, "enemy", 0, 2)
	entomb(&b, "enemy", 0)
	curseRuntime(&b).Preparations = []state.CursePreparation{{CardID: "second_knell", Source: "player", Target: "enemy"}, {CardID: "black_dividend", Source: "player", Target: "enemy"}}
	e := NewEngine()
	e.namedRandom = &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "curse_dice", Bound: 6, Value: 0}, {Stream: "curse_dice", Bound: 6, Value: 1}}}
	energy := b.Actors["player"].Resources.EnergyPoints
	d, err := e.ownedRoll(&b, lib, "enemy", 0, "curse_dice", true)
	if err != nil {
		t.Fatal(err)
	}
	if d.Face != 2 || !d.EffectRetried || stacks(&b, "enemy", "curse_count") != 2 || curseRuntime(&b).Dice["enemy"][0].Entombed {
		t.Fatalf("trigger/retry failed: %+v", d)
	}
	if b.Actors["player"].Resources.EnergyPoints != energy+1 {
		t.Fatal("dividend must reward once per round")
	}
	applyStatus(&b, lib, "enemy", "curse_count", 100)
	if stacks(&b, "enemy", "curse_count") != 102 {
		t.Fatal("Count is capped")
	}
}
func TestCursePlacementCards(t *testing.T) {
	for _, tc := range []struct {
		id, key string
		want    int
	}{{"mark_the_number", "apply", 1}, {"shared_misfortune", "4", 2}, {"rotten_numeral", "6", 3}, {"maledictions_refusal", "apply", 1}} {
		t.Run(tc.id, func(t *testing.T) {
			b, lib := curseFixture(t)
			cursePlan(&b)
			e := NewEngine()
			cursePlay(t, e, &b, lib, tc.id, tc.key, "enemy")
			if markedCount(&b, "enemy") != tc.want {
				t.Fatal("wrong placements")
			}
			if len(curseCardChoices(&b, lib, "player", lib.Cards[tc.id])) != 0 {
				t.Fatal("same named card reusable this round")
			}
		})
	}
}
func TestCurseChoiceCards(t *testing.T) {
	t.Run("widen", func(t *testing.T) {
		b, lib := curseFixture(t)
		cursePlan(&b)
		markCurse(&b, "enemy", 0, 1)
		e := NewEngine()
		e.namedRandom = &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "curse_dice", Bound: 6, Value: 2}}}
		cursePlay(t, e, &b, lib, "widen_the_crack", "0", "enemy")
		finishCurseChoice(t, e, &b, lib, "4")
		if !containsInt(curseRuntime(&b).Dice["enemy"][0].CursedFaces, 4) || stacks(&b, "enemy", "curse_count") != 0 {
			t.Fatal("adjacent mark retroactively triggered")
		}
	})
	for _, key := range []string{"count", "roll"} {
		t.Run("tomb_"+key, func(t *testing.T) {
			b, lib := curseFixture(t)
			cursePlan(&b)
			for i := 0; i < 5; i++ {
				for f := 1; f <= 6; f++ {
					markCurse(&b, "enemy", i, f)
				}
			}
			e := NewEngine()
			cursePlay(t, e, &b, lib, "tombs_choice", "choose", "enemy")
			finishCurseChoice(t, e, &b, lib, key)
			want := 3
			if key == "roll" {
				want = 5
			}
			if stacks(&b, "enemy", "curse_count") != want {
				t.Fatal("wrong Tomb outcome")
			}
		})
	}
	for _, key := range []string{"damage", "entangle"} {
		t.Run("misfortune_"+key, func(t *testing.T) {
			b, lib := curseFixture(t)
			cursePlan(&b)
			applyStatus(&b, lib, "enemy", "curse_count", 3)
			e := NewEngine()
			cursePlay(t, e, &b, lib, "misfortunes_choice", "choose", "enemy")
			finishCurseChoice(t, e, &b, lib, key)
			if stacks(&b, "enemy", "curse_count") != 0 {
				t.Fatal("Count cost not paid")
			}
			if key == "entangle" && stacks(&b, "enemy", "cursed_entangle") != 1 {
				t.Fatal("missing entangle")
			}
			if key == "damage" && (b.Settled.PendingDamage == nil || b.Settled.PendingDamage.Sources[0].BaseAmount != 2) {
				t.Fatal("missing normal damage response")
			}
		})
	}
}
func TestCursedEntangleAndEntombRerollRules(t *testing.T) {
	b, lib := curseFixture(t)
	cursePlan(&b)
	e := NewEngine()
	for i := 0; i < 5; i++ {
		for f := 1; f <= 6; f++ {
			markCurse(&b, "enemy", i, f)
		}
	}
	applyStatus(&b, lib, "enemy", "cursed_entangle", 1)
	applyStatus(&b, lib, "enemy", "entangle", 1)
	if err := e.cursedEntangleEntry(&b, lib); err != nil {
		t.Fatal(err)
	}
	r := b.Settled.Actors["enemy"]
	if r.MaxRolls != 2 || r.RollsUsed != 0 || len(r.FinalDice) != 0 || stacks(&b, "enemy", "curse_count") != 5 || stacks(&b, "enemy", "entangle") != 0 {
		t.Fatal("Cursed Entangle combined incorrectly")
	}
	markCurse(&b, "player", 0, 6)
	entomb(&b, "player", 0)
	r = b.Settled.Actors["player"]
	r.RollsUsed = 1
	r.FinalDice = rolledFaces(lib, "curse_d6", []int{1, 1, 1, 1, 1})
	b.Settled.Actors["player"] = r
	for _, cmd := range e.LegalActions(&b, "player") {
		switch cmd.Type {
		case command.TypePlanningKeep:
			var p command.PlanningKeepPayload
			_ = command.DecodePayload(cmd, &p)
			if containsInt(p.KeptIndices, 0) {
				t.Fatal("bound die can be kept")
			}
		case command.TypePlanningReroll:
			var p command.PlanningRerollPayload
			_ = command.DecodePayload(cmd, &p)
			if !containsInt(p.RerollIndices, 0) {
				t.Fatal("bound die can be excluded")
			}
		}
	}
}

func TestCursePreparedCardsAndConversionModes(t *testing.T) {
	for _, tc := range []struct {
		id                                   string
		count, damage, remainder, taxE, taxC int
	}{{"three_knocks", 8, 4, 2, 0, 0}, {"grave_interest", 8, 1, 2, 1, 0}, {"black_tax", 8, 1, 2, 0, 1}, {"stored_calamity", 8, 0, 8, 0, 0}, {"three_knocks", 2, 0, 2, 0, 0}} {
		t.Run(fmt.Sprintf("%s_%d", tc.id, tc.count), func(t *testing.T) {
			b, lib := curseFixture(t)
			cursePlan(&b)
			e := NewEngine()
			applyStatus(&b, lib, "enemy", "curse_count", tc.count)
			key := "apply"
			if tc.id == "stored_calamity" {
				key = "store"
			}
			cursePlay(t, e, &b, lib, tc.id, key, "enemy")
			b.Segment.Current = segment.OngoingEffects
			b.Segment.Round++
			closeSettledWindow(&b)
			before := b.Actors["enemy"].CurrentHealth()
			if _, err := e.convertCurse(&b, lib); err != nil {
				t.Fatal(err)
			}
			if tc.damage > 0 {
				if b.Settled.PendingDamage == nil || b.Settled.PendingDamage.Sources[0].BaseAmount != tc.damage {
					t.Fatalf("wrong conversion: %+v", b.Settled.PendingDamage)
				}
				for i := range b.Settled.PendingDamage.Removals {
					b.Settled.PendingDamage.Removals[i].Accepted = true
				}
				if _, err := e.finishDamageBatch(&b, lib); err != nil {
					t.Fatal(err)
				}
			}
			if before-b.Actors["enemy"].CurrentHealth() != tc.damage || stacks(&b, "enemy", "curse_count") != tc.remainder || stacks(&b, "enemy", "three_knocks_status") != 0 {
				t.Fatalf("wrong conversion outcome: count %d health %d", stacks(&b, "enemy", "curse_count"), b.Actors["enemy"].CurrentHealth())
			}
			c := curseRuntime(&b)
			if stacks(&b, "enemy", "grave_debt") != tc.taxE || c.TaxCards["enemy"] != tc.taxC {
				t.Fatal("tax missing")
			}
		})
	}
	for _, id := range []string{"grave_interest", "black_tax", "stored_calamity"} {
		t.Run("mode_exclusion_"+id, func(t *testing.T) {
			b, lib := curseFixture(t)
			cursePlan(&b)
			e := NewEngine()
			cursePlay(t, e, &b, lib, "grave_interest", "apply", "enemy")
			if len(curseCardChoices(&b, lib, "player", lib.Cards[id])) > 0 {
				t.Fatal("second mode accepted")
			}
		})
	}
}
func TestCurseRollCardsAndCleanseGuard(t *testing.T) {
	t.Run("unquiet", func(t *testing.T) {
		b, lib := curseFixture(t)
		cursePlan(&b)
		for f := 1; f <= 6; f++ {
			markCurse(&b, "enemy", 2, f)
		}
		e := NewEngine()
		r := b.Settled.Actors["enemy"]
		r.FinalDice = []state.RolledDie{{Index: 2, Face: 5, DieID: "curse_d6"}}
		r.RollHistory = []state.RollBatch{{Dice: append([]state.RolledDie(nil), r.FinalDice...)}}
		b.Settled.Actors["enemy"] = r
		before := b.Settled.Actors["enemy"]
		before.FinalDice = append([]state.RolledDie(nil), before.FinalDice...)
		before.RollHistory = []state.RollBatch{{Dice: append([]state.RolledDie(nil), before.RollHistory[0].Dice...)}}
		marks := markedCount(&b, "enemy")
		start := len(curseRuntime(&b).Logs)
		cursePlay(t, e, &b, lib, "unquiet_hands", "2", "enemy")
		after := b.Settled.Actors["enemy"]
		if !reflect.DeepEqual(before.FinalDice, after.FinalDice) || !reflect.DeepEqual(before.RollHistory, after.RollHistory) || marks != markedCount(&b, "enemy") {
			t.Fatal("extra check changed saved offense or cursed faces")
		}
		found := false
		for _, log := range curseRuntime(&b).Logs[start:] {
			if log.Data["kind"] == "owned_roll" {
				found = true
				if log.Data["source_card_id"] != "unquiet_hands" || log.Data["source_actor_id"] != "player" || log.Data["cursed"] != true {
					t.Fatal("extra check feedback missing card or result")
				}
			}
		}
		if !found {
			t.Fatal("extra check roll not published")
		}
		if stacks(&b, "enemy", "curse_count") != 1 {
			t.Fatal("extra roll missing")
		}
	})
	t.Run("instrument", func(t *testing.T) {
		b, lib := curseFixture(t)
		cursePlan(&b)
		markCurse(&b, "enemy", 4, 1)
		markCurse(&b, "enemy", 1, 1)
		entomb(&b, "enemy", 1)
		e := NewEngine()
		cursePlay(t, e, &b, lib, "chosen_instrument", "4", "enemy")
		i, err := e.selectOwned(&b, "enemy", "defense", allOwned(&b, "enemy"))
		if err != nil || i != 1 {
			t.Fatal("instrument overrode entomb")
		}
		i, err = e.selectOwned(&b, "enemy", "defense", allOwned(&b, "enemy"))
		if err != nil || i != 4 {
			t.Fatal("instrument did not wait for eligible slot")
		}
		if len(curseRuntime(&b).Preparations) != 0 {
			t.Fatal("instrument not consumed")
		}
	})
	for _, id := range []string{"black_fingerprint"} {
		t.Run("prepare_"+id, func(t *testing.T) {
			b, lib := curseFixture(t)
			cursePlan(&b)
			e := NewEngine()
			cursePlay(t, e, &b, lib, id, "apply", "enemy")
			if len(curseRuntime(&b).Preparations) != 1 {
				t.Fatal("preparation missing")
			}
			b.Segment.Current = segment.OngoingEffects
			b.Segment.Round++
			expireCursePreparations(&b)
			if len(curseRuntime(&b).Preparations) != 0 {
				t.Fatal("preparation didn't expire")
			}
		})
	}
	for _, face := range []int{1, 6} {
		t.Run(fmt.Sprintf("refusal_%d", face), func(t *testing.T) {
			b, lib := curseFixture(t)
			cursePlan(&b)
			e := NewEngine()
			cursePlay(t, e, &b, lib, "maledictions_refusal", "apply", "enemy")
			for i := 0; i < 5; i++ {
				markCurse(&b, "enemy", i, 1)
			}
			applyStatus(&b, lib, "enemy", "curse_count", 4)
			e.namedRandom = &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "owned_die_selection", Bound: 5, Value: 0}, {Stream: "curse_dice", Bound: 6, Value: face - 1}}}
			if err := e.cleanseCurseCount(&b, lib, "enemy", 0); err != nil {
				t.Fatal(err)
			}
			want := 0
			if face == 1 {
				want = 5
			}
			if stacks(&b, "enemy", "curse_count") != want || len(curseRuntime(&b).Preparations) != 0 {
				t.Fatal("cleanse guard outcome wrong")
			}
		})
	}
}
func TestCurseEaterAndOffensiveReactions(t *testing.T) {
	for _, key := range []string{"energy", "draw"} {
		t.Run("eater_"+key, func(t *testing.T) {
			b, lib := curseFixture(t)
			cursePlan(&b)
			applyStatus(&b, lib, "enemy", "curse_count", 3)
			energy := b.Actors["player"].Resources.EnergyPoints
			e := NewEngine()
			cursePlay(t, e, &b, lib, "curse_eater", key, "enemy")
			if stacks(&b, "enemy", "curse_count") != 0 {
				t.Fatal("cost missing")
			}
			if key == "draw" && len(b.Actors["player"].Cards.Hand) != 2 || key == "energy" && b.Actors["player"].Resources.EnergyPoints != energy+2 {
				t.Fatal("wrong eater benefit")
			}
		})
	}
	for _, id := range []string{"call_the_mark", "no_safe_keep", "ruin_made_flesh", "blind_omen"} {
		t.Run(id, func(t *testing.T) {
			b, lib := curseFixture(t)
			e := NewEngine()
			for _, a := range []string{"player", "enemy"} {
				r := b.Settled.Actors[a]
				r.FinalDice = rolledFaces(lib, "curse_d6", []int{1, 1, 1, 4, 6})
				r.SelectedAbilityID = "hexbrand"
				r.SelectedTierID = "skull_3"
				r.SelectedTargetIDs = []string{enemyOf(&b, a)}
				r.KeptIndices = []int{0}
				r.RollsUsed = 1
				b.Settled.Actors[a] = r
			}
			markCurse(&b, "enemy", 0, 6)
			entomb(&b, "enemy", 0)
			r := b.Settled.Actors["enemy"]
			r.KeptIndices = []int{0}
			b.Settled.Actors["enemy"] = r
			openSettledWindow(&b, "react", stageOffensiveReact, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePass})
			applyStatus(&b, lib, "enemy", "curse_count", 9)
			key := "enemy:0:6"
			switch id {
			case "no_safe_keep":
				key = "enemy:0:0"
			case "ruin_made_flesh":
				key = "6"
			case "blind_omen":
				key = "blind"
			}
			cursePlay(t, e, &b, lib, id, key, "enemy")
			switch id {
			case "call_the_mark":
				if b.Settled.Actors["enemy"].FinalDice[0].Face != 6 || stacks(&b, "enemy", "curse_count") != 9 || !curseRuntime(&b).Dice["enemy"][0].Entombed {
					t.Fatal("set face must not physically trigger")
				}
				found := false
				for _, ev := range flushCurseEvents(&b) {
					if ev.Data["kind"] != "offensive_face_set" {
						continue
					}
					found = true
					if ev.ActorID != "enemy" || ev.Data["index"] != 0 || ev.Data["face_before"] != 1 || ev.Data["face"] != 6 || ev.Data["card_id"] != id {
						t.Fatalf("incorrect face-change feedback: %+v", ev)
					}
				}
				if !found {
					t.Fatal("face-change feedback missing")
				}
			case "no_safe_keep":
				r := b.Settled.Actors["enemy"]
				if containsInt(r.KeptIndices, 0) || !r.FinalDice[0].EffectRetried || r.RollsUsed != 1 {
					t.Fatal("forced reroll state wrong")
				}
				found := false
				for _, ev := range flushCurseEvents(&b) {
					if ev.Data["kind"] != "owned_roll" {
						continue
					}
					found = true
					if ev.Data["offensive_reroll"] != true || ev.Data["face_before"] != 1 || ev.Data["card_id"] != id || ev.Data["source_actor_id"] != "player" {
						t.Fatalf("forced reroll lost its card/baseline metadata: %+v", ev.Data)
					}
				}
				if !found {
					t.Fatal("forced reroll feedback missing")
				}
			case "ruin_made_flesh":
				ops, _ := resolvedOffensiveOperations(&b, lib, "player")
				amount, _ := operationAmount(ops[0], 0)
				if amount != 7 || stacks(&b, "enemy", "curse_count") != 3 {
					t.Fatal("ruin damage/cost wrong")
				}
			case "blind_omen":
				if stacks(&b, "enemy", "blind") != 1 || stacks(&b, "enemy", "curse_count") != 6 {
					t.Fatal("Blind/cost wrong")
				}
			}
		})
	}
}
func TestCurseDefensiveCards(t *testing.T) {
	for _, id := range []string{"hexward_retort", "spiteful_ward"} {
		t.Run(id, func(t *testing.T) {
			b, lib := curseFixture(t)
			e := NewEngine()
			s := newSettledDamageSource(&b, "enemy", "player", "hexbrand", 4)
			b.Settled.OffensiveSources = []state.SettledDamageSource{s}
			key, target := "apply", "enemy"
			if id == "hexward_retort" {
				b.Settled.DefenseSelections = map[string]state.SettledDefense{"player": {ActorID: "player", SourceID: s.ID, AbilityID: "hexward_rebuttal", RolledFace: 1}}
				openSettledWindow(&b, "def", stageDefenseReact, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePass})
			} else {
				batch, err := e.buildDamageBatch(&b, []state.SettledDamageSource{s})
				if err != nil {
					t.Fatal(err)
				}
				b.Settled.PendingDamage = batch
				openSettledWindow(&b, "damage", stageDamageReact, "damage_response", []command.Type{command.TypeCommitInteraction, command.TypePass})
				key, target = "prevent", s.ID
			}
			cursePlay(t, e, &b, lib, id, key, target)
			if id == "spiteful_ward" {
				if b.Settled.PendingDamage.Sources[0].ReactionPrevention != 2 {
					t.Fatal("ward prevention missing")
				}
				if err := e.curseDamageCompleted(&b, lib, b.Settled.PendingDamage); err != nil {
					t.Fatal(err)
				}
				found := false
				for _, log := range b.Settled.Curse.Logs {
					if log.Data["source_card_id"] == "spiteful_ward" && log.Data["attack_source_id"] == s.ID && log.Data["source_actor_id"] == "player" {
						found = true
					}
				}
				if !found {
					t.Fatal("Ward follow-up must name the card and resolved attack for playback")
				}
			}
			if markedCount(&b, "enemy") != 1 {
				t.Fatal("retaliatory Curse missing")
			}
		})
	}
}
func TestCurseFingerprintBloomAndDelayedCount(t *testing.T) {
	for _, id := range []string{"black_fingerprint", "curse_bloom"} {
		t.Run(id, func(t *testing.T) {
			b, lib := curseFixture(t)
			cursePlan(&b)
			e := NewEngine()
			cursePlay(t, e, &b, lib, id, "apply", "enemy")
			sourceID := "funeral_rattle"
			if id == "curse_bloom" {
				sourceID = "curse_count"
				for i := 0; i < 5; i++ {
					for f := 1; f <= 6; f++ {
						markCurse(&b, "enemy", i, f)
					}
				}
				b.Segment.Current = segment.OngoingEffects
				b.Segment.Round++
				curseRuntime(&b).ConversionRound = b.Segment.Round
			}
			source := newSettledDamageSource(&b, "player", "enemy", sourceID, 2)
			batch := &state.SettledDamageBatch{Sources: []state.SettledDamageSource{source}, Removals: []state.ProposedCardRemoval{{Accepted: true, DamageProposalIDs: []string{source.ID}}, {Accepted: true, DamageProposalIDs: []string{source.ID}}}}
			if err := e.curseDamageCompleted(&b, lib, batch); err != nil {
				t.Fatal(err)
			}
			if id == "black_fingerprint" {
				found := false
				for _, d := range curseRuntime(&b).Dice["enemy"] {
					if containsInt(d.CursedFaces, 2) {
						found = true
					}
				}
				if !found {
					t.Fatal("health-based face mark missing")
				}
			} else if stacks(&b, "enemy", "curse_count") != 3 {
				t.Fatal("Bloom must perform three separate Surges after conversion")
			}
			if len(curseRuntime(&b).Preparations) != 0 {
				t.Fatal("preparation did not consume")
			}
		})
	}
}
func TestAllCurseAbilities(t *testing.T) {
	for _, tc := range []struct {
		id    string
		faces []int
	}{{"hexbrand", []int{1, 2, 3, 4, 6}}, {"grasp_of_the_sarcophagus", []int{4, 5, 6, 1, 2}}, {"funeral_rattle", []int{1, 2, 4, 6, 5}}, {"eclipse_of_the_black_star", []int{2, 3, 4, 5, 6}}} {
		t.Run(tc.id, func(t *testing.T) {
			b, lib := curseFixture(t)
			e := NewEngine()
			r := b.Settled.Actors["player"]
			r.FinalDice = rolledFaces(lib, "curse_d6", tc.faces)
			r.SelectedAbilityID = tc.id
			r.SelectedTierID = "skull_3"
			r.SelectedTargetIDs = []string{"enemy"}
			b.Settled.Actors["player"] = r
			if _, ok := resolvedOffensiveOperations(&b, lib, "player"); !ok {
				t.Fatal("ability didn't qualify")
			}
			switch tc.id {
			case "grasp_of_the_sarcophagus":
				if err := e.queueCurseAbilities(&b, lib); err != nil {
					t.Fatal(err)
				}
				if _, err := e.startCurseWork(&b, lib, false); err != nil {
					t.Fatal(err)
				}
				w := curseRuntime(&b).Active
				if w == nil || len(w.Options) != 2 || markedCount(&b, "enemy") != 2 {
					t.Fatal("grasp didn't seed before choosing")
				}
				if err := e.resolveCurseChoice(&b, lib, w.Options[0]); err != nil {
					t.Fatal(err)
				}
				if len(entombedIndices(&b, "enemy")) != 1 {
					t.Fatal("grasp no binding")
				}
			case "eclipse_of_the_black_star":
				if err := e.queueCurseAbilities(&b, lib); err != nil {
					t.Fatal(err)
				}
				finishCurseChoice(t, e, &b, lib, "5")
				if markedCount(&b, "enemy") != 5 {
					t.Fatal("eclipse didn't mark every die")
				}
			case "hexbrand":
				s := newSettledDamageSource(&b, "player", "enemy", tc.id, 3)
				s.Prevention = 3
				if err := e.curseDamageCompleted(&b, lib, &state.SettledDamageBatch{Sources: []state.SettledDamageSource{s}}); err != nil {
					t.Fatal(err)
				}
				if markedCount(&b, "enemy") != 1 {
					t.Fatal("prevented Hexbrand still applies Curse")
				}
			case "funeral_rattle":
				for i := 0; i < 5; i++ {
					for f := 1; f <= 6; f++ {
						markCurse(&b, "enemy", i, f)
					}
				}
				applyStatus(&b, lib, "enemy", "curse_count", 3)
				s := newSettledDamageSource(&b, "player", "enemy", tc.id, 4)
				if err := e.curseDamageCompleted(&b, lib, &state.SettledDamageBatch{Sources: []state.SettledDamageSource{s}}); err != nil {
					t.Fatal(err)
				}
				if stacks(&b, "enemy", "curse_count") != 6 || stacks(&b, "enemy", "blind") != 1 {
					t.Fatal("rattle didn't roll three/check threshold")
				}
			}
		})
	}
	for _, id := range []string{"hexward_rebuttal", "misfortune_repaid"} {
		t.Run(id, func(t *testing.T) {
			b, lib := curseFixture(t)
			e := NewEngine()
			b.Segment.Current = segment.Defensive
			s := newSettledDamageSource(&b, "enemy", "player", "hexbrand", 5)
			s.Prevention = 2
			b.Settled.OffensiveSources = []state.SettledDamageSource{s}
			d := state.SettledDefense{ActorID: "player", SourceID: s.ID, AbilityID: id, RolledFace: 4, RolledFaces: []int{4, 6}}
			if err := e.curseDefenseCompleted(&b, lib, d); err != nil {
				t.Fatal(err)
			}
			if id == "hexward_rebuttal" {
				if markedCount(&b, "enemy") != 1 {
					t.Fatal("rebuttal no Curse")
				}
			} else {
				if stacks(&b, "enemy", "curse_count") != 2 {
					t.Fatal("Omen Count missing")
				}
				finishCurseChoice(t, e, &b, lib, "2")
				if !containsInt(curseRuntime(&b).Dice["enemy"][2].CursedFaces, 1) {
					t.Fatal("repaid chosen seed missing")
				}
			}
		})
	}
}

func TestCurseBothConversionsCaptureBeforeBloomAndLethal(t *testing.T) {
	b, lib := curseFixture(t)
	e := NewEngine()
	b.Segment.Current = segment.OngoingEffects
	applyStatus(&b, lib, "player", "curse_count", 6)
	applyStatus(&b, lib, "enemy", "curse_count", 9)
	if _, err := e.convertCurse(&b, lib); err != nil {
		t.Fatal(err)
	}
	batch := b.Settled.PendingDamage
	if batch == nil || len(batch.Sources) != 2 || stacks(&b, "player", "curse_count") != 0 || stacks(&b, "enemy", "curse_count") != 0 {
		t.Fatal("both conversions must capture before damage")
	}
	dead := b.Actors["enemy"]
	dead.Cards = state.CardZones{}
	dead.DefeatState = state.ActorPendingDefeat
	b.Actors["enemy"] = dead
	b.Settled.PendingDamage = nil
	applyStatus(&b, lib, "player", "curse_count", 9)
	b.Segment.Round++
	if _, err := e.convertCurse(&b, lib); err != nil {
		t.Fatal(err)
	}
	if !state.IsTerminalBattleStatus(b.Status) || b.Settled.PendingDamage != nil || stacks(&b, "player", "curse_count") != 9 {
		t.Fatal("lethal prior damage must stop later Curse conversion")
	}
}
func TestCurseChoiceSurvivesSaveWithoutReroll(t *testing.T) {
	b, lib := curseFixture(t)
	cursePlan(&b)
	markCurse(&b, "enemy", 1, 1)
	queueCurse(&b, state.CurseWork{Kind: "adjacent", Source: "player", Target: "enemy", CardID: "widen_the_crack", Die: 1})
	e := NewEngine()
	e.namedRandom = &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "curse_dice", Bound: 6, Value: 2}}}
	if _, err := e.startCurseWork(&b, lib, false); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(b.Clone())
	var restored state.Battle
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	w := restored.Settled.Curse.Active
	if w == nil || w.Face != 3 || !reflect.DeepEqual(w.Options, []string{"2", "4"}) {
		t.Fatal("saved adjacent choice changed")
	}
	if err := e.resolveCurseChoice(&restored, lib, "2"); err != nil {
		t.Fatal(err)
	}
	if !containsInt(restored.Settled.Curse.Dice["enemy"][1].CursedFaces, 2) {
		t.Fatal("saved choice didn't resolve")
	}
}
func TestCurseSecondKnellBlocksCatalystForSameCheck(t *testing.T) {
	b, lib := curseFixture(t)
	b.Segment.Current = segment.OngoingEffects
	markCurse(&b, "enemy", 0, 1)
	applyStatus(&b, lib, "player", "catalyst", 1)
	curseRuntime(&b).Preparations = append(curseRuntime(&b).Preparations, state.CursePreparation{CardID: "second_knell", Source: "player", Target: "enemy"})
	e := NewEngine()
	script := &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "owned_die_selection", Bound: 5, Value: 0}, {Stream: "status_effect_dice", Bound: 6, Value: 0}, {Stream: "status_effect_dice", Bound: 6, Value: 5}}}
	e.namedRandom = script
	roll := state.SettledEffectRoll{ActorID: "enemy", SourceContentID: "poison", SourceContentType: "status", Die: state.RolledDie{DieID: "standard_d6"}}
	if err := e.rollOwnedEffect(&b, lib, &roll, "test", "status_effect_dice", false); err != nil {
		t.Fatal(err)
	}
	b.Settled.TriggerBatch = &state.SettledTriggerBatch{ID: "test", Rolls: []state.SettledEffectRoll{roll}}
	if _, opened, err := e.automaticCatalyst(&b, lib); err != nil || opened {
		t.Fatalf("second retry attempted: %v", err)
	}
	if stacks(&b, "player", "catalyst") != 1 || stacks(&b, "enemy", "curse_count") != 1 {
		t.Fatal("retry costs/results incorrect")
	}
	if err := script.AssertExhausted(); err != nil {
		t.Fatal(err)
	}
}

func TestCurseDefenseEveryFaceAndPair(t *testing.T) {
	for _, id := range []string{"hexward_rebuttal", "misfortune_repaid"} {
		for a := 1; a <= 6; a++ {
			limit := 1
			if id == "misfortune_repaid" {
				limit = 6
			}
			for z := 1; z <= limit; z++ {
				t.Run(fmt.Sprintf("%s_%d_%d", id, a, z), func(t *testing.T) {
					b, lib := curseFixture(t)
					e := NewEngine()
					s := newSettledDamageSource(&b, "enemy", "player", "hexbrand", 20)
					b.Settled.OffensiveSources = []state.SettledDamageSource{s}
					faces := []int{a}
					if id == "misfortune_repaid" {
						faces = append(faces, z)
					}
					want := 0
					for _, f := range faces {
						if id == "hexward_rebuttal" {
							if f <= 3 {
								want += 2
							} else if f <= 5 {
								want += 3
							} else {
								want++
							}
						} else {
							if f <= 3 {
								want++
							} else if f <= 5 {
								want += 2
							}
						}
						r, err := e.executeResolvedEffects(&b, lib, effectContext{SourceActorID: "player", SourceContentID: id, SourceContentType: "ability", ProposalIDs: []string{s.ID}, TargetActorIDs: []string{"player"}, RolledFace: f}, lib.Abilities[id].Resolution.Operations)
						if err != nil {
							t.Fatal(err)
						}
						if err = e.applyEffectMutations(&b, lib, "", r); err != nil {
							t.Fatal(err)
						}
					}
					if b.Settled.OffensiveSources[0].Prevention != want {
						t.Fatalf("prevention %d, want %d", b.Settled.OffensiveSources[0].Prevention, want)
					}
				})
			}
		}
	}
}
func TestCurseFullSaturationAndCompetingEntomb(t *testing.T) {
	b, lib := curseFixture(t)
	cursePlan(&b)
	e := NewEngine()
	for i := 0; i < 5; i++ {
		for f := 1; f <= 6; f++ {
			markCurse(&b, "enemy", i, f)
		}
	}
	queueCurse(&b, state.CurseWork{Kind: "eclipse", Source: "player", Target: "enemy", CardID: "eclipse_of_the_black_star"})
	if _, err := e.startCurseWork(&b, lib, false); err != nil {
		t.Fatal(err)
	}
	if stacks(&b, "enemy", "curse_count") != 5 || curseRuntime(&b).Active != nil {
		t.Fatal("saturated Eclipse must Surge all five without a choice")
	}
	if err := e.applyCurse(&b, lib, "enemy", "surge-test", 3); err != nil {
		t.Fatal(err)
	}
	if stacks(&b, "enemy", "curse_count") != 8 || markedCount(&b, "enemy") != 30 {
		t.Fatal("saturated ordinary Curse must Surge")
	}
	entomb(&b, "enemy", 1)
	entomb(&b, "enemy", 4)
	e.namedRandom = &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "owned_die_selection", Bound: 2, Value: 1}, {Stream: "owned_die_selection", Bound: 2, Value: 0}}}
	first, err := e.selectOwned(&b, "enemy", "defense-one", allOwned(&b, "enemy"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := e.selectOwned(&b, "enemy", "defense-two", allOwned(&b, "enemy"))
	if err != nil {
		t.Fatal(err)
	}
	if first != 4 || second != 1 {
		t.Fatal("separate one-die defenses must randomly prioritize both Entombed dice")
	}
}
