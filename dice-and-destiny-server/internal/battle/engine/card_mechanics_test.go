package engine

import (
	"diceanddestiny/server/internal/battle/operation"
	battlerandom "diceanddestiny/server/internal/battle/random"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func configuredClone(t *testing.T, lib *content.BattleLibrary, id string) content.BattleCardDefinition {
	t.Helper()
	c, err := content.EditableCard(lib.Cards[id])
	if err != nil {
		t.Fatal(err)
	}
	c.ID = "custom_" + id
	c.Name = "Custom " + c.Name
	c = content.PrepareMechanicCard(c, lib)
	lib.Cards[c.ID] = c
	if err = content.ValidateAuthoredCard(c, *lib); err != nil {
		t.Fatal(err)
	}
	return c
}
func putMechanic(b *state.Battle, c content.BattleCardDefinition) {
	a := b.Actors["player"]
	a.Cards.Hand = append(a.Cards.Hand, "custom")
	a.Resources.EnergyPoints = 100
	b.Actors["player"] = a
	rt := b.Settled.Actors["player"]
	rt.CardInstances["custom"] = state.CardInstance{InstanceID: "custom", DefinitionID: c.ID}
	b.Settled.Actors["player"] = rt
}
func TestEverySpecializedCardCanBeRenamedAndPlayed(t *testing.T) {
	for id, spec := range content.CardMechanics() {
		if id == "roll_table" {
			continue
		}
		t.Run(id, func(t *testing.T) {
			b, lib := curseFixture(t)
			c := configuredClone(t, &lib, id)
			putMechanic(&b, c)
			cursePlan(&b)
			for _, actor := range []string{"player", "enemy"} {
				applyStatus(&b, lib, actor, "poison", 2)
				applyStatus(&b, lib, actor, "volatile_poison", 1)
				applyStatus(&b, lib, actor, "curse_count", 8)
				for i := 0; i < 5; i++ {
					markCurse(&b, actor, i, 1)
				}
				rt := b.Settled.Actors[actor]
				rt.FinalDice = rolledFaces(lib, "venom_d6", []int{1, 1, 1, 4, 6})
				rt.RollsUsed = 1
				rt.KeptIndices = []int{0, 1}
				rt.SelectedAbilityID = "needlefang"
				rt.SelectedTierID = "fang_3"
				rt.SelectedTargetIDs = []string{enemyOf(&b, actor)}
				rt.OffensiveAbilityIDs = append(rt.OffensiveAbilityIDs, "needlefang", "terminal_bite")
				b.Settled.Actors[actor] = rt
			}
			applyStatus(&b, lib, "player", "catalyst", 2)
			if id == "terminal_formula" {
				rt := b.Settled.Actors["player"]
				rt.SelectedAbilityID = "terminal_bite"
				rt.FinalDice = rolledFaces(lib, "venom_d6", []int{1, 1, 6, 6, 6})
				b.Settled.Actors["player"] = rt
			}
			switch spec.Windows[0] {
			case "offensive_reaction":
				b.Settled.Stage = stageOffensiveReact
			case "defense_selection":
				b.Settled.Stage = stageDefenseSelect
				b.Segment.Current = segment.Defensive
				b.Settled.UnifiedDefense = true
			case "defense_reaction":
				b.Settled.Stage = stageDefenseReact
			}
			source := state.SettledDamageSource{ID: "incoming", SourceActorID: "enemy", TargetActorID: "player", SourceContentID: "sword_cut", BaseAmount: 5, FinalAmount: 5}
			b.Settled.OffensiveSources = []state.SettledDamageSource{source}
			b.Settled.PendingDamage = &state.SettledDamageBatch{ID: "damage", Sources: []state.SettledDamageSource{source}}
			b.Settled.DefenseSelections = map[string]state.SettledDefense{"player": {ActorID: "player", SourceID: source.ID, RolledFace: 4}}
			var choices []venomCardChoice
			if spec.Family == "venom" {
				choices = venomCardChoices(&b, lib, "player", c)
			} else {
				choices = curseCardChoices(&b, lib, "player", c)
			}
			if len(choices) == 0 {
				t.Fatal("configured clone has no legal action")
			}
			for _, choice := range choices {
				branch := b.Clone()
				e := NewEngine()
				var err error
				if spec.Family == "venom" {
					err = e.playVenomCard(&branch, lib, "player", "custom", c, choice.Targets, choice.Key)
				} else {
					err = e.playCurseCard(&branch, lib, "player", "custom", c, choice.Targets, choice.Key)
				}
				if err != nil {
					t.Fatalf("choice %s: %v", choice.Key, err)
				}
				if !containsString(branch.Actors["player"].Cards.Discard, "custom") {
					t.Fatal("not discarded")
				}
				raw, _ := json.Marshal(branch)
				var restored state.Battle
				if err = json.Unmarshal(raw, &restored); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(branch.Settled.Venom, restored.Settled.Venom) {
					t.Fatal("mechanic lost during save")
				}
			}
		})
	}
}
func TestConfiguredAmountsAndDelayedRewards(t *testing.T) {
	t.Run("status cost and draw", func(t *testing.T) {
		b, lib := curseFixture(t)
		c := configuredClone(t, &lib, "repurpose")
		c.Mechanic.Params["amount"] = 4
		c.Mechanic.Params["cost_stacks"] = 2
		putMechanic(&b, c)
		cursePlan(&b)
		applyStatus(&b, lib, "player", "catalyst", 1)
		if len(venomCardChoices(&b, lib, "player", c)) != 0 {
			t.Fatal("unaffordable status cost")
		}
		applyStatus(&b, lib, "player", "catalyst", 1)
		before := len(b.Actors["player"].Cards.Hand)
		if err := NewEngine().playVenomCard(&b, lib, "player", "custom", c, []string{"player"}, "draw"); err != nil {
			t.Fatal(err)
		}
		if stacks(&b, "player", "catalyst") != 0 || len(b.Actors["player"].Cards.Hand) != before+3 {
			t.Fatal("configured draw/cost ignored")
		}
	})
	t.Run("dividend reward survives reload", func(t *testing.T) {
		b, lib := curseFixture(t)
		c := configuredClone(t, &lib, "black_dividend")
		c.Mechanic.Params["energy"] = 3
		c.Mechanic.Params["rewards"] = 1
		lib.Cards[c.ID] = c
		putMechanic(&b, c)
		cursePlan(&b)
		if err := NewEngine().playCurseCard(&b, lib, "player", "custom", c, []string{"enemy"}, "apply"); err != nil {
			t.Fatal(err)
		}
		b = b.Clone()
		before := b.Actors["player"].Resources.EnergyPoints
		rewardBlackDividend(&b, "enemy", state.RolledDie{}, "curse_dice")
		if b.Actors["player"].Resources.EnergyPoints != before+3 || mechanicStacks(&b, "enemy", "black_dividend") != 0 {
			t.Fatal("configured delayed reward ignored")
		}
		rewardBlackDividend(&b, "enemy", state.RolledDie{}, "curse_dice")
		if b.Actors["player"].Resources.EnergyPoints != before+3 {
			t.Fatal("reward repeated")
		}
	})
	t.Run("conversion quantity", func(t *testing.T) {
		b, lib := curseFixture(t)
		c := configuredClone(t, &lib, "distill")
		c.Mechanic.Params["convert_stacks"] = 2
		c.Mechanic.Params["gain_stacks"] = 3
		lib.Cards[c.ID] = c
		putMechanic(&b, c)
		cursePlan(&b)
		applyStatus(&b, lib, "enemy", "poison", 2)
		applyStatus(&b, lib, "player", "catalyst", 1)
		if err := NewEngine().playVenomCard(&b, lib, "player", "custom", c, []string{"enemy"}, "convert"); err != nil {
			t.Fatal(err)
		}
		finishVenomApplications(t, NewEngine(), &b, lib)
		if stacks(&b, "enemy", "poison") != 0 || stacks(&b, "enemy", "volatile_poison") != 3 {
			t.Fatal("configured conversion ignored")
		}
	})
	t.Run("preparation visible consumed expired", func(t *testing.T) {
		b, lib := curseFixture(t)
		c := configuredClone(t, &lib, "black_fingerprint")
		putMechanic(&b, c)
		cursePlan(&b)
		if err := NewEngine().playCurseCard(&b, lib, "player", "custom", c, []string{"enemy"}, "apply"); err != nil {
			t.Fatal(err)
		}
		if mechanicStacks(&b, "enemy", "black_fingerprint") != 1 {
			t.Fatal("hidden preparation")
		}
		b.Segment.Current = segment.DamageResolution
		expireMechanicStatuses(&b)
		expireCursePreparations(&b)
		if mechanicStacks(&b, "enemy", "black_fingerprint") != 0 || len(curseRuntime(&b).Preparations) != 0 {
			t.Fatal("preparation did not expire")
		}
	})
}

func TestConfiguredCardPilesCostsAndLimits(t *testing.T) {
	for _, source := range []operation.CardZone{operation.ZoneHand, operation.ZoneDeck, operation.ZoneDiscard} {
		for _, destination := range []string{"hand", "deck", "discard", "removed"} {
			t.Run(string(source)+" to "+destination, func(t *testing.T) {
				b, lib := curseFixture(t)
				c := configuredClone(t, &lib, "repurpose")
				c.Play.SourceZones = []string{string(source)}
				c.Play.Destination = destination
				c.Cost.Energy = 3
				c.Mechanic.Params["amount"] = 3
				c.Mechanic.Params["cost_stacks"] = 0
				c.Mechanic.UsesPerRound = 1
				lib.Cards[c.ID] = c
				putMechanic(&b, c)
				cursePlan(&b)
				a := b.Actors["player"]
				moveCard(&a.Cards, "custom", operation.ZoneHand, source)
				b.Actors["player"] = a
				hp := a.CurrentHealth()
				energy := a.Resources.EnergyPoints
				if err := NewEngine().playSettledCard(&b, lib, "player", "custom", []string{"player"}, "", 0, "draw"); err != nil {
					t.Fatal(err)
				}
				if programCardZone(&b, "player", "custom") != operation.CardZone(destination) {
					t.Fatal("play destination ignored")
				}
				if b.Actors["player"].Resources.EnergyPoints != energy-3 {
					t.Fatal("energy cost ignored")
				}
				expected := hp
				if destination == "removed" {
					expected--
					if len(b.Wounds) != 1 {
						t.Fatal("missing sacrifice wound")
					}
				}
				if b.Actors["player"].CurrentHealth() != expected {
					t.Fatal("pile move changed health incorrectly")
				}
				b = b.Clone()
				if mechanicAvailable(&b, "player", c) {
					t.Fatal("per-round limit lost on reload")
				}
				b.Segment.Round++
				if !mechanicAvailable(&b, "player", c) {
					t.Fatal("round limit did not reset")
				}
			})
		}
	}
}
func TestSpecializedPreventionDestinations(t *testing.T) {
	for _, destination := range []string{"original", "discard", "hand", "deck", "removed"} {
		t.Run(destination, func(t *testing.T) {
			b, lib, e := unifiedFixture(t, 5)
			_, cards := curseFixture(t)
			for id, c := range cards.Cards {
				lib.Cards[id] = c
			}
			for id, s := range cards.Symbols {
				lib.Symbols[id] = s
			}
			for id, a := range cards.Abilities {
				lib.Abilities[id] = a
			}
			for id, d := range cards.Dice {
				lib.Dice[id] = d
			}
			for id, s := range cards.Statuses {
				lib.Statuses[id] = s
			}
			c := configuredClone(t, &lib, "emergency_molt")
			c.Mechanic.Params["prevent"] = 3
			c.SavedCardDestination = destination
			lib.Cards[c.ID] = c
			putMechanic(&b, c)
			before := b.Actors["player"].CurrentHealth()
			if err := e.playVenomCard(&b, lib, "player", "custom", c, []string{"a"}, "prevent"); err != nil {
				t.Fatal(err)
			}
			if len(activeReservations(b, "a")) != 2 {
				t.Fatal("prevention amount not applied")
			}
			released := 0
			for _, r := range b.Settled.PendingDamage.Removals {
				if r.Released {
					released++
					want := operation.CardZone(destination)
					if destination == "original" {
						want = r.OriginalZone
					}
					if programCardZone(&b, "player", r.CardID) != want {
						t.Fatalf("saved %s in wrong pile", r.CardID)
					}
				}
			}
			if released != 3 {
				t.Fatal("incorrect released count")
			}
			wantHP := before
			if destination == "removed" {
				wantHP -= 3
			}
			if b.Actors["player"].CurrentHealth() != wantHP {
				t.Fatal("prevention destination health mismatch")
			}
			saved := b.Clone()
			if err := e.reconcilePreventionDestination(&saved, destination); err != nil {
				t.Fatal(err)
			}
			if saved.Actors["player"].CurrentHealth() != wantHP {
				t.Fatal("reconciliation resurrected cards")
			}
		})
	}
}
func TestConfiguredStatusExpirationAndReload(t *testing.T) {
	for _, checkpoint := range []string{"offensive_exit", "damage_exit", "next_income", "next_ongoing", "battle"} {
		t.Run(checkpoint, func(t *testing.T) {
			b, lib := curseFixture(t)
			c := configuredClone(t, &lib, "black_fingerprint")
			c.Mechanic.Expiration = checkpoint
			c.Mechanic.Rounds = 2
			lib.Cards[c.ID] = c
			putMechanic(&b, c)
			cursePlan(&b)
			if err := NewEngine().playCurseCard(&b, lib, "player", "custom", c, []string{"enemy"}, "apply"); err != nil {
				t.Fatal(err)
			}
			b = b.Clone()
			start := b.Segment.Round
			phases := map[string]segment.Segment{"offensive_exit": segment.Offensive, "damage_exit": segment.DamageResolution, "next_income": segment.Income, "next_ongoing": segment.OngoingEffects, "battle": segment.Income}
			b.Segment.Current = phases[checkpoint]
			expireCursePreparations(&b)
			if mechanicStacks(&b, "enemy", "black_fingerprint") != 1 {
				t.Fatal("expired too early")
			}
			if checkpoint == "battle" {
				b.Status = state.BattleStatus("defeat")
			} else {
				b.Segment.Round = start + 2
				if checkpoint == "offensive_exit" || checkpoint == "damage_exit" {
					b.Segment.Round--
				}
			}

			expireCursePreparations(&b)
			if mechanicStacks(&b, "enemy", "black_fingerprint") != 0 || len(curseRuntime(&b).Preparations) != 0 {
				t.Fatal("configured expiration ignored")
			}
		})
	}
}
func TestSpecializedParametersRejectIgnoredValues(t *testing.T) {
	for kind := range content.CardMechanics() {
		if kind == "roll_table" {
			continue
		}
		t.Run(kind, func(t *testing.T) {
			_, lib := curseFixture(t)
			c := configuredClone(t, &lib, kind)
			c.Mechanic.Params["misspelled_amount"] = 4
			if content.ValidateAuthoredCard(c, lib) == nil {
				t.Fatal("ignored field accepted")
			}
			delete(c.Mechanic.Params, "misspelled_amount")
			c.Operations[0].Amount = 4
			if content.ValidateAuthoredCard(c, lib) == nil {
				t.Fatal("ignored operation amount accepted")
			}
		})
	}
}

func TestConfiguredRollTableAllFaces(t *testing.T) {
	for face := 1; face <= 6; face++ {
		t.Run(fmt.Sprint(face), func(t *testing.T) {
			b, lib := curseFixture(t)
			c := configuredClone(t, &lib, "alchemists_gamble")
			c.Cost.Energy = 2
			c.Play.Destination = "removed"
			c.Operations[0].Outcomes = nil
			for n := 1; n <= 6; n++ {
				c.Operations[0].Outcomes = append(c.Operations[0].Outcomes, content.BattleOutcome{Faces: []int{n}, Operations: []content.BattleOperation{{Type: "gain_resource", Resource: "energy", Target: "self", Amount: n}, {Type: "deal_damage", Target: "selected_targets", Amount: n + 1}}})
			}
			lib.Cards[c.ID] = c
			if err := content.ValidateAuthoredCard(c, lib); err != nil {
				t.Fatal(err)
			}
			putMechanic(&b, c)
			cursePlan(&b)
			energy := b.Actors["player"].Resources.EnergyPoints
			hp := b.Actors["player"].CurrentHealth()
			e := NewEngine()
			e.namedRandom = &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "owned_die_selection", Bound: 5, Value: 0}, {Stream: "effect_dice", Bound: 6, Value: face - 1}}}
			if err := e.playSettledCard(&b, lib, "player", "custom", []string{"enemy"}, "", 0, ""); err != nil {
				t.Fatal(err)
			}
			if b.Actors["player"].Resources.EnergyPoints != energy-2+face {
				t.Fatal("outcome energy ignored")
			}
			if len(b.Settled.OffensiveSources) != 1 || b.Settled.OffensiveSources[0].BaseAmount != face+1 {
				t.Fatal("outcome damage ignored")
			}
			if b.Actors["player"].CurrentHealth() != hp-1 || len(b.Wounds) != 1 {
				t.Fatal("roll table removal destination not recorded")
			}
		})
	}
}

func TestConfiguredKnellRetriesRetainEveryResult(t *testing.T) {
	b, lib := curseFixture(t)
	cursePlan(&b)
	c := configuredClone(t, &lib, "second_knell")
	c.Mechanic.Params["retries"] = 3
	lib.Cards[c.ID] = c
	putMechanic(&b, c)
	e := NewEngine()
	cursePlay(t, e, &b, lib, c.ID, "apply", "enemy")
	markCurse(&b, "enemy", 2, 1)
	b = b.Clone()
	flushCurseEvents(&b)
	e.namedRandom = &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{
		{Stream: "curse_dice", Bound: 6, Value: 0}, {Stream: "curse_dice", Bound: 6, Value: 1},
		{Stream: "curse_dice", Bound: 6, Value: 0}, {Stream: "curse_dice", Bound: 6, Value: 5},
	}}
	result, err := e.ownedRoll(&b, lib, "enemy", 2, "curse_dice", true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Face != 6 || stacks(&b, "enemy", "curse_count") != 2 || mechanicStacks(&b, "enemy", "second_knell") != 0 {
		t.Fatal("configured retries/count/consumption failed")
	}
	found := false
	for _, ev := range flushCurseEvents(&b) {
		if ev.Data["kind"] != "second_knell_trigger" {
			continue
		}
		data, _ := json.Marshal(ev.Data)
		var event struct {
			Source string            `json:"source_card_id"`
			Dice   []state.RolledDie `json:"retry_dice"`
			Cursed []bool            `json:"retry_cursed_faces"`
		}
		if err := json.Unmarshal(data, &event); err != nil {
			t.Fatal(err)
		}
		if event.Source != c.ID || len(event.Dice) != 3 || !reflect.DeepEqual(event.Cursed, []bool{false, true, false}) {
			t.Fatalf("incomplete retry event: %s", data)
		}
		found = true
	}
	if !found {
		t.Fatal("missing retry event")
	}
}
